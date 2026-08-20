#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Verify that a publication tag belongs to the version embedded in GoPMgr.
# GA uses v<version>; prereleases may append SemVer prerelease identifiers.
# Build metadata is intentionally excluded because package filenames and the
# GitHub Release title should have one canonical version string.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "release-tag: $*" >&2
	exit 1
}

TAG="${GOPMGR_RELEASE_TAG:-}"
if [ -z "$TAG" ]; then
	fail "GOPMGR_RELEASE_TAG is required; use v<product-version> or a matching SemVer prerelease."
fi

APP_VERSION="$(grep -oE 'Version *= *"[^"]+"' internal/cli/parser.go | head -1 | sed -E 's/.*"([^"]+)".*/\1/' || true)"
WAILS_VERSION="$(grep -oE '"productVersion" *: *"[^"]+"' wails.json | sed -E 's/.*"([^"]+)"$/\1/' || true)"

if [ -z "$APP_VERSION" ] || [ -z "$WAILS_VERSION" ]; then
	fail "could not read the application and Wails product versions."
fi
if [ "$APP_VERSION" != "$WAILS_VERSION" ]; then
	fail "version mismatch: CLI is $APP_VERSION while wails.json is $WAILS_VERSION."
fi
if [[ ! "$APP_VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
	fail "product version $APP_VERSION is not a clean major.minor.patch SemVer."
fi

case "$TAG" in
"v$APP_VERSION")
	echo "Release tag verified: $TAG (GA for product $APP_VERSION)."
	exit 0
	;;
"v$APP_VERSION"-*)
	prerelease="${TAG#"v$APP_VERSION"-}"
	;;
*)
	fail "tag $TAG does not match product version $APP_VERSION."
	;;
esac

# SemVer prerelease identifiers are dot-separated ASCII alphanumerics/hyphens.
# Reject empty identifiers and numeric leading zeroes so artifact versions do
# not depend on package-manager-specific normalization.
if [[ ! "$prerelease" =~ ^[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*$ ]]; then
	fail "tag $TAG has an invalid SemVer prerelease suffix."
fi

IFS='.' read -r -a identifiers <<<"$prerelease"
for identifier in "${identifiers[@]}"; do
	if [[ "$identifier" =~ ^[0-9]+$ ]] && [ "${#identifier}" -gt 1 ] && [[ "$identifier" == 0* ]]; then
		fail "tag $TAG has a numeric prerelease identifier with a leading zero."
	fi
done

echo "Release tag verified: $TAG (prerelease for product $APP_VERSION)."
