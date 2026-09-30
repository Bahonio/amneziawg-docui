package hostagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
)

type call struct {
	name  string
	args  []string
	stdin string
}

type fakeRunner struct {
	mu      sync.Mutex
	calls   []call
	running map[string]bool
	active  map[string]bool
	enabled map[string]bool
	fail    map[string]error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{running: map[string]bool{}, active: map[string]bool{}, enabled: map[string]bool{}, fail: map[string]error{}}
}

func (f *fakeRunner) Run(_ context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{name: name, args: slices.Clone(args), stdin: string(stdin)})
	key := name + " " + strings.Join(args, " ")
	if err := f.fail[key]; err != nil {
		return nil, err
	}
	if name == "ip" && len(args) >= 5 && args[0] == "-4" && args[1] == "route" {
		return []byte("1.1.1.1 via 192.0.2.1 dev eth0 src 192.0.2.10\n"), nil
	}
	if name == "ip" && len(args) == 4 && args[0] == "link" && args[1] == "show" {
		if f.running[args[3]] {
			return []byte("up"), nil
		}
		return nil, errors.New("not found")
	}
	if name == "systemctl" && len(args) == 3 && args[0] == "is-active" {
		if f.active[args[2]] {
			return nil, nil
		}
		return nil, errors.New("inactive")
	}
	if name == "systemctl" && len(args) == 3 && args[0] == "is-enabled" {
		if f.enabled[args[2]] {
			return nil, nil
		}
		return nil, errors.New("disabled")
	}
	if name == "systemctl" {
		return nil, nil
	}
	if name == "awg" && len(args) == 1 && args[0] == "genkey" {
		return []byte(testKey(1) + "\n"), nil
	}
	if name == "awg" && len(args) == 1 && args[0] == "genpsk" {
		return []byte(testKey(2) + "\n"), nil
	}
	if name == "awg" && len(args) == 1 && args[0] == "pubkey" {
		return []byte(testKey(3) + "\n"), nil
	}
	if name == "awg" && len(args) == 2 && args[0] == "show" && args[1] == "interfaces" {
		return nil, nil
	}
	if name == "awg" && len(args) == 3 && args[0] == "show" && args[2] == "public-key" {
		return []byte(testKey(3) + "\n"), nil
	}
	if name == "awg" && len(args) == 3 && args[0] == "show" && args[2] == "dump" {
		return []byte("private\tpublic\t54844\toff\n" + testKey(4) + "\tpsk\t203.0.113.2:9\t10.0.0.2/32\t1700000000\t1024\t2048\t25\n"), nil
	}
	if name == "awg-quick" && len(args) == 2 && args[0] == "strip" {
		return os.ReadFile(args[1])
	}
	if name == "awg" && len(args) == 3 && args[0] == "syncconf" {
		return nil, nil
	}
	if name == "iptables" {
		return nil, nil
	}
	return nil, nil
}

func testKey(fill byte) string { return base64.StdEncoding.EncodeToString(bytesOf(fill, 32)) }

func bytesOf(fill byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return b
}

