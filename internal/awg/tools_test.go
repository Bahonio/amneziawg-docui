// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package awg_test

import (
	"encoding/base64"
	"testing"

	"github.com/Bahonio/awg-docui/internal/awg"
	"github.com/Bahonio/awg-docui/internal/awg/awgtest"
)

const showOutput = `interface: wg-abc123
  public key: SRVPUB
  listening port: 54844

peer: PEER1
  endpoint: 203.0.113.5:51820
  allowed ips: 10.0.1.2/32
  latest handshake: 1 minute, 3 seconds ago
  transfer: 1.20 MiB received, 3.40 MiB sent

peer: PEER2
  allowed ips: 10.0.1.3/32
`

func TestParseShow(t *testing.T) {
	peers := awg.ParseShow(showOutput)
	if len(peers) != 2 {
		t.Fatalf("peers = %v", peers)
	}
	p1 := peers["PEER1"]
	if p1.Received != "1.20 MiB" || p1.Sent != "3.40 MiB" || p1.Endpoint != "203.0.113.5:51820" ||
		p1.LastHandshake != "1 minute, 3 seconds ago" {
		t.Errorf("PEER1 = %+v", p1)
	}
	p2 := peers["PEER2"]
	if p2.Received != "0 B" || p2.Sent != "0 B" || p2.LastHandshake != "Never" || p2.Endpoint != "" {
		t.Errorf("PEER2 = %+v", p2)
	}
}

func TestInterfaceCounters(t *testing.T) {
	run := awgtest.New().Stub("ifconfig wg-up",
		"wg-up: flags=209<UP,POINTOPOINT,RUNNING,NOARP>\n"+
			"          RX bytes:1234567 (1.1 MiB)  TX bytes:7654321 (7.2 MiB)\n")
	tools := awg.New(run)

	rx, tx, ok := tools.InterfaceCounters("wg-up")
	if !ok || rx != "1.1 MiB" || tx != "7.2 MiB" {
		t.Errorf("InterfaceCounters = %q, %q, %v", rx, tx, ok)
	}
	if _, _, ok := tools.InterfaceCounters("wg-down"); ok {
		t.Error("a down interface reported counters")
	}
}

func TestInterfaceUpAsksTheKernel(t *testing.T) {
	run := awgtest.New().Stub("ip link show wg-up", "5: wg-up: <POINTOPOINT,NOARP,UP,LOWER_UP> state UNKNOWN")
	tools := awg.New(run)

	if !tools.InterfaceUp("wg-up") {
		t.Error("an interface the kernel lists was reported down")
	}
	if tools.InterfaceUp("wg-down") || tools.InterfaceUp("") {
		t.Error("a missing interface was reported up")
	}
}

// The in-memory test backend supplies well-formed keys without host tools.
// Production has no such path and delegates key generation to host-agent.
func TestTestBackendSuppliesWellFormedKeys(t *testing.T) {
	tools := awg.New(awgtest.New())

	pair := tools.GenerateKeyPair()
	for _, k := range []string{pair.Private, pair.Public, tools.GeneratePresharedKey()} {
		raw, err := base64.StdEncoding.DecodeString(k)
		if err != nil || len(raw) != 32 {
			t.Errorf("test key %q: %d bytes, %v", k, len(raw), err)
		}
	}
	if pair.Private == pair.Public {
		t.Error("the test backend handed out one key twice")
	}
}

func TestKeyGenerationUsesAwgWhenPresent(t *testing.T) {
	run := awgtest.New().Stub("awg genkey", "PRIV").Stub("echo 'PRIV' | awg pubkey", "PUB").Stub("awg genpsk", "PSK")
	tools := awg.New(run)

	if pair := tools.GenerateKeyPair(); pair.Private != "PRIV" || pair.Public != "PUB" {
		t.Errorf("GenerateKeyPair = %+v", pair)
	}
	if psk := tools.GeneratePresharedKey(); psk != "PSK" {
		t.Errorf("GeneratePresharedKey = %q", psk)
	}
}
