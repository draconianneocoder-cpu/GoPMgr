#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Isolated regressions for release-history truthfulness. Fixtures use their own
# Git indexes so repository staging state cannot influence the outcomes.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-release-reference-truth.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-release-reference-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "check-release-reference-truth test: $*" >&2
	exit 1
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	mkdir -p "$fixture/scripts" "$fixture/docs/release-notes"
	cp "$CHECK" "$fixture/scripts/check-release-reference-truth.sh"
	printf '%s\n' 'Published release: v0.9.1-alpha' >"$fixture/README.md"
	printf '%s\n' 'Use v<version-of-record>-<prerelease> for testing.' >"$fixture/docs/release-preflight.md"
	printf '%s\n' 'v0.9.1-alpha' >"$fixture/docs/published-release-tags.txt"
	git -C "$fixture" init -q
	git -C "$fixture" add .
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
bash "$CHECK" "$baseline" >/dev/null || fail "symbolic prerelease guidance was rejected"

concrete_reference="$(new_fixture concrete-reference)"
candidate_tag='v9.9.9-'"rc.9"
printf 'Draft notes for %s\n' "$candidate_tag" >"$concrete_reference/DRAFT.md"
git -C "$concrete_reference" add DRAFT.md
expect_failure "$concrete_reference" "concrete unpublished release-candidate reference found"

candidate_note="$(new_fixture candidate-note)"
candidate_filename='v9.9.9-'"rc.9.md"
printf '%s\n' 'Draft candidate notes' >"$candidate_note/docs/release-notes/$candidate_filename"
git -C "$candidate_note" add "docs/release-notes/$candidate_filename"
expect_failure "$candidate_note" "unpublished candidate note is tracked"

published_candidate="$(new_fixture published-candidate)"
published_tag='v9.9.9-'"rc.9"
published_note="$published_candidate/docs/release-notes/$published_tag.md"
printf '%s\n' "$published_tag" >>"$published_candidate/docs/published-release-tags.txt"
printf 'Published notes for %s\n' "$published_tag" >"$published_note"
git -C "$published_candidate" add .
bash "$CHECK" "$published_candidate" >/dev/null ||
	fail "candidate recorded in the published-tag snapshot was rejected"

echo "check-release-reference-truth tests passed."
