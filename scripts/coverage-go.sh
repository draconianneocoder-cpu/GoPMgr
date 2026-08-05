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

pkgs="$(go list ./... | grep -v '/frontend/node_modules/')"
# shellcheck disable=SC2086
go test $tags_arg -coverprofile="$profile" $pkgs >/dev/null

exclude_file="$ROOT/scripts/coverage-exclude-go.txt"
filtered="$profile.filtered"
{
	head -1 "$profile"
	tail -n +2 "$profile" | grep -Ev -f <(grep -Ev '^\s*(#|$)' "$exclude_file") || true
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
