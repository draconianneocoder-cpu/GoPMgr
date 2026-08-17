# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# GoPMgr build automation. All targets are .PHONY because they
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
# Go quality gates scope to the root package, internal packages, and tracked
# tools. Avoid bare ./... because generated
# frontend dependencies may expose unrelated Go packages.
GO_PACKAGES := . ./internal/... ./tools/...

export CGO_ENABLED := 1
export CC

.PHONY: help build dev tidy test race verify lint lint-go lint-frontend lint-all \
        license-check memory-scan package-linux package-windows package-darwin package-macos package-macos-installer \
        check-release clean fonts icc check-pdfa frontend-stability \
        frontend-build-budget frontend-smoke release-scope check-pades check-pades-external \
        check-pades-trusted pades-harness-tests check-encrypted-db linux-runtime-target \
        help-guide-current wails-version wails-cli-version wails-version-test package-version-lib-test tag-preflight config-check \
        installer-tool-pins windows-installer-scaffold required-font-assets reset-clean-test clean-test-reset-tests \
        code-map code-map-current coverage-ledger-current coverage-ledger-drift coverage-ratchet \
        coverage-ratchet-update no-text-timestamp-ordering no-raw-import-in-tests

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build a production app via Wails with embedded DuckDB analytics.
	# scripts/wails-build.sh wraps `wails build`: the CLI injects the required
	# desktop,production tags, links the macOS frameworks (UniformTypeIdentifiers
	# / UTType), builds the frontend, and embeds it; the wrapper then strips
	# extended-attribute detritus and ad-hoc signs the macOS .app (Wails' own
	# self-sign fails on iCloud-synced trees - see the script header).
	# Output: build/bin/<ProductName>.app on macOS, build/bin/gopmgr elsewhere.
	@bash scripts/wails-build.sh -tags "$(WAILS_BUILD_TAGS)" $(WAILS_BUILD_FLAGS)

dev: ## Run Wails in development mode (hot-reload Svelte + Go).
	$(WAILS) dev

tidy: ## go mod tidy + npm install.
	$(GO) mod tidy
	cd frontend && $(NPM) install

fonts: ## Download the bundled TrueType fonts into internal/fonts/assets.
	@bash scripts/fetch-fonts.sh

required-font-assets: ## Verify the tracked Source Sans 3 PDF/A baseline and its provenance.
	@bash scripts/check-required-font-assets_test.sh
	@bash scripts/check-required-font-assets.sh

reset-clean-test: ## Move GoPMgr data to a recoverable backup for first-launch testing (quit GoPMgr first).
	@bash scripts/reset-clean-test.sh

clean-test-reset-tests: ## Verify clean-test reset/restore safety in isolated fixtures.
	@bash scripts/reset-clean-test_test.sh

icc: ## Download the sRGB ICC profile for PDF/A-3 OutputIntent embedding.
	@bash scripts/fetch-icc.sh

check-pdfa: ## Validate generated PDFs for PDF/A-3 conformance using veraPDF (hard gate; GOPMGR_PDFA_STRICT=0 to skip locally).
	@bash scripts/validate-pdfa.sh

check-pades: ## Generate and locally verify an embedded PAdES signed PDF sample.
	@bash scripts/validate-pades.sh

check-pades-external: ## Generate a fresh PAdES sample and run available external validators.
	@bash scripts/validate-pades-external.sh

check-pades-trusted: ## Classify trusted-source PAdES evidence; set GOPMGR_PADES_TRUSTED_REQUIRED=1 to require verified CLI trust.
	@bash scripts/validate-pades-trusted-source.sh

pades-harness-tests: ## Run deterministic local, external, locking, and trusted-source PAdES shell regressions.
	# Keep the real local generator alongside the fake-validator matrices: the
	# matrices isolate error branches, while generation exercises GoPMgr's
	# current CMS, RFC 3161, and PDF incremental-update implementation.
	@bash scripts/validate-pades.sh
	@bash scripts/validate-pades-external_test.sh
	@bash scripts/validate-pades-trusted-source_test.sh
	@bash scripts/validate-pades-parallel_test.sh

check-encrypted-db: ## Validate SQLCipher encrypted project DB create/open/migration/backup.
	@bash scripts/validate-encrypted-db.sh

test: ## Run Go unit tests.
	$(GO) test -tags "$(GO_TEST_TAGS)" $(GO_PACKAGES)

race: ## Run Go tests with the race detector (concurrency gate).
	$(GO) test -race -tags "$(GO_TEST_TAGS)" $(GO_PACKAGES)

verify: config-check installer-tool-pins windows-installer-scaffold required-font-assets clean-test-reset-tests wails-version package-version-lib-test test code-map-current frontend-stability frontend-build-budget coverage-ledger-current no-text-timestamp-ordering no-raw-import-in-tests ## Fast pre-commit gate: config + packaging/toolchain/font/reset/code-map contracts + Go tests + frontend checks.
	@echo "verify: configuration, packaging/Wails/font/reset/code-map contracts, Go tests, svelte-check, and frontend build all passed."

code-map: ## Regenerate the portable first-party Go package dependency map.
	@$(GO) run ./tools/code-map

code-map-current: ## Fail when the checked-in package dependency map is stale.
	@$(GO) run ./tools/code-map -check

coverage-ledger-current: ## Fail if a *_test.go/*.{test,spec}.{ts,js} file exists that TEST_COVERAGE_LEDGER.md never mentions.
	@bash scripts/check-coverage-ledger-current_test.sh
	@bash scripts/check-coverage-ledger-current.sh

