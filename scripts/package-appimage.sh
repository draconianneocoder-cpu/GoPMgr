#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build a portable AppImage from build/bin/pmforge using linuxdeploy + the
# GTK plugin, which bundles the GTK3/WebKit2GTK runtime so the AppImage runs
# on distros that don't have those libraries installed.
#
# Requires network access (fetches the linuxdeploy tools). The
# APPIMAGE_EXTRACT_AND_RUN flag avoids needing FUSE, so it works on CI.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-0.0.0}"

if [ ! -x build/bin/pmforge ]; then
	echo "package-appimage: build/bin/pmforge missing — run wails build first." >&2
	exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
appdir="$work/AppDir"
mkdir -p "$appdir/usr/bin" \
	"$appdir/usr/share/applications" \
	"$appdir/usr/share/icons/hicolor/256x256/apps"
cp build/bin/pmforge "$appdir/usr/bin/pmforge"
cp build/linux/pmforge.desktop "$appdir/usr/share/applications/pmforge.desktop"
cp build/appicon.png "$appdir/usr/share/icons/hicolor/256x256/apps/pmforge.png"

tools="$work/tools"
mkdir -p "$tools"

# Supply-chain hardening. linuxdeploy publishes only a rolling "continuous"
# release, so a plain download is an unpinned, unverified moving target that
# executes inside the release pipeline. We instead pin the exact artifact
# bytes by SHA-256 (build/linux/appimage-tools.sha256) and verify on every
# build, fail-closed. The build tools are NOT chmod +x'd or run until their
# bytes match the committed digests.
#
# To adopt new upstream tool builds, refresh the digests deliberately on a
# trusted network and commit the result:
#     APPIMAGE_TOOLS_REFRESH=1 bash scripts/package-appimage.sh
LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage"
LINUXDEPLOY_GTK_URL="https://github.com/linuxdeploy/linuxdeploy-plugin-gtk/releases/download/continuous/linuxdeploy-plugin-gtk.sh"
sums="build/linux/appimage-tools.sha256"

ld="$tools/linuxdeploy"
gtk="$tools/linuxdeploy-plugin-gtk.sh"
# Download bytes only; do not chmod/execute until verified.
curl --fail --silent --show-error --location "$LINUXDEPLOY_URL" -o "$ld"
curl --fail --silent --show-error --location "$LINUXDEPLOY_GTK_URL" -o "$gtk"

if [ "${APPIMAGE_TOOLS_REFRESH:-0}" = "1" ]; then
	{
		echo "# Pinned SHA-256 digests for the AppImage build tools."
		echo "# Upstream ships only a rolling 'continuous' release, so these bytes are"
		echo "# pinned here and verified fail-closed by scripts/package-appimage.sh."
		echo "# Regenerate deliberately on a trusted network with:"
		echo "#   APPIMAGE_TOOLS_REFRESH=1 bash scripts/package-appimage.sh"
		printf '%s  linuxdeploy\n' "$(sha256sum "$ld" | cut -d' ' -f1)"
		printf '%s  linuxdeploy-plugin-gtk.sh\n' "$(sha256sum "$gtk" | cut -d' ' -f1)"
	} >"$sums"
	echo "package-appimage: wrote pinned digests to $sums — review and commit them." >&2
	exit 0
fi

if [ ! -f "$sums" ]; then
	echo "package-appimage: missing pinned tool digests ($sums)." >&2
	echo "  Run once on a trusted network, then commit the file:" >&2
	echo "    APPIMAGE_TOOLS_REFRESH=1 bash scripts/package-appimage.sh" >&2
	exit 1
fi

verify_tool() {
	# verify_tool <file> <name-in-sums>
	local file="$1" name="$2" want got
	want="$(awk -v n="$name" '$2 == n {print $1}' "$sums")"
	if [ -z "$want" ]; then
		echo "package-appimage: no pinned digest for '$name' in $sums." >&2
		exit 1
	fi
	got="$(sha256sum "$file" | cut -d' ' -f1)"
	if [ "$want" != "$got" ]; then
		echo "package-appimage: SHA-256 mismatch for '$name' — refusing to run untrusted build tooling." >&2
		echo "  expected: $want" >&2
		echo "  actual:   $got" >&2
		echo "  If upstream changed intentionally, refresh with APPIMAGE_TOOLS_REFRESH=1 and commit $sums." >&2
		exit 1
	fi
}
verify_tool "$ld" "linuxdeploy"
verify_tool "$gtk" "linuxdeploy-plugin-gtk.sh"
chmod +x "$ld" "$gtk"

mkdir -p build/packages
out="$(pwd)/build/packages/PMForge-${VERSION}-x86_64.AppImage"
rm -f "$out"

export APPIMAGE_EXTRACT_AND_RUN=1
export OUTPUT="$out"
PATH="$tools:$PATH" "$tools/linuxdeploy" \
	--appdir "$appdir" \
	--plugin gtk \
	--desktop-file "$appdir/usr/share/applications/pmforge.desktop" \
	--icon-file "$appdir/usr/share/icons/hicolor/256x256/apps/pmforge.png" \
	--output appimage

echo "package-appimage: $out"
