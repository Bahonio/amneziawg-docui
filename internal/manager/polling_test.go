package manager

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
	"github.com/Bahonio/amneziawg-docui/internal/awg"
	"github.com/Bahonio/amneziawg-docui/internal/awg/awgtest"
)

type snapshotBackend struct {
	*awgtest.Runner
	calls atomic.Int32
	fail  bool
}

func (b *snapshotBackend) Snapshot() ([]agentapi.InterfaceDetail, error) {
	b.calls.Add(1)
	if b.fail {
		return nil, errors.New("unavailable")
	}
	return []agentapi.InterfaceDetail{{
		Interface: agentapi.Interface{Name: "wg-test-absent", Running: true},
		Config:    "[Interface]\nPrivateKey = " + adoptionKey(1) + "\nAddress = 10.0.1.1/24\nListenPort = 54844\nMTU = 1280\n",
	}}, nil
}

func TestConcurrentDashboardPollsShareSnapshotAndStatus(t *testing.T) {
	m, runner := newTestManager(t)
	b := &snapshotBackend{Runner: runner}
	m.tools = awg.New(b)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			servers := m.Servers()
			if len(servers) != 1 || servers[0].Status != "running" {
				t.Errorf("snapshot status lost: %+v", servers)
			}
		})
	}
	wg.Wait()
	if b.calls.Load() != 1 {
		t.Fatalf("snapshot calls = %d", b.calls.Load())
	}
	m.hostMu.Lock()
	m.lastAdoption = time.Now().Add(-statusTTL - time.Second)
	m.hostMu.Unlock()
	m.Servers()
	if b.calls.Load() != 2 {
		t.Fatal("snapshot did not refresh after expiry")
	}
}

func TestFailedSnapshotDoesNotDelayRecovery(t *testing.T) {
	m, runner := newTestManager(t)
	b := &snapshotBackend{Runner: runner, fail: true}
	m.tools = awg.New(b)
	m.Servers()
	b.fail = false
	servers := m.Servers()
	if b.calls.Load() != 2 || servers[0].Status != "running" {
		t.Fatalf("recovery delayed: calls=%d servers=%+v", b.calls.Load(), servers)
	}
}
