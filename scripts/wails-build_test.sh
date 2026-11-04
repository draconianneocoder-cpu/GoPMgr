#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export REAL_WAILS="$ROOT/.tmp/wails-v3/bin/wails3"
[ -x "$REAL_WAILS" ] || { echo 'Pinned task-local wails3 required for fixtures' >&2; exit 1; }
fixture="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-build-test.XXXXXX")"
trap 'rm -rf "$fixture"' EXIT
export FIXTURE="$fixture"
mkdir -p "$fixture/scripts" "$fixture/build" "$fixture/frontend" "$fixture/bin" "$fixture/.tmp/wails-v3/bin"
cp "${GOPMGR_WAILS_BUILD_SCRIPT:-$ROOT/scripts/wails-build.sh}" "$fixture/scripts/wails-build.sh"
cp "$ROOT/scripts/wails-v3-qualification.sh" "$fixture/scripts/"
mkdir -p "$fixture/code-map"
printf '{"methods":[{"method":"AccountSetup"}]}' > "$fixture/code-map/app-api.json"
for path in Taskfile.yml build/Taskfile.yml build/config.yml build/darwin/Taskfile.yml build/linux/Taskfile.yml build/windows/Taskfile.yml build/darwin/Info.plist build/darwin/Info.dev.plist build/windows/info.json build/windows/wails.exe.manifest build/appicon.png; do
    if [ -f "$ROOT/$path" ]; then mkdir -p "$fixture/$(dirname "$path")"; cp "$ROOT/$path" "$fixture/$path"; fi
done
printf 'module gopmgr\nrequire github.com/wailsapp/wails/v3 v3.1.0\n' > "$fixture/go.mod"
printf '{"dependencies":{"@wailsio/runtime":"3.1.0"}}\n' > "$fixture/frontend/package.json"
cat > "$fixture/bin/go" <<'STUB'
#!/bin/bash
set -eu
case "$1" in
 env) case "$2" in GOOS) echo "${GOOS:-${HOST_OS:-darwin}}";; GOHOSTOS) echo "${HOST_OS:-darwin}";; GOARCH) echo "${GOARCH:-arm64}";; esac ;;
 version)
   [ -f "$3" ] || exit 93
   if [ "${HOST_OS:-darwin}" = windows ]; then [[ "$3" = *.exe ]] || exit 94; fi
   printf '\tmod\tgithub.com/wailsapp/wails/v3\t%s\n' "${CLI_VERSION:-v3.1.0}" ;;
 build)
   printf 'go %s\nC=%s\nCXX=%s\nLD=%s\nCGO=%s\n' "$*" "${CGO_CFLAGS:-}" "${CGO_CXXFLAGS:-}" "${CGO_LDFLAGS:-}" "${CGO_ENABLED:-}" >> "$FIXTURE/trace"
   [ "${COMPILE_FAIL:-0}" = 0 ] || exit 42
   while [ "$#" -gt 0 ]; do if [ "$1" = -o ]; then shift; output=$1; fi; shift; done
   if [ "${MISSING_OUTPUT:-0}" = 0 ]; then printf '#!/bin/sh\nexit 0\n' > "$output"; chmod +x "$output"; fi ;;
 *) exit 90 ;;
esac
STUB
cat > "$fixture/.tmp/wails-v3/bin/wails3" <<'STUB'
#!/bin/bash
set -eu
printf 'wails3 %s\n' "$*" >> "$FIXTURE/trace"
case "$1 ${2:-}" in
 'version '*) echo v3.1.0 >&2 ;;
 'generate bindings')
   [ "${BINDINGS_FAIL:-0}" = 0 ] || exit 43
   while [ "$#" -gt 0 ]; do if [ "$1" = -d ]; then shift; output=$1; fi; shift; done
   mkdir -p "$output/gopmgr/internal/desktop"
   echo 'export function AccountSetup(): void {}' > "$output/gopmgr/internal/desktop/app.ts" ;;
 'generate icons')
   while [ "$#" -gt 0 ]; do case "$1" in -macfilename|-windowsfilename) shift; mkdir -p "$(dirname "$1")"; echo icon > "$1";; esac; shift; done ;;
 'generate syso') for ((j=1; j<=$#; j++)); do if [ "${!j}" = -manifest ]; then k=$((j+1)); cp "${!k}" "$FIXTURE/windows-manifest.xml"; fi; done; cp "$GOPMGR_BUILD_STAGE/info.json" "$FIXTURE/windows-info.json"; [ "${RESOURCE_FAIL:-0}" = 0 ] || exit 45; while [ "$#" -gt 0 ]; do if [ "$1" = -out ]; then shift; touch "$1"; fi; shift; done ;;
 'task '*) exec "$REAL_WAILS" "$@" ;;
 *) exit 91 ;;
