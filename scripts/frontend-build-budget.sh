#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
cd "$(dirname "$0")/.."

log_file="$(mktemp "${TMPDIR:-/tmp}/pmforge-vite-build.XXXXXX")"
trap 'rm -f "$log_file"' EXIT

(cd frontend && npm run build) 2>&1 | tee "$log_file"

if rg -q "Some chunks are larger than" "$log_file"; then
	echo "frontend-budget: Vite emitted a large-chunk warning; split feature islands instead of raising the limit." >&2
	exit 1
fi

main_budget_bytes=500000
for chunk in frontend/dist/assets/index-*.js; do
	if [ ! -e "$chunk" ]; then
		echo "frontend-budget: no main index chunk found in frontend/dist/assets." >&2
		exit 1
	fi
	size_bytes="$(wc -c < "$chunk" | tr -d '[:space:]')"
	if [ "$size_bytes" -gt "$main_budget_bytes" ]; then
		echo "frontend-budget: $chunk is ${size_bytes} bytes, above ${main_budget_bytes} byte main-chunk budget." >&2
		exit 1
	fi
done
