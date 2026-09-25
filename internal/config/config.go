// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

// Package config reads the environment the backend is started with. It is the
// only place that looks at os.Getenv: everything else takes a Settings value
// and can be tested with one built by hand.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// The container writes only panel metadata below ConfigDir. Config paths are
// host paths used as identifiers in API payloads; the container never mounts
// or opens that directory.
const (
	ConfigDir          = "/app/data"
	WireguardConfigDir = "/etc/amnezia/amneziawg"
	ConfigFile         = "/app/data/web_config.json"
	AgentSocket        = "/run/awg-docui/agent.sock"
)

// Settings are the environment-driven defaults, resolved once at startup.
type Settings struct {
	// WebUIPort is the TCP port the panel listens on. It is also reserved
	// against server ports: see Manager.CreateServer.
	WebUIPort int

	// AutoStart is the default for newly created servers. Existing interface
	// boot policy is owned by host systemd and is never replayed at UI boot.
	AutoStart bool

	DefaultMTU        int
	DefaultSubnet     string
	DefaultPort       int
	DNSServers        []string
	EnableObfuscation bool

	// SuspendCheckInterval is how often scheduled client suspensions are
	// looked at.
	SuspendCheckInterval time.Duration

	// Pprof mounts /debug/pprof behind the same credentials as the API.
	Pprof bool

	// User and PasswordHash are the basic-auth credentials; the hash is the
	// base64 of the SHA-256 of the password, as fiber's middleware expects.
	User         string
	PasswordHash string

	// ConfigFile is where web_config.json lives. Tests point it at a temp
	// file so they never touch /etc.
	ConfigFile string

	// WireguardConfigDir is the host-side path recorded in panel metadata.
	WireguardConfigDir string

	// AgentSocket is the sole host integration exposed to the container.
	AgentSocket string
}

// FromEnv resolves every setting from the environment, falling back to the
// defaults the container documents.
func FromEnv() Settings {
	s := Settings{
		WebUIPort:            atoiDefault(getenv("WEB_UI_PORT", ""), 54845),
		AutoStart:            strings.EqualFold(getenv("AUTO_START_SERVERS", "true"), "true"),
		DefaultMTU:           atoiDefault(getenv("DEFAULT_MTU", ""), 1280),
		DefaultSubnet:        getenv("DEFAULT_SUBNET", "10.0.0.0/24"),
		DefaultPort:          atoiDefault(getenv("DEFAULT_PORT", ""), 54844),
		DNSServers:           splitList(getenv("DEFAULT_DNS", "8.8.8.8,1.1.1.1")),
		EnableObfuscation:    true,
		SuspendCheckInterval: time.Minute,
		Pprof:                parseBool(os.Getenv("WEB_UI_PPROF")),
		User:                 getenv("WEB_UI_USER", "admin"),
		PasswordHash:         os.Getenv("WEB_UI_PASSWORD"),
		ConfigFile:           ConfigFile,
		WireguardConfigDir:   WireguardConfigDir,
		AgentSocket:          getenv("HOST_AGENT_SOCKET", AgentSocket),
	}
	return s
}

// Validate rejects an unauthenticated or ambiguously configured public
// panel. PasswordHash is Fiber basic-auth's base64-encoded SHA-256 digest.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.User) == "" {
		return fmt.Errorf("WEB_UI_USER is required")
	}
	if s.PasswordHash == "" {
		return fmt.Errorf("WEB_UI_PASSWORD is required; set a base64-encoded SHA-256 password digest")
	}
	digest, err := base64.StdEncoding.DecodeString(s.PasswordHash)
	if err != nil || len(digest) != 32 {
		return fmt.Errorf("WEB_UI_PASSWORD must be a base64-encoded SHA-256 password digest")
	}
	return nil
}

// EnsureDirectories creates the directories the backend writes into. Errors
// are ignored on purpose: a missing directory surfaces on the first write,
// with a message that names the file.
func (s Settings) EnsureDirectories() {
	os.MkdirAll(ConfigDir, 0o700)
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// parseBool reads a Go boolean ("1", "t", "true", "TRUE" and their
// negatives); anything else counts as "off" rather than aborting the start.
func parseBool(s string) bool {
	enabled, err := strconv.ParseBool(s)
	return err == nil && enabled
}

// splitList parses a comma separated list, dropping blanks.
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