esac
STUB
cat > "$fixture/bin/npm" <<'STUB'
#!/bin/bash
set -eu
printf 'npm %s\n' "$*" >> "$FIXTURE/trace"
[ "${FRONTEND_FAIL:-0}" = 0 ] || exit 44
if [ "$1 ${2:-}" = 'run build' ]; then mkdir -p dist; echo fresh > dist/index.html; fi
STUB
cat > "$fixture/bin/pkg-config" <<'STUB'
#!/bin/bash
printf 'pkg-config %s\n' "$*" >> "$FIXTURE/trace"
exit "${GTK_FAIL:-0}"
STUB
cat > "$fixture/bin/codesign" <<'STUB'
#!/bin/bash
printf 'codesign %s\n' "$*" >> "$FIXTURE/trace"
case "$1" in --verify) exit "${VERIFY_FAIL:-0}";; *) exit "${SIGN_FAIL:-0}";; esac
STUB
printf '#!/bin/sh\nexit 0\n' > "$fixture/bin/xattr"
printf '#!/bin/sh\nexit 92\n' > "$fixture/bin/wails"
chmod +x "$fixture/bin/"* "$fixture/.tmp/wails-v3/bin/wails3"
cp "$fixture/.tmp/wails-v3/bin/wails3" "$fixture/.tmp/wails-v3/bin/wails3.exe"
export PATH="$fixture/bin:$PATH"
cd "$fixture"
failures=0
run_case() {
    local name=$1 expected=$2; shift 2
    mkdir -p build/bin/gopmgr.app/Contents/MacOS build/bin/unrelated.app
    echo stale > build/bin/gopmgr.app/Contents/MacOS/gopmgr
    echo keep > build/bin/unrelated.app/sentinel
    echo keep > build/bin/unrelated.txt
    : > trace
    local status=0
    env "$@" bash scripts/wails-build.sh > output 2>&1 || status=$?
    if { [ "$expected" = fail ] && [ "$status" = 0 ]; } || { [ "$expected" = pass ] && [ "$status" != 0 ]; } || [ ! -f build/bin/unrelated.txt ] || [ ! -f build/bin/unrelated.app/sentinel ]; then
        echo "FAIL $name (status=$status, expected=$expected)"; cat output; failures=$((failures+1)); return
    fi
    if [ "$expected" = fail ]; then
        [ "$(cat build/bin/gopmgr.app/Contents/MacOS/gopmgr)" = stale ] || { echo "FAIL $name replaced previous bundle"; failures=$((failures+1)); }
    fi
    echo "PASS $name"
}
run_case 'failed compile cannot be rescued by stale app' fail COMPILE_FAIL=1
run_case 'failed bindings block build' fail BINDINGS_FAIL=1
run_case 'failed frontend blocks build' fail FRONTEND_FAIL=1
run_case 'missing compiler output fails' fail MISSING_OUTPUT=1
run_case 'signature failure propagates' fail SIGN_FAIL=1
run_case 'signature verification failure propagates' fail VERIFY_FAIL=1
run_case 'wrong CLI rejected' fail CLI_VERSION=v3.0.0
run_case 'Windows resource generation failure' fail HOST_OS=windows RESOURCE_FAIL=1
run_case 'Windows compile failure' fail HOST_OS=windows COMPILE_FAIL=1
run_case 'Windows build with exe-only metadata lookup' pass HOST_OS=windows GOOS=windows GOARCH=amd64
[ -x build/bin/gopmgr.exe ] || { echo 'FAIL Windows output'; failures=$((failures+1)); }
if [ -e gopmgr_build_windows_amd64.syso ] || [ -e gopmgr_build_windows_arm64.syso ]; then echo 'FAIL Windows resource leaked'; failures=$((failures+1)); fi
if ! grep -F -- '-H windowsgui' trace >/dev/null; then echo 'FAIL Windows GUI linker flags'; failures=$((failures+1)); fi
if ! python3 - <<'CHECK'
import json
p=json.load(open('windows-info.json'))
assert p['fixed']['file_version'] == '1.1.0'
assert p['info']['0000']['ProductName'] == 'GoPMgr'
assert p['info']['0000']['ProductVersion'] == '1.1.0'
import xml.etree.ElementTree as ET
identity=ET.parse('windows-manifest.xml').getroot().find('{urn:schemas-microsoft-com:asm.v1}assemblyIdentity')
assert identity.attrib['name'] == 'dev.gopmgr.gopmgr'
assert identity.attrib['version'] == '1.1.0.0'
CHECK
then failures=$((failures+1)); fi
run_case 'successful macOS build' pass
if ! python3 - <<'PY'
import os, plistlib
assert 'gopmgr.app' in os.listdir('build/bin'), 'bundle name must have exact lowercase spelling'
with open('build/bin/gopmgr.app/Contents/Info.plist','rb') as f: p=plistlib.load(f)
assert (p['CFBundleIdentifier'],p['CFBundleShortVersionString'],p['LSMinimumSystemVersion'],p['CFBundleExecutable']) == ('dev.gopmgr.GoPMgr','1.1.0','13.0.0','gopmgr')
PY
then failures=$((failures+1)); fi
for required in 'npm ci' 'npm run build' '-ts -i -names -time-type string' '-tags production' 'C=-mmacosx-version-min=13.0' 'CXX=-mmacosx-version-min=13.0' 'LD=-mmacosx-version-min=13.0' 'CGO=1'; do
    if ! grep -F -- "$required" trace >/dev/null; then echo "FAIL missing $required"; failures=$((failures+1)); fi
