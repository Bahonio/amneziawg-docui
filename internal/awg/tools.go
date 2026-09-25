// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

// Package awg is the Web backend's typed facade over the Unix-socket host
// agent. It contains no direct host command or filesystem implementation.
package awg

import (
	"fmt"
	"strings"

	"github.com/Bahonio/awg-docui/internal/agentapi"
	"github.com/Bahonio/awg-docui/internal/agentclient"
)

type PeerStats struct{ Received, Sent, LastHandshake, Endpoint string }

// Backend is deliberately expressed in VPN operations rather than command
// lines. Production uses agentBackend; tests provide an in-memory backend.
type Backend interface {
	Health() (agentapi.BackendStatus, error)
	Interfaces() ([]agentapi.Interface, error)
	InterfaceStatus(string) (bool, error)
	QuickUp(string) error
	QuickDown(string) error
	Restart(string) error
	ShowPeers(string) (map[string]PeerStats, error)
	InterfaceCounters(string) (string, string, error)
	ReadConfig(string, string) (string, error)
	CreateConfig(string, string, string, string) error
	ApplyConfig(string, string, string) error
	DeleteConfig(string, string) error
	SyncConf(string) error
	CheckFirewall(string, string) (map[string]string, error)
	RouteSourceIP() (string, error)
	GenerateKeyPair() (KeyPair, error)
	GeneratePresharedKey() (string, error)
}

type Tools struct{ backend Backend }

func New(backend Backend) *Tools { return &Tools{backend: backend} }

func NewAgent(socket string) *Tools {
	return New(agentBackend{client: agentclient.New(socket)})
}

func (t *Tools) BackendStatus() agentapi.BackendStatus {
	status, err := t.backend.Health()
	if err != nil {
		return agentapi.BackendStatus{Runtime: agentapi.Runtime, Message: "Host agent unavailable"}
	}
	return status
}

func (t *Tools) RequireReady() error {
	status, err := t.backend.Health()
	if err != nil {
		return err
	}
	if !status.Ready() {
		if status.Message != "" {
			return fmt.Errorf("%s", status.Message)
		}
		return fmt.Errorf("host backend is not ready")
	}
	return nil
}

func (t *Tools) QuickUp(iface string) error                { return t.backend.QuickUp(iface) }
func (t *Tools) QuickDown(iface string) error              { return t.backend.QuickDown(iface) }
func (t *Tools) Restart(iface string) error                { return t.backend.Restart(iface) }
func (t *Tools) SyncConf(iface string) error               { return t.backend.SyncConf(iface) }
func (t *Tools) Interfaces() ([]agentapi.Interface, error) { return t.backend.Interfaces() }

func (t *Tools) InterfaceStatus(iface string) (bool, error) {
	if iface == "" {
		return false, nil
	}
	return t.backend.InterfaceStatus(iface)
}

func (t *Tools) InterfaceUp(iface string) bool {
	up, _ := t.InterfaceStatus(iface)
	return up
}

func (t *Tools) CheckIPTables(iface, subnet string) map[string]string {
	checks, err := t.backend.CheckFirewall(iface, subnet)
	if err != nil {
		return map[string]string{"host-agent": err.Error()}
	}
	return checks
}

func (t *Tools) RouteSourceIP() (string, error) { return t.backend.RouteSourceIP() }

func (t *Tools) ShowPeers(iface string) map[string]PeerStats {
	peers, err := t.backend.ShowPeers(iface)
	if err != nil {
		return nil
	}
	return peers
}

func (t *Tools) InterfaceCounters(iface string) (rx, tx string, ok bool) {
	rx, tx, err := t.backend.InterfaceCounters(iface)
	return rx, tx, err == nil
}

func (t *Tools) ReadConfig(iface, path string) (string, error) {
	return t.backend.ReadConfig(iface, path)
}

func (t *Tools) CreateConfig(iface, path, content, subnet string) error {
	return t.backend.CreateConfig(iface, path, content, subnet)
}

