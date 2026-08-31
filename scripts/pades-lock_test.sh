#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/scripts/pades-lock.sh"
mkdir -p "$ROOT/.tmp"
TEST_ROOT="$(mktemp -d "$ROOT/.tmp/pades-lock-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

lock_path="$TEST_ROOT/lock"
pades_acquire_directory_lock "$lock_path" 0
[ -s "$lock_path/pid" ] || { echo "FAIL: successful lock did not record an owner PID" >&2; exit 1; }
rm -rf "$lock_path"

mkdir "$lock_path"
printf '%s\n' "abandoned-test-owner" >"$lock_path/pid"
if output="$(pades_acquire_directory_lock "$lock_path" 0 2>&1)"; then
	echo "FAIL: abandoned lock unexpectedly acquired" >&2
	exit 1
fi
echo "$output" | grep -F "timed out after 0s waiting for PAdES lock" >/dev/null || {
	echo "FAIL: bounded lock failure did not identify timeout" >&2
	echo "$output" >&2
	exit 1
}
[ -d "$lock_path" ] || { echo "FAIL: timeout removed an existing lock" >&2; exit 1; }

echo "pades lock tests passed."
