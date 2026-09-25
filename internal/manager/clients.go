// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Bahonio/awg-docui/internal/amnezialink"
	"github.com/Bahonio/awg-docui/internal/api"
	"github.com/Bahonio/awg-docui/internal/awg"
	"github.com/Bahonio/awg-docui/internal/netutil"
	"github.com/Bahonio/awg-docui/internal/wgconf"
)

// Clients returns detached copies of every client, optionally narrowed to
// one server.
func (m *Manager) Clients(serverID string) []api.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := m.cfg.Servers
	if serverID != "" {
		srv := m.findServer(serverID)
		if srv == nil {
			return []api.Client{}
		}
		servers = []api.Server{*srv}
	}

	clients := []api.Client{}
	for i := range servers {
		for j := range servers[i].Clients {
			c := cloneClient(&servers[i].Clients[j])
			// Fields a config written before they existed may still be
			// missing; the UI expects both to be set.
			if c.Status == "" {
				c.Status = "active"
			}
			if c.ISettings == nil {
				c.ISettings = map[string]string{}
			}
			clients = append(clients, c)
		}
	}
	return clients
}

// ClientConfig renders the .conf a client imports, with or without the
// header comments.
func (m *Manager) ClientConfig(serverID, clientID string, includeComments bool) (string, error) {
	srv, client, err := m.serverAndClient(serverID, clientID)
	if err != nil {
		return "", err
	}
	if client.ClientPrivateKey == "" {
		return "", fmt.Errorf("the private key for this host-adopted peer is unavailable: %w", ErrConflict)
	}
	return wgconf.ClientConf(&srv, &client, m.endpointFor(&srv), includeComments), nil
}

// ClientLink renders the vpn:// link for a client.
func (m *Manager) ClientLink(serverID, clientID string) (string, error) {
	srv, client, err := m.serverAndClient(serverID, clientID)
	if err != nil {
		return "", err
	}
	if client.ClientPrivateKey == "" {
		return "", fmt.Errorf("the private key for this host-adopted peer is unavailable: %w", ErrConflict)
	}
	return amnezialink.Build(&srv, &client, m.endpointFor(&srv))
}

// serverAndClient snapshots both halves of a client export.
func (m *Manager) serverAndClient(serverID, clientID string) (api.Server, api.Client, error) {
	srv, ok := m.Server(serverID)
	if !ok {
		return api.Server{}, api.Client{}, serverNotFound(serverID)
	}
	client, ok := m.Client(serverID, clientID)
	if !ok {
		return api.Server{}, api.Client{}, clientNotFound(clientID)
	}
	return srv, client, nil
}

// clientConfigOf renders the config for a client the caller already holds a
// copy of - the one an update just produced.
func (m *Manager) clientConfigOf(serverID string, client *api.Client) string {
	if client.ClientPrivateKey == "" {
		return ""
	}
	srv, ok := m.Server(serverID)
	if !ok {
		return ""
	}
	return wgconf.ClientConf(&srv, client, m.endpointFor(&srv), true)
}

// AddClient adds a WireGuard peer to a server and returns the client with
// its rendered config.
func (m *Manager) AddClient(serverID string, req api.AddClientRequest) (*api.Client, string, error) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	// The name lands in a .conf comment, in the peer marker and in a
	// Content-Disposition filename, so it is cleaned before anything stores
	// it - not at each of those points.
	name := wgconf.SanitizeName(req.Name, "client")
	if err := validateAllowedIPs(req.AllowedIPs); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	// Checked before any key is generated: a malformed signature packet would
	// otherwise reach the client's .conf and only surface there, as a tunnel
	// that will not start.
	if req.ApplyISettings {
		if err := validateISettings(req.ISettings); err != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrInvalid, err)
		}
	}

	// Key generation shells out to awg three times. Doing that under the
	// write lock would stall every other request, including plain reads, for
	// the duration - and the keys do not depend on any config state.
	keys, err := m.tools.GenerateKeyPairE()
	if err != nil {
		return nil, "", err
	}
	psk, err := m.tools.GeneratePresharedKeyE()
	if err != nil {
		return nil, "", err
	}

	client, ifaceName, err := m.addClientLocked(serverID, name, req, keys, psk)
	if err != nil {
		return nil, "", err
	}

	m.syncLiveConfig(ifaceName)

	fmt.Printf("Client %s added with AllowedIPs: %s\n", name, client.AllowedIPs)
	return client, m.clientConfigOf(serverID, client), nil
}

