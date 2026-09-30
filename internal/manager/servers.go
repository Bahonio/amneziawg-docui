// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Bahonio/amneziawg-docui/internal/api"
	"github.com/Bahonio/amneziawg-docui/internal/awg"
	"github.com/Bahonio/amneziawg-docui/internal/netutil"
	"github.com/Bahonio/amneziawg-docui/internal/wgconf"
)

var endpointLabelRE = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)

// Servers returns all servers with their current live status.
//
// Status is filled in from the kernel rather than read out of the config: the
// stored field is only the last thing this process observed, and the
// interface can go up or down without it. The lookups happen between the two
// locks, not under one - this is what every dashboard poll calls, and holding
// the write lock across a shell command per server would stall every other
// request for as long as a host-agent call takes.
func (m *Manager) Servers() []api.Server {
	if err := m.adoptExisting(); err != nil {
		fmt.Printf("Failed to adopt host interfaces: %v\n", err)
	}
	servers := m.copyServers()

	for i := range servers {
		servers[i].Status = m.serverStatus(servers[i].Interface)
	}

	m.storeServerStatuses(servers)
	return servers
}

// storeServerStatuses keeps the config's echo of the status current, in
// memory only. It is not persisted: the kernel is asked on every read anyway,
// so writing an observation to disk would rewrite a file full of private keys
// on a plain dashboard poll and still tell the next reader nothing it can
// trust.
func (m *Manager) storeServerStatuses(observed []api.Server) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range observed {
		if srv := m.findServer(observed[i].ID); srv != nil {
			srv.Status = observed[i].Status
		}
	}
}

// ServerInfo is the detailed view of one server: its fields, its clients and
// the head of its .conf.
func (m *Manager) ServerInfo(id string) (api.ServerInfo, error) {
	srv, ok := m.Server(id)
	if !ok {
		return api.ServerInfo{}, serverNotFound(id)
	}

	status := m.serverStatus(srv.Interface)
	clients := m.Clients(id)

	preview := ""
	if data, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath); err == nil {
		lines := strings.Split(data, "\n")
		if len(lines) > 10 {
			lines = lines[:10]
		}
		for i, line := range lines {
			key, _, ok := strings.Cut(line, "=")
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "privatekey", "presharedkey":
				if ok {
					lines[i] = strings.TrimRight(key, " \t") + " = [redacted]"
				}
			}
		}
		preview = strings.Join(lines, "\n")
	}

	return api.ServerInfo{
		ID:                 srv.ID,
		Name:               srv.Name,
		Protocol:           srv.Protocol,
		Port:               srv.Port,
		Status:             status,
		Interface:          srv.Interface,
		ConfigPath:         srv.ConfigPath,
		PublicIP:           srv.PublicIP,
		ServerIP:           srv.ServerIP,
		Subnet:             srv.Subnet,
		MTU:                srv.MTU,
		ObfuscationEnabled: srv.ObfuscationEnabled,
		ObfuscationParams:  srv.ObfuscationParams,
		ClientsCount:       len(clients),
		Clients:            clients,
		CreatedAt:          srv.CreatedAt,
		ConfigPreview:      preview,
		PublicKey:          srv.ServerPublicKey,
		DNS:                srv.DNS,
		DefaultISettings:   wgconf.DefaultISettings(),
	}, nil
}

// ServerConfig is a server's .conf as it sits on disk, with its identity.
func (m *Manager) ServerConfig(id string) (api.ServerConfig, error) {
	srv, ok := m.Server(id)
	if !ok {
		return api.ServerConfig{}, serverNotFound(id)
	}
	data, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath)
	if err != nil {
		return api.ServerConfig{}, fmt.Errorf("reading host config %s: %w", srv.ConfigPath, err)
	}
	return api.ServerConfig{
		ServerID:      id,
		ServerName:    srv.Name,
		ConfigPath:    srv.ConfigPath,
		ConfigContent: data,
		Interface:     srv.Interface,
		PublicKey:     srv.ServerPublicKey,
	}, nil
}

