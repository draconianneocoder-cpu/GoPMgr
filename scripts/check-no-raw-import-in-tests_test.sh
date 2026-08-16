#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Exercises check-no-raw-import-in-tests.sh against known-bad and
# known-good fixture lines with the real grep binary this script will
# actually run under (BSD grep on macOS dev machines, GNU grep in CI) --
# an unverified guard is worse than no guard, since it reads as coverage
# that isn't actually there.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-no-raw-import-in-tests.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# Run the real check script against a temporary frontend/src fixture tree,
# not the real repo tree, so bad fixture lines never need to touch tracked
# files. Writes the fixture as a *.test.ts file, since the guard is
# scoped to test files.
run_against_fixture() {
	local body="$1"
	local tmp result
	tmp="$(mktemp -d)"
	mkdir -p "$tmp/frontend/src/lib"
	printf '%s\n' "$body" >"$tmp/frontend/src/lib/fixture.test.ts"

	local fixture_check="$tmp/check.sh"
	sed "s|find frontend/src|find \"$tmp/frontend/src\"|" "$CHECK" >"$fixture_check"
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

# Prove the harness itself actually detects a bad fixture, not just that
# the sed rewrite silently no-ops and every case reads as passing.
if run_against_fixture "import x from './Foo.svelte?raw';"; then
	fail "harness sanity check: expected the known-bad static ?raw import to fail, but it passed -- the find-path sed rewrite may not be matching check-no-raw-import-in-tests.sh"
fi

# Known-bad: static ?raw import.
expect_fail "import x from './Foo.svelte?raw';" "static ?raw import"
expect_fail "import helpSource from './components/HelpGuide.svelte?raw';" "static ?raw import, real-shape specifier"
# Known-bad: dynamic ?raw import.
expect_fail "const x = await import('./Foo.svelte?raw');" "dynamic ?raw import"
# Known-bad: ?raw import as a secondary statement, not the first line.
expect_fail $'import { describe, it } from \'vitest\';\nimport x from \'./Foo.svelte?raw\';' "?raw import after an unrelated import"

# Known-good: reading the file from disk instead of importing it raw.
expect_pass "import { readFileSync } from 'node:fs'; const x = readFileSync('./Foo.svelte', 'utf-8');" "node:fs read"
# Known-good: a normal (non-raw) import of the same file.
expect_pass "import Foo from './Foo.svelte';" "normal import, no ?raw"

# Known-good: exemption marker suppresses a genuinely-bad-looking line.
expect_pass "import x from './Foo.svelte?raw'; // raw-import-guard-exempt: fixture" "exempted ?raw import"

# Known-good: doc-comment prose that merely discusses ?raw imports, not a
# live import statement -- this is the case that motivated anchoring the
# regex on import syntax rather than the bare `?raw` token, since this
# guard's own source file discusses `?raw` at length in its header.
expect_pass "// A \`?raw\` import pulls the target file into this test's own module graph." "doc-comment prose mentioning ?raw"
expect_pass "// See './Foo.svelte?raw' for the pattern this guard forbids." "doc-comment prose with a quoted ?raw-looking specifier"

echo "check-no-raw-import-in-tests tests passed."
