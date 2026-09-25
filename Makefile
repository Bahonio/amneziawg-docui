# Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
# SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

SHELL = /usr/bin/env bash
.SHELLFLAGS = -o pipefail -ec
GO_BUILD_FLAGS = -trimpath -ldflags="-s -w"

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n"} /^[a-zA-Z_0-9-]+:.*?##/ {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build-server
build-server: ## Build the management Web application with embedded assets
	go build $(GO_BUILD_FLAGS) -o awg-docui.bin .

.PHONY: build-host-agent
build-host-agent: ## Build the Linux host agent
	CGO_ENABLED=0 GOOS=linux go build $(GO_BUILD_FLAGS) -o awg-docui-agent.bin ./cmd/host-agent

.PHONY: build
build: build-server build-host-agent ## Build the Web application and host agent

.PHONY: test test-installer-safety
test: test-installer-safety ## Run all Go and installer tests
	go test ./...

test-installer-safety: ## Prove the installer cannot restart existing VPN units
	./scripts/test-installer-safety.sh
	./scripts/test-uninstaller-safety.sh

.PHONY: test-installer-sandbox
test-installer-sandbox: ## Exercise the installer on a simulated host in a disposable container
	./scripts/test-installer-sandbox.sh
	./scripts/test-bootstrap-sandbox.sh

.PHONY: vet
vet: ## Run Go static analysis
	go vet ./...

.PHONY: docker-build
docker-build: ## Build the management-only image locally
	docker build --tag awg-docui:local .

.PHONY: run
run: ## Run Compose with a locally built image
	docker compose -f docker-compose.yml -f docker-compose.build.yml up --build

.PHONY: install-host-agent
install-host-agent: ## Install management agent on this Linux host
	sudo ./install-host-agent.sh

.PHONY: uninstall-host-agent
uninstall-host-agent: ## Remove management agent without touching VPN interfaces
	sudo ./uninstall-host-agent.sh

.PHONY: e2e
e2e: ## Run browser tests against E2E_URL and a real test host agent
	cd e2e && npm install --no-audit --no-fund && npx playwright test
