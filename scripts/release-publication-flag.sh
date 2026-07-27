#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Translate a tag that already passed check-release-tag.sh into the optional
# GitHub CLI classification flag. GitHub does not infer prerelease state from
# a SemVer suffix, so omitting this flag would present an alpha or RC as GA.

set -euo pipefail

tag="${1:-}"
if [ -z "$tag" ]; then
	echo "release-publication-flag: a validated version tag is required." >&2
	exit 1
fi

case "$tag" in
	v*-*)
		printf '%s\n' "--prerelease"
		;;
	v*)
		# A GA tag intentionally emits no optional gh argument.
		;;
	*)
		echo "release-publication-flag: expected a v-prefixed version tag, got $tag." >&2
		exit 1
		;;
esac
