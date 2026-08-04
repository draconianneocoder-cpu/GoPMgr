#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Prevent draft prerelease material from being presented as published history.
# Concrete RC identifiers become valid only after publication evidence exists;
# until then, documentation must use symbolic SemVer prerelease placeholders.

set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"
PUBLISHED_TAGS="docs/published-release-tags.txt"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	echo "release-reference-truth: $ROOT is not a Git worktree." >&2
	exit 1
fi
if [ ! -f "$PUBLISHED_TAGS" ]; then
	echo "release-reference-truth: missing $PUBLISHED_TAGS." >&2
	exit 1
fi

fail=0
candidate_matches="$(
	git grep -nE 'v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+' -- \
		. ':(exclude)scripts/check-release-reference-truth.sh' || true
)"
candidate_tags="$(printf '%s\n' "$candidate_matches" |
	rg -o 'v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+' | sort -u || true)"

is_published() {
	local tag="$1"
	rg -Fxq "$tag" "$PUBLISHED_TAGS"
}

while IFS= read -r tag; do
	if [ -n "$tag" ] && ! is_published "$tag"; then
		echo "release-reference-truth: concrete unpublished release-candidate reference found: $tag" >&2
		fail=1
	fi
done <<<"$candidate_tags"

# git ls-files retains an unstaged deletion in the index. Check filesystem
# existence as well so maintainers can verify a cleanup before staging it.
while IFS= read -r note; do
	if [ -z "$note" ] || [ ! -e "$note" ]; then
		continue
	fi
	tag="$(basename "$note" .md)"
	if ! is_published "$tag"; then
		echo "release-reference-truth: unpublished candidate note is tracked: $note" >&2
		fail=1
	fi
done < <(git ls-files 'docs/release-notes/*-rc*.md')

if [ "$fail" -ne 0 ]; then
	exit "$fail"
fi

echo "release-reference-truth: no concrete unpublished release-candidate claims remain."
