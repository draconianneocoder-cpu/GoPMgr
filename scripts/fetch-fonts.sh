#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Downloads the bundled TrueType fonts into internal/fonts/assets/.
#
# The IBM Plex families and Source Sans 3 are committed; IBM Plex Sans is the
# default document font and the release-critical PDF/A baseline. This script
# fetches the remaining optional families and can refresh the committed fonts
# from their pinned upstream commits when --force is used. CI and release
# builds never run it (scripts/release-gate-scope-check.sh enforces that), so
# the optional families only reach local developer builds.
#
# Every font fetched here is free for commercial AND personal use. Each one is
# licensed under the SIL Open Font License 1.1, except DejaVu Sans, which uses
# the Bitstream Vera license (its Arev glyphs carry equivalent terms, and the
# DejaVu changes are public domain). Both licenses allow the fonts to be
# bundled and embedded with any software; the fonts keep their own license.
#
# Every source below is pinned to an immutable upstream commit, tag, or
# release archive, and every download is checked against a SHA-256 recorded
# here. This keeps the bytes, and therefore the license documented in
# internal/fonts/assets/README.md, from changing underneath the catalog.
#
# This script downloads font files only. Each family's upstream license
# notice is listed in internal/fonts/assets/README.md.
#
# Usage:
#   bash scripts/fetch-fonts.sh            # fetch all families
#   bash scripts/fetch-fonts.sh --list     # list what would be fetched
#   bash scripts/fetch-fonts.sh --force    # re-download files that exist
#
# Idempotent: existing files are skipped unless --force is passed. A file is
# replaced only after its download passes verification, so a failed or
# offline run never deletes a font that is already present.

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

OK_COUNT=0
FAIL_COUNT=0
FAILED_LIST=""
WORK=""

if [ "$LIST" -eq 0 ]; then
	mkdir -p "$ASSETS"
	WORK="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-fonts.XXXXXX")"
	trap 'rm -rf "$WORK"' EXIT
fi

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

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		echo "ERROR: sha256sum or shasum is required to verify downloads" >&2
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

# has_sha256 <file> <expected> succeeds when the file's SHA-256 matches.
has_sha256() {
	[ "$(sha256_file "$1")" = "$2" ]
}

record_ok() {
	echo "ok     $1"
	OK_COUNT=$((OK_COUNT + 1))
}

record_fail() {
	echo "FAIL   $1  ($2)"
	FAIL_COUNT=$((FAIL_COUNT + 1))
	FAILED_LIST="$FAILED_LIST $1"
}

# skip_existing <dest-filename> succeeds (and counts the file) when it is
# already present and --force was not passed.
skip_existing() {
	if [ -f "$ASSETS/$1" ] && [ "$FORCE" -eq 0 ]; then
		echo "skip   $1 (exists)"
		OK_COUNT=$((OK_COUNT + 1))
		return 0
	fi
	return 1
}

# fetch <dest-filename> <url> <sha256>. Fault-tolerant: a single failure is
# recorded and the run continues, so one unreachable host doesn't block the
# rest of the catalog.
fetch() {
	local name="$1" url="$2" sha="$3"
	local tmp="$WORK/$1"
	if [ "$LIST" -eq 1 ]; then
		printf '%-34s <- %s\n' "$name" "$url"
		return
	fi
	if skip_existing "$name"; then
		return
	fi
	if download "$url" "$tmp" 2>/dev/null && is_truetype "$tmp" && has_sha256 "$tmp" "$sha"; then
		mv -f "$tmp" "$ASSETS/$name"
		record_ok "$name"
	else
		rm -f "$tmp"
		record_fail "$name" "$url"
	fi
}

# Some families publish static TTFs only inside a release archive. use_archive
# names the archive and its SHA-256; extract then pulls single members out of
# it. The archive is downloaded at most once, and only if a member is needed.
ARCHIVE_URL=""
ARCHIVE_SHA=""
ARCHIVE_PATH=""
ARCHIVE_STATE=""
ARCHIVE_COUNT=0

use_archive() {
	ARCHIVE_URL="$1"
	ARCHIVE_SHA="$2"
	ARCHIVE_PATH=""
	ARCHIVE_STATE=""
}

