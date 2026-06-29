#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPORT="$ROOT/.tmp/pmforge-pades-trusted-source/trusted-source-validation-report.txt"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

rm -rf "$ROOT/.tmp/pmforge-pades-trusted-source"
bash "$ROOT/scripts/validate-pades-trusted-source.sh" >/tmp/pmforge-pades-trusted-test.out

[ -s "$REPORT" ] || fail "trusted-source report was not written"
if ! grep -q "status=NOT_CONFIGURED" "$REPORT"; then
	cat "$REPORT" >&2
	fail "missing NOT_CONFIGURED status"
fi
if grep -q "status=PASS" "$REPORT"; then
	cat "$REPORT" >&2
	fail "unconfigured trusted-source validation claimed PASS"
fi

if PMFORGE_PADES_TRUSTED_REQUIRED=1 bash "$ROOT/scripts/validate-pades-trusted-source.sh" >/tmp/pmforge-pades-trusted-required-test.out 2>&1; then
	fail "required trusted-source validation passed without a configured PDF"
fi

if ! grep -q "status=NOT_CONFIGURED" "$REPORT"; then
	cat "$REPORT" >&2
	fail "required-mode report did not preserve NOT_CONFIGURED status"
fi

echo "validate-pades-trusted-source tests passed."
