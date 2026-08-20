#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-release-tag.sh"
VERSION="$(grep -oE 'Version *= *"[^"]+"' "$ROOT/internal/cli/parser.go" | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

expect_pass() {
	local tag="$1"
	if ! GOPMGR_RELEASE_TAG="$tag" bash "$CHECK" >/dev/null 2>&1; then
		fail "expected release tag to pass: $tag"
	fi
}

expect_fail() {
	local tag="$1"
	if GOPMGR_RELEASE_TAG="$tag" bash "$CHECK" >/dev/null 2>&1; then
		fail "expected release tag to fail: $tag"
	fi
}

expect_pass "v$VERSION"
expect_pass "v$VERSION-preview.1"
expect_pass "v$VERSION-preview-feature.2"

expect_fail "$VERSION"
expect_fail "v${VERSION}.1"
expect_fail "v$VERSION-"
expect_fail "v$VERSION-preview..1"
expect_fail "v$VERSION-preview.01"
expect_fail "v$VERSION+build.1"

if env -u GOPMGR_RELEASE_TAG bash "$CHECK" >/dev/null 2>&1; then
	fail "release-tag validation passed without GOPMGR_RELEASE_TAG"
fi

echo "check-release-tag tests passed."
