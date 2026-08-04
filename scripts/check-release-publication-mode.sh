#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Keep the tag-classification helper connected to GitHub release publication.
# This is a source contract; native installer execution is verified separately.

set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
WORKFLOW="$ROOT/.github/workflows/release.yml"
HELPER="$ROOT/scripts/release-publication-flag.sh"

fail=0
require_literal() {
	local file="$1"
	local literal="$2"
	local message="$3"
	if ! grep -Fq -- "$literal" "$file"; then
		echo "release-publication-mode: $message" >&2
		fail=1
	fi
}

if [ ! -f "$WORKFLOW" ] || [ ! -f "$HELPER" ]; then
	echo "release-publication-mode: workflow and classification helper are required." >&2
	exit 1
fi

require_literal "$WORKFLOW" \
	'publication_flag="$(bash scripts/release-publication-flag.sh "$GITHUB_REF_NAME")"' \
	"publish job must classify the validated Git tag."
require_literal "$WORKFLOW" 'publication_args+=("$publication_flag")' \
	"publish job must preserve the helper output as one gh argument."
require_literal "$WORKFLOW" '"${publication_args[@]}"' \
	"gh release create must consume the optional publication argument."
require_literal "$HELPER" 'printf '\''%s\n'\'' "--prerelease"' \
	"classification helper must mark SemVer suffixes as prereleases."

classify_line="$(awk '/publication_flag=.*release-publication-flag/ { print NR; exit }' "$WORKFLOW")"
publish_line="$(awk '/gh release create/ { print NR; exit }' "$WORKFLOW")"
if [ -n "$classify_line" ] && [ -n "$publish_line" ] &&
	[ "$classify_line" -ge "$publish_line" ]; then
	echo "release-publication-mode: tag classification must precede release creation." >&2
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit "$fail"
fi

echo "release-publication-mode: prerelease and GA publication wiring is consistent."
