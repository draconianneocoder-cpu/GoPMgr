#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Downloads the bundled TrueType fonts into internal/fonts/assets/.
#
# The fonts are NOT committed to the repository (they are large
# binaries with their own upstream licenses). This script fetches them
# from their canonical release URLs so the go:embed directive in
# internal/fonts/manager.go can bundle them into the build.
#
# Every font fetched here is free for commercial AND personal use and
# GPL-3.0-compatible (SIL OFL 1.1 or the Bitstream Vera license).
#
# Usage:
#   scripts/fetch-fonts.sh            # fetch all families
#   scripts/fetch-fonts.sh --list     # list what would be fetched
#
# Idempotent: existing files are skipped unless --force is passed.

set -eu
cd "$(dirname "$0")/.."

ASSETS="internal/fonts/assets"
FORCE=0
LIST=0

for arg in "$@"; do
	case "$arg" in
		--force) FORCE=1 ;;
		--list)  LIST=1 ;;
		*) echo "unknown flag: $arg" >&2; exit 2 ;;
	esac
done

mkdir -p "$ASSETS"

OK_COUNT=0
FAIL_COUNT=0
FAILED_LIST=""

# downloader picks curl or wget, whichever is present.
download() {
	# $1 = url, $2 = output path. Returns non-zero on HTTP/transport error.
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -q "$1" -O "$2"
	else
		echo "ERROR: neither curl nor wget is installed" >&2
		exit 1
	fi
}

# is_truetype verifies the first 4 bytes are a TrueType signature
# (0x00010000 or "true"). Catches the common failure where a 404 page
# is saved as a .ttf. Rejects OpenType/CFF ("OTTO") which the PDF
# engine cannot embed.
is_truetype() {
	local f="$1"
	[ -s "$f" ] || return 1
	local sig
	sig=$(head -c 4 "$f" | od -An -tx1 | tr -d ' \n')
	case "$sig" in
		00010000) return 0 ;;  # sfnt 1.0 TrueType
		74727565) return 0 ;;  # "true"
		*) return 1 ;;
	esac
}

# fetch <dest-filename> <url>. Fault-tolerant: a single failure is
# recorded and the run continues, so one drifted URL doesn't block the
# rest of the catalog.
fetch() {
	local name="$1"
	local dest="$ASSETS/$1"
	local url="$2"
	if [ "$LIST" -eq 1 ]; then
		printf '%-34s <- %s\n' "$name" "$url"
		return
	fi
	if [ -f "$dest" ] && [ "$FORCE" -eq 0 ]; then
		echo "skip   $name (exists)"
		OK_COUNT=$((OK_COUNT + 1))
		return
	fi
	if download "$url" "$dest" 2>/dev/null && is_truetype "$dest"; then
		echo "ok     $name"
		OK_COUNT=$((OK_COUNT + 1))
	else
		rm -f "$dest"
		echo "FAIL   $name  ($url)"
		FAIL_COUNT=$((FAIL_COUNT + 1))
		FAILED_LIST="$FAILED_LIST $name"
	fi
}

# --- Liberation (Sans / Serif / Mono), SIL OFL 1.1 -------------------
# Canonical release tarball is unpacked upstream; the GitHub raw mirror
# below serves the individual TTFs at a pinned release tag.
LIB="https://raw.githubusercontent.com/liberationfonts/liberation-fonts/2.1.5/src"
# NOTE: the upstream layout ships compiled TTFs in release archives, not
# in src/. If this path 404s, download the release archive instead:
#   https://github.com/liberationfonts/liberation-fonts/files/7261482/liberation-fonts-ttf-2.1.5.tar.gz
# and extract the .ttf files into internal/fonts/assets/.
fetch "LiberationSans-Regular.ttf"     "$LIB/LiberationSans-Regular.ttf"
fetch "LiberationSans-Bold.ttf"        "$LIB/LiberationSans-Bold.ttf"
fetch "LiberationSans-Italic.ttf"      "$LIB/LiberationSans-Italic.ttf"
fetch "LiberationSans-BoldItalic.ttf"  "$LIB/LiberationSans-BoldItalic.ttf"
fetch "LiberationSerif-Regular.ttf"    "$LIB/LiberationSerif-Regular.ttf"
fetch "LiberationSerif-Bold.ttf"       "$LIB/LiberationSerif-Bold.ttf"
fetch "LiberationSerif-Italic.ttf"     "$LIB/LiberationSerif-Italic.ttf"
fetch "LiberationSerif-BoldItalic.ttf" "$LIB/LiberationSerif-BoldItalic.ttf"
fetch "LiberationMono-Regular.ttf"     "$LIB/LiberationMono-Regular.ttf"
fetch "LiberationMono-Bold.ttf"        "$LIB/LiberationMono-Bold.ttf"
fetch "LiberationMono-Italic.ttf"      "$LIB/LiberationMono-Italic.ttf"
fetch "LiberationMono-BoldItalic.ttf"  "$LIB/LiberationMono-BoldItalic.ttf"

