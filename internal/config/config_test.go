// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package config

import (
	"slices"
	"testing"
)

func TestFromEnvReadsTheDocumentedVariables(t *testing.T) {
	t.Setenv("WEB_UI_PORT", "8080")
	t.Setenv("AUTO_START_SERVERS", "FALSE")
	t.Setenv("DEFAULT_MTU", "1420")
	t.Setenv("DEFAULT_DNS", " 9.9.9.9 ,, 1.0.0.1")
	t.Setenv("WEB_UI_PPROF", "1")

	s := FromEnv()
	if s.WebUIPort != 8080 || s.AutoStart || s.DefaultMTU != 1420 || !s.Pprof {
		t.Errorf("Settings = %+v", s)
	}
	if !slices.Equal(s.DNSServers, []string{"9.9.9.9", "1.0.0.1"}) {
		t.Errorf("DNSServers = %v", s.DNSServers)
	}
}

func TestFromEnvFallsBackOnGarbage(t *testing.T) {
	t.Setenv("WEB_UI_PORT", "eighty")
	t.Setenv("WEB_UI_PPROF", "yes please")

	s := FromEnv()
	if s.WebUIPort != 54845 || s.Pprof {
		t.Errorf("Settings = %+v", s)
	}
}

func TestSettingsRequireARealPasswordDigest(t *testing.T) {
	valid := Settings{User: "admin", PasswordHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid digest rejected: %v", err)
	}
	for _, password := range []string{"", "changeme", "YWJj"} {
		invalid := valid
		invalid.PasswordHash = password
		if err := invalid.Validate(); err == nil {
			t.Errorf("password value %q was accepted", password)
		}
	}
}