func testService(t *testing.T) (*Service, *fakeRunner) {
	t.Helper()
	dir := t.TempDir()
	module := filepath.Join(dir, "module")
	if err := os.Mkdir(module, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := newFakeRunner()
	s := NewService()
	s.ConfigDir, s.ModuleDir, s.Runner = filepath.Join(dir, "configs"), module, runner
	s.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	s.ProbeUDP = func(int) error { return nil }
	return s, runner
}

func serverConfig() string {
	return "[Interface]\nPrivateKey = " + testKey(1) + "\nAddress = 10.0.0.1/24\nListenPort = 54844\nMTU = 1280\n"
}

func TestValidationRejectsTraversalAndCommandHooks(t *testing.T) {
	for _, name := range []string{"../wg0", "/tmp/x", "wg0;id", "wg0/x", strings.Repeat("a", 16), ""} {
		if validateInterfaceName(name) == nil {
			t.Errorf("accepted interface name %q", name)
		}
	}
	if err := validateInterfaceName("wg0"); err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{"PreUp", "PostUp", "PreDown", "PostDown"} {
		if err := validateConfig(serverConfig() + directive + " = id\n"); err == nil {
			t.Errorf("accepted privileged %s hook", directive)
		}
	}
	for _, invalid := range []string{
		strings.Replace(serverConfig(), testKey(1), "not-a-key", 1),
		strings.Replace(serverConfig(), "10.0.0.1/24", "10.0.0.1/24;id", 1),
		strings.Replace(serverConfig(), "MTU = 1280", "MTU = huge", 1),
		serverConfig() + "\n[Peer]\nPublicKey = bad\nAllowedIPs = ../etc/passwd\n",
	} {
		if err := validateConfig(invalid); err == nil {
			t.Errorf("accepted malformed config:\n%s", invalid)
		}
	}
}

func TestConfigSymlinkCannotEscapeConfigDirectory(t *testing.T) {
	s, _ := testService(t)
	if err := os.MkdirAll(s.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.conf")
	if err := os.WriteFile(outside, []byte(serverConfig()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(s.ConfigDir, "wg0.conf")); err != nil {
		t.Fatal(err)
	}
	if detail, err := s.Interface(context.Background(), "wg0", true); err == nil || detail.Config != "" {
		t.Fatalf("symlink content was exposed: detail=%+v err=%v", detail, err)
	}
	if err := s.Apply(context.Background(), "wg0", serverConfig(), false); err == nil {
		t.Fatal("config symlink was accepted for update")
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != serverConfig() {
		t.Fatalf("outside file changed: %q, %v", data, err)
	}
}

func TestCreateApplyAndDeleteConfigAreAtomicAndBackedUp(t *testing.T) {
	s, runner := testService(t)
	req := agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}
	if err := s.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.ConfigDir, "wg0.conf")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode: %v, %v", info, err)
	}
	created, _ := os.ReadFile(path)
	if !strings.Contains(string(created), "PostUp = iptables") {
		t.Fatal("managed firewall hooks were not added")
	}
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg1", Config: serverConfig(), Subnet: "10.0.1.0/24"}); err == nil || !strings.Contains(err.Error(), "Port already in use") {
		t.Fatalf("duplicate listen port error = %v", err)
	}

	runner.running["wg0"] = true
	updated := strings.Replace(string(created), "MTU = 1280", "MTU = 1360", 1)
	if err := s.Apply(context.Background(), "wg0", updated, true); err != nil {
		t.Fatal(err)
	}
	backups, err := os.ReadDir(filepath.Join(s.ConfigDir, "backups"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	if !runner.called("awg", "syncconf", "wg0", "/dev/stdin") {
		t.Fatal("live sync was not called")
	}
	if runner.called("systemctl", "restart") {
		t.Fatal("peer/config update restarted the interface")
	}

	if err := s.Delete(context.Background(), "wg0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config was not deleted: %v", err)
	}
}

func TestCreateRejectsPortAlreadyUsedByHostConfig(t *testing.T) {
	s, _ := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	second := strings.Replace(serverConfig(), "10.0.0.1/24", "10.1.0.1/24", 1)
	err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg1", Config: second, Subnet: "10.1.0.0/24"})
	if err == nil || !strings.Contains(err.Error(), "Port already in use by wg0") {
		t.Fatalf("port conflict error = %v", err)
	}
}

func TestPeerCRUDUsesLiveSyncWithoutRestart(t *testing.T) {
	s, runner := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	runner.running["wg0"] = true
	peer := agentapi.Peer{PublicKey: testKey(4), PresharedKey: testKey(5), AllowedIPs: "10.0.0.2/32"}
	if err := s.AddPeer(context.Background(), "wg0", peer); err != nil {
		t.Fatal(err)
	}
	updated := agentapi.UpdatePeerRequest{OriginalPublicKey: peer.PublicKey, Peer: agentapi.Peer{PublicKey: testKey(6), AllowedIPs: "10.0.0.3/32"}}
	if err := s.UpdatePeer(context.Background(), "wg0", updated); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePeer(context.Background(), "wg0", updated.PublicKey); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(s.ConfigDir, "wg0.conf"))
	if strings.Contains(string(content), testKey(4)) || strings.Contains(string(content), testKey(6)) {
		t.Fatal("deleted peer remains in config")
	}
	if runner.callCount("awg", "syncconf") != 3 {
		t.Fatalf("syncconf calls = %d", runner.callCount("awg", "syncconf"))
	}
	if runner.called("systemctl", "restart") || runner.called("systemctl", "stop") {
		t.Fatal("peer CRUD changed interface lifecycle")
	}
}

func TestDeleteUsesExistingVendorUnitForAdoptedInterface(t *testing.T) {
	s, runner := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	runner.running["wg0"] = true
	runner.active["awg-quick@wg0.service"] = true
	if err := s.Delete(context.Background(), "wg0"); err != nil {
		t.Fatal(err)
	}
	if !runner.called("systemctl", "disable", "--now", "awg-quick@wg0.service") {
		t.Fatalf("existing vendor unit was not stopped: %+v", runner.calls)
	}
}

