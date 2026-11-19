#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# The release build embeds an update manifest URL and public key only when
# a key is present and the platform is not linux/amd64.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/scripts/release-update-ldflags.sh"
TAG="v1.1.0-alpha.3"
REPO="example-owner/example-repo"
KEY="distinctive-test-key"
BASE="-X gopmgr/internal/update.CurrentVersion=1.1.0-alpha.3 -X gopmgr/internal/update.UpdateChannel=alpha"

fail() {
	echo "release-update-ldflags test: $*" >&2
	exit 1
}

expect_off() {
	local label="$1"
	shift
	local actual
	actual="$(bash "$SCRIPT" "$@")"
	if [ "$actual" != "$BASE" ]; then
		fail "$label embedded update settings: $actual"
	fi
	case "$actual" in
	*ManifestURL*|*UpdateChannelPublicKey*) fail "$label named an update setting" ;;
	esac
}

expect_on() {
	local platform="$1"
	local dashed="${platform//\//-}"
	local url="https://github.com/${REPO}/releases/download/${TAG}/update-alpha-${dashed}.json"
	local want="$BASE -X gopmgr/internal/update.ManifestURL=${url} -X gopmgr/internal/update.UpdateChannelPublicKey=${KEY}"
	local actual
	actual="$(bash "$SCRIPT" "$TAG" "$platform" "$KEY" "$REPO")"
	if [ "$actual" != "$want" ]; then
		fail "$platform ldflags = $actual; want $want"
	fi
}

expect_off "unset key on darwin" "$TAG" "darwin/arm64"
expect_off "empty key on darwin" "$TAG" "darwin/arm64" "" "$REPO"
expect_off "key on linux" "$TAG" "linux/amd64" "$KEY" "$REPO"
expect_on "darwin/arm64"
expect_on "windows/amd64"

echo "release-update-ldflags tests passed."
