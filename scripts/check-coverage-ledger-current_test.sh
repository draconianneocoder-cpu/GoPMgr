#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-coverage-ledger-current.sh gates `make verify` directly but had no
# test of its own until this file. Two things specifically motivate it,
# both found on 2026-08-12 while widening the script's file-discovery
# glob to match frontend/vitest.config.ts's real test-inclusion pattern
# (`src/**/*.{test,spec}.{ts,js}`), which is strictly broader than the
# script's previous `*_test.go`/`*.test.ts`-only glob:
#  - a `.spec.ts`/`.spec.js`/`.test.js` test file (all of which vitest
#    picks up and runs) could exist, be unmentioned in the ledger, and
#    never be flagged -- the same pass-by-omission bug class fixed in
#    check-coverage-ledger-drift.sh the previous cycle;
#  - the script's exclusion of `.claude/worktrees` and `frontend/coverage`
#    uses `-path`, not `-name` -- a different, more fragile `find`
#    primitive (path-relative, not name-anywhere) that a `-name`-only
#    regression test wouldn't actually exercise.
#
# Runs the real checker against a throwaway directory tree via
# GOPMGR_REPO_ROOT, not the real repository. Unlike
# check-coverage-ledger-drift_test.sh's fixture, this one needs no
# buildable Go module or real vitest run -- the checker is pure
# find+grep over file names, so plain files with the right names and
# extensions are sufficient.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
checker="$repo_root/scripts/check-coverage-ledger-current.sh"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-coverage-ledger-current-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

fail() {
	echo "check-coverage-ledger-current_test: $*" >&2
	exit 1
}

# make_fixture builds a directory tree with one file of each of the five
# recognized test-file shapes (all mentioned in the fixture ledger, so the
# baseline is a clean pass), plus files of recognized shapes sitting
# under each pruned location -- node_modules and .git (`-name`-style
# prune, matches at any depth) and frontend/coverage and
# .claude/worktrees (`-path`-style prune, matches only that exact
# relative location). None of the pruned files are mentioned in the
# fixture ledger; if pruning is broken, they turn a clean pass into a
# false failure.
make_fixture() {
	local root=$1
	mkdir -p "$root/internal/foo" "$root/frontend/src/lib" \
		"$root/node_modules/some-dep" "$root/frontend/coverage/lcov" \
		"$root/.claude/worktrees/branch-x" "$root/.git/hooks"

	: >"$root/internal/foo/foo_test.go"
	: >"$root/frontend/src/lib/session.test.ts"
	: >"$root/frontend/src/lib/session.spec.ts"
	: >"$root/frontend/src/lib/legacy.test.js"
	: >"$root/frontend/src/lib/legacy.spec.js"

	# Recognized-shape files under every pruned location, deliberately
	# unmentioned in the ledger below -- pruning must exclude them, not
	# the ledger mentioning them.
	: >"$root/node_modules/some-dep/vendored_test.go"
	: >"$root/frontend/coverage/lcov/generated.test.ts"
	: >"$root/.claude/worktrees/branch-x/stale_test.go"
	: >"$root/.git/hooks/hook.test.ts"

	cat >"$root/TEST_COVERAGE_LEDGER.md" <<'EOF'
# Fixture ledger

foo_test.go covers internal/foo.
session.test.ts covers session state.
session.spec.ts covers session state (spec-suffixed variant).
legacy.test.js covers legacy JS.
legacy.spec.js covers legacy JS (spec-suffixed variant).
EOF
}

expect_pass() {
	local fixture=$1
	local output
	if ! output="$(GOPMGR_REPO_ROOT="$fixture" bash "$checker" 2>&1)"; then
		fail "$fixture: expected success, got: $output"
	fi
	echo "$output"
}

expect_failure() {
	local fixture=$1
	local expected=$2
	local output
	if output="$(GOPMGR_REPO_ROOT="$fixture" bash "$checker" 2>&1)"; then
		fail "$fixture: expected failure containing: $expected -- got success: $output"
	fi
	if [[ "$output" != *"$expected"* ]]; then
		fail "$fixture: failure did not contain '$expected': $output"
	fi
}

# --- happy path: all five shapes mentioned, all four pruned locations
# (two -name-style, two -path-style) correctly excluded despite holding
# unmentioned recognized-shape files.
happy="$test_root/happy"
make_fixture "$happy"
expect_pass "$happy" >/dev/null

# --- the specific gap this cycle closes: an unmentioned .spec.ts file.
# vitest would run this file; the pre-widening checker would never have
# looked at it at all (see the hybrid check further down, which proves
# that directly rather than inferring it from the glob).
spec_ts_case="$test_root/spec-ts-unmentioned"
make_fixture "$spec_ts_case"
: >"$spec_ts_case/frontend/src/lib/orphan.spec.ts"
expect_failure "$spec_ts_case" "orphan.spec.ts is not mentioned"

# --- the other two newly recognized shapes, each independently.
spec_js_case="$test_root/spec-js-unmentioned"
make_fixture "$spec_js_case"
: >"$spec_js_case/frontend/src/lib/orphan.spec.js"
expect_failure "$spec_js_case" "orphan.spec.js is not mentioned"

test_js_case="$test_root/test-js-unmentioned"
make_fixture "$test_js_case"
: >"$test_js_case/frontend/src/lib/orphan.test.js"
expect_failure "$test_js_case" "orphan.test.js is not mentioned"

# --- unmentioned *_test.go and *.test.ts still fail too (pre-existing
# behavior, not a new gap, but must not have regressed).
go_case="$test_root/go-unmentioned"
make_fixture "$go_case"
: >"$go_case/internal/foo/orphan_test.go"
expect_failure "$go_case" "orphan_test.go is not mentioned"

ts_case="$test_root/ts-unmentioned"
make_fixture "$ts_case"
: >"$ts_case/frontend/src/lib/orphan.test.ts"
expect_failure "$ts_case" "orphan.test.ts is not mentioned"

# --- prune correctness, isolated from the shape check: an unmentioned
# recognized-shape file under each -path-style pruned location (the more
# fragile primitive -- -path is relative-position-specific, unlike -name)
# must NOT be flagged, even though it would fail every other case above
# if it weren't pruned. This is on top of make_fixture's baseline (which
# already puts unmentioned files under all four pruned locations in every
# case above) -- these two assert the pass explicitly rather than relying
# on absence of failure to imply the prune worked for the right reason.
path_prune_coverage="$test_root/path-prune-coverage"
make_fixture "$path_prune_coverage"
: >"$path_prune_coverage/frontend/coverage/lcov/another_generated.test.ts"
: >"$path_prune_coverage/.claude/worktrees/branch-x/another_stale_test.go"
expect_pass "$path_prune_coverage" >/dev/null

echo "check-coverage-ledger-current_test: all cases passed."
