#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build a local Apple Silicon .pkg installer for PMForge by wrapping the
# .app produced by `wails build` (via `make build`). The .app's bundle
# metadata (CFBundleIdentifier dev.pmforge.PMForge, name, version) comes
# from build/darwin/Info.plist, so this script no longer hand-assembles a
# bundle - it ad-hoc signs the Wails output and packages it for
# /Applications. The package is local-test only: it is not signed with a
# Developer ID nor notarized.

set -euo pipefail
cd "$(dirname "$0")/.."

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

if ! command -v wails >/dev/null 2>&1; then
	echo "package-macos-installer: the 'wails' CLI is required. Install with:" >&2
	echo "  go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
	exit 2
fi

# Produce build/bin/<App>.app via the Wails CLI (make build -> wails build).
# Remove any prior output first so a stale, previously-executed binary (which
# macOS 15 stamps with com.apple.provenance) cannot break Wails' self-sign.
rm -rf build/bin
make build

# Locate the Wails-produced .app bundle.
app_dir="$(find build/bin -maxdepth 1 -name '*.app' -type d 2>/dev/null | head -1)"
if [ -z "$app_dir" ] || [ ! -d "$app_dir" ]; then
	echo "package-macos-installer: no .app found under build/bin after 'make build'." >&2
	exit 1
fi
app_name="$(basename "$app_dir" .app)"

# Read the canonical version from wails.json (single source of truth; also
# what gets templated into the bundle's Info.plist).
full_version="$(grep -oE '"productVersion" *: *"[^"]+"' wails.json | sed -E 's/.*"([^"]+)"$/\1/')"
if [ -z "$full_version" ]; then
	echo "package-macos-installer: failed to read productVersion from wails.json." >&2
	exit 1
fi
safe_version="$(printf '%s' "$full_version" | tr -c 'A-Za-z0-9._-' '-')"

pkg_dir="build/packages"
pkg_path="$pkg_dir/${app_name}-${safe_version}-darwin-${arch}.pkg"
mkdir -p "$pkg_dir"

# Stage a clean copy so we strip extended attributes / resource forks
# without mutating the build/bin output.
staging="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-macos.XXXXXX")"
trap 'rm -rf "$staging"' EXIT
staged_app="$staging/${app_name}.app"
cp -R "$app_dir" "$staged_app"

if command -v xattr >/dev/null 2>&1; then
	xattr -cr "$staged_app"
fi
find "$staged_app" -name '._*' -delete
if command -v codesign >/dev/null 2>&1; then
	codesign --force --deep --sign - "$staged_app"
	codesign --verify --deep --strict --verbose=2 "$staged_app"
fi

rm -f "$pkg_path"
COPYFILE_DISABLE=1 pkgbuild \
	--component "$staged_app" \
	--install-location /Applications \
	"$pkg_path"

echo "$pkg_path"
