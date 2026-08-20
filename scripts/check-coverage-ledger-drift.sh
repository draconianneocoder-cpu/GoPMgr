#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# TEST_COVERAGE_LEDGER.md's package-coverage headings ("## `internal/X` —
# NN.N%", the root "## Root package (`gopmgr`, ...)", "## `scripts` (...)",
# "## `tools/X`") are hand-written and nothing previously checked them
# against reality. check-coverage-ledger-current.sh only verifies a
# *_test.go file is *mentioned* somewhere in the ledger -- it says nothing
# about whether a heading's claimed percentage still matches the package's
# actual coverage. That drift is not hypothetical: a 2026-08-10 audit found
# internal/pdfmeta's heading three points stale (claimed 96.4%, actual
# 97.6%), left behind when a landed increment's own commit message got
# the number right but the ledger heading update didn't land. This script
# is that prevention: it runs the real Go test suite and fails on any
# heading whose claimed percentage no longer matches.
#
# Deliberately does NOT silently skip a heading it can't parse. A heading
# in a format this script doesn't recognize is exactly the kind of drift
# it exists to catch (see the applog/analytics handling below) -- an
# unparseable heading is a hard failure, not a pass-by-omission, the same
# principle check-coverage-ledger-current.sh's own header describes for
# missing test-file mentions. This originally covered only `internal/*`
# headings (2026-08-10); every other package-coverage heading (root,
# `scripts`, `tools/*`) was silently unchecked until 2026-08-11, which was
# itself exactly the kind of pass-by-omission gap this script exists to
# close -- see the "every '## ' heading is classified" block below.
#
# GOPMGR_REPO_ROOT overrides the repo root, for check-coverage-ledger-drift_test.sh
# to run this against isolated fixtures instead of the real repository.
#
# Usage: check-coverage-ledger-drift.sh

set -euo pipefail

ROOT="${GOPMGR_REPO_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"
ledger="TEST_COVERAGE_LEDGER.md"

if [ ! -f "$ledger" ]; then
	echo "check-coverage-ledger-drift: $ledger is missing." >&2
	exit 1
fi

mkdir -p "$ROOT/.tmp"
default_log="$ROOT/.tmp/coverage-ledger-drift-default.log"
duckdb_analytics_log="$ROOT/.tmp/coverage-ledger-drift-duckdb-analytics.log"

echo "check-coverage-ledger-drift: running go test ./... -cover (default build)..." >&2
if ! go test ./... -cover >"$default_log" 2>&1; then
	echo "check-coverage-ledger-drift: go test ./... -cover failed:" >&2
	cat "$default_log" >&2
	exit 1
fi

# internal/analytics is the only heading with a second, -tags duckdb
# number, so only that package is re-run under the duckdb tag rather than
# paying for a full second suite run here (coverage-ratchet.sh already
# covers the full duckdb build elsewhere in `make verify`).
echo "check-coverage-ledger-drift: running go test -tags duckdb ./internal/analytics/... -cover..." >&2
if ! go test -tags duckdb ./internal/analytics/... -cover >"$duckdb_analytics_log" 2>&1; then
	echo "check-coverage-ledger-drift: go test -tags duckdb ./internal/analytics/... -cover failed:" >&2
	cat "$duckdb_analytics_log" >&2
	exit 1
fi

python3 - "$ledger" "$default_log" "$duckdb_analytics_log" <<'PY'
import re
import sys

ledger_path, default_log_path, duckdb_analytics_log_path = sys.argv[1:4]

with open(default_log_path) as f:
    default_out = f.read()
with open(duckdb_analytics_log_path) as f:
    duckdb_analytics_out = f.read()
with open(ledger_path) as f:
    content = f.read()

# "ok  \tgopmgr/internal/foo\t(cached)\tcoverage: 92.6% of statements"
# "ok  \tgopmgr/internal/foo\t0.214s\tcoverage: [no statements]"
# The (cached) / elapsed-time segment is optional and its exact form
# varies run to run, so it's matched loosely rather than enumerated.
# Only "ok" is matched, not "FAIL": the calling shell script already
# treats a nonzero `go test` exit as a hard failure and exits before this
# runs, so a FAIL line here would mean this script is being fed output it
# wasn't designed for -- better to not match it than to silently compare
# a heading against a number from a failed run.
line_re = re.compile(r'^ok\s+(\S+)\s+(?:\S+\s+)?coverage:\s+(.+)$', re.M)

