#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
# Real Wails generation + PE metadata; controlled makensis failures. Optional
# actual NSIS compilation checks syntax, never native Windows lifecycle.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-windows-package.XXXXXX")"
trap 'rm -rf "$fixture"' EXIT
repo="$fixture/repo with spaces"
mkdir -p "$repo/scripts" "$repo/build/windows/installer" "$repo/build/bin" "$repo/.tmp/wails-v3/bin" "$fixture/bin" "$repo/third_party/go-sqlcipher" "$repo/internal/fonts/assets" "$repo/LICENSES"
if [ -f "$ROOT/scripts/package-windows.sh" ]; then cp "$ROOT/scripts/package-windows.sh" "$repo/scripts/"; fi
cp "$ROOT/scripts/verify-module-notices.sh" "$repo/scripts/"
cp "$ROOT/build/config.yml" "$repo/build/"
cp "$ROOT/build/appicon.png" "$repo/build/"
cp "$ROOT/build/windows/installer/project.nsi" "$repo/build/windows/installer/"
cp "$ROOT/third_party/GO-MODULE-NOTICES.txt" "$ROOT/third_party/FRONTEND-NOTICES.txt" "$repo/third_party/"
cp "$ROOT/third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt" "$repo/third_party/go-sqlcipher/"
cp "$ROOT/internal/fonts/assets/FONT-NOTICES.txt" "$repo/internal/fonts/assets/"
cp "$ROOT/LICENSES/GPL-3.0-or-later.txt" "$repo/LICENSES/"
export REAL_WAILS="$ROOT/.tmp/wails-v3/bin/wails3"
REAL_GO="$(command -v go)"
export REAL_GO
REAL_NSIS="$(command -v makensis || true)"
export REAL_NSIS
export FIXTURE_REPO="$repo"
cat > "$repo/.tmp/wails-v3/bin/wails3" <<'STUB'
#!/bin/bash
set -eu
case "$1 $2" in
 'update build-assets') [ "${ASSETS_FAIL:-0}" = 0 ] || exit 41 ;;
 'generate webview2bootstrapper') [ "${BOOTSTRAPPER_FAIL:-0}" = 0 ] || exit 42 ;;
esac
exec "$REAL_WAILS" "$@"
STUB
cat > "$fixture/bin/go" <<'STUB'
#!/bin/bash
set -eu
if [ "$1" = version ] && [ "$2" = -m ] && [[ "$3" = */wails3 ]]; then
 if [ "${CLI_WRONG:-0}" = 1 ]; then printf '\tmod\tgithub.com/wailsapp/wails/v3\tv3.0.0\n'; exit 0; fi
 exec "$REAL_GO" version -m "$REAL_WAILS"
fi
exec "$REAL_GO" "$@"
STUB
cat > "$fixture/bin/makensis" <<'STUB'
#!/bin/bash
set -eu
[ "${NSIS_FAIL:-0}" = 0 ] || exit 43
[ "${NSIS_MISSING_OUTPUT:-0}" = 0 ] || exit 0
cp wails_tools.nsh "$FIXTURE_REPO/captured-wails-tools.nsh"
cp MicrosoftEdgeWebview2Setup.exe "$FIXTURE_REPO/captured-bootstrapper.exe"
printf '%s\n' "$@" > "$FIXTURE_REPO/captured-nsis-args"
if [ "${RUN_REAL_NSIS:-0}" = 1 ]; then exec "$REAL_NSIS" "$@"; fi
case "$*" in *ARG_WAILS_AMD64_BINARY=*) arch=amd64;; *ARG_WAILS_ARM64_BINARY=*) arch=arm64;; *) exit 44;; esac
printf fresh > "../../bin/gopmgr-$arch-installer.exe"
STUB
chmod +x "$fixture/bin/"* "$repo/.tmp/wails-v3/bin/wails3"
export PATH="$fixture/bin:$PATH"
printf 'package main\nfunc main() {}\n' > "$fixture/main.go"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "$REAL_GO" build -o "$fixture/amd64.exe" "$fixture/main.go"
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 "$REAL_GO" build -o "$fixture/arm64.exe" "$fixture/main.go"
cd "$repo"
cp "$fixture/amd64.exe" build/bin/gopmgr.exe
fail() { echo "FAIL $*" >&2; exit 1; }
run_failure() {
 local name=$1; shift
 printf stale > build/bin/gopmgr-amd64-installer.exe
 echo keep > build/bin/unrelated
 if env "$@" bash scripts/package-windows.sh > output 2>&1; then cat output; fail "accepted $name"; fi
 [ "$(cat build/bin/gopmgr-amd64-installer.exe)" = stale ] || fail "$name lost previous installer"
 [ "$(cat build/bin/unrelated)" = keep ] || fail "$name lost unrelated artifact"
 echo "PASS rejects $name"
}
# Positive first ensures absent or broken production code cannot make all
# negative cases look green by failing before their intended boundary.
if ! bash scripts/package-windows.sh > output 2>&1; then cat output; fail 'valid package pipeline'; fi
[ "$(cat build/bin/gopmgr-amd64-installer.exe)" = fresh ] || fail 'expected installer output'
for expected in '!define INFO_PROJECTNAME "gopmgr"' '!define INFO_COMPANYNAME "The GoPMgr Contributors"' '!define INFO_PRODUCTNAME "GoPMgr"' '!define INFO_PRODUCTVERSION "1.1.0"' 'File "MicrosoftEdgeWebview2Setup.exe"'; do
 grep -F -- "$expected" captured-wails-tools.nsh >/dev/null || fail "generated macro missing $expected"
