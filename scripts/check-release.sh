#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Final release gate. Exits non-zero on any failure so it can be
# wired into CI.

set -eu
cd "$(dirname "$0")/.."

GO_PACKAGES="./cmd/... ./internal/..."

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
rm -rf cmd/pmforge/frontend/dist
find . -name .DS_Store -delete

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

if [ -f scripts/frontend-build-budget.sh ]; then
    if ! bash scripts/frontend-build-budget.sh >/dev/null; then
        echo "Frontend build budget failed. Run 'make frontend-build-budget' for details."
        exit 1
    fi
    echo "Frontend build budget passed."
fi

rm -rf cmd/pmforge/frontend/dist
mkdir -p cmd/pmforge/frontend
cp -R frontend/dist cmd/pmforge/frontend/dist

# --- 3. Release gate scope -------------------------------------------
if [ -f scripts/release-gate-scope-check.sh ]; then
    if ! bash scripts/release-gate-scope-check.sh >/dev/null; then
        echo "Release gate scope check failed. Run 'make release-scope' for details."
        exit 1
    fi
    echo "Release gate scope verified."
fi

# --- 4. Frontend stability gate --------------------------------------
if [ -f scripts/frontend-stability-check.sh ]; then
    if ! bash scripts/frontend-stability-check.sh >/dev/null; then
        echo "Frontend stability gate failed. Run 'make frontend-stability' for details."
        exit 1
    fi
    echo "Frontend stability gate passed."
fi

# --- 4b. Frontend runtime smoke check --------------------------------
# Loads and renders App.svelte through the real Vite + Svelte compiler.
# Catches load-time crashes (e.g. a $state rune in a plain .ts) that
# svelte-check and the build pass but that leave #app empty in the app.
if [ -f scripts/frontend-smoke-check.sh ]; then
    if ! bash scripts/frontend-smoke-check.sh >/dev/null; then
        echo "Frontend runtime smoke check failed. Run 'make frontend-smoke' for details."
        exit 1
    fi
    echo "Frontend runtime smoke check passed."
fi

# --- 5. Memory-safety gate -------------------------------------------
if [ -f scripts/memory-safety-scan.sh ]; then
    if ! bash scripts/memory-safety-scan.sh >/dev/null; then
        echo "Memory-safety gate failed. Run 'make memory-scan' for details."
        exit 1
    fi
    echo "Memory-safety gate passed."
else
    echo "memory-safety-scan.sh missing; skipping."
fi

# --- 6. Race detector ------------------------------------------------
if command -v go >/dev/null 2>&1; then
    if ! go test -race $GO_PACKAGES >/dev/null 2>&1; then
        echo "Race detector flagged tests. Run 'make race' for details."
        exit 1
    fi
    echo "Race detector clean."
fi

# --- 7. Test build ---------------------------------------------------
if ! PMFORGE_FRONTEND_BUILT=1 make build >/dev/null; then
    echo "Final build failed."
    exit 1
fi
echo "Build verified."

# --- 8. PDF/A-3 validation gate (hard) ----------------------------------
# Strict: a missing validator/ICC/sample set fails the release rather than
# certifying PDF/A-3 conformance we could not actually verify.
if [ -f scripts/validate-pdfa.sh ]; then
    if ! PMFORGE_PDFA_STRICT=1 bash scripts/validate-pdfa.sh >/dev/null 2>&1; then
        echo "PDF/A-3 validation gate failed. Run 'make check-pdfa' for details."
        exit 1
    fi
    echo "PDF/A-3 validation gate passed."
fi

# --- 9. PAdES local validation gate -----------------------------------
if [ -f scripts/validate-pades.sh ]; then
    if ! bash scripts/validate-pades.sh >/dev/null 2>&1; then
        echo "PAdES local validation gate failed. Run 'make check-pades' for details."
        exit 1
    fi
    echo "PAdES local validation gate passed."
fi

echo "PMForge is ready for release."
