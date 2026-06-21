#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

run_pair() {
	local iter="$1"
	local log_dir="$ROOT/.tmp/pmforge-pades-parallel-test-$iter"
	rm -rf "$ROOT/.tmp/pmforge-pades-test" "$log_dir"
	mkdir -p "$log_dir"

	set +e
	make check-pades >"$log_dir/check-pades.log" 2>&1 &
	local local_pid=$!
	make check-pades-external >"$log_dir/check-pades-external.log" 2>&1 &
	local external_pid=$!
	wait "$local_pid"
	local local_status=$?
	wait "$external_pid"
	local external_status=$?
	set -e

	if [ "$local_status" -ne 0 ] || [ "$external_status" -ne 0 ]; then
		echo "check-pades status: $local_status" >&2
		cat "$log_dir/check-pades.log" >&2
		echo "check-pades-external status: $external_status" >&2
		cat "$log_dir/check-pades-external.log" >&2
		fail "parallel PAdES validation gates raced in iteration $iter"
	fi
}

run_trio() {
	local iter="$1"
	local log_dir="$ROOT/.tmp/pmforge-pades-parallel-test-trio-$iter"
	rm -rf "$ROOT/.tmp/pmforge-pades-test" "$log_dir"
	mkdir -p "$log_dir"

	set +e
	make check-pades >"$log_dir/check-pades.log" 2>&1 &
	local local_pid=$!
	make check-pades-external >"$log_dir/check-pades-external.log" 2>&1 &
	local external_pid=$!
	bash scripts/validate-pades-external_test.sh >"$log_dir/check-pades-external-test.log" 2>&1 &
	local test_pid=$!
	wait "$local_pid"
	local local_status=$?
	wait "$external_pid"
	local external_status=$?
	wait "$test_pid"
	local test_status=$?
	set -e

	if [ "$local_status" -ne 0 ] || [ "$external_status" -ne 0 ] || [ "$test_status" -ne 0 ]; then
		echo "check-pades status: $local_status" >&2
		cat "$log_dir/check-pades.log" >&2
		echo "check-pades-external status: $external_status" >&2
		cat "$log_dir/check-pades-external.log" >&2
		echo "validate-pades-external_test status: $test_status" >&2
		cat "$log_dir/check-pades-external-test.log" >&2
		fail "parallel PAdES validation gates raced in trio iteration $iter"
	fi
}

run_pair 1
run_pair 2
run_trio 1

echo "validate-pades parallel tests passed."