def parse_live(text):
    live = {}
    for pkg, cov in line_re.findall(text):
        if pkg.startswith("gopmgr/"):
            pkg = pkg[len("gopmgr/"):]
        cov = cov.strip()
        if cov == "[no statements]":
            live[pkg] = None  # None sentinel: package has no statements
        else:
            m = re.match(r'([\d.]+)% of statements', cov)
            if not m:
                print(f"check-coverage-ledger-drift: could not parse go test coverage output for {pkg}: {cov!r}", file=sys.stderr)
                sys.exit(1)
            live[pkg] = float(m.group(1))
    return live

live_default = parse_live(default_out)
live_duckdb_analytics = parse_live(duckdb_analytics_out)

# Every "## ..." heading in the ledger is classified into exactly one of:
#  (a) a package-coverage heading this script checks against live `go
#      test` output -- `internal/*`, the root `gopmgr` package, `scripts`,
#      `tools/*`;
#  (b) a non-package prose section (Purpose, Known gaps) -- listed by
#      exact heading text in NON_PACKAGE_HEADINGS; renaming one of those
#      section titles means updating that set, or it starts hard-failing
#      as an unrecognized heading;
#  (c) coverage tracked by a different tool than `go test`, checked
#      elsewhere -- currently just Frontend (vitest, via
#      coverage-frontend.sh / `make coverage-ratchet`), matched by prefix
#      since its trailing percentage is expected to drift independently.
# Anything not in one of those three buckets is a hard failure: an
# unrecognized heading format, per this script's own stated "unparseable
# is a failure, not a pass-by-omission" principle. This replaces the
# original internal/*-only heading_re, which silently never looked at any
# other heading at all -- itself an instance of the same gap.
all_heading_re = re.compile(r'^## (.+)$', re.M)
# `<path>`[ (parenthetical)] — <rest>. The optional parenthetical covers
# `scripts`, whose heading has one between the package name and the
# percentage ("`scripts` (package `scripts`, config-check helper) —
# 63.4%"); `internal/*` and `tools/*` headings have no such parenthetical
# and match with it empty.
generic_pkg_re = re.compile(r'^## `([a-zA-Z0-9_/.-]+)`(?:\s*\([^)]*\))?\s*—\s*(.+)$', re.M)
# The root package heading doesn't lead with a backtick at all ("## Root
# package (`gopmgr`, package `main`) — 62.4%"), so it needs its own literal
# pattern rather than fitting generic_pkg_re.
root_re = re.compile(r'^## Root package \(`gopmgr`, package `main`\) — (.+)$', re.M)

NON_PACKAGE_HEADINGS = {"Purpose", "Known gaps (cross-reference)"}
FRONTEND_HEADING_PREFIX = "Frontend (`frontend/src`)"

# Packages whose heading is known, by inspection, not to be a bare "NN.N%"
# -- each gets its own explicit parsing rule below. A heading NOT in this
# set and NOT matching the bare-percent fallback is an error, not a skip.
NO_STATEMENTS = {"internal/sigma/domain", "internal/sqlitedriver"}
DUAL_BUILD = {"internal/analytics"}
# internal/applog's heading leads with a per-FILE claim ("100% of
# applog.go") before the package-level number in parens; only the
# package-level number is checked here. The per-file claim is not
# independently verified -- disclosed, not silently assumed correct.
OF_PACKAGE_PATTERN = re.compile(r'([\d.]+)% of package')

failures = []
checked = 0
seen_headings = set()          # package keys checked against live coverage
consumed_heading_text = set()  # heading text already classified by some bucket

