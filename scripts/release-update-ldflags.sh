#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Ldflags for a release build. An empty key leaves the update channel off.
# linux/amd64 stays off even when a key is present, because that platform
# has no signed installer in the update channel.

set -euo pipefail

tag="${1:-}"
platform="${2:-}"
key="${3-}"
repository="${4:-}"

root="$(cd "$(dirname "$0")" && pwd)"
channel="$(bash "$root/release-update-channel.sh" "$tag")"
version="${tag#v}"
ldflags="-X gopmgr/internal/update.CurrentVersion=${version} -X gopmgr/internal/update.UpdateChannel=${channel}"

if [ -n "$key" ] && [ "$platform" != "linux/amd64" ]; then
	target="${platform//\//-}"
	url="https://github.com/${repository}/releases/download/${tag}/update-${channel}-${target}.json"
	ldflags="${ldflags} -X gopmgr/internal/update.ManifestURL=${url} -X gopmgr/internal/update.UpdateChannelPublicKey=${key}"
fi

printf '%s\n' "$ldflags"
