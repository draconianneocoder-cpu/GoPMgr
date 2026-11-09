#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

fail() {
	echo "check-linux-runtime-target: $*" >&2
	exit 1
}

require_contains() {
	local file=$1
	local pattern=$2
	if ! rg -q --fixed-strings -- "$pattern" "$file"; then
		fail "$file must contain: $pattern"
	fi
}

require_not_contains() {
	local file=$1
	local pattern=$2
	if rg -q --fixed-strings -- "$pattern" "$file"; then
		fail "$file must not contain: $pattern"
	fi
}

# CI (test/lint/verify) deliberately validates on the newest available
# GitHub-hosted runner image, ubuntu-26.04, to catch toolchain drift early.
require_contains .github/workflows/ci.yml "ubuntu-26.04"
require_contains .github/workflows/ci.yml "libwebkitgtk-6.0-dev"
require_not_contains .github/workflows/ci.yml "ubuntu-22.04"
require_not_contains .github/workflows/ci.yml "libwebkit2gtk-4.0-dev"

# release.yml's non-packaging jobs (preflight) share CI's ubuntu-26.04
# baseline; only its Linux *packaging* job is pinned separately below.
require_contains .github/workflows/release.yml "ubuntu-26.04"
require_contains .github/workflows/release.yml "libwebkitgtk-6.0-dev"
require_not_contains .github/workflows/release.yml "ubuntu-22.04"
require_not_contains .github/workflows/release.yml "libwebkit2gtk-4.0-dev"

# Keep the existing build-old/run-new packaging baseline. This checks
# configuration consistency, not clean-minimum-target native execution.
require_contains .github/workflows/release.yml "os: ubuntu-26.04"
# Ubuntu 26.04 is the Linux minimum (owner decision 2026-11-09); packages
# built on an older runner would link older libraries than the target.
require_not_contains .github/workflows/release.yml "ubuntu-24.04"

require_not_contains Makefile "webkit2_41"
require_not_contains Makefile "gtk3"
require_contains build/linux/Taskfile.yml "pkg-config --exists gtk4 webkitgtk-6.0"
require_not_contains build/linux/Taskfile.yml "purego"
require_contains debian/control "golang-1.27-go (>= 1.27.3)"
require_contains debian/control "libgtk-4-dev"
require_contains debian/control "libwebkitgtk-6.0-dev"
require_contains debian/control "libgtk-4-1"
require_contains debian/control "libwebkitgtk-6.0-4"
require_not_contains debian/control "libgtk-3"
require_not_contains debian/control "libwebkit2gtk"
require_not_contains debian/rules "webkit2_41"

require_contains build/linux/nfpm.yaml "libgtk-4-1"
require_contains build/linux/nfpm.yaml "libwebkitgtk-6.0-4"
require_contains build/linux/nfpm.yaml "- gtk4"
require_contains build/linux/nfpm.yaml "- webkitgtk6.0"
require_not_contains build/linux/nfpm.yaml "- libgtk-3"
require_not_contains build/linux/nfpm.yaml "- gtk3"
require_not_contains build/linux/nfpm.yaml "- libwebkit2gtk"

require_contains build/linux/nfpm.yaml "./third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt"
require_contains build/linux/nfpm.yaml "/usr/share/doc/gopmgr/sqlcipher-license.txt"
require_contains build/linux/nfpm.yaml "./internal/fonts/assets/FONT-NOTICES.txt"
require_contains build/linux/nfpm.yaml "/usr/share/doc/gopmgr/font-notices.txt"
require_contains build/linux/nfpm.yaml "./third_party/GO-MODULE-NOTICES.txt"
require_contains build/linux/nfpm.yaml "/usr/share/doc/gopmgr/go-module-notices.txt"
require_contains build/linux/nfpm.yaml "./third_party/FRONTEND-NOTICES.txt"
require_contains build/linux/nfpm.yaml "/usr/share/doc/gopmgr/frontend-notices.txt"
# The Debian source package (debian/rules) must install the same notices as
# the nfpm packages.
require_contains debian/rules "third_party/go-sqlcipher/THIRD_PARTY_NOTICES.txt debian/gopmgr/usr/share/doc/gopmgr/sqlcipher-license.txt"
require_contains debian/rules "internal/fonts/assets/FONT-NOTICES.txt debian/gopmgr/usr/share/doc/gopmgr/font-notices.txt"
require_contains debian/rules "third_party/GO-MODULE-NOTICES.txt debian/gopmgr/usr/share/doc/gopmgr/go-module-notices.txt"
require_contains debian/rules "third_party/FRONTEND-NOTICES.txt debian/gopmgr/usr/share/doc/gopmgr/frontend-notices.txt"
require_contains debian/rules "bash scripts/verify-module-notices.sh build/ppa/gopmgr"
# DuckDB is retired: no package may build with its tag or ship its notices.
require_not_contains Makefile "duckdb"
require_not_contains debian/rules "duckdb"
require_not_contains build/linux/nfpm.yaml "duckdb"
require_not_contains build/linux/nfpm.yaml "libwebkit2gtk-4.0-37"
require_not_contains build/linux/nfpm.yaml "webkit2gtk3"

release_docs=(
	README.md
	docs/ROADMAP.md
	docs/INSTALL.md
	docs/release-preflight.md
)
for file in "${release_docs[@]}"; do
	require_not_contains "$file" "Ubuntu 22.04"
	require_not_contains "$file" "ubuntu-22.04"
	require_not_contains "$file" "libwebkit2gtk-4.0"
	require_not_contains "$file" "WebKit2GTK 4.0"
done

echo "check-linux-runtime-target: CI and Linux packaging on Ubuntu 26.04, GTK4/WebKitGTK6 throughout -- configuration consistent."
