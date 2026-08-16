#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Runs the frontend's full-repo statement coverage (every src/**/*.ts and
# .svelte file, not just ones already imported by a test -- --coverage.all
# is required so an entirely-untested component reads as its real uncovered
# count instead of being absent from the denominator) and prints
# "<covered> <total>" statement counts on stdout.
#
# A file can still read as a false 0/0 under --coverage.all despite this:
# see TEST_COVERAGE_LEDGER.md's "Frontend" section note on `?raw` imports
# shadowing coverage instrumentation.
#
# frontend/wailsjs/ (generated Wails bindings) is outside the src/** glob
# and so is never in the denominator; nothing else is currently excluded.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND="$ROOT/frontend"

rm -rf "$FRONTEND/coverage"
(
	cd "$FRONTEND"
	npx vitest run --coverage --coverage.all \
		--coverage.include='src/**/*.{ts,svelte}' \
		--coverage.reporter=json-summary >/dev/null
)

python3 - "$FRONTEND/coverage/coverage-summary.json" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    d = json.load(f)
total = d["total"]["statements"]
print(f'{total["covered"]} {total["total"]}')
PY

rm -rf "$FRONTEND/coverage"
