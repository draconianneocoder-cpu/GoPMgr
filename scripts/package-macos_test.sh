#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

backup_root="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-package-macos-test.XXXXXX")"
stub_bin="$backup_root/bin"
mkdir -p "$stub_bin"

restore_path() {
	local path="$1"
	local backup="$2"
	rm -rf "$path"
	if [ -e "$backup" ]; then
		mv "$backup" "$path"
	fi
}

cleanup() {
	restore_path "$ROOT/build/bin" "$backup_root/build-bin.backup"
	restore_path "$ROOT/build/packages" "$backup_root/build-packages.backup"
	rm -rf "$backup_root"
}
trap cleanup EXIT

if [ -e build/bin ]; then
	mv build/bin "$backup_root/build-bin.backup"
fi
if [ -e build/packages ]; then
	mv build/packages "$backup_root/build-packages.backup"
fi

mkdir -p build/bin/pmforge.app/Contents/MacOS build/packages
printf 'fake app binary\n' > build/bin/pmforge.app/Contents/MacOS/pmforge
chmod +x build/bin/pmforge.app/Contents/MacOS/pmforge
cat > build/bin/pmforge.app/Contents/Info.plist << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
	"http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>PMForge</string>
</dict>
</plist>
PLIST

cat > "$stub_bin/create-dmg" << 'STUB'
#!/bin/bash
set -euo pipefail
previous=""
current=""
for arg in "$@"; do
	previous="$current"
	current="$arg"
done
outfile="$previous"
srcfolder="$current"

if [ -z "$outfile" ] || [ -z "$srcfolder" ]; then
	echo "create-dmg stub: missing output/source arguments" >&2
	exit 1
fi
if [ ! -d "$srcfolder/PMForge.app" ]; then
	echo "create-dmg stub: source folder lacks PMForge.app: $srcfolder" >&2
	exit 1
fi
if [ -d "$srcfolder/Contents" ]; then
	echo "create-dmg stub: source folder points at app bundle contents: $srcfolder" >&2
	exit 1
fi

mkdir -p "$(dirname "$outfile")"
printf 'stub create-dmg\n' > "$outfile"
STUB
chmod +x "$stub_bin/create-dmg"

cat > "$stub_bin/hdiutil" << 'STUB'
#!/bin/bash
set -euo pipefail
srcfolder=""
outfile=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-srcfolder)
			shift
			srcfolder="$1"
			;;
		*.dmg)
			outfile="$1"
			;;
	esac
	shift
done

if [ -z "$srcfolder" ]; then
	echo "hdiutil stub: missing -srcfolder" >&2
	exit 1
fi
if [ -z "$outfile" ]; then
	echo "hdiutil stub: missing output dmg path" >&2
	exit 1
fi
if [ ! -d "$srcfolder/PMForge.app" ]; then
	echo "hdiutil stub: DMG root lacks PMForge.app: $srcfolder" >&2
	exit 1
fi
if [ ! -L "$srcfolder/Applications" ]; then
	echo "hdiutil stub: DMG root lacks Applications symlink: $srcfolder" >&2
	exit 1
fi
if [ "$(readlink "$srcfolder/Applications")" != "/Applications" ]; then
	echo "hdiutil stub: Applications symlink has wrong target" >&2
	exit 1
fi

mkdir -p "$(dirname "$outfile")"
printf 'stub dmg\n' > "$outfile"
STUB
chmod +x "$stub_bin/hdiutil"

output="$(PATH="$stub_bin:$PATH" PMFORGE_PACKAGE_LAYOUT_TEST=1 VERSION=test-create-dmg bash scripts/package-macos.sh 2>&1)" || {
	printf '%s\n' "$output" >&2
	fail "package-macos create-dmg layout failed"
}

case "$output" in
	*"build/packages/PMForge-test-create-dmg-arm64.dmg"*) ;;
	*)
		printf '%s\n' "$output" >&2
		fail "package-macos did not report the expected create-dmg artifact path"
		;;
esac

if [ ! -f build/packages/PMForge-test-create-dmg-arm64.dmg ]; then
	fail "package-macos did not create the expected create-dmg artifact"
fi

cat > "$stub_bin/create-dmg" << 'STUB'
#!/bin/bash
exit 127
STUB
chmod +x "$stub_bin/create-dmg"

output="$(PATH="$stub_bin:$PATH" PMFORGE_PACKAGE_LAYOUT_TEST=1 VERSION=test-fallback bash scripts/package-macos.sh 2>&1)" || {
	printf '%s\n' "$output" >&2
	fail "package-macos fallback layout failed"
}

case "$output" in
	*"build/packages/PMForge-test-fallback-arm64.dmg"*) ;;
	*)
		printf '%s\n' "$output" >&2
		fail "package-macos did not report the expected DMG path"
		;;
esac

if [ ! -f build/packages/PMForge-test-fallback-arm64.dmg ]; then
	fail "package-macos did not create the expected DMG artifact"
fi

echo "package-macos layout tests passed."
