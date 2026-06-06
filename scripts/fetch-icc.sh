#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Refreshes the compact sRGB ICC profile used for PDF/A-3 OutputIntent.
#
# The profile is committed because it is a small build input required by
# go:embed in clean checkouts. It is sourced from Compact ICC Profiles,
# whose profile collection is released under CC0-1.0.
#
# The chosen profile is sRGB-v2-magic.icc, a compact V2 sRGB profile
# intended for embedding.
#
# Usage:
#   scripts/fetch-icc.sh          # fetch the profile
#   scripts/fetch-icc.sh --list   # show the source URL
#   scripts/fetch-icc.sh --force  # redownload even if present
#
# Idempotent unless --force is used.

set -eu
cd "$(dirname "$0")/.."

ASSETS="internal/pdfmeta/assets"
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

# Stable compact sRGB profile suitable for PDF/A-3 OutputIntent.
URL="https://raw.githubusercontent.com/saucecontrol/Compact-ICC-Profiles/master/profiles/sRGB-v2-magic.icc"
DEST="$ASSETS/sRGB.icc"

if [ "$LIST" -eq 1 ]; then
	printf 'sRGB.icc <- %s\n' "$URL"
	exit 0
fi

if [ -f "$DEST" ] && [ "$FORCE" -eq 0 ]; then
	echo "skip   sRGB.icc (exists)"
	exit 0
fi

echo "fetch  sRGB.icc"

if command -v curl >/dev/null 2>&1; then
	curl -fsSL "$URL" -o "$DEST"
elif command -v wget >/dev/null 2>&1; then
	wget -q "$URL" -O "$DEST"
else
	echo "ERROR: neither curl nor wget is installed" >&2
	exit 1
fi

if [ -s "$DEST" ]; then
	# Basic sanity: ICC profiles start with "acsp" at offset 36
	if head -c 40 "$DEST" | tail -c 4 | grep -q "acsp"; then
		echo "ok     sRGB.icc"
		echo "Rebuild with 'make build' to embed the ICC profile for PDF/A-3."
		exit 0
	else
		echo "FAIL   sRGB.icc (not a valid ICC profile)"
		rm -f "$DEST"
		exit 1
	fi
else
	echo "FAIL   sRGB.icc (download error or empty file)"
	rm -f "$DEST"
	exit 1
fi
