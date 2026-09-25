// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package wgconf

import (
	"strings"

	"github.com/Bahonio/awg-docui/internal/api"
)

// AppendPeerText is the pure form used before sending an atomic update to the
// host agent.
func AppendPeerText(content string, client *api.Client, allowedIPs string) string {
	return strings.TrimRight(content, "\n") + PeerBlock(client, allowedIPs)
}

// RemovePeerText removes a peer block without touching the filesystem.
func RemovePeerText(content string, client *api.Client) (string, []string, bool) {
	rest, block, found := SplitPeerBlock(strings.Split(content, "\n"), client)
	if !found {
		return content, nil, false
	}
	return strings.Join(rest, "\n") + "\n", block, true
}

// RetagPeerText updates only the comment that identifies an existing peer.
// The cryptographic and routing fields in the block remain byte-for-byte
// unchanged.
func RetagPeerText(content string, client *api.Client) (string, bool) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if MarkerMatches(line, client) {
			lines[i] = PeerMarker(client.Name, client.ID)
			return strings.Join(lines, "\n"), true
		}
	}
	start, end := peerBlockBounds(lines, client.ClientPublicKey)
	if start >= 0 {
		block := RetagPeerBlock(lines[start:end], client)
		lines = append(append(append([]string{}, lines[:start]...), block...), lines[end:]...)
		return strings.Join(lines, "\n"), true
	}
	return content, false
}
