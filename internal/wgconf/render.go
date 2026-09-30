// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package wgconf

import (
	"fmt"
	"strings"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/api"
)

// DefaultAllowedIPs routes everything through the tunnel.
const DefaultAllowedIPs = "0.0.0.0/0, ::/0"

// DefaultPersistentKeepalive keeps a NAT mapping alive from the client side.
const DefaultPersistentKeepalive = "25"

// ServerInterface is what a new server's .conf starts as: the [Interface]
// section alone, peers get appended as clients are added.
type ServerInterface struct {
	PrivateKey string
	Address    string // "10.0.0.1/24"
	ListenPort int
	MTU        int

	// Obfuscation is nil when the server runs plain WireGuard.
	Obfuscation *api.ObfuscationParams
}

// Render produces the [Interface] section.
func (s ServerInterface) Render() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[Interface]\n")
	fmt.Fprintf(&sb, "PrivateKey = %s\n", s.PrivateKey)
	fmt.Fprintf(&sb, "Address = %s\n", s.Address)
	fmt.Fprintf(&sb, "ListenPort = %d\n", s.ListenPort)
	fmt.Fprintf(&sb, "SaveConfig = false\n")
	fmt.Fprintf(&sb, "MTU = %d\n", s.MTU)
	writeObfuscation(&sb, s.Obfuscation)
	return sb.String()
}

// ClientConf renders the .conf a client imports. endpoint is the host the
// client connects to; the caller resolves it, since the fallback chain ends
// at the backend's detected public IP.
func ClientConf(srv *api.Server, client *api.Client, endpoint string, includeComments bool) string {
	var sb strings.Builder

	if includeComments {
		fmt.Fprintf(&sb, "# AmneziaWG Client Configuration\n")
		fmt.Fprintf(&sb, "# Server: %s\n", srv.Name)
		fmt.Fprintf(&sb, "# Client: %s\n", client.Name)
		fmt.Fprintf(&sb, "# Generated: %s\n", time.Unix(int64(client.CreatedAt), 0).UTC().String())
		fmt.Fprintf(&sb, "# Server Endpoint: %s:%d\n", endpoint, srv.Port)
	}

	fmt.Fprintf(&sb, "[Interface]\n")
	fmt.Fprintf(&sb, "PrivateKey = %s\n", client.ClientPrivateKey)
	fmt.Fprintf(&sb, "Address = %s/32\n", client.ClientIP)
	fmt.Fprintf(&sb, "DNS = %s\n", strings.Join(srv.DNS, ", "))
	fmt.Fprintf(&sb, "MTU = %d\n", srv.MTU)

	if client.ObfuscationEnabled {
		writeObfuscation(&sb, client.ObfuscationParams)
	}

	// I1 is the signature packet the rest hang off: without it the others
	// are not sent at all, so they are not written either.
	if client.ApplyISettings && client.ISettings["i1"] != "" {
		for n := 1; n <= 5; n++ {
			if v := client.ISettings[fmt.Sprintf("i%d", n)]; v != "" {
				fmt.Fprintf(&sb, "I%d = %s\n", n, v)
			}
		}
	}

	fmt.Fprintf(&sb, "\n[Peer]\n")
	fmt.Fprintf(&sb, "PublicKey = %s\n", srv.ServerPublicKey)
	fmt.Fprintf(&sb, "PresharedKey = %s\n", client.PresharedKey)
	fmt.Fprintf(&sb, "Endpoint = %s:%d\n", endpoint, srv.Port)
	fmt.Fprintf(&sb, "AllowedIPs = %s\n", AllowedIPsOrDefault(client.AllowedIPs))
	fmt.Fprintf(&sb, "PersistentKeepalive = %s\n", PersistentKeepalive(client.ObfuscationParams))

	return sb.String()
}

// AllowedIPsOrDefault substitutes the full-tunnel default for a blank value.
func AllowedIPsOrDefault(allowedIPs string) string {
	if strings.TrimSpace(allowedIPs) == "" {
		return DefaultAllowedIPs
	}
	return strings.TrimSpace(allowedIPs)
}

// PersistentKeepalive is the client's keepalive, from its obfuscation
// parameters when set there.
func PersistentKeepalive(p *api.ObfuscationParams) string {
	if p != nil && p.PersistentKeepalive != "" {
		return p.PersistentKeepalive
	}
	return DefaultPersistentKeepalive
}

// writeObfuscation writes the AmneziaWG parameter set, the same lines for the
// server's [Interface] and the client's: both ends must agree on every one.
// A nil set writes nothing, which is plain WireGuard.
func writeObfuscation(sb *strings.Builder, p *api.ObfuscationParams) {
	if p == nil {
		return
	}
	fmt.Fprintf(sb, "Jc = %d\n", p.Jc)
	fmt.Fprintf(sb, "Jmin = %d\n", p.Jmin)
	fmt.Fprintf(sb, "Jmax = %d\n", p.Jmax)
	fmt.Fprintf(sb, "S1 = %d\n", p.S1)
	fmt.Fprintf(sb, "S2 = %d\n", p.S2)
	fmt.Fprintf(sb, "S3 = %d\n", p.S3)
	fmt.Fprintf(sb, "S4 = %d\n", p.S4)
	fmt.Fprintf(sb, "H1 = %d\n", p.H1)
	fmt.Fprintf(sb, "H2 = %d\n", p.H2)
	fmt.Fprintf(sb, "H3 = %d\n", p.H3)
	fmt.Fprintf(sb, "H4 = %d\n", p.H4)
	writeIfSet(sb, "HeaderProtectionKey", p.HeaderProtectionKey)
	writeIfSet(sb, "ContentPaddingAddition", p.ContentPaddingAddition)
	writeIfSet(sb, "RekeyAfterTime", p.RekeyAfterTime)
	writeIfSet(sb, "RekeyTimeout", p.RekeyTimeout)
	writeIfSet(sb, "RejectAfterTime", p.RejectAfterTime)
	writeIfSet(sb, "KeepaliveTimeout", p.KeepaliveTimeout)
	writeIfSet(sb, "MaxHandshakeAttempts", p.MaxHandshakeAttempts)
	writeAwgBoolIfOn(sb, "RandomTrailers", p.RandomTrailers)
	writeAwgBoolIfOn(sb, "DisableCookies", p.DisableCookies)
}

// writeIfSet writes "key = value\n" to sb only if value is non-empty.
func writeIfSet(sb *strings.Builder, key, value string) {
	if value != "" {
		fmt.Fprintf(sb, "%s = %s\n", key, value)
	}
}

// writeAwgBoolIfOn writes an AmneziaWG 3.1 "key = on" line, and writes
// nothing when the flag is off. amneziawg-tools accepts on/off and 0/1, but
// an omitted key is the only form every pre-3.1 parser tolerates - clients
// still on AmneziaWG 3.0 reject an unknown key outright and would fail to
// import the config, so "off" is expressed by silence.
func writeAwgBoolIfOn(sb *strings.Builder, key string, enabled bool) {
	if enabled {
		fmt.Fprintf(sb, "%s = on\n", key)
	}
}
