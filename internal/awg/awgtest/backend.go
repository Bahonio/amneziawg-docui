// Package awgtest provides a typed, filesystem-backed Backend for tests. It
// preserves the concise command stubs used by older tests without putting a
// command runner in the production Web backend.
package awgtest

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
	"github.com/Bahonio/amneziawg-docui/internal/atomicfile"
	"github.com/Bahonio/amneziawg-docui/internal/awg"
)

var ErrNotInstalled = errors.New("awgtest: command not stubbed")

type Runner struct {
	mu       sync.Mutex
	stubs    map[string]stub
	Commands []string
}

type stub struct {
	out string
	err error
}

func New() *Runner { return &Runner{stubs: map[string]stub{}} }

func (r *Runner) Stub(prefix, out string) *Runner {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs[prefix] = stub{out: out}
	return r
}

func (r *Runner) Fail(prefix string, err error) *Runner {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs[prefix] = stub{err: err}
	return r
}

func (r *Runner) Unstub(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.stubs, prefix)
}

func (r *Runner) run(command string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Commands = append(r.Commands, command)
	best, found := "", false
	for prefix := range r.stubs {
		if strings.HasPrefix(command, prefix) && len(prefix) >= len(best) {
			best, found = prefix, true
		}
	}
	if !found {
		return "", ErrNotInstalled
	}
	s := r.stubs[best]
	return s.out, s.err
}

func (r *Runner) Ran(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, command := range r.Commands {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

func (r *Runner) Health() (agentapi.BackendStatus, error) {
	return agentapi.BackendStatus{HostAgent: true, KernelModuleInstalled: true, KernelModuleLoaded: true,
		AWGAvailable: true, AWGQuickAvailable: true, Runtime: agentapi.Runtime}, nil
}

func (r *Runner) Interfaces() ([]agentapi.Interface, error) { return nil, nil }

func (r *Runner) InterfaceStatus(name string) (bool, error) {
	_, err := r.run("ip link show " + name)
	return err == nil, nil
}

func (r *Runner) QuickUp(name string) error {
	_, err := r.run("/usr/bin/awg-quick up " + name)
	if err != nil {
		return fmt.Errorf("awg-quick up %s: %w", name, err)
	}
	return nil
}

func (r *Runner) QuickDown(name string) error {
	_, err := r.run("/usr/bin/awg-quick down " + name)
	if err != nil {
		return fmt.Errorf("awg-quick down %s: %w", name, err)
	}
	return nil
}

func (r *Runner) Restart(name string) error {
	_, err := r.run("systemctl restart " + name)
	return err
}

func (r *Runner) ShowPeers(name string) (map[string]awg.PeerStats, error) {
	out, err := r.run("/usr/bin/awg show " + name)
	if err != nil {
		return nil, err
	}
	return awg.ParseShow(out), nil
}

var (
	rxRe = regexp.MustCompile(`RX bytes:\d+\s+\(([^)]+)\)`)
	txRe = regexp.MustCompile(`TX bytes:\d+\s+\(([^)]+)\)`)
)

func (r *Runner) InterfaceCounters(name string) (string, string, error) {
	out, err := r.run("ifconfig " + name)
	if err != nil {
		return "", "", err
	}
	rx, tx := "0 B", "0 B"
	if match := rxRe.FindStringSubmatch(out); len(match) > 1 {
		rx = match[1]
	}
	if match := txRe.FindStringSubmatch(out); len(match) > 1 {
		tx = match[1]
	}
	return rx, tx, nil
}

func (r *Runner) ReadConfig(_, path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func (r *Runner) CreateConfig(_, path, content, _ string) error {
	return atomicfile.Write(path, []byte(content), 0o600)
}

func (r *Runner) ApplyConfig(_, path, content string) error {
	return atomicfile.Write(path, []byte(content), 0o600)
}

func (r *Runner) DeleteConfig(_, path string) error { return os.Remove(path) }

func (r *Runner) SyncConf(name string) error {
	_, err := r.run("awg syncconf " + name)
	return err
}

func (r *Runner) CheckFirewall(name, subnet string) (map[string]string, error) {
	commands := []string{
		"iptables -L INPUT -n | grep " + name,
		"iptables -L FORWARD -n | grep " + name,
		"iptables -t nat -L POSTROUTING -n | grep " + subnet,
	}
	result := map[string]string{}
	for _, command := range commands {
		out, err := r.run(command)
		if err == nil && out != "" {
			result[command] = "Found"
		} else {
			result[command] = "Not found"
		}
	}
	return result, nil
}

func (r *Runner) RouteSourceIP() (string, error) { return r.run("ip route get 1") }

func (r *Runner) GenerateKeyPair() (awg.KeyPair, error) {
	private, err := r.run("awg genkey")
	if err != nil {
		return awg.KeyPair{Private: awg.RandomKey(), Public: awg.RandomKey()}, nil
	}
	public, err := r.run(fmt.Sprintf("echo '%s' | awg pubkey", private))
	return awg.KeyPair{Private: private, Public: public}, err
}

func (r *Runner) GeneratePresharedKey() (string, error) {
	key, err := r.run("awg genpsk")
	if err != nil {
		return awg.RandomKey(), nil
	}
	return key, nil
}
