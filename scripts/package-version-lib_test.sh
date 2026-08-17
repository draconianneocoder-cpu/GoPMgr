#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
. "$ROOT/scripts/package-version-lib.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

work="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-package-version-lib-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT

repo="$work/repo"
mkdir -p "$repo"
(
	cd "$repo"
	git init -q
	git config user.email "test@example.invalid"
	git config user.name "test"
	echo one > file.txt
	git add file.txt
	git commit -q -m "first commit"
	git tag v1.2.3
)

# Case 1: explicit VERSION always wins, even when git state would produce
# something different. This is the property CI depends on for real
# releases, so a regression here breaks releases, not just local builds.
out="$(cd "$repo" && VERSION="9.9.9-explicit" release_version)"
[ "$out" = "9.9.9-explicit" ] || fail "explicit VERSION was not honored (got '$out')"

# Case 2: VERSION unset, HEAD exactly on a tag -> the bare tag (no 'v').
out="$(cd "$repo" && VERSION= release_version)"
[ "$out" = "1.2.3" ] || fail "exact-tag case did not return the bare tag (got '$out')"

# Case 3: VERSION unset, HEAD past the tag -> must be DISTINGUISHABLE from
# case 2's output, not a silent repeat of the stale tag name. This is the
# case --abbrev=0 collapsed onto case 2; reintroducing --abbrev=0 makes
# this assertion fail.
(
	cd "$repo"
	echo two > file.txt
	git add file.txt
	git commit -q -m "second commit"
)
out="$(cd "$repo" && VERSION= release_version)"
case "$out" in
	"1.2.3")
		fail "post-tag commit produced the bare tag name; distance/sha info was lost (got '$out')"
		;;
	1.2.3-*) ;;
	*)
		fail "post-tag version did not start with the nearest tag (got '$out')"
		;;
esac
[ "$out" != "1.2.3" ] || fail "post-tag version must differ from the exact-tag version"

# Case 3b: same as case 3, but with VERSION genuinely unset (not merely
# empty) — the actual invocation shape package-macos.sh/
# package-macos-installer.sh use under `set -u`. VERSION= and unset VERSION
# both satisfy `${VERSION:-}`, but only this case proves the real call path.
out="$(cd "$repo" && env -u VERSION bash -c '. "'"$ROOT"'/scripts/package-version-lib.sh"; set -euo pipefail; release_version')"
case "$out" in
	"1.2.3") fail "unset-VERSION post-tag case returned the bare tag (got '$out')" ;;
	1.2.3-*) ;;
	*) fail "unset-VERSION post-tag version did not start with the nearest tag (got '$out')" ;;
esac

# Case 4: VERSION unset, no tags reachable at all -> "0.0.0", not a
# plausible-looking-but-wrong stand-in.
notags="$work/no-tags-repo"
mkdir -p "$notags"
(
	cd "$notags"
	git init -q
	git config user.email "test@example.invalid"
	git config user.name "test"
	echo one > file.txt
	git add file.txt
	git commit -q -m "only commit, no tags"
)
out="$(cd "$notags" && VERSION= release_version)"
[ "$out" = "0.0.0" ] || fail "no-tags case did not fall back to 0.0.0 (got '$out')"

echo "package-version-lib tests passed."