// addClientLocked builds the client, appends it to the server's peer config
// file and to the in-memory config, all under a single write lock.
func (m *Manager) addClientLocked(serverID, name string, req api.AddClientRequest, keys awg.KeyPair, psk string) (client *api.Client, ifaceName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv := m.findServer(serverID)
	if srv == nil {
		return nil, "", serverNotFound(serverID)
	}

	originalServer := cloneServer(srv)
	clientIP := m.nextClientIP(srv)
	if clientIP == "" {
		return nil, "", fmt.Errorf("subnet is full: %w", ErrConflict)
	}

	iSettings := map[string]string{}
	if req.ApplyISettings {
		iSettings = wgconf.MergeISettings(req.ISettings)
	}

	newClient := api.Client{
		ID:                 uuid.New().String()[:6],
		Name:               name,
		ServerID:           serverID,
		ServerName:         srv.Name,
		Status:             "active",
		CreatedAt:          float64(time.Now().Unix()),
		ClientPrivateKey:   keys.Private,
		ClientPublicKey:    keys.Public,
		PresharedKey:       psk,
		ClientIP:           clientIP,
		ObfuscationEnabled: srv.ObfuscationEnabled,
		ObfuscationParams:  cloneObfuscationParams(srv.ObfuscationParams),
		ApplyISettings:     req.ApplyISettings,
		ISettings:          iSettings,
		AllowedIPs:         wgconf.AllowedIPsOrDefault(req.AllowedIPs),
		ConfigAvailable:    true,
	}

	// The server side routes only the client's own address to it, whatever
	// the client itself sends through the tunnel.
	content, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath)
	if err != nil {
		*srv = originalServer
		return nil, "", fmt.Errorf("reading host config: %w", err)
	}
	originalContent := content
	content = wgconf.AppendPeerText(content, &newClient, clientIP+"/32")
	if err := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, content); err != nil {
		*srv = originalServer
		return nil, "", fmt.Errorf("failed to write client to server config: %w", err)
	}

	srv.Clients = append(srv.Clients, newClient)
	if err := m.saveLocked("new client"); err != nil {
		*srv = originalServer
		if rollbackErr := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, originalContent); rollbackErr != nil {
			return nil, "", fmt.Errorf("%w; rolling back host config: %v", err, rollbackErr)
		}
		return nil, "", err
	}

	// newClient is already a detached copy; the caller gets it rather than a
	// pointer into srv.Clients, which the next append could move.
	return &newClient, srv.Interface, nil
}

// nextClientIP hands out an address in the server's subnet, reusing one a
// deleted client gave back before allocating a fresh one. Caller holds mu.
func (m *Manager) nextClientIP(srv *api.Server) string {
	if len(srv.UnboundNATIPs) > 0 {
		ip := srv.UnboundNATIPs[0]
		srv.UnboundNATIPs = srv.UnboundNATIPs[1:]
		return ip
	}

	used := map[string]bool{srv.ServerIP: true}
	for _, c := range srv.Clients {
		used[c.ClientIP] = true
	}
	return netutil.NextFreeIP(srv.Subnet, used)
}

// DeleteClient removes a peer from a server.
func (m *Manager) DeleteClient(serverID, clientID string) error {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	serverName, clientCopy, ifaceName, err := m.deleteClientLocked(serverID, clientID)
	if err != nil {
		return err
	}

	m.syncLiveConfig(ifaceName)

	fmt.Printf("Client %s:%s removed\n", serverName, clientCopy.Name)
	return nil
}

// deleteClientLocked drops the client from the server's peer list and from
// its .conf under a single write lock, so the file and the config cannot
// disagree about which peers exist. It returns copies only - no pointer into
// the config outlives the lock.
func (m *Manager) deleteClientLocked(serverID, clientID string) (serverName string, clientCopy api.Client, ifaceName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv, target := m.findClient(serverID, clientID)
	if srv == nil {
		return "", api.Client{}, "", serverNotFound(serverID)
	}
	if target == nil {
		return "", api.Client{}, "", clientNotFound(clientID)
	}

	originalServer := cloneServer(srv)
	clientCopy = cloneClient(target)
	content, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath)
	if err != nil {
		return "", api.Client{}, "", fmt.Errorf("reading host config: %w", err)
	}
	originalContent := content
	content, _, _ = wgconf.RemovePeerText(content, target)
	if err := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, content); err != nil {
		return "", api.Client{}, "", fmt.Errorf("removing the peer from %s: %w", srv.ConfigPath, err)
	}

	newClients := make([]api.Client, 0, len(srv.Clients)-1)
	for _, c := range srv.Clients {
		if c.ID != clientID {
			newClients = append(newClients, c)
		}
	}
	srv.Clients = newClients
	srv.UnboundNATIPs = append(srv.UnboundNATIPs, clientCopy.ClientIP)
	if err := m.saveLocked("client removal"); err != nil {
		*srv = originalServer
		if rollbackErr := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, originalContent); rollbackErr != nil {
			return "", api.Client{}, "", fmt.Errorf("%w; rolling back host config: %v", err, rollbackErr)
		}
		return "", api.Client{}, "", err
	}

	return srv.Name, clientCopy, srv.Interface, nil
}

