#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Guards against reintroducing the timestamp lexicographic-ordering hazard
# closed in 2026-08 (see internal/db/timestamps.go, timestampCapture): an
# ORDER BY on the bare created_at/updated_at TEXT column instead of its
# _unixnano sibling. Numeric comparison on the _unixnano column is what
# makes the ordering hazard structurally impossible -- it cannot be
# defeated by a string-formatting mistake the way the TEXT column's
# lexicographic order could be. A new ORDER BY that reaches for the TEXT
# column silently reintroduces that hazard instead of failing loudly,
# which is exactly what this script exists to prevent.
#
# A line may reference the bare TEXT column in an ORDER BY only if it
# carries an inline "timestamp-order-guard-exempt: <reason>" comment
# explaining why. The reason is required, not optional, so an exemption
# can't become a silent escape hatch -- see internal/db/sigma.go's
# sigma_projects query for the one case that actually needs it
# (strftime-managed, fixed 3-digit fraction, never subject to the hazard
# by construction, so there is nothing to fix there).
#
# Known limitation: this is a single-line grep, matching this package's
# existing convention of keeping each ORDER BY clause on one line. A
# future ORDER BY that wraps across multiple lines would not be caught --
# not a silent gap, just documented here so it doesn't look like a fully
# general guarantee.
#
# Usage: check-no-text-timestamp-ordering.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail=0
while IFS= read -r -d '' file; do
	while IFS=: read -r lineno line; do
		[ -z "$lineno" ] && continue
		trimmed="${line#"${line%%[![:space:]]*}"}"
		case "$trimmed" in
		"//"*) continue ;; # doc-comment prose discussing ORDER BY, not a live query
		esac
		case "$line" in
		*timestamp-order-guard-exempt*) continue ;;
		esac
		echo "FAIL: $file:$lineno: ORDER BY references a bare TEXT timestamp column -- use its _unixnano sibling, or add a 'timestamp-order-guard-exempt: <reason>' comment if this one is genuinely safe:" >&2
		echo "  $line" >&2
		fail=1
	done < <(grep -nE 'ORDER BY.*\b(created_at|updated_at)\b' "$file" || true)
done < <(find internal/db -maxdepth 1 -name '*.go' ! -name '*_test.go' -print0)

if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "check-no-text-timestamp-ordering: no bare TEXT-column ORDER BY found."
