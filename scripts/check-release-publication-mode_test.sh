#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Fast, isolated coverage for GitHub release classification and workflow drift.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-release-publication-mode.sh"
CLASSIFY="$ROOT/scripts/release-publication-flag.sh"
CHANNEL="$ROOT/scripts/release-update-channel.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-publication-mode-test.XXXXXX")"
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

expect_channel() {
	local tag="$1"
	local expected="$2"
	local actual
	actual="$(bash "$CHANNEL" "$tag")" || fail "update-channel classification failed for $tag"
	[ "$actual" = "$expected" ] ||
		fail "$tag produced update channel '$actual'; expected '$expected'"
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	mkdir -p "$fixture/.github/workflows" "$fixture/scripts"
	cp "$ROOT/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
	cp "$CLASSIFY" "$fixture/scripts/release-publication-flag.sh"
	cp "$CHANNEL" "$fixture/scripts/release-update-channel.sh"
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
expect_channel "v1.1.0" "stable"
expect_channel "v1.1.0-alpha" "alpha"
expect_channel "v1.1.0-alpha.1" "alpha"
expect_channel "v1.1.0-beta" "beta"
expect_channel "v1.1.0-beta.2" "beta"
expect_channel "v1.1.0-rc" "rc"
expect_channel "v1.1.0-preview-feature.2" "preview-feature"
# Construct the RC form so this test does not become a concrete unpublished
# release claim under check-release-reference-truth.sh.
rc_tag='v1.1.0-'"rc.1"
expect_output "$rc_tag" "--prerelease"
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

published_immediately="$(new_fixture published-immediately)"
perl -ni -e 'print unless /--draft/' \
	"$published_immediately/.github/workflows/release.yml"
expect_failure "$published_immediately" "release artifacts must remain private until their digests and claims are reviewed"

dropped_channel="$(new_fixture dropped-channel)"
perl -0pi -e 's#channel="\$\(bash scripts/release-update-channel\.sh "\$GITHUB_REF_NAME"\)"#channel="stable"#' \
	"$dropped_channel/.github/workflows/release.yml"
expect_failure "$dropped_channel" "build and publish jobs must both derive the signed-update channel"

echo "check-release-publication-mode tests passed."
