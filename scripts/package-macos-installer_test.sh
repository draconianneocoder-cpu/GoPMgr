#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ "$(uname -s)" != Darwin ]; then
	echo 'package-macos-installer tests require macOS plutil and xmllint.' >&2
	exit 2
fi
work="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-installer-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT
bin="$work/bin"
mkdir -p "$bin" "$work/source.app/Contents/MacOS"
cat > "$work/source.app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>dev.gopmgr.GoPMgr</string>
<key>CFBundleVersion</key><string>7.8.9</string>
<key>CFBundleShortVersionString</key><string>7.8.9</string>
</dict></plist>
PLIST
printf 'fixture app\n' > "$work/source.app/Contents/MacOS/gopmgr"

# `go version -m` reports the fixture app's linked modules for
# verify-module-notices.sh; the missing-module-notice case adds one that the
# fixture notices file lacks.
cat > "$bin/go" <<'STUB'
#!/bin/bash
case "$*" in
	'env GOOS') echo darwin ;;
	'env GOARCH') echo arm64 ;;
	'version -m '*)
		printf '%s: go1.27.3\n\tpath\tgopmgr\n\tdep\texample.com/fixture\tv1.0.0\th1:x=\n' "$3"
		if [ "$PKG_TEST_CASE" = missing-module-notice ]; then
			printf '\tdep\texample.com/unlisted\tv2.0.0\th1:y=\n'
		fi
		;;
	*) exit 2 ;;
esac
STUB
cat > "$bin/wails" <<'STUB'
#!/bin/bash
exit 0
STUB
cat > "$bin/make" <<'STUB'
#!/bin/bash
set -euo pipefail
echo build >> "$PKG_TEST_LOG"
mkdir -p build/bin
cp -R "$PKG_TEST_SOURCE" build/bin/gopmgr.app
STUB
for tool in xattr codesign; do
	cp "$bin/wails" "$bin/$tool"
done

cat > "$bin/pkgbuild" <<'STUB'
#!/bin/bash
set -euo pipefail
echo package >> "$PKG_TEST_LOG"
if [ "$PKG_TEST_CASE" = package-failure ]; then exit 43; fi
root='' policy='' identity='' version='' location='' output=''
while [ "$#" -gt 0 ]; do
	case "$1" in
		--root) root="$2"; shift 2 ;;
		--component-plist) policy="$2"; shift 2 ;;
		--identifier) identity="$2"; shift 2 ;;
		--version) version="$2"; shift 2 ;;
		--install-location) location="$2"; shift 2 ;;
		*) output="$1"; shift ;;
	esac
done
[[ -d "$root/gopmgr.app" && "$(ls -A "$root")" == gopmgr.app ]]
[[ "$identity" == dev.gopmgr.GoPMgr && "$version" == 7.8.9 && "$location" == /Applications ]]
[[ "$output" == build/packages/.gopmgr-pkg.*/installer.pkg ]]
[[ -s "$root/gopmgr.app/Contents/Resources/Licenses/go-sqlcipher.txt" ]]
grep -Fq "SIL OPEN FONT LICENSE" "$root/gopmgr.app/Contents/Resources/Licenses/fonts.txt"
cmp -s "$root/gopmgr.app/Contents/Resources/Licenses/go-modules.txt" third_party/GO-MODULE-NOTICES.txt
cmp -s "$root/gopmgr.app/Contents/Resources/Licenses/duckdb-native.txt" third_party/DUCKDB-NATIVE-NOTICES.txt
[[ "$(/usr/bin/xmllint --nonet --xpath 'count(/plist/array/dict)' "$policy")" == 1 ]]
[[ "$(plutil -extract 0.RootRelativeBundlePath raw -expect string -o - "$policy")" == gopmgr.app ]]
[[ "$(plutil -extract 0.BundleIsVersionChecked raw -expect bool -o - "$policy")" == true ]]
[[ "$(plutil -extract 0.BundleHasStrictIdentifier raw -expect bool -o - "$policy")" == true ]]
[[ "$(plutil -extract 0.BundleOverwriteAction raw -expect string -o - "$policy")" == upgrade ]]
if ! relocation="$(plutil -extract 0.BundleIsRelocatable raw -expect bool -o - "$policy" 2>/dev/null)"; then
	relocation="$PKG_TEST_DEFAULT"
	echo default-relocation >> "$PKG_TEST_LOG"
fi
# A native package is tested separately; this fixture carries only tool inputs.
printf '%s\n' "$identity" "$version" "$location" "$relocation" > "$output"
STUB

