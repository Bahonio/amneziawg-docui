package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
	"github.com/Bahonio/amneziawg-docui/internal/hostagent"
)

func main() {
	socket := flag.String("socket", getenv("AWG_DOCUI_AGENT_SOCKET", agentapi.SocketPath), "Unix socket path")
	configDir := flag.String("config-dir", getenv("AWG_DOCUI_CONFIG_DIR", "/etc/amnezia/amneziawg"), "host config directory")
	unit := flag.String("unit", getenv("AWG_DOCUI_VPN_UNIT", "awg-docui-vpn@%s.service"), "systemd template pattern")
	socketMode := flag.String("socket-mode", getenv("AWG_DOCUI_SOCKET_MODE", "0660"), "Unix socket mode")
	socketGroup := flag.String("socket-group", getenv("AWG_DOCUI_SOCKET_GROUP", "awg-docui"), "Unix socket group")
	flag.Parse()

	if os.Geteuid() != 0 {
		log.Fatal("awg-docui-agent must run as root")
	}
	if flag.NArg() != 0 {
		log.Fatal("unexpected positional arguments")
	}
	mode, err := strconv.ParseUint(*socketMode, 8, 32)
	if err != nil || mode&0o007 != 0 {
		log.Fatal("AWG_DOCUI_SOCKET_MODE must be an octal mode without permissions for other users")
	}
	group, err := user.LookupGroup(*socketGroup)
	if err != nil {
		log.Fatalf("socket group %q: %v", *socketGroup, err)
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		log.Fatalf("invalid gid for socket group %q", *socketGroup)
	}
	if err := os.MkdirAll(filepath.Dir(*socket), 0o750); err != nil {
		log.Fatal(err)
	}
	if err := os.Chown(filepath.Dir(*socket), 0, gid); err != nil {
		log.Fatalf("cannot assign socket directory to group %q: %v", *socketGroup, err)
	}
	if err := os.Chmod(filepath.Dir(*socket), 0o750); err != nil {
		log.Fatal(err)
	}
	if err := removeStaleSocket(*socket); err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("unix", *socket)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	defer os.Remove(*socket)
	if err := os.Chmod(*socket, os.FileMode(mode)); err != nil {
		log.Fatal(err)
	}
	if err := os.Chown(*socket, 0, gid); err != nil {
		log.Fatalf("cannot assign socket to group %q: %v", *socketGroup, err)
	}

	service := hostagent.NewService()
	service.ConfigDir = *configDir
	service.SocketPath = *socket
	service.Unit = *unit
	server := &http.Server{Handler: hostagent.Handler{Service: service}, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	log.Printf("host agent listening on Unix socket %s; VPN runtime is %s", *socket, agentapi.Runtime)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	// Shutting down the management agent intentionally does not issue any
	// interface or systemd action. The kernel data plane keeps running.
	_ = server.Close()
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace non-socket %s", path)
	}
	if conn, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond); dialErr == nil {
		conn.Close()
		return fmt.Errorf("another host agent is already listening on %s", path)
	}
	return os.Remove(path)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
