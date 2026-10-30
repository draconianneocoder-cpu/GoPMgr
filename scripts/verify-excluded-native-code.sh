#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Fail if a built binary or a source tree carries native code that GoPMgr
# must never ship: the TPC-H and TPC-DS generators DuckDB's prebuilt
# libraries bundle (maintainer decision of 2026-10-26,
# docs/review/TPC-benchmark-exclusion-decision.md). The list of excluded
# archives and code markers is third_party/EXCLUDED-NATIVE-CODE.txt.
#
# Usage:
#   scripts/verify-excluded-native-code.sh [--list FILE] <binary>...
#       Fail if any binary contains a marker, so the excluded code was linked.
#   scripts/verify-excluded-native-code.sh [--list FILE] --tree <dir>
#       Fail if the tree holds an excluded archive or any file with a marker,
#       such as a vendor/ directory made by `go mod vendor`.
#
# Markers are strings only the excluded code contains (class names and
# messages), so they survive symbol stripping. tools/duckdb-notices -check
# proves for every platform module that they are absent from all other
# linked libraries and that no linked library references the excluded code.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
list="$root/third_party/EXCLUDED-NATIVE-CODE.txt"
me="verify-excluded-native-code"

usage() {
	echo "usage: scripts/verify-excluded-native-code.sh [--list FILE] <binary>... | --tree <dir>" >&2
	exit 2
}

if [ "${1:-}" = --list ]; then
	[ $# -ge 2 ] || usage
	list="$2"
	shift 2
fi
tree=""
if [ "${1:-}" = --tree ]; then
	[ $# -eq 2 ] || usage
	tree="$2"
	shift 2
fi
[ -n "$tree" ] || [ $# -ge 1 ] || usage

if [ ! -s "$list" ]; then
	echo "$me: exclusion list missing or empty: $list" >&2
	exit 1
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-excluded-native-code.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# Tolerate CRLF, as a Windows checkout may produce.
tr -d '\r' < "$list" > "$work/list"
sed -n 's/^marker[[:space:]]\{1,\}//p' "$work/list" | sed 's/[[:space:]]*$//' | grep -v '^$' > "$work/markers" || true
sed -n 's/^archive[[:space:]]\{1,\}//p' "$work/list" | sed 's/[[:space:]]*$//' | grep -v '^$' > "$work/archives" || true
if [ ! -s "$work/markers" ] || [ ! -s "$work/archives" ]; then
	echo "$me: $list names no marker or no archive." >&2
	exit 1
fi
markers="$(wc -l < "$work/markers" | tr -d ' ')"

# found_markers <file>: print each marker the file contains, once.
found_markers() {
	LC_ALL=C grep -a -o -F -f "$work/markers" -- "$1" | LC_ALL=C sort -u || true
}

if [ -n "$tree" ]; then
	if [ ! -d "$tree" ]; then
		echo "$me: not a directory: $tree" >&2
		exit 1
	fi
	bad=0
	while IFS= read -r archive; do
		while IFS= read -r hit; do
			[ -n "$hit" ] || continue
			echo "$me: $tree holds excluded archive $hit" >&2
			bad=1
		done <<< "$(find "$tree" -type f -name "$archive" -print)"
	done < "$work/archives"
	while IFS= read -r hit; do
		[ -n "$hit" ] || continue
		echo "$me: $hit contains excluded code markers: $(found_markers "$hit" | paste -sd ',' -)" >&2
		bad=1
	done <<< "$(LC_ALL=C grep -r -l -a -F -f "$work/markers" -- "$tree" || true)"
	if [ "$bad" -ne 0 ]; then
		echo "$me: $tree carries the excluded TPC-H/TPC-DS code; it must not be built from or distributed (see $list)." >&2
		exit 1
	fi
	echo "$me: $tree holds none of the excluded archives and none of the $markers markers"
	exit 0
fi

bad=0
for binary in "$@"; do
	if [ ! -f "$binary" ]; then
		echo "$me: binary not found: $binary" >&2
		exit 1
	fi
	hits="$(found_markers "$binary")"
	if [ -n "$hits" ]; then
		echo "$me: $binary contains excluded code; markers found:" >&2
		sed 's/^/  /' <<< "$hits" >&2
		bad=1
		continue
	fi
	echo "$me: $binary contains none of the $markers excluded-code markers"
done
if [ "$bad" -ne 0 ]; then
	echo "$me: the excluded TPC-H/TPC-DS code was linked; do not distribute this build (see $list)." >&2
	exit 1
fi
