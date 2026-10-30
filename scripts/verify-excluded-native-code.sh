#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Check the reviewed TPC markers in binaries or source trees. A scan error is
# a failure, not evidence of absence. This does not prove arbitrary code absent.
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
cat "$list" > "$work/list"
: > "$work/markers"
: > "$work/archives"
n=0
while IFS= read -r line || [ -n "$line" ]; do
	n=$((n + 1))
	line="${line%$'\r'}"
	line="${line#"${line%%[![:space:]]*}"}"
	line="${line%"${line##*[![:space:]]}"}"
	case "$line" in ''|\#*) continue ;; esac
	kind="${line%% *}"
	value="${line#* }"
	value="${value#"${value%%[![:space:]]*}"}"
	if [ "$value" = "$line" ] || [ -z "$value" ]; then
		echo "$me: $list:$n: invalid exclusion entry" >&2
		exit 1
	fi
	case "$kind" in
		archive)
			case "$value" in *[!a-zA-Z0-9._-]*|.*) echo "$me: $list:$n: invalid archive name" >&2; exit 1 ;; esac
			printf '%s\n' "$value" >> "$work/archives" ;;
		marker) printf '%s\n' "$value" >> "$work/markers" ;;
		*) echo "$me: $list:$n: invalid exclusion entry" >&2; exit 1 ;;
	esac
done < "$work/list"
if [ ! -s "$work/markers" ] || [ ! -s "$work/archives" ]; then
	echo "$me: $list names no marker or no archive." >&2
	exit 1
fi
markers="$(wc -l < "$work/markers" | tr -d ' ')"
found_markers() {
	local status=0
	LC_ALL=C grep -a -o -F -f "$work/markers" -- "$1" > "$work/hits" || status=$?
	if [ "$status" -gt 1 ]; then
		echo "$me: scan failed: $1" >&2
		return 1
	fi
	if ! LC_ALL=C sort -u "$work/hits"; then
		echo "$me: sorting scan results failed: $1" >&2
		return 1
	fi
}
bad=0
if [ -n "$tree" ]; then
	if [ ! -d "$tree" ] || [ -L "$tree" ]; then
		echo "$me: not a directory or a linked directory: $tree" >&2
		exit 1
	fi
	if ! find "$tree" -print0 > "$work/files"; then
		echo "$me: tree traversal failed: $tree" >&2
		exit 1
	fi
	while IFS= read -r -d '' file; do
		if [ -L "$file" ] || { [ ! -d "$file" ] && [ ! -f "$file" ]; }; then
			echo "$me: uninspected nonregular tree entry: $file" >&2
			exit 1
		fi
		[ ! -d "$file" ] || continue
		while IFS= read -r archive; do
			if [ "${file##*/}" = "$archive" ]; then
				echo "$me: $tree holds excluded archive $file" >&2
				bad=1
			fi
		done < "$work/archives"
		if ! hits="$(found_markers "$file")"; then exit 1; fi
		if [ -n "$hits" ]; then
			echo "$me: $file contains excluded code markers: $(paste -sd ',' - <<< "$hits")" >&2
			bad=1
		fi
	done < "$work/files"
	if [ "$bad" -ne 0 ]; then
		echo "$me: $tree matches the excluded archive or marker list; it must not be built from or distributed (see $list)." >&2
		exit 1
	fi
	echo "$me: $tree holds none of the excluded archives and none of the $markers markers"
	exit 0
fi
for binary in "$@"; do
	if [ ! -f "$binary" ]; then
		echo "$me: binary not found: $binary" >&2
		exit 1
	fi
	if ! hits="$(found_markers "$binary")"; then exit 1; fi
	if [ -n "$hits" ]; then
		echo "$me: $binary matches excluded code markers:" >&2
		sed 's/^/  /' <<< "$hits" >&2
		bad=1
		continue
	fi
	echo "$me: $binary contains none of the $markers excluded-code markers"
done
if [ "$bad" -ne 0 ]; then
	echo "$me: excluded code markers were found; do not distribute this build (see $list)." >&2
	exit 1
fi
