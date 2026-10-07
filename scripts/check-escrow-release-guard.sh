#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# ADR-004 release guard (owner decision 2026-10-02). Phase 1 seals every
# enrolled account's data key to an escrow key that administrators hold;
# phase 2 adds the recorded access flow that justifies it. Shipping phase 1
# alone would let an administrator's password unlock every enrolled account
# outside the app with nothing recorded, so the two must ship in the same
# release (docs/design/ADR-004-administrator-access-to-user-data.md).
#
# Fails while non-test Go code seals a DEK: it writes to sealed_deks, or
# calls SealDEK outside internal/crypto. Nothing lifts it automatically;
# phase 2's pull request deletes this guard, its test, its make target, and
# its check-release step.
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
	"$ROOT"/internal/crypto/*) ;;
	*)
		if grep -q 'SealDEK(' "$file"; then
			found="$found ${file#"$ROOT"/}"
		fi
		;;
	esac
done < <(find "$ROOT" -name '*.go' ! -name '*_test.go' \
	! -path '*/node_modules/*' ! -path '*/.tmp/*' ! -path '*/frontend/*' -print)

if [ -n "$found" ]; then
	echo "escrow-release-guard: escrow sealing exists without the recorded access flow (ADR-004 phase 2)." >&2
	echo "escrow-release-guard: sealing in:$found" >&2
	echo "escrow-release-guard: phase 1 and phase 2 must ship in the same release; do not tag until phase 2 lands and removes this guard." >&2
	exit 1
fi
echo "escrow-release-guard: ok"
