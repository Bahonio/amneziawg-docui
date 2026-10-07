package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func webAccessCommand(t *testing.T, script, input string, env ...string) ([]byte, error) {
	t.Helper()
	dir := t.TempDir()
	// Simulate two real host addresses and reject absent ones. No host network
	// or deployment files are touched by these tests.
	stub := `#!/bin/sh
case "$*" in
    '-o addr show scope global') printf '2: awg0 inet 10.66.66.1/24 scope global awg0\n3: eth0 inet6 fd00::1/64 scope global\n' ;;
    '-o addr show to 10.66.66.1') echo '2: awg0 inet 10.66.66.1/24 scope global awg0' ;;
    '-o addr show to fd00::1') echo '3: eth0 inet6 fd00::1/64 scope global' ;;
    '-o addr show to 192.0.2.99') exit 0 ;;
    *) exit 98 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "ip"), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", ". ./web-access.sh\n"+script)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.CombinedOutput()
}

func TestWebAccessAddressValidation(t *testing.T) {
	for _, tt := range []struct {
		ip string
		ok bool
	}{
		{"127.0.0.1", true}, {"0.0.0.0", true}, {"10.66.66.1", true},
		{"::", true}, {"::1", true}, {"fd00::1", true}, {"2001:db8:0:0:0:0:0:1", true}, {"::ffff:192.0.2.1", true},
		{"", false}, {"panel.example.com", false}, {"256.0.0.1", false}, {"01.2.3.4", false},
		{"1.2.3", false}, {"1:2:3", false}, {"1::2::3", false}, {":::1", false},
		{"1:2:3:4:5:6:7:8::", false}, {"192.0.2.1::", false}, {"fe80::1%eth0", false}, {"10.1.1.1\nOTHER=x", false},
	} {
		t.Run(tt.ip, func(t *testing.T) {
			out, err := webAccessCommand(t, `web_valid_ip "$TEST_IP"`, "", "TEST_IP="+tt.ip)
			if (err == nil) != tt.ok {
				t.Fatalf("IP validation: %v, %s", err, out)
			}
		})
	}
}

func TestWebAccessURLValidation(t *testing.T) {
	for _, tt := range []struct {
		url string
		ok  bool
	}{
		{"https://panel.example.com", true}, {"http://10.66.66.1:54845/", true}, {"http://[fd00::1]:54845", true},
		{"http://localhost:8080", true}, {"https://panel.example.com:443", true},
		{"panel.example.com", false}, {"ftp://panel.example.com", false}, {"http://", false},
		{"https://user:secret@panel.example.com", false}, {"https://panel.example.com/path", false},
		{"https://panel.example.com?q=1", false}, {"https://panel.example.com#x", false},
		{"https://panel.example.com:0", false}, {"https://panel.example.com:65536", false}, {"http://[invalid]", false},
		{"http://[::1]:", false}, {"https://-bad.example.com", false}, {"https://bad..example", false},
		{"http://0.0.0.0:54845", false}, {"http://[::]:54845", false}, {"http://256.0.0.1", false},
		{"https://panel.example.com\nOTHER=x", false}, {"https://panel.example.com//", false},
	} {
		t.Run(tt.url, func(t *testing.T) {
			out, err := webAccessCommand(t, `web_valid_url "$TEST_URL"`, "", "TEST_URL="+tt.url)
			if (err == nil) != tt.ok {
				t.Fatalf("URL validation: %v, %s", err, out)
			}
		})
	}
}

const webAccessInit = `
access=${TEST_ACCESS:-}
requested_bind=${TEST_BIND:-}
requested_port=${TEST_PORT:-}
requested_url=${TEST_URL:-}
url_was_requested=${TEST_HAS_URL:-no}
access_flags=yes
configure_access=no
non_interactive=yes
`

func TestWebAccessOptionsAndPreservation(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	// Include a value that would execute if the installer sourced .env.
	contents := " export WEB_UI_BIND_ADDRESS = '10.66.66.1' # VPN\nWEB_UI_PORT=8445 # panel port\nWEB_UI_URL=\"https://panel.example.com\" # browser\nOTHER=$(exit 99)\n"
	if err := os.WriteFile(envPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		env  []string
		want string
		ok   bool
	}{
		{"preserve", nil, "Panel URL: https://panel.example.com", true},
		{"ssh clears old URL", []string{"TEST_ACCESS=ssh"}, "ssh -L 8445:127.0.0.1:8445", true},
		{"VPN", []string{"TEST_ACCESS=vpn", "TEST_BIND=10.66.66.1", "TEST_PORT=8080"}, "Panel URL: http://10.66.66.1:8080", true},
		{"IPv6", []string{"TEST_ACCESS=vpn", "TEST_BIND=fd00::1"}, "Panel URL: http://[fd00::1]:8445", true},
		{"proxy", []string{"TEST_ACCESS=proxy", "TEST_HAS_URL=yes", "TEST_URL=https://other.example.com"}, "Reverse proxy upstream on this host: http://127.0.0.1:8445", true},
		{"all", []string{"TEST_ACCESS=all"}, "Panel URL: http://YOUR_SERVER_IP:8445", true},
		{"absent address", []string{"TEST_BIND=192.0.2.99"}, "not assigned to this host", false},
		{"hostname as bind", []string{"TEST_BIND=panel.example.com"}, "Invalid bind IP", false},
		{"bad port", []string{"TEST_PORT=65536"}, "Invalid panel port", false},
		{"empty proxy URL", []string{"TEST_ACCESS=proxy"}, "requires --panel-url", false},
		{"VPN wildcard", []string{"TEST_ACCESS=vpn", "TEST_BIND=0.0.0.0"}, "requires a specific server IP", false},
		{"SSH remote", []string{"TEST_ACCESS=ssh", "TEST_BIND=10.66.66.1"}, "requires a loopback bind", false},
		{"unknown mode", []string{"TEST_ACCESS=bad"}, "Invalid --access", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := append([]string{"TEST_ENV_PATH=" + envPath}, tt.env...)
			out, err := webAccessCommand(t, webAccessInit+`configure_web_access "$TEST_ENV_PATH" || exit 1; print_web_access`, "", env...)
			if (err == nil) != tt.ok || !strings.Contains(string(out), tt.want) {
				t.Fatalf("configure: %v, %s; want %s", err, out, tt.want)
			}
			got, err := os.ReadFile(envPath)
			if err != nil || string(got) != contents {
				t.Fatal("configuration step changed existing deployment file")
			}
		})
	}
	// Updates may run before a VPN address returns: preserve it without probing.
	out, err := webAccessCommand(t, webAccessInit+`configure_web_access "$TEST_ENV_PATH"`, "", "TEST_ENV_PATH="+envPath, "PATH=/usr/bin:/bin")
	if err != nil {
		t.Fatalf("unchanged binding should not need ip: %v: %s", err, out)
	}
}

func TestWebAccessMenu(t *testing.T) {
	for _, tt := range []struct{ name, input, want string }{
		{"SSH default", "\n\n", "ssh -L 54845:127.0.0.1:54845"},
		{"VPN selection", "2\n1\n8080\n", "Panel URL: http://10.66.66.1:8080"},
		{"VPN manual IPv6", "2\nfd00::1\n\n", "Panel URL: http://[fd00::1]:54845"},
		{"proxy", "3\n8080\nhttps://panel.example.com\n", "Panel URL: https://panel.example.com"},
		{"all", "4\n\n", "Panel URL: http://YOUR_SERVER_IP:54845"},
		{"retry invalid input", "9\n2\n192.0.2.99\n1\n0\n8080\n", "Panel URL: http://10.66.66.1:8080"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := webAccessCommand(t, `bind_address=127.0.0.1; web_port=54845; exec 4<&0 5>&1; web_access_dialogue || exit 1; print_web_access`, tt.input)
			if err != nil || !strings.Contains(string(out), tt.want) {
				t.Fatalf("menu: %v, %s", err, out)
			}
		})
	}
	out, err := webAccessCommand(t, `bind_address=127.0.0.1; web_port=54845; exec 4<&0 5>&1; web_access_dialogue`, "2\n")
	if err == nil || !strings.Contains(string(out), "terminal input ended") {
		t.Fatalf("EOF should cancel configuration: %v, %s", err, out)
	}
}