// CreateServer creates a new WireGuard server configuration.
func (m *Manager) CreateServer(req api.CreateServerRequest) (*api.Server, error) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	if err := m.tools.RequireReady(); err != nil {
		return nil, err
	}
	name := wgconf.SanitizeName(req.Name, "New Server")
	port := req.Port
	if port == 0 {
		port = m.settings.DefaultPort
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535: %w", ErrInvalid)
	}
	if holder, ok := m.portInUse(port); ok {
		return nil, fmt.Errorf("port %d is already used by %s: %w", port, holder, ErrConflict)
	}
	subnet := req.Subnet
	if subnet == "" {
		subnet = m.settings.DefaultSubnet
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(subnet))
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() > 30 {
		return nil, fmt.Errorf("subnet must be an IPv4 CIDR with client capacity: %w", ErrInvalid)
	}
	prefix = prefix.Masked()
	subnet = prefix.String()
	mtu := req.MTU
	if mtu == 0 {
		mtu = m.settings.DefaultMTU
	}
	if mtu < api.MinMTU || mtu > api.MaxMTU {
		return nil, fmt.Errorf("MTU must be between %d and %d, got %d: %w", api.MinMTU, api.MaxMTU, mtu, ErrInvalid)
	}

	endpoint, err := normalizeEndpoint(req.Endpoint)
	if err != nil {
		return nil, err
	}
	publicIP := m.PublicIP()

	dnsServers := parseDNS(req.DNS)
	if len(dnsServers) == 0 {
		dnsServers = m.settings.DNSServers
	}
	for _, dns := range dnsServers {
		if !netutil.IsIPv4(dns) {
			return nil, fmt.Errorf("invalid DNS server IP %s: %w", dns, ErrInvalid)
		}
	}

	enableObfuscation := m.settings.EnableObfuscation
	if req.Obfuscation != nil {
		enableObfuscation = *req.Obfuscation
	}

	autoStart := m.settings.AutoStart
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}

	// AmneziaWG 1.0/1.5/2.0-only modes are no longer supported: enabling
	// obfuscation always means the full AmneziaWG 3.x parameter set,
	// including mandatory header protection.
	var obfParams *api.ObfuscationParams
	if enableObfuscation {
		if req.ObfuscationParams != nil {
			obfParams = req.ObfuscationParams
		} else {
			p := generateObfuscationParams(mtu)
			obfParams = &p
		}
		if obfParams.HeaderProtectionKey == "" {
			obfParams.HeaderProtectionKey = awg.RandomKey()
		}
		if err := validateObfuscationParams(obfParams, mtu); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
		}
	}

	serverID := uuid.New().String()[:6]
	ifaceName := "wg-" + serverID
	configPath := filepath.Join(m.settings.WireguardConfigDir, ifaceName+".conf")

	keys, err := m.tools.GenerateKeyPairE()
	if err != nil {
		return nil, err
	}

	serverIP := prefix.Addr().Next().String()

	configContent := wgconf.ServerInterface{
		PrivateKey:  keys.Private,
		Address:     serverIP + "/" + fmt.Sprint(prefix.Bits()),
		ListenPort:  port,
		MTU:         mtu,
		Obfuscation: obfParams,
	}.Render()
	if err := m.tools.CreateConfig(ifaceName, configPath, configContent, subnet); err != nil {
		return nil, err
	}

	srv := api.Server{
		ID:                 serverID,
		Name:               name,
		Protocol:           "wireguard",
		Port:               port,
		Status:             "stopped",
		Interface:          ifaceName,
		ConfigPath:         configPath,
		ServerPublicKey:    keys.Public,
		Subnet:             subnet,
		ServerIP:           serverIP,
		MTU:                mtu,
		PublicIP:           publicIP,
		Endpoint:           endpoint,
		ObfuscationEnabled: enableObfuscation,
		ObfuscationParams:  obfParams,
		AutoStart:          autoStart,
		DNS:                dnsServers,
		Clients:            []api.Client{},
		UnboundNATIPs:      []string{},
		CreatedAt:          float64(time.Now().Unix()),
	}

	m.mu.Lock()
	m.cfg.Servers = append(m.cfg.Servers, srv)
	if err := m.saveLocked("new server"); err != nil {
		m.cfg.Servers = m.cfg.Servers[:len(m.cfg.Servers)-1]
		m.mu.Unlock()
		if cleanupErr := m.tools.DeleteConfig(ifaceName, configPath); cleanupErr != nil {
			return nil, fmt.Errorf("%w; cleaning up unsaved host config: %v", err, cleanupErr)
		}
		return nil, err
	}
	m.mu.Unlock()

	if autoStart {
		fmt.Printf("Auto-starting new server: %s\n", name)
		if err := m.StartServer(serverID); err != nil {
			fmt.Printf("Auto-start failed for %s: %v\n", name, err)
			return nil, err
		}
	}

	return &srv, nil
}

