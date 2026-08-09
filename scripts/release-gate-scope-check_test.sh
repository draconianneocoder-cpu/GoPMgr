#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Regression coverage for command-scope matching. Use disposable clones so
# the release gate sees a real repository without touching this worktree.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/release-gate-scope-check.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-release-scope-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "release-gate-scope-check test: $*" >&2
	exit 1
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	git clone -q --no-hardlinks "$ROOT" "$fixture"
	cp "$CHECK" "$fixture/scripts/release-gate-scope-check.sh"
	mkdir -p "$fixture/frontend/dist"
	: >"$fixture/frontend/dist/index.html"
	printf '%s\n' "$fixture"
}

expect_failure() {
	local fixture="$1"
	local expected="$2"
	local output
	if output="$(GOPMGR_RELEASE_SCOPE_SKIP_SELF_TEST=1 bash "$fixture/scripts/release-gate-scope-check.sh" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	case "$output" in
	*"$expected"*) ;;
	*) fail "failure did not contain '$expected': $output" ;;
	esac
}

baseline="$(new_fixture baseline)"
GOPMGR_RELEASE_SCOPE_SKIP_SELF_TEST=1 \
	bash "$baseline/scripts/release-gate-scope-check.sh" >/dev/null ||
	fail "valid release scope was rejected"

historical_prose="$(new_fixture historical-prose)"
printf '%s\n' 'Historical evidence: go test ./... passed before this gate existed.' >>"$historical_prose/DEVELOPER_HANDBOOK.md"
GOPMGR_RELEASE_SCOPE_SKIP_SELF_TEST=1 \
	bash "$historical_prose/scripts/release-gate-scope-check.sh" >/dev/null ||
	fail "historical handbook prose was treated as a live quality gate"

unscoped_command="$(new_fixture unscoped-command)"
printf '%s\n' 'quality-check:' $'\tgo test ./...' >>"$unscoped_command/Makefile"
expect_failure "$unscoped_command" "Go quality gates must target"

unscoped_script="$(new_fixture unscoped-script)"
printf '%s\n' 'go test ./...' >"$unscoped_script/scripts/quality-check.sh"
expect_failure "$unscoped_script" "Go quality gates must target"

echo "release-gate-scope-check tests passed."
