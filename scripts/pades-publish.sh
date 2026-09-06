#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Publish a complete PAdES sample directory while the caller holds PADES_LOCK.
# Shell functions wrap the two filesystem operations so the regression test can
# fault-inject the second move without depending on host permissions.
pades_publish_mv() {
	command mv "$@"
}

pades_publish_remove_tree() {
	command rm -rf "$1"
}

pades_publish_sample_dir() {
	local new_dir="$1"
	local sample_dir="$2"
	local new_parent sample_parent old_dir publish_status

	if [ ! -d "$new_dir" ] || [ -L "$new_dir" ]; then
		echo "pades publish: new sample must be a real directory: $new_dir" >&2
		return 1
	fi
	if [ -L "$sample_dir" ]; then
		echo "pades publish: refusing symlinked sample directory: $sample_dir" >&2
		return 1
	fi
	if [ -e "$sample_dir" ] && [ ! -d "$sample_dir" ]; then
		echo "pades publish: existing sample is not a directory: $sample_dir" >&2
		return 1
	fi

	new_parent="$(cd "$(dirname "$new_dir")" && pwd -P)" || return 1
	sample_parent="$(cd "$(dirname "$sample_dir")" && pwd -P)" || return 1
	if [ "$new_parent" != "$sample_parent" ]; then
		echo "pades publish: new and published samples must share one parent filesystem" >&2
		return 1
	fi

	old_dir=""
	if [ -e "$sample_dir" ]; then
		old_dir="$sample_dir.stale.$$"
		if [ -e "$old_dir" ] || [ -L "$old_dir" ]; then
			echo "pades publish: recovery path already exists: $old_dir" >&2
			return 1
		fi
		pades_publish_mv "$sample_dir" "$old_dir" || return 1
	fi

	if pades_publish_mv "$new_dir" "$sample_dir"; then
		if [ -n "$old_dir" ] && ! pades_publish_remove_tree "$old_dir"; then
			echo "pades publish: new sample is live, but previous sample remains at $old_dir" >&2
			return 1
		fi
		return 0
	else
		publish_status=$?
	fi

	if [ -n "$old_dir" ]; then
		if pades_publish_mv "$old_dir" "$sample_dir"; then
			echo "pades publish: replacement failed; restored previous sample at $sample_dir" >&2
		else
			echo "pades publish: replacement and rollback failed; preserve recovery sample at $old_dir" >&2
		fi
	else
		echo "pades publish: initial sample publication failed; generated sample remains at $new_dir" >&2
	fi
	return "$publish_status"
}
