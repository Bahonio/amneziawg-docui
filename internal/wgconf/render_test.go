// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package wgconf

import (
	"strings"
	"testing"

	"github.com/Bahonio/awg-docui/internal/api"
)

func fixture(p *api.ObfuscationParams) (*api.Server, *api.Client) {
	srv := &api.Server{
		ID: "abc123", Name: "srv", Interface: "wg-abc123", Port: 54844,
		Subnet: "10.0.1.0/24", ServerIP: "10.0.1.1", MTU: 1280,
		Endpoint: "vpn.example", ServerPublicKey: "SRVPUB",
		ObfuscationEnabled: true, ObfuscationParams: p,
		DNS: []string{"8.8.8.8", "1.1.1.1"},
	}
	cl := &api.Client{
		ID: "c1", Name: "client", ServerID: srv.ID, ClientIP: "10.0.1.2",
		ClientPrivateKey: "CPRIV", ClientPublicKey: "CPUB", PresharedKey: "PSK",
		ObfuscationEnabled: true, ObfuscationParams: p,
	}
	return srv, cl
}

func TestAwg31FlagsOnInClientConf(t *testing.T) {
	p := &api.ObfuscationParams{S1: 50, S2: 60, S3: 20, S4: 16, MTU: 1280,
		HeaderProtectionKey: "KEY", RandomTrailers: true, DisableCookies: true}
	srv, cl := fixture(p)

	conf := ClientConf(srv, cl, "vpn.example", false)
	if !strings.Contains(conf, "RandomTrailers = on\n") || !strings.Contains(conf, "DisableCookies = on\n") {
		t.Fatalf("client conf missing 3.1 switches:\n%s", conf)
	}
	if !strings.Contains(conf, "HeaderProtectionKey = KEY\n") {
		t.Fatalf("client conf missing the header protection key:\n%s", conf)
	}
	if !strings.Contains(conf, "Endpoint = vpn.example:54844\n") || !strings.Contains(conf, "AllowedIPs = "+DefaultAllowedIPs+"\n") {
		t.Fatalf("client conf peer section wrong:\n%s", conf)
	}
}

func TestAwg31FlagsOffAreOmitted(t *testing.T) {
	p := &api.ObfuscationParams{S1: 50, S2: 60, S3: 20, S4: 16, MTU: 1280, HeaderProtectionKey: "KEY"}
	srv, cl := fixture(p)

	conf := ClientConf(srv, cl, "vpn.example", false)
	if strings.Contains(conf, "RandomTrailers") || strings.Contains(conf, "DisableCookies") {
		t.Fatalf("off flags leaked into conf:\n%s", conf)
	}
}

// The server's [Interface] and the client's carry the same parameter lines:
// obfuscation only works when both ends were issued the same values.
func TestServerAndClientAgreeOnObfuscationLines(t *testing.T) {
	p := &api.ObfuscationParams{Jc: 4, Jmin: 8, Jmax: 80, S1: 50, S2: 60, S3: 20, S4: 16,
		H1: 1, H2: 2, H3: 3, H4: 4, HeaderProtectionKey: "KEY", RandomTrailers: true}
	srv, cl := fixture(p)

	server := ServerInterface{PrivateKey: "SPRIV", Address: "10.0.1.1/24", ListenPort: 54844, MTU: 1280, Obfuscation: p}.Render()
	client := ClientConf(srv, cl, "vpn.example", false)

	for _, line := range []string{"Jc = 4", "S1 = 50", "H4 = 4", "HeaderProtectionKey = KEY", "RandomTrailers = on"} {
		if !strings.Contains(server, line+"\n") || !strings.Contains(client, line+"\n") {
			t.Errorf("%q missing from one side:\n--- server\n%s\n--- client\n%s", line, server, client)
		}
	}
}

// Without I1 nothing signs the first packet, so the rest are not written.
func TestISettingsHangOffI1(t *testing.T) {
	srv, cl := fixture(nil)
	cl.ObfuscationEnabled = false
	cl.ApplyISettings = true
	cl.ISettings = map[string]string{"i2": "<r 20>"}

	if conf := ClientConf(srv, cl, "vpn.example", false); strings.Contains(conf, "I2 = ") {
		t.Errorf("I2 written without I1:\n%s", conf)
	}

	cl.ISettings["i1"] = "<b 0xab>"
	conf := ClientConf(srv, cl, "vpn.example", false)
	if !strings.Contains(conf, "I1 = <b 0xab>\n") || !strings.Contains(conf, "I2 = <r 20>\n") {
		t.Errorf("I-settings missing:\n%s", conf)
	}
}

func TestMergeISettingsLayersOverrides(t *testing.T) {
	got := MergeISettings(map[string]string{"i1": "stored", "i2": "stored2"}, map[string]string{"i1": "new", "i2": ""})
	if got["i1"] != "new" || got["i2"] != "stored2" || got["i3"] != DefaultI3 {
		t.Errorf("MergeISettings = %v", got)
	}
	if got := MergeISettings(); got["i1"] != DefaultI1 {
		t.Errorf("no layers should yield the defaults, got %v", got)
	}
}
