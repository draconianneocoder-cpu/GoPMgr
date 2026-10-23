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
	VERIFIED+=("$path")
}

VERIFIED=()

ASSETS="internal/fonts/assets"
WEB="frontend/public/fonts/ibm-plex"

# IBM Plex TrueType faces from IBM's release tags in github.com/IBM/plex:
# Sans @ibm/plex-sans@1.1.0 (commit 1da12f02587b630c07e92692d21492d722f53614),
# Serif @ibm/plex-serif@2.0.0 (434af578549afcfdabd281f386e0ff7314fd20b0),
# Mono @ibm/plex-mono@2.5.0 (2f9ba1b25957d958db71a849e85d72e3ecfb845a), and
# Sans Condensed @ibm/plex-sans-condensed@2.0.0
# (bb3ab6404e1881ea286f8742dc839e09057db6dd), and the unhinted TrueType files
# of Sans JP @ibm/plex-sans-jp@3.0.0 (89cba80dad75561262e758f4b6ddd474c5119796),
# Sans KR @ibm/plex-sans-kr@1.1.0 (1da12f02587b630c07e92692d21492d722f53614),
# and Sans SC @ibm/plex-sans-sc@1.1.0 (222657a016f31e6d913ede88520de7dec84dc81b).
# The WOFF2 faces are from the
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
verify_asset "$ASSETS/IBMPlexSansJP-Regular.ttf" "0168493ae143d16e4c0d374c090b1ee5eab9ee475536d2e429368dbcfc4a6abe"
verify_asset "$ASSETS/IBMPlexSansJP-SemiBold.ttf" "3ae17af73062926b013afdec8949923a8adfd4ec8c4f689876c0a359e63e4ceb"
verify_asset "$ASSETS/IBMPlexSansKR-Regular.ttf" "9b9a2fd392af19b1bd6535fb506e4f4a80bfc23f5421b1772983502394ade125"
verify_asset "$ASSETS/IBMPlexSansKR-SemiBold.ttf" "c957e30a0744bd98d1f7e8c19ee7fcd3740ab59a92e4d9b8168a82eda91d25ea"
verify_asset "$ASSETS/IBMPlexSansSC-Regular.ttf" "0b8b5c398c3198ec8932f3d3eb5eea0b3e7ff4f6cb5b84fcde180ad890094d5e"
verify_asset "$ASSETS/IBMPlexSansSC-SemiBold.ttf" "a785ab071ae8d75865c8aa9adc74fe6e5af67a9ba7cff76ca533dafba8cb7b2d"

