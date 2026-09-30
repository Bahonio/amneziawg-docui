// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Bahonio/amneziawg-docui/internal/api"
)

func TestLoadOfAMissingFileIsAnEmptyConfig(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "web_config.json"))
	cfg, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Servers == nil || len(cfg.Servers) != 0 {
		t.Errorf("Load = %+v, want an empty server list", cfg)
	}
}

// A file written before Clients and UnboundNATIPs existed leaves them nil;
// the loaded config must not, or JSON renders null and appends need guards.
func TestLoadFillsInMissingSlices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web_config.json")
	if err := os.WriteFile(path, []byte(`{"servers":[{"id":"s1"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := New(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0].Clients == nil || cfg.Servers[0].UnboundNATIPs == nil {
		t.Errorf("Load = %+v", cfg)
	}
}

func TestSaveRoundTrips(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "web_config.json"))
	cfg := &AppConfig{SchemaVersion: SchemaVersion, Servers: []api.Server{{
		ID: "s1", Clients: []api.Client{{ID: "c1", ServerID: "s1"}},
	}}}
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || len(got.Servers) != 1 || len(got.Servers[0].Clients) != 1 {
		t.Errorf("Load after Save = %+v", got)
	}
}

func TestLoadRejectsMalformedConfigWithoutReplacingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web_config.json")
	want := []byte(`{"servers":[`)
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := New(path).Load(); err == nil {
		t.Fatal("Load accepted malformed metadata")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("malformed metadata was changed: got %q, want %q", got, want)
	}
}

func TestConcurrentSavesLeaveAValidConfig(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "web_config.json"))
	cfg := &AppConfig{Servers: []api.Server{{ID: "s1", Clients: []api.Client{{ID: "c1"}}}}}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Save(cfg); err != nil {
				t.Errorf("Save: %v", err)
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	var got AppConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("config is not valid JSON after concurrent saves: %v\n%s", err, data)
	}
	if len(got.Servers) != 1 || len(got.Servers[0].Clients) != 1 {
		t.Errorf("config lost data: %+v", got)
	}
}
