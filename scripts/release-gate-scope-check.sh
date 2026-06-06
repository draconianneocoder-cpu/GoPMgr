#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
go_scope_matches="$(mktemp "${TMPDIR:-/tmp}/pmforge-go-scope-matches.XXXXXX")"
go_list_scope="$(mktemp "${TMPDIR:-/tmp}/pmforge-go-list-scope.XXXXXX")"
readme_text="$(tr '\n' ' ' < README.md)"
trap 'rm -f "$go_scope_matches" "$go_list_scope"' EXIT

if rg -n '((go|\$\(GO\)) (test|vet)( -race)?|staticcheck|gosec -quiet|govulncheck) \./\.\.\.' Makefile scripts AGENT.md >"$go_scope_matches"; then
	echo "release-scope: Go quality gates must target ./cmd/... ./internal/... instead of ./..." >&2
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

if ! printf '%s\n' "$readme_text" | rg -q 'FileVault.*BitLocker.*LUKS|BitLocker.*FileVault.*LUKS|LUKS.*FileVault.*BitLocker'; then
	echo "release-scope: README.md must document OS-level disk encryption as the V2 at-rest protection path." >&2
	fail=1
fi

if ! printf '%s\n' "$readme_text" | rg -q 'SQLCipher.*V3|V3.*SQLCipher|deferred.*SQLCipher|SQLCipher.*deferred'; then
	echo "release-scope: README.md must state that SQLCipher/native database encryption is deferred beyond V2." >&2
	fail=1
fi

if awk '/^package-(linux|windows|darwin):/{in_target=1; next} /^[A-Za-z0-9_-]+:/{in_target=0} in_target && /(\$\(WAILS\)|wails) build/' Makefile | rg -n '(\$\(WAILS\)|wails) build' >"$go_scope_matches"; then
	echo "release-scope: package targets must use the deterministic package script, not Wails CLI packaging." >&2
	cat "$go_scope_matches" >&2
	fail=1
fi

if command -v go >/dev/null 2>&1; then
	if go list ./cmd/... ./internal/... | rg -n '/frontend/|/node_modules/' >"$go_list_scope"; then
		echo "release-scope: scoped Go package list unexpectedly includes frontend or node_modules packages." >&2
		cat "$go_list_scope" >&2
		fail=1
	fi
fi

exit "$fail"
