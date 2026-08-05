#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Coverage ratchet: fails only if statement coverage drops below the last
# recorded high-water mark, for any of the three independently-tracked
# targets (Go default build, Go -tags duckdb build, frontend). It never
# requires 100% today -- it requires "not worse than the best we've ever
# measured." See DEVELOPER_HANDBOOK.md's "100% coverage: scope and
# exclusions" entry for why three separate marks are needed instead of one:
# the default and duckdb Go builds compile different files (stub.go vs
# duckdb.go), so a single merged percentage can't represent either
# faithfully and would let one config's regression hide behind the other's
# improvement.
#
# coverage-baseline.json (checked into the repo) holds the three marks as
# exact "<covered> <total>" statement counts, not percentages -- percentages
# round, and a rounded ratchet can't catch a one-statement regression.
#
# GOPMGR_ALLOW_COVERAGE_REGRESSION is an explicit, developer-set escape
# hatch, the same shape as GOPMGR_SKIP_NSIS_COMPILE: while 100% coverage is
# a work in progress (see DEVELOPER_HANDBOOK.md's "100% coverage: scope and
# exclusions" entry), a strict ratchet with no override would fail
# `make verify` on ANY unrelated commit that adds a new, not-yet-tested
# line -- a bugfix, a feature, an untested error branch -- which is exactly
# the "blocks all unrelated work" problem a ratchet was chosen over a hard
# floor to avoid. Set it to skip the fail-on-regression check for one run;
# it still runs and reports the numbers, and `--update` still refuses to
# write a worse mark into coverage-baseline.json even when this is set, so
# a regression can never get silently recorded as the new baseline.
#
# Usage: coverage-ratchet.sh [--update]
#   --update   after checking, rewrite coverage-baseline.json with any marks
#              that improved (never writes a mark that got worse -- use
#              this after adding tests, not to silence a real regression).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASELINE="$ROOT/coverage-baseline.json"
UPDATE=0
if [ "${1:-}" = "--update" ]; then
	UPDATE=1
fi

go_default="$(bash "$ROOT/scripts/coverage-go.sh" default)"
go_duckdb="$(bash "$ROOT/scripts/coverage-go.sh" duckdb)"
frontend="$(bash "$ROOT/scripts/coverage-frontend.sh")"

python3 - "$BASELINE" "$UPDATE" "$go_default" "$go_duckdb" "$frontend" <<'PY'
import json, os, sys

baseline_path, update, go_default, go_duckdb, frontend = sys.argv[1:6]
update = update == "1"
allow_regression = bool(os.environ.get("GOPMGR_ALLOW_COVERAGE_REGRESSION"))

def parse(pair):
    covered, total = (int(x) for x in pair.split())
    return covered, total

current = {
    "go_default": parse(go_default),
    "go_duckdb": parse(go_duckdb),
    "frontend": parse(frontend),
}

try:
    with open(baseline_path) as f:
        baseline = json.load(f)
except FileNotFoundError:
    baseline = {}

failed = False
next_baseline = dict(baseline)
for key, (covered, total) in current.items():
    pct = 100.0 * covered / total if total else 0.0
    prev = baseline.get(key)
    if prev is None:
        print(f"{key}: {covered}/{total} ({pct:.2f}%) -- no prior mark, recording as the initial baseline.")
        next_baseline[key] = {"covered": covered, "total": total}
        continue
    prev_covered, prev_total = prev["covered"], prev["total"]
    prev_pct = 100.0 * prev_covered / prev_total if prev_total else 0.0
    # Compare by percentage, since `total` legitimately shifts as code is
    # added/removed; only a statement-count regression relative to the same
    # denominator would be a false negative here, and that's covered by
    # comparing pct rather than raw covered count.
    if pct + 1e-9 < prev_pct:
        suffix = " (GOPMGR_ALLOW_COVERAGE_REGRESSION set -- not failing, but NOT recording this as the new baseline)" if allow_regression else ""
        print(f"{key}: {covered}/{total} ({pct:.2f}%) REGRESSED below baseline {prev_covered}/{prev_total} ({prev_pct:.2f}%){suffix}")
        if not allow_regression:
            failed = True
    else:
        verdict = "IMPROVED" if pct > prev_pct + 1e-9 else "held"
        print(f"{key}: {covered}/{total} ({pct:.2f}%) {verdict}, baseline {prev_pct:.2f}%")
        if pct > prev_pct + 1e-9:
            next_baseline[key] = {"covered": covered, "total": total}

if update:
    with open(baseline_path, "w") as f:
        json.dump(next_baseline, f, indent=2, sort_keys=True)
        f.write("\n")

sys.exit(1 if failed else 0)
PY
