#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Fail unless the Go module notices file covers a built GoPMgr binary: the
# Go standard library and every module, at the version the binary embeds.
# The module list comes from the binary itself (`go version -m`), so this
# checks what actually ships, whatever tags or platform built it.
#
# A binary that links DuckDB's prebuilt libraries
# (github.com/duckdb/duckdb-go-bindings/lib/*) also needs the DuckDB native
# notices to cover that exact module version.
#
# Every binary must also be free of the native code GoPMgr excludes, the
# TPC-H and TPC-DS generators (scripts/verify-excluded-native-code.sh), so
# each packaging path that runs this check also runs that one.
#
# Usage: scripts/verify-module-notices.sh <binary> [notices-file] [duckdb-native-notices-file]
# Relative paths are relative to the current directory. The files default to
# the repository's third_party/GO-MODULE-NOTICES.txt (`make module-notices`)
# and third_party/DUCKDB-NATIVE-NOTICES.txt (`make duckdb-notices`).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

binary="${1:-}"
notices="${2:-$root/third_party/GO-MODULE-NOTICES.txt}"
native_notices="${3:-$root/third_party/DUCKDB-NATIVE-NOTICES.txt}"

if [ -z "$binary" ]; then
	echo "usage: scripts/verify-module-notices.sh <binary> [notices-file] [duckdb-native-notices-file]" >&2
	exit 2
fi
if [ ! -f "$binary" ]; then
	echo "verify-module-notices: binary not found: $binary" >&2
	exit 1
fi
if [ ! -s "$notices" ]; then
	echo "verify-module-notices: notices file missing or empty: $notices (run 'make module-notices')." >&2
	exit 1
fi

if ! metadata="$(go version -m "$binary" 2>/dev/null)"; then
	echo "verify-module-notices: $binary carries no Go build information ('go version -m' failed)." >&2
	exit 1
fi

# Tolerate CRLF line endings, as a Windows checkout may produce. Here-strings,
# not pipes: under pipefail, grep -q closing a pipe early fails the writer.
notice_lines="$(tr -d '\r' < "$notices")"

if ! grep -Fqx 'Module: std' <<< "$notice_lines"; then
	echo "verify-module-notices: $notices has no Go standard library section." >&2
	exit 1
fi

# `go version -m` prints one tab-indented "dep<TAB>path<TAB>version" line per
# linked module. A replaced module keeps its original path and version.
deps="$(printf '%s\n' "$metadata" | awk -F'\t' '$2 == "dep" { print $3 " " $4 }')"
missing=()
count=0
while IFS= read -r dep; do
	[ -n "$dep" ] || continue
	count=$((count + 1))
	if ! grep -Fqx "Module: $dep" <<< "$notice_lines"; then
		missing+=("$dep")
	fi
done <<< "$deps"

if [ "${#missing[@]}" -gt 0 ]; then
	echo "verify-module-notices: $notices lacks notices for ${#missing[@]} module(s) linked into $binary:" >&2
	printf '  %s\n' "${missing[@]}" >&2
	echo "verify-module-notices: run 'make module-notices', review the new texts, and commit them." >&2
	exit 1
fi

# DuckDB's prebuilt libraries carry third-party code their module's license
# does not cover; the native notices must name each linked library module.
native_deps="$(grep '^github\.com/duckdb/duckdb-go-bindings/lib/' <<< "$deps" || true)"
if [ -n "$native_deps" ]; then
	if [ ! -s "$native_notices" ]; then
		echo "verify-module-notices: $binary links DuckDB's native libraries, but $native_notices is missing or empty (run 'make duckdb-notices')." >&2
		exit 1
	fi
	native_lines="$(tr -d '\r' < "$native_notices")"
	while IFS= read -r dep; do
		if ! grep -Fqx "Covers: $dep" <<< "$native_lines"; then
			echo "verify-module-notices: $native_notices does not cover $dep, which $binary links." >&2
			echo "verify-module-notices: review the new DuckDB release's bundled components and update tools/duckdb-notices (LICENSES.md)." >&2
			exit 1
		fi
	done <<< "$native_deps"
fi

# Release-blocking (docs/review/TPC-benchmark-exclusion-decision.md).
if ! bash "$root/scripts/verify-excluded-native-code.sh" "$binary"; then
	exit 1
fi

echo "verify-module-notices: $notices covers the Go standard library and all $count modules linked into $binary"
if [ -n "$native_deps" ]; then
	echo "verify-module-notices: $native_notices covers DuckDB's native libraries ($(tr '\n' ' ' <<< "$native_deps" | sed 's/ $//'))"
fi
