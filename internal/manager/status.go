// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"fmt"
	"time"
)

// statusTTL is how long an observed interface state is reused. Long enough to
// collapse the burst of lookups one dashboard refresh makes, short enough that
// an interface going down is noticed within a poll or two.
const statusTTL = 2 * time.Second

type statusObservation struct {
	status string
	at     time.Time
}

// ServerStatus checks the real interface state of a server.
func (m *Manager) ServerStatus(serverID string) string {
	srv, ok := m.Server(serverID)
	if !ok {
		return "not_found"
	}
	return m.serverStatus(srv.Interface)
}

// serverStatus reports whether the server's interface is up, from a short
// lived cache. The kernel is the only authority on this - the Status field in
// the config is a stale echo of the last observation - but every dashboard
// poll asks about every server, and the traffic poll asks again every few
// seconds, so the answer is reused for statusTTL rather than shelling out per
// caller. It shells out on a miss, so it must never be called with mu held.
func (m *Manager) serverStatus(iface string) string {
	if iface == "" {
		return "stopped"
	}
	if status, ok := m.cachedStatus(iface); ok {
		return status
	}

	status := "stopped"
	up, err := m.tools.InterfaceStatus(iface)
	if err != nil {
		status = "unknown"
	} else if up {
		status = "running"
	}
	m.noteServerStatus(iface, status)
	return status
}

func (m *Manager) cachedStatus(iface string) (string, bool) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	seen, ok := m.statuses[iface]
	if !ok || time.Since(seen.at) > statusTTL {
		return "", false
	}
	return seen.status, true
}

// noteServerStatus records a status this code just caused, so the next reader
// does not have to wait out the cache or race the kernel.
func (m *Manager) noteServerStatus(iface, status string) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	m.statuses[iface] = statusObservation{status: status, at: time.Now()}
}

// forgetServerStatus drops a cached observation, for an interface that no
// longer exists.
func (m *Manager) forgetServerStatus(iface string) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	delete(m.statuses, iface)
}

// syncLiveConfig pushes the .conf onto the interface when it is actually up.
// The check is deliberately made here, after the config change and outside
// the lock, rather than from the stored Status: a peer added while the stored
// value says "stopped" but the interface is up would silently not take effect
// until the next restart.
func (m *Manager) syncLiveConfig(iface string) {
	if m.serverStatus(iface) != "running" {
		return
	}
	if err := m.tools.SyncConf(iface); err != nil {
		fmt.Printf("Failed to apply live config to %s: %v\n", iface, err)
		return
	}
	fmt.Printf("Live config applied to %s\n", iface)
}
