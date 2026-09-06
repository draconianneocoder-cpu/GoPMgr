#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COVERAGE="$ROOT/scripts/coverage-go.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-coverage-go-cleanup-test.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT

fail() {
	echo "coverage-go cleanup test: $*" >&2
	exit 1
}

mkdir -p "$FIXTURE/repo/scripts" "$FIXTURE/bin"
# grep -Ev returns 1 for an entirely empty filtered pattern set under `set -e`;
# use a harmless real pattern so this fixture reaches the cleanup behavior it
# is intended to test.
printf '%s\n' 'never/matches/fixture.go' >"$FIXTURE/repo/scripts/coverage-exclude-go.txt"

cat >"$FIXTURE/bin/go" <<'EOF'
#!/bin/bash
set -eu

case "$1" in
list)
	echo gopmgr
	;;
test)
	if [ "${FAKE_GO_TEST_FAIL:-0}" = "1" ]; then
		echo "seeded stdout failure"
		echo "seeded stderr failure" >&2
		exit 42
	fi
	profile=""
	for arg in "$@"; do
		case "$arg" in
		-coverprofile=*) profile="${arg#-coverprofile=}" ;;
		esac
	done
	[ -n "$profile" ] || exit 43
	cat >"$profile" <<'PROFILE'
mode: set
gopmgr/main.go:1.1,1.2 1 1
PROFILE
	;;
*) exit 44 ;;
esac
EOF
chmod +x "$FIXTURE/bin/go"

run_coverage() {
	PATH="$FIXTURE/bin:$PATH" GOPMGR_REPO_ROOT="$FIXTURE/repo" bash "$COVERAGE" "$@"
}

result="$(run_coverage default)"
[ "$result" = "1 1" ] || fail "unexpected successful result: $result"
if find "$FIXTURE/repo/.tmp" -mindepth 1 -maxdepth 1 -print | grep -q .; then
	fail "successful default run retained producer scratch"
fi

custom_profile="$FIXTURE/custom profile.out"
result="$(run_coverage duckdb "$custom_profile")"
[ "$result" = "1 1" ] || fail "unexpected custom-profile result: $result"
[ -f "$custom_profile" ] || fail "caller-requested profile was removed"
grep -Fq 'mode: set' "$custom_profile" || fail "caller-requested profile was corrupted"

failure_log="$FIXTURE/failure.log"
set +e
FAKE_GO_TEST_FAIL=1 run_coverage default >"$FIXTURE/failure.out" 2>"$failure_log"
failure_status=$?
set -e
[ "$failure_status" -eq 42 ] || fail "go test status changed: got $failure_status, want 42"
grep -Fq "seeded stdout failure" "$failure_log" || fail "go test stdout was not replayed"
grep -Fq "seeded stderr failure" "$failure_log" || fail "go test stderr was not replayed"
grep -Fq "diagnostics retained at" "$failure_log" || fail "failure did not report retained diagnostics"
failed_run="$(find "$FIXTURE/repo/.tmp" -mindepth 1 -maxdepth 1 -type d -name 'coverage-go-default.*' -print)"
[ -n "$failed_run" ] || fail "failed run diagnostics were deleted"
[ -f "$failed_run/packages.txt" ] || fail "failed run package inventory is missing"
grep -Fq "seeded stdout failure" "$failed_run/go-test.log" || fail "retained log lost stdout"
grep -Fq "seeded stderr failure" "$failed_run/go-test.log" || fail "retained log lost stderr"

echo "coverage-go cleanup tests passed."
