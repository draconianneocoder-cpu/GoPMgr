#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Guards against reintroducing the frontend-coverage shadow bug fixed
# 2026-08-15 (see TEST_COVERAGE_LEDGER.md's frontend-coverage note and
# persistence-boundary-strings.test.ts): a Vite `?raw` import pulls its
# target file into the importing test's own module graph under a raw-text
# loader, which suppresses Vitest's `--coverage.all` instrumentation for
# that file. The file then silently reports 0/0 covered statements
# instead of its true uncovered count -- a measurement blind spot, not a
# visible failure, so nothing else catches it. `Dashboard.svelte` and
# three `Help*.svelte` components were shadowed this way for an unknown
# period before discovery.
#
# Scoped to test files (*.test.ts / *.spec.ts) only, not all of
# frontend/src: the shadowing mechanism itself doesn't care whether the
# importer is a test, but every real occurrence found and fixed was in a
# test file reading component/fixture source as text, and a repo-wide
# grep confirmed zero `?raw` imports currently exist outside test files.
# This is a deliberate, narrower boundary chosen to match the actual
# failure mode, not an oversight -- widen it if a legitimate non-test
# `?raw` import is ever added.
#
# A line may keep a `?raw` import only if it carries an inline
# "raw-import-guard-exempt: <reason>" comment explaining why. The reason
# is required, not optional, so an exemption can't become a silent escape
# hatch (same convention as check-no-text-timestamp-ordering.sh).
#
# Matches the import *syntax* (a quoted module specifier ending in
# `?raw`), not the bare token `?raw` -- this file's own header comment,
# and any other prose mentioning `?raw`, must not trip the guard.
#
# Known limitation: this is a single-line grep, matching this package's
# existing convention (see check-no-text-timestamp-ordering.sh) of one
# import per line. A `?raw` specifier split across multiple lines would
# not be caught.
#
# Usage: check-no-raw-import-in-tests.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail=0
while IFS= read -r -d '' file; do
	while IFS=: read -r lineno line; do
		[ -z "$lineno" ] && continue
		case "$line" in
		*raw-import-guard-exempt*) continue ;;
		esac
		echo "FAIL: $file:$lineno: ?raw import shadows this file's coverage.all instrumentation to a false 0/0 -- read it with node:fs.readFileSync instead, or add a 'raw-import-guard-exempt: <reason>' comment if this one is genuinely safe:" >&2
		echo "  $line" >&2
		fail=1
	done < <(grep -nE "(from|import\()[[:space:]]*['\"][^'\"]*\?raw['\"]" "$file" || true)
done < <(find frontend/src -type f \( -name '*.test.ts' -o -name '*.spec.ts' \) -print0)

if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "check-no-raw-import-in-tests: no ?raw import found in a frontend test file."
