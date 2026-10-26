#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Layout test for scripts/package.sh, the tarball path. It runs the real
# script in an isolated copy of the files it reads, with a stub `make` in
# place of the Wails build, and checks that the tarball carries the binary,
# the README, the license texts and every notices file byte for byte.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

work="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-package-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT

repo="$work/repo"
mkdir -p "$repo/scripts" "$repo/third_party/go-sqlcipher" "$repo/internal/fonts/assets" "$work/bin" "$work/src"
cp scripts/package.sh scripts/verify-module-notices.sh scripts/verify-excluded-native-code.sh "$repo/scripts/"
cp README.md "$repo/"
cp -R LICENSES "$repo/"
cp third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt "$repo/third_party/go-sqlcipher/"
cp third_party/GO-MODULE-NOTICES.txt third_party/DUCKDB-NATIVE-NOTICES.txt third_party/FRONTEND-NOTICES.txt third_party/EXCLUDED-NATIVE-CODE.txt "$repo/third_party/"
cp internal/fonts/assets/FONT-NOTICES.txt "$repo/internal/fonts/assets/"

# A real Go binary, so verify-module-notices.sh checks it for real.
printf 'package main\n\nfunc main() {}\n' > "$work/src/main.go"
(cd "$work/src" && CGO_ENABLED=0 GOFLAGS= go build -o "$work/gopmgr" main.go) ||
	fail "could not build the fixture binary (needs a Go toolchain)"

cat > "$work/bin/make" << 'STUB'
#!/bin/bash
[ "$*" = build ] || { echo "make stub: unexpected arguments '$*'" >&2; exit 2; }
mkdir -p build/bin
cp "$PACKAGE_TEST_BINARY" build/bin/gopmgr
STUB
chmod +x "$work/bin/make"

target="$(go env GOOS)"
arch="$(go env GOARCH)"
base="gopmgr-$target-$arch"
archive="$repo/build/packages/$base.tar.gz"

run_package() {
	(cd "$repo" && PATH="$work/bin:$PATH" PACKAGE_TEST_BINARY="$work/gopmgr" bash scripts/package.sh "$target" 2>&1)
}

output="$(run_package)" || {
	printf '%s\n' "$output" >&2
	fail "package.sh failed for $target"
}
[ -f "$archive" ] || fail "package.sh did not produce $archive"

# check_member <path in the tarball> <expected file>
check_member() {
	if ! tar -xOf "$archive" "$base/$1" | cmp -s - "$2"; then
		fail "the tarball's $1 is not an exact copy of $2"
	fi
}
check_member gopmgr "$work/gopmgr"
check_member README.md README.md
check_member LICENSES/GPL-3.0-or-later.txt LICENSES/GPL-3.0-or-later.txt
check_member LICENSES/OFL-1.1.txt LICENSES/OFL-1.1.txt
check_member LICENSES/go-sqlcipher.txt third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt
check_member LICENSES/fonts.txt internal/fonts/assets/FONT-NOTICES.txt
check_member LICENSES/go-modules.txt third_party/GO-MODULE-NOTICES.txt
check_member LICENSES/duckdb-native.txt third_party/DUCKDB-NATIVE-NOTICES.txt
check_member LICENSES/frontend.txt third_party/FRONTEND-NOTICES.txt

# Notices that do not cover the binary stop packaging before any archive is
# written. The fixture links no modules, so a file without the standard
# library section stands in for a missing module.
rm -rf "$repo/build/packages"
grep -vx 'Module: std' third_party/GO-MODULE-NOTICES.txt > "$repo/third_party/GO-MODULE-NOTICES.txt"
if output="$(run_package)"; then
	printf '%s\n' "$output" >&2
	fail "package.sh succeeded with Go module notices that do not cover the binary"
fi
case "$output" in
*"verify-module-notices: "*"no Go standard library section"*) ;;
*)
	printf '%s\n' "$output" >&2
	fail "package.sh's incomplete-notices failure did not come from verify-module-notices"
	;;
esac
[ ! -e "$archive" ] || fail "package.sh wrote $archive despite incomplete notices"

echo "package layout test passed ($target/$arch)."
