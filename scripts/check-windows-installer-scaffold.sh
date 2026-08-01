#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Validate the source-owned Windows packaging inputs without pretending a
# non-Windows host can compile or launch the NSIS installer.

set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
PROJECT_NSI="$ROOT/build/windows/installer/project.nsi"
INFO_JSON="$ROOT/build/windows/info.json"
MANIFEST="$ROOT/build/windows/wails.exe.manifest"
WORKFLOW="$ROOT/.github/workflows/release.yml"
IGNORE="$ROOT/.gitignore"

fail=0
require_file() {
	local file="$1"
	if [ ! -f "$file" ]; then
		echo "windows-installer-scaffold: required file missing: ${file#"$ROOT/"}" >&2
		fail=1
	fi
}

require_literal() {
	local file="$1"
	local literal="$2"
	local message="$3"
	if ! grep -Fq -- "$literal" "$file"; then
		echo "windows-installer-scaffold: $message" >&2
		fail=1
	fi
}

for file in "$PROJECT_NSI" "$INFO_JSON" "$MANIFEST" "$WORKFLOW" "$IGNORE"; do
	require_file "$file"
done
if [ "$fail" -ne 0 ]; then
	exit "$fail"
fi

require_literal "$PROJECT_NSI" '!include "wails_tools.nsh"' \
	"project.nsi must consume Wails' pinned generated macro file."
require_literal "$PROJECT_NSI" '!define MUI_WELCOMEPAGE_TITLE "Install ${INFO_PRODUCTNAME}"' \
	"project.nsi must retain PMForge-owned welcome branding."
require_literal "$PROJECT_NSI" '!insertmacro MUI_PAGE_LICENSE "..\..\..\LICENSES\GPL-3.0-or-later.txt"' \
	"project.nsi must display the shipped GPL license."
require_literal "$PROJECT_NSI" '!insertmacro wails.files' \
	"project.nsi must install the Wails-produced application binary."
require_literal "$PROJECT_NSI" '!insertmacro wails.writeUninstaller' \
	"project.nsi must register an uninstaller."
require_literal "$PROJECT_NSI" "project databases live in the user's Documents\\PMForge tree" \
	"project.nsi must explain the uninstall data-preservation boundary."

# An uninstaller may remove its install directory and disposable WebView cache,
# but never PMForge's user-owned project tree or project database files.
if grep -Eiq 'RMDir[[:space:]]+/r.*Documents.*PMForge|Delete.*\.pmforge' "$PROJECT_NSI"; then
	echo "windows-installer-scaffold: uninstaller must not delete PMForge project data." >&2
	fail=1
fi

require_literal "$INFO_JSON" '"file_version": "{{.Info.ProductVersion}}"' \
	"info.json must derive file version from wails.json."
require_literal "$INFO_JSON" '"ProductName": "{{.Info.ProductName}}"' \
	"info.json must derive product branding from wails.json."
node -e 'JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"))' "$INFO_JSON" ||
	{ echo "windows-installer-scaffold: info.json is not valid JSON." >&2; fail=1; }

require_literal "$MANIFEST" 'name="dev.pmforge.{{.Name}}"' \
	"Windows manifest must use PMForge's stable identity namespace."
require_literal "$MANIFEST" 'permonitorv2,permonitor' \
	"Windows manifest must retain per-monitor DPI awareness."

for literal in \
	'!/build/windows/info.json' \
	'!/build/windows/wails.exe.manifest' \
	'!/build/windows/installer/project.nsi'; do
	require_literal "$IGNORE" "$literal" \
		".gitignore must expose each source-owned Windows scaffold file."
done

require_literal "$WORKFLOW" 'bash scripts/check-windows-installer-scaffold.sh' \
	"Windows release job must validate the scaffold before Wails packaging."
require_literal "$WORKFLOW" 'bash scripts/validate-windows-nsis-template.sh' \
	"Windows release job must compile the NSIS template fixture before Wails packaging."
require_literal "$WORKFLOW" 'wails build -platform windows/amd64 -tags duckdb' \
	"Windows installer must embed DuckDB analytics."
require_literal "$WORKFLOW" '-ldflags "$PMFORGE_RELEASE_LDFLAGS" -nsis' \
	"Windows installer must embed the reviewed release identity before NSIS packaging."
require_literal "$WORKFLOW" 'bash scripts/verify-duckdb-linked.sh build/bin/pmforge.exe' \
	"Windows release job must verify DuckDB linkage before upload."

# GITHUB Actions executes the run block in source order. Guard that ordering so
# a future edit cannot leave validation present but move it after packaging.
check_line="$(awk '/bash scripts\/check-windows-installer-scaffold\.sh/ { print NR; exit }' "$WORKFLOW")"
validate_line="$(awk '/bash scripts\/validate-windows-nsis-template\.sh/ { print NR; exit }' "$WORKFLOW")"
build_line="$(awk '/wails build -platform windows\/amd64 -tags duckdb/ { print NR; exit }' "$WORKFLOW")"
link_line="$(awk '/bash scripts\/verify-duckdb-linked\.sh build\/bin\/pmforge\.exe/ { print NR; exit }' "$WORKFLOW")"
if [ -n "$check_line" ] && [ -n "$validate_line" ] && [ -n "$build_line" ] && [ -n "$link_line" ] &&
	{
		[ "$check_line" -ge "$validate_line" ] ||
			[ "$validate_line" -ge "$build_line" ] ||
			[ "$build_line" -ge "$link_line" ]
	}; then
	echo "windows-installer-scaffold: workflow order must be scaffold check, NSIS fixture, build, then DuckDB verification." >&2
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit "$fail"
fi

echo "windows-installer-scaffold: source templates and Windows release wiring are consistent."