done
if grep -F '{{' captured-wails-tools.nsh; then fail 'unrendered macro'; fi
module="$(cd "$ROOT" && "$REAL_GO" list -m -f '{{.Dir}}' github.com/wailsapp/wails/v3)"
cmp captured-bootstrapper.exe "$module/internal/commands/webview2/MicrosoftEdgeWebview2Setup.exe" || fail 'bootstrapper provenance'
run_failure 'wrong CLI' CLI_WRONG=1
run_failure 'failed macro generation' ASSETS_FAIL=1
run_failure 'failed bootstrapper generation' BOOTSTRAPPER_FAIL=1
run_failure 'failed makensis' NSIS_FAIL=1
run_failure 'missing installer output' NSIS_MISSING_OUTPUT=1
run_failure 'invalid scope' WAILS_INSTALL_SCOPE=unexpected
run_failure 'wrong architecture request' GOARCH=arm64
cp build/bin/gopmgr.exe binary.saved; printf 'not PE\n' > build/bin/gopmgr.exe
run_failure 'non-PE binary'
mv binary.saved build/bin/gopmgr.exe
cp third_party/GO-MODULE-NOTICES.txt notices.saved
grep -vx 'Module: std' notices.saved > third_party/GO-MODULE-NOTICES.txt
run_failure 'missing notices'
mv notices.saved third_party/GO-MODULE-NOTICES.txt
cp "$fixture/arm64.exe" build/bin/gopmgr.exe
WAILS_INSTALL_SCOPE=user bash scripts/package-windows.sh > output 2>&1 || { cat output; fail 'ARM64/user package'; }
[ "$(cat build/bin/gopmgr-arm64-installer.exe)" = fresh ] || fail 'ARM64 output'
grep -F -- '-DWAILS_INSTALL_SCOPE=user' captured-nsis-args >/dev/null || fail 'per-user scope'
# Real preprocessor/compiler consumes the exact generated macros, owned nsi,
# icon, bootstrapper and notices. No fixture executable is ever launched.
if [ -n "$REAL_NSIS" ]; then
 for arch in amd64 arm64; do
  cp "$fixture/$arch.exe" build/bin/gopmgr.exe
  RUN_REAL_NSIS=1 bash scripts/package-windows.sh > output 2>&1 || { cat output; fail "real NSIS compile $arch"; }
  [ -s "build/bin/gopmgr-$arch-installer.exe" ] || fail "real NSIS output $arch"
 done
 echo "PASS actual NSIS syntax compilation ($($REAL_NSIS -VERSION))"
else echo 'SKIP actual NSIS compilation: makensis unavailable'; fi
echo 'package-windows fixtures passed; native Windows install/upgrade/remove remains unqualified'
