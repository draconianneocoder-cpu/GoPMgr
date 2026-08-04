#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build Linux installers from the Wails Linux binary at build/bin/gopmgr:
#   - .deb and .rpm via nfpm (build/linux/nfpm.yaml)
#
# Run `wails build -platform linux/amd64` first. VERSION defaults to the
# latest git tag with the leading "v" stripped (e.g. v1.2.0 -> 1.2.0).
set -euo pipefail
cd "$(dirname "$0")/.."

NFPM_VERSION="$(sed -n 's/^NFPM_VERSION=//p' scripts/release-tool-versions.env)"
if [[ ! "$NFPM_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "package-linux: scripts/release-tool-versions.env has an invalid NFPM_VERSION." >&2
	exit 1
fi

VERSION="${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-0.0.0}"
export VERSION

if [ ! -x build/bin/gopmgr ]; then
	echo "package-linux: build/bin/gopmgr missing — run 'wails build -platform linux/amd64' first." >&2
	exit 1
fi
if ! command -v nfpm >/dev/null 2>&1; then
	echo "package-linux: nfpm not found. Install with:" >&2
	echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@$NFPM_VERSION" >&2
	exit 1
fi
# A source-built nFPM reports a development CLI version, so Go's embedded
# module metadata is the stable way to prove the binary matches the release pin.
nfpm_binary="$(command -v nfpm)"
actual_nfpm_version="$(go version -m "$nfpm_binary" 2>/dev/null | awk '$1 == "mod" && $2 == "github.com/goreleaser/nfpm/v2" { print $3 }')"
if [ "$actual_nfpm_version" != "$NFPM_VERSION" ]; then
	echo "package-linux: nfpm $NFPM_VERSION is required; found ${actual_nfpm_version:-unknown} at $nfpm_binary." >&2
	echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@$NFPM_VERSION" >&2
	exit 1
fi

mkdir -p build/packages

echo "package-linux: building .deb and .rpm (version $VERSION) ..."
nfpm package --config build/linux/nfpm.yaml --packager deb --target "build/packages/gopmgr-${VERSION}-amd64.deb"
nfpm package --config build/linux/nfpm.yaml --packager rpm --target "build/packages/gopmgr-${VERSION}-x86_64.rpm"

echo "package-linux: done. Artifacts in build/packages/:"
ls -1 build/packages/ | sed 's/^/  /'
