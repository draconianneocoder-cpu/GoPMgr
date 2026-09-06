#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VALIDATOR="$ROOT/scripts/validate-pades.sh"
SYSTEM_PATH="$PATH"
mkdir -p "$ROOT/.tmp"
FIXTURE="$(mktemp -d "$ROOT/.tmp/gopmgr-pades-cleanup-test.XXXXXX")"
OUTSIDE_FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-pades-outside-test.XXXXXX")"
trap 'rm -rf "$FIXTURE" "$OUTSIDE_FIXTURE"' EXIT

fail() {
	echo "validate-pades cleanup test: $*" >&2
	exit 1
}

outside_scratch="$OUTSIDE_FIXTURE/must-not-be-created"
set +e
outside_output="$(GOPMGR_PADES_SCRATCH_ROOT="$outside_scratch" bash "$VALIDATOR" 2>&1)"
outside_status=$?
set -e
[ "$outside_status" -eq 64 ] || fail "outside scratch-root status = $outside_status, want 64"
grep -Fq "must remain inside the repository" <<<"$outside_output" || fail "outside scratch-root rejection was not explained"
[ ! -e "$outside_scratch" ] || fail "rejected outside scratch root was created"

special_parent="$FIXTURE/special-parent"
mkdir -p "$special_parent"
for special_leaf in . ..; do
	candidate="$special_parent/$special_leaf"
	before="$(find "$FIXTURE" -mindepth 1 -print | sort)"
	set +e
	special_output="$(GOPMGR_PADES_SCRATCH_ROOT="$candidate" bash "$VALIDATOR" 2>&1)"
	special_status=$?
	set -e
	[ "$special_status" -eq 64 ] || fail "$special_leaf scratch-root status = $special_status, want 64"
	grep -Fq "Invalid PAdES scratch-root leaf" <<<"$special_output" || fail "$special_leaf scratch-root rejection was not explained"
	after="$(find "$FIXTURE" -mindepth 1 -print | sort)"
	[ "$before" = "$after" ] || fail "$special_leaf scratch-root rejection mutated the fixture"
done

scratch="$FIXTURE/scratch"
lock="$scratch/gopmgr-pades-test.lock"
mkdir -p "$lock" "$FIXTURE/bin"
printf '%s\n' caller-owned >"$lock/pid"

cat >"$FIXTURE/bin/go" <<'EOF'
#!/bin/bash
echo "seeded generator failure" >&2
exit 55
EOF
chmod +x "$FIXTURE/bin/go"

set +e
output="$(
	PATH="$FIXTURE/bin:$PATH" \
		GOPMGR_PADES_SCRATCH_ROOT="$scratch" \
		GOPMGR_PADES_LOCK_HELD=1 \
		bash "$VALIDATOR" 2>&1
)"
status=$?
set -e

[ "$status" -eq 55 ] || fail "generator status changed: got $status, want 55"
grep -Fq "generated diagnostics retained at" <<<"$output" || fail "retained path was not reported"
[ -f "$lock/pid" ] || fail "caller-owned inherited lock was removed"
grep -Fqx caller-owned "$lock/pid" || fail "caller-owned inherited lock was modified"
run_dir="$(find "$scratch" -mindepth 1 -maxdepth 1 -type d -name 'gopmgr-pades-test.build.*' -print)"
[ -n "$run_dir" ] || fail "failed generator working directory was removed"
[ -f "$run_dir/validate_pades.go" ] || fail "retained generator source is missing"

success_output="$(
	PATH="$SYSTEM_PATH" \
		GOPMGR_PADES_SCRATCH_ROOT="$scratch" \
		GOPMGR_PADES_LOCK_HELD=1 \
		bash "$VALIDATOR"
)"
published_sample="$scratch/gopmgr-pades-test/signed-sample.pdf"
[ -s "$published_sample" ] || fail "alternate-root sample was not published"
grep -Fq "Generated $published_sample" <<<"$success_output" || fail "alternate-root output reported the wrong sample path"
grep -Fq "PAdES-T local validation gate PASSED." <<<"$success_output" || fail "successful alternate-root validation was not reported"
[ -f "$lock/pid" ] || fail "successful inherited-lock run removed the caller-owned lock"

echo "validate-pades cleanup tests passed."