ensure_archive() {
	case "$ARCHIVE_STATE" in
		ok) return 0 ;;
		failed) return 1 ;;
	esac
	ARCHIVE_COUNT=$((ARCHIVE_COUNT + 1))
	ARCHIVE_PATH="$WORK/archive-$ARCHIVE_COUNT-${ARCHIVE_URL##*/}"
	if download "$ARCHIVE_URL" "$ARCHIVE_PATH" 2>/dev/null && has_sha256 "$ARCHIVE_PATH" "$ARCHIVE_SHA"; then
		ARCHIVE_STATE=ok
		return 0
	fi
	rm -f "$ARCHIVE_PATH"
	ARCHIVE_STATE=failed
	echo "FAIL   archive download or checksum ($ARCHIVE_URL)"
	return 1
}

# extract_member <archive> <member> <output>
extract_member() {
	case "$1" in
		*.zip) unzip -p "$1" "$2" >"$3" ;;
		*) tar -xOf "$1" "$2" >"$3" ;;
	esac
}

# extract <dest-filename> <member-path-inside-the-current-archive>
extract() {
	local name="$1" member="$2"
	local tmp="$WORK/$1"
	if [ "$LIST" -eq 1 ]; then
		printf '%-34s <- %s (%s)\n' "$name" "$ARCHIVE_URL" "$member"
		return
	fi
	if skip_existing "$name"; then
		return
	fi
	if ensure_archive && extract_member "$ARCHIVE_PATH" "$member" "$tmp" 2>/dev/null && is_truetype "$tmp"; then
		mv -f "$tmp" "$ASSETS/$name"
		record_ok "$name"
	else
		rm -f "$tmp"
		record_fail "$name" "$ARCHIVE_URL: $member"
	fi
}

# --- IBM Plex (Sans, Serif, Mono, Sans Condensed), SIL OFL 1.1 ----------
# Committed: the default document font, the PDF/A baseline, and the app's
# typeface. Each family is pinned to the commit of its IBM release tag; the
# SHA-256 values match check-required-font-assets.sh. fpdf maps its Bold
# style to SemiBold (docs/design/typography-ibm-plex.md), so only these four
# faces per family are bundled.
# Sans 3.005, tag @ibm/plex-sans@1.1.0
PLEX_SANS="https://raw.githubusercontent.com/IBM/plex/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans/fonts/complete/ttf"
fetch "IBMPlexSans-Regular.ttf" "$PLEX_SANS/IBMPlexSans-Regular.ttf" "975dcda37d80f038dcd143c22e33ca2d97a0cc5a929aace1c749153b0fe1afa5"
fetch "IBMPlexSans-SemiBold.ttf" "$PLEX_SANS/IBMPlexSans-SemiBold.ttf" "a20caf8286023a6a7a85e40b1d2a4ae9fc3e3b1f9eda8f4c542dd4986af67bb1"
fetch "IBMPlexSans-Italic.ttf" "$PLEX_SANS/IBMPlexSans-Italic.ttf" "a9c6ef9942c49e49d11e11a6dacc0b3a087978757e9b22a06b8ac22a6400fb15"
fetch "IBMPlexSans-SemiBoldItalic.ttf" "$PLEX_SANS/IBMPlexSans-SemiBoldItalic.ttf" "147106b619423a82b40f6d64bace074ad5b786e59f48023bda69d63b1755fb82"

# Serif 3.006, tag @ibm/plex-serif@2.0.0
PLEX_SERIF="https://raw.githubusercontent.com/IBM/plex/434af578549afcfdabd281f386e0ff7314fd20b0/packages/plex-serif/fonts/complete/ttf"
fetch "IBMPlexSerif-Regular.ttf" "$PLEX_SERIF/IBMPlexSerif-Regular.ttf" "6c490ccd38e856c674e0d163f3f40742a75058b7b935998dbfbc1745ba54e474"
fetch "IBMPlexSerif-SemiBold.ttf" "$PLEX_SERIF/IBMPlexSerif-SemiBold.ttf" "857edeaaff0f3913f4dfaf6bc0e9c89ef73e475429cf63d6a3cb5d1022a213d5"
fetch "IBMPlexSerif-Italic.ttf" "$PLEX_SERIF/IBMPlexSerif-Italic.ttf" "d28aa63cf618439712d6b459c5649af1d6d789cb911994f2357f1ce44128f163"
fetch "IBMPlexSerif-SemiBoldItalic.ttf" "$PLEX_SERIF/IBMPlexSerif-SemiBoldItalic.ttf" "c1bf6e7e5d3fd740916184e171703db25729fefd4d2423ae1b79a6afcf1a34ef"

