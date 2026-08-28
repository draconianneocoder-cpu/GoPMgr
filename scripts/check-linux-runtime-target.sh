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

# CI (test/lint/verify) deliberately validates on the newest available
# GitHub-hosted runner image, ubuntu-26.04, to catch toolchain drift early.
require_contains .github/workflows/ci.yml "ubuntu-26.04"
require_contains .github/workflows/ci.yml "libwebkit2gtk-4.1-dev"
require_not_contains .github/workflows/ci.yml "ubuntu-22.04"
require_not_contains .github/workflows/ci.yml "libwebkit2gtk-4.0-dev"

# release.yml's non-packaging jobs (preflight) share CI's ubuntu-26.04
# baseline; only its Linux *packaging* job is pinned separately below.
require_contains .github/workflows/release.yml "ubuntu-26.04"
require_contains .github/workflows/release.yml "libwebkit2gtk-4.1-dev"
require_not_contains .github/workflows/release.yml "ubuntu-22.04"
require_not_contains .github/workflows/release.yml "libwebkit2gtk-4.0-dev"

# The Linux packaging job (build/.deb/.rpm) deliberately pins ubuntu-24.04
# rather than following CI onto ubuntu-26.04: GitHub's ubuntu-26.04 hosted
# runner image is still experimental, and this job only runs on a tag push
# -- rare, high-stakes, and a poor place to absorb a beta runner's
# instability. Building on the older 24.04 baseline is the standard
# build-old-run-new approach: the resulting .deb/.rpm's only runtime
# requirement is libwebkit2gtk-4.1 (present on 24.04 and every release
# since, including 26.04+), so it installs and runs correctly on this
# project's actual target audience -- Ubuntu 26.04+ users -- without the
# release pipeline itself depending on an experimental runner. This
# previously went unverified: a prior version of this check only asserted
# "ubuntu-26.04 appears somewhere in release.yml," which passed vacuously
# via the preflight job above regardless of what the packaging job's own
# `os:` pin said.
require_contains .github/workflows/release.yml "os: ubuntu-24.04"

require_contains Makefile "WAILS_BUILD_TAGS ?= duckdb,webkit2_41"

require_contains build/linux/nfpm.yaml "libwebkit2gtk-4.1-0"
require_contains build/linux/nfpm.yaml "webkit2gtk4.1"
require_not_contains build/linux/nfpm.yaml "libwebkit2gtk-4.0-37"
require_not_contains build/linux/nfpm.yaml "webkit2gtk3"

release_docs=(
	README.md
	ROADMAP.md
	docs/INSTALL.md
	docs/release-preflight.md
)
for file in "${release_docs[@]}"; do
	require_not_contains "$file" "Ubuntu 22.04"
	require_not_contains "$file" "ubuntu-22.04"
	require_not_contains "$file" "libwebkit2gtk-4.0"
	require_not_contains "$file" "WebKit2GTK 4.0"
done

echo "check-linux-runtime-target: CI on Ubuntu 26.04, Linux packaging pinned to Ubuntu 24.04, WebKit2GTK 4.1 throughout -- consistent."
