// Package agentapi contains the versioned wire contract shared by the
// management-only web container and the privileged host agent.
package agentapi

const (
	Version    = "v1"
	SocketPath = "/run/awg-docui/agent.sock"
	Runtime    = "Kernel"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type BackendStatus struct {
	HostAgent             bool   `json:"host_agent"`
	KernelModuleInstalled bool   `json:"kernel_module_installed"`
	KernelModuleLoaded    bool   `json:"kernel_module_loaded"`
	AWGAvailable          bool   `json:"awg_available"`
	AWGQuickAvailable     bool   `json:"awg_quick_available"`
	Runtime               string `json:"vpn_runtime"`
	Message               string `json:"message,omitempty"`
}

func (s BackendStatus) Ready() bool {
	return s.HostAgent && s.KernelModuleInstalled && s.KernelModuleLoaded && s.AWGAvailable && s.AWGQuickAvailable
}

type Interface struct {
	Name       string `json:"name"`
	ConfigPath string `json:"config_path"`
	Running    bool   `json:"running"`
	Enabled    bool   `json:"enabled"`
	PublicKey  string `json:"public_key,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
}

type InterfaceDetail struct {
	Interface
	Config string `json:"config"`
}

type CreateInterfaceRequest struct {
	Name   string `json:"name"`
	Config string `json:"config"`
	Subnet string `json:"subnet"`
	Enable bool   `json:"enable"`
	Start  bool   `json:"start"`
}

type ApplyConfigRequest struct {
	Config    string `json:"config"`
	ApplyLive bool   `json:"apply_live"`
}

type ActionResult struct {
	Status string `json:"status"`
}

type KeyPair struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

type PublicKeyRequest struct {
	Private string `json:"private"`
}

type PublicKeyResponse struct {
	Public string `json:"public"`
}

type PresharedKey struct {
	Key string `json:"key"`
}

type Peer struct {
	PublicKey    string `json:"public_key"`
	PresharedKey string `json:"preshared_key,omitempty"`
	AllowedIPs   string `json:"allowed_ips"`
}

type UpdatePeerRequest struct {
	OriginalPublicKey string `json:"original_public_key"`
	Peer
}

type DeletePeerRequest struct {
	PublicKey string `json:"public_key"`
}

type PeerStats struct {
	Received      string `json:"received"`
	Sent          string `json:"sent"`
	LastHandshake string `json:"last_handshake"`
	Endpoint      string `json:"endpoint"`
}

type InterfaceStats struct {
	Received string               `json:"received"`
	Sent     string               `json:"sent"`
	Peers    map[string]PeerStats `json:"peers"`
}

type RouteSource struct {
	Address string `json:"address"`
}

type FirewallStatus struct {
	Checks map[string]string `json:"checks"`
}
