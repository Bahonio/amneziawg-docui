package hostagent

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// These are the interface settings the panel may change. Routing, DNS and
// command hooks belong to the host administrator, including unknown future
// directives. Apply may preserve them, but cannot introduce or change them.
func panelInterfaceKey(key string) bool {
	switch key {
	case "privatekey", "listenport", "mtu",
		"jc", "jmin", "jmax", "s1", "s2", "s3", "s4", "h1", "h2", "h3", "h4",
		"i1", "i2", "i3", "i4", "i5", "headerprotectionkey", "contentpaddingaddition",
		"rekeyaftertime", "rekeytimeout", "rejectaftertime", "keepalivetimeout",
		"maxhandshakeattempts", "randomtrailers", "disablecookies":
		return true
	}
	return false
}

type policySection struct {
	name   string
	values map[string][]string
}

func policySections(content string) []policySection {
	var sections []policySection
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, policySection{
				name:   strings.ToLower(strings.TrimSpace(line[1 : len(line)-1])),
				values: map[string][]string{},
			})
			continue
		}
		key, value, ok := cutDirective(line)
		if ok && len(sections) > 0 {
			values := sections[len(sections)-1].values
			key = strings.ToLower(key)
			values[key] = append(values[key], value)
		}
	}
	return sections
}

func validateNetworkPolicy(content, previous string) error {
	var oldInterface map[string][]string
	oldPeers := map[string]map[string][]string{}
	for _, section := range policySections(previous) {
		if section.name == "interface" {
			oldInterface = section.values
		} else if keys := section.values["publickey"]; len(keys) == 1 {
			oldPeers[keys[0]] = section.values
		}
	}
	sections := policySections(content)
	var networks []netip.Prefix
	for _, section := range sections {
		if section.name != "interface" {
			continue
		}
		for key, values := range section.values {
			if panelInterfaceKey(key) {
				if len(values) != 1 {
					return fmt.Errorf("invalid duplicate interface directive %s", key)
				}
				continue
			}
			if previous == "" && key == "address" {
				continue
			}
			if key == "saveconfig" && len(values) == 1 && strings.EqualFold(values[0], "false") && len(oldInterface[key]) == 0 {
				continue
			}
			if !slices.Equal(values, oldInterface[key]) {
				return fmt.Errorf("interface directive %s cannot be changed through the API", key)
			}
		}
		for key, values := range oldInterface {
			if !panelInterfaceKey(key) && !slices.Equal(values, section.values[key]) {
				return fmt.Errorf("interface directive %s cannot be removed through the API", key)
			}
		}
		for _, value := range section.values["address"] {
			for _, raw := range strings.Split(value, ",") {
				prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
				if err != nil {
					return fmt.Errorf("invalid interface address %q", raw)
				}
				networks = append(networks, prefix)
			}
		}
	}
	for _, section := range sections {
		if section.name != "peer" {
			continue
		}
		values := section.values
		// A host-configured route is retained only with the entire peer's
		// directives unchanged, so its key or endpoint cannot be reassigned.
		if keys := values["publickey"]; len(keys) == 1 && equalDirectives(values, oldPeers[keys[0]]) {
			continue
		}
		for key, entries := range values {
			if len(entries) != 1 {
				return fmt.Errorf("invalid duplicate peer directive %s", key)
			}
			switch key {
			case "publickey", "presharedkey", "allowedips", "endpoint", "persistentkeepalive":
			default:
				return fmt.Errorf("unsupported peer directive %s", key)
			}
		}
		for _, entry := range values["allowedips"] {
			for _, raw := range strings.Split(entry, ",") {
				prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
				allowed := false
				if err == nil && prefix.Bits() == prefix.Addr().BitLen() {
					for _, network := range networks {
						if network.Contains(prefix.Addr()) && network.Addr() != prefix.Addr() {
							allowed = true
						}
					}
				}
				if !allowed {
					return fmt.Errorf("invalid peer route %q: new routes must be individual addresses within the VPN subnet", raw)
				}
			}
		}
	}
	return nil
}

func equalDirectives(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, values := range a {
		if !slices.Equal(values, b[key]) {
			return false
		}
	}
	return true
}
