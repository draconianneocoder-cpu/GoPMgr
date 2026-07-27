#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Fast, isolated coverage for GitHub release classification and workflow drift.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-release-publication-mode.sh"
CLASSIFY="$ROOT/scripts/release-publication-flag.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-publication-mode-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "check-release-publication-mode test: $*" >&2
	exit 1
}

expect_output() {
	local tag="$1"
	local expected="$2"
	local actual
	actual="$(bash "$CLASSIFY" "$tag")" || fail "classification failed for $tag"
	[ "$actual" = "$expected" ] ||
		fail "$tag produced '$actual'; expected '$expected'"
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	mkdir -p "$fixture/.github/workflows" "$fixture/scripts"
	cp "$ROOT/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
	cp "$CLASSIFY" "$fixture/scripts/release-publication-flag.sh"
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

expect_output "v1.1.0" ""
expect_output "v1.1.0-alpha.1" "--prerelease"
expect_output "v1.1.0-rc.1" "--prerelease"
if bash "$CLASSIFY" "not-a-version-tag" >/dev/null 2>&1; then
	fail "invalid non-version tag was accepted"
fi

baseline="$(new_fixture baseline)"
bash "$CHECK" "$baseline" >/dev/null || fail "valid publication wiring was rejected"

missing_classifier="$(new_fixture missing-classifier)"
perl -0pi -e 's#publication_flag=.*#publication_flag=""#' \
	"$missing_classifier/.github/workflows/release.yml"
expect_failure "$missing_classifier" "publish job must classify the validated Git tag"

dropped_flag="$(new_fixture dropped-flag)"
perl -ni -e 'print unless /"\$\{publication_args\[\@\]\}"/' \
	"$dropped_flag/.github/workflows/release.yml"
expect_failure "$dropped_flag" "gh release create must consume the optional publication argument"

echo "check-release-publication-mode tests passed."
