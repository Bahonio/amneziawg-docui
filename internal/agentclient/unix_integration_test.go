package agentclient_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bahonio/awg-docui/internal/agentclient"
	"github.com/Bahonio/awg-docui/internal/hostagent"
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

	status, err := agentclient.New(socket).Health()
	if err != nil {
		t.Fatal(err)
	}
	if !status.HostAgent || !status.KernelModuleLoaded || !status.AWGAvailable || status.Runtime != "Kernel" {
		t.Fatalf("status = %+v", status)
	}
}
