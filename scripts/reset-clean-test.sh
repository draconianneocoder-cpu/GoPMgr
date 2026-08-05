#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Prepare a recoverable first-launch test without deleting GoPMgr data. The
# active GoPMgr directory is renamed beside itself, so reopening the app
# creates a fresh system.db while the complete prior state remains restorable.
# The data-root leaf name here ("GoPMgr") matches users.DefaultRootDir; this
# script does not handle a not-yet-migrated pre-2026-08-04 "PMForge"-named
# install — see MigrateLegacyRoot for that one-time copy, which runs inside
# the app itself on next launch, not via this script.

set -euo pipefail

usage() {
	cat <<'EOF'
Usage:
  scripts/reset-clean-test.sh [--data-root /absolute/path/to/GoPMgr]
  scripts/reset-clean-test.sh [--data-root /absolute/path/to/GoPMgr] --restore BACKUP

Quit GoPMgr before running this command.

Reset moves the active GoPMgr data directory to a timestamped sibling backup.
Restore requires the active directory to be absent and moves the selected
backup back into place.
EOF
}

fail() {
	echo "reset-clean-test: $*" >&2
	exit 1
}

data_root="${GOPMGR_DATA_ROOT:-}"
restore_path=""

while (($# > 0)); do
	case "$1" in
		--data-root)
			(($# >= 2)) || fail "--data-root requires an absolute path"
			data_root="$2"
			shift 2
			;;
		--restore)
			(($# >= 2)) || fail "--restore requires a backup path"
			restore_path="$2"
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1"
			;;
	esac
done

if [[ -z "$data_root" ]]; then
	if [[ -n "${XDG_DATA_HOME:-}" ]]; then
		data_root="$XDG_DATA_HOME/GoPMgr"
	elif [[ "$(uname -s)" == "Darwin" ]]; then
		data_root="${HOME:?HOME is required}/Library/Application Support/GoPMgr"
	else
		data_root="${HOME:?HOME is required}/Documents/GoPMgr"
	fi
fi

[[ "$data_root" == /* ]] || fail "data root must be an absolute path"
[[ "$(basename "$data_root")" == "GoPMgr" ]] ||
	fail "data root must name the GoPMgr directory, got: $data_root"

# Resolve the parent rather than the target so reset still works when the app
# has never launched and the GoPMgr directory does not exist yet.
data_parent="$(dirname "$data_root")"
[[ -d "$data_parent" ]] || fail "data-root parent does not exist: $data_parent"
data_parent="$(cd "$data_parent" && pwd -P)"
[[ "$data_parent" != "/" ]] || fail "refusing a GoPMgr data root directly under /"
data_root="$data_parent/GoPMgr"

[[ ! -L "$data_root" ]] || fail "refusing symlinked data root: $data_root"

# Moving a live WAL-mode database risks an inconsistent backup. Exact process
# names cover the packaged binary and development builds without matching this
# script or unrelated paths containing the word "gopmgr".
if command -v pgrep >/dev/null 2>&1 &&
	{ pgrep -x gopmgr >/dev/null 2>&1 || pgrep -x GoPMgr >/dev/null 2>&1; }; then
	fail "GoPMgr is running; quit it before resetting or restoring data"
fi
if [[ -e "$data_root/system.db" ]] && command -v lsof >/dev/null 2>&1 &&
	lsof "$data_root/system.db" >/dev/null 2>&1; then
	fail "system.db is open; quit GoPMgr before resetting or restoring data"
fi

if [[ -n "$restore_path" ]]; then
	[[ "$restore_path" == /* ]] || fail "backup path must be absolute"
	[[ -d "$restore_path" ]] || fail "backup directory does not exist: $restore_path"
	[[ ! -L "$restore_path" ]] || fail "refusing symlinked backup: $restore_path"

	restore_parent="$(cd "$(dirname "$restore_path")" && pwd -P)"
	restore_name="$(basename "$restore_path")"
	[[ "$restore_parent" == "$data_parent" ]] ||
		fail "backup must be beside the GoPMgr data root"
	[[ "$restore_name" =~ ^GoPMgr\.clean-test-backup-[0-9]{8}T[0-9]{6}Z$ ]] ||
		fail "backup name is not a GoPMgr clean-test backup: $restore_name"
	[[ ! -e "$data_root" ]] ||
		fail "active data root exists; reset it first before restoring: $data_root"

	mv "$restore_path" "$data_root"
	echo "reset-clean-test: restored $data_root"
	exit 0
fi

if [[ ! -e "$data_root" ]]; then
	echo "reset-clean-test: already clean; no data root exists at $data_root"
	exit 0
fi
[[ -d "$data_root" ]] || fail "data root is not a directory: $data_root"

# The override makes the shell regression deterministic. Validate it as
# strictly as a generated timestamp so it cannot add path separators.
timestamp="${GOPMGR_RESET_TIMESTAMP:-$(date -u +%Y%m%dT%H%M%SZ)}"
[[ "$timestamp" =~ ^[0-9]{8}T[0-9]{6}Z$ ]] ||
	fail "invalid reset timestamp: $timestamp"

backup="$data_root.clean-test-backup-$timestamp"
[[ ! -e "$backup" ]] || fail "backup already exists: $backup"

mv "$data_root" "$backup"
echo "reset-clean-test: clean first-launch state is ready"
echo "reset-clean-test: backup=$backup"
echo "reset-clean-test: reopen GoPMgr to create a fresh administrator"
