#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
# Package the existing build/bin/gopmgr.exe using GoPMgr's owned NSIS entry.
# Wails v3 assets are generated only in fresh private staging. Native Windows
# install/upgrade/remove and trust qualification are separate release gates.
set -euo pipefail
cd "$(dirname "$0")/.."
root="$(pwd)"
cli="$root/.tmp/wails-v3/bin/wails3"
if [ "$(go env GOHOSTOS)" = windows ]; then cli="$cli.exe"; fi
fail() { echo "package-windows: $*" >&2; exit 1; }
[ -x "$cli" ] || fail 'task-local Wails v3.1.0 CLI is required'
cli_version="$(go version -m "$cli" | awk '$1 == "mod" && $2 == "github.com/wailsapp/wails/v3" { print $3 }')"
[ "$cli_version" = v3.1.0 ] || fail "requires CLI v3.1.0; found ${cli_version:-unknown}"
command -v makensis >/dev/null 2>&1 || fail 'makensis is required (release tool version is pinned in scripts/release-tool-versions.env)'
scope="${WAILS_INSTALL_SCOPE:-machine}"
case "$scope" in machine|user) ;; *) fail 'WAILS_INSTALL_SCOPE must be machine or user';; esac
[ -s build/bin/gopmgr.exe ] || fail 'build/bin/gopmgr.exe missing; run scripts/wails-build.sh on Windows first'
# Inspect actual PE architecture so a stale non-Windows/wrong-architecture
# artifact cannot silently be wrapped as an installer for the requested target.
arch="$(python3 - <<'PY'
import struct, sys
with open('build/bin/gopmgr.exe', 'rb') as f:
    header = f.read(64)
    if len(header) != 64 or header[:2] != b'MZ':
        sys.exit('package-windows: expected a Windows PE executable')
    f.seek(struct.unpack_from('<I', header, 60)[0])
    pe = f.read(6)
    if len(pe) != 6 or pe[:4] != b'PE\0\0':
        sys.exit('package-windows: invalid Windows PE header')
    machine = struct.unpack_from('<H', pe, 4)[0]
    if machine not in (0x8664, 0xaa64):
        sys.exit('package-windows: only Windows amd64/arm64 is supported')
    print({0x8664: 'amd64', 0xaa64: 'arm64'}[machine])
PY
)"
[ -z "${GOARCH:-}" ] || [ "$GOARCH" = "$arch" ] || fail "PE architecture $arch does not match requested GOARCH=$GOARCH"
# Existing macro identity/registry keys come from config.info. A clean
# three-component product version is required by the NSIS .0 resource suffix.
python3 - <<'PY'
import json, pathlib, re, sys
config = json.loads('\n'.join(line for line in pathlib.Path('build/config.yml').read_text().splitlines() if not line.startswith('#')))
if config.get('version') != '3' or not re.fullmatch(r'\d+\.\d+\.\d+', config['info']['version']):
    sys.exit('package-windows: config.info.version must be a clean three-component version')
PY
mkdir -p build/bin
stage="$(mktemp -d "$root/build/bin/.windows-package.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
installer="$stage/build/windows/installer"
mkdir -p "$installer" "$stage/build/bin" "$stage/LICENSES" "$stage/third_party/go-sqlcipher" "$stage/internal/fonts/assets"
# Freeze the binary and notices used by this one packaging operation.
cp build/bin/gopmgr.exe "$stage/build/bin/gopmgr.exe"
cp third_party/GO-MODULE-NOTICES.txt third_party/FRONTEND-NOTICES.txt "$stage/third_party/"
bash scripts/verify-module-notices.sh "$stage/build/bin/gopmgr.exe" "$stage/third_party/GO-MODULE-NOTICES.txt"
cp LICENSES/GPL-3.0-or-later.txt "$stage/LICENSES/"
cp third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt "$stage/third_party/go-sqlcipher/"
cp internal/fonts/assets/FONT-NOTICES.txt "$stage/internal/fonts/assets/"
cp build/windows/installer/project.nsi "$installer/project.nsi"
# Selective generation from the pinned CLI: do not run this against build/,
# where upstream templates could replace owned plist/installer policies.
"$cli" update build-assets -name gopmgr -binaryname gopmgr -config "$root/build/config.yml" -dir "$stage/generated"
cp "$stage/generated/windows/nsis/wails_tools.nsh" "$installer/wails_tools.nsh"
"$cli" generate icons -input build/appicon.png -windowsfilename "$stage/build/windows/icon.ico" -macfilename "$stage/unused.icns"
# v3.1's NSIS macro embeds this file directly from the installer directory.
# Fresh staging avoids generate's skip-if-present behavior retaining old bytes.
"$cli" generate webview2bootstrapper -dir "$installer"
[ -s "$installer/MicrosoftEdgeWebview2Setup.exe" ] || fail 'bootstrapper generation produced no file'
nsis_binary="$stage/build/bin/gopmgr.exe"
if command -v cygpath >/dev/null 2>&1; then nsis_binary="$(cygpath -w "$nsis_binary")"; fi
case "$arch" in amd64) arch_define=AMD64;; arm64) arch_define=ARM64;; esac
(
    cd "$installer"
    makensis -V2 "-DWAILS_INSTALL_SCOPE=$scope" "-DARG_WAILS_${arch_define}_BINARY=$nsis_binary" project.nsi
)
output="gopmgr-${arch}-installer.exe"
[ -s "$stage/build/bin/$output" ] || fail "makensis succeeded without expected $output"
mv "$stage/build/bin/$output" "build/bin/$output"
echo "package-windows: wrote build/bin/$output ($scope scope; native Windows qualification required)"
