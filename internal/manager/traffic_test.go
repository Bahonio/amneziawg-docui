// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"testing"

	"github.com/Bahonio/awg-docui/internal/api"
)

func TestTrafficSnapshotJoinsPeersWithClients(t *testing.T) {
	m, run := newTestManager(t)
	seen, _, _ := m.AddClient("s1", api.AddClientRequest{Name: "seen"})
	quiet, _, _ := m.AddClient("s1", api.AddClientRequest{Name: "quiet"})

	run.Stub("/usr/bin/awg show wg-test-absent",
		"interface: wg-test-absent\n\npeer: "+seen.ClientPublicKey+"\n  endpoint: 203.0.113.5:1\n  transfer: 1 KiB received, 2 KiB sent\n")
	run.Stub("ifconfig wg-test-absent", "RX bytes:10 (10.0 B)  TX bytes:20 (20.0 B)")

	snap := m.TrafficSnapshot()
	peers := snap.ClientTraffic["s1"]
	if peers[seen.ID].Received != "1 KiB" || peers[seen.ID].Endpoint != "203.0.113.5:1" {
		t.Errorf("seen client = %+v", peers[seen.ID])
	}
	if peers[quiet.ID].Received != "0 B" || peers[quiet.ID].LastHandshake != "Never" {
		t.Errorf("quiet client = %+v", peers[quiet.ID])
	}
	if snap.ServerTraffic["s1"]["rx"] != "10.0 B" || snap.ServerTraffic["s1"]["tx"] != "20.0 B" {
		t.Errorf("server traffic = %v", snap.ServerTraffic["s1"])
	}
}

// A server whose interface is down contributes nothing rather than zeros, so
// the page keeps showing the last counters it had.
func TestTrafficSnapshotSkipsDownInterfaces(t *testing.T) {
	m, _ := newTestManager(t)
	snap := m.TrafficSnapshot()
	if len(snap.ClientTraffic) != 0 || len(snap.ServerTraffic) != 0 {
		t.Errorf("down interface reported traffic: %+v", snap)
	}
}
