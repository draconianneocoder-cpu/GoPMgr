#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build a local Apple Silicon .pkg installer for GoPMgr by wrapping the
# .app produced by `wails build` (via `make build`). The .app's bundle
# metadata (CFBundleIdentifier dev.gopmgr.GoPMgr, name, version) comes
# from build/darwin/Info.plist, so this script no longer hand-assembles a
# bundle - it ad-hoc signs the Wails output and packages it for
# /Applications. The package is local-test only: it is not signed with a
# Developer ID nor notarized.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/package-version-lib.sh"
cd "$SCRIPT_DIR/.."

if [ "$(go env GOOS)" != "darwin" ]; then
	echo "package-macos-installer: macOS packaging requires a Darwin host." >&2
	exit 2
fi

arch="$(go env GOARCH)"
if [ "$arch" != "arm64" ]; then
	echo "package-macos-installer: expected an Apple Silicon arm64 host, found '$arch'." >&2
	exit 2
fi

if ! command -v pkgbuild >/dev/null 2>&1; then
	echo "package-macos-installer: pkgbuild is required. Install Xcode command line tools first." >&2
	exit 2
fi

for tool in pkgutil plutil xmllint; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "package-macos-installer: $tool is required to validate the installer." >&2
		exit 2
	fi
done

if ! command -v wails >/dev/null 2>&1; then
	echo "package-macos-installer: the 'wails' CLI is required. Install with:" >&2
	echo "  go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0" >&2
	exit 2
fi

if ! GOPMGR_REQUIRE_WAILS_CLI=1 bash scripts/check-wails-version.sh >/dev/null; then
	echo "package-macos-installer: the installed Wails CLI does not match go.mod." >&2
	exit 2
fi

# Produce build/bin/<App>.app via the Wails CLI (make build -> wails build).
# Remove any prior output first so a stale, previously-executed binary (which
# macOS 15 stamps with com.apple.provenance) cannot break Wails' self-sign.
rm -rf build/bin
make build

# The component policy and approved install destination name this exact app.
app_dir="build/bin/gopmgr.app"
if [ ! -d "$app_dir" ] || [ -L "$app_dir" ]; then
	echo "package-macos-installer: expected build/bin/gopmgr.app after 'make build'." >&2
	exit 1
fi
app_name="$(basename "$app_dir" .app)"
app_binary="$app_dir/Contents/MacOS/$app_name"
if [ ! -f "$app_binary" ]; then
	echo "package-macos-installer: expected the app executable at $app_binary." >&2
	exit 1
fi
bash scripts/verify-module-notices.sh "$app_binary"

# Filename version: same derivation as package-macos.sh (see
# package-version-lib.sh), so the .dmg and .pkg from one build always agree.
# This is independent of the bundle's actual CFBundleVersion/
# CFBundleShortVersionString, which pkgbuild reads from the .app's own
# Info.plist (templated from wails.json's productVersion by `wails build`)
# and which stays a clean major.minor.patch on purpose — release gates
# require it.
full_version="$(release_version)"
safe_version="$(printf '%s' "$full_version" | tr -c 'A-Za-z0-9._-' '-')"

pkg_dir="build/packages"
pkg_path="$pkg_dir/${app_name}-${safe_version}-darwin-${arch}.pkg"
mkdir -p "$pkg_dir"

# Stage a clean copy so we strip extended attributes / resource forks
# without mutating the build/bin output.
staging="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-macos.XXXXXX")"
publish_dir=""
cleanup() {
	rm -rf "$staging"
	if [ -n "$publish_dir" ]; then
		rm -rf "$publish_dir"
	fi
}
trap cleanup EXIT
# Keep the candidate on the final artifact's filesystem for atomic replacement.
publish_dir="$(mktemp -d "$pkg_dir/.gopmgr-pkg.XXXXXX")"
payload="$staging/payload"
mkdir -p "$payload"
staged_app="$payload/${app_name}.app"
cp -R "$app_dir" "$staged_app"
mkdir -p "$staged_app/Contents/Resources/Licenses"
cp third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt "$staged_app/Contents/Resources/Licenses/go-sqlcipher.txt"
cp internal/fonts/assets/FONT-NOTICES.txt "$staged_app/Contents/Resources/Licenses/fonts.txt"
cp third_party/GO-MODULE-NOTICES.txt "$staged_app/Contents/Resources/Licenses/go-modules.txt"

if command -v xattr >/dev/null 2>&1; then
	xattr -cr "$staged_app"
fi
find "$staged_app" -name '._*' -delete
if command -v codesign >/dev/null 2>&1; then
	codesign --force --deep --sign - "$staged_app"
	codesign --verify --deep --strict --verbose=2 "$staged_app"
fi

bundle_id="$(plutil -extract CFBundleIdentifier raw -expect string -o - "$staged_app/Contents/Info.plist")"
bundle_version="$(plutil -extract CFBundleVersion raw -expect string -o - "$staged_app/Contents/Info.plist")"
if [ "$bundle_id" != "dev.gopmgr.GoPMgr" ] || [ -z "$bundle_version" ]; then
	echo "package-macos-installer: unexpected app identity or empty bundle version." >&2
	exit 1
fi

candidate="$publish_dir/installer.pkg"
COPYFILE_DISABLE=1 pkgbuild \
	--root "$payload" \
	--component-plist build/darwin/installer-components.plist \
	--identifier "$bundle_id" \
	--version "$bundle_version" \
	--install-location /Applications \
	"$candidate"

expanded="$publish_dir/expanded"
pkgutil --expand "$candidate" "$expanded"
if [ -e "$expanded/Scripts" ] || [ -L "$expanded/Scripts" ]; then
	echo "package-macos-installer: generated package contains installer scripts." >&2
	exit 1
fi
if ! valid_policy="$(xmllint --nonet --xpath 'boolean(/pkg-info[
	@identifier="dev.gopmgr.GoPMgr" and @install-location="/Applications"
	and @relocatable="false" and count(bundle)=1
	and bundle[@path="./gopmgr.app" and @id="dev.gopmgr.GoPMgr"]
	and count(relocate)=1 and not(relocate/*)
	and bundle-version/bundle[@id="dev.gopmgr.GoPMgr"]
	and upgrade-bundle/bundle[@id="dev.gopmgr.GoPMgr"]
	and strict-identifier/bundle[@id="dev.gopmgr.GoPMgr"]
	and not(update-bundle/*)
])' "$expanded/PackageInfo")" || [ "$valid_policy" != "true" ]; then
	echo "package-macos-installer: generated package violates the fixed-location policy." >&2
	exit 1
fi
if ! package_version="$(xmllint --nonet --xpath 'string(/pkg-info/@version)' "$expanded/PackageInfo")" ||
	[ "$package_version" != "$bundle_version" ]; then
	echo "package-macos-installer: generated package version does not match the app." >&2
	exit 1
fi
if [ -L "$pkg_path" ] || { [ -e "$pkg_path" ] && [ ! -f "$pkg_path" ]; }; then
	echo "package-macos-installer: refusing to replace a non-regular package target." >&2
	exit 1
fi
mv -f "$candidate" "$pkg_path"

echo "$pkg_path"