// UpdateServerEndpoint changes only panel metadata used for client exports;
// the listening interface and every existing peer stay untouched. Passing an
// empty value clears the override so endpointFor falls back to PublicIP.
func (m *Manager) UpdateServerEndpoint(serverID, value string) (*api.Server, error) {
	endpoint, err := normalizeEndpoint(value)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	srv := m.findServer(serverID)
	if srv == nil {
		return nil, serverNotFound(serverID)
	}
	original := srv.Endpoint
	srv.Endpoint = endpoint
	if err := m.saveLocked("server endpoint"); err != nil {
		srv.Endpoint = original
		return nil, err
	}
	updated := cloneServer(srv)
	updated.Clients = nil
	return &updated, nil
}

func normalizeEndpoint(value string) (string, error) {
	endpoint := strings.TrimSpace(value)
	if endpoint == "" {
		return "", nil
	}
	if addr, err := netip.ParseAddr(endpoint); err == nil {
		if addr.Is4() {
			return addr.String(), nil
		}
		return "", fmt.Errorf("endpoint must be an IPv4 address or DNS hostname without a port: %w", ErrInvalid)
	}
	if strings.Contains(endpoint, ".") && strings.Trim(endpoint, "0123456789.") == "" {
		return "", fmt.Errorf("endpoint must be an IPv4 address or DNS hostname without a port: %w", ErrInvalid)
	}
	if len(endpoint) > 253 {
		return "", fmt.Errorf("endpoint must be an IPv4 address or DNS hostname without a port: %w", ErrInvalid)
	}
	for _, label := range strings.Split(endpoint, ".") {
		if len(label) == 0 || len(label) > 63 || !endpointLabelRE.MatchString(label) {
			return "", fmt.Errorf("endpoint must be an IPv4 address or DNS hostname without a port: %w", ErrInvalid)
		}
	}
	return endpoint, nil
}

// parseDNS reads the DNS field of a create request, which the form sends as
// a comma separated string and an API caller may send as a list.
func parseDNS(raw interface{}) []string {
	var out []string
	switch v := raw.(type) {
	case string:
		for _, d := range strings.Split(v, ",") {
			if s := strings.TrimSpace(d); s != "" {
				out = append(out, s)
			}
		}
	case []interface{}:
		for _, d := range v {
			if s, ok := d.(string); ok {
				if t := strings.TrimSpace(s); t != "" {
					out = append(out, t)
				}
			}
		}
	}
	return out
}

