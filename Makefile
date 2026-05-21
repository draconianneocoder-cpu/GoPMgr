# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# PMForge build automation. All targets are .PHONY because they
# represent actions, not files.

CC      := gcc
GO      := go
WAILS   := wails
NPM     := npm

export CGO_ENABLED := 1
export CC

.PHONY: help build dev tidy test lint lint-go lint-frontend lint-all \
        license-check package-linux package-windows package-darwin \
        check-release clean fonts

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build a production binary (CGO + Wails frontend embed).
	$(WAILS) build -clean -compiler $(CC) -ldflags="-s -w"

dev: ## Run Wails in development mode (hot-reload Svelte + Go).
	$(WAILS) dev

tidy: ## go mod tidy + npm install.
	$(GO) mod tidy
	cd frontend && $(NPM) install

fonts: ## Download the bundled TrueType fonts into internal/fonts/assets.
	@bash scripts/fetch-fonts.sh

test: ## Run Go unit tests.
	$(GO) test ./...

race: ## Run Go tests with the race detector (concurrency gate).
	$(GO) test -race ./...

memory-scan: ## Run the memory-safety hardening gate.
	@bash scripts/memory-safety-scan.sh

lint-go: ## Lint Go packages with golangci-lint.
	@echo "Linting Go code..."
	golangci-lint run ./internal/... ./cmd/...

lint-frontend: ## Lint Svelte + TS with the npm lint script.
	@echo "Linting Frontend code..."
	cd frontend && $(NPM) run lint

lint-all: lint-go lint-frontend ## Run both linters.

lint: lint-all ## Alias for lint-all.

license-check: ## Verify REUSE/SPDX compliance.
	reuse lint

package-linux: ## Build a Linux package via Wails.
	$(WAILS) build -platform linux/amd64 -package

package-windows: ## Build a Windows package via Wails.
	$(WAILS) build -platform windows/amd64 -package

package-darwin: ## Build a macOS package via Wails.
	$(WAILS) build -platform darwin/universal -package

check-release: ## Run the full release gate (versions + REUSE + build).
	@bash scripts/check-release.sh

clean: ## Remove build artifacts.
	rm -rf build/ bin/ frontend/dist/ frontend/wailsjs/
