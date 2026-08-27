#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Build Linux installers from the Wails Linux binary at build/bin/gopmgr:
#   - .deb and .rpm via nfpm (build/linux/nfpm.yaml)
#
# Run `wails build -platform linux/amd64` first. VERSION defaults to
# scripts/package-version-lib.sh's release_version() -- the same derivation
# package-macos.sh/package-macos-installer.sh use, so a build past its
# nearest tag gets an honest "-<N>-g<sha>" suffix instead of silently
# reusing the tag's bare name (see that file's own comment for why
# `git describe --tags --abbrev=0` alone is wrong).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/package-version-lib.sh"
cd "$SCRIPT_DIR/.."

NFPM_VERSION="$(sed -n 's/^NFPM_VERSION=//p' scripts/release-tool-versions.env)"
if [[ ! "$NFPM_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "package-linux: scripts/release-tool-versions.env has an invalid NFPM_VERSION." >&2
	exit 1
fi

VERSION="$(release_version)"
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
# `|| true`: if $nfpm_binary isn't a Go executable at all, `go version -m`
# exits non-zero and, under pipefail, so does this whole assignment -- which
# would trip `set -e` and kill the script before the version-mismatch error
# below ever prints. Fold that failure into an empty actual_nfpm_version
# instead, so it still hits the intended mismatch check and message.
actual_nfpm_version="$(go version -m "$nfpm_binary" 2>/dev/null | awk '$1 == "mod" && $2 == "github.com/goreleaser/nfpm/v2" { print $3 }' || true)"
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
