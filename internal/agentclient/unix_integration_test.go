package agentclient_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bahonio/amneziawg-docui/internal/agentclient"
	"github.com/Bahonio/amneziawg-docui/internal/hostagent"
)

type noopRunner struct{}

func (noopRunner) Run(context.Context, []byte, string, ...string) ([]byte, error) { return nil, nil }

func TestClientTalksToAgentOverUnixSocket(t *testing.T) {
	dir := t.TempDir()
	tmp, err := os.CreateTemp("/tmp", fmt.Sprintf("awg-agent-%d-", os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	socket := tmp.Name()
	tmp.Close()
	os.Remove(socket)
	t.Cleanup(func() { os.Remove(socket) })
	module := filepath.Join(dir, "module")
	if err := os.Mkdir(module, 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	service := hostagent.NewService()
	service.ConfigDir, service.ModuleDir, service.Runner = filepath.Join(dir, "configs"), module, noopRunner{}
	service.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	server := &http.Server{Handler: hostagent.Handler{Service: service}}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close(); listener.Close() })

	client := agentclient.New(socket)
	status, err := client.Health()
	if err != nil {
		t.Fatal(err)
	}
	if !status.HostAgent || !status.KernelModuleLoaded || !status.AWGAvailable || status.Runtime != "Kernel" {
		t.Fatalf("status = %+v", status)
	}
	// The same client must reconnect when the agent unlinks and recreates
	// its socket. A directory mount exposes the replacement inode.
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Health(); err == nil {
		t.Fatal("stopped agent is healthy")
	}
	replacement, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &http.Server{Handler: hostagent.Handler{Service: service}}
	go restarted.Serve(replacement)
	t.Cleanup(func() { restarted.Close() })
	if status, err := client.Health(); err != nil || !status.HostAgent {
		t.Fatalf("client did not recover after socket replacement: %+v %v", status, err)
	}
}
