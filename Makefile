# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# PMForge build automation. All targets are .PHONY because they
# represent actions, not files.

CC      := gcc
GO      := go
WAILS   := wails
NPM     := npm
GO_PACKAGES := ./cmd/... ./internal/...

export CGO_ENABLED := 1
export CC

.PHONY: help build dev tidy test race lint lint-go lint-frontend lint-all \
        license-check memory-scan package-linux package-windows package-darwin \
        check-release clean fonts icc check-pdfa frontend-stability \
        frontend-build-budget release-scope check-pades check-pades-external

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build a production binary (CGO + Wails frontend embed).
	@if [ "$${PMFORGE_FRONTEND_BUILT:-0}" = "1" ]; then \
		echo "Using existing frontend/dist from prior frontend-build-budget gate."; \
	else \
		$(MAKE) frontend-build-budget; \
	fi
	rm -rf cmd/pmforge/frontend/dist
	mkdir -p cmd/pmforge/frontend
	cp -R frontend/dist cmd/pmforge/frontend/dist
	mkdir -p build/bin
	$(GO) build -trimpath -ldflags="-s -w" -o build/bin/pmforge ./cmd/pmforge

dev: ## Run Wails in development mode (hot-reload Svelte + Go).
	$(WAILS) dev

tidy: ## go mod tidy + npm install.
	$(GO) mod tidy
	cd frontend && $(NPM) install

fonts: ## Download the bundled TrueType fonts into internal/fonts/assets.
	@bash scripts/fetch-fonts.sh

icc: ## Download the sRGB ICC profile for PDF/A-3 OutputIntent embedding.
	@bash scripts/fetch-icc.sh

check-pdfa: ## Validate generated PDFs for PDF/A-3 conformance using veraPDF (soft gate).
	@bash scripts/validate-pdfa.sh

check-pades: ## Generate and locally verify an embedded PAdES signed PDF sample.
	@bash scripts/validate-pades.sh

check-pades-external: ## Run available external validators against the signed PAdES sample.
	@bash scripts/validate-pades-external.sh

test: ## Run Go unit tests.
	$(GO) test $(GO_PACKAGES)

race: ## Run Go tests with the race detector (concurrency gate).
	$(GO) test -race $(GO_PACKAGES)

frontend-stability: ## Run Svelte warning-clean and Sigma regression gates.
	@bash scripts/frontend-stability-check.sh

frontend-build-budget: ## Build frontend and enforce route-split bundle budgets.
	@bash scripts/frontend-build-budget.sh

release-scope: ## Verify release gates target PMForge-owned source only.
	@bash scripts/release-gate-scope-check.sh

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
	rm -rf cmd/pmforge/frontend/dist
	find . -name .DS_Store -delete
	reuse lint

package-linux: ## Build a Linux tarball on a Linux host.
	@bash scripts/package.sh linux

package-windows: ## Build a Windows tarball on a Windows host.
	@bash scripts/package.sh windows

package-darwin: ## Build a macOS tarball on a macOS host.
	@bash scripts/package.sh darwin

check-release: ## Run the full release gate (versions, REUSE, memory-safety, race, frontend, build, PDF/A, PAdES).
	@bash scripts/check-release.sh

clean: ## Remove build artifacts.
	rm -rf build/ bin/ frontend/dist/ frontend/wailsjs/ cmd/pmforge/frontend/dist/
