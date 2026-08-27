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

backup_root="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-package-linux-test.XXXXXX")"
stub_bin="$backup_root/bin"
mkdir -p "$stub_bin"

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

# --- Negative-path guards (run unconditionally -- none of these need a real
# nfpm on PATH, unlike Case 1/2 below, so they must not sit behind the
# nfpm-availability skip). Each is a pre-existing guard in package-linux.sh
# that predates this session and had zero dynamic test coverage before now;
# scripts/check-installer-tool-pins.sh only checks the *source* contains
# certain literal substrings, it never runs the guards against a real
# mismatched/missing tool. ---

# R3: build/bin/gopmgr missing must hard-fail with an actionable message,
# not silently package a stale binary from a previous run or crash mid-way.
mv build/bin/gopmgr "$backup_root/gopmgr-saved"
r3_output="$(VERSION="$test_version" bash scripts/package-linux.sh 2>&1)" && {
	printf '%s\n' "$r3_output" >&2
	fail "R3: package-linux.sh should fail when build/bin/gopmgr is missing, but it succeeded"
}
case "$r3_output" in
*"build/bin/gopmgr missing"*) ;;
*)
	printf '%s\n' "$r3_output" >&2
	fail "R3: package-linux.sh's missing-binary error did not mention build/bin/gopmgr"
	;;
esac
mv "$backup_root/gopmgr-saved" build/bin/gopmgr

# R4: nfpm absent from PATH must hard-fail with install instructions, not a
# confusing raw "command not found" further down. Restrict PATH to bare
# system dirs so this doesn't depend on -- or get fooled by -- whatever the
# invoking host actually has installed.
r4_output="$(VERSION="$test_version" PATH=/usr/bin:/bin bash scripts/package-linux.sh 2>&1)" && {
	printf '%s\n' "$r4_output" >&2
	fail "R4: package-linux.sh should fail when nfpm is not on PATH, but it succeeded"
}
case "$r4_output" in
*"nfpm not found"*) ;;
*)
	printf '%s\n' "$r4_output" >&2
	fail "R4: package-linux.sh's nfpm-missing error did not say 'nfpm not found'"
	;;
esac

# R5: an nfpm on PATH whose Go module build metadata doesn't match the
# pinned NFPM_VERSION must hard-fail *before* packaging runs -- the
# anti-drift guard that stops an untrusted/wrong nfpm binary from silently
# building release artifacts. The stub below is a plain shell script, not a
# real Go binary, so `go version -m` on it reports no module info and
# actual_nfpm_version resolves empty -- this proves the guard rejects an
# nfpm whose version can't be verified, which is the guard's actual
# contract; it does NOT prove the comparison discriminates between two
# specific version strings (that would need a second real pinned nfpm
# build, which needs network access this test doesn't assume). The sentinel
# proves the guard fires before `nfpm package` ever runs, not just that the
# run eventually fails.
nfpm_called_sentinel="$backup_root/nfpm-called"
cat > "$stub_bin/nfpm" << 'STUB'
#!/bin/bash
: > "${NFPM_CALLED_SENTINEL:?NFPM_CALLED_SENTINEL not set}"
exit 0
STUB
chmod +x "$stub_bin/nfpm"
rm -f "$nfpm_called_sentinel"
r5_output="$(VERSION="$test_version" PATH="$stub_bin:$PATH" NFPM_CALLED_SENTINEL="$nfpm_called_sentinel" bash scripts/package-linux.sh 2>&1)" && {
	printf '%s\n' "$r5_output" >&2
	fail "R5: package-linux.sh should fail when nfpm's module version can't be verified, but it succeeded"
}
case "$r5_output" in
*"is required; found"*) ;;
*)
	printf '%s\n' "$r5_output" >&2
	fail "R5: package-linux.sh's nfpm-version-mismatch error did not say 'is required; found'"
	;;
esac
if [ -f "$nfpm_called_sentinel" ]; then
	fail "R5: package-linux.sh invoked nfpm despite an unverifiable version -- the version guard did not block packaging"
fi

# R2 (malformed NFPM_VERSION in release-tool-versions.env) is deliberately
# not covered here: scripts/check-installer-tool-pins.sh already validates
# NFPM_VERSION against the identical regex from the same file, and runs
# inside `make verify` on every commit -- a malformed pin can't reach this
# script without that check already having failed first. What's untested is
# package-linux.sh's own duplicate re-validation of the same value, which
# only matters if someone runs this script directly without `make verify`;
# lower value than R3/R4/R5 above and skipped to bound this pass's scope.

if ! command -v nfpm >/dev/null 2>&1; then
	# Safe to skip Case 1/2 below: they build real .deb/.rpm via the actual
	# pinned nfpm and need it on PATH. package-linux.sh itself hard-fails
	# with a clear error if nfpm is missing (just proven above by R4), so
	# this isn't a path that can go green without exercising the real
	# check -- only the positive-path artifact-building cases are skipped.
	echo "package-linux_test: nfpm not found on PATH, skipping Case 1/2 (install with" >&2
	echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@\$(sed -n 's/^NFPM_VERSION=//p' scripts/release-tool-versions.env))." >&2
	echo "package-linux negative-path tests (R3/R4/R5) passed; Case 1/2 skipped." >&2
	exit 0
fi

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
