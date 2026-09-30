// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Bahonio/amneziawg-docui/internal/api"
	"github.com/Bahonio/amneziawg-docui/internal/publicip"
)

// TestCreateServerRejectsAPortAnotherServerAlreadyUses covers the check that
// runs before anything is written: two interfaces on one UDP port would only
// fail much later, when the second one cannot bind.
func TestCreateServerRejectsAPortAnotherServerAlreadyUses(t *testing.T) {
	m, _ := newTestManager(t) // its one server, "srv", listens on 54844

	_, err := m.CreateServer(api.CreateServerRequest{Name: "second", Port: 54844, MTU: 1420})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if !strings.Contains(err.Error(), `"srv"`) {
		t.Fatalf("error should name the server holding the port, got %v", err)
	}
	if got := len(m.cfg.Servers); got != 1 {
		t.Fatalf("nothing should have been created, got %d servers", got)
	}
}

// The panel's own port counts as taken too: publishing one number for both
// the web UI and a tunnel is a trap even though TCP and UDP would coexist.
func TestCreateServerRejectsTheWebUIsOwnPort(t *testing.T) {
	m, _ := newTestManager(t)

	_, err := m.CreateServer(api.CreateServerRequest{Name: "second", Port: 54845, MTU: 1420})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if !strings.Contains(err.Error(), "web UI") {
		t.Fatalf("error should say what holds the port, got %v", err)
	}
}

func TestPortInUse(t *testing.T) {
	m, _ := newTestManager(t)

	if holder, ok := m.portInUse(54844); !ok || holder != `server "srv"` {
		t.Errorf(`portInUse(54844) = %q, %v; want server "srv", true`, holder, ok)
	}
	if holder, ok := m.portInUse(54845); !ok || holder != "the web UI" {
		t.Errorf(`portInUse(54845) = %q, %v; want the web UI, true`, holder, ok)
	}
	if holder, ok := m.portInUse(54846); ok {
		t.Errorf("portInUse(54846) = %q, %v; want the port to be free", holder, ok)
	}
}

