// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package netutil

import "testing"

func TestNextFreeIPSkipsNetworkBroadcastAndUsed(t *testing.T) {
	used := map[string]bool{"10.0.1.1": true, "10.0.1.2": true}
	if got := NextFreeIP("10.0.1.0/24", used); got != "10.0.1.3" {
		t.Errorf("NextFreeIP = %q, want 10.0.1.3", got)
	}

	// A /30 has exactly two hosts; both taken means full.
	full := map[string]bool{"10.0.1.1": true, "10.0.1.2": true}
	if got := NextFreeIP("10.0.1.0/30", full); got != "" {
		t.Errorf("full subnet handed out %q", got)
	}

	if got := NextFreeIP("10.0.1.0", nil); got != "" {
		t.Errorf("subnet without a prefix handed out %q", got)
	}
}

func TestSplitCIDRAndServerIP(t *testing.T) {
	if n, p := SplitCIDR("10.1.2.0/16"); n != "10.1.2.0" || p != "16" {
		t.Errorf("SplitCIDR = %q/%q", n, p)
	}
	if n, p := SplitCIDR("10.1.2.0"); n != "10.1.2.0" || p != "24" {
		t.Errorf("SplitCIDR without prefix = %q/%q", n, p)
	}
	if got := ServerIP("10.1.2.0"); got != "10.1.2.1" {
		t.Errorf("ServerIP = %q", got)
	}
}