# Mono 2.005, tag @ibm/plex-mono@2.5.0
PLEX_MONO="https://raw.githubusercontent.com/IBM/plex/2f9ba1b25957d958db71a849e85d72e3ecfb845a/packages/plex-mono/fonts/complete/ttf"
fetch "IBMPlexMono-Regular.ttf" "$PLEX_MONO/IBMPlexMono-Regular.ttf" "7c6fbddca4b700be918f5f6183d9bd4464fa427fe435f0b480d77fe2bb8c5a43"
fetch "IBMPlexMono-SemiBold.ttf" "$PLEX_MONO/IBMPlexMono-SemiBold.ttf" "f04d7c488ddf7d1fa99f2574efc3406ea4cbe17bb1af3a1ab960f84d0c96a172"
fetch "IBMPlexMono-Italic.ttf" "$PLEX_MONO/IBMPlexMono-Italic.ttf" "a55368deb953f594ac18336a1029009ed820e2d9abbb57ad601f61e22bec072b"
fetch "IBMPlexMono-SemiBoldItalic.ttf" "$PLEX_MONO/IBMPlexMono-SemiBoldItalic.ttf" "d2039ddfa9236d642fafae5aa46446c54eb7961b3771a1ca3e9bba960172b9bb"

# Sans Condensed 3.000, tag @ibm/plex-sans-condensed@2.0.0
PLEX_SANS_CONDENSED="https://raw.githubusercontent.com/IBM/plex/bb3ab6404e1881ea286f8742dc839e09057db6dd/packages/plex-sans-condensed/fonts/complete/ttf"
fetch "IBMPlexSansCondensed-Regular.ttf" "$PLEX_SANS_CONDENSED/IBMPlexSansCondensed-Regular.ttf" "87aaee6ba9d2f80b8a3c1f6669e8c3d478b28b482f1d804917b3894694ccc12b"
fetch "IBMPlexSansCondensed-SemiBold.ttf" "$PLEX_SANS_CONDENSED/IBMPlexSansCondensed-SemiBold.ttf" "acc2b1e3b9597ee13b08b767f845bd038fe120cbee806859af7c01b60fe6e9e6"
fetch "IBMPlexSansCondensed-Italic.ttf" "$PLEX_SANS_CONDENSED/IBMPlexSansCondensed-Italic.ttf" "368d896f1cd1dc9f9b926135bd5c3154aa6f3dcd53f2b330bce487968a3a3e4a"
fetch "IBMPlexSansCondensed-SemiBoldItalic.ttf" "$PLEX_SANS_CONDENSED/IBMPlexSansCondensed-SemiBoldItalic.ttf" "d6dbb283c007854cd5fd56b86f5c9da8fcac497e50aa8e029258de21599ab8f6"
# --- Liberation 2.1.5 (Sans / Serif / Mono), SIL OFL 1.1 -------------
# The repository holds only FontForge sources; the TTFs ship in this release
# archive (linked from the 2.1.5 release page). Its SHA-256 matches the
# Homebrew font-liberation cask and FreeBSD's x11-fonts/liberation-fonts-ttf.
use_archive \
	"https://github.com/liberationfonts/liberation-fonts/files/7261482/liberation-fonts-ttf-2.1.5.tar.gz" \
	"7191c669bf38899f73a2094ed00f7b800553364f90e2637010a69c0e268f25d0"
LIB="liberation-fonts-ttf-2.1.5"
extract "LiberationSans-Regular.ttf"     "$LIB/LiberationSans-Regular.ttf"
extract "LiberationSans-Bold.ttf"        "$LIB/LiberationSans-Bold.ttf"
extract "LiberationSans-Italic.ttf"      "$LIB/LiberationSans-Italic.ttf"
extract "LiberationSans-BoldItalic.ttf"  "$LIB/LiberationSans-BoldItalic.ttf"
extract "LiberationSerif-Regular.ttf"    "$LIB/LiberationSerif-Regular.ttf"
extract "LiberationSerif-Bold.ttf"       "$LIB/LiberationSerif-Bold.ttf"
extract "LiberationSerif-Italic.ttf"     "$LIB/LiberationSerif-Italic.ttf"
extract "LiberationSerif-BoldItalic.ttf" "$LIB/LiberationSerif-BoldItalic.ttf"
extract "LiberationMono-Regular.ttf"     "$LIB/LiberationMono-Regular.ttf"
extract "LiberationMono-Bold.ttf"        "$LIB/LiberationMono-Bold.ttf"
extract "LiberationMono-Italic.ttf"      "$LIB/LiberationMono-Italic.ttf"
extract "LiberationMono-BoldItalic.ttf"  "$LIB/LiberationMono-BoldItalic.ttf"

