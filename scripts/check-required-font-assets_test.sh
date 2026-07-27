#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Exercise the font gate in an isolated repository so both clean-checkout and
# corrupted-asset failures are proven without modifying the working tree.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-required-fonts.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT

fail() {
	echo "required-font-assets test: $*" >&2
	exit 1
}

assert_fails_with() {
	local expected="$1"
	shift
	local output
	if output="$("$@" 2>&1)"; then
		fail "expected command to fail with: $expected"
	fi
	printf '%s\n' "$output" | grep -Fq "$expected" ||
		fail "failure did not contain '$expected': $output"
}

mkdir -p "$FIXTURE/scripts" "$FIXTURE/internal/fonts/assets"
cp "$ROOT/scripts/check-required-font-assets.sh" "$FIXTURE/scripts/"
cp "$ROOT"/internal/fonts/assets/SourceSans3-*.ttf "$FIXTURE/internal/fonts/assets/"
git -C "$FIXTURE" init -q
git -C "$FIXTURE" add scripts/check-required-font-assets.sh internal/fonts/assets

(cd "$FIXTURE" && bash scripts/check-required-font-assets.sh >/dev/null)

git -C "$FIXTURE" rm --cached -q internal/fonts/assets/SourceSans3-Regular.ttf
assert_fails_with "must be tracked" \
	bash -c "cd '$FIXTURE' && bash scripts/check-required-font-assets.sh"
git -C "$FIXTURE" add internal/fonts/assets/SourceSans3-Regular.ttf

printf 'corruption' >>"$FIXTURE/internal/fonts/assets/SourceSans3-Bold.ttf"
assert_fails_with "checksum changed" \
	bash -c "cd '$FIXTURE' && bash scripts/check-required-font-assets.sh"

echo "required-font-assets tests passed."
