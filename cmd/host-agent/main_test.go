package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStaleSocketRefusesNonSocketAndLiveListener(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(regular); err == nil {
		t.Fatal("non-socket path was removed")
	}
	if _, err := os.Stat(regular); err != nil {
		t.Fatalf("non-socket path changed: %v", err)
	}

	tmp, err := os.CreateTemp("/tmp", "awg-main-sock-")
	if err != nil {
		t.Fatal(err)
	}
	socket := tmp.Name()
	tmp.Close()
	os.Remove(socket)
	t.Cleanup(func() { os.Remove(socket) })
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := removeStaleSocket(socket); err == nil {
		t.Fatal("live listener socket was unlinked")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("live listener became unreachable: %v", err)
	}
	conn.Close()
}
