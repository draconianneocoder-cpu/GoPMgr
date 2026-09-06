#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Derive the signed-update channel from a tag that has already passed
# check-release-tag.sh. GA builds use stable; prereleases use their first
# SemVer identifier so alpha, alpha.1, and alpha.2 share one channel.

set -euo pipefail

tag="${1:-}"
if [[ ! "$tag" =~ ^v[^-]+(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$ ]]; then
	echo "release-update-channel: a validated v-prefixed version tag is required." >&2
	exit 1
fi

if [[ "$tag" != *-* ]]; then
	printf '%s\n' "stable"
	exit 0
fi

prerelease="${tag#*-}"
printf '%s\n' "${prerelease%%.*}"