# --- DejaVu Sans 2.37, Bitstream Vera license --------------------------
# The repository holds only sources; the TTFs ship in the release archive.
use_archive \
	"https://github.com/dejavu-fonts/dejavu-fonts/releases/download/version_2_37/dejavu-fonts-ttf-2.37.tar.bz2" \
	"fa9ca4d13871dd122f61258a80d01751d603b4d3ee14095d65453b4e846e17d7"
DEJAVU="dejavu-fonts-ttf-2.37/ttf"
extract "DejaVuSans.ttf"             "$DEJAVU/DejaVuSans.ttf"
extract "DejaVuSans-Bold.ttf"        "$DEJAVU/DejaVuSans-Bold.ttf"
extract "DejaVuSans-Oblique.ttf"     "$DEJAVU/DejaVuSans-Oblique.ttf"
extract "DejaVuSans-BoldOblique.ttf" "$DEJAVU/DejaVuSans-BoldOblique.ttf"

# --- Noto Sans 2.015, SIL OFL 1.1 ----------------------------------------
NOTO="https://raw.githubusercontent.com/notofonts/notofonts.github.io/42af67ccd12bc0e31ed85384bf73e39248265374/fonts/NotoSans/hinted/ttf"
fetch "NotoSans-Regular.ttf"    "$NOTO/NotoSans-Regular.ttf"    "478c558ea716033cd60c03438f628dfa75694dcf6b5f6d505a2f05fd2b4f3823"
fetch "NotoSans-Bold.ttf"       "$NOTO/NotoSans-Bold.ttf"       "1df075a380fc7cb898acf64c1f7b3b4dd780de3caa860178bf929de35817a913"
fetch "NotoSans-Italic.ttf"     "$NOTO/NotoSans-Italic.ttf"     "467e3f89eeca4108bb8710a2b9e0cf2281ac56d5b0609211a83776d0505eecb5"
fetch "NotoSans-BoldItalic.ttf" "$NOTO/NotoSans-BoldItalic.ttf" "1b602a9d6353be42c91df097a4857b69fa2696f26703d7a33b54a15d87c2622c"

# --- Source Sans 3 3.052, SIL OFL 1.1 ------------------------------------
# Pin the reviewed upstream commit used by check-required-font-assets.sh.
# These bytes are identical to the 3.052R release tag.
SSP="https://raw.githubusercontent.com/adobe-fonts/source-sans/87b37a2daaed80fcb8e8ccb0085c4d72ddade12e/TTF"
fetch "SourceSans3-Regular.ttf" "$SSP/SourceSans3-Regular.ttf" "4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f"
fetch "SourceSans3-Bold.ttf"    "$SSP/SourceSans3-Bold.ttf"    "9214b9d95e4231c609802815c2646c98174e2102d0d37f88978a7f8e71006e6a"
fetch "SourceSans3-It.ttf"      "$SSP/SourceSans3-It.ttf"      "192afd78f0f54a3c69eaf02d43f4d9a821e9d6110e41d3d25d61a7385cd580e4"
fetch "SourceSans3-BoldIt.ttf"  "$SSP/SourceSans3-BoldIt.ttf"  "7978291fc1bf314db887e0366853b33c5cf2e964c7b95cfb9ce403a6ec46a842"

# --- JetBrains Mono 2.304, SIL OFL 1.1 -----------------------------------
# Commit of the v2.304 tag.
JBM="https://raw.githubusercontent.com/JetBrains/JetBrainsMono/cd5227bd1f61dff3bbd6c814ceaf7ffd95e947d9/fonts/ttf"
fetch "JetBrainsMono-Regular.ttf"    "$JBM/JetBrainsMono-Regular.ttf"    "a0bf60ef0f83c5ed4d7a75d45838548b1f6873372dfac88f71804491898d138f"
fetch "JetBrainsMono-Bold.ttf"       "$JBM/JetBrainsMono-Bold.ttf"       "5590990c82e097397517f275f430af4546e1c45cff408bde4255dad142479dcb"
fetch "JetBrainsMono-Italic.ttf"     "$JBM/JetBrainsMono-Italic.ttf"     "9d0a1f7a708e6af183f1193b7e81d40da294f5c67682c085d8401c60aac8ded4"
fetch "JetBrainsMono-BoldItalic.ttf" "$JBM/JetBrainsMono-BoldItalic.ttf" "4039d5ce0ed225bf9c8b2c8c6436290ae2f356b7e90d70fa666227238324aa3b"

