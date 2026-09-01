#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-coverage-ledger-drift.sh had no test of its own until this file:
# unlike its ~15 check-*.sh siblings (see check-wails-version_test.sh for
# the pattern this follows), a broken heading regex here fails silently --
# it just never flags real drift, rather than erroring loudly. That's
# exactly the failure mode a 2026-08-11 fix closed (the original regex
# only recognized `internal/*` headings; the root `gopmgr`, `scripts`, and
# `tools/*` headings were never checked against reality at all) -- so this
# test exists to keep that fix from silently regressing.
#
# Runs the real checker against a throwaway Go module via
# GOPMGR_REPO_ROOT, not the real repository.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
checker="$repo_root/scripts/check-coverage-ledger-drift.sh"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-coverage-ledger-drift-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

fail() {
	echo "check-coverage-ledger-drift_test: $*" >&2
	exit 1
}

# make_fixture builds a minimal buildable Go module named exactly `gopmgr`
# -- the checker's Python step strips a literal "gopmgr/" prefix off `go
# test` package paths, so a differently-named fixture module would make
# every package fail to match for the wrong reason. One package stands in
# for each real-ledger heading shape: the root package, a plain
# `internal/*` package, `internal/analytics` (needed because the checker
# unconditionally runs a second, -tags duckdb pass against it -- an empty
# or missing package there would fail that pass and abort before the
# Python classification logic under test ever runs), `scripts`, and
# `tools/thing`. Every package has exactly one function fully exercised by
# one test, so `go test -cover` reports a deterministic 100.0% -- the
# coverage arithmetic itself is not what this test is checking.
#
# The checker's DUAL_BUILD set hardcodes the literal package name
# "internal/analytics" (matching the real repo, where it's the only
# package with a second -tags duckdb number) -- since this fixture's
# analytics package sits at that same path for the reason above, its
# ledger heading below must use the dual-build format too, or
# check_pkg_heading rejects it as an unrecognized dual-build heading.
make_fixture() {
	local root=$1
	mkdir -p "$root/internal/foo" "$root/internal/analytics" "$root/scripts" "$root/tools/thing" "$root/docs"

	cat >"$root/go.mod" <<'EOF'
module gopmgr

go 1.26
EOF

	cat >"$root/main.go" <<'EOF'
package main

func Add(a, b int) int { return a + b }

func main() {}
EOF
	cat >"$root/main_test.go" <<'EOF'
package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("Add(1, 2) != 3")
	}
}
EOF

	cat >"$root/internal/foo/foo.go" <<'EOF'
package foo

func Double(n int) int { return n * 2 }
EOF
	cat >"$root/internal/foo/foo_test.go" <<'EOF'
package foo

import "testing"

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("Double(2) != 4")
	}
}
EOF

	cat >"$root/internal/analytics/analytics.go" <<'EOF'
package analytics

func Sum(a, b int) int { return a + b }
EOF
	cat >"$root/internal/analytics/analytics_test.go" <<'EOF'
package analytics

import "testing"

func TestSum(t *testing.T) {
	if Sum(2, 3) != 5 {
		t.Fatal("Sum(2, 3) != 5")
	}
}
EOF

	cat >"$root/scripts/scripts.go" <<'EOF'
package scripts

func Triple(n int) int { return n * 3 }
EOF
	cat >"$root/scripts/scripts_test.go" <<'EOF'
package scripts

import "testing"

func TestTriple(t *testing.T) {
	if Triple(2) != 6 {
		t.Fatal("Triple(2) != 6")
	}
}
EOF

	cat >"$root/tools/thing/thing.go" <<'EOF'
package thing

func Negate(n int) int { return -n }
EOF
	cat >"$root/tools/thing/thing_test.go" <<'EOF'
package thing

import "testing"

func TestNegate(t *testing.T) {
	if Negate(2) != -2 {
		t.Fatal("Negate(2) != -2")
	}
}
EOF

	cat >"$root/docs/TEST_COVERAGE_LEDGER.md" <<'EOF'
# Fixture ledger

## Purpose

