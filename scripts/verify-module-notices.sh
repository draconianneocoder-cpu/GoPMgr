#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Fail unless the Go module notices file covers a built GoPMgr binary: the
# Go standard library and every module, at the version the binary embeds.
# The module list comes from the binary itself (`go version -m`), so this
# checks what actually ships, whatever tags or platform built it.
#
# Usage: scripts/verify-module-notices.sh <binary> [notices-file]
# Relative paths are relative to the current directory. The notices file
# defaults to the repository's third_party/GO-MODULE-NOTICES.txt, written by
# `make module-notices`.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

binary="${1:-}"
notices="${2:-$root/third_party/GO-MODULE-NOTICES.txt}"

if [ -z "$binary" ]; then
	echo "usage: scripts/verify-module-notices.sh <binary> [notices-file]" >&2
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

echo "verify-module-notices: $notices covers the Go standard library and all $count modules linked into $binary"
