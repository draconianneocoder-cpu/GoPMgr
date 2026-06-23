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

mkdir -p build/packages
dmg="build/packages/PMForge-${VERSION}-arm64.dmg"
rm -f "$dmg"

# --- Code-signing hook (no-op unless MACOS_SIGN_IDENTITY is set) ---
if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
	echo "package-macos: codesigning $app ..."
	codesign --deep --force --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" "$app"
	# Notarize + staple once credentials are configured:
	#   xcrun notarytool submit "$dmg" --keychain-profile "PMFORGE_NOTARY" --wait
	#   xcrun stapler staple "$dmg"
fi

if command -v create-dmg >/dev/null 2>&1; then
	create-dmg \
		--volname "PMForge ${VERSION}" \
		--window-size 640 360 \
		--icon-size 110 \
		--icon "$(basename "$app")" 165 190 \
		--app-drop-link 470 190 \
		"$dmg" "$app" \
	|| hdiutil create -volname "PMForge ${VERSION}" -srcfolder "$app" -ov -format UDZO "$dmg"
else
	echo "package-macos: create-dmg not found; using hdiutil (no drop-link layout)." >&2
	hdiutil create -volname "PMForge ${VERSION}" -srcfolder "$app" -ov -format UDZO "$dmg"
fi

echo "package-macos: $dmg"
