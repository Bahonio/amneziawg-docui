// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"github.com/Bahonio/awg-docui/internal/api"
	"github.com/Bahonio/awg-docui/internal/publicip"
)

// PublicIP returns the address every generated config points clients at.
func (m *Manager) PublicIP() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publicIP
}

func (m *Manager) setPublicIP(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publicIP = ip
}

// RefreshPublicIP re-detects the public IP and stamps it on every server.
func (m *Manager) RefreshPublicIP() (string, error) {
	ip := publicip.Detect(m.tools.RouteSourceIP)

	m.mu.Lock()
	originalPublicIP := m.publicIP
	originalServerIPs := make([]string, len(m.cfg.Servers))
	m.publicIP = ip
	for i := range m.cfg.Servers {
		originalServerIPs[i] = m.cfg.Servers[i].PublicIP
		m.cfg.Servers[i].PublicIP = ip
	}
	if err := m.saveLocked("public IP"); err != nil {
		m.publicIP = originalPublicIP
		for i := range m.cfg.Servers {
			m.cfg.Servers[i].PublicIP = originalServerIPs[i]
		}
		m.mu.Unlock()
		return "", err
	}
	m.mu.Unlock()

	return ip, nil
}

// endpointFor is the host a client of srv connects to: the endpoint the
// server was created with, else the public IP stamped on it, else whatever
// the backend detected last.
func (m *Manager) endpointFor(srv *api.Server) string {
	if srv.Endpoint != "" {
		return srv.Endpoint
	}
	if srv.PublicIP != "" {
		return srv.PublicIP
	}
	return m.PublicIP()
}
