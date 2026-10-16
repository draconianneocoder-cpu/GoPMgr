#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Regression coverage for command-scope matching. Use disposable clones so
# the release gate sees a real repository without touching this worktree.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/release-gate-scope-check.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-release-scope-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
	echo "release-gate-scope-check test: $*" >&2
	exit 1
}

new_fixture() {
	local name="$1"
	local fixture="$TEST_ROOT/$name"
	git clone -q --no-hardlinks "$ROOT" "$fixture"
	cp "$CHECK" "$fixture/scripts/release-gate-scope-check.sh"
	cp "$ROOT/Makefile" "$fixture/Makefile"
	cp "$ROOT/scripts/check-release.sh" "$fixture/scripts/check-release.sh"
	cp "$ROOT/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
	cp "$ROOT/scripts/check-coverage-ledger-drift.sh" "$fixture/scripts/check-coverage-ledger-drift.sh"
	cp "$ROOT/scripts/check-help-guide-current.sh" "$fixture/scripts/check-help-guide-current.sh"
	cp "$ROOT/scripts/check-release-publication-mode.sh" "$fixture/scripts/check-release-publication-mode.sh"
	cp "$ROOT/scripts/check-release-publication-mode_test.sh" "$fixture/scripts/check-release-publication-mode_test.sh"
	cp "$ROOT/scripts/release-publication-flag.sh" "$fixture/scripts/release-publication-flag.sh"
	cp "$ROOT/scripts/release-update-channel.sh" "$fixture/scripts/release-update-channel.sh"
	mkdir -p "$fixture/frontend/dist"
	: >"$fixture/frontend/dist/index.html"
	printf '%s\n' "$fixture"
}

expect_failure() {
	local fixture="$1"
	local expected="$2"
	local output
	if output="$(GOPMGR_RELEASE_SCOPE_SKIP_SELF_TEST=1 bash "$fixture/scripts/release-gate-scope-check.sh" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	case "$output" in
	*"$expected"*) ;;
	*) fail "failure did not contain '$expected': $output" ;;
	esac
}

baseline="$(new_fixture baseline)"
GOPMGR_RELEASE_SCOPE_SKIP_SELF_TEST=1 \
	bash "$baseline/scripts/release-gate-scope-check.sh" >/dev/null ||
	fail "valid release scope was rejected"

historical_prose="$(new_fixture historical-prose)"
printf '%s\n' 'Historical evidence: go test ./... passed before this gate existed.' >>"$historical_prose/docs/DEVELOPER_HANDBOOK.md"
GOPMGR_RELEASE_SCOPE_SKIP_SELF_TEST=1 \
	bash "$historical_prose/scripts/release-gate-scope-check.sh" >/dev/null ||
	fail "historical handbook prose was treated as a live quality gate"

unscoped_command="$(new_fixture unscoped-command)"
printf '%s\n' 'quality-check:' $'\tgo test ./...' >>"$unscoped_command/Makefile"
expect_failure "$unscoped_command" "Go quality gates must use explicit first-party package roots"

unscoped_script="$(new_fixture unscoped-script)"
printf '%s\n' 'go test ./...' >"$unscoped_script/scripts/quality-check.sh"
expect_failure "$unscoped_script" "Go quality gates must use explicit first-party package roots"

unscoped_coverage_script="$(new_fixture unscoped-coverage-script)"
printf '%s\n' 'govulncheck ./...' >>"$unscoped_coverage_script/scripts/check-coverage-ledger-drift.sh"
expect_failure "$unscoped_coverage_script" "Go quality gates must use explicit first-party package roots"

unscoped_ledger_test="$(new_fixture unscoped-ledger-test-trailing-cover)"
perl -0pi -e 's/if ! go test -cover \. \.\/internal\/\.\.\. \.\/scripts \.\/tools\/\.\.\./if ! go test .\/... -cover/' \
	"$unscoped_ledger_test/scripts/check-coverage-ledger-drift.sh"
expect_failure "$unscoped_ledger_test" "Go quality gates must use explicit first-party package roots"

