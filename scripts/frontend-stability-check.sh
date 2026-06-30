#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -eu
cd "$(dirname "$0")/.."

fail=0

if ! (cd frontend && npx svelte-check --tsconfig ./tsconfig.json --fail-on-warnings); then
	echo "frontend-stability: svelte-check must be clean, including warnings." >&2
	fail=1
fi

if ! (cd frontend && npm run test:bug-report-regressions); then
	echo "frontend-stability: bug report regression checks must pass." >&2
	fail=1
fi

if rg -n "^import \\* as XLSX from ['\"]xlsx['\"];" frontend/src/lib/components/sigma >/dev/null; then
	echo "frontend-stability: Sigma views must lazy-load xlsx instead of statically importing it." >&2
	rg -n "^import \\* as XLSX from ['\"]xlsx['\"];" frontend/src/lib/components/sigma >&2
	fail=1
fi

if rg -n "on:[a-zA-Z][a-zA-Z0-9_-]*=" frontend/src/lib/components/sigma >/dev/null; then
	echo "frontend-stability: Sigma components must use Svelte 5 callback props/event attributes, not deprecated on: directives." >&2
	rg -n "on:[a-zA-Z][a-zA-Z0-9_-]*=" frontend/src/lib/components/sigma >&2
	fail=1
fi

if rg -n "createEventDispatcher" frontend/src/lib/components/sigma >/dev/null; then
	echo "frontend-stability: Sigma components must use Svelte 5 callback props instead of createEventDispatcher." >&2
	rg -n "createEventDispatcher" frontend/src/lib/components/sigma >&2
	fail=1
fi

for action in openFiveWhys addCause; do
	if rg -n "onclick=.*$action" frontend/src/lib/components/sigma/SigmaFishbone.svelte >/dev/null &&
		! rg -n "onkeydown=.*$action" frontend/src/lib/components/sigma/SigmaFishbone.svelte >/dev/null; then
		echo "frontend-stability: clickable Sigma SVG action '$action' must share a keyboard activation handler." >&2
		fail=1
	fi
	if rg -n "onclick=.*$action" frontend/src/lib/components/sigma/SigmaFishbone.svelte >/dev/null &&
		! rg -n "role=\"button\"" frontend/src/lib/components/sigma/SigmaFishbone.svelte >/dev/null; then
		echo "frontend-stability: clickable Sigma SVG action '$action' must expose button semantics." >&2
		fail=1
	fi
done

if rg -n "onclick=\\{\\(\\) => (openFiveWhys|addCause)" frontend/src/lib/components/sigma/SigmaFishbone.svelte >/dev/null &&
	! rg -n "activateSvgAction" frontend/src/lib/components/sigma/SigmaFishbone.svelte >/dev/null; then
	echo "frontend-stability: clickable Sigma SVG text must share keyboard activation handlers." >&2
	rg -n "onclick=\\{\\(\\) => (openFiveWhys|addCause)" frontend/src/lib/components/sigma/SigmaFishbone.svelte >&2
	fail=1
fi

exit "$fail"