// portInUse names whatever already holds this port - another server, or the
// web UI's own listener - if anything does. Two interfaces cannot share a
// port: the second one comes up only to have awg-quick fail on bind, long
// after the create request was answered, so the clash is worth catching up
// front. The panel's port is reserved along with them: it is TCP rather than
// UDP and would technically coexist, but a deployment that publishes one
// number for two different things is a trap, not a feature.
func (m *Manager) portInUse(port int) (string, bool) {
	if m.settings.WebUIPort == port {
		return "the web UI", true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.cfg.Servers {
		if m.cfg.Servers[i].Port == port {
			return fmt.Sprintf("server %q", m.cfg.Servers[i].Name), true
		}
	}
	return "", false
}

// DeleteServer stops and removes a server and its clients.
func (m *Manager) DeleteServer(serverID string) error {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	srv, ok := m.Server(serverID)
	if !ok {
		return serverNotFound(serverID)
	}
	// A server deletion removes the host's only active config. Verify that the
	// metadata volume is writable before touching it; if the final save still
	// fails because the disk changes underneath us, the agent's archived host
	// config and the unchanged metadata remain available for recovery.
	if err := m.SaveConfig(); err != nil {
		return fmt.Errorf("verifying metadata persistence before server removal: %w", err)
	}

	// The stored Status is only the last observation; the interface may have
	// been brought up or down since - including by a restart of this process
	// - and deleting a server whose interface is still up would leave it and
	// its iptables rules behind.
	if m.serverStatus(srv.Interface) == "running" {
		if err := m.StopServer(serverID); err != nil {
			return fmt.Errorf("stopping the server first: %w", err)
		}
	}
	if err := m.tools.DeleteConfig(srv.Interface, srv.ConfigPath); err != nil {
		return fmt.Errorf("deleting host config: %w", err)
	}

	m.mu.Lock()
	original := cloneConfig(m.cfg)
	if !m.removeServerLockedWithoutLock(serverID) {
		m.mu.Unlock()
		return serverNotFound(serverID)
	}
	if err := m.saveLocked("server removal"); err != nil {
		m.cfg = original
		m.mu.Unlock()
		return fmt.Errorf("%w; host config was archived by the agent and panel metadata was retained", err)
	}
	m.mu.Unlock()
	m.forgetServerStatus(srv.Interface)
	return nil
}

// removeServerLockedWithoutLock removes the panel metadata after the host
// agent has stopped the explicitly deleted interface and archived its config.
// Caller holds m.mu.
func (m *Manager) removeServerLockedWithoutLock(serverID string) bool {

	idx := -1
	for i, s := range m.cfg.Servers {
		if s.ID == serverID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}

	// The server owns its clients, so dropping it drops them with it.
	m.cfg.Servers = append(m.cfg.Servers[:idx], m.cfg.Servers[idx+1:]...)
	return true
}

// StartServer brings up a WireGuard interface.
func (m *Manager) StartServer(serverID string) error {
	srv, ok := m.Server(serverID)
	if !ok {
		return serverNotFound(serverID)
	}

	// awg-quick has nothing to do on an interface that is already up: it
	// fails with "File exists", which is a conflict, not a broken server.
	if m.serverStatus(srv.Interface) == "running" {
		return fmt.Errorf("server %s is already running: %w", serverID, ErrConflict)
	}

	if err := m.tools.QuickUp(srv.Interface); err != nil {
		fmt.Printf("Failed to start server %s: %v\n", srv.Name, err)
		return err
	}

	m.setServerStatus(serverID, "running")
	m.noteServerStatus(srv.Interface, "running")

	fmt.Printf("Server %s started\n", srv.Name)
	return nil
}

// StopServer tears down a WireGuard interface.
func (m *Manager) StopServer(serverID string) error {
	srv, ok := m.Server(serverID)
	if !ok {
		return serverNotFound(serverID)
	}

	status := m.serverStatus(srv.Interface)
	if status == "unknown" {
		return m.tools.RequireReady()
	}
	if status != "running" {
		return fmt.Errorf("server %s is not running: %w", serverID, ErrConflict)
	}

	if err := m.tools.QuickDown(srv.Interface); err != nil {
		fmt.Printf("Failed to stop server %s: %v\n", srv.Name, err)
		return err
	}

	m.setServerStatus(serverID, "stopped")
	m.noteServerStatus(srv.Interface, "stopped")

	fmt.Printf("Server %s stopped\n", srv.Name)
	return nil
}

// setServerStatus locks and updates the status field of a server.
func (m *Manager) setServerStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if srv := m.findServer(id); srv != nil {
		srv.Status = status
	}
}
