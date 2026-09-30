package hostagent

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
)

type Handler struct{ Service *Service }

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"+agentapi.Version), "/")
	parts := strings.Split(path, "/")
	if path == "health" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, h.Service.Health())
		return
	}
	if path == "interfaces" {
		switch r.Method {
		case http.MethodGet:
			out, err := h.Service.Interfaces(r.Context())
			h.reply(w, out, err)
		case http.MethodPost:
			var req agentapi.CreateInterfaceRequest
			if err := decodeJSON(r, &req); err != nil {
				h.reply(w, nil, err)
				return
			}
			h.reply(w, agentapi.ActionResult{Status: "created"}, h.Service.Create(r.Context(), req))
		default:
			h.methodNotAllowed(w)
		}
		return
	}
	if path == "keys" && r.Method == http.MethodPost {
		out, err := h.Service.GenerateKeys(r.Context())
		h.reply(w, out, err)
		return
	}
	if path == "preshared-key" && r.Method == http.MethodPost {
		key, err := h.Service.PresharedKey(r.Context())
		h.reply(w, agentapi.PresharedKey{Key: key}, err)
		return
	}
	if path == "public-key" && r.Method == http.MethodPost {
		var req agentapi.PublicKeyRequest
		if err := decodeJSON(r, &req); err != nil {
			h.reply(w, nil, err)
			return
		}
		key, err := h.Service.PublicKey(r.Context(), req.Private)
		h.reply(w, agentapi.PublicKeyResponse{Public: key}, err)
		return
	}
	if path == "route-source" && r.Method == http.MethodGet {
		address, err := h.Service.RouteSource(r.Context())
		h.reply(w, agentapi.RouteSource{Address: address}, err)
		return
	}
	if len(parts) >= 2 && parts[0] == "interfaces" {
		name, err := url.PathUnescape(parts[1])
		if err != nil || validateInterfaceName(name) != nil {
			h.reply(w, nil, errors.New("invalid interface name"))
			return
		}
		h.interfaceRoute(w, r, name, parts[2:])
		return
	}
	writeJSON(w, http.StatusNotFound, agentapi.ErrorResponse{Error: "operation not found"})
}

func (h Handler) interfaceRoute(w http.ResponseWriter, r *http.Request, name string, tail []string) {
	if len(tail) == 0 {
		switch r.Method {
		case http.MethodGet:
			out, err := h.Service.Interface(r.Context(), name, true)
			h.reply(w, out, err)
		case http.MethodDelete:
			h.reply(w, agentapi.ActionResult{Status: "deleted"}, h.Service.Delete(r.Context(), name))
		default:
			h.methodNotAllowed(w)
		}
		return
	}
	if len(tail) != 1 {
		writeJSON(w, http.StatusNotFound, agentapi.ErrorResponse{Error: "operation not found"})
		return
	}
	switch tail[0] {
	case "start", "stop", "restart":
		if r.Method != http.MethodPost {
			h.methodNotAllowed(w)
			return
		}
		h.reply(w, agentapi.ActionResult{Status: tail[0]}, h.Service.Action(r.Context(), name, tail[0]))
	case "config":
		if r.Method != http.MethodPut {
			h.methodNotAllowed(w)
			return
		}
		var req agentapi.ApplyConfigRequest
		if err := decodeJSON(r, &req); err != nil {
			h.reply(w, nil, err)
			return
		}
		h.reply(w, agentapi.ActionResult{Status: "applied"}, h.Service.Apply(r.Context(), name, req.Config, req.ApplyLive))
	case "stats":
		if r.Method != http.MethodGet {
			h.methodNotAllowed(w)
			return
		}
		out, err := h.Service.Stats(r.Context(), name)
		h.reply(w, out, err)
	case "firewall":
		if r.Method != http.MethodGet {
			h.methodNotAllowed(w)
			return
		}
		h.reply(w, agentapi.FirewallStatus{Checks: h.Service.Firewall(r.Context(), name, r.URL.Query().Get("subnet"))}, nil)
	case "peers":
		h.peerRoute(w, r, name)
	default:
		writeJSON(w, http.StatusNotFound, agentapi.ErrorResponse{Error: "operation not found"})
	}
}

func (h Handler) peerRoute(w http.ResponseWriter, r *http.Request, name string) {
	var err error
	switch r.Method {
	case http.MethodPost:
		var req agentapi.Peer
		if err = decodeJSON(r, &req); err == nil {
			err = h.Service.AddPeer(r.Context(), name, req)
		}
	case http.MethodPut:
		var req agentapi.UpdatePeerRequest
		if err = decodeJSON(r, &req); err == nil {
			err = h.Service.UpdatePeer(r.Context(), name, req)
		}
	case http.MethodDelete:
		var req agentapi.DeletePeerRequest
		if err = decodeJSON(r, &req); err == nil {
			err = h.Service.DeletePeer(r.Context(), name, req.PublicKey)
		}
	default:
		h.methodNotAllowed(w)
		return
	}
	h.reply(w, agentapi.ActionResult{Status: "applied"}, err)
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxConfigBytes+64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return errors.New("malformed request: " + err.Error())
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("malformed request: multiple JSON values")
	}
	return nil
}

func (h Handler) reply(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		message := err.Error()
		if errors.Is(err, os.ErrNotExist) {
			message = "interface or config not found"
		}
		writeJSON(w, classify(err), agentapi.ErrorResponse{Error: message})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h Handler) methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, agentapi.ErrorResponse{Error: "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
