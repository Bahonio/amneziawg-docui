// Package agentclient implements the web backend's only connection to the
// host. It speaks HTTP over one Unix socket and never opens a TCP connection.
package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Bahonio/amneziawg-docui/internal/agentapi"
)

var ErrUnavailable = errors.New("Host agent unavailable")

type APIError struct {
	Status int
	Text   string
}

func (e *APIError) Error() string   { return e.Text }
func (e *APIError) HTTPStatus() int { return e.Status }

type Client struct {
	socket string
	http   *http.Client
}

func New(socket string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
	return &Client{socket: socket, http: &http.Client{Transport: transport, Timeout: 15 * time.Second}}
}

func (c *Client) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, "http://unix/"+agentapi.Version+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var payload agentapi.ErrorResponse
		if json.Unmarshal(data, &payload) != nil || payload.Error == "" {
			payload.Error = strings.TrimSpace(string(data))
		}
		if payload.Error == "" {
			payload.Error = http.StatusText(resp.StatusCode)
		}
		return &APIError{Status: resp.StatusCode, Text: payload.Error}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func ifacePath(name string) string { return "/interfaces/" + url.PathEscape(name) }

func (c *Client) Health() (agentapi.BackendStatus, error) {
	var out agentapi.BackendStatus
	err := c.do(http.MethodGet, "/health", nil, &out)
	return out, err
}

func (c *Client) Interfaces() ([]agentapi.Interface, error) {
	var out []agentapi.Interface
	err := c.do(http.MethodGet, "/interfaces", nil, &out)
	return out, err
}

func (c *Client) Snapshot() ([]agentapi.InterfaceDetail, error) {
	var out []agentapi.InterfaceDetail
	err := c.do(http.MethodGet, "/snapshot", nil, &out)
	return out, err
}

func (c *Client) ReadConfig(name string) (string, error) {
	var out agentapi.InterfaceDetail
	err := c.do(http.MethodGet, ifacePath(name)+"/config", nil, &out)
	return out.Config, err
}

func (c *Client) InterfaceStatus(name string) (bool, error) {
	var out agentapi.Interface
	err := c.do(http.MethodGet, ifacePath(name)+"/state", nil, &out)
	return out.Running, err
}

func (c *Client) Interface(name string) (agentapi.InterfaceDetail, error) {
	var out agentapi.InterfaceDetail
	err := c.do(http.MethodGet, ifacePath(name), nil, &out)
	return out, err
}

func (c *Client) Create(req agentapi.CreateInterfaceRequest) error {
	return c.do(http.MethodPost, "/interfaces", req, nil)
}

func (c *Client) Apply(name, config string, live bool) error {
	return c.do(http.MethodPut, ifacePath(name)+"/config", agentapi.ApplyConfigRequest{Config: config, ApplyLive: live}, nil)
}

func (c *Client) Delete(name string) error {
	return c.do(http.MethodDelete, ifacePath(name), nil, nil)
}

func (c *Client) Action(name, action string) error {
	return c.do(http.MethodPost, ifacePath(name)+"/"+action, nil, nil)
}

func (c *Client) Stats(name string) (agentapi.InterfaceStats, error) {
	var out agentapi.InterfaceStats
	err := c.do(http.MethodGet, ifacePath(name)+"/stats", nil, &out)
	return out, err
}

func (c *Client) Keys() (agentapi.KeyPair, error) {
	var out agentapi.KeyPair
	err := c.do(http.MethodPost, "/keys", struct{}{}, &out)
	return out, err
}

func (c *Client) PresharedKey() (string, error) {
	var out agentapi.PresharedKey
	err := c.do(http.MethodPost, "/preshared-key", struct{}{}, &out)
	return out.Key, err
}

func (c *Client) PublicKey(private string) (string, error) {
	var out agentapi.PublicKeyResponse
	err := c.do(http.MethodPost, "/public-key", agentapi.PublicKeyRequest{Private: private}, &out)
	return out.Public, err
}

func (c *Client) RouteSource() (string, error) {
	var out agentapi.RouteSource
	err := c.do(http.MethodGet, "/route-source", nil, &out)
	return out.Address, err
}

func (c *Client) Firewall(name, subnet string) (map[string]string, error) {
	var out agentapi.FirewallStatus
	err := c.do(http.MethodGet, ifacePath(name)+"/firewall?subnet="+url.QueryEscape(subnet), nil, &out)
	return out.Checks, err
}
