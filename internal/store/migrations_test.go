// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package store

import (
	"testing"

	"github.com/Bahonio/amneziawg-docui/internal/api"
)

// The stamp is the part every migration shares: a config that predates the
// field is brought up to the current revision and reported as changed, and
// one that is already current is left alone.
func TestMigrateStampsAnUnversionedConfig(t *testing.T) {
	cfg := &AppConfig{Servers: []api.Server{{ID: "s1"}}}
	if !Migrate(cfg) {
		t.Error("an unversioned config must be reported as changed")
	}
	if got := cfg.SchemaVersion; got != SchemaVersion {
		t.Errorf("schema version = %d, want %d", got, SchemaVersion)
	}
}

func TestMigrateLeavesACurrentConfigAlone(t *testing.T) {
	cfg := &AppConfig{SchemaVersion: SchemaVersion}
	if Migrate(cfg) {
		t.Error("a config that needs nothing must not be reported as changed")
	}
}

// Configs written before 3.1.5 can hold a client ServerID that decayed into a
// slice of an unrelated request path, because fiber recycles the buffer a
// route parameter points into. The server a client is stored under is what
// binds them, so the migration rewrites the copy from it.
func TestMigrationRepairsClientServerIDs(t *testing.T) {
	cfg := &AppConfig{SchemaVersion: 3, Servers: []api.Server{{
		ID: "s1",
		Clients: []api.Client{
			{ID: "c1", ServerID: "tatus5"},
			{ID: "c2", ServerID: "s1"},
		},
	}, {
		ID:      "s2",
		Clients: []api.Client{{ID: "c3", ServerID: "i-sett"}},
	}}}

	Migrate(cfg)

	for _, srv := range cfg.Servers {
		for _, c := range srv.Clients {
			if c.ServerID != srv.ID {
				t.Errorf("client %s: ServerID = %q, want %q", c.ID, c.ServerID, srv.ID)
			}
		}
	}
	if got := cfg.SchemaVersion; got != SchemaVersion {
		t.Errorf("schema version = %d, want %d", got, SchemaVersion)
	}
}

func TestMigrationSeparatesLegacyAutomaticIPFromExplicitDomain(t *testing.T) {
	cfg := &AppConfig{SchemaVersion: 4, Servers: []api.Server{
		{ID: "auto", PublicIP: "198.51.100.7", Endpoint: "198.51.100.7"},
		{ID: "unknown", PublicIP: "YOUR_SERVER_IP", Endpoint: "YOUR_SERVER_IP"},
		{ID: "domain", PublicIP: "vpn.example.com", Endpoint: "vpn.example.com"},
	}}

	if !Migrate(cfg) {
		t.Fatal("Migrate reported no change")
	}
	if cfg.Servers[0].Endpoint != "" || cfg.Servers[1].Endpoint != "" {
		t.Fatalf("legacy automatic endpoints were not cleared: %+v", cfg.Servers)
	}
	if cfg.Servers[2].Endpoint != "vpn.example.com" {
		t.Fatalf("explicit domain was cleared: %+v", cfg.Servers[2])
	}
}
