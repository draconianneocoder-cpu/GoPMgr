#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
go_scope_matches="$(mktemp "${TMPDIR:-/tmp}/pmforge-go-scope-matches.XXXXXX")"
go_list_scope="$(mktemp "${TMPDIR:-/tmp}/pmforge-go-list-scope.XXXXXX")"
readme_text="$(tr '\n' ' ' < README.md)"
agent_text="$(tr '\n' ' ' < DEVELOPER_HANDBOOK.md)"
trap 'rm -f "$go_scope_matches" "$go_list_scope"' EXIT

# Concrete release-candidate names are publication claims, not harmless
# examples. Exercise the isolated fixtures before checking the live tree so
# future draft notes cannot silently recreate the false release history.
if ! bash scripts/check-release-reference-truth_test.sh >/dev/null ||
	! bash scripts/check-release-reference-truth.sh >/dev/null; then
	echo "release-scope: unpublished release-candidate reference check failed." >&2
	fail=1
fi

if rg -n '((go|\$\(GO\)) (test|vet)( -race)?|staticcheck|gosec -quiet|govulncheck) \./\.\.\.' Makefile scripts DEVELOPER_HANDBOOK.md >"$go_scope_matches"; then
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

if ! rg -q 'check-wails-version\.sh' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run scripts/check-wails-version.sh." >&2
	fail=1
fi

# Configuration files control CI and packaging before application code runs.
# Guard both the public target and its release invocation so the parser cannot
# remain available but silently fall out of the publication path.
if ! rg -q '^config-check:' Makefile; then
	echo "release-scope: Makefile must define the config-check target." >&2
	fail=1
fi

if ! rg -q '^[[:space:]]*if ! make config-check([[:space:]]|>)' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run make config-check." >&2
	fail=1
fi

# Installer tools execute after the source preflight on separate native
# runners. Preserve both the public target and its full-gate invocation so
# version drift cannot bypass the local/tag source checks.
if ! rg -q '^installer-tool-pins:' Makefile; then
	echo "release-scope: Makefile must define the installer-tool-pins target." >&2
	fail=1
fi

if ! rg -q '^[[:space:]]*if ! make installer-tool-pins([[:space:]]|>)' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run make installer-tool-pins." >&2
	fail=1
fi

# Strict PDF/A output cannot depend on ignored files left by a developer's
# previous `make fonts` run. Preserve both the public integrity target and its
# early release invocation.
if ! rg -q '^required-font-assets:' Makefile; then
	echo "release-scope: Makefile must define the required-font-assets target." >&2
	fail=1
fi

if ! rg -q '^[[:space:]]*if ! make required-font-assets([[:space:]]|>)' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run make required-font-assets." >&2
	fail=1
fi

# Windows packaging must not fall back to an auto-generated default NSIS
# entrypoint or silently ship the analytics stub.
if ! rg -q '^windows-installer-scaffold:' Makefile; then
	echo "release-scope: Makefile must define the windows-installer-scaffold target." >&2
	fail=1
fi

if ! rg -q '^[[:space:]]*if ! make windows-installer-scaffold([[:space:]]|>)' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run make windows-installer-scaffold." >&2
	fail=1
fi

# The PAdES shell regressions cover generated artifacts, external-validator
# parsing, shared-directory locking, and trusted-source result classification.
# Guard all three entry points so a future refactor cannot leave the target
# available locally while silently removing it from release or pre-merge CI.
if ! rg -q '^pades-harness-tests:' Makefile; then
	echo "release-scope: Makefile must define the pades-harness-tests target." >&2
	fail=1
fi

if ! rg -q '^[[:space:]]*if ! make pades-harness-tests([[:space:]]|>)' scripts/check-release.sh; then
	echo "release-scope: check-release.sh must run make pades-harness-tests." >&2
	fail=1
fi

if ! rg -q '^[[:space:]]*run: make pades-harness-tests[[:space:]]*$' .github/workflows/ci.yml; then
	echo "release-scope: CI must run make pades-harness-tests." >&2
	fail=1
fi

