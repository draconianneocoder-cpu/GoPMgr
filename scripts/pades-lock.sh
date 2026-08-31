#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Acquire a directory lock with a bounded wait. Callers own cleanup after a
# successful return; this helper never removes an existing lock on timeout.
pades_acquire_directory_lock() {
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
	while ! mkdir "$lock_path" 2>/dev/null; do
		if [ "$((SECONDS - started_at))" -ge "$timeout_seconds" ]; then
			local owner="unknown"
			if [ -r "$lock_path/pid" ]; then
				owner="$(tr -d '\r\n' < "$lock_path/pid" 2>/dev/null || true)"
				[ -n "$owner" ] || owner="unknown"
			fi
			echo "timed out after ${timeout_seconds}s waiting for PAdES lock $lock_path (owner PID: $owner); verify no owner is running before removing the lock" >&2
			return 75
		fi
		sleep 0.1
	done

	if ! printf '%s\n' "$$" >"$lock_path/pid"; then
		rm -rf "$lock_path"
		echo "cannot record owner PID in PAdES lock $lock_path" >&2
		return 1
	fi
}