coverage-ledger-drift: ## Fail if a package-coverage heading in TEST_COVERAGE_LEDGER.md no longer matches live `go test -cover` output. NOT in `verify`: like coverage-ratchet, it runs a -tags duckdb pass CI's verify job has no toolchain for; run manually or once CI installs the duckdb toolchain.
	@bash scripts/check-coverage-ledger-drift_test.sh
	@bash scripts/check-coverage-ledger-drift.sh

no-text-timestamp-ordering: ## Fail if internal/db ORDER BYs on a bare created_at/updated_at TEXT column instead of its _unixnano sort-key sibling.
	@bash scripts/check-no-text-timestamp-ordering_test.sh
	@bash scripts/check-no-text-timestamp-ordering.sh

no-raw-import-in-tests: ## Fail if a frontend *.test.ts/*.spec.ts file imports another file via Vite's ?raw query, which silently shadows that file's coverage.all statement count to a false 0/0.
	@bash scripts/check-no-raw-import-in-tests_test.sh
	@bash scripts/check-no-raw-import-in-tests.sh

coverage-ratchet: ## Fail only if statement coverage drops below its recorded high-water mark (Go default, Go duckdb, frontend -- tracked independently). NOT in `verify`: CI's verify job builds with GO_TEST_TAGS=webkit2_41 only and installs no DuckDB CGO toolchain, so a duckdb-tagged coverage run would hard-fail there today. Run manually; wire into verify once CI installs the duckdb toolchain, or in Phase 7 when this converts to a hard 100% floor.
	@bash scripts/coverage-ratchet.sh

coverage-ratchet-update: ## Re-run the ratchet and record any improved marks in coverage-baseline.json. Run this after adding tests; never after a real regression.
	@bash scripts/coverage-ratchet.sh --update

frontend-stability: ## Run Svelte warning-clean and Sigma regression gates.
	@bash scripts/frontend-stability-check.sh

frontend-build-budget: ## Build frontend and enforce route-split bundle budgets.
	@bash scripts/frontend-build-budget.sh

frontend-smoke: ## Load + render App.svelte via Vite SSR to catch runtime mount crashes.
	@bash scripts/frontend-smoke-check.sh

release-scope: ## Verify release gates target GoPMgr-owned source only.
	@bash scripts/release-gate-scope-check.sh

config-check: ## Parse tracked YAML/TOML and enforce each tool's supported format.
	# Run controlled malformed/duplicate-key cases before trusting the live
	# repository result; both commands use the same parser implementation.
	@$(GO) test ./scripts
	@$(GO) run ./scripts

installer-tool-pins: ## Reject mutable or mismatched native installer tool selections.
	# Exercise drift cases in isolated fixtures before checking the live release
	# workflow, version record, and Linux packaging guidance.
	@bash scripts/check-installer-tool-pins_test.sh
	@bash scripts/check-installer-tool-pins.sh

windows-installer-scaffold: ## Validate GoPMgr-owned NSIS templates and Windows release wiring.
	@bash scripts/check-windows-installer-scaffold_test.sh
	@bash scripts/check-windows-installer-scaffold.sh
	@bash scripts/validate-windows-nsis-template_test.sh
	@bash scripts/validate-windows-nsis-template.sh

linux-runtime-target: ## Verify Linux CI/packages target Ubuntu 24.04+ WebKit2GTK 4.1.
	@bash scripts/check-linux-runtime-target.sh

wails-version: ## Verify Wails runtime, CLI, and current documentation pins match go.mod.
	@bash scripts/check-wails-version.sh

wails-cli-version: ## Verify the installed Wails CLI also matches the go.mod pin.
	@GOPMGR_REQUIRE_WAILS_CLI=1 bash scripts/check-wails-version.sh

wails-version-test: ## Run isolated regression cases for the Wails version gate.
	@bash scripts/check-wails-version_test.sh

package-version-lib-test: ## Run isolated regression cases for the shared macOS package version derivation.
	@bash scripts/package-version-lib_test.sh

help-guide-current: ## Verify in-app Help Guide covers recent release corrections.
	@bash scripts/check-help-guide-current.sh

memory-scan: ## Run the memory-safety hardening gate.
	@bash scripts/memory-safety-scan.sh

lint-go: ## Lint Go packages with golangci-lint.
	@echo "Linting Go code..."
	golangci-lint run $(GO_PACKAGES)

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
	@bash scripts/package-version-lib_test.sh
	@bash scripts/package-macos_test.sh
	@$(MAKE) build
	@bash scripts/package-macos.sh

package-macos-installer: ## Build a local macOS .pkg installer for /Applications.
	@bash scripts/package-version-lib_test.sh
	@bash scripts/package-macos-installer.sh

check-release: ## Run the full release gate (versions, REUSE, memory-safety, race, frontend, build, encrypted DB, PDF/A, PAdES).
	@bash scripts/check-release.sh

tag-preflight: ## Validate PMFORGE_RELEASE_TAG and run the full gate before tag-triggered packaging.
	# Test the tag contract before applying it to the real tag, then run the
	# complete gate in the same checkout that the package matrix will consume.
	@command -v reuse >/dev/null 2>&1 || { echo "tag-preflight: reuse is required; install the pinned release tool first." >&2; exit 1; }
	@bash scripts/check-release-tag_test.sh
	@bash scripts/check-release-tag.sh
	@$(MAKE) check-release

clean: ## Remove build artifacts (keeps the tracked build/darwin scaffold).
	rm -rf build/bin/ build/packages/ build/macos/ build/appicon.png bin/ frontend/dist/ frontend/wailsjs/