# A version tag starts installer publication directly, so it needs its own
# fail-closed preflight rather than relying on a previous branch workflow.
if ! rg -q '^tag-preflight:' Makefile; then
	echo "release-scope: Makefile must define the tag-preflight target." >&2
	fail=1
fi

if ! rg -q '^  preflight:$' .github/workflows/release.yml; then
	echo "release-scope: Release workflow must define the preflight job." >&2
	fail=1
fi

if ! rg -Fq 'run: PMFORGE_RELEASE_TAG="$GITHUB_REF_NAME" make tag-preflight' .github/workflows/release.yml; then
	echo "release-scope: Release workflow must run the tag-aware preflight target." >&2
	fail=1
fi

# The hosted Ubuntu image is not the dependency contract. Keep ripgrep
# explicit because this script and packaging guards execute before application
# compilation and otherwise emit misleading missing-literal errors.
if ! grep -Eq 'sudo apt-get install -y .*ripgrep' .github/workflows/release.yml ||
	! grep -Fq 'command -v rg >/dev/null' .github/workflows/release.yml; then
	echo "release-scope: Release preflight must install and discover ripgrep before source gates." >&2
	fail=1
fi

if ! awk '
	$0 == "  verify:" { in_verify = 1; next }
	in_verify && /^  [A-Za-z0-9_-]+:/ { in_verify = 0 }
	in_verify && /sudo apt-get install -y/ && /ripgrep/ { installed = 1 }
	in_verify && /command -v rg >\/dev\/null/ { discovered = 1 }
	END { exit !(installed && discovered) }
' .github/workflows/ci.yml; then
	echo "release-scope: CI verify job must install and discover ripgrep before make verify." >&2
	fail=1
fi

if ! awk '
	/node-version:/ && $0 !~ /"26"/ { bad = 1 }
	END { exit bad }
' .github/workflows/ci.yml .github/workflows/release.yml; then
	echo "release-scope: GitHub workflows must run frontend jobs on Node.js 26." >&2
	fail=1
fi

if ! grep -Fq -- "window.localStorage" frontend/src/lib/theme.ts ||
	! grep -Fq -- "frontend browser storage must use window.localStorage" scripts/frontend-stability-check.sh ||
	! grep -Fq -- "--experimental-webstorage" scripts/frontend-stability-check.sh ||
	! grep -Fq -- "execArgv: ['--no-experimental-webstorage']" frontend/vitest.config.ts; then
	echo "release-scope: frontend storage must avoid Node.js 26's process-wide localStorage." >&2
	fail=1
fi

if ! awk '
	$0 == "  build:" { in_build = 1; next }
	in_build && /^  [A-Za-z0-9_-]+:/ { in_build = 0 }
	in_build && $0 == "    needs: preflight" { found = 1 }
	END { exit !found }
' .github/workflows/release.yml; then
	echo "release-scope: Release package matrix must depend on the preflight job." >&2
	fail=1
fi

# GitHub does not infer its prerelease flag from a SemVer suffix. Exercise the
# classification helper and a mutated workflow fixture before trusting the
# live publish job, otherwise an alpha or RC tag could be marked as GA.
if ! bash scripts/check-release-publication-mode_test.sh >/dev/null ||
	! bash scripts/check-release-publication-mode.sh >/dev/null; then
	echo "release-scope: GitHub release publication-mode check failed." >&2
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

if ! printf '%s\n' "$readme_text $agent_text" | rg -q 'DSS.*PAdES-BASELINE-T|PAdES-BASELINE-T.*DSS'; then
	echo "release-scope: README.md/DEVELOPER_HANDBOOK.md must document the current DSS PAdES-BASELINE-T fixture result." >&2
	fail=1
fi

if rg -n 'Acrobat/DSS coverage|DSS validation coverage when available|DSS remains skipped|DSS CLI tooling is not installed' README.md DEVELOPER_HANDBOOK.md >"$go_scope_matches"; then
	echo "release-scope: README.md/DEVELOPER_HANDBOOK.md contain stale DSS validation status." >&2
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
