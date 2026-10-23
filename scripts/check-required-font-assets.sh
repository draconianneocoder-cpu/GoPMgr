#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Verify the font files every build bundles. The IBM Plex TrueType faces are
# the PDF/A baseline and the default document font, and the IBM Plex WOFF2
# faces set the app interface; Source Sans 3 remains a bundled choice. All of
# them must be present in every clean checkout so release output never falls
# back to the non-embeddable PDF core fonts or to system fonts. Optional
# catalog families remain fetch-on-demand.

set -euo pipefail
cd "$(dirname "$0")/.."

fail() {
	echo "required-font-assets: $*" >&2
	exit 1
}

sha256_file() {
	local path="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$path" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$path" | awk '{print $1}'
	else
		fail "sha256sum or shasum is required to verify bundled font provenance."
	fi
}

# verify_asset <repo-relative path> <expected sha256>
verify_asset() {
	local path="$1"
	local expected_sha256="$2"
	local signature
	local actual_sha256

	[ -s "$path" ] || fail "$path is missing or empty."
	git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 ||
		fail "$path must be tracked; ignored local downloads do not reach clean release runners."

	# go-pdf/fpdf's UTF-8 font loader requires sfnt TrueType outlines, and the
	# app's @font-face rules load WOFF2. Reject HTML error pages, OpenType/CFF
	# inputs, and mislabeled files before they enter go:embed or the bundle.
	signature="$(head -c 4 "$path" | od -An -tx1 | tr -d ' \n')"
	case "$path:$signature" in
		*.ttf:00010000 | *.ttf:74727565 | *.woff2:774f4632) ;;
		*) fail "$path does not have a supported font signature for its type." ;;
	esac

	actual_sha256="$(sha256_file "$path")"
	[ "$actual_sha256" = "$expected_sha256" ] ||
		fail "$path checksum changed (expected $expected_sha256, got $actual_sha256)."
}

ASSETS="internal/fonts/assets"
WEB="frontend/public/fonts/ibm-plex"

# IBM Plex TrueType faces from IBM's release tags in github.com/IBM/plex:
# Sans @ibm/plex-sans@1.1.0 (commit 1da12f02587b630c07e92692d21492d722f53614),
# Serif @ibm/plex-serif@2.0.0 (434af578549afcfdabd281f386e0ff7314fd20b0),
# Mono @ibm/plex-mono@2.5.0 (2f9ba1b25957d958db71a849e85d72e3ecfb845a), and
# Sans Condensed @ibm/plex-sans-condensed@2.0.0
# (bb3ab6404e1881ea286f8742dc839e09057db6dd). The WOFF2 faces are from the
# matching npm packages and have the same version, glyphs, character map,
# advance widths, and outlines as these TTFs. Updating any hash requires
# reviewing the upstream change and its OFL-1.1 licensing, and updating the
# TTF and WOFF2 faces together (docs/design/typography-ibm-plex.md).
verify_asset "$ASSETS/IBMPlexSans-Regular.ttf" "975dcda37d80f038dcd143c22e33ca2d97a0cc5a929aace1c749153b0fe1afa5"
verify_asset "$ASSETS/IBMPlexSans-SemiBold.ttf" "a20caf8286023a6a7a85e40b1d2a4ae9fc3e3b1f9eda8f4c542dd4986af67bb1"
verify_asset "$ASSETS/IBMPlexSans-Italic.ttf" "a9c6ef9942c49e49d11e11a6dacc0b3a087978757e9b22a06b8ac22a6400fb15"
verify_asset "$ASSETS/IBMPlexSans-SemiBoldItalic.ttf" "147106b619423a82b40f6d64bace074ad5b786e59f48023bda69d63b1755fb82"
verify_asset "$ASSETS/IBMPlexSerif-Regular.ttf" "6c490ccd38e856c674e0d163f3f40742a75058b7b935998dbfbc1745ba54e474"
verify_asset "$ASSETS/IBMPlexSerif-SemiBold.ttf" "857edeaaff0f3913f4dfaf6bc0e9c89ef73e475429cf63d6a3cb5d1022a213d5"
verify_asset "$ASSETS/IBMPlexSerif-Italic.ttf" "d28aa63cf618439712d6b459c5649af1d6d789cb911994f2357f1ce44128f163"
verify_asset "$ASSETS/IBMPlexSerif-SemiBoldItalic.ttf" "c1bf6e7e5d3fd740916184e171703db25729fefd4d2423ae1b79a6afcf1a34ef"
verify_asset "$ASSETS/IBMPlexMono-Regular.ttf" "7c6fbddca4b700be918f5f6183d9bd4464fa427fe435f0b480d77fe2bb8c5a43"
verify_asset "$ASSETS/IBMPlexMono-SemiBold.ttf" "f04d7c488ddf7d1fa99f2574efc3406ea4cbe17bb1af3a1ab960f84d0c96a172"
verify_asset "$ASSETS/IBMPlexMono-Italic.ttf" "a55368deb953f594ac18336a1029009ed820e2d9abbb57ad601f61e22bec072b"
verify_asset "$ASSETS/IBMPlexMono-SemiBoldItalic.ttf" "d2039ddfa9236d642fafae5aa46446c54eb7961b3771a1ca3e9bba960172b9bb"
verify_asset "$ASSETS/IBMPlexSansCondensed-Regular.ttf" "87aaee6ba9d2f80b8a3c1f6669e8c3d478b28b482f1d804917b3894694ccc12b"
verify_asset "$ASSETS/IBMPlexSansCondensed-SemiBold.ttf" "acc2b1e3b9597ee13b08b767f845bd038fe120cbee806859af7c01b60fe6e9e6"
verify_asset "$ASSETS/IBMPlexSansCondensed-Italic.ttf" "368d896f1cd1dc9f9b926135bd5c3154aa6f3dcd53f2b330bce487968a3a3e4a"
verify_asset "$ASSETS/IBMPlexSansCondensed-SemiBoldItalic.ttf" "d6dbb283c007854cd5fd56b86f5c9da8fcac497e50aa8e029258de21599ab8f6"