done
run_case 'GTK4 unavailable has no fallback' fail HOST_OS=linux GTK_FAIL=1
run_case 'Linux compile failure' fail HOST_OS=linux COMPILE_FAIL=1
run_case 'Linux build' pass HOST_OS=linux
if ! grep -F 'pkg-config --exists gtk4 webkitgtk-6.0' trace >/dev/null || [ ! -x build/bin/gopmgr ]; then echo 'FAIL Linux GTK4/output contract'; failures=$((failures+1)); fi
# Explicit task variables, not old v2 flags, select development and updater metadata.
: > trace
if ! bash scripts/wails-build.sh DEV=true 'EXTRA_LDFLAGS=-X gopmgr/internal/update.CurrentVersion=fixture' > output 2>&1; then cat output; failures=$((failures+1)); fi
if grep -F -- '-tags production' trace >/dev/null || ! grep -F -- 'gopmgr/internal/update.CurrentVersion=fixture' trace >/dev/null; then echo 'FAIL development tags/updater flags'; failures=$((failures+1)); fi
if ! python3 - <<'CHECK'
import os, plistlib
assert 'gopmgr.app' in os.listdir('build/bin'), 'bundle name must have exact lowercase spelling'
with open('build/bin/gopmgr.app/Contents/Info.plist','rb') as f: p=plistlib.load(f)
assert p['NSAppTransportSecurity']['NSAllowsLocalNetworking'] is True
CHECK
then failures=$((failures+1)); fi
if bash scripts/wails-build.sh -platform darwin/arm64 > output 2>&1; then echo 'FAIL accepted v2 flags'; failures=$((failures+1)); fi
[ "$failures" = 0 ] || { echo "$failures failing build fixture checks"; exit 1; }
echo 'wails-build fixtures passed'
