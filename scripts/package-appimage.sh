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
fetch() { curl -fsSL "$1" -o "$2"; chmod +x "$2"; }
fetch "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage" "$tools/linuxdeploy"
fetch "https://github.com/linuxdeploy/linuxdeploy-plugin-gtk/releases/download/continuous/linuxdeploy-plugin-gtk.sh" "$tools/linuxdeploy-plugin-gtk.sh"

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
