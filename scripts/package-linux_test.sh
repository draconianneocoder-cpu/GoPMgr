#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-package-linux-test.XXXXXX")"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/scripts" "$fixture/build/bin" "$fixture/build/linux" "$fixture/internal/fonts/assets" "$fixture/third_party/go-sqlcipher" "$fixture/LICENSES" "$fixture/bin"
cp "$ROOT/scripts/"{package-linux.sh,package-version-lib.sh,release-tool-versions.env,verify-module-notices.sh} "$fixture/scripts/"
cp "$ROOT/build/linux/"{nfpm.yaml,gopmgr.desktop} "$fixture/build/linux/"
cp "$ROOT/build/appicon.png" "$fixture/build/"
cp "$ROOT/third_party/"{GO-MODULE-NOTICES.txt,FRONTEND-NOTICES.txt} "$fixture/third_party/"
cp "$ROOT/third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt" "$fixture/third_party/go-sqlcipher/"
cp "$ROOT/internal/fonts/assets/FONT-NOTICES.txt" "$fixture/internal/fonts/assets/"
cp "$ROOT/LICENSES/GPL-3.0-or-later.txt" "$fixture/LICENSES/"
cd "$fixture"
printf 'package main\nfunc main() {}\n' > main.go
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/bin/gopmgr main.go
cp build/bin/gopmgr good-binary
# The cross-built fixture has real Go/ELF metadata but cannot link Linux CGO
# on macOS. Control only external readelf output for ABI rejection tests.
cat > bin/readelf <<'STUB'
#!/bin/bash
case "${ELF_BACKEND:-gtk4}" in
 gtk4) printf ' 0x00000001 (NEEDED) Shared library: [libgtk-4.so.1]\n 0x00000001 (NEEDED) Shared library: [libwebkitgtk-6.0.so.4]\n' ;;
 gtk3) printf ' 0x00000001 (NEEDED) Shared library: [libgtk-3.so.0]\n 0x00000001 (NEEDED) Shared library: [libwebkit2gtk-4.1.so.0]\n' ;;
 mixed) printf ' (NEEDED) Shared library: [libgtk-4.so.1]\n (NEEDED) Shared library: [libwebkitgtk-6.0.so.4]\n (NEEDED) Shared library: [libgtk-3.so.0]\n' ;;
 missing) exit 0 ;;
 failed) exit 1 ;;
esac
STUB
chmod +x bin/readelf
export PATH="$fixture/bin:$PATH"
fail() { echo "FAIL $*" >&2; exit 1; }
reject() {
 local name=$1 diagnostic=$2; shift 2
 if env VERSION=1.2.0-3-gabc1234 "$@" bash scripts/package-linux.sh > output 2>&1; then fail "accepted $name"; fi
 grep -F "$diagnostic" output >/dev/null || { cat output; fail "$name diagnostic"; }
 [ ! -d build/packages ] || fail "$name published package"
 echo "PASS rejects $name"
}
for backend in gtk3 mixed missing failed; do reject "$backend backend" 'GTK 4/WebKitGTK 6.0' ELF_BACKEND="$backend"; done
mv build/bin/gopmgr saved-binary
reject 'missing binary' 'build/bin/gopmgr missing'
mv saved-binary build/bin/gopmgr
printf '#!/bin/sh\n' > build/bin/gopmgr; chmod +x build/bin/gopmgr
reject 'non ELF' 'not a linux/amd64 ELF'
cp good-binary build/bin/gopmgr
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o build/bin/gopmgr main.go
reject 'wrong architecture' 'not a linux/amd64 ELF'
cp good-binary build/bin/gopmgr
reject 'missing nfpm' 'nfpm not found' PATH=/usr/bin:/bin
printf '#!/bin/sh\nexit 0\n' > bin/nfpm; chmod +x bin/nfpm
reject 'unverifiable nfpm' 'is required; found'
rm bin/nfpm
if ! command -v nfpm >/dev/null; then echo 'SKIP package contents: pinned nfpm unavailable'; exit 0; fi
cp third_party/GO-MODULE-NOTICES.txt notices.saved
grep -vx 'Module: std' notices.saved > third_party/GO-MODULE-NOTICES.txt
reject 'missing module notices' 'no Go standard library section'
mv notices.saved third_party/GO-MODULE-NOTICES.txt
VERSION=1.2.0-3-gabc1234 bash scripts/package-linux.sh > output 2>&1 || { cat output; fail 'valid packaging'; }
[ -f build/packages/gopmgr-1.2.0-3-gabc1234-amd64.deb ] || fail 'deb output'
[ -f build/packages/gopmgr-1.2.0-3-gabc1234-x86_64.rpm ] || fail 'rpm output'
mkdir extracted
(cd extracted && ar x ../build/packages/gopmgr-1.2.0-3-gabc1234-amd64.deb)
tar -xOf extracted/control.tar.* ./control > control
for expected in 'Package: gopmgr' 'Version: 1.2.0~3-gabc1234' libgtk-4-1 libwebkitgtk-6.0-4; do grep -F "$expected" control >/dev/null || fail "package control missing $expected"; done
if grep -E 'libgtk-3|libwebkit2gtk' control; then fail 'legacy runtime dependency'; fi
for pair in 'third_party/GO-MODULE-NOTICES.txt go-module-notices.txt' 'third_party/FRONTEND-NOTICES.txt frontend-notices.txt' 'third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt sqlcipher-license.txt' 'internal/fonts/assets/FONT-NOTICES.txt font-notices.txt' 'LICENSES/GPL-3.0-or-later.txt LICENSE'; do
 read -r src dest <<< "$pair"
 tar -xOf extracted/data.tar.* "./usr/share/doc/gopmgr/$dest" > extracted-notice
 cmp "$src" extracted-notice || fail "notice $dest"
done
# Exercise the shared version derivation through the package wrapper.
git init -q; git config user.email test@example.invalid; git config user.name test
git add main.go; git -c commit.gpgsign=false commit -q -m fixture; git tag v1.2.3
echo '// next' >> main.go; git add main.go; git -c commit.gpgsign=false commit -q -m next
expected="$(git describe --tags | sed 's/^v//')"
env -u VERSION bash scripts/package-linux.sh > output 2>&1 || { cat output; fail 'git-derived version'; }
[ -f "build/packages/gopmgr-$expected-amd64.deb" ] || fail 'version derivation'
echo 'package-linux fixtures passed (synthetic dynamic dependencies; no native launch)'
