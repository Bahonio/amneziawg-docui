// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/pprof"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/Bahonio/amneziawg-docui/internal/awg"
	"github.com/Bahonio/amneziawg-docui/internal/config"
	"github.com/Bahonio/amneziawg-docui/internal/frontend"
	"github.com/Bahonio/amneziawg-docui/internal/httpapi"
	"github.com/Bahonio/amneziawg-docui/internal/manager"
	"github.com/Bahonio/amneziawg-docui/internal/store"
	"github.com/Bahonio/amneziawg-docui/web"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := runHealthcheck(); err != nil {
			log.Printf("healthcheck failed: %v", err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("AWG DocUI %s starting (commit %s, built %s)\n", version, commit, buildDate)

	settings := config.FromEnv()
	if err := settings.Validate(); err != nil {
		log.Fatal(err)
	}
	settings.EnsureDirectories()

	mgr, err := manager.New(settings, store.New(settings.ConfigFile), awg.NewAgent(settings.AgentSocket))
	if err != nil {
		log.Fatalf("load panel metadata: %v", err)
	}
	if err := mgr.Start(); err != nil {
		log.Fatalf("start manager: %v", err)
	}

	app := fiber.New(httpapi.FiberConfig())

	app.Use(recover.New())
	app.Use(httpapi.SecurityHeaders)
	app.Use(httpapi.SameOriginMutations)
	app.Use(logger.New())

	assets := frontend.Open(web.Assets())
	app.Use(assets.Compression())

	// Basic auth protects everything except the health check: every asset of
	// the UI is behind the same credentials as the API.
	app.Use(basicauth.New(basicauth.Config{
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/status"
		},
		Users: map[string]string{
			settings.User: settings.PasswordHash,
		},
		Realm: "Restricted Content",
	}))

	// Profiling is opt-in: collecting a CPU profile costs the running server
	// real time, and the endpoints hand out stack traces and command line of
	// the process. Registered after basicauth on purpose, so /debug/pprof
	// requires the same credentials as everything else.
	if settings.Pprof {
		app.Use(pprof.New())
		fmt.Println("pprof enabled at /debug/pprof/")
	}

	// REST routes first, so the asset catch-all cannot shadow them.
	httpapi.New(mgr).RegisterRoutes(app)
	assets.Mount(app)

	fmt.Printf("Web listener on :%d; host access is configured by Docker port publishing (WEB_UI_BIND_ADDRESS).\n", settings.WebUIPort)
	log.Fatal(app.Listen(":"+strconv.Itoa(settings.WebUIPort), fiber.ListenConfig{DisableStartupMessage: true}))
}

func runHealthcheck() error {
	portText := os.Getenv("WEB_UI_PORT")
	if portText == "" {
		portText = "54845"
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid WEB_UI_PORT %q", portText)
	}

	client := &http.Client{Timeout: 4 * time.Second}
	return probeHealth(client, fmt.Sprintf("http://127.0.0.1:%d/status", port))
}

func probeHealth(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return nil
}
