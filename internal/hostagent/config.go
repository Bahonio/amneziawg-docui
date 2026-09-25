package hostagent

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/Bahonio/awg-docui/internal/agentapi"
)

const maxConfigBytes = 1 << 20

var interfaceNameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,14}$`)

func validateInterfaceName(name string) error {
	if !interfaceNameRE.MatchString(name) || name == "." || name == ".." {
		return fmt.Errorf("invalid interface name %q", name)
	}
	return nil
}

func validateConfig(content string) error {
	return validateConfigWithHooks(content, false)
}

func validateConfigWithHooks(content string, allowHooks bool) error {
	if len(content) == 0 || len(content) > maxConfigBytes {
		return fmt.Errorf("config size must be between 1 and %d bytes", maxConfigBytes)
	}
	if strings.ContainsRune(content, 0) {
		return errors.New("config contains a NUL byte")
	}
	section := ""
	interfaceCount := 0
	for number, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section != "interface" && section != "peer" {
				return fmt.Errorf("line %d: unsupported section", number+1)
			}
			if section == "interface" {
				interfaceCount++
			}
			continue
		}
		key, value, ok := cutDirective(line)
		if !ok || section == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("line %d: malformed directive", number+1)
		}
		lower := strings.ToLower(key)
		if !allowHooks && (lower == "preup" || lower == "postup" || lower == "predown" || lower == "postdown") {
			return fmt.Errorf("line %d: command hooks are managed by the host and cannot be supplied through the API", number+1)
		}
	}
	if interfaceCount == 0 {
		return errors.New("config has no [Interface] section")
	}
	if interfaceCount != 1 {
		return errors.New("config must contain exactly one [Interface] section")
	}
	return validateKnownConfig(content)
}

func validateKnownConfig(content string) error {
	if err := validateKey("interface private key", configValue(content, "interface", "PrivateKey"), false); err != nil {
		return err
	}
	address := configValue(content, "interface", "Address")
	if address == "" {
		return errors.New("interface address is required")
	}
	for _, raw := range strings.Split(address, ",") {
		if _, err := netip.ParsePrefix(strings.TrimSpace(raw)); err != nil {
			return fmt.Errorf("invalid interface address %q", strings.TrimSpace(raw))
		}
	}
	port := configPort(content)
	if port < 1 || port > 65535 {
		return errors.New("invalid or missing ListenPort (must be 1-65535)")
	}
	if raw := configValue(content, "interface", "MTU"); raw != "" {
		mtu, err := strconv.Atoi(raw)
		if err != nil || mtu < 576 || mtu > 65535 {
			return fmt.Errorf("invalid MTU %q", raw)
		}
	}
	if raw := strings.ToLower(configValue(content, "interface", "SaveConfig")); raw != "" && raw != "true" && raw != "false" {
		return fmt.Errorf("invalid SaveConfig %q", raw)
	}
	if raw := strings.ToLower(configValue(content, "interface", "Table")); raw != "" && raw != "auto" && raw != "off" {
		if table, err := strconv.ParseUint(raw, 10, 32); err != nil || table == 0 {
			return fmt.Errorf("invalid Table %q", raw)
		}
	}
	return validateConfigPeers(content)
}

func validateConfigPeers(content string) error {
	section := ""
	peer := agentapi.Peer{}
	peerSeen := false
	flush := func() error {
		if !peerSeen {
			return nil
		}
		if err := validatePeer(peer); err != nil {
			return fmt.Errorf("invalid peer: %w", err)
		}
		peer, peerSeen = agentapi.Peer{}, false
		return nil
	}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if section == "peer" {
				if err := flush(); err != nil {
					return err
				}
			}
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			peerSeen = section == "peer"
			continue
		}
		if section != "peer" || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := cutDirective(line)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "publickey":
			peer.PublicKey = value
		case "presharedkey":
			peer.PresharedKey = value
		case "allowedips":
			peer.AllowedIPs = value
		}
	}
	if section == "peer" {
		return flush()
	}
	return nil
}

func addManagedFirewallHooks(content, subnet string) string {
	postUp := "PostUp = iptables -A INPUT -i %i -j ACCEPT; iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -A POSTROUTING -s " + subnet + " -j MASQUERADE"
	postDown := "PostDown = iptables -t nat -D POSTROUTING -s " + subnet + " -j MASQUERADE; iptables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -D FORWARD -i %i -j ACCEPT; iptables -D INPUT -i %i -j ACCEPT"
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	insert := len(lines)
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "[Peer]") {
			insert = i
			break
		}
	}
	out := append([]string{}, lines[:insert]...)
	out = append(out, postUp, postDown)
	out = append(out, lines[insert:]...)
	return strings.Join(out, "\n") + "\n"
}

func cutDirective(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, "=")
	return strings.TrimSpace(key), strings.TrimSpace(value), ok
}

func validateSubnet(subnet string) error {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(subnet))
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 1 || prefix.Bits() > 32 {
		return fmt.Errorf("invalid IPv4 subnet %q", subnet)
	}
	return nil
}

func validateKey(name, key string, optional bool) error {
	if optional && strings.TrimSpace(key) == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(key))
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("invalid %s", name)
	}
	return nil
}

func validatePeer(peer agentapi.Peer) error {
	if err := validateKey("public key", peer.PublicKey, false); err != nil {
		return err
	}
	if err := validateKey("preshared key", peer.PresharedKey, true); err != nil {
		return err
	}
	if strings.TrimSpace(peer.AllowedIPs) == "" {
		return errors.New("allowed IPs are required")
	}
	for _, raw := range strings.Split(peer.AllowedIPs, ",") {
		if _, err := netip.ParsePrefix(strings.TrimSpace(raw)); err != nil {
			return fmt.Errorf("invalid allowed IP %q", strings.TrimSpace(raw))
		}
	}
	return nil
}

func configValue(content, sectionName, wanted string) string {
	section := ""
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != strings.ToLower(sectionName) {
			continue
		}
		key, value, ok := cutDirective(line)
		if ok && strings.EqualFold(key, wanted) {
			return value
		}
	}
	return ""
}

func configPort(content string) int {
	n, _ := strconv.Atoi(configValue(content, "interface", "ListenPort"))
	return n
}

// replacePeer edits only [Peer] directives. It never interprets comments or
// permits interface command hooks.
func replacePeer(content, original string, replacement *agentapi.Peer) (string, bool) {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "[Peer]") {
			if start >= 0 {
				end = i
				break
			}
			for j := i + 1; j < len(lines); j++ {
				key, value, ok := cutDirective(strings.TrimSpace(lines[j]))
				if ok && strings.EqualFold(key, "PublicKey") && strings.TrimSpace(value) == strings.TrimSpace(original) {
					start = i
					break
				}
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "[") {
					break
				}
			}
		}
	}
	if start < 0 {
		return content, false
	}
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "#") {
		start--
	}
	var block []string
	if replacement != nil {
		block = []string{"[Peer]", "PublicKey = " + replacement.PublicKey}
		if replacement.PresharedKey != "" {
			block = append(block, "PresharedKey = "+replacement.PresharedKey)
		}
		block = append(block, "AllowedIPs = "+replacement.AllowedIPs)
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, block...)
	out = append(out, lines[end:]...)
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n", true
}

func appendPeer(content string, peer agentapi.Peer) string {
	block := "\n[Peer]\nPublicKey = " + peer.PublicKey + "\n"
	if peer.PresharedKey != "" {
		block += "PresharedKey = " + peer.PresharedKey + "\n"
	}
	block += "AllowedIPs = " + peer.AllowedIPs + "\n"
	return strings.TrimRight(content, "\n") + "\n" + block
}
