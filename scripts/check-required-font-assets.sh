#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Verify the minimal embedded font family required by strict PDF/A exports.
# Optional catalog families remain fetch-on-demand, but Source Sans 3 must be
# present in every clean checkout so release output never falls back to the
# non-embeddable PDF core fonts.

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

verify_font() {
	local name="$1"
	local expected_sha256="$2"
	local path="internal/fonts/assets/$name"
	local signature
	local actual_sha256

	[ -s "$path" ] || fail "$path is missing or empty."
	git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 ||
		fail "$path must be tracked; ignored local downloads do not reach clean release runners."

	# go-pdf/fpdf's UTF-8 font loader requires sfnt TrueType outlines. Reject
	# HTML error pages and OpenType/CFF inputs before they enter go:embed.
	signature="$(head -c 4 "$path" | od -An -tx1 | tr -d ' \n')"
	case "$signature" in
		00010000 | 74727565) ;;
		*) fail "$path does not have a supported TrueType signature." ;;
	esac

	actual_sha256="$(sha256_file "$path")"
	[ "$actual_sha256" = "$expected_sha256" ] ||
		fail "$path checksum changed (expected $expected_sha256, got $actual_sha256)."
}

# These hashes bind the tracked files to Adobe Source Sans release commit
# 87b37a2daaed80fcb8e8ccb0085c4d72ddade12e. Updating them requires reviewing
# the upstream font change and its OFL-1.1 licensing before committing bytes.
verify_font "SourceSans3-Regular.ttf" "4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f"
verify_font "SourceSans3-Bold.ttf" "9214b9d95e4231c609802815c2646c98174e2102d0d37f88978a7f8e71006e6a"
verify_font "SourceSans3-It.ttf" "192afd78f0f54a3c69eaf02d43f4d9a821e9d6110e41d3d25d61a7385cd580e4"
verify_font "SourceSans3-BoldIt.ttf" "7978291fc1bf314db887e0366853b33c5cf2e964c7b95cfb9ce403a6ec46a842"

echo "required-font-assets: tracked Source Sans 3 PDF/A baseline verified."
