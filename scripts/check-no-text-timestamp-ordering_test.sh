#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Exercises check-no-text-timestamp-ordering.sh against known-bad and
# known-good fixture lines with the real grep binary this script will
# actually run under (BSD grep on macOS dev machines, GNU grep in CI) --
# an unverified guard is worse than no guard, since it reads as coverage
# that isn't actually there.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-no-text-timestamp-ordering.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# Run the real check script against a temporary internal/db/*.go fixture
# tree, not the real repo tree, so bad fixture lines never need to touch
# tracked files.
run_against_fixture() {
	local body="$1"
	local tmp result
	tmp="$(mktemp -d)"
	mkdir -p "$tmp/internal/db"
	printf 'package db\n\n%s\n' "$body" >"$tmp/internal/db/fixture.go"

	local fixture_check="$tmp/check.sh"
	sed "s|find internal/db|find \"$tmp/internal/db\"|" "$CHECK" >"$fixture_check"
	result=0
	bash "$fixture_check" >/dev/null 2>&1 || result=$?
	rm -rf "$tmp"
	return "$result"
}

expect_pass() {
	local body="$1"
	local label="$2"
	if ! run_against_fixture "$body"; then
		fail "expected pass ($label): $body"
	fi
}

expect_fail() {
	local body="$1"
	local label="$2"
	if run_against_fixture "$body"; then
		fail "expected fail ($label): $body"
	fi
}

# Known-bad: bare TEXT column, single reference.
expect_fail 'const q = `SELECT * FROM t WHERE x = ? ORDER BY updated_at DESC`' "bare updated_at"
expect_fail 'const q = `SELECT * FROM t WHERE x = ? ORDER BY created_at ASC`' "bare created_at"
# Known-bad: TEXT column as a secondary sort key alongside other columns.
expect_fail 'const q = `ORDER BY is_active DESC, created_at DESC, name ASC`' "TEXT column as secondary sort key"

# Known-good: the _unixnano sibling. This is the case the original \b-based
# regex needed to get right -- "created_at" must NOT match inside
# "created_at_unixnano" (underscore is a word character, so a naive \b
# check could wrongly treat the boundary as present).
expect_pass 'const q = `SELECT * FROM t WHERE x = ? ORDER BY updated_at_unixnano DESC`' "_unixnano sibling"
expect_pass 'const q = `ORDER BY is_active DESC, created_at_unixnano DESC, name ASC`' "_unixnano as secondary sort key"

# Known-good: exemption marker suppresses a genuinely-bad-looking line.
expect_pass 'const q = `ORDER BY updated_at DESC` // timestamp-order-guard-exempt: fixture' "exempted bare column"

# Known-good: doc-comment prose that merely discusses ORDER BY, not a live query.
expect_pass '// This package used to do "ORDER BY updated_at DESC" as a plain string comparison.' "doc-comment prose"

echo "check-no-text-timestamp-ordering tests passed."
