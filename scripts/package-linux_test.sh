#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

if ! command -v nfpm >/dev/null 2>&1; then
	# Safe to skip silently here: package-linux.sh itself hard-fails with a
	# clear error if nfpm is missing, so this isn't a path that can go green
	# without exercising the real check. Re-verify that invariant before
	# wiring this test into `make verify` or CI, where a silent skip would
	# otherwise let a missing-nfpm environment look untested rather than
	# failing loudly.
	echo "package-linux_test: nfpm not found on PATH, skipping (install with" >&2
	echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@\$(sed -n 's/^NFPM_VERSION=//p' scripts/release-tool-versions.env))." >&2
	exit 0
fi

backup_root="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-package-linux-test.XXXXXX")"

restore_path() {
	local path="$1"
	local backup="$2"
	rm -rf "$path"
	if [ -e "$backup" ]; then
		mv "$backup" "$path"
	fi
}

cleanup() {
	restore_path "$ROOT/build/bin" "$backup_root/build-bin.backup"
	restore_path "$ROOT/build/packages" "$backup_root/build-packages.backup"
	rm -rf "$backup_root"
}
trap cleanup EXIT

if [ -e build/bin ]; then
	mv build/bin "$backup_root/build-bin.backup"
fi
if [ -e build/packages ]; then
	mv build/packages "$backup_root/build-packages.backup"
fi

mkdir -p build/bin build/packages
printf '#!/bin/bash\necho fake gopmgr\n' > build/bin/gopmgr
chmod +x build/bin/gopmgr

# A hyphenated, untagged-past-its-nearest-tag version, matching real
# `git describe --tags` output (without --abbrev=0) for a checkout that
# isn't exactly on a tag -- the exact shape package-version-lib.sh's
# release_version() produces and package-macos.sh already relies on. Proves
# package-linux.sh now derives VERSION the same way rather than silently
# reusing --abbrev=0's bare-tag-name behavior.
test_version="1.2.0-3-gabc1234"

output="$(VERSION="$test_version" bash scripts/package-linux.sh 2>&1)" || {
	printf '%s\n' "$output" >&2
	fail "package-linux.sh failed with an explicit hyphenated VERSION"
}

deb="build/packages/gopmgr-${test_version}-amd64.deb"
rpm="build/packages/gopmgr-${test_version}-x86_64.rpm"

if [ ! -f "$deb" ]; then
	fail "package-linux.sh did not produce the expected .deb at $deb"
fi
if [ ! -f "$rpm" ]; then
	fail "package-linux.sh did not produce the expected .rpm at $rpm"
fi
# The .rpm's own internal Version: metadata is NOT independently inspected
# here (unlike the .deb below) -- this host has no rpm/rpm2cpio/dnf and no
# such Python library, which is exactly docs/beta-release-backlog.md row
# 52's own disclosed remaining blocker. nfpm is the same tool producing
# both formats from the same VERSION input, so its .deb-observed
# sanitization behavior is a reasonable basis for confidence, not proof, for
# the .rpm. Whether rpm/dnf themselves correctly PARSE and ORDER the
# resulting version string relative to a plain release version -- the real
# question this whole backlog item is blocked on -- is verified by nothing
# in this test file; that needs an actual Linux host with rpm/dnf installed.