func TestStatusReportsMissingKernelAndToolsPrecisely(t *testing.T) {
	s := NewService()
	s.ConfigDir, s.ModuleDir = t.TempDir(), filepath.Join(t.TempDir(), "missing")
	s.Runner = newFakeRunner()
	s.LookPath = func(name string) (string, error) {
		if name == "modinfo" {
			return "", errors.New("missing")
		}
		return "", errors.New("missing")
	}
	status := s.Health()
	if status.KernelModuleInstalled || status.AWGAvailable || status.AWGQuickAvailable || status.Runtime != "Kernel" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Message != "AmneziaWG kernel module is not available on the host" {
		t.Fatalf("message = %q", status.Message)
	}

	if err := os.Mkdir(s.ModuleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	status = s.Health()
	if status.Message != "awg-tools not installed: awg and awg-quick are missing" {
		t.Fatalf("message = %q", status.Message)
	}

	tests := []struct {
		name, missing, message string
	}{
		{"missing awg", "awg", "awg-tools not installed: awg is missing"},
		{"missing awg-quick", "awg-quick", "awg-tools not installed: awg-quick is missing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s.LookPath = func(name string) (string, error) {
				if name == tc.missing {
					return "", errors.New("missing")
				}
				return "/usr/bin/" + name, nil
			}
			if got := s.Health().Message; got != tc.message {
				t.Fatalf("message = %q", got)
			}
		})
	}

	os.Remove(s.ModuleDir)
	s.LookPath = func(name string) (string, error) {
		if name == "modinfo" || name == "awg" || name == "awg-quick" {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("missing")
	}
	status = s.Health()
	if !status.KernelModuleInstalled || status.KernelModuleLoaded || status.Message != "AmneziaWG kernel module not loaded" {
		t.Fatalf("installed-but-unloaded status = %+v", status)
	}
}

func TestMalformedRequestsAndTraversalReturn400(t *testing.T) {
	s, _ := testService(t)
	h := Handler{Service: s}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/interfaces", "{not-json"},
		{http.MethodPost, "/v1/interfaces", `{"name":"wg0","config":"[Interface]\\nPrivateKey=x","subnet":"10.0.0.0/24","extra":true}`},
		{http.MethodPut, "/v1/interfaces/wg0%3Bid/config", `{"config":"x"}`},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, body %s", tc.path, res.Code, res.Body.String())
		}
	}
}

func TestHostAgentAPIHappyPathUsesTypedOperations(t *testing.T) {
	s, runner := testService(t)
	h := Handler{Service: s}
	request := func(method, path string, payload any) *httptest.ResponseRecorder {
		t.Helper()
		var body io.Reader
		if payload != nil {
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			body = strings.NewReader(string(data))
		}
		req := httptest.NewRequest(method, path, body)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res
	}

	create := agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}
	if res := request(http.MethodPost, "/v1/interfaces", create); res.Code != http.StatusOK {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodGet, "/v1/interfaces", nil); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"wg0"`) {
		t.Fatalf("list: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodGet, "/v1/interfaces/wg0", nil); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "ListenPort = 54844") {
		t.Fatalf("get: %d %s", res.Code, res.Body.String())
	}

	runner.running["wg0"] = true
	peer := agentapi.Peer{PublicKey: testKey(8), PresharedKey: testKey(9), AllowedIPs: "10.0.0.8/32"}
	if res := request(http.MethodPost, "/v1/interfaces/wg0/peers", peer); res.Code != http.StatusOK {
		t.Fatalf("add peer: %d %s", res.Code, res.Body.String())
	}
	if !runner.called("awg", "syncconf", "wg0", "/dev/stdin") {
		t.Fatal("peer API did not live-sync the running interface")
	}
	if res := request(http.MethodDelete, "/v1/interfaces/wg0", nil); res.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", res.Code, res.Body.String())
	}
}

func TestInterfaceStatsAreReadFromAwgDump(t *testing.T) {
	s, runner := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	runner.running["wg0"] = true
	stats, err := s.Stats(context.Background(), "wg0")
	if err != nil {
		t.Fatal(err)
	}
	peer := stats.Peers[testKey(4)]
	if peer.Received != "1.00 KiB" || peer.Sent != "2.00 KiB" || peer.LastHandshake == "Never" {
		t.Fatalf("peer stats = %+v", peer)
	}
}

func TestNoCommandIsExecutedThroughAShell(t *testing.T) {
	s, runner := testService(t)
	if err := s.Create(context.Background(), agentapi.CreateInterfaceRequest{Name: "wg0", Config: serverConfig(), Subnet: "10.0.0.0/24", Start: true}); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if call.name == "sh" || call.name == "bash" {
			t.Fatalf("shell command executed: %+v", call)
		}
	}
}

func (f *fakeRunner) called(name string, args ...string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if call.name != name || len(call.args) < len(args) {
			continue
		}
		if slices.Equal(call.args[:len(args)], args) {
			return true
		}
	}
	return false
}

func (f *fakeRunner) callCount(name, firstArg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, call := range f.calls {
		if call.name == name && len(call.args) > 0 && call.args[0] == firstArg {
			n++
		}
	}
	return n
}

func TestHandlerNeverAcceptsACommandOperation(t *testing.T) {
	s, _ := testService(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/command", strings.NewReader(`{"command":"id"}`))
	res := httptest.NewRecorder()
	Handler{Service: s}.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.Code, body)
	}
}
