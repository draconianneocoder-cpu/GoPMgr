#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Compile project.nsi with harmless fixture binaries. This proves NSIS syntax
# and Wails macro compatibility on any host with makensis, but it does not
# replace a native Windows application build or installer launch.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if ! command -v makensis >/dev/null 2>&1; then
	echo "validate-windows-nsis-template: SKIP (makensis is not installed)."
	exit 0
fi

WAILS_DIR="$(go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v2)"
WAILS_WINDOWS="$WAILS_DIR/pkg/buildassets/build/windows"
if [ ! -f "$WAILS_WINDOWS/installer/wails_tools.nsh" ] ||
	[ ! -f "$WAILS_WINDOWS/icon.ico" ]; then
	echo "validate-windows-nsis-template: pinned Wails installer assets are unavailable." >&2
	exit 1
fi

TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-nsis-template.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT
INSTALLER_DIR="$TEST_ROOT/build/windows/installer"
mkdir -p "$INSTALLER_DIR/tmp" "$TEST_ROOT/build/bin" "$TEST_ROOT/LICENSES"

cp "$ROOT/build/windows/installer/project.nsi" "$INSTALLER_DIR/project.nsi"
cp "$WAILS_WINDOWS/icon.ico" "$TEST_ROOT/build/windows/icon.ico"
cp "$ROOT/LICENSES/GPL-3.0-or-later.txt" "$TEST_ROOT/LICENSES/GPL-3.0-or-later.txt"

# Resolve the same Wails project fields used by the real build. Range blocks
# are empty because PMForge currently declares no file associations/protocols.
sed \
	-e 's|{{.Name}}|pmforge|g' \
	-e 's|{{.Info.CompanyName}}|The PMForge Contributors|g' \
	-e 's|{{.Info.ProductName}}|PMForge|g' \
	-e 's|{{.Info.ProductVersion}}|1.1.0|g' \
	-e 's|{{.Info.Copyright}}|Copyright (C) 2026 The PMForge Contributors|g' \
	"$WAILS_WINDOWS/installer/wails_tools.nsh" |
	awk '
		/{{range[[:space:]]/ { in_range = 1; next }
		in_range && /{{end}}/ { in_range = 0; next }
		!in_range { print }
	' >"$INSTALLER_DIR/wails_tools.nsh"

# NSIS only embeds these fixtures; they are never executed during compilation.
: >"$INSTALLER_DIR/tmp/MicrosoftEdgeWebview2Setup.exe"
: >"$TEST_ROOT/build/bin/pmforge.exe"

(
	cd "$INSTALLER_DIR"
	makensis -V2 -DARG_WAILS_AMD64_BINARY=../../bin/pmforge.exe project.nsi
)

output="$TEST_ROOT/build/bin/pmforge-amd64-installer.exe"
if [ ! -s "$output" ]; then
	echo "validate-windows-nsis-template: makensis did not produce the expected fixture installer." >&2
	exit 1
fi

echo "validate-windows-nsis-template: NSIS 3 template compilation passed."
