# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# PMForge build automation. All targets are .PHONY because they
# represent actions, not files.

CC      := gcc
GO      := go
WAILS   := wails
NPM     := npm
# Tags and flags passed to `wails build`. Bindings ARE generated (no -skipbindings):
# Wails needs them so multi-value method results marshal correctly to the
# frontend. The codesign "detritus" problem that -skipbindings previously
# worked around is handled instead by scripts/wails-build.sh, which strips
# extended attributes and ad-hoc signs the .app after the build. Production
# builds include DuckDB analytics by default and target Ubuntu 24.04+
# WebKit2GTK 4.1 on Linux. Override WAILS_BUILD_TAGS only for explicit
# no-DuckDB / legacy-WebKit development checks.
WAILS_BUILD_TAGS ?= duckdb,webkit2_41
WAILS_BUILD_FLAGS ?=
GO_TEST_TAGS ?= webkit2_41
# The main package now lives at the repo root (canonical Wails layout), so
# Go quality gates scope to the root package plus internal/... . Avoid the
# bare ./... form: the release-scope gate forbids it.
GO_PACKAGES := . ./internal/...

export CGO_ENABLED := 1
export CC

.PHONY: help build dev tidy test race verify lint lint-go lint-frontend lint-all \
        license-check memory-scan package-linux package-windows package-darwin package-macos package-macos-installer \
        check-release clean fonts icc check-pdfa frontend-stability \
        frontend-build-budget frontend-smoke release-scope check-pades check-pades-external \
        check-encrypted-db linux-runtime-target help-guide-current

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build a production app via Wails with embedded DuckDB analytics.
	# scripts/wails-build.sh wraps `wails build`: the CLI injects the required
	# desktop,production tags, links the macOS frameworks (UniformTypeIdentifiers
	# / UTType), builds the frontend, and embeds it; the wrapper then strips
	# extended-attribute detritus and ad-hoc signs the macOS .app (Wails' own
	# self-sign fails on iCloud-synced trees - see the script header).
	# Output: build/bin/<ProductName>.app on macOS, build/bin/pmforge elsewhere.
	@bash scripts/wails-build.sh -tags "$(WAILS_BUILD_TAGS)" $(WAILS_BUILD_FLAGS)

dev: ## Run Wails in development mode (hot-reload Svelte + Go).
	$(WAILS) dev

tidy: ## go mod tidy + npm install.
	$(GO) mod tidy
	cd frontend && $(NPM) install

fonts: ## Download the bundled TrueType fonts into internal/fonts/assets.
	@bash scripts/fetch-fonts.sh

icc: ## Download the sRGB ICC profile for PDF/A-3 OutputIntent embedding.
	@bash scripts/fetch-icc.sh

check-pdfa: ## Validate generated PDFs for PDF/A-3 conformance using veraPDF (hard gate; PMFORGE_PDFA_STRICT=0 to skip locally).
	@bash scripts/validate-pdfa.sh

check-pades: ## Generate and locally verify an embedded PAdES signed PDF sample.
	@bash scripts/validate-pades.sh

check-pades-external: ## Run available external validators against the signed PAdES sample.
	@bash scripts/validate-pades-external.sh

check-encrypted-db: ## Validate SQLCipher encrypted project DB create/open/migration/backup.
	@bash scripts/validate-encrypted-db.sh

test: ## Run Go unit tests.
	$(GO) test -tags "$(GO_TEST_TAGS)" $(GO_PACKAGES)

race: ## Run Go tests with the race detector (concurrency gate).
	$(GO) test -race -tags "$(GO_TEST_TAGS)" $(GO_PACKAGES)

verify: test frontend-stability frontend-build-budget ## Fast pre-commit gate: Go tests + svelte-check + frontend (Vite) build.
	@echo "verify: Go tests, svelte-check, and frontend build all passed."

frontend-stability: ## Run Svelte warning-clean and Sigma regression gates.
	@bash scripts/frontend-stability-check.sh

frontend-build-budget: ## Build frontend and enforce route-split bundle budgets.
	@bash scripts/frontend-build-budget.sh

frontend-smoke: ## Load + render App.svelte via Vite SSR to catch runtime mount crashes.
	@bash scripts/frontend-smoke-check.sh

release-scope: ## Verify release gates target PMForge-owned source only.
	@bash scripts/release-gate-scope-check.sh

linux-runtime-target: ## Verify Linux CI/packages target Ubuntu 24.04+ WebKit2GTK 4.1.
	@bash scripts/check-linux-runtime-target.sh

help-guide-current: ## Verify in-app Help Guide covers recent release corrections.
	@bash scripts/check-help-guide-current.sh

memory-scan: ## Run the memory-safety hardening gate.
	@bash scripts/memory-safety-scan.sh

lint-go: ## Lint Go packages with golangci-lint.
	@echo "Linting Go code..."
	golangci-lint run . ./internal/...

lint-frontend: ## Lint Svelte + TS with the npm lint script.
	@echo "Linting Frontend code..."
	cd frontend && $(NPM) run lint

lint-all: lint-go lint-frontend ## Run both linters.

lint: lint-all ## Alias for lint-all.

license-check: ## Verify REUSE/SPDX compliance.
	find . -name .DS_Store -delete
	reuse lint

package-linux: ## Build a Linux tarball on a Linux host.
	@bash scripts/package.sh linux

package-windows: ## Build a Windows tarball on a Windows host.
	@bash scripts/package.sh windows

package-darwin: ## Build a macOS tarball on a macOS host.
	@bash scripts/package.sh darwin

package-macos: ## Build a macOS drag-to-Applications .dmg installer.
	@$(MAKE) build
	@bash scripts/package-macos.sh

package-macos-installer: ## Build a local macOS .pkg installer for /Applications.
	@bash scripts/package-macos-installer.sh

check-release: ## Run the full release gate (versions, REUSE, memory-safety, race, frontend, build, encrypted DB, PDF/A, PAdES).
	@bash scripts/check-release.sh

clean: ## Remove build artifacts (keeps the tracked build/darwin scaffold).
	rm -rf build/bin/ build/packages/ build/macos/ build/appicon.png bin/ frontend/dist/ frontend/wailsjs/