cat > "$bin/pkgutil" <<'STUB'
#!/bin/bash
set -euo pipefail
echo expand >> "$PKG_TEST_LOG"
if [ "$PKG_TEST_CASE" = expansion-failure ]; then exit 43; fi
[[ "$1" == --expand ]]
identity="$(sed -n '1p' "$2")"
version="$(sed -n '2p' "$2")"
location="$(sed -n '3p' "$2")"
relocation="$(sed -n '4p' "$2")"
reloc_attr="relocatable=\"$relocation\""
relocate='<relocate/>'
bundle='<bundle path="./gopmgr.app" id="dev.gopmgr.GoPMgr"/>'
case "$PKG_TEST_CASE" in
	wrong-destination) location=/tmp ;;
	wrong-identity) identity=dev.invalid.Other ;;
	wrong-version) version=0 ;;
	relocatable-output) reloc_attr='relocatable="true"' ;;
	missing-relocation-attribute) reloc_attr='' ;;
	missing-relocate) relocate='' ;;
	relocation-child) relocate='<relocate><bundle id="dev.gopmgr.GoPMgr"/></relocate>' ;;
	duplicate-bundle) bundle="$bundle$bundle" ;;
esac
mkdir "$3"
cat > "$3/PackageInfo" <<XML
<pkg-info identifier="$identity" version="$version" install-location="$location" $reloc_attr>
$bundle
<bundle-version><bundle id="dev.gopmgr.GoPMgr"/></bundle-version>
<upgrade-bundle><bundle id="dev.gopmgr.GoPMgr"/></upgrade-bundle>
<strict-identifier><bundle id="dev.gopmgr.GoPMgr"/></strict-identifier>
<update-bundle/>$relocate
</pkg-info>
XML
case "$PKG_TEST_CASE" in
	malformed-xml) printf '<broken' > "$3/PackageInfo" ;;
	scripts) mkdir "$3/Scripts" ;;
esac
STUB
cat > "$bin/xmllint" <<'STUB'
#!/bin/bash
echo parse >> "$PKG_TEST_LOG"
if [ "$PKG_TEST_CASE" = parser-failure ]; then echo true; exit 43; fi
if [ "$PKG_TEST_CASE" = version-parser-failure ] && [[ "$*" == *'string(/pkg-info/@version)'* ]]; then
	echo 7.8.9
	exit 43
fi
exec /usr/bin/xmllint "$@"
STUB
cat > "$bin/mv" <<'STUB'
#!/bin/bash
echo publish >> "$PKG_TEST_LOG"
if [ "$PKG_TEST_CASE" = publication-failure ]; then exit 43; fi
exec /bin/mv "$@"
STUB
chmod +x "$bin/"*

