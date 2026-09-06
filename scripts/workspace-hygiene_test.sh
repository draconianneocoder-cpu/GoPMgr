#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPORTER="$ROOT/scripts/workspace-hygiene.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-workspace-hygiene-test.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT

fail() {
	echo "workspace-hygiene test: $*" >&2
	exit 1
}

make_repo() {
	local path="$1"
	mkdir -p "$path/.tmp"
	printf '%s\n' 'module gopmgr' >"$path/go.mod"
	printf '%s\n' 'help:' >"$path/Makefile"
}

repo="$FIXTURE/repo with spaces"
make_repo "$repo"
mkdir -p "$repo/.tmp/verapdf" "$repo/.tmp/gopmgr-pades-test" "$repo/.tmp/unknown directory"
mkdir -p "$repo/.tmp/coverage-go-default.active"
printf '%s\n' keep >"$repo/.tmp/verapdf/tool.jar"
printf '%s\n' keep >"$repo/.tmp/gopmgr-pades-test/signed-sample.pdf"
printf '%s\n' keep >"$repo/.tmp/unknown directory/sentinel"
printf '%s\n' keep >"$repo/.tmp/coverage-go-default.out"
newline_name="$repo/.tmp/name
with-newline"
printf '%s\n' keep >"$newline_name"

before="$(find "$repo/.tmp" -type f -exec shasum -a 256 {} \; | sort)"
output="$(GOPMGR_REPO_ROOT="$repo" bash "$REPORTER")"
after="$(find "$repo/.tmp" -type f -exec shasum -a 256 {} \; | sort)"
[ "$before" = "$after" ] || fail "report modified fixture content"
grep -Fq "retained-validator-cache" <<<"$output" || fail "validator cache was not classified"
grep -Fq "retained-validation-evidence" <<<"$output" || fail "validation evidence was not classified"
grep -Fq "unclassified-preserve" <<<"$output" || fail "unknown material was not preserved"
grep -Fq "legacy-coverage-output" <<<"$output" || fail "legacy coverage output was not classified neutrally"
grep -Fq "coverage-or-validation-run" <<<"$output" || fail "active or retained run was not classified neutrally"
grep -Fq "report only; no files removed" <<<"$output" || fail "read-only contract was not reported"

outside="$FIXTURE/outside"
mkdir -p "$outside"
printf '%s\n' external >"$outside/sentinel"
symlink_repo="$FIXTURE/symlink-repo"
make_repo "$symlink_repo"
rm -rf "$symlink_repo/.tmp"
ln -s "$outside" "$symlink_repo/.tmp"
if GOPMGR_REPO_ROOT="$symlink_repo" bash "$REPORTER" >/dev/null 2>&1; then
	fail "symlinked scratch root was accepted"
fi
grep -Fqx external "$outside/sentinel" || fail "external symlink target changed"

wrong_repo="$FIXTURE/wrong-repo"
mkdir -p "$wrong_repo/.tmp"
printf '%s\n' 'module unrelated' >"$wrong_repo/go.mod"
printf '%s\n' 'help:' >"$wrong_repo/Makefile"
if GOPMGR_REPO_ROOT="$wrong_repo" bash "$REPORTER" >/dev/null 2>&1; then
	fail "unrelated module root was accepted"
fi
if GOPMGR_REPO_ROOT=/ bash "$REPORTER" >/dev/null 2>&1; then
	fail "filesystem root was accepted"
fi

# Simulate a producer removing one entry after find enumerates it. The report
# must continue to later entries, explain the incomplete size, and return
# nonzero without touching unrelated content.
race_repo="$FIXTURE/race-repo"
make_repo "$race_repo"
printf '%s\n' transient >"$race_repo/.tmp/vanishing"
printf '%s\n' preserve >"$race_repo/.tmp/survivor"
cat >"$FIXTURE/fake-du" <<'EOF'
#!/bin/bash
target="${@: -1}"
if [ "$(basename "$target")" = "vanishing" ]; then
	rm -f "$target"
	exit 1
fi
exec du "$@"
EOF
chmod +x "$FIXTURE/fake-du"
set +e
race_output="$(GOPMGR_REPO_ROOT="$race_repo" GOPMGR_HYGIENE_DU_COMMAND="$FIXTURE/fake-du" bash "$REPORTER" 2>&1)"
race_status=$?
set -e
[ "$race_status" -ne 0 ] || fail "incomplete report unexpectedly succeeded"
grep -Fq "survivor" <<<"$race_output" || fail "report aborted before classifying later entries"
grep -Fq "report incomplete" <<<"$race_output" || fail "incomplete report was not explained"
grep -Fqx preserve "$race_repo/.tmp/survivor" || fail "unrelated entry changed during incomplete report"

# Enumeration itself can fail before producing any entries. Its status must
# propagate instead of allowing a successful but incomplete report.
enumeration_repo="$FIXTURE/enumeration-repo"
make_repo "$enumeration_repo"
printf '%s\n' preserve >"$enumeration_repo/.tmp/sentinel"
cat >"$FIXTURE/fake-find" <<'EOF'
#!/bin/bash
echo "seeded enumeration failure" >&2
exit 77
EOF
chmod +x "$FIXTURE/fake-find"
set +e
enumeration_output="$(
	GOPMGR_REPO_ROOT="$enumeration_repo" \
		GOPMGR_HYGIENE_FIND_COMMAND="$FIXTURE/fake-find" \
		bash "$REPORTER" 2>&1
)"
enumeration_status=$?
set -e
[ "$enumeration_status" -ne 0 ] || fail "enumeration failure unexpectedly succeeded"
grep -Fq "seeded enumeration failure" <<<"$enumeration_output" || fail "enumeration diagnostic was hidden"
grep -Fq "report incomplete" <<<"$enumeration_output" || fail "enumeration failure was not explained"
grep -Fqx preserve "$enumeration_repo/.tmp/sentinel" || fail "enumeration failure modified scratch content"

echo "workspace-hygiene tests passed."
