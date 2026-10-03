#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# ADR-004 release guard (owner decision 2026-10-02). Administrators can open
# another account's data in the app, recorded in account history, but the
# accepted design also tells the user at their next sign-in, and that notice
# is phase 3 (docs/design/ADR-004-administrator-access-to-user-data.md). No
# release may ship access its users are not told about.
#
# Fails while non-test Go code seals a DEK (writes sealed_deks, or calls
# SealDEK outside internal/crypto) or, outside internal/users, calls
# OpenUserForAdmin. Sealing exists from phase 1 on and opening needs it, so
# renaming or wrapping the access call cannot lift the guard, and neither
# can removing access while sealing stays (which would bring back phase 1's
# unrecorded-access risk). Nothing lifts it short of deleting it; the pull
# request that adds the sign-in notice deletes this guard, its test, its make
# targets, and its check-release step.
#
# GOPMGR_GUARD_ROOT points the check at another tree, for its own test.

set -euo pipefail

ROOT="${GOPMGR_GUARD_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"

found=""
while IFS= read -r file; do
	if grep -Eiq '(INSERT( OR [A-Z]+)? INTO|UPDATE|REPLACE INTO) +sealed_deks' "$file"; then
		found="$found ${file#"$ROOT"/}"
		continue
	fi
	case "$file" in
	"$ROOT"/internal/crypto/*) continue ;;
	esac
	if grep -q 'SealDEK(' "$file"; then
		found="$found ${file#"$ROOT"/}"
		continue
	fi
	case "$file" in
	"$ROOT"/internal/users/*) continue ;;
	esac
	if grep -q 'OpenUserForAdmin(' "$file"; then
		found="$found ${file#"$ROOT"/}"
	fi
done < <(find "$ROOT" -name '*.go' ! -name '*_test.go' \
	! -path '*/node_modules/*' ! -path '*/.tmp/*' ! -path '*/frontend/*' -print)

if [ -n "$found" ]; then
	echo "access-notice-release-guard: administrators can open users' data, but users are not yet told at sign-in (ADR-004 phase 3)." >&2
	echo "access-notice-release-guard: escrow sealing or access in:$found" >&2
	echo "access-notice-release-guard: do not tag until the sign-in notice lands and removes this guard." >&2
	exit 1
fi
echo "access-notice-release-guard: ok"
