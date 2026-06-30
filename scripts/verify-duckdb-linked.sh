#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
cd "$(dirname "$0")/.."

binary="${1:-}"
if [ -z "$binary" ]; then
	binary="$(find build/bin -path '*.app/Contents/MacOS/*' -type f -perm -111 2>/dev/null | head -1 || true)"
fi
if [ -z "$binary" ] && [ -f build/bin/pmforge ]; then
	binary="build/bin/pmforge"
fi
if [ -z "$binary" ] || [ ! -f "$binary" ]; then
	echo "verify-duckdb-linked: PMForge binary not found under build/bin." >&2
	exit 1
fi

metadata="$(go version -m "$binary" 2>/dev/null || true)"
if ! printf '%s\n' "$metadata" | rg -q $'\tbuild\t-tags=.*duckdb'; then
	echo "verify-duckdb-linked: $binary was not built with the duckdb tag." >&2
	exit 1
fi
if ! printf '%s\n' "$metadata" | rg -q $'\tdep\tgithub.com/duckdb/duckdb-go/v2\t'; then
	echo "verify-duckdb-linked: $binary does not link github.com/duckdb/duckdb-go/v2." >&2
	exit 1
fi
if ! printf '%s\n' "$metadata" | rg -q $'\tdep\tgithub.com/duckdb/duckdb-go-bindings/lib/'; then
	echo "verify-duckdb-linked: $binary does not link a platform DuckDB binding library." >&2
	exit 1
fi

echo "verify-duckdb-linked: DuckDB analytics is embedded in $binary"
