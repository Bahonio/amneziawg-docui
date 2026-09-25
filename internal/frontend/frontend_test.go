// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package frontend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
)

func newApp() *fiber.App {
	files := fstest.MapFS{
		"index.html":       {Data: []byte("<html>AWG DocUI</html>")},
		"app.js":           {Data: []byte("console.log('docui')")},
		"fonts/sample.ttf": {Data: []byte("font")},
	}
	app := fiber.New()
	assets := Open(files)
	app.Use(assets.Compression())
	assets.Mount(app)
	return app
}

func get(t *testing.T, app *fiber.App, requestPath string, headers map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, requestPath, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAssetsAreServedWithContentTypes(t *testing.T) {
	app := newApp()
	for requestPath, contentType := range map[string]string{
		"/": "text/html", "/app.js": "text/javascript", "/fonts/sample.ttf": "font/ttf",
	} {
		resp := get(t, app, requestPath, nil)
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), contentType) || len(body) == 0 {
			t.Errorf("%s: status=%d content-type=%q body=%q", requestPath, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		}
	}
}

func TestEveryAssetRevalidatesByContentHash(t *testing.T) {
	app := newApp()
	for _, requestPath := range []string{"/", "/index.html", "/app.js", "/fonts/sample.ttf"} {
		resp := get(t, app, requestPath, nil)
		tag := resp.Header.Get("ETag")
		if resp.StatusCode != http.StatusOK || tag == "" || resp.Header.Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s: %d %v", requestPath, resp.StatusCode, resp.Header)
		}
		resp = get(t, app, requestPath, map[string]string{"If-None-Match": tag})
		if resp.StatusCode != http.StatusNotModified {
			t.Errorf("%s with matching tag: %d", requestPath, resp.StatusCode)
		}
	}
}

func TestEtagMatches(t *testing.T) {
	tag := `W/"abc"`
	for header, want := range map[string]bool{
		`W/"abc"`: true, `"abc"`: true, `"x", W/"abc"`: true, "*": true, `"other"`: false, "": false,
	} {
		if got := etagMatches(header, tag); got != want {
			t.Errorf("etagMatches(%q) = %v, want %v", header, got, want)
		}
	}
}
