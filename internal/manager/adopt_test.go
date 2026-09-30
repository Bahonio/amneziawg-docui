package manager

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
	"github.com/Bahonio/amneziawg-docui/internal/api"
)

func adoptionKey(fill byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = fill
	}
	return base64.StdEncoding.EncodeToString(b)
}

func TestAdoptExistingConfigPreservesValuesWithoutMutation(t *testing.T) {
	content := "[Interface]\n" +
		"PrivateKey = " + adoptionKey(1) + "\n" +
		"Address = 10.17.0.1/24\nListenPort = 42000\nMTU = 1380\n" +
		"Jc = 7\nJmin = 40\nJmax = 90\nS1 = 64\nS2 = 80\nS3 = 96\nS4 = 112\n" +
		"H1 = 11\nH2 = 22\nH3 = 33\nH4 = 44\nHeaderProtectionKey = " + adoptionKey(2) + "\n" +
		"ContentPaddingAddition = 4-12\nRandomTrailers = on\nDisableCookies = on\n\n" +
		"# Client: existing phone [id:phone1]\n[Peer]\nPublicKey = " + adoptionKey(3) + "\n" +
		"PresharedKey = " + adoptionKey(4) + "\nAllowedIPs = 10.17.0.2/32\n"
	before := sha256.Sum256([]byte(content))
	srv := adoptedServer(agentapi.Interface{Name: "wg0", ConfigPath: "/etc/amnezia/amneziawg/wg0.conf", PublicKey: adoptionKey(5), Running: true, Enabled: true}, content, []string{"1.1.1.1"})
	after := sha256.Sum256([]byte(content))
	if before != after {
		t.Fatal("adoption changed source config")
	}
	if srv.Interface != "wg0" || srv.Port != 42000 || srv.Subnet != "10.17.0.0/24" || srv.ServerIP != "10.17.0.1" || srv.MTU != 1380 {
		t.Fatalf("server values = %+v", srv)
	}
	if srv.ObfuscationParams == nil || !srv.ObfuscationParams.RandomTrailers || !srv.ObfuscationParams.DisableCookies || srv.ObfuscationParams.H4 != 44 {
		t.Fatalf("obfuscation = %+v", srv.ObfuscationParams)
	}
	if len(srv.Clients) != 1 || srv.Clients[0].ID != "phone1" || srv.Clients[0].Name != "existing phone" || srv.Clients[0].ClientPrivateKey != "" {
		t.Fatalf("clients = %+v", srv.Clients)
	}
}

func TestAdoptionKeepsKnownClientSecretsAndAddsExternalPeer(t *testing.T) {
	known := adoptionKey(6)
	external := adoptionKey(7)
	srv := api.Server{ID: "server", Name: "wg0", Clients: []api.Client{{ID: "known", ClientPublicKey: known, ClientPrivateKey: "KEEP-ME"}}}
	content := "[Interface]\nPrivateKey = " + adoptionKey(1) + "\n\n[Peer]\nPublicKey = " + known + "\nAllowedIPs = 10.0.0.2/32\n\n[Peer]\nPublicKey = " + external + "\nAllowedIPs = 10.0.0.3/32\n"
	if !mergeHostPeers(&srv, content) {
		t.Fatal("external peer was not discovered")
	}
	if len(srv.Clients) != 2 || srv.Clients[0].ClientPrivateKey != "KEEP-ME" || srv.Clients[1].ClientPublicKey != external {
		t.Fatalf("clients = %+v", srv.Clients)
	}
}