verify_asset "$WEB/IBMPlexSans-Regular.woff2" "ba711a3085ff9f27440b6b9c4550cfc47c97bf36591d5da958b975bb3add8c1a"
verify_asset "$WEB/IBMPlexSans-Italic.woff2" "13284fab1821ba6e3652c1580fcf2bbfd8c9309520c69b3d1224dab40b37c597"
verify_asset "$WEB/IBMPlexSans-Medium.woff2" "5660f8a658f8bb50dbc005232f885eadffd2bc1c235c4f6fbb63469d1f9cde6d"
verify_asset "$WEB/IBMPlexSans-SemiBold.woff2" "f78048030eab62e860efa39a0df79e2e5581bf122eb95b9bc42c0b8a4988d205"
verify_asset "$WEB/IBMPlexSans-Bold.woff2" "fa7130d854a660b39a7fc9e6e0f2dc23dba5f1346e2adea3e1fe37b6d884133d"
verify_asset "$WEB/IBMPlexMono-Regular.woff2" "ba204497f16b6d334cee9d1e963a831b73e3a56e1d6300a8489d18df7214b350"
verify_asset "$WEB/IBMPlexMono-Medium.woff2" "33faf307fa6031fb4062276d7320a6d632de890cbb347576fd80cfa01077bc25"
verify_asset "$WEB/IBMPlexMono-SemiBold.woff2" "6a825b4824c01cbb401e829e5a066a1818411bcb3538b5a5792c5ca9b82343c3"
verify_asset "$WEB/IBMPlexMono-Bold.woff2" "ea576f38d05cc44cca48c45314984beb8cc1d2b886f58e1dce99f15dc344eb1d"

# These hashes bind the tracked files to the Source Sans 3 version 3.052 TTFs
# at Adobe commit 87b37a2daaed80fcb8e8ccb0085c4d72ddade12e (byte-identical to
# the 3.052R release tag). Updating them requires reviewing the upstream font
# change and its OFL-1.1 licensing before committing bytes.
verify_asset "$ASSETS/SourceSans3-Regular.ttf" "4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f"
verify_asset "$ASSETS/SourceSans3-Bold.ttf" "9214b9d95e4231c609802815c2646c98174e2102d0d37f88978a7f8e71006e6a"
verify_asset "$ASSETS/SourceSans3-It.ttf" "192afd78f0f54a3c69eaf02d43f4d9a821e9d6110e41d3d25d61a7385cd580e4"
verify_asset "$ASSETS/SourceSans3-BoldIt.ttf" "7978291fc1bf314db887e0366853b33c5cf2e964c7b95cfb9ce403a6ec46a842"

echo "required-font-assets: tracked IBM Plex and Source Sans 3 fonts verified."