fail() { echo "FAIL: $*" >&2; exit 1; }
cases=0
run_case() {
	local name="$1" expected_stage="$2" expected_error="$3"
	local repo="$work/repo with spaces 'quote'-$name"
	local output status final events
	mkdir -p "$repo/scripts" "$repo/build/darwin" "$repo/build/packages" "$repo/tmp" "$repo/third_party/go-sqlcipher" "$repo/internal/fonts/assets"
	cp "$ROOT/scripts/package-macos-installer.sh" "$ROOT/scripts/package-version-lib.sh" \
		"$ROOT/scripts/verify-module-notices.sh" "$repo/scripts/"
	cp "$ROOT/build/darwin/installer-components.plist" "$repo/build/darwin/"
	cp "$ROOT/third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt" "$repo/third_party/go-sqlcipher/"
	cp "$ROOT/internal/fonts/assets/FONT-NOTICES.txt" "$repo/internal/fonts/assets/"
	printf '%s\n' 'Go module notices fixture' '' 'Module: std' 'Module: example.com/fixture v1.0.0' \
		> "$repo/third_party/GO-MODULE-NOTICES.txt"
	cp "$ROOT/third_party/DUCKDB-NATIVE-NOTICES.txt" "$repo/third_party/"
	# Wails pinning has its own regressions. This matrix owns installer behavior.
	printf '#!/bin/bash\nexit 0\n' > "$repo/scripts/check-wails-version.sh"
	printf 'prior good package\n' > "$repo/prior.pkg"
	final="$repo/build/packages/gopmgr-fixture-darwin-arm64.pkg"
	cp "$repo/prior.pkg" "$final"
	case "$name" in
		missing-policy) plutil -remove 0.BundleIsRelocatable "$repo/build/darwin/installer-components.plist" ;;
		true-policy) plutil -replace 0.BundleIsRelocatable -bool YES "$repo/build/darwin/installer-components.plist" ;;
		symlink-target) rm "$final"; ln -s "$repo/prior.pkg" "$final" ;;
		directory-target) rm "$final"; mkdir "$final" ;;
	esac
	status=0
	output="$(cd "$repo" && PATH="$bin:$PATH" TMPDIR="$repo/tmp" VERSION=fixture \
		PKG_TEST_CASE="$name" PKG_TEST_DEFAULT="${4:-true}" PKG_TEST_LOG="$repo/events" \
		PKG_TEST_SOURCE="$work/source.app" bash scripts/package-macos-installer.sh 2>&1)" || status=$?
	events="$(cat "$repo/events")"
	case "$name" in
		success-*)
			[[ "$status" == 0 ]] || { printf '%s\n' "$output" >&2; fail "$name failed"; }
			[[ "$output" == *build/packages/gopmgr-fixture-darwin-arm64.pkg* ]] || fail "$name missing output path"
			! cmp -s "$repo/prior.pkg" "$final" || fail "$name did not publish"
			[[ "$(sed -n '2p' "$final")" == 7.8.9 && "$(sed -n '4p' "$final")" == false ]] || fail "$name changed version/relocation"
			[[ "$events" == *publish* && "$events" != *default-relocation* ]] || fail "$name used implicit policy"
			;;
		*)
			[[ "$status" != 0 ]] || fail "$name unexpectedly succeeded"
			if [ "$expected_error" = injected ]; then
				[[ "$status" == 43 ]] || { printf '%s\n' "$output" >&2; fail "$name failed at wrong stage"; }
			else
				[[ "$output" == *"$expected_error"* ]] || { printf '%s\n' "$output" >&2; fail "$name missing intended diagnostic"; }
			fi
			if [ "$name" = directory-target ]; then
				[[ -d "$final" && -z "$(ls -A "$final")" ]] || fail "$name changed target directory"
			else
				cmp -s "$repo/prior.pkg" "$final" || fail "$name changed prior artifact"
			fi
			if [ "$name" = symlink-target ]; then
				[[ -L "$final" && "$(cat "$repo/prior.pkg")" == 'prior good package' ]] || fail "$name changed symlink or referent"
			fi
			case "$expected_stage" in
				build) [[ "$events" == build ]] || fail "$name proceeded past the notices check" ;;
				package) [[ "$events" == $'build\npackage' ]] || fail "$name proceeded past packaging failure" ;;
				expand) [[ "$events" == $'build\npackage\nexpand' ]] || fail "$name proceeded past expansion failure" ;;
				scripts) [[ "$events" == $'build\npackage\nexpand' ]] || fail "$name parsed/published a scripted package" ;;
				parse) [[ "$events" == *expand* && "$events" == *parse* && "$events" != *publish* ]] || fail "$name did not reject during validation" ;;
				publish) [[ "$events" == *expand* && "$events" == *parse* && "$events" == *publish* ]] || fail "$name did not reach publication" ;;
			esac
			;;
	esac
	if [ "$name" = missing-policy ]; then
		[[ "$events" == *default-relocation* ]] || fail 'missing policy did not exercise simulated old default'
	fi
	[[ -z "$(find "$repo/tmp" -mindepth 1 -print)" ]] || fail "$name leaked staging"
	[[ -z "$(find "$repo/build/packages" -name '.gopmgr-pkg.*' -print)" ]] || fail "$name leaked publication staging"
	[[ "$(cat "$repo/build/bin/gopmgr.app/Contents/MacOS/gopmgr")" == 'fixture app' ]] || fail "$name changed source bundle"
	cases=$((cases + 1))
}

run_case success-true publish '' true
run_case success-false publish '' false
for name in missing-policy true-policy relocatable-output missing-relocation-attribute missing-relocate relocation-child duplicate-bundle wrong-destination wrong-identity malformed-xml parser-failure; do
	run_case "$name" parse 'violates the fixed-location policy'
done
run_case wrong-version parse 'version does not match the app'
run_case version-parser-failure parse 'version does not match the app'
run_case symlink-target parse 'refusing to replace a non-regular package target'
run_case directory-target parse 'refusing to replace a non-regular package target'
run_case scripts scripts 'contains installer scripts'
run_case missing-module-notice build 'lacks notices for 1 module(s)'
run_case package-failure package injected
run_case expansion-failure expand injected
run_case publication-failure publish injected
echo "package-macos-installer tests passed: $cases cases."
