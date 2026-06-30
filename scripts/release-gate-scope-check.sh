#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
go_scope_matches="$(mktemp "${TMPDIR:-/tmp}/pmforge-go-scope-matches.XXXXXX")"
go_list_scope="$(mktemp "${TMPDIR:-/tmp}/pmforge-go-list-scope.XXXXXX")"
readme_text="$(tr '\n' ' ' < README.md)"
agent_text="$(tr '\n' ' ' < AGENT.md)"
trap 'rm -f "$go_scope_matches" "$go_list_scope"' EXIT

if rg -n '((go|\$\(GO\)) (test|vet)( -race)?|staticcheck|gosec -quiet|govulncheck) \./\.\.\.' Makefile scripts AGENT.md >"$go_scope_matches"; then
	echo "release-scope: Go quality gates must target . ./internal/... instead of ./..." >&2
	cat "$go_scope_matches" >&2
	fail=1
fi

if ! rg -q 'frontend-stability-check\.sh' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run scripts/frontend-stability-check.sh." >&2
	fail=1
fi

if ! rg -q 'frontend-build-budget\.sh' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run scripts/frontend-build-budget.sh." >&2
	fail=1
fi

if [ -f scripts/check-help-guide-current.sh ]; then
	if ! bash scripts/check-help-guide-current.sh >/dev/null; then
		echo "release-scope: Help Guide is missing recent release corrections. Run 'make help-guide-current' for details." >&2
		fail=1
	fi
else
	echo "release-scope: scripts/check-help-guide-current.sh is missing." >&2
	fail=1
fi

if ! printf '%s\n' "$readme_text" | rg -q 'FileVault.*BitLocker.*LUKS|BitLocker.*FileVault.*LUKS|LUKS.*FileVault.*BitLocker'; then
	echo "release-scope: README.md must document OS-level disk encryption as whole-device protection." >&2
	fail=1
fi

if ! rg -q 'github.com/mutecomm/go-sqlcipher/v4' go.mod; then
	echo "release-scope: go.mod must include github.com/mutecomm/go-sqlcipher/v4 for native encrypted project databases." >&2
	fail=1
fi

if printf '%s\n' "$readme_text" | rg -q 'SQLCipher[^.]{0,160}deferred|deferred[^.]{0,160}SQLCipher|native database encryption[^.]{0,160}deferred|native encryption[^.]{0,160}deferred'; then
	echo "release-scope: README.md still says SQLCipher/native database encryption is deferred." >&2
	rg -n 'SQLCipher|deferred|native database encryption|native encryption' README.md >"$go_scope_matches" || true
	cat "$go_scope_matches" >&2
	fail=1
fi

if ! printf '%s\n' "$readme_text" | rg -q 'SQLCipher.*encrypted.*\.pmforge|\.pmforge.*SQLCipher.*encrypted'; then
	echo "release-scope: README.md must document SQLCipher-encrypted per-user .pmforge project databases." >&2
	fail=1
fi

if ! printf '%s\n' "$readme_text $agent_text" | rg -q 'DSS.*PAdES-BASELINE-B|PAdES-BASELINE-B.*DSS'; then
	echo "release-scope: README.md/AGENT.md must document the current DSS PAdES-BASELINE-B validation result." >&2
	fail=1
fi

if rg -n 'Acrobat/DSS coverage|DSS validation coverage when available|DSS remains skipped|DSS CLI tooling is not installed' README.md AGENT.md >"$go_scope_matches"; then
	echo "release-scope: README.md/AGENT.md contain stale DSS validation status." >&2
	cat "$go_scope_matches" >&2
	fail=1
fi

if awk '/^package-(linux|windows|darwin):/{in_target=1; next} /^[A-Za-z0-9_-]+:/{in_target=0} in_target && /(\$\(WAILS\)|wails) build/' Makefile | rg -n '(\$\(WAILS\)|wails) build' >"$go_scope_matches"; then
	echo "release-scope: package targets must use the deterministic package script, not Wails CLI packaging." >&2
	cat "$go_scope_matches" >&2
	fail=1
fi

if command -v go >/dev/null 2>&1; then
	if go list . ./internal/... | rg -n '/frontend/|/node_modules/' >"$go_list_scope"; then
		echo "release-scope: scoped Go package list unexpectedly includes frontend or node_modules packages." >&2
		cat "$go_list_scope" >&2
		fail=1
	fi
fi

exit "$fail"