verify_asset "$WEB/IBMPlexSans-Regular.woff2" "ba711a3085ff9f27440b6b9c4550cfc47c97bf36591d5da958b975bb3add8c1a"
verify_asset "$WEB/IBMPlexSans-Italic.woff2" "13284fab1821ba6e3652c1580fcf2bbfd8c9309520c69b3d1224dab40b37c597"
verify_asset "$WEB/IBMPlexSans-Medium.woff2" "5660f8a658f8bb50dbc005232f885eadffd2bc1c235c4f6fbb63469d1f9cde6d"
verify_asset "$WEB/IBMPlexSans-SemiBold.woff2" "f78048030eab62e860efa39a0df79e2e5581bf122eb95b9bc42c0b8a4988d205"
verify_asset "$WEB/IBMPlexSans-Bold.woff2" "fa7130d854a660b39a7fc9e6e0f2dc23dba5f1346e2adea3e1fe37b6d884133d"
verify_asset "$WEB/IBMPlexMono-Regular.woff2" "ba204497f16b6d334cee9d1e963a831b73e3a56e1d6300a8489d18df7214b350"
verify_asset "$WEB/IBMPlexMono-Medium.woff2" "33faf307fa6031fb4062276d7320a6d632de890cbb347576fd80cfa01077bc25"
verify_asset "$WEB/IBMPlexMono-SemiBold.woff2" "6a825b4824c01cbb401e829e5a066a1818411bcb3538b5a5792c5ca9b82343c3"
verify_asset "$WEB/IBMPlexMono-Bold.woff2" "ea576f38d05cc44cca48c45314984beb8cc1d2b886f58e1dce99f15dc344eb1d"
# The app's Arabic, Hebrew, Devanagari, and Thai faces, from npm packages
# @ibm/plex-sans-arabic, -hebrew, -devanagari, and -thai 1.1.0 (tag commit
# 1da12f02587b630c07e92692d21492d722f53614), each equivalent to its TTF there.
verify_asset "$WEB/IBMPlexSansArabic-Regular.woff2" "74112e1b7bf2e2c547989a4e947a60cba6cd1f9156e0065670134636e717199d"
verify_asset "$WEB/IBMPlexSansArabic-Medium.woff2" "c72f6986f50ded745d06e71e508096209c52a55a6b3a81029954e33c52ba1dd2"
verify_asset "$WEB/IBMPlexSansArabic-SemiBold.woff2" "869b2acd0a9660607a2025f0fe1c640110bf23d33417509328f07722922b8063"
verify_asset "$WEB/IBMPlexSansArabic-Bold.woff2" "db6650a05969c144ab5ab9bb6c3beca6ca4dbb3537bb5f2311c878dd293875b4"
verify_asset "$WEB/IBMPlexSansHebrew-Regular.woff2" "9e56b8035b123978d479e50c6fcccaebc462888dc49902d8572f7869210a077e"
verify_asset "$WEB/IBMPlexSansHebrew-Medium.woff2" "91911819babe8f811aea1c531401443c7886fd473bef02282880693600dd58f5"
verify_asset "$WEB/IBMPlexSansHebrew-SemiBold.woff2" "22792f09b91eeac408bce0df1aea7849a103e993a9ee31c707b8b9b6396dddf4"
verify_asset "$WEB/IBMPlexSansHebrew-Bold.woff2" "4a2181ea6604ad78e1e46806b8f83a31c072d2495d8b274ea41fdad212727cd4"
verify_asset "$WEB/IBMPlexSansDevanagari-Regular.woff2" "9414954cbb343be6c8fa97b716a93eb7cd5501043356cdbb704e233db5f8ca85"
verify_asset "$WEB/IBMPlexSansDevanagari-Medium.woff2" "2b39f69997b1de44cfef6f6f6ef899a4f434a84af1dbc21886558e0f4d8d29b9"
verify_asset "$WEB/IBMPlexSansDevanagari-SemiBold.woff2" "adbd61f32837a5b83b51f5a201305e4163f73505bf02262b3d30965fa96f948a"
verify_asset "$WEB/IBMPlexSansDevanagari-Bold.woff2" "514fe29e79c3dfc0aad4bfb12ede03975acfc26349c9250eee7f77b126e5c3c0"
verify_asset "$WEB/IBMPlexSansThai-Regular.woff2" "0350508969dad82ffd2b7608d8299454bf19ec54e464a16a66e5a9733e83654d"
verify_asset "$WEB/IBMPlexSansThai-Medium.woff2" "8249537de3768f46ce7b04e9e832c47f3ffa84900bdac2b18968d431d8814c61"
verify_asset "$WEB/IBMPlexSansThai-SemiBold.woff2" "83f9d86c099e0006077854cac1cf6f9d3177fb0c4f356254a7d56d047c097e52"
verify_asset "$WEB/IBMPlexSansThai-Bold.woff2" "c65640f017e19c668aa927ec7e539d4306a8c77fec717ea979b12a1bb1c40121"

# These hashes bind the tracked files to the Source Sans 3 version 3.052 TTFs
# at Adobe commit 87b37a2daaed80fcb8e8ccb0085c4d72ddade12e (byte-identical to
# the 3.052R release tag). Updating them requires reviewing the upstream font
# change and its OFL-1.1 licensing before committing bytes.
verify_asset "$ASSETS/SourceSans3-Regular.ttf" "4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f"
verify_asset "$ASSETS/SourceSans3-Bold.ttf" "9214b9d95e4231c609802815c2646c98174e2102d0d37f88978a7f8e71006e6a"
verify_asset "$ASSETS/SourceSans3-It.ttf" "192afd78f0f54a3c69eaf02d43f4d9a821e9d6110e41d3d25d61a7385cd580e4"
verify_asset "$ASSETS/SourceSans3-BoldIt.ttf" "7978291fc1bf314db887e0366853b33c5cf2e964c7b95cfb9ce403a6ec46a842"

# Every tracked font file must be pinned above: a font committed without a
# reviewed hash would otherwise ship unchecked.
while IFS= read -r tracked; do
	pinned=0
	for path in "${VERIFIED[@]}"; do
		if [ "$path" = "$tracked" ]; then
			pinned=1
			break
		fi
	done
	[ "$pinned" = 1 ] || fail "$tracked is tracked but has no pinned SHA-256 in this script."
done < <(git ls-files -- "$ASSETS/*.ttf" "$WEB/*.woff2")

echo "required-font-assets: ${#VERIFIED[@]} tracked IBM Plex and Source Sans 3 fonts verified."
