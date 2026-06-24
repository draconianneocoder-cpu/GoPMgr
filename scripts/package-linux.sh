#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build Linux installers from the Wails Linux binary at build/bin/pmforge:
#   - .deb and .rpm via nfpm (build/linux/nfpm.yaml)
#
# Run `wails build -platform linux/amd64` first. VERSION defaults to the
# latest git tag with the leading "v" stripped (e.g. v1.2.0 -> 1.2.0).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-0.0.0}"
export VERSION

if [ ! -x build/bin/pmforge ]; then
	echo "package-linux: build/bin/pmforge missing — run 'wails build -platform linux/amd64' first." >&2
	exit 1
fi
if ! command -v nfpm >/dev/null 2>&1; then
	echo "package-linux: nfpm not found. Install with:" >&2
	echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest" >&2
	exit 1
fi

mkdir -p build/packages

echo "package-linux: building .deb and .rpm (version $VERSION) ..."
nfpm package --config build/linux/nfpm.yaml --packager deb --target "build/packages/pmforge-${VERSION}-amd64.deb"
nfpm package --config build/linux/nfpm.yaml --packager rpm --target "build/packages/pmforge-${VERSION}-x86_64.rpm"

echo "package-linux: done. Artifacts in build/packages/:"
ls -1 build/packages/ | sed 's/^/  /'
