// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package wgconf

import (
	"strings"
	"testing"

	"github.com/Bahonio/awg-docui/internal/api"
)

func TestParsePeerMarker(t *testing.T) {
	cases := []struct {
		line, name, id string
		ok             bool
	}{
		{line: "# Client: alice [id:ab12cd]", name: "alice", id: "ab12cd", ok: true},
		{line: "  # Client: alice [id:ab12cd]  ", name: "alice", id: "ab12cd", ok: true},
		{line: "# Client: alice", name: "alice", ok: true},
		{line: "# Client: two words [id:x1]", name: "two words", id: "x1", ok: true},
		{line: "[Peer]"},
		{line: "PublicKey = ABC"},
		{line: "# something else"},
	}
	for _, c := range cases {
		name, id, ok := ParsePeerMarker(c.line)
		if ok != c.ok || name != c.name || id != c.id {
			t.Errorf("ParsePeerMarker(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.line, name, id, ok, c.name, c.id, c.ok)
		}
	}
}

// Two clients sharing a name is the case the ID exists for: matching by name
// alone removed both blocks at once.
func TestSplitPeerBlockPicksTheRightOneOfTwoNamesakes(t *testing.T) {
	conf := []string{
		"[Interface]",
		"PrivateKey = PRIV",
		"",
		"# Client: alice [id:aaa]",
		"[Peer]",
		"PublicKey = FIRST",
		"",
		"# Client: alice [id:bbb]",
		"[Peer]",
		"PublicKey = SECOND",
	}

	rest, block, found := SplitPeerBlock(conf, &api.Client{ID: "bbb", Name: "alice"})
	if !found {
		t.Fatal("block not found")
	}
	if strings.Join(block, "\n") != "# Client: alice [id:bbb]\n[Peer]\nPublicKey = SECOND" {
		t.Errorf("wrong block extracted:\n%s", strings.Join(block, "\n"))
	}
	joined := strings.Join(rest, "\n")
	if !strings.Contains(joined, "FIRST") {
		t.Errorf("the namesake's block was removed too:\n%s", joined)
	}
	if strings.Contains(joined, "SECOND") {
		t.Errorf("the block is still there:\n%s", joined)
	}
}

// A marker with no ID belongs to no client - it was not written here - but it
// still ends the block above it, so removing that peer leaves it in place.
func TestSplitPeerBlockIgnoresAnUntaggedMarker(t *testing.T) {
	conf := []string{
		"[Interface]",
		"",
		"# Client: alice [id:aaa]",
		"[Peer]",
		"PublicKey = MINE",
		"",
		"# Client: alice",
		"[Peer]",
		"PublicKey = FOREIGN",
	}

	if _, _, found := SplitPeerBlock(conf, &api.Client{ID: "", Name: "alice"}); found {
		t.Error("an untagged marker must not match a client")
	}

	rest, block, found := SplitPeerBlock(conf, &api.Client{ID: "aaa", Name: "alice"})
	if !found || len(block) != 3 {
		t.Fatalf("found = %v, block = %v", found, block)
	}
	joined := strings.Join(rest, "\n")
	if strings.Contains(joined, "MINE") {
		t.Errorf("block not removed:\n%s", joined)
	}
	if !strings.Contains(joined, "FOREIGN") {
		t.Errorf("the untagged block was swallowed:\n%s", joined)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"alice", "alice"},
		{"  alice  ", "alice"},
		{"", "fb"},
		{"   ", "fb"},
		// A newline would end the comment line and turn the rest into config.
		{"alice\n[Peer]\nPublicKey = EVIL", "alice Peer PublicKey = EVIL"},
		// Brackets would let a name forge an ID tag.
		{"alice [id:other]", "alice id:other"},
		{`quote" name`, "quote name"},
		{strings.Repeat("x", 100), strings.Repeat("x", MaxNameRunes)},
	}
	for _, c := range cases {
		if got := SanitizeName(c.in, "fb"); got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The marker a sanitised name produces must survive a round trip, or a
// crafted name could still detach a peer block from its client.
func TestSanitizedNameCannotForgeAMarker(t *testing.T) {
	name := SanitizeName("evil [id:victim]", "client")
	line := PeerMarker(name, "mine12")

	gotName, gotID, ok := ParsePeerMarker(line)
	if !ok || gotID != "mine12" || gotName != name {
		t.Fatalf("ParsePeerMarker(%q) = (%q, %q, %v)", line, gotName, gotID, ok)
	}
	if MarkerMatches(line, &api.Client{ID: "victim", Name: name}) {
		t.Errorf("marker %q matched the forged ID", line)
	}
}

func TestRetagPeerTextChangesOnlyTheMarker(t *testing.T) {
	client := &api.Client{ID: "abc123", Name: "renamed"}
	before := "[Interface]\nPrivateKey = secret\n\n# Client: old [id:abc123]\n[Peer]\nPublicKey = public\nAllowedIPs = 10.0.0.2/32\n\n# Client: other [id:def456]\n[Peer]\nPublicKey = other\n"
	want := strings.Replace(before, "# Client: old [id:abc123]", "# Client: renamed [id:abc123]", 1)
	got, found := RetagPeerText(before, client)
	if !found {
		t.Fatal("peer marker was not found")
	}
	if got != want {
		t.Fatalf("retag changed more than the marker:\n%s", got)
	}
}

func TestUntaggedAdoptedPeerIsFoundByPublicKey(t *testing.T) {
	conf := []string{
		"[Interface]", "PrivateKey = secret", "",
		"# externally managed", "[Peer]", "PublicKey = FIRST", "AllowedIPs = 10.0.0.2/32", "",
		"[Peer]", "PublicKey = SECOND", "AllowedIPs = 10.0.0.3/32",
	}
	rest, block, found := SplitPeerBlock(conf, &api.Client{ID: "stable", Name: "peer", ClientPublicKey: "FIRST"})
	if !found {
		t.Fatal("adopted peer was not found by public key")
	}
	if !strings.Contains(strings.Join(block, "\n"), "PublicKey = FIRST") {
		t.Fatalf("wrong block: %v", block)
	}
	joined := strings.Join(rest, "\n")
	if strings.Contains(joined, "FIRST") || !strings.Contains(joined, "SECOND") {
		t.Fatalf("removal crossed a peer boundary:\n%s", joined)
	}

	retagged := RetagPeerBlock(block, &api.Client{ID: "stable", Name: "adopted"})
	if retagged[0] != PeerMarker("adopted", "stable") {
		t.Fatalf("untagged block was not tagged: %v", retagged)
	}
}

// Looking up by public key must end at the next [Peer] section even when that
// section has no DocUI marker. This is a realistic adopted config: otherwise
// deleting or suspending the first peer could remove an unrelated host peer.
func TestSplitTaggedPeerDoesNotSwallowFollowingUntaggedPeer(t *testing.T) {
	conf := []string{
		"[Interface]", "PrivateKey = secret", "",
		"# Client: managed [id:stable]", "[Peer]", "PublicKey = MANAGED", "AllowedIPs = 10.0.0.2/32", "",
		"# external peer", "[Peer]", "PublicKey = EXTERNAL", "AllowedIPs = 10.0.0.3/32",
	}

	rest, block, found := SplitPeerBlock(conf, &api.Client{ID: "stable", Name: "managed", ClientPublicKey: "MANAGED"})
	if !found {
		t.Fatal("managed peer was not found")
	}
	if strings.Contains(strings.Join(block, "\n"), "EXTERNAL") {
		t.Fatalf("following untagged peer was included in the block: %v", block)
	}
	joined := strings.Join(rest, "\n")
	if strings.Contains(joined, "MANAGED") || !strings.Contains(joined, "EXTERNAL") {
		t.Fatalf("wrong peer was removed:\n%s", joined)
	}
}