// A created server gets a .conf whose [Interface] carries the same
// obfuscation lines its clients will, and is brought up when asked.
func TestCreateServerWritesTheConfAndAutoStarts(t *testing.T) {
	m, run := newTestManager(t)
	run.Stub("awg genkey", "SPRIV").Stub("echo 'SPRIV' | awg pubkey", "SPUB")
	run.Stub("/usr/bin/awg-quick up wg-", "")
	yes := true

	srv, err := m.CreateServer(api.CreateServerRequest{Name: "second", Port: 54846, Subnet: "10.0.2.0/24", AutoStart: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if srv.ServerPublicKey != "SPUB" || srv.ServerIP != "10.0.2.1" || !srv.ObfuscationEnabled {
		t.Errorf("server = %+v", srv)
	}

	conf, err := os.ReadFile(srv.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"PrivateKey = SPRIV", "Address = 10.0.2.1/24", "ListenPort = 54846", "HeaderProtectionKey = "} {
		if !strings.Contains(string(conf), line) {
			t.Errorf("conf missing %q:\n%s", line, conf)
		}
	}

	if !run.Ran("/usr/bin/awg-quick up " + srv.Interface) {
		t.Errorf("the server was not brought up: %v", run.Commands)
	}
	if got := m.ServerStatus(srv.ID); got != "running" {
		t.Errorf("status after auto-start = %q", got)
	}
}

func TestCreateServerKeepsAutomaticIPSeparateFromEndpointOverride(t *testing.T) {
	m, _ := newTestManager(t)
	m.setPublicIP("198.51.100.7")
	no := false

	srv, err := m.CreateServer(api.CreateServerRequest{
		Name: "automatic", Port: 54846, Subnet: "10.0.2.0/24", AutoStart: &no,
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.PublicIP != "198.51.100.7" || srv.Endpoint != "" {
		t.Fatalf("automatic endpoint fields = public %q, override %q", srv.PublicIP, srv.Endpoint)
	}
}

func TestUpdateServerEndpointChangesFutureExportsWithoutTouchingHost(t *testing.T) {
	m, _ := newTestManager(t)
	m.cfg.Servers[0].PublicIP = "198.51.100.7"
	client, _, err := m.AddClient("s1", api.AddClientRequest{Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(m.cfg.Servers[0].ConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := m.UpdateServerEndpoint("s1", "vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Endpoint != "vpn.example.com" {
		t.Fatalf("updated endpoint = %q", updated.Endpoint)
	}
	conf, err := m.ClientConfig("s1", client.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(conf, "Endpoint = vpn.example.com:54844") {
		t.Fatalf("domain missing from client export:\n%s", conf)
	}

	if _, err := m.UpdateServerEndpoint("s1", ""); err != nil {
		t.Fatal(err)
	}
	conf, err = m.ClientConfig("s1", client.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(conf, "Endpoint = 198.51.100.7:54844") {
		t.Fatalf("cleared override did not restore automatic IP:\n%s", conf)
	}
	after, err := os.ReadFile(m.cfg.Servers[0].ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("endpoint metadata edit changed the host AWG config")
	}
}

func TestUpdateServerEndpointValidatesAndRollsBack(t *testing.T) {
	m, _ := newTestManager(t)
	for _, invalid := range []string{"vpn.example.com:54844", "vpn..example.com", "2001:db8::1", "999.999.999.999", "-vpn.example.com", strings.Repeat("a", 64) + ".example"} {
		if _, err := m.UpdateServerEndpoint("s1", invalid); err == nil {
			t.Errorf("accepted invalid endpoint %q", invalid)
		}
	}
	if _, err := m.UpdateServerEndpoint("s1", "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	failFutureSaves(t, m)
	if _, err := m.UpdateServerEndpoint("s1", "new.example.com"); err == nil {
		t.Fatal("endpoint update reported success although metadata could not be saved")
	}
	srv, _ := m.Server("s1")
	if srv.Endpoint != "vpn.example.com" {
		t.Fatalf("failed endpoint edit was not rolled back: %q", srv.Endpoint)
	}
}

func TestRefreshPublicIPAffectsAutomaticButNotExplicitEndpoint(t *testing.T) {
	m, run := newTestManager(t)
	client, _, err := m.AddClient("s1", api.AddClientRequest{Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	originalServices := publicip.Services
	publicip.Services = nil
	t.Cleanup(func() { publicip.Services = originalServices })

	run.Stub("ip route get 1", "198.51.100.8")
	if address, err := m.RefreshPublicIP(); err != nil || address != "198.51.100.8" {
		t.Fatalf("RefreshPublicIP = %q, %v", address, err)
	}
	conf, _ := m.ClientConfig("s1", client.ID, false)
	if !strings.Contains(conf, "Endpoint = 198.51.100.8:54844") {
		t.Fatalf("automatic endpoint did not follow refreshed IP:\n%s", conf)
	}

	if _, err := m.UpdateServerEndpoint("s1", "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	run.Stub("ip route get 1", "203.0.113.9")
	if _, err := m.RefreshPublicIP(); err != nil {
		t.Fatal(err)
	}
	conf, _ = m.ClientConfig("s1", client.ID, false)
	if !strings.Contains(conf, "Endpoint = vpn.example.com:54844") {
		t.Fatalf("explicit domain was overwritten by refreshed IP:\n%s", conf)
	}
}

func TestCreateServerRemovesHostConfigWhenMetadataSaveFails(t *testing.T) {
	m, _ := newTestManager(t)
	failFutureSaves(t, m)
	no := false

	if _, err := m.CreateServer(api.CreateServerRequest{
		Name: "unsaved", Port: 54846, Subnet: "10.0.2.0/24", AutoStart: &no,
	}); err == nil {
		t.Fatal("CreateServer reported success although metadata could not be saved")
	}
	if got := len(m.Servers()); got != 1 {
		t.Fatalf("unsaved server remained in memory: count = %d", got)
	}
	entries, err := os.ReadDir(m.settings.WireguardConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	configs := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			configs++
		}
	}
	if configs != 1 {
		t.Fatalf("host config cleanup left %d .conf files, want 1", configs)
	}
}

func TestDeleteServerStopsARunningInterfaceFirst(t *testing.T) {
	m, run := newTestManager(t)
	run.Stub("ip link show wg-test-absent", "wg-test-absent: state UNKNOWN")
	run.Stub("/usr/bin/awg-quick down wg-test-absent", "")
	confPath := m.cfg.Servers[0].ConfigPath

	if err := m.DeleteServer("s1"); err != nil {
		t.Fatal(err)
	}
	if !run.Ran("/usr/bin/awg-quick down wg-test-absent") {
		t.Errorf("the interface was left up: %v", run.Commands)
	}
	if _, err := os.Stat(confPath); !os.IsNotExist(err) {
		t.Errorf("the .conf was left behind (err %v)", err)
	}
	if _, ok := m.Server("s1"); ok {
		t.Error("the server is still listed")
	}
}

func TestDeleteServerDoesNotTouchHostWhenMetadataIsUnwritable(t *testing.T) {
	m, _ := newTestManager(t)
	confPath := m.cfg.Servers[0].ConfigPath
	failFutureSaves(t, m)

	if err := m.DeleteServer("s1"); err == nil {
		t.Fatal("DeleteServer reported success although metadata was unwritable")
	}
	if _, err := os.Stat(confPath); err != nil {
		t.Fatalf("host config was touched before persistence preflight succeeded: %v", err)
	}
	if _, ok := m.Server("s1"); !ok {
		t.Fatal("server disappeared from memory after failed persistence preflight")
	}
}
