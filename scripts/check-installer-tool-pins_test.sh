#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Hermetic regressions for the release installer-tool pin contract. Each case
# mutates an isolated fixture so failures identify one drift class at a time.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-installer-tool-pins.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/pmforge-installer-pins-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "check-installer-tool-pins test: $*" >&2
	exit 1
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	mkdir -p "$fixture/.github/workflows" "$fixture/scripts"
	cp "$ROOT/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
	cp "$ROOT/scripts/package-linux.sh" "$fixture/scripts/package-linux.sh"
	cp "$ROOT/scripts/release-tool-versions.env" "$fixture/scripts/release-tool-versions.env"
	printf '%s\n' "$fixture"
}

expect_failure() {
	local fixture="$1"
	local expected="$2"
	local output
	if output="$(bash "$CHECK" "$fixture" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	case "$output" in
		*"$expected"*) ;;
		*) fail "failure did not contain '$expected': $output" ;;
	esac
}

baseline="$(new_fixture baseline)"
bash "$CHECK" "$baseline" >/dev/null || fail "valid pinned fixture was rejected"

latest_nfpm="$(new_fixture latest-nfpm)"
perl -0pi -e 's/\@"\$\{NFPM_VERSION\}"/\@latest/' "$latest_nfpm/.github/workflows/release.yml"
expect_failure "$latest_nfpm" "nFPM install must use the declared NFPM_VERSION"

unversioned_nsis="$(new_fixture unversioned-nsis)"
perl -0pi -e 's/ --version="\$\{NSIS_VERSION\}"//' "$unversioned_nsis/.github/workflows/release.yml"
expect_failure "$unversioned_nsis" "NSIS install must use the declared NSIS_VERSION"

missing_nsis_path="$(new_fixture missing-nsis-path)"
perl -0pi -e 's#echo '\''C:\\Program Files \(x86\)\\NSIS'\'' >> "\$GITHUB_PATH"#echo '\''broken path'\''#' "$missing_nsis_path/.github/workflows/release.yml"
expect_failure "$missing_nsis_path" "workflow must publish Chocolatey's NSIS directory"

unused_create_dmg="$(new_fixture unused-create-dmg)"
printf '\n      - name: Unpinned create-dmg\n        run: brew install create-dmg\n' >>"$unused_create_dmg/.github/workflows/release.yml"
expect_failure "$unused_create_dmg" "release workflow must not install create-dmg"

missing_loader="$(new_fixture missing-loader)"
perl -0pi -e "s/grep -E '\\^\\(NFPM_VERSION\\|NSIS_VERSION\\)=/grep -E '^(BROKEN_VERSION)=/" "$missing_loader/.github/workflows/release.yml"
expect_failure "$missing_loader" "workflow must load release-tool-versions.env"

late_loader="$(new_fixture late-loader)"
perl -0pi -e "s/grep -E '\\^\\(NFPM_VERSION\\|NSIS_VERSION\\)=/grep -E '^(BROKEN_VERSION)=/" "$late_loader/.github/workflows/release.yml"
printf "\n      - name: Too-late installer versions\n        run: grep -E '^(NFPM_VERSION|NSIS_VERSION)=' scripts/release-tool-versions.env >> \"\\\$GITHUB_ENV\"\n" >>"$late_loader/.github/workflows/release.yml"
expect_failure "$late_loader" "workflow must load installer versions before native tool installation"

invalid_version="$(new_fixture invalid-version)"
perl -0pi -e 's/^NFPM_VERSION=.*/NFPM_VERSION=latest/m' "$invalid_version/scripts/release-tool-versions.env"
expect_failure "$invalid_version" "NFPM_VERSION must be a v-prefixed stable semantic version"

stale_local_guidance="$(new_fixture stale-local-guidance)"
perl -0pi -e 's/\@\$NFPM_VERSION/\@latest/g' "$stale_local_guidance/scripts/package-linux.sh"
expect_failure "$stale_local_guidance" "package-linux must recommend the declared nFPM version"

echo "check-installer-tool-pins tests passed."