Fixture ledger for check-coverage-ledger-drift_test.sh, not the real repo's.

## Root package (`gopmgr`, package `main`) — 100.0%

## `internal/foo` — 100.0%

## `internal/analytics` — 100.0% (default build) / 100.0% (`-tags duckdb`)

## `scripts` (package `scripts`, config-check helper) — 100.0%

## `tools/thing` — 100.0%

## Frontend (`frontend/src`) — 12.3% of statements (see `DEVELOPER_HANDBOOK.md`)

## Known gaps (cross-reference)

None.
EOF
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

# --- happy path: every heading shape (root, internal/*, `scripts` with its
# interposed parenthetical, `tools/*`) matches live coverage, and the
# non-package/externally-tracked headings (Purpose, Frontend, Known gaps)
# don't trip the unrecognized-heading catch-all.
happy="$test_root/happy"
make_fixture "$happy"
if ! output="$(GOPMGR_REPO_ROOT="$happy" bash "$checker" 2>&1)"; then
	fail "happy path: expected success, got: $output"
fi
# Hardcodes the fixture's package count (5) as a deliberate tripwire: if
# make_fixture grows a package and this isn't updated to match, the
# failure below should be read as "update this count", not as a reason to
# loosen the assertion into a substring-free pass.
if [[ "$output" != *"all 5 package headings match"* ]]; then
	fail "happy path: expected 5 package headings checked, got: $output"
fi

# --- root package drift: this is the specific gap the 2026-08-11 fix
# closed -- the original checker's heading_re only matched `internal/*`
# and silently never looked at the root heading at all, so this case would
# have passed (wrongly) against the pre-fix script.
root_drift="$test_root/root-drift"
make_fixture "$root_drift"
perl -0pi -e 's/Root package \(`gopmgr`, package `main`\) — 100\.0%/Root package (`gopmgr`, package `main`) — 50.0%/' \
	"$root_drift/docs/TEST_COVERAGE_LEDGER.md"
expect_failure "$root_drift" "gopmgr: heading claims 50.0%"

# --- `scripts` heading drift: exercises the parenthetical-before-em-dash
# format specifically, another heading shape the pre-fix script never
# looked at.
scripts_drift="$test_root/scripts-drift"
make_fixture "$scripts_drift"
perl -0pi -e 's/config-check helper\) — 100\.0%/config-check helper) — 50.0%/' \
	"$scripts_drift/docs/TEST_COVERAGE_LEDGER.md"
expect_failure "$scripts_drift" "scripts: heading claims 50.0%"

# --- missing heading: a package with live coverage (`tools/thing`) but no
# ledger heading at all must fail, not pass by omission.
missing_heading="$test_root/missing-heading"
make_fixture "$missing_heading"
perl -0pi -e 's/\n## `tools\/thing` — 100\.0%\n//' "$missing_heading/docs/TEST_COVERAGE_LEDGER.md"
expect_failure "$missing_heading" "tools/thing: has live coverage"

# --- unrecognized heading format: garbling a heading past what
# generic_pkg_re can parse must hard-fail via the catch-all, not silently
# skip it (the design principle the script's own header states). This
# also removes `internal/foo` from seen_headings, so it doubles as another
# missing-heading failure -- both are asserted.
garbled="$test_root/garbled-heading"
make_fixture "$garbled"
perl -0pi -e 's/## `internal\/foo` — 100\.0%/## internal foo coverage is fine, trust me/' \
	"$garbled/docs/TEST_COVERAGE_LEDGER.md"
if output="$(GOPMGR_REPO_ROOT="$garbled" bash "$checker" 2>&1)"; then
	fail "garbled heading: expected failure, got success: $output"
fi
if [[ "$output" != *"unrecognized heading"* ]]; then
	fail "garbled heading: expected an unrecognized-heading failure: $output"
fi
if [[ "$output" != *"internal/foo: has live coverage"* ]]; then
	fail "garbled heading: expected internal/foo to also be flagged as missing a heading: $output"
fi

echo "check-coverage-ledger-drift_test: all cases passed."