# --- DejaVu Sans, Bitstream Vera license -----------------------------
DEJAVU="https://raw.githubusercontent.com/dejavu-fonts/dejavu-fonts/version_2_37/ttf"
fetch "DejaVuSans.ttf"             "$DEJAVU/DejaVuSans.ttf"
fetch "DejaVuSans-Bold.ttf"        "$DEJAVU/DejaVuSans-Bold.ttf"
fetch "DejaVuSans-Oblique.ttf"     "$DEJAVU/DejaVuSans-Oblique.ttf"
fetch "DejaVuSans-BoldOblique.ttf" "$DEJAVU/DejaVuSans-BoldOblique.ttf"

# --- Noto Sans, SIL OFL 1.1 ------------------------------------------
NOTO="https://raw.githubusercontent.com/notofonts/latin-greek-cyrillic/main/fonts/NotoSans/hinted/ttf"
fetch "NotoSans-Regular.ttf"    "$NOTO/NotoSans-Regular.ttf"
fetch "NotoSans-Bold.ttf"       "$NOTO/NotoSans-Bold.ttf"
fetch "NotoSans-Italic.ttf"     "$NOTO/NotoSans-Italic.ttf"
fetch "NotoSans-BoldItalic.ttf" "$NOTO/NotoSans-BoldItalic.ttf"

# --- Source Sans 3, SIL OFL 1.1 --------------------------------------
SSP="https://raw.githubusercontent.com/adobe-fonts/source-sans/release/TTF"
fetch "SourceSans3-Regular.ttf" "$SSP/SourceSans3-Regular.ttf"
fetch "SourceSans3-Bold.ttf"    "$SSP/SourceSans3-Bold.ttf"
fetch "SourceSans3-It.ttf"      "$SSP/SourceSans3-It.ttf"
fetch "SourceSans3-BoldIt.ttf"  "$SSP/SourceSans3-BoldIt.ttf"

# --- JetBrains Mono, SIL OFL 1.1 -------------------------------------
JBM="https://raw.githubusercontent.com/JetBrains/JetBrainsMono/master/fonts/ttf"
fetch "JetBrainsMono-Regular.ttf"    "$JBM/JetBrainsMono-Regular.ttf"
fetch "JetBrainsMono-Bold.ttf"       "$JBM/JetBrainsMono-Bold.ttf"
fetch "JetBrainsMono-Italic.ttf"     "$JBM/JetBrainsMono-Italic.ttf"
fetch "JetBrainsMono-BoldItalic.ttf" "$JBM/JetBrainsMono-BoldItalic.ttf"

if [ "$LIST" -eq 1 ]; then
	exit 0
fi

echo
echo "Fonts fetched: $OK_COUNT ok, $FAIL_COUNT failed."
if [ "$FAIL_COUNT" -gt 0 ]; then
	echo "Failed files:$FAILED_LIST" >&2
	echo "URLs drift over time; update scripts/fetch-fonts.sh or drop the" >&2
	echo "missing .ttf files into $ASSETS/ manually. The build still works" >&2
	echo "with whatever fonts are present (others fall back to Helvetica)." >&2
fi
echo "Rebuild with 'make build' to embed the fetched fonts."

# Exit non-zero only if the default family (Liberation Sans) is missing,
# since that's the one the renderers prefer.
if [ ! -f "$ASSETS/LiberationSans-Regular.ttf" ]; then
	echo "WARNING: default family Liberation Sans was not fetched." >&2
	exit 1
fi
