package hostagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
	"github.com/Bahonio/amneziawg-docui/internal/atomicfile"
)

const defaultUnitPattern = "awg-docui-vpn@%s.service"

type unitObservation struct {
	unit string
	at   time.Time
}

type Service struct {
	unitMu     sync.Mutex
	units      map[string]unitObservation
	ConfigDir  string
	ModuleDir  string
	SocketPath string
	Unit       string
	AWG        string
	AWGQuick   string
	IP         string
	Systemctl  string
	Runner     CommandRunner
	LookPath   func(string) (string, error)
	ProbeUDP   func(int) error
}

func NewService() *Service {
	return &Service{
		ConfigDir: "/etc/amnezia/amneziawg", ModuleDir: "/sys/module/amneziawg",
		SocketPath: agentapi.SocketPath, Unit: defaultUnitPattern,
		AWG: "awg", AWGQuick: "awg-quick", IP: "ip", Systemctl: "systemctl",
		Runner: ExecRunner{}, LookPath: exec.LookPath,
		ProbeUDP: func(port int) error {
			listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: port})
			if err != nil {
				return err
			}
			return listener.Close()
		},
	}
}

func (s *Service) configPath(name string) (string, error) {
	if err := validateInterfaceName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.ConfigDir, name+".conf"), nil
}

func (s *Service) selectedUnit(name string) string { return fmt.Sprintf(s.Unit, name) }

// managedUnit preserves the template already responsible for an adopted
// interface. This avoids enabling a second unit or stopping the wrong one
// when an existing host installation uses a vendor template.
func (s *Service) managedUnit(ctx context.Context, name string) string {
	candidates := []string{
		s.selectedUnit(name),
		fmt.Sprintf("awg-quick@%s.service", name),
		fmt.Sprintf("amneziawg@%s.service", name),
		fmt.Sprintf("amneziawg-ui-vpn@%s.service", name),
	}
	seen := map[string]bool{}
	for _, state := range []string{"is-active", "is-enabled"} {
		for _, candidate := range candidates {
			if seen[state+candidate] {
				continue
			}
			seen[state+candidate] = true
			if _, err := s.command(ctx, nil, s.Systemctl, state, "--quiet", candidate); err == nil {
				return candidate
			}
		}
	}
	return s.selectedUnit(name)
}

func (s *Service) command(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return s.Runner.Run(ctx, stdin, name, args...)
}

func (s *Service) Health() agentapi.BackendStatus {
	status := agentapi.BackendStatus{HostAgent: true, Runtime: agentapi.Runtime}
	_, status.AWGAvailable = s.lookup(s.AWG)
	_, status.AWGQuickAvailable = s.lookup(s.AWGQuick)
	if _, err := os.Stat(s.ModuleDir); err == nil {
		status.KernelModuleInstalled = true
		status.KernelModuleLoaded = true
	} else if _, ok := s.lookup("modinfo"); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := s.Runner.Run(ctx, nil, "modinfo", "amneziawg")
		cancel()
		status.KernelModuleInstalled = err == nil
	}
	switch {
	case !status.KernelModuleInstalled:
		status.Message = "AmneziaWG kernel module is not available on the host"
	case !status.KernelModuleLoaded:
		status.Message = "AmneziaWG kernel module not loaded"
	case !status.AWGAvailable && !status.AWGQuickAvailable:
		status.Message = "awg-tools not installed: awg and awg-quick are missing"
	case !status.AWGAvailable:
		status.Message = "awg-tools not installed: awg is missing"
	case !status.AWGQuickAvailable:
		status.Message = "awg-tools not installed: awg-quick is missing"
	}
	return status
}

func (s *Service) lookup(name string) (string, bool) {
	path, err := s.LookPath(name)
	return path, err == nil
}

func (s *Service) requireReady() error {
	status := s.Health()
	if !status.Ready() {
		return errors.New(status.Message)
	}
	return nil
}

func (s *Service) Interfaces(ctx context.Context) ([]agentapi.Interface, error) {
	details, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]agentapi.Interface, 0, len(details))
	for _, detail := range details {
		result = append(result, detail.Interface)
	}
	return result, nil
}