def check_pkg_heading(pkg, heading):
    global checked
    checked += 1
    seen_headings.add(pkg)
    if pkg in NO_STATEMENTS:
        live = live_default.get(pkg, "MISSING")
        if live is not None:
            failures.append(f"{pkg}: heading claims 'no statements' but go test reports {live!r} -- package gained testable statements, update the heading")
        return

    if pkg in DUAL_BUILD:
        m_default = re.search(r'([\d.]+)% \(default build\)', heading)
        m_duckdb = re.search(r'([\d.]+)% \(`-tags duckdb`\)', heading)
        if not (m_default and m_duckdb):
            failures.append(f"{pkg}: dual-build heading format not recognized: {heading!r}")
            return
        claimed_default = float(m_default.group(1))
        claimed_duckdb = float(m_duckdb.group(1))
        live_d = live_default.get(pkg)
        live_k = live_duckdb_analytics.get(pkg)
        if live_d is None or abs(live_d - claimed_default) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed_default}% (default build), go test reports {live_d}")
        if live_k is None or abs(live_k - claimed_duckdb) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed_duckdb}% (-tags duckdb), go test reports {live_k}")
        return

    m_of_package = OF_PACKAGE_PATTERN.search(heading)
    if m_of_package:
        claimed = float(m_of_package.group(1))
        live = live_default.get(pkg)
        if live is None or abs(live - claimed) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed}% of package, go test reports {live}")
        return

    m_bare = re.match(r'([\d.]+)%', heading)
    if m_bare:
        claimed = float(m_bare.group(1))
        live = live_default.get(pkg)
        if live is None or abs(live - claimed) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed}%, go test reports {live}")
        return

    failures.append(f"{pkg}: heading format not recognized, cannot verify: {heading!r} -- add explicit handling in check-coverage-ledger-drift.sh")

for m in root_re.finditer(content):
    heading_text = m.group(0)[len("## "):]
    consumed_heading_text.add(heading_text)
    check_pkg_heading("gopmgr", m.group(1))

for m in generic_pkg_re.finditer(content):
    heading_text = m.group(0)[len("## "):]
    if heading_text in consumed_heading_text:
        continue
    consumed_heading_text.add(heading_text)
    check_pkg_heading(m.group(1), m.group(2))

for heading_text in all_heading_re.findall(content):
    if heading_text in consumed_heading_text:
        continue
    if heading_text in NON_PACKAGE_HEADINGS:
        continue
    if heading_text.startswith(FRONTEND_HEADING_PREFIX):
        continue
    failures.append(f"(unrecognized heading) {heading_text!r}: not a package-coverage heading this script knows how to check, not a known non-package section, and not the externally-tracked Frontend heading -- add explicit handling in check-coverage-ledger-drift.sh")

# Reverse direction: every heading-having package is checked against live
# coverage above, but nothing so far checks that every package WITH live
# coverage has a heading at all. Without this, deleting a heading (or
# adding a new package and never giving it one) passes silently -- the
# same pass-by-omission failure mode this script exists to close, just at
# the section level instead of the row level. Scoped to the full universe
# this script's own `go test ./...` run covers: internal/*, the root
# `gopmgr` package, `scripts`, and `tools/*`. Frontend coverage is a
# different tool's output (vitest, not `go test`) and is out of scope
# here by design (see coverage-frontend.sh), not omission.
def in_scope(pkg):
    return pkg == "gopmgr" or pkg.startswith("internal/") or pkg == "scripts" or pkg.startswith("tools/")

missing_headings = sorted(pkg for pkg in live_default if in_scope(pkg) and pkg not in seen_headings)
for pkg in missing_headings:
    failures.append(f"{pkg}: has live coverage ({live_default[pkg]!r}) but no matching heading in {ledger_path} -- add one")

if failures:
    print(f"check-coverage-ledger-drift: {len(failures)} check(s) failed ({checked} package headings checked):", file=sys.stderr)
    for f in failures:
        print(f"  - {f}", file=sys.stderr)
    sys.exit(1)

all_headings_count = len(all_heading_re.findall(content))
print(f"check-coverage-ledger-drift: all {checked} package headings match live go test -cover output; all {all_headings_count} '## ' headings in {ledger_path} accounted for.")
PY
