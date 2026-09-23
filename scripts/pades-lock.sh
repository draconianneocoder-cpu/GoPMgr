#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Acquire a file lock with a bounded wait. Callers own cleanup after a
# successful return (rm -rf "$lock_path"); this helper never removes an
# existing lock on timeout.
#
# The lock is created by bash itself with noclobber, which refuses to replace
# an existing regular file, rather than with the mkdir binary. mkdir is not
# exclusive everywhere: Ubuntu 26.04 (CI's runner) ships uutils coreutils,
# whose mkdir returned success to BOTH callers in 1921 of 2000 concurrent
# attempts on one path, so two PAdES gates ran inside the "exclusive" section
# at once. The same race against this noclobber create found no double
# acquisition in 2000 attempts on bash 3.2 (macOS) or 5.3 (Linux). An existing
# directory at the path, such as a lock left by an older release, also counts
# as held.
pades_acquire_lock() {
	local lock_path="$1"
	local timeout_seconds="$2"
	case "$timeout_seconds" in
	'' | *[!0-9]*)
		echo "invalid PAdES lock timeout '$timeout_seconds'; use a non-negative integer" >&2
		return 64
		;;
	esac

	if ! mkdir -p "$(dirname "$lock_path")"; then
		echo "cannot create PAdES lock parent for $lock_path" >&2
		return 1
	fi

	local started_at=$SECONDS
	while ! (set -o noclobber && : >"$lock_path") 2>/dev/null; do
		if [ "$((SECONDS - started_at))" -ge "$timeout_seconds" ]; then
			local owner_file="$lock_path"
			[ -d "$lock_path" ] && owner_file="$lock_path/pid"
			local owner="unknown"
			if [ -f "$owner_file" ] && [ -r "$owner_file" ]; then
				owner="$(tr -d '\r\n' < "$owner_file" 2>/dev/null || true)"
				[ -n "$owner" ] || owner="unknown"
			fi
			echo "timed out after ${timeout_seconds}s waiting for PAdES lock $lock_path (owner PID: $owner); verify no owner is running before removing the lock" >&2
			return 75
		fi
		sleep 0.1
	done

	if ! printf '%s\n' "$$" >"$lock_path"; then
		rm -f "$lock_path"
		echo "cannot record owner PID in PAdES lock $lock_path" >&2
		return 1
	fi
}
