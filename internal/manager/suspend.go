// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"fmt"
	"time"

	"github.com/Bahonio/awg-docui/internal/api"
	"github.com/Bahonio/awg-docui/internal/wgconf"
)

// SuspendClient removes the client peer block from the active server config.
func (m *Manager) SuspendClient(serverID, clientID string) (string, error) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	ifaceName, err := m.suspendClientLocked(serverID, clientID)
	if err != nil {
		return "", err
	}

	m.syncLiveConfig(ifaceName)
	return "client suspended", nil
}

// suspendClientLocked moves the client's peer block out of the live server
// config and marks it suspended, all under a single write lock, so the file
// and the in-memory status cannot disagree.
func (m *Manager) suspendClientLocked(serverID, clientID string) (ifaceName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv, client := m.findClient(serverID, clientID)
	if srv == nil {
		return "", serverNotFound(serverID)
	}
	if client == nil {
		return "", clientNotFound(clientID)
	}

	originalClient := cloneClient(client)
	content, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath)
	if err != nil {
		return "", err
	}
	originalContent := content
	content, _, _ = wgconf.RemovePeerText(content, client)
	if err := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, content); err != nil {
		return "", err
	}
	client.Status = "suspended"
	if err := m.saveLocked("client suspend"); err != nil {
		*client = originalClient
		if rollbackErr := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, originalContent); rollbackErr != nil {
			return "", fmt.Errorf("%w; rolling back host config: %v", err, rollbackErr)
		}
		return "", err
	}

	return srv.Interface, nil
}

// ActivateClient restores a suspended client peer block.
func (m *Manager) ActivateClient(serverID, clientID string) (string, error) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	ifaceName, err := m.activateClientLocked(serverID, clientID)
	if err != nil {
		return "", err
	}

	m.syncLiveConfig(ifaceName)
	return "client activated", nil
}

// activateClientLocked restores the client's peer block into the live server
// config and marks it active, all under a single write lock.
func (m *Manager) activateClientLocked(serverID, clientID string) (ifaceName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv, client := m.findClient(serverID, clientID)
	if srv == nil {
		return "", serverNotFound(serverID)
	}
	if client == nil {
		return "", clientNotFound(clientID)
	}
	if client.Status != "suspended" {
		return "", fmt.Errorf("client %s is not suspended: %w", clientID, ErrConflict)
	}

	originalClient := cloneClient(client)
	content, err := m.tools.ReadConfig(srv.Interface, srv.ConfigPath)
	if err != nil {
		return "", err
	}
	originalContent := content
	content = wgconf.AppendPeerText(content, client, client.ClientIP+"/32")
	if err := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, content); err != nil {
		return "", err
	}
	client.Status = "active"
	if err := m.saveLocked("client activation"); err != nil {
		*client = originalClient
		if rollbackErr := m.tools.ApplyConfig(srv.Interface, srv.ConfigPath, originalContent); rollbackErr != nil {
			return "", fmt.Errorf("%w; rolling back host config: %v", err, rollbackErr)
		}
		return "", err
	}

	return srv.Interface, nil
}

// UpdateClientSuspendTime sets or clears the scheduled auto-suspend time.
func (m *Manager) UpdateClientSuspendTime(serverID, clientID string, suspendAt *float64) (*api.Client, string, error) {
	clientCopy, err := m.updateClientLocked(serverID, clientID, func(client *api.Client) {
		client.SuspendAt = suspendAt
	})
	if err != nil {
		return nil, "", err
	}

	return clientCopy, "suspension time updated", nil
}

// pendingSuspension identifies a client that has reached its scheduled
// auto-suspend time.
type pendingSuspension struct {
	serverID, clientID string
}

// runSuspender suspends clients as their scheduled time passes. It runs for
// the life of the process.
func (m *Manager) runSuspender() {
	ticker := time.NewTicker(m.settings.SuspendCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		m.suspendDue(time.Now())
	}
}

// suspendDue is one pass of the suspender.
func (m *Manager) suspendDue(now time.Time) {
	for _, it := range m.findClientsPastSuspendTime(float64(now.Unix())) {
		if _, err := m.SuspendClient(it.serverID, it.clientID); err != nil {
			fmt.Printf("Auto-suspend failed for client %s: %v\n", it.clientID, err)
			continue
		}
		fmt.Printf("Auto-suspended client %s at %s\n", it.clientID, now.Format(time.RFC1123))
	}
}

// findClientsPastSuspendTime locks and returns all active clients whose
// scheduled suspend time has passed.
func (m *Manager) findClientsPastSuspendTime(now float64) []pendingSuspension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var items []pendingSuspension
	for _, srv := range m.cfg.Servers {
		for _, c := range srv.Clients {
			if c.SuspendAt != nil && c.Status == "active" && now >= *c.SuspendAt {
				items = append(items, pendingSuspension{srv.ID, c.ID})
			}
		}
	}
	return items
}
