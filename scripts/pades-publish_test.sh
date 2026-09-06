#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-pades-publish-test.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT
source "$ROOT/scripts/pades-publish.sh"

fail() {
	echo "pades-publish test: $*" >&2
	exit 1
}

write_sample() {
	local path="$1"
	local value="$2"
	mkdir -p "$path"
	printf '%s\n' "$value" >"$path/signed-sample.pdf"
}

# First publication has no recovery directory to manage.
write_sample "$FIXTURE/new" first
pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/current"
grep -Fqx first "$FIXTURE/current/signed-sample.pdf" || fail "initial sample was not published"

# Normal replacement publishes the complete new directory and removes recovery
# state only after the replacement is live.
rm -rf "$FIXTURE/current"
write_sample "$FIXTURE/current" old
write_sample "$FIXTURE/new" new
pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/current"
grep -Fqx new "$FIXTURE/current/signed-sample.pdf" || fail "new sample was not published"
if find "$FIXTURE" -maxdepth 1 -name 'current.stale.*' -print | grep -q .; then
	fail "successful publication retained stale recovery state"
fi

# A failed replacement must restore the previous sample. Override only the
# operation seam; the production function and paths remain unchanged.
rm -rf "$FIXTURE/current"
write_sample "$FIXTURE/current" old
write_sample "$FIXTURE/new" new
move_count=0
pades_publish_mv() {
	move_count=$((move_count + 1))
	if [ "$move_count" -eq 2 ]; then
		return 73
	fi
	command mv "$@"
}
if pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/current" 2>"$FIXTURE/rollback.log"; then
	fail "replacement fault unexpectedly succeeded"
fi
grep -Fqx old "$FIXTURE/current/signed-sample.pdf" || fail "previous sample was not restored"
grep -Fq "restored previous sample" "$FIXTURE/rollback.log" || fail "rollback was not reported"
grep -Fqx new "$FIXTURE/new/signed-sample.pdf" || fail "failed replacement discarded the new sample"

# If both publication and rollback fail, neither recovery directory may be
# deleted. This is the fail-safe branch that prevents silent evidence loss.
rm -rf "$FIXTURE/current" "$FIXTURE/new"
write_sample "$FIXTURE/current" old
write_sample "$FIXTURE/new" new
move_count=0
pades_publish_mv() {
	move_count=$((move_count + 1))
	if [ "$move_count" -ge 2 ]; then
		return 74
	fi
	command mv "$@"
}
if pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/current" 2>"$FIXTURE/double-fault.log"; then
	fail "double fault unexpectedly succeeded"
fi
old_recovery="$(find "$FIXTURE" -maxdepth 1 -type d -name 'current.stale.*' -print)"
[ -n "$old_recovery" ] || fail "double fault discarded the previous sample"
grep -Fqx old "$old_recovery/signed-sample.pdf" || fail "recovery sample content changed"
grep -Fqx new "$FIXTURE/new/signed-sample.pdf" || fail "double fault discarded the generated sample"
grep -Fq "replacement and rollback failed" "$FIXTURE/double-fault.log" || fail "double fault was not reported"

# Symlinked targets and cross-parent publication are outside the owned
# publication boundary. Restore the real operation seam first so each rejection
# fails only because its own guard is present.
pades_publish_mv() {
	command mv "$@"
}
rm -rf "$old_recovery" "$FIXTURE/current" "$FIXTURE/new"
mkdir -p "$FIXTURE/external" "$FIXTURE/elsewhere"
write_sample "$FIXTURE/new" new
ln -s "$FIXTURE/external" "$FIXTURE/current"
if pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/current" >/dev/null 2>&1; then
	fail "symlinked sample target was accepted"
fi
[ ! -e "$FIXTURE/external/signed-sample.pdf" ] || fail "symlink target was modified"
unlink "$FIXTURE/current"
printf '%s\n' preserve >"$FIXTURE/current"
if pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/current" >/dev/null 2>&1; then
	fail "non-directory sample target was accepted"
fi
grep -Fqx preserve "$FIXTURE/current" || fail "unexpected target type was modified"
rm -f "$FIXTURE/current"
if pades_publish_sample_dir "$FIXTURE/new" "$FIXTURE/elsewhere/current" >/dev/null 2>&1; then
	fail "cross-parent publication was accepted"
fi

echo "pades-publish tests passed."