// Snapshot reads config and runtime metadata once per interface.
func (s *Service) Snapshot(ctx context.Context) ([]agentapi.InterfaceDetail, error) {
	entries, err := os.ReadDir(s.ConfigDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	names := map[string]bool{}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".conf")
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".conf") && validateInterfaceName(name) == nil {
			names[name] = true
		}
	}
	if s.Health().AWGAvailable {
		if out, err := s.command(ctx, nil, s.AWG, "show", "interfaces"); err == nil {
			for _, name := range strings.Fields(string(out)) {
				if validateInterfaceName(name) == nil {
					names[name] = true
				}
			}
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	result := make([]agentapi.InterfaceDetail, 0, len(ordered))
	for _, name := range ordered {
		info, err := s.Interface(ctx, name, true)
		if err == nil {
			result = append(result, info)
		}
	}
	return result, nil
}

func (s *Service) Interface(ctx context.Context, name string, includeConfig bool) (agentapi.InterfaceDetail, error) {
	path, err := s.configPath(name)
	if err != nil {
		return agentapi.InterfaceDetail{}, err
	}
	data, readErr := readConfigFile(path)
	running := s.interfaceRunning(ctx, name)
	if readErr != nil && !running {
		return agentapi.InterfaceDetail{}, os.ErrNotExist
	}
	content := string(data)
	info := agentapi.InterfaceDetail{Interface: agentapi.Interface{
		Name: name, ConfigPath: path, Running: running, Enabled: s.unitEnabled(ctx, name), ListenPort: configPort(content),
	}}
	if running {
		if out, err := s.command(ctx, nil, s.AWG, "show", name, "public-key"); err == nil {
			info.PublicKey = strings.TrimSpace(string(out))
		}
	} else if private := configValue(content, "interface", "PrivateKey"); private != "" && s.Health().AWGAvailable {
		if out, err := s.command(ctx, []byte(private+"\n"), s.AWG, "pubkey"); err == nil {
			info.PublicKey = strings.TrimSpace(string(out))
		}
	}
	if includeConfig {
		info.Config = content
	}
	return info, nil
}

func (s *Service) interfaceRunning(ctx context.Context, name string) bool {
	if validateInterfaceName(name) != nil {
		return false
	}
	_, err := s.command(ctx, nil, s.IP, "link", "show", "dev", name)
	return err == nil
}

func (s *Service) unitEnabled(ctx context.Context, name string) bool {
	// Cache discovery for reads only. Lifecycle operations always resolve
	// the unit afresh so an external template change cannot target the wrong VPN.
	s.unitMu.Lock()
	if s.units == nil {
		s.units = map[string]unitObservation{}
	}
	seen, ok := s.units[name]
	if !ok || time.Since(seen.at) >= 30*time.Second {
		seen = unitObservation{unit: s.managedUnit(ctx, name), at: time.Now()}
		s.units[name] = seen
	}
	s.unitMu.Unlock()
	_, err := s.command(ctx, nil, s.Systemctl, "is-enabled", "--quiet", seen.unit)
	return err == nil
}

func (s *Service) Create(ctx context.Context, req agentapi.CreateInterfaceRequest) error {
	if err := s.requireReady(); err != nil {
		return err
	}
	path, err := s.configPath(req.Name)
	if err != nil {
		return err
	}
	if err := validateConfig(req.Config); err != nil {
		return err
	}
	if err := validateSubnet(req.Subnet); err != nil {
		return err
	}
	if err := validateNetworkPolicy(req.Config, ""); err != nil {
		return err
	}
	prefix, _ := netip.ParsePrefix(strings.TrimSpace(req.Subnet))
	req.Config = addManagedFirewallHooks(req.Config, prefix.Masked().String())
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("interface %s already has a config", req.Name)
	} else if !os.IsNotExist(err) {
		return err
	}
	if port := configPort(req.Config); port > 0 && port <= 65535 {
		if holder := s.portHolder(port, req.Name); holder != "" {
			return fmt.Errorf("Port already in use by %s", holder)
		}
		if listenErr := s.ProbeUDP(port); listenErr != nil {
			return fmt.Errorf("Port already in use by another host process")
		}
	} else {
		return errors.New("invalid or missing ListenPort (must be 1-65535)")
	}
	if err := os.MkdirAll(s.ConfigDir, 0o700); err != nil {
		return err
	}
	if err := atomicfile.Write(path, []byte(req.Config), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if req.Enable || req.Start {
		args := []string{"enable"}
		if req.Start {
			args = append(args, "--now")
		}
		args = append(args, s.managedUnit(ctx, req.Name))
		if _, err := s.command(ctx, nil, s.Systemctl, args...); err != nil {
			return fmt.Errorf("Interface %s failed to start: %w", req.Name, err)
		}
	}
	return nil
}

func (s *Service) Apply(ctx context.Context, name, content string, live bool) error {
	if err := s.requireReady(); err != nil {
		return err
	}
	path, err := s.configPath(name)
	if err != nil {
		return err
	}
	if err := validateConfigWithHooks(content, true); err != nil {
		return err
	}
	old, err := readConfigFile(path)
	if err != nil {
		return err
	}
	if err := validateNetworkPolicy(content, string(old)); err != nil {
		return err
	}
	if hooks(string(old)) != hooks(content) {
		return errors.New("interface command hooks cannot be changed through the API")
	}
	if err := s.backup(name, old); err != nil {
		return err
	}
	if err := atomicfile.Write(path, []byte(content), 0o600); err != nil {
		return err
	}
	if live && s.interfaceRunning(ctx, name) {
		if err := s.sync(ctx, name); err != nil {
			// Put the persistent config back too: otherwise a failed live
			// update would produce different running and reboot state.
			_ = atomicfile.Write(path, old, 0o600)
			return err
		}
	}
	return nil
}

func hooks(content string) string {
	var found []string
	for _, raw := range strings.Split(content, "\n") {
		key, value, ok := cutDirective(strings.TrimSpace(raw))
		if ok {
			switch strings.ToLower(key) {
			case "preup", "postup", "predown", "postdown":
				found = append(found, strings.ToLower(key)+"="+value)
			}
		}
	}
	return strings.Join(found, "\n")
}

func (s *Service) backup(name string, data []byte) error {
	dir := filepath.Join(s.ConfigDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("2006-01-02T150405.000000000Z")
	return atomicfile.Write(filepath.Join(dir, name+".conf."+stamp+".backup"), data, 0o600)
}

func (s *Service) sync(ctx context.Context, name string) error {
	path, _ := s.configPath(name)
	stripped, err := s.command(ctx, nil, s.AWGQuick, "strip", path)
	if err != nil {
		return fmt.Errorf("awg-quick strip %s: %w", name, err)
	}
	if _, err := s.command(ctx, stripped, s.AWG, "syncconf", name, "/dev/stdin"); err != nil {
		return fmt.Errorf("live update for %s: %w", name, err)
	}
	return nil
}

func (s *Service) Action(ctx context.Context, name, action string) error {
	if err := s.requireReady(); err != nil {
		return err
	}
	if _, err := s.configPath(name); err != nil {
		return err
	}
	unit := s.managedUnit(ctx, name)
	var args []string
	switch action {
	case "start":
		args = []string{"enable", "--now", unit}
	case "stop":
		args = []string{"disable", "--now", unit}
	case "restart":
		args = []string{"restart", unit}
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	_, err := s.command(ctx, nil, s.Systemctl, args...)
	if err != nil {
		return fmt.Errorf("Interface %s failed to %s: %w", name, action, err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, name string) error {
	path, err := s.configPath(name)
	if err != nil {
		return err
	}
	// Delete is an explicit destructive UI action. Always ask systemd to
	// stop and disable the unit before archiving/removing its config; an
	// ambiguous status check must never leave a live interface orphaned.
	if err := s.Action(ctx, name, "stop"); err != nil {
		return err
	}
	data, err := readConfigFile(path)
	if err != nil {
		return err
	}
	if err := s.backup(name, data); err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *Service) GenerateKeys(ctx context.Context) (agentapi.KeyPair, error) {
	if err := s.requireReady(); err != nil {
		return agentapi.KeyPair{}, err
	}
	private, err := s.command(ctx, nil, s.AWG, "genkey")
	if err != nil {
		return agentapi.KeyPair{}, err
	}
	priv := strings.TrimSpace(string(private))
	public, err := s.command(ctx, []byte(priv+"\n"), s.AWG, "pubkey")
	if err != nil {
		return agentapi.KeyPair{}, err
	}
	return agentapi.KeyPair{Private: priv, Public: strings.TrimSpace(string(public))}, nil
}

func (s *Service) PresharedKey(ctx context.Context) (string, error) {
	if err := s.requireReady(); err != nil {
		return "", err
	}
	out, err := s.command(ctx, nil, s.AWG, "genpsk")
	return strings.TrimSpace(string(out)), err
}

func (s *Service) PublicKey(ctx context.Context, private string) (string, error) {
	if err := validateKey("private key", private, false); err != nil {
		return "", err
	}
	out, err := s.command(ctx, []byte(strings.TrimSpace(private)+"\n"), s.AWG, "pubkey")
	return strings.TrimSpace(string(out)), err
}

func (s *Service) RouteSource(ctx context.Context) (string, error) {
	out, err := s.command(ctx, nil, s.IP, "-4", "route", "get", "1.1.1.1")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "src" {
			return fields[i+1], nil
		}
	}
	return "", errors.New("host route has no source address")
}

func (s *Service) Stats(ctx context.Context, name string) (agentapi.InterfaceStats, error) {
	if _, err := s.configPath(name); err != nil {
		return agentapi.InterfaceStats{}, err
	}
	out, err := s.command(ctx, nil, s.AWG, "show", name, "dump")
	if err != nil {
		return agentapi.InterfaceStats{}, err
	}
	stats := agentapi.InterfaceStats{Peers: map[string]agentapi.PeerStats{}}
	for i, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			continue
		}
		handshake := "Never"
		if stamp, _ := strconv.ParseInt(fields[4], 10, 64); stamp > 0 {
			handshake = time.Unix(stamp, 0).UTC().Format(time.RFC3339)
		}
		stats.Peers[fields[0]] = agentapi.PeerStats{Endpoint: fields[2], LastHandshake: handshake,
			Received: formatBytes(fields[5]), Sent: formatBytes(fields[6])}
	}
	stats.Received = readCounter(filepath.Join("/sys/class/net", name, "statistics/rx_bytes"))
	stats.Sent = readCounter(filepath.Join("/sys/class/net", name, "statistics/tx_bytes"))
	return stats, nil
}

func readCounter(path string) string {
	data, err := readConfigFile(path)
	if err != nil {
		return "0 B"
	}
	return formatBytes(strings.TrimSpace(string(data)))
}

func formatBytes(raw string) string {
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return "0 B"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}

func (s *Service) Firewall(ctx context.Context, name, subnet string) map[string]string {
	checks := map[string]string{}
	if validateInterfaceName(name) != nil || validateSubnet(subnet) != nil {
		return checks
	}
	commands := [][]string{
		{"-C", "INPUT", "-i", name, "-j", "ACCEPT"},
		{"-C", "FORWARD", "-i", name, "-j", "ACCEPT"},
		{"-t", "nat", "-C", "POSTROUTING", "-s", subnet, "-j", "MASQUERADE"},
	}
	for _, args := range commands {
		key := "iptables " + strings.Join(args, " ")
		if _, err := s.command(ctx, nil, "iptables", args...); err == nil {
			checks[key] = "Found"
		} else {
			checks[key] = "Not found"
		}
	}
	return checks
}

func (s *Service) portHolder(port int, except string) string {
	entries, _ := os.ReadDir(s.ConfigDir)
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".conf")
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") || name == except {
			continue
		}
		if data, err := readConfigFile(filepath.Join(s.ConfigDir, entry.Name())); err == nil && configPort(string(data)) == port {
			return name
		}
	}
	return ""
}

// readConfigFile refuses links and special files and caps reads at the same
// size accepted by the API. ConfigDir is root-owned, so the lstat/open pair
// cannot be raced by a socket client.
func readConfigFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("config must be a regular file")
	}
	if info.Size() > maxConfigBytes {
		return nil, fmt.Errorf("config exceeds %d bytes", maxConfigBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigBytes {
		return nil, fmt.Errorf("config exceeds %d bytes", maxConfigBytes)
	}
	return data, nil
}

// classify is shared with HTTP error mapping without exposing command errors.
func classify(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "Port already"):
		return http.StatusConflict
	case strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "malformed") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "cannot be") || strings.Contains(err.Error(), "unsupported"):
		return http.StatusBadRequest
	default:
		return http.StatusServiceUnavailable
	}
}
