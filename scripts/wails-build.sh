#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
# Native CGO build through the reviewed Wails v3 tasks. Publish only after
# compilation, bundle construction and (on macOS) ad-hoc verification pass.
# Usage: scripts/wails-build.sh [DEV=true] [ARCH=arm64] [EXTRA_LDFLAGS=...]
set -euo pipefail
cd "$(dirname "$0")/.."
root="$(pwd)"
cli="$root/.tmp/wails-v3/bin/wails3"
if [ "$(go env GOHOSTOS)" = windows ]; then cli="$cli.exe"; fi
fail() { echo "wails-build: $*" >&2; exit 1; }
[ -x "$cli" ] || fail 'missing task-local .tmp/wails-v3/bin/wails3 (requires v3.1.0)'
cli_version="$(go version -m "$cli" | awk '$1 == "mod" && $2 == "github.com/wailsapp/wails/v3" {print $3}')"
[ "$cli_version" = v3.1.0 ] || fail "requires CLI v3.1.0; found ${cli_version:-unknown}"
# The Wails version gate checks runtime/npm/CLI consistency together.
# This runner also fails closed if invoked directly with legacy CLI options.
dev=false
arch="$(go env GOARCH)"
ldflags="${GOPMGR_RELEASE_LDFLAGS:-}"
for arg in "$@"; do
    case "$arg" in
        DEV=true|DEV=false) dev=${arg#DEV=} ;;
        ARCH=amd64|ARCH=arm64) arch=${arg#ARCH=} ;;
        EXTRA_LDFLAGS=*) ldflags=${arg#EXTRA_LDFLAGS=} ;;
        *) fail "unsupported argument '$arg'; use DEV=true, ARCH=amd64|arm64 or EXTRA_LDFLAGS=... (v2 flags are not supported)" ;;
    esac
done
os="$(go env GOOS)"
case "$os" in darwin|linux|windows) ;; *) fail "unsupported native target $os";; esac
# Never compile with an experimental or legacy backend supplied via GOFLAGS.
[ "${WAILS_PUREGO:-0}" = 0 ] || fail 'WAILS_PUREGO is not supported; CGO is required'
case " ${GOFLAGS:-} " in
    *gtk3*|*webkit2_41*|*purego*|*'-tags'*) fail 'backend/tag overrides are not supported; standard CGO GTK4 is required' ;;
esac
export CGO_ENABLED=1
if [ "$os" = darwin ]; then
    export MACOSX_DEPLOYMENT_TARGET=13.0
    export CGO_CFLAGS="${CGO_CFLAGS:+$CGO_CFLAGS }-mmacosx-version-min=13.0"
    export CGO_CXXFLAGS="${CGO_CXXFLAGS:+$CGO_CXXFLAGS }-mmacosx-version-min=13.0"
    export CGO_LDFLAGS="${CGO_LDFLAGS:+$CGO_LDFLAGS }-mmacosx-version-min=13.0"
fi
mkdir -p build/bin
stage="$(mktemp -d "$root/build/bin/.wails-v3.XXXXXX")"
syso=''
cleanup() { rm -rf "$stage"; if [ -n "$syso" ]; then rm -f "$syso"; fi; }
trap cleanup EXIT
export GOPMGR_BUILD_STAGE="$stage" GOPMGR_BUILD_DEV="$dev" GOPMGR_BUILD_ARCH="$arch" GOPMGR_BUILD_LDFLAGS="$ldflags"
export PATH="$root/.tmp/wails-v3/bin:$PATH"
# Each command must succeed. In particular a failed build is never recovered
# by discovering an older app, and failed codesign verification is fatal.
"$cli" task -x common:frontend
if [ "$os" != linux ]; then
    "$cli" generate icons -input build/appicon.png -macfilename "$stage/iconfile.icns" -windowsfilename "$stage/icon.ico"
fi
# JSON is a YAML subset: config.yml remains valid Wails configuration while
# stdlib tooling renders the retained GoPMgr templates without a YAML package.
python3 - "$stage" <<'PY'
import json, os, pathlib, sys
from xml.sax.saxutils import escape
stage = pathlib.Path(sys.argv[1])
config = json.loads('\n'.join(line for line in pathlib.Path('build/config.yml').read_text().splitlines() if not line.startswith('#')))
i = config['info']
values = {'BinaryName': 'gopmgr', 'ProductName': i['productName'], 'ProductVersion': i['version'], 'ProductComments': i['comments'], 'ProductCompany': i['companyName'], 'ProductCopyright': i['copyright'], 'ProductIdentifier': i['productIdentifier'], 'Name': 'gopmgr', 'Info.ProductVersion': i['version']}
plist = 'build/darwin/Info.dev.plist' if os.environ['GOPMGR_BUILD_DEV'] == 'true' else 'build/darwin/Info.plist'
for src, dest, xml in [(plist, 'Info.plist', True), ('build/windows/info.json', 'info.json', False), ('build/windows/wails.exe.manifest', 'wails.exe.manifest', True)]:
    data = pathlib.Path(src).read_text()
    for key, value in values.items():
        data = data.replace('{{.' + key + '}}', escape(value) if xml else json.dumps(value)[1:-1])
    if '{{' in data:
        raise SystemExit('unresolved metadata template in ' + src)
    (stage / dest).write_text(data)
PY
if [ "$os" = windows ]; then
    # Go requires the resource in the root package. Reserve only a new owned
    # filename; never remove another build's or the user's existing .syso.
    syso="$root/gopmgr_build_windows_${arch}.syso"
    [ ! -e "$syso" ] || { syso=''; fail 'Windows resource already exists; another build may be active'; }
    "$cli" generate syso -arch "$arch" -icon "$stage/icon.ico" -manifest "$stage/wails.exe.manifest" -info "$stage/info.json" -out "$syso"
fi
"$cli" task -x "$os:compile"
binary=gopmgr
[ "$os" != windows ] || binary=gopmgr.exe
[ -x "$stage/$binary" ] || fail "compile succeeded without expected $binary"
if [ "$os" = darwin ]; then
    app="$stage/gopmgr.app"
    mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
    cp "$stage/gopmgr" "$app/Contents/MacOS/gopmgr"
    cp "$stage/Info.plist" "$app/Contents/Info.plist"
    cp "$stage/iconfile.icns" "$app/Contents/Resources/iconfile.icns"
    xattr -cr "$app"
    codesign --force --deep --sign - "$app"
    codesign --verify --deep --strict "$app"
    # Both outputs are retained for the existing DMG/PKG and tarball consumers.
    rm -rf build/bin/gopmgr.app
    mv "$app" build/bin/gopmgr.app
fi
mv "$stage/$binary" "build/bin/$binary"
echo "wails-build: built $os/$arch in build/bin (DEV=$dev)"