// UpdateClientAllowedIPs changes the AllowedIPs field for a client.
func (m *Manager) UpdateClientAllowedIPs(serverID, clientID, allowedIPs string) (*api.Client, string, error) {
	if err := validateAllowedIPs(allowedIPs); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	clientCopy, err := m.updateClientLocked(serverID, clientID, func(client *api.Client) {
		client.AllowedIPs = wgconf.AllowedIPsOrDefault(allowedIPs)
	})
	if err != nil {
		return nil, "", err
	}

	return clientCopy, m.clientConfigOf(serverID, clientCopy), nil
}

// UpdateClientISettings updates the I1-I5 settings for a client.
func (m *Manager) UpdateClientISettings(serverID, clientID string, applyI *bool, iSettings map[string]string) (*api.Client, string, error) {
	if err := validateISettings(iSettings); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	clientCopy, err := m.updateClientLocked(serverID, clientID, func(client *api.Client) {
		if applyI != nil {
			client.ApplyISettings = *applyI
		}
		if iSettings == nil {
			return
		}
		if client.ApplyISettings {
			client.ISettings = wgconf.MergeISettings(client.ISettings, iSettings)
		} else {
			client.ISettings = map[string]string{}
		}
	})
	if err != nil {
		return nil, "", err
	}

	return clientCopy, m.clientConfigOf(serverID, clientCopy), nil
}

// UpdateClient changes the client properties exposed by the editor. Renaming
// an active client only retags its comment in the host config and applies the
// resulting config live; it never cycles the interface.
func (m *Manager) UpdateClient(serverID, clientID string, req api.UpdateClientRequest) (*api.Client, string, error) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	if req.AllowedIPs != nil {
		if err := validateAllowedIPs(*req.AllowedIPs); err != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrInvalid, err)
		}
	}
	if req.ISettings != nil {
		if err := validateISettings(req.ISettings); err != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrInvalid, err)
		}
	}

	m.mu.Lock()
	srv, client := m.findClient(serverID, clientID)
	if srv == nil {
		m.mu.Unlock()
		return nil, "", serverNotFound(serverID)
	}
	if client == nil {
		m.mu.Unlock()
		return nil, "", clientNotFound(clientID)
	}

	original := cloneClient(client)
	originalName := client.Name
	originalContent := ""
	hostChanged := false
	if req.Name != nil {
		client.Name = wgconf.SanitizeName(*req.Name, "client")
	}
	if req.AllowedIPs != nil {
		client.AllowedIPs = wgconf.AllowedIPsOrDefault(*req.AllowedIPs)
	}
	if req.ApplyISettings != nil {
		client.ApplyISettings = *req.ApplyISettings
	}
	if req.ISettings != nil {
		if client.ApplyISettings {
			client.ISettings = wgconf.MergeISettings(client.ISettings, req.ISettings)
		} else {
			client.ISettings = map[string]string{}
		}
	}

	if client.Name != originalName && client.Status != "suspended" {
		content, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath)
		if err != nil {
			*client = original
			m.mu.Unlock()
			return nil, "", fmt.Errorf("reading host config: %w", err)
		}
		updated, found := wgconf.RetagPeerText(content, client)
		if !found {
			*client = original
			m.mu.Unlock()
			return nil, "", fmt.Errorf("client peer block is missing: %w", ErrConflict)
		}
		if err := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, updated); err != nil {
			*client = original
			m.mu.Unlock()
			return nil, "", fmt.Errorf("renaming client in host config: %w", err)
		}
		originalContent = content
		hostChanged = true
	}

	if err := m.saveLocked("client update"); err != nil {
		*client = original
		if hostChanged {
			if rollbackErr := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, originalContent); rollbackErr != nil {
				m.mu.Unlock()
				return nil, "", fmt.Errorf("%w; rolling back host config: %v", err, rollbackErr)
			}
		}
		m.mu.Unlock()
		return nil, "", err
	}
	clientCopy := cloneClient(client)
	m.mu.Unlock()
	return &clientCopy, m.clientConfigOf(serverID, &clientCopy), nil
}

func validateAllowedIPs(value string) error {
	value = wgconf.AllowedIPsOrDefault(value)
	for _, raw := range strings.Split(value, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil || (!prefix.Addr().Is4() && !prefix.Addr().Is6()) {
			return fmt.Errorf("invalid AllowedIPs prefix %q", strings.TrimSpace(raw))
		}
	}
	return nil
}

// updateClientLocked applies edit to the client under a single write lock
// and returns a copy of the result.
func (m *Manager) updateClientLocked(serverID, clientID string, edit func(*api.Client)) (*api.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv, client := m.findClient(serverID, clientID)
	if srv == nil {
		return nil, serverNotFound(serverID)
	}
	if client == nil {
		return nil, clientNotFound(clientID)
	}

	original := cloneClient(client)
	edit(client)
	if err := m.saveLocked("client update"); err != nil {
		*client = original
		return nil, err
	}

	clientCopy := cloneClient(client)
	return &clientCopy, nil
}
