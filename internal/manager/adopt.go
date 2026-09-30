package manager

import (
	"crypto/sha256"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
	"github.com/Bahonio/amneziawg-docui/internal/api"
	"github.com/Bahonio/amneziawg-docui/internal/wgconf"
)

// adoptExisting imports metadata from host-owned configs without writing the
// configs or changing a running interface. Known clients keep their private
// material; externally added peers are represented read-only until the
// operator replaces them with a UI-created client.
func (m *Manager) adoptExisting() error {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()

	interfaces, err := m.tools.Interfaces()
	if err != nil || len(interfaces) == 0 {
		return nil
	}
	type candidate struct {
		info    agentapi.Interface
		content string
	}
	candidates := make([]candidate, 0, len(interfaces))
	for _, iface := range interfaces {
		content, err := m.tools.ReadConfig(iface.Name, iface.ConfigPath)
		if err == nil && content != "" {
			candidates = append(candidates, candidate{info: iface, content: content})
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	original := cloneConfig(m.cfg)
	changed := false
	for _, item := range candidates {
		srv := m.serverByInterface(item.info.Name)
		if srv == nil {
			adopted := adoptedServer(item.info, item.content, m.settings.DNSServers)
			m.cfg.Servers = append(m.cfg.Servers, adopted)
			changed = true
		} else {
			if refreshServerFromHost(srv, item.info, item.content) {
				changed = true
			}
			if mergeHostPeers(srv, item.content) {
				changed = true
			}
		}
	}
	if changed {
		if err := m.saveLocked("existing host interface adoption"); err != nil {
			m.cfg = original
			return err
		}
	}
	return nil
}

func refreshServerFromHost(srv *api.Server, info agentapi.Interface, content string) bool {
	before := fmt.Sprintf("%d|%s|%s|%d|%s|%t|%#v", srv.Port, srv.ServerIP, srv.Subnet, srv.MTU,
		srv.ServerPublicKey, srv.AutoStart, srv.ObfuscationParams)
	values := interfaceValues(content)
	serverIP, subnet := addressAndSubnet(firstCSV(values["address"]))
	port, _ := strconv.Atoi(values["listenport"])
	mtu, _ := strconv.Atoi(values["mtu"])
	if port > 0 {
		srv.Port = port
	}
	if serverIP != "" {
		srv.ServerIP = serverIP
	}
	if subnet != "" {
		srv.Subnet = subnet
	}
	if mtu > 0 {
		srv.MTU = mtu
	}
	if info.PublicKey != "" {
		srv.ServerPublicKey = info.PublicKey
	}
	srv.ConfigPath = info.ConfigPath
	srv.AutoStart = info.Enabled
	srv.ObfuscationParams = parsedObfuscation(values, srv.MTU)
	srv.ObfuscationEnabled = srv.ObfuscationParams != nil
	after := fmt.Sprintf("%d|%s|%s|%d|%s|%t|%#v", srv.Port, srv.ServerIP, srv.Subnet, srv.MTU,
		srv.ServerPublicKey, srv.AutoStart, srv.ObfuscationParams)
	return before != after
}

func (m *Manager) serverByInterface(name string) *api.Server {
	for i := range m.cfg.Servers {
		if m.cfg.Servers[i].Interface == name {
			return &m.cfg.Servers[i]
		}
	}
	return nil
}

func adoptedServer(info agentapi.Interface, content string, dns []string) api.Server {
	values := interfaceValues(content)
	address := firstCSV(values["address"])
	serverIP, subnet := addressAndSubnet(address)
	port, _ := strconv.Atoi(values["listenport"])
	mtu, _ := strconv.Atoi(values["mtu"])
	if mtu == 0 {
		mtu = 1280
	}
	obf := parsedObfuscation(values, mtu)
	id := stableID("server:" + info.Name)
	srv := api.Server{
		ID: id, Name: info.Name, Protocol: "wireguard", Port: port,
		Status: "stopped", Interface: info.Name, ConfigPath: info.ConfigPath,
		ServerPublicKey: info.PublicKey, Subnet: subnet, ServerIP: serverIP, MTU: mtu,
		ObfuscationEnabled: obf != nil, ObfuscationParams: obf, AutoStart: info.Enabled,
		DNS: append([]string{}, dns...), Clients: []api.Client{}, UnboundNATIPs: []string{},
		CreatedAt: float64(time.Now().Unix()), AdoptedFromHost: true,
	}
	mergeHostPeers(&srv, content)
	return srv
}

func interfaceValues(content string) map[string]string {
	values := map[string]string{}
	section := ""
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != "interface" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	return values
}

func parsedObfuscation(v map[string]string, mtu int) *api.ObfuscationParams {
	if v["jc"] == "" && v["headerprotectionkey"] == "" {
		return nil
	}
	num := func(key string) int { n, _ := strconv.Atoi(v[strings.ToLower(key)]); return n }
	on := func(key string) bool {
		value := strings.ToLower(v[strings.ToLower(key)])
		return value == "on" || value == "1" || value == "true"
	}
	return &api.ObfuscationParams{
		Jc: num("Jc"), Jmin: num("Jmin"), Jmax: num("Jmax"),
		S1: num("S1"), S2: num("S2"), S3: num("S3"), S4: num("S4"),
		H1: num("H1"), H2: num("H2"), H3: num("H3"), H4: num("H4"), MTU: mtu,
		HeaderProtectionKey: v["headerprotectionkey"], ContentPaddingAddition: v["contentpaddingaddition"],
		RekeyAfterTime: v["rekeyaftertime"], RekeyTimeout: v["rekeytimeout"], RejectAfterTime: v["rejectaftertime"],
		KeepaliveTimeout: v["keepalivetimeout"], MaxHandshakeAttempts: v["maxhandshakeattempts"],
		PersistentKeepalive: v["persistentkeepalive"], RandomTrailers: on("RandomTrailers"), DisableCookies: on("DisableCookies"),
	}
}

func mergeHostPeers(srv *api.Server, content string) bool {
	known := map[string]bool{}
	present := map[string]bool{}
	for _, client := range srv.Clients {
		known[client.ClientPublicKey] = true
	}
	changed := false
	section := ""
	name, markerID := "", ""
	pendingName, pendingID := "", ""
	peer := map[string]string{}
	flush := func() {
		public := peer["publickey"]
		if public == "" {
			peer = map[string]string{}
			return
		}
		present[public] = true
		if known[public] {
			peer = map[string]string{}
			return
		}
		id := markerID
		if id == "" {
			id = stableID("peer:" + public)
		}
		if name == "" {
			name = "peer-" + id
		}
		ip := firstCSV(peer["allowedips"])
		ip = strings.TrimSpace(strings.SplitN(ip, "/", 2)[0])
		srv.Clients = append(srv.Clients, api.Client{
			ID: id, Name: name, ServerID: srv.ID, ServerName: srv.Name, Status: "active",
			CreatedAt: float64(time.Now().Unix()), ClientPublicKey: public, PresharedKey: peer["presharedkey"],
			ClientIP: ip, ObfuscationEnabled: srv.ObfuscationEnabled,
			ObfuscationParams: cloneObfuscationParams(srv.ObfuscationParams), ISettings: map[string]string{},
			AllowedIPs: wgconf.DefaultAllowedIPs,
		})
		known[public], changed = true, true
		peer = map[string]string{}
	}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if parsedName, parsedID, ok := wgconf.ParsePeerMarker(line); ok {
			pendingName, pendingID = parsedName, parsedID
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if section == "peer" {
				flush()
			}
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "peer" {
				peer, name, markerID = map[string]string{}, pendingName, pendingID
				pendingName, pendingID = "", ""
			}
			continue
		}
		if section == "peer" {
			key, value, ok := strings.Cut(line, "=")
			if ok {
				peer[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
			}
		}
	}
	if section == "peer" {
		flush()
	}
	for i := range srv.Clients {
		client := &srv.Clients[i]
		if present[client.ClientPublicKey] && client.Status == "suspended" {
			client.Status = "active"
			changed = true
		} else if !present[client.ClientPublicKey] && client.Status == "active" {
			client.Status = "suspended"
			changed = true
		}
	}
	return changed
}

func addressAndSubnet(address string) (string, string) {
	ip, network, err := net.ParseCIDR(strings.TrimSpace(address))
	if err != nil {
		return "", ""
	}
	return ip.String(), network.String()
}

func firstCSV(value string) string {
	if head, _, ok := strings.Cut(value, ","); ok {
		return strings.TrimSpace(head)
	}
	return strings.TrimSpace(value)
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:3])
}
