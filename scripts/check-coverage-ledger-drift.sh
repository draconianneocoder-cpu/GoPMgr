#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# TEST_COVERAGE_LEDGER.md's "## `internal/X` — NN.N%" headings are
# hand-written and nothing previously checked them against reality.
# check-coverage-ledger-current.sh only verifies a *_test.go file is
# *mentioned* somewhere in the ledger -- it says nothing about whether a
# heading's claimed percentage still matches the package's actual
# coverage. That drift is not hypothetical: a 2026-08-10 audit found
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
# missing test-file mentions.
#
# Usage: check-coverage-ledger-drift.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
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

heading_re = re.compile(r'^## `(internal/[a-zA-Z0-9_/]+)` — (.+)$', re.M)

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
seen_headings = set()

for pkg, heading in heading_re.findall(content):
    checked += 1
    seen_headings.add(pkg)
    if pkg in NO_STATEMENTS:
        live = live_default.get(pkg, "MISSING")
        if live is not None:
            failures.append(f"{pkg}: heading claims 'no statements' but go test reports {live!r} -- package gained testable statements, update the heading")
        continue

    if pkg in DUAL_BUILD:
        m_default = re.search(r'([\d.]+)% \(default build\)', heading)
        m_duckdb = re.search(r'([\d.]+)% \(`-tags duckdb`\)', heading)
        if not (m_default and m_duckdb):
            failures.append(f"{pkg}: dual-build heading format not recognized: {heading!r}")
            continue
        claimed_default = float(m_default.group(1))
        claimed_duckdb = float(m_duckdb.group(1))
        live_d = live_default.get(pkg)
        live_k = live_duckdb_analytics.get(pkg)
        if live_d is None or abs(live_d - claimed_default) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed_default}% (default build), go test reports {live_d}")
        if live_k is None or abs(live_k - claimed_duckdb) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed_duckdb}% (-tags duckdb), go test reports {live_k}")
        continue

    m_of_package = OF_PACKAGE_PATTERN.search(heading)
    if m_of_package:
        claimed = float(m_of_package.group(1))
        live = live_default.get(pkg)
        if live is None or abs(live - claimed) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed}% of package, go test reports {live}")
        continue

    m_bare = re.match(r'([\d.]+)%', heading)
    if m_bare:
        claimed = float(m_bare.group(1))
        live = live_default.get(pkg)
        if live is None or abs(live - claimed) > 0.05:
            failures.append(f"{pkg}: heading claims {claimed}%, go test reports {live}")
        continue

    failures.append(f"{pkg}: heading format not recognized, cannot verify: {heading!r} -- add explicit handling in check-coverage-ledger-drift.sh")

# Reverse direction: every heading-having package is checked against live
# coverage above, but nothing so far checks that every internal/* package
# WITH live coverage has a heading at all. Without this, deleting a
# heading (or adding a new package and never giving it one) passes
# silently -- the same pass-by-omission failure mode this script exists
# to close, just at the section level instead of the row level. Scoped to
# internal/* only, matching the heading regex above; the root "gopmgr"
# package (also present in live_default) has no internal/ prefix and is
# not part of this check.
missing_headings = sorted(
    pkg for pkg in live_default
    if pkg.startswith("internal/") and pkg not in seen_headings
)
for pkg in missing_headings:
    failures.append(f"{pkg}: has live coverage ({live_default[pkg]!r}) but no '## `{pkg}`' heading in {ledger_path} -- add one")

if failures:
    print(f"check-coverage-ledger-drift: {len(failures)} of {checked} headings failed:", file=sys.stderr)
    for f in failures:
        print(f"  - {f}", file=sys.stderr)
    sys.exit(1)

print(f"check-coverage-ledger-drift: all {checked} internal/* headings match live go test -cover output.")
PY