# Prove VERSION is actually threaded through to nfpm's own package metadata,
# not just the output filename -- ar+tar for .deb (both are standard on
# macOS/Linux); dpkg-deb is Debian-only and not assumed present.
if command -v ar >/dev/null 2>&1 && command -v tar >/dev/null 2>&1; then
	extract_dir="$backup_root/deb-extract"
	mkdir -p "$extract_dir"
	(cd "$extract_dir" && ar x "$ROOT/$deb")
	control_tar="$(ls "$extract_dir"/control.tar.* 2>/dev/null | head -1 || true)"
	if [ -z "$control_tar" ]; then
		fail "package-linux.sh's .deb has no control.tar.* member"
	fi
	control_contents="$(tar -xOf "$control_tar" -C "$extract_dir" ./control 2>/dev/null || tar -xOf "$control_tar" ./control 2>/dev/null || true)"
	# nfpm sanitizes VERSION for the package's own internal metadata field
	# (Debian version syntax forbids an unescaped "-" before the upstream
	# version's final component): the first hyphen becomes "~", matching
	# what a prior session observed running the same pinned nfpm directly
	# (docs/beta-release-backlog.md row 52). The produced FILENAME still
	# carries the raw VERSION verbatim, since --target sets it explicitly --
	# only the in-package Version: field is sanitized.
	sanitized_version="1.2.0~3-gabc1234"
	case "$control_contents" in
	*"Version: $sanitized_version"*) ;;
	*)
		printf '%s\n' "$control_contents" >&2
		fail ".deb control file does not declare the expected sanitized Version: $sanitized_version"
		;;
	esac
fi

# Case 2: prove package-linux.sh actually WIRES UP release_version() for
# the real (VERSION unset) path, not just that an explicit VERSION passes
# through -- Case 1 above, like package-macos_test.sh's own cases, always
# sets VERSION explicitly, so neither would catch a regression to the old
# inline `git describe --tags --abbrev=0` fallback (or any other reverted
# logic) as long as an explicit VERSION is always supplied by the caller.
# release_version()'s own git-describe-vs-abbrev0 behavior is already
# exhaustively tested in isolation by package-version-lib_test.sh; what's
# unverified here is that package-linux.sh's real fallback path actually
# calls it. package-linux.sh forces its own CWD to the real repo root
# before calling release_version(), so that behavior can't be exercised
# safely against this actual checkout's tags without mutating them -- copy
# the script (and its static file dependencies) into an isolated throwaway
# repo instead, matching package-version-lib_test.sh's own isolation
# technique one level up the call stack.
fake_repo="$backup_root/fake-repo"
mkdir -p "$fake_repo/scripts" "$fake_repo/build/linux" "$fake_repo/build/bin"
cp scripts/package-linux.sh scripts/package-version-lib.sh scripts/release-tool-versions.env "$fake_repo/scripts/"
cp build/linux/nfpm.yaml build/linux/gopmgr.desktop "$fake_repo/build/linux/"
cp build/appicon.png "$fake_repo/build/"
printf '#!/bin/bash\necho fake gopmgr\n' > "$fake_repo/build/bin/gopmgr"
chmod +x "$fake_repo/build/bin/gopmgr"
(
	cd "$fake_repo"
	git init -q
	git config user.email "test@example.invalid"
	git config user.name "test"
	git add -A
	git commit -q -m "first commit"
	git tag v1.2.3
	echo two > file.txt
	git add file.txt
	git commit -q -m "second commit, past the tag"
)

fallback_output="$(cd "$fake_repo" && env -u VERSION bash scripts/package-linux.sh 2>&1)" || {
	printf '%s\n' "$fallback_output" >&2
	fail "package-linux.sh failed with VERSION unset in the isolated repo"
}
case "$fallback_output" in
*"gopmgr-1.2.3-amd64.deb"* | *"gopmgr-1.2.3-x86_64.rpm"*)
	printf '%s\n' "$fallback_output" >&2
	fail "package-linux.sh with VERSION unset past a tag produced the bare tag name -- the --abbrev=0 regression is back"
	;;
*"gopmgr-1.2.3-"*"-amd64.deb"*) ;;
*)
	printf '%s\n' "$fallback_output" >&2
	fail "package-linux.sh with VERSION unset did not produce a tag-distance-suffixed artifact name"
	;;
esac
if ! ls "$fake_repo"/build/packages/gopmgr-1.2.3-*-amd64.deb >/dev/null 2>&1; then
	fail "package-linux.sh (VERSION unset, isolated repo) did not create the expected distance-suffixed .deb"
fi

echo "package-linux layout test passed."
