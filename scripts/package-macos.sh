#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build a macOS .dmg (drag-to-Applications) from the Wails .app under
# build/bin. Run `wails build -platform darwin/arm64` first (Apple Silicon).
#
# Signing/notarization is OFF by default: the .dmg installs and runs, but
# Gatekeeper shows an "unidentified developer" warning. To sign, export
# MACOS_SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)" and, for
# notarization, fill in the notarytool block below (needs an Apple Developer
# account + an App Store Connect API key or app-specific password).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-0.0.0}"

app="$(ls -d build/bin/*.app 2>/dev/null | head -1 || true)"
if [ -z "$app" ] || [ ! -d "$app" ]; then
	echo "package-macos: no .app under build/bin — run 'wails build -platform darwin/arm64' first." >&2
	exit 1
fi
app_binary="$(find "$app/Contents/MacOS" -maxdepth 1 -type f -perm -111 2>/dev/null | head -1 || true)"
if [ -z "$app_binary" ] || [ ! -f "$app_binary" ]; then
	echo "package-macos: no executable found in $app/Contents/MacOS." >&2
	exit 1
fi
if [ "${PMFORGE_PACKAGE_LAYOUT_TEST:-0}" != "1" ]; then
	scripts/verify-duckdb-linked.sh "$app_binary"
fi
product_name="$(grep -oE '"productName" *: *"[^"]+"' wails.json | sed -E 's/.*"([^"]+)"$/\1/' || true)"
product_name="${product_name:-$(basename "$app" .app)}"
visible_app="${product_name}.app"

mkdir -p build/packages
dmg="build/packages/PMForge-${VERSION}-arm64.dmg"
rm -f "$dmg"

staging="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-dmg.XXXXXX")"
trap 'rm -rf "$staging"' EXIT
staged_app="$staging/$visible_app"
cp -R "$app" "$staged_app"

if command -v xattr >/dev/null 2>&1; then
	xattr -cr "$staged_app"
fi
find "$staged_app" -name '._*' -delete

stage_dmg_root() {
	local dmg_root="$1"
	local applications_link="$2"
	rm -rf "$dmg_root"
	mkdir -p "$dmg_root"
	cp -R "$staged_app" "$dmg_root/$visible_app"
	if [ "$applications_link" = "yes" ]; then
		ln -s /Applications "$dmg_root/Applications"
	fi
}

create_hdiutil_dmg() {
	local dmg_root="$staging/dmg-root"
	stage_dmg_root "$dmg_root" yes
	COPYFILE_DISABLE=1 hdiutil create -volname "PMForge ${VERSION}" -srcfolder "$dmg_root" -ov -format UDZO "$dmg"
}

# --- Code-signing hook (no-op unless MACOS_SIGN_IDENTITY is set) ---
if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
	echo "package-macos: codesigning $staged_app ..."
	codesign --deep --force --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" "$staged_app"
	# Notarize + staple once credentials are configured:
	#   xcrun notarytool submit "$dmg" --keychain-profile "PMFORGE_NOTARY" --wait
	#   xcrun stapler staple "$dmg"
fi

if command -v create-dmg >/dev/null 2>&1; then
	create_dmg_root="$staging/create-dmg-root"
	stage_dmg_root "$create_dmg_root" no
	create-dmg \
		--volname "PMForge ${VERSION}" \
		--window-size 640 360 \
		--icon-size 110 \
		--icon "$visible_app" 165 190 \
		--app-drop-link 470 190 \
		"$dmg" "$create_dmg_root" \
	|| create_hdiutil_dmg
else
	echo "package-macos: create-dmg not found; using hdiutil fallback layout." >&2
	create_hdiutil_dmg
fi

echo "package-macos: $dmg"
