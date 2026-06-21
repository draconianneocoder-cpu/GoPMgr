#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail
cd "$(dirname "$0")/.."
repo_root="$(pwd)"

target="${1:-}"
if [ -z "$target" ]; then
	echo "usage: scripts/package.sh <linux|windows|darwin>" >&2
	exit 2
fi

host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"

case "$target" in
	linux|windows|darwin) ;;
	*)
		echo "package: unsupported target '$target' (expected linux, windows, or darwin)." >&2
		exit 2
		;;
esac

if [ "$target" != "$host_os" ]; then
	echo "package: $target packaging requires a $target host/toolchain; current host is $host_os/$host_arch." >&2
	echo "package: build the binary with 'make build' here, or run this package target on a $target machine/CI runner." >&2
	exit 2
fi

if [ "${PMFORGE_FRONTEND_BUILT:-0}" = "1" ]; then
	PMFORGE_FRONTEND_BUILT=1 make build
else
	make build
fi

pkg_dir="build/packages"
mkdir -p "$pkg_dir"

binary="build/bin/pmforge"
archive_base="pmforge-${target}-${host_arch}"
archive="$pkg_dir/${archive_base}.tar.gz"

if [ ! -f "$binary" ]; then
	echo "package: expected built binary missing: $binary" >&2
	exit 1
fi

staging="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-package.XXXXXX")"
trap 'rm -rf "$staging"' EXIT
mkdir -p "$staging/$archive_base"
cp "$binary" "$staging/$archive_base/"
cp README.md "$staging/$archive_base/"
if [ -d LICENSES ]; then
	cp -R LICENSES "$staging/$archive_base/"
fi
if [ -f LICENSE ]; then
	cp LICENSE "$staging/$archive_base/"
fi

(cd "$staging" && tar -czf "$repo_root/$archive" "$archive_base")

echo "$archive"
