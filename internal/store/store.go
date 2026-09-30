// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

// Package store persists web_config.json - the one file that holds every
// server and client, private keys included - and brings an older copy of it
// up to the current schema.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/Bahonio/amneziawg-docui/internal/api"
	"github.com/Bahonio/amneziawg-docui/internal/atomicfile"
)

// AppConfig is the top-level config stored on disk.
type AppConfig struct {
	// SchemaVersion records which one-shot migrations have already been
	// applied to this file, so they don't re-run on every start and undo a
	// choice the operator made afterwards. A file written before this field
	// existed unmarshals to 0.
	SchemaVersion int `json:"schema_version"`

	// Servers owns every client: a client exists exactly once, inside the
	// Clients slice of the server it belongs to. There is no second copy to
	// keep in sync.
	Servers []api.Server `json:"servers"`
}

// Store reads and writes the config file at one path.
type Store struct {
	path string

	// mu orders writes to the file: two concurrent saves cannot reorder
	// into the older one landing last, because each holds mu from the
	// moment it serialises the config until the rename is done.
	mu sync.Mutex
}

// New returns a store over path. Nothing is read until Load.
func New(path string) *Store {
	return &Store{path: path}
}

// Path is the file the store reads and writes.
func (s *Store) Path() string {
	return s.path
}

// Load reads the config, or returns an empty one only when the file does not
// exist yet. Any other read or decoding error is fatal: this file contains the
// only copy of UI-created client private keys, so treating damage as a fresh
// install would let the next save overwrite recoverable metadata.
//
// Slices a file written before they existed leaves nil are made empty, so JSON
// renders them as [] and appends need no guards.
func (s *Store) Load() (*AppConfig, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AppConfig{Servers: []api.Server{}}, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", s.path, err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decoding config %s: %w", s.path, err)
	}
	if cfg.Servers == nil {
		cfg.Servers = []api.Server{}
	}
	for i := range cfg.Servers {
		if cfg.Servers[i].Clients == nil {
			cfg.Servers[i].Clients = []api.Client{}
		}
		if cfg.Servers[i].UnboundNATIPs == nil {
			cfg.Servers[i].UnboundNATIPs = []string{}
		}
	}
	return &cfg, nil
}

// Save writes cfg to disk atomically. The caller must keep cfg from changing
// for the duration - the manager holds either its read lock for a preflight or
// its write lock for a mutation - since the snapshot is taken here, under mu,
// so that concurrent saves land in the order they serialised.
func (s *Store) Save(cfg *AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := atomicfile.Write(s.path, data, 0o600); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}
