#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Runs `go test -cover` for one build configuration ("default" or "duckdb"),
# strips the excluded-file lines listed in scripts/coverage-exclude-go.txt
# from the resulting profile, and prints "<covered> <total>" statement
# counts on stdout (not a percentage -- the ratchet needs exact integers to
# detect a one-statement regression, which a rounded percentage can hide).
#
# gopmgr/frontend/node_modules/flatted/... is vendored third-party Go code
# that `go list ./...` sweeps in incidentally; it is dropped from the
# package list before `go test` ever runs, not just from the profile, so it
# never counts as either covered or uncovered work.
#
# Usage: coverage-go.sh <default|duckdb> [profile-out-path]

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# `go list ./...` and `go test` below resolve against the CALLER's working
# directory, not $ROOT -- pin it explicitly so this script always measures
# this repo's own module regardless of what CWD happened to be active when
# it was invoked. Identified as a latent correctness gap (not a proven
# cause) during a 2026-08/2026-08-10 investigation into an intermittent
# coverage-ratchet flake -- an occasional 2-3x inflation in the reported
# statement count, most recently reproduced as a triplicated coverage
# profile during the 2026-08-10 sigma.go increment (see
# .agent_memory/db-sigma-crud-increment-2026-08-10.md, gitignored, or
# `git log --all -p -- DEVELOPER_HANDBOOK.md` for the original
# "coverage-ratchet exclude-filter flake" investigation entry, trimmed
# from this file's public-facing cleanup in commit ebfd971). Status:
# unconfirmed, not fixed -- `coverage-ratchet.sh`'s own comment on
# "never write a worse mark" is why a flaked run can't corrupt the
# baseline; re-run once on an anomalous result before treating it as real.
cd "$ROOT"
variant="${1:?usage: coverage-go.sh <default|duckdb> [profile-out-path]}"

# Plain string, not an array: bash 3.2 (macOS's shipped /bin/bash) treats an
# empty array expansion as an unbound-variable error under `set -u`.
tags_arg=""
case "$variant" in
default) ;;
duckdb) tags_arg="-tags duckdb" ;;
*)
	echo "coverage-go: unknown variant '$variant' (want default|duckdb)" >&2
	exit 1
	;;
esac

mkdir -p "$ROOT/.tmp"
profile="${2:-$ROOT/.tmp/coverage-go-$variant.out}"

# The .claude/worktrees/ exclusion guards a narrow case `go list`'s own
# go.mod-boundary handling doesn't cover on its own: a nested checkout
# under .claude/worktrees/ (left by a background agent session) that lacks
# its own go.mod would otherwise be swept in as part of this module. Every
# worktree observed in this repo so far has carried its own go.mod (it's a
# full checkout of tracked files, and go.mod predates every worktree this
# project has created), so this has not been confirmed to fire in practice
# -- it is defense-in-depth, not a confirmed fix for anything.
pkgs="$(go list ./... | grep -v '/frontend/node_modules/' | grep -v '/\.claude/worktrees/')"
# Package count and the exact list are printed for diagnosability, not
# correctness: an anomalous coverage-ratchet run (see the flake entry
# above) is otherwise unreconstructable after the fact, since a background
# session's stray worktree can be auto-cleaned before anyone investigates.
pkg_count="$(echo "$pkgs" | wc -l | tr -d ' ')"
echo "coverage-go ($variant): $pkg_count packages" >&2
echo "$pkgs" >"$ROOT/.tmp/coverage-go-$variant.pkgs"
# shellcheck disable=SC2086
go test $tags_arg -coverprofile="$profile" $pkgs >/dev/null

exclude_file="$ROOT/scripts/coverage-exclude-go.txt"
filtered="$profile.filtered"
# Patterns are written to a real temp file first, not read via
# `grep -f <(...)` process substitution: under macOS's shipped bash 3.2,
# `grep -f` opening a not-yet-fully-written pipe can read an empty pattern
# file, silently letting every line through. This is hardening, not a
# confirmed fix for the flake described above (line ~24): that
# investigation ruled this mechanism OUT as the actual cause (the exclude
# list only ever touches ~3 statements, nowhere near the flake's observed
# ~20,000+ statement inflations, and 5000 stress-test iterations of this
# exact pipeline reproduced zero anomalies) -- kept anyway on general
# robustness grounds.
tmp_patterns="$(mktemp)"
trap 'rm -f "$tmp_patterns"' EXIT
grep -Ev '^\s*(#|$)' "$exclude_file" >"$tmp_patterns"
{
	head -1 "$profile"
	tail -n +2 "$profile" | grep -Ev -f "$tmp_patterns" || true
} >"$filtered"
mv "$filtered" "$profile"

awk '
	NR == 1 { next }
	{
		n = $2   # numStmt
		c = $3   # count
		total += n
		if (c > 0) covered += n
	}
	END { printf "%d %d\n", covered + 0, total + 0 }
' "$profile"
