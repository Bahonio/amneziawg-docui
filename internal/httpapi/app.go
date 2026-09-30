// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

// Package httpapi is the REST surface over the manager: routing, request
// decoding, and turning the manager's typed errors into status codes. It
// never touches the disk or the host itself.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/Bahonio/amneziawg-docui/internal/agentclient"
	"github.com/Bahonio/amneziawg-docui/internal/api"
	"github.com/Bahonio/amneziawg-docui/internal/manager"
)

// SecurityHeaders locks the embedded application to its own origin. Inline
// styles are permitted because the responsive UI sets a few layout values;
// executable code and network requests remain self-only.
func SecurityHeaders(c fiber.Ctx) error {
	c.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	return c.Next()
}

// SameOriginMutations rejects browser cross-origin writes while keeping
// Origin-less CLI and local health requests usable. This complements Basic
// Auth, whose credentials browsers otherwise attach to cross-site requests.
func SameOriginMutations(c fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		origin := c.Get("Origin")
		if origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host == "" || !strings.EqualFold(parsed.Host, c.Host()) {
				return respond(c, fiber.StatusForbidden, api.ErrorResponse{Error: "cross-origin management request rejected"})
			}
		}
	}
	return c.Next()
}

// Handlers holds dependencies for HTTP request handlers.
type Handlers struct {
	mgr *manager.Manager
}

func publicClient(client api.Client) api.Client {
	client.ClientPrivateKey = ""
	client.PresharedKey = ""
	return client
}

func publicClientPtr(client *api.Client) *api.Client {
	if client == nil {
		return nil
	}
	redacted := publicClient(*client)
	return &redacted
}

func publicClients(clients []api.Client) []api.Client {
	out := make([]api.Client, len(clients))
	for i := range clients {
		out[i] = publicClient(clients[i])
	}
	return out
}

func publicServers(servers []api.Server) []api.Server {
	for i := range servers {
		servers[i].Clients = publicClients(servers[i].Clients)
	}
	return servers
}

// New creates a Handlers instance.
func New(mgr *manager.Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// FiberConfig is the configuration the app is built with. It lives here
// rather than in main.go so the handler tests run against the same one:
// Immutable is a correctness setting, not a tuning knob, and a test app
// without it would not exercise what production does.
func FiberConfig() fiber.Config {
	return fiber.Config{
		// c.Params() hands back a string pointing into the request buffer,
		// which fiber recycles as soon as the handler returns. Anything the
		// manager keeps past that - a client's ServerID is taken straight
		// from the route - would then read as whatever the next request put
		// in that buffer ("i-sett", a slice of some later .../i-settings
		// path). This app answers a handful of requests per minute, so the
		// allocation the immutable mode costs is worth removing the whole
		// class of bug rather than copying at each call site.
		Immutable: true,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				code = fe.Code
			}
			return c.Status(code).JSON(api.ErrorResponse{Error: err.Error()})
		},
	}
}

// RegisterRoutes attaches all API routes to the Fiber app.
func (h *Handlers) RegisterRoutes(app *fiber.App) {
	app.Get("/status", h.containerUptime)

	r := app.Group("/api")

	r.Get("/traffic", h.getTraffic)

	// Servers
	r.Get("/servers", h.getServers)
	r.Post("/servers", h.createServer)
	r.Put("/servers/:id/endpoint", h.updateServerEndpoint)
	r.Delete("/servers/:id", h.deleteServer)
	r.Post("/servers/:id/start", h.startServer)
	r.Post("/servers/:id/stop", h.stopServer)
	r.Get("/servers/:id/config", h.getServerConfig)
	r.Get("/servers/:id/config/download", h.downloadServerConfig)
	r.Get("/servers/:id/info", h.getServerInfo)

	// Clients
	r.Get("/servers/:id/clients", h.getServerClients)
	r.Post("/servers/:id/clients", h.addClient)
	r.Delete("/servers/:id/clients/:clientId", h.deleteClient)
	r.Put("/servers/:id/clients/:clientId", h.updateClient)
	r.Put("/servers/:id/clients/:clientId/allowed-ips", h.updateClientAllowedIPs)
	r.Put("/servers/:id/clients/:clientId/i-settings", h.updateClientISettings)
	r.Get("/servers/:id/clients/:clientId/config", h.downloadClientConfig)
	r.Get("/servers/:id/clients/:clientId/config-both", h.getClientConfigBoth)
	r.Get("/servers/:id/clients/:clientId/link", h.getClientAmneziaLink)
	r.Get("/servers/:id/clients/:clientId/qr", h.getClientQR)
	r.Post("/servers/:id/clients/:clientId/suspend", h.suspendClient)
	r.Post("/servers/:id/clients/:clientId/activate", h.activateClient)
	r.Put("/servers/:id/clients/:clientId/suspend-time", h.updateClientSuspendTime)

	// Misc
	r.Get("/clients", h.getAllClients)
	r.Get("/default-i-settings", h.getDefaultISettings)
	r.Get("/system/status", h.systemStatus)
	r.Post("/system/refresh-ip", h.refreshIP)
	r.Get("/system/iptables-test", h.iptablesTest)
}

// fail turns a manager error into a response. The status code comes from the
// kind of error, so "no such server" and "the interface refused to come up"
// stop sharing one code the way they did when the manager answered in
// booleans.
func fail(c fiber.Ctx, err error) error {
	var upstream interface{ HTTPStatus() int }
	switch {
	case errors.Is(err, manager.ErrNotFound):
		return respond(c, fiber.StatusNotFound, api.ErrorResponse{Error: err.Error()})
	case errors.Is(err, manager.ErrInvalid):
		return respond(c, fiber.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
	case errors.Is(err, manager.ErrConflict):
		return respond(c, fiber.StatusConflict, api.ErrorResponse{Error: err.Error()})
	case errors.Is(err, agentclient.ErrUnavailable):
		return respond(c, fiber.StatusServiceUnavailable, api.ErrorResponse{Error: "Host agent unavailable"})
	case errors.As(err, &upstream):
		code := upstream.HTTPStatus()
		if code < 400 || code > 599 {
			code = fiber.StatusBadGateway
		}
		return respond(c, code, api.ErrorResponse{Error: err.Error()})
	default:
		return respond(c, fiber.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
	}
}

// respond writes a typed payload with an explicit status.
func respond(c fiber.Ctx, status int, payload any) error {
	return c.Status(status).JSON(payload)
}

// decode reads a JSON request body. An unparsable body is a bad request, not
// a silent fallback to the zero value of every field.
func decode(c fiber.Ctx, out any) error {
	body := c.Body()
	if len(body) == 0 {
		return nil // every payload here is optional in full
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %s", manager.ErrInvalid, err)
	}
	return nil
}