func (t *Tools) ApplyConfig(iface, path, content string) error {
	return t.backend.ApplyConfig(iface, path, content)
}

func (t *Tools) DeleteConfig(iface, path string) error {
	return t.backend.DeleteConfig(iface, path)
}

// ParseShow remains a pure parser used by test support and import tools. Live
// production statistics arrive as typed data from the agent.
func ParseShow(output string) map[string]PeerStats {
	peers := map[string]PeerStats{}
	current := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "peer:"):
			current = strings.TrimSpace(strings.TrimPrefix(line, "peer:"))
			peers[current] = PeerStats{Received: "0 B", Sent: "0 B", LastHandshake: "Never"}
		case current == "":
		case strings.HasPrefix(line, "transfer:"):
			parts := strings.SplitN(strings.TrimPrefix(line, "transfer:"), ",", 2)
			if len(parts) == 2 {
				p := peers[current]
				p.Received = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[0]), " received"))
				p.Sent = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), " sent"))
				peers[current] = p
			}
		case strings.HasPrefix(line, "endpoint:"):
			p := peers[current]
			p.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
			peers[current] = p
		case strings.HasPrefix(line, "latest handshake:"):
			p := peers[current]
			p.LastHandshake = strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			peers[current] = p
		}
	}
	return peers
}

type agentBackend struct{ client *agentclient.Client }

func (b agentBackend) Health() (agentapi.BackendStatus, error)   { return b.client.Health() }
func (b agentBackend) Interfaces() ([]agentapi.Interface, error) { return b.client.Interfaces() }
func (b agentBackend) InterfaceStatus(name string) (bool, error) {
	info, err := b.client.Interface(name)
	return info.Running, err
}
func (b agentBackend) QuickUp(name string) error   { return b.client.Action(name, "start") }
func (b agentBackend) QuickDown(name string) error { return b.client.Action(name, "stop") }
func (b agentBackend) Restart(name string) error   { return b.client.Action(name, "restart") }
func (b agentBackend) ShowPeers(name string) (map[string]PeerStats, error) {
	stats, err := b.client.Stats(name)
	if err != nil {
		return nil, err
	}
	out := make(map[string]PeerStats, len(stats.Peers))
	for key, peer := range stats.Peers {
		out[key] = PeerStats{Received: peer.Received, Sent: peer.Sent, LastHandshake: peer.LastHandshake, Endpoint: peer.Endpoint}
	}
	return out, nil
}
func (b agentBackend) InterfaceCounters(name string) (string, string, error) {
	stats, err := b.client.Stats(name)
	return stats.Received, stats.Sent, err
}
func (b agentBackend) ReadConfig(name, _ string) (string, error) {
	info, err := b.client.Interface(name)
	return info.Config, err
}
func (b agentBackend) CreateConfig(name, _ string, content, subnet string) error {
	return b.client.Create(agentapi.CreateInterfaceRequest{Name: name, Config: content, Subnet: subnet})
}
func (b agentBackend) ApplyConfig(name, _ string, content string) error {
	return b.client.Apply(name, content, true)
}
func (b agentBackend) DeleteConfig(name, _ string) error { return b.client.Delete(name) }

// ApplyConfig requests a live transaction from the agent, so a second sync
// at the manager layer would apply the same change twice.
func (b agentBackend) SyncConf(string) error { return nil }
func (b agentBackend) CheckFirewall(name, subnet string) (map[string]string, error) {
	return b.client.Firewall(name, subnet)
}
func (b agentBackend) RouteSourceIP() (string, error) { return b.client.RouteSource() }
func (b agentBackend) GenerateKeyPair() (KeyPair, error) {
	keys, err := b.client.Keys()
	return KeyPair{Private: keys.Private, Public: keys.Public}, err
}
func (b agentBackend) GeneratePresharedKey() (string, error) { return b.client.PresharedKey() }