# --- Roboto 3.016, SIL OFL 1.1 -------------------------------------------
# Roboto moved from Apache-2.0 to OFL-1.1, and google/fonts now ships it only
# as a variable font. The static hinted faces ship in this release archive.
use_archive \
	"https://github.com/googlefonts/roboto-3-classic/releases/download/v3.016/Roboto_v3.016.zip" \
	"1653dbe12f248da8fb0b9920db7b9496cd677ed3981154f6f15285c8bd4e334f"
extract "Roboto-Regular.ttf"    "hinted/static/Roboto-Regular.ttf"
extract "Roboto-Bold.ttf"       "hinted/static/Roboto-Bold.ttf"
extract "Roboto-Italic.ttf"     "hinted/static/Roboto-Italic.ttf"
extract "Roboto-BoldItalic.ttf" "hinted/static/Roboto-BoldItalic.ttf"

# --- Arimo 1.33 (Arial-metric-compatible), SIL OFL 1.1 -------------------
# Arimo moved from Apache-2.0 to OFL-1.1. google/fonts now ships it only as
# a variable font; the upstream project still publishes static faces.
ARIMO="https://raw.githubusercontent.com/googlefonts/arimo/4a6255f269916ae7ad3fc2706b0935e7621396b8/fonts/ttf"
fetch "Arimo-Regular.ttf"    "$ARIMO/Arimo-Regular.ttf"    "41b22bc8f0b51f932825d37bc55b5eb6ba67dfe599a626e4aff2b43b624f9f8c"
fetch "Arimo-Bold.ttf"       "$ARIMO/Arimo-Bold.ttf"       "d7a8b187cf8444d4cfee102e8eae9e3043682fd5106d5d33ed677fe268a0e2ba"
fetch "Arimo-Italic.ttf"     "$ARIMO/Arimo-Italic.ttf"     "ffdf690e4357a392e4edcc3df0b3adb0302920f75e54f2e6eec355afbe7edcba"
fetch "Arimo-BoldItalic.ttf" "$ARIMO/Arimo-BoldItalic.ttf" "77c967d49d4ef4a63c39fbcd1e27892102e503e35a20983cb058430c4f268b34"

# --- Cousine 1.241 (monospaced companion to Arimo), SIL OFL 1.1 ----------
# Cousine moved from Apache-2.0 to OFL-1.1 (google/fonts ofl/cousine).
GOOGLE_FONTS="https://raw.githubusercontent.com/google/fonts/e7db6ca603fd4862010383ff17ffd6cc4ffac1c0/ofl"
fetch "Cousine-Regular.ttf"    "$GOOGLE_FONTS/cousine/Cousine-Regular.ttf"    "1da22250675fc4c42fcf3a9736c44bc0570516105331443b663fd5cfbd1412fe"
fetch "Cousine-Bold.ttf"       "$GOOGLE_FONTS/cousine/Cousine-Bold.ttf"       "17c8a7245156d2253531c9e529474937b09d9f641c5ae7695c5e33f22822eef4"
fetch "Cousine-Italic.ttf"     "$GOOGLE_FONTS/cousine/Cousine-Italic.ttf"     "ea2a76ae3d0ece9cd59f0d30fdc08dd70e8f5f457beee5b0852a7b50c2286c7c"
fetch "Cousine-BoldItalic.ttf" "$GOOGLE_FONTS/cousine/Cousine-BoldItalic.ttf" "848e858726fee0ae27b754e4cd6a2755209bf1428a8c91f747696d58c33906c3"

# --- Ledger 1.003 (modern business serif), SIL OFL 1.1 -------------------
fetch "Ledger-Regular.ttf" "$GOOGLE_FONTS/ledger/Ledger-Regular.ttf" "0d9e4086208d5aee8813ae1185010c58fa2f07933fb5c1a1d0950f42bd976272"

if [ "$LIST" -eq 1 ]; then
	exit 0
fi

echo
echo "Fonts fetched: $OK_COUNT ok, $FAIL_COUNT failed."
if [ "$FAIL_COUNT" -gt 0 ]; then
	echo "Failed files:$FAILED_LIST" >&2
	echo "Check your network connection and re-run. If an upstream source has" >&2
	echo "moved, update scripts/fetch-fonts.sh and the license table in" >&2
	echo "$ASSETS/README.md together. The build still works with whatever" >&2
	echo "fonts are present (missing families are left out of the font list)." >&2
fi
echo "Rebuild with 'make build' to embed the fetched fonts."

# The default family is committed, so a failed run leaves every build working;
# still exit non-zero so the caller knows the optional catalog is incomplete.
if [ "$FAIL_COUNT" -gt 0 ]; then
	exit 1
fi
