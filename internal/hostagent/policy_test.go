package hostagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
)

func TestApplyRejectsNetworkChangesBeforeWriting(t *testing.T) {
	s, runner := testService(t)
	ctx := context.Background()
	if err := s.Create(ctx, agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.ConfigDir, "wg0.conf")
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	runner.running["wg0"] = true
	peer := "\n[Peer]\nPublicKey = " + testKey(4) + "\nAllowedIPs = "
	for name, content := range map[string]string{
		"dns":                string(old) + "DNS = 192.0.2.1\n",
		"table":              string(old) + "Table = 123\n",
		"fwmark":             string(old) + "FwMark = 42\n",
		"unknown":            string(old) + "FutureRoutingOption = on\n",
		"saveconfig":         string(old) + "SaveConfig = true\n",
		"address":            strings.Replace(string(old), "10.0.0.1/24", "0.0.0.1/1", 1),
		"default":            string(old) + peer + "0.0.0.0/0\n",
		"split default":      string(old) + peer + "0.0.0.0/1, 128.0.0.0/1\n",
		"ipv6 default":       string(old) + peer + "::/0\n",
		"outside subnet":     string(old) + peer + "192.0.2.1/32\n",
		"server address":     string(old) + peer + "10.0.0.1/32\n",
		"duplicate route":    string(old) + peer + "0.0.0.0/0\nAllowedIPs = 10.0.0.2/32\n",
		"hook moved to peer": string(old) + peer + "10.0.0.2/32\nPostUp = id\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := s.Apply(ctx, "wg0", content, true); err == nil {
				t.Fatal("unsafe config accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(old) {
				t.Fatalf("config changed: %v", err)
			}
			if runner.callCount("awg", "syncconf") != 0 {
				t.Fatal("unsafe config reached kernel")
			}
		})
	}
	if _, err := os.Stat(filepath.Join(s.ConfigDir, "backups")); !os.IsNotExist(err) {
		t.Fatalf("rejected writes created backups: %v", err)
	}
	if err := s.Apply(ctx, "wg0", string(old)+peer+"10.0.0.2/32\n", true); err != nil {
		t.Fatal(err)
	}
	if runner.callCount("awg", "syncconf") != 1 {
		t.Fatal("valid peer was not applied")
	}
}

func TestNetworkPolicyPreservesHostOwnedSettings(t *testing.T) {
	old := serverConfig() + "DNS = 192.0.2.1\nTable = 123\nFwMark = 42\nPostUp = echo trusted\n" +
		"\n[Peer]\nPublicKey = " + testKey(4) + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = 192.0.2.2:51820\n"
	updated := strings.Replace(old, "MTU = 1280", "MTU = 1360", 1)
	updated += "\n[Peer]\nPublicKey = " + testKey(5) + "\nAllowedIPs = 10.0.0.3/32\n"
	if err := validateNetworkPolicy(updated, old); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{
		strings.Replace(updated, "Table = 123", "Table = off", 1),
		strings.Replace(updated, "DNS = 192.0.2.1\n", "", 1),
		strings.Replace(updated, "192.0.2.2:51820", "192.0.2.3:51820", 1),
		strings.Replace(updated, testKey(4), testKey(6), 1),
	} {
		if err := validateNetworkPolicy(changed, old); err == nil {
			t.Fatal("host policy modification accepted")
		}
	}
	if err := validateNetworkPolicy(old, ""); err == nil {
		t.Fatal("new host-owned settings accepted")
	}
}

func TestCreateNormalizesFirewallSubnet(t *testing.T) {
	s, _ := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "\n10.0.0.7/24\t"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.ConfigDir, "wg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "-s 10.0.0.0/24 -j MASQUERADE") != 2 {
		t.Fatalf("bad hooks: %s", data)
	}
}

func TestPeerMutationEndpointsRemoved(t *testing.T) {
	s, runner := testService(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		res := httptest.NewRecorder()
		Handler{Service: s}.ServeHTTP(res, httptest.NewRequest(method, "/v1/interfaces/wg0/peers", strings.NewReader(`{}`)))
		if res.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", method, res.Code)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatal("removed endpoint executed host commands")
	}
}

func TestSnapshotReusesUnitDiscoveryButActionsResolveAgain(t *testing.T) {
	s, runner := testService(t)
	ctx := context.Background()
	if err := s.Create(ctx, agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	runner.running["wg0"] = true
	runner.active["awg-quick@wg0.service"] = true
	first, err := s.Snapshot(ctx)
	if err != nil || len(first) != 1 || first[0].Config == "" || !first[0].Running {
		t.Fatalf("snapshot: %+v %v", first, err)
	}
	before := runner.callCount("systemctl", "is-active")
	if _, err := s.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.callCount("systemctl", "is-active") != before {
		t.Fatal("snapshot repeated unit discovery")
	}
	runner.active["awg-quick@wg0.service"] = false
	runner.active["amneziawg@wg0.service"] = true
	if err := s.Action(ctx, "wg0", "restart"); err != nil {
		t.Fatal(err)
	}
	if !runner.called("systemctl", "restart", "amneziawg@wg0.service") {
		t.Fatal("action used stale unit")
	}
	s.unitMu.Lock()
	s.units["wg0"] = unitObservation{unit: "awg-quick@wg0.service", at: time.Now().Add(-time.Minute)}
	s.unitMu.Unlock()
	if _, err := s.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if !runner.called("systemctl", "is-enabled", "--quiet", "amneziawg@wg0.service") {
		t.Fatal("unit cache did not expire")
	}
}

func TestConfigReadAndStateAvoidUnitDiscovery(t *testing.T) {
	s, runner := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/config", "/state"} {
		res := httptest.NewRecorder()
		Handler{Service: s}.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/interfaces/wg0"+path, nil))
		if res.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, res.Code)
		}
	}
	if runner.callCount("systemctl", "is-active") != 0 || runner.callCount("systemctl", "is-enabled") != 0 {
		t.Fatal("simple read launched systemctl")
	}
}
