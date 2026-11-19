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
CHANNEL_HELPER="$ROOT/scripts/release-update-channel.sh"

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

if [ ! -f "$WORKFLOW" ] || [ ! -f "$HELPER" ] || [ ! -f "$CHANNEL_HELPER" ]; then
	echo "release-publication-mode: workflow and classification helpers are required." >&2
	exit 1
fi

# shellcheck disable=SC2016 # Match literal source text; expansion would change the guard.
require_literal "$WORKFLOW" \
	'publication_flag="$(bash scripts/release-publication-flag.sh "$GITHUB_REF_NAME")"' \
	"publish job must classify the validated Git tag."
# shellcheck disable=SC2016 # Match literal source text; expansion would change the guard.
require_literal "$WORKFLOW" 'publication_args+=("$publication_flag")' \
	"publish job must preserve the helper output as one gh argument."
# shellcheck disable=SC2016 # Match literal source text; expansion would change the guard.
require_literal "$WORKFLOW" '"${publication_args[@]}"' \
	"gh release create must consume the optional publication argument."
require_literal "$WORKFLOW" '--draft' \
	"release artifacts must remain private until their digests and claims are reviewed."
require_literal "$HELPER" 'printf '\''%s\n'\'' "--prerelease"' \
	"classification helper must mark SemVer suffixes as prereleases."
# shellcheck disable=SC2016 # Match literal source text; expansion would change the guard.
channel_line='channel="$(bash scripts/release-update-channel.sh "$GITHUB_REF_NAME")"'
channel_calls="$(grep -Fc "$channel_line" "$WORKFLOW" || true)"
ldflags_script="$ROOT/scripts/release-update-ldflags.sh"
script_calls=0
if [ -f "$ldflags_script" ]; then
	script_calls="$(grep -Fc 'channel="$(bash "$root/release-update-channel.sh" "$tag")"' "$ldflags_script" || true)"
fi
publish_block="$(awk '
	/name: Create \/ update GitHub Release/ { grab = 1 }
	grab { print }
	grab && /gh release create / { exit }
' "$WORKFLOW")"
if [ "$channel_calls" -ne 1 ] || [ "$script_calls" -ne 1 ]; then
	echo "release-publication-mode: the publication job and the release ldflags script must both derive the signed-update channel from the validated tag." >&2
	fail=1
elif ! printf '%s\n' "$publish_block" | awk -v helper="$channel_line" -v use='-channel "$channel"' '
	index($0, helper) { seen = 1; next }
	seen && index($0, "channel=") { overwritten = 1 }
	seen && index($0, use) { used = 1; exit }
	END { exit !(seen && used && !overwritten) }
'; then
	echo "release-publication-mode: publication job must derive the signed-update channel from the validated tag." >&2
	fail=1
fi

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

echo "release-publication-mode: GitHub and signed-update tag classification wiring is consistent."
