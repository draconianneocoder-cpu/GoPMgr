#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Shared helpers for scripts/validate-pdfa.sh. Keep this file free of
# side effects so validate-pdfa-lib_test.sh can source it directly.

verapdf_output_is_compliant() {
	local output compact
	output="$(cat)"
	compact="$(printf '%s' "$output" | tr -d '[:space:]')"

	case "$compact" in
		*'<isCompliant>true</isCompliant>'* | *'isCompliant="true"'* | *"isCompliant='true'"* | *'"isCompliant":true'*)
			return 0
			;;
	esac

	return 1
}

verapdf_sample_arg() {
	local mode root sample rel
	mode="$1"
	root="${2%/}"
	sample="$3"

	case "$mode" in
		docker)
			case "$sample" in
				"$root"/*)
					rel="${sample#"$root"/}"
					printf '/work/%s\n' "$rel"
					;;
				*)
					printf 'validate-pdfa: sample is outside repo root: %s\n' "$sample" >&2
					return 1
					;;
			esac
			;;
		cli)
			printf '%s\n' "$sample"
			;;
		*)
			printf 'validate-pdfa: unknown veraPDF mode: %s\n' "$mode" >&2
			return 1
			;;
	esac
}

find_verapdf_executable() {
	local dir candidate
	dir="$1"

	while IFS= read -r candidate; do
		if [ -x "$candidate" ]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done < <(find "$dir" -type f -name verapdf -print 2>/dev/null)

	return 1
}

zip_archive_is_valid() {
	local path
	path="$1"

	if [ ! -s "$path" ]; then
		return 1
	fi

	if command -v unzip >/dev/null 2>&1; then
		unzip -tq "$path" >/dev/null 2>&1
		return $?
	fi

	return 0
}

verapdf_cli_needs_refresh() {
	local cli jar
	cli="$1"
	jar="$2"

	if [ ! -x "$cli" ]; then
		return 0
	fi

	if grep -F "$jar" "$cli" >/dev/null 2>&1 && ! zip_archive_is_valid "$jar"; then
		return 0
	fi

	return 1
}
