#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Final release gate. Exits non-zero on any failure so it can be
# wired into CI.

set -eu
cd "$(dirname "$0")/.."

echo "Running Final Release Gates for PMForge..."

# --- 1. Version consistency ------------------------------------------
APP_VERSION=$(grep -oE 'Version *= *"[^"]+"' internal/cli/parser.go | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
WAILS_VERSION=$(grep -oE '"productVersion" *: *"[^"]+"' wails.json | sed -E 's/.*"([^"]+)"$/\1/')

if [ "$APP_VERSION" != "$WAILS_VERSION" ]; then
    echo "Version mismatch: CLI ($APP_VERSION) vs Wails ($WAILS_VERSION)"
    exit 1
fi
echo "Versions match: $APP_VERSION"

# --- 2. REUSE / SPDX licensing ---------------------------------------
if ! command -v reuse >/dev/null 2>&1; then
    echo "reuse tool not installed; skipping license check."
    echo "  Install with:  pip install reuse"
else
    if ! reuse lint >/dev/null; then
        echo "REUSE/SPDX compliance failed. Run 'reuse lint' for details."
        exit 1
    fi
    echo "Licensing compliant."
fi

# --- 3. Memory-safety gate -------------------------------------------
if [ -x scripts/memory-safety-scan.sh ]; then
    if ! bash scripts/memory-safety-scan.sh >/dev/null; then
        echo "Memory-safety gate failed. Run 'make memory-scan' for details."
        exit 1
    fi
    echo "Memory-safety gate passed."
else
    echo "memory-safety-scan.sh missing; skipping."
fi

# --- 4. Race detector ------------------------------------------------
if command -v go >/dev/null 2>&1; then
    if ! go test -race ./... >/dev/null 2>&1; then
        echo "Race detector flagged tests. Run 'make race' for details."
        exit 1
    fi
    echo "Race detector clean."
fi

# --- 5. Test build ---------------------------------------------------
if ! make build >/dev/null; then
    echo "Final build failed."
    exit 1
fi
echo "Build verified."

echo "PMForge is ready for release."
