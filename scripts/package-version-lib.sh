#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Shared version derivation for scripts/package-macos.sh and
# scripts/package-macos-installer.sh, so a single build's two macOS
# artifacts always carry the same filename version.
#
# Precedence:
#   1. An explicit VERSION env var. The release workflow always sets this
#      from the tag (VERSION="${GITHUB_REF_NAME#v}"), so a real release
#      build never reaches the fallback below.
#   2. `git describe --tags` against the nearest reachable tag, WITHOUT
#      --abbrev=0. Dropping --abbrev=0 matters: with it, a checkout that is
#      several commits past its nearest tag silently reuses that tag's bare
#      name, so two different trees can produce identically-named artifacts.
#      Without it, git appends "-<N>-g<sha>" whenever HEAD isn't exactly on
#      the tag, making an untagged local build's filename honestly
#      distinguishable from the tagged release it's built near.
#   3. "0.0.0" if the checkout has no tags at all (e.g. a shallow clone).
#      Deliberately not wails.json's productVersion here: that value is a
#      clean major.minor.patch with no build/commit information, and using
#      it as a silent fallback would produce a plausible-looking release
#      version for a build that isn't one.
#
# Keep this file free of side effects so package-version-lib_test.sh can
# source it directly.
release_version() {
	if [ -n "${VERSION:-}" ]; then
		printf '%s\n' "$VERSION"
		return 0
	fi

	local described
	described="$(git describe --tags 2>/dev/null | sed 's/^v//')"
	if [ -n "$described" ]; then
		printf '%s\n' "$described"
		return 0
	fi

	printf '%s\n' "0.0.0"
}
