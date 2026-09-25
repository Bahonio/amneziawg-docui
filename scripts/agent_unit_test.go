package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentUnitPreflight(t *testing.T) {
	cases := []struct {
		name, state, exit string
		ok                bool
	}{
		{"not installed", "LoadState=not-found\n", "0", true},
		{"ordinary unit", "LoadState=loaded\nNeedDaemonReload=no\nRequires=sysinit.target -.mount\nConflicts=shutdown.target\nConflictedBy=shutdown.target\n", "0", true},
		{"reported host PrivateTmp dependency", "LoadState=loaded\nRequires=system.slice sysinit.target -.mount\nWants=tmp.mount\nDropInPaths=\nNeedDaemonReload=no\n", "0", true},
		{"temporary filesystem ancestors", "LoadState=loaded\nWants=-.mount tmp.mount var.mount var-tmp.mount\n", "0", true},
		{"temporary mount and VPN service", "LoadState=loaded\nWants=tmp.mount amneziawg@awg0.service\n", "0", false},
		{"temporary mount and unknown service", "LoadState=loaded\nWants=tmp.mount network-helper.service\n", "0", false},
		{"mount-like service name", "LoadState=loaded\nWants=tmp.mount.service\n", "0", false},
		{"unrelated mount", "LoadState=loaded\nWants=mnt-vpn.mount\n", "0", false},
		{"mount stop propagation", "LoadState=loaded\nConsistsOf=tmp.mount\n", "0", false},
		{"unreadable systemd", "", "1", false},
		{"empty response", "", "0", false},
		{"masked unit", "LoadState=masked\n", "0", false},
		{"pending reload", "LoadState=loaded\nNeedDaemonReload=yes\n", "0", false},
		{"local override", "LoadState=loaded\nDropInPaths=/etc/systemd/system/awg-docui-agent.service.d/custom.conf\n", "0", false},
	}
	for _, property := range []string{"Requires", "Wants", "Requisite", "BindsTo", "PartOf", "ConsistsOf", "BoundBy", "RequiredBy", "RequisiteOf", "Upholds", "UpheldBy", "Conflicts", "ConflictedBy", "PropagatesStopTo", "StopPropagatedFrom", "OnFailure", "OnSuccess"} {
		cases = append(cases, struct {
			name, state, exit string
			ok                bool
		}{property, "LoadState=loaded\n" + property + "=amneziawg@awg0.service\n", "0", false})
	}
	for _, property := range []string{"ExecStartPre", "ExecStartPost", "ExecStop", "ExecStopPost"} {
		cases = append(cases, struct {
			name, state, exit string
			ok                bool
		}{property, "LoadState=loaded\n" + property + "=SECRET-COMMAND\n", "0", false})
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := filepath.Join(dir, "unit-state")
			if err := os.WriteFile(fixture, []byte(tt.state), 0o600); err != nil {
				t.Fatal(err)
			}
			stub := "#!/bin/sh\n[ \"$1\" = show ] && [ \"$2\" = awg-docui-agent.service ] || exit 98\ncat \"$UNIT_FIXTURE\"\nexit \"$UNIT_EXIT\"\n"
			if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(stub), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("sh", "check-agent-unit.sh")
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "UNIT_FIXTURE="+fixture, "UNIT_EXIT="+tt.exit)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tt.ok {
				t.Fatalf("check returned %v: %s", err, out)
			}
			if strings.Contains(string(out), "SECRET-COMMAND") {
				t.Fatal("preflight logged command hook contents")
			}
		})
	}
}
