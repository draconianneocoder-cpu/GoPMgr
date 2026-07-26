#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Hermetic mutations for the Windows installer source contract. These tests do
# not claim native NSIS execution; they keep deterministic input failures local.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-windows-installer-scaffold.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-windows-scaffold-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "check-windows-installer-scaffold test: $*" >&2
	exit 1
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	mkdir -p "$fixture/build/windows/installer" "$fixture/.github/workflows"
	cp "$ROOT/build/windows/installer/project.nsi" "$fixture/build/windows/installer/project.nsi"
	cp "$ROOT/build/windows/info.json" "$fixture/build/windows/info.json"
	cp "$ROOT/build/windows/wails.exe.manifest" "$fixture/build/windows/wails.exe.manifest"
	cp "$ROOT/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
	cp "$ROOT/.gitignore" "$fixture/.gitignore"
	printf '%s\n' "$fixture"
}

expect_failure() {
	local fixture="$1"
	local expected="$2"
	local output
	if output="$(bash "$CHECK" "$fixture" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	case "$output" in
		*"$expected"*) ;;
		*) fail "failure did not contain '$expected': $output" ;;
	esac
}

baseline="$(new_fixture baseline)"
bash "$CHECK" "$baseline" >/dev/null || fail "valid Windows scaffold was rejected"

missing_project="$(new_fixture missing-project)"
rm "$missing_project/build/windows/installer/project.nsi"
expect_failure "$missing_project" "required file missing: build/windows/installer/project.nsi"

generic_branding="$(new_fixture generic-branding)"
perl -0pi -e 's/!define MUI_WELCOMEPAGE_TITLE .*/!define MUI_WELCOMEPAGE_TITLE "Install Application"/' "$generic_branding/build/windows/installer/project.nsi"
expect_failure "$generic_branding" "project.nsi must retain PMForge-owned welcome branding"

destructive_uninstall="$(new_fixture destructive-uninstall)"
printf '\n  RMDir /r "$DOCUMENTS\\PMForge"\n' >>"$destructive_uninstall/build/windows/installer/project.nsi"
expect_failure "$destructive_uninstall" "uninstaller must not delete PMForge project data"

stub_analytics="$(new_fixture stub-analytics)"
perl -0pi -e 's#wails build -platform windows/amd64 -tags duckdb -nsis#wails build -platform windows/amd64 -nsis#' "$stub_analytics/.github/workflows/release.yml"
expect_failure "$stub_analytics" "Windows installer must embed DuckDB analytics"

missing_link_check="$(new_fixture missing-link-check)"
perl -0pi -e 's#\Qbash scripts/verify-duckdb-linked.sh build/bin/pmforge.exe\E#echo "link check removed"#' "$missing_link_check/.github/workflows/release.yml"
expect_failure "$missing_link_check" "Windows release job must verify DuckDB linkage"

missing_fixture_compile="$(new_fixture missing-fixture-compile)"
perl -0pi -e 's#\Qbash scripts/validate-windows-nsis-template.sh\E#echo "NSIS fixture removed"#' "$missing_fixture_compile/.github/workflows/release.yml"
expect_failure "$missing_fixture_compile" "Windows release job must compile the NSIS template fixture"

ignored_manifest="$(new_fixture ignored-manifest)"
perl -0pi -e 's#\Q!/build/windows/wails.exe.manifest\E##' "$ignored_manifest/.gitignore"
expect_failure "$ignored_manifest" ".gitignore must expose each source-owned Windows scaffold file"

late_scaffold_check="$(new_fixture late-scaffold-check)"
perl -0pi -e 's#\Qbash scripts/check-windows-installer-scaffold.sh\E#echo "scaffold check moved"#' "$late_scaffold_check/.github/workflows/release.yml"
printf '\n          bash scripts/check-windows-installer-scaffold.sh\n' >>"$late_scaffold_check/.github/workflows/release.yml"
expect_failure "$late_scaffold_check" "workflow order must be scaffold check, NSIS fixture, build, then DuckDB verification"

echo "check-windows-installer-scaffold tests passed."
