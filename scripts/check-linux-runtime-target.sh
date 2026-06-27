#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

fail() {
	echo "check-linux-runtime-target: $*" >&2
	exit 1
}

require_contains() {
	local file=$1
	local pattern=$2
	if ! rg -q --fixed-strings "$pattern" "$file"; then
		fail "$file must contain: $pattern"
	fi
}

require_not_contains() {
	local file=$1
	local pattern=$2
	if rg -q --fixed-strings "$pattern" "$file"; then
		fail "$file must not contain: $pattern"
	fi
}

workflow_files=(.github/workflows/ci.yml .github/workflows/release.yml)
for file in "${workflow_files[@]}"; do
	require_contains "$file" "ubuntu-24.04"
	require_contains "$file" "libwebkit2gtk-4.1-dev"
	require_not_contains "$file" "ubuntu-22.04"
	require_not_contains "$file" "libwebkit2gtk-4.0-dev"
done

require_contains Makefile "WAILS_BUILD_TAGS ?= duckdb,webkit2_41"

require_contains build/linux/nfpm.yaml "libwebkit2gtk-4.1-0"
require_contains build/linux/nfpm.yaml "webkit2gtk4.1"
require_not_contains build/linux/nfpm.yaml "libwebkit2gtk-4.0-37"
require_not_contains build/linux/nfpm.yaml "webkit2gtk3"

release_docs=(
	docs/INSTALL.md
	docs/release-notes/v1.1.0-rc.1.md
	docs/release-preflight.md
)
for file in "${release_docs[@]}"; do
	require_not_contains "$file" "Ubuntu 22.04"
	require_not_contains "$file" "ubuntu-22.04"
	require_not_contains "$file" "libwebkit2gtk-4.0"
	require_not_contains "$file" "WebKit2GTK 4.0"
done

echo "check-linux-runtime-target: Ubuntu 24.04 + WebKit2GTK 4.1 target is consistent."
