#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/scripts/pades-lock.sh"
mkdir -p "$ROOT/.tmp"
TEST_ROOT="$(mktemp -d "$ROOT/.tmp/pades-lock-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

lock_path="$TEST_ROOT/lock"
pades_acquire_lock "$lock_path" 0
[ "$(cat "$lock_path")" = "$$" ] || fail "successful lock did not record this shell's PID"
rm -f "$lock_path"

# An abandoned lock is never reclaimed, and the timeout names its owner.
printf '%s\n' "abandoned-test-owner" >"$lock_path"
if output="$(pades_acquire_lock "$lock_path" 0 2>&1)"; then
	fail "abandoned lock unexpectedly acquired"
fi
echo "$output" | grep -F "timed out after 0s waiting for PAdES lock" >/dev/null ||
	fail "bounded lock failure did not identify timeout: $output"
echo "$output" | grep -F "owner PID: abandoned-test-owner" >/dev/null ||
	fail "timeout did not name the lock owner: $output"
[ -e "$lock_path" ] || fail "timeout removed an existing lock"
rm -f "$lock_path"

# A directory lock left by a release that used mkdir still counts as held.
mkdir "$lock_path"
printf '%s\n' "legacy-owner" >"$lock_path/pid"
if output="$(pades_acquire_lock "$lock_path" 0 2>&1)"; then
	fail "legacy directory lock unexpectedly acquired"
fi
echo "$output" | grep -F "owner PID: legacy-owner" >/dev/null ||
	fail "timeout did not name the legacy lock owner: $output"
[ -d "$lock_path" ] || fail "timeout removed a legacy directory lock"
rm -rf "$lock_path"

# Exclusion must not depend on the mkdir tool refusing an existing path:
# uutils coreutils' mkdir (Ubuntu 26.04) reports success for a path another
# caller created concurrently. A fake mkdir that always succeeds reproduces
# that outcome deterministically; a held lock must still refuse a second
# acquirer.
fake_bin="$TEST_ROOT/fake-bin"
mkdir -p "$fake_bin"
cat >"$fake_bin/mkdir" <<'EOF'
#!/bin/bash
# Stand-in for a mkdir that reports success even when the path exists.
for arg in "$@"; do
	case "$arg" in -*) ;; *) command -p mkdir -p "$arg" ;; esac
done
exit 0
EOF
chmod +x "$fake_bin/mkdir"
held_lock="$TEST_ROOT/held"
PATH="$fake_bin:$PATH" pades_acquire_lock "$held_lock" 0 || fail "first acquire failed with a permissive mkdir"
if PATH="$fake_bin:$PATH" pades_acquire_lock "$held_lock" 0 2>/dev/null; then
	fail "a held lock was acquired again when mkdir reports success for existing paths"
fi
rm -f "$held_lock"

# Concurrent acquirers: exactly one may win. mkdir-based locking failed this
# on uutils coreutils (Ubuntu 26.04), where concurrent mkdir calls on one path
# can all report success.
contenders=8
for round in $(seq 1 25); do
	race_lock="$TEST_ROOT/race-$round"
	start="$TEST_ROOT/start-$round"
	pids=()
	for _ in $(seq 1 "$contenders"); do
		# Spin until the start flag exists so all contenders race at once;
		# forking them one by one would otherwise stagger their attempts.
		(
			until [ -e "$start" ]; do :; done
			pades_acquire_lock "$race_lock" 0 >/dev/null 2>&1
		) &
		pids+=("$!")
	done
	sleep 0.2
	: >"$start"
	winners=0
	for pid in "${pids[@]}"; do
		if wait "$pid"; then
			winners=$((winners + 1))
		fi
	done
	[ "$winners" -eq 1 ] || fail "round $round: $winners of $contenders concurrent acquirers took the lock"
done

echo "pades lock tests passed."
