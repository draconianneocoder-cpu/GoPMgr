#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Exercise reset and restore behavior entirely under a temporary parent. These
# fixtures prove the production helper never needs a developer's real data.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RESET="$ROOT/scripts/reset-clean-test.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-clean-test-reset.XXXXXX")"
FIXTURE="$(cd "$FIXTURE" && pwd -P)"
trap 'rm -rf "$FIXTURE"' EXIT

fail() {
	echo "reset-clean-test test: $*" >&2
	exit 1
}

assert_fails_with() {
	local expected="$1"
	shift
	local output
	if output="$("$@" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	grep -Fq "$expected" <<<"$output" ||
		fail "failure did not contain '$expected': $output"
}

data_root="$FIXTURE/GoPMgr"
backup="$FIXTURE/GoPMgr.clean-test-backup-20260728T120000Z"
mkdir -p "$data_root/clean_admin/projects"
printf '%s\n' "test database sentinel" >"$data_root/system.db"
printf '%s\n' "test project sentinel" >"$data_root/clean_admin/projects/example.gopmgr"

output="$(
	GOPMGR_RESET_TIMESTAMP=20260728T120000Z \
		bash "$RESET" --data-root "$data_root"
)"
[[ ! -e "$data_root" ]] || fail "reset left the active data root in place"
[[ -f "$backup/system.db" ]] || fail "reset did not preserve system.db"
[[ -f "$backup/clean_admin/projects/example.gopmgr" ]] ||
	fail "reset did not preserve user project data"
grep -Fq "backup=$backup" <<<"$output" ||
	fail "reset output did not report the recoverable backup path"

already_clean="$(bash "$RESET" --data-root "$data_root")"
grep -Fq "already clean" <<<"$already_clean" ||
	fail "missing data root did not report an already-clean state"

bash "$RESET" --data-root "$data_root" --restore "$backup" >/dev/null
[[ -f "$data_root/system.db" ]] || fail "restore did not recover system.db"
[[ -f "$data_root/clean_admin/projects/example.gopmgr" ]] ||
	fail "restore did not recover user project data"
[[ ! -e "$backup" ]] || fail "restore left the backup in place"

mkdir "$backup"
assert_fails_with "backup already exists" \
	env GOPMGR_RESET_TIMESTAMP=20260728T120000Z \
	bash "$RESET" --data-root "$data_root"
rmdir "$backup"

mkdir "$backup"
assert_fails_with "active data root exists" \
	bash "$RESET" --data-root "$data_root" --restore "$backup"
rmdir "$backup"

assert_fails_with "invalid reset timestamp" \
	env GOPMGR_RESET_TIMESTAMP='../unsafe' \
	bash "$RESET" --data-root "$data_root"

assert_fails_with "must be an absolute path" \
	bash "$RESET" --data-root "relative/GoPMgr"

assert_fails_with "directly under /" \
	bash "$RESET" --data-root "/GoPMgr"

assert_fails_with "must name the GoPMgr directory" \
	bash "$RESET" --data-root "$FIXTURE/not-gopmgr"

mkdir -p "$FIXTURE/symlink-target"
mv "$data_root" "$FIXTURE/restored-data"
ln -s "$FIXTURE/symlink-target" "$data_root"
assert_fails_with "refusing symlinked data root" \
	bash "$RESET" --data-root "$data_root"
unlink "$data_root"

mkdir "$FIXTURE/not-a-clean-test-backup"
assert_fails_with "backup name is not a GoPMgr clean-test backup" \
	bash "$RESET" --data-root "$data_root" --restore "$FIXTURE/not-a-clean-test-backup"

ln -s "$FIXTURE/symlink-target" "$FIXTURE/GoPMgr.clean-test-backup-symlink"
assert_fails_with "refusing symlinked backup" \
	bash "$RESET" --data-root "$data_root" \
	--restore "$FIXTURE/GoPMgr.clean-test-backup-symlink"

echo "reset-clean-test tests passed."