unscoped_ledger_test="$(new_fixture unscoped-ledger-test-leading-cover)"
perl -0pi -e 's/if ! go test -cover \. \.\/internal\/\.\.\. \.\/scripts \.\/tools\/\.\.\./if ! go test -cover .\/.../' \
	"$unscoped_ledger_test/scripts/check-coverage-ledger-drift.sh"
expect_failure "$unscoped_ledger_test" "Go quality gates must use explicit first-party package roots"

wrong_pipx_path="$(new_fixture wrong-pipx-path)"
perl -0pi -e 's#pipx environment --value PIPX_BIN_DIR >> "\$GITHUB_PATH"#echo "\$\(python3 -m site --user-base\)/bin" >> "\$GITHUB_PATH"#' \
	"$wrong_pipx_path/.github/workflows/release.yml"
expect_failure "$wrong_pipx_path" "Release preflight must add pipx's configured binary directory"

race_policy="$(new_fixture race-policy)"
for entrypoint in Makefile scripts/check-release.sh; do
	for mutation in package-overlap no-race cached filtered short conflicting-limit scope ignored-failure duplicate duplicate-scope exported-scope missing-entrypoint; do
		cp "$ROOT/Makefile" "$race_policy/Makefile"
		cp "$ROOT/scripts/check-release.sh" "$race_policy/scripts/check-release.sh"
		case "$mutation" in
		package-overlap) perl -0pi -e 's/-race -p=1 -count=1/-race -count=1/g' "$race_policy/$entrypoint" ;;
		no-race) perl -0pi -e 's/test -race -p=1/test -p=1/g' "$race_policy/$entrypoint" ;;
		cached) perl -0pi -e 's/-p=1 -count=1/-p=1/g' "$race_policy/$entrypoint" ;;
		filtered) perl -0pi -e 's/-count=1/-count=1 -run OnlyThis/g' "$race_policy/$entrypoint" ;;
		short) perl -0pi -e 's/-count=1/-count=1 -short/g' "$race_policy/$entrypoint" ;;
		conflicting-limit) perl -0pi -e 's/-count=1/-count=1 -p=2/g' "$race_policy/$entrypoint" ;;
		scope) perl -0pi -e 's{\./internal/\.\.\.}{./internal/users}g' "$race_policy/$entrypoint" ;;
		ignored-failure)
			if [ "$entrypoint" = Makefile ]; then
				perl -0pi -e 's/^(\t)(\$\(GO\) test -race)/$1-$2/m' "$race_policy/$entrypoint"
			else
				perl -0pi -e 's/(Race detector flagged tests[^\n]*\n\s*)exit 1/${1}exit 0/' "$race_policy/$entrypoint"
			fi
			;;
		duplicate)
			if [ "$entrypoint" = Makefile ]; then
				printf '\nrace:\n\t@true\n' >>"$race_policy/$entrypoint"
			else
				printf '\n# --- 6. Race detector ------------------------------------------------\n' >>"$race_policy/$entrypoint"
			fi
			;;
		duplicate-scope)
			if [ "$entrypoint" = Makefile ]; then
				printf '\n GO_PACKAGES := ./internal/users\n' >>"$race_policy/$entrypoint"
			else
				printf '\n GO_PACKAGES="./internal/users"\n' >>"$race_policy/$entrypoint"
			fi
			;;
		exported-scope)
			if [ "$entrypoint" = Makefile ]; then
				printf '\nexport GO_PACKAGES := ./internal/users\n' >>"$race_policy/$entrypoint"
			else
				printf '\nexport GO_PACKAGES="./internal/users"\n' >>"$race_policy/$entrypoint"
			fi
			;;
		missing-entrypoint)
			if [ "$entrypoint" = Makefile ]; then
				perl -0pi -e 's/^race:/race-not-real:/m' "$race_policy/$entrypoint"
			else
				perl -0pi -e 's/^# --- 6\. Race detector[^\n]*\n//m' "$race_policy/$entrypoint"
			fi
			;;
		esac
		expect_failure "$race_policy" 'race entrypoints must preserve uncached, bounded-package tests'
	done
done

echo "release-gate-scope-check tests passed."
