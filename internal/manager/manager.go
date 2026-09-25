// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

// Package manager holds the backend's state - the servers, their clients and
// what the kernel currently says about them - and every operation on it. It
// is the only package that takes the config lock, and it never shells out
// itself: the host is reached through awg.Tools, the disk through store and
// wgconf.
package manager

import (
	"fmt"
	"sync"

	"github.com/Bahonio/awg-docui/internal/awg"
	"github.com/Bahonio/awg-docui/internal/config"
	"github.com/Bahonio/awg-docui/internal/publicip"
	"github.com/Bahonio/awg-docui/internal/store"
)

// Manager orchestrates all AmneziaWG operations.
type Manager struct {
	settings config.Settings
	store    *store.Store
	tools    *awg.Tools

	// cfg is the live config. Every read and write goes through mu; nothing
	// handed out of this package points into it.
	cfg *store.AppConfig

	// publicIP is re-detected on demand from an HTTP handler while other
	// requests are generating configs from it, so it lives behind mu rather
	// than as a bare field.
	publicIP string

	mu sync.RWMutex

	// hostMu serialises operations that read or rewrite host configs across
	// their matching metadata save. Without it, an adoption poll could read a
	// config, a client mutation could commit, and the stale adoption snapshot
	// could then mark that just-added client absent.
	hostMu sync.Mutex

	// statuses caches observed interface states, keyed by interface name;
	// see serverStatus. Guarded by statusMu, not mu, so a status lookup
	// never waits on a config write.
	statuses map[string]statusObservation
	statusMu sync.Mutex
}

// New loads the config from st and brings it up to the current schema. It
// refuses to start on unreadable or malformed metadata because continuing as
// a fresh install could overwrite the only copy of client private keys. It
// does not touch the host: see Start for that.
func New(settings config.Settings, st *store.Store, tools *awg.Tools) (*Manager, error) {
	m := &Manager{
		settings: settings,
		store:    st,
		tools:    tools,
		statuses: map[string]statusObservation{},
	}

	var err error
	m.cfg, err = st.Load()
	if err != nil {
		return nil, err
	}
	if store.Migrate(m.cfg) {
		if err := m.store.Save(m.cfg); err != nil {
			return nil, fmt.Errorf("persisting schema migration: %w", err)
		}
	}
	return m, nil
}

// Start does what a booting backend does once the config is in memory:
// detects the public address, adopts host configs and launches the
// scheduled-suspension checker. It returns once the servers are up; the
// checker keeps running in the background for the life of the process.
func (m *Manager) Start() error {
	m.setPublicIP(publicip.Detect(m.tools.RouteSourceIP))

	// Interface boot policy belongs to host systemd. A Web UI restart must
	// never start, stop, restart, or clean up the data plane.
	if err := m.adoptExisting(); err != nil {
		return err
	}

	go m.runSuspender()

	fmt.Printf("=== Environment Configuration ===\n")
	fmt.Printf("WEB_UI_PORT: %d\n", m.settings.WebUIPort)
	fmt.Printf("AUTO_START: %v\n", m.settings.AutoStart)
	fmt.Printf("DEFAULT_MTU: %d\n", m.settings.DefaultMTU)
	fmt.Printf("DEFAULT_SUBNET: %s\n", m.settings.DefaultSubnet)
	fmt.Printf("DEFAULT_PORT: %d\n", m.settings.DefaultPort)
	fmt.Printf("DNS_SERVERS: %v\n", m.settings.DNSServers)
	fmt.Printf("Detected public IP: %s\n", m.PublicIP())
	return nil
}

// Settings are the defaults the manager was started with.
func (m *Manager) Settings() config.Settings {
	return m.settings
}

// SaveConfig writes the config to disk. It takes the read lock itself, so it
// must not be called with mu held.
func (m *Manager) SaveConfig() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.Save(m.cfg)
}

// saveLocked persists cfg while the caller holds m.mu for writing. Keeping the
// mutation and its save under one lock prevents another request from being
// accidentally reverted when a failed save is rolled back in memory.
func (m *Manager) saveLocked(what string) error {
	if err := m.store.Save(m.cfg); err != nil {
		return fmt.Errorf("persisting %s: %w", what, err)
	}
	return nil
}
