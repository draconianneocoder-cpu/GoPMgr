#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Memory-safety hardening gate.
#
# Runs a sequence of static checks against the Go source tree. Each
# check that finds a violation prints the offending lines and the
# script exits non-zero so CI can fail the build.
#
# What this catches:
#   1. go vet . ./internal/... — standard correctness checks
#      (printf format, suspicious assignments, lock copies, ...)
#   2. unsafe.Pointer usage         — forbidden in this codebase
#   3. os.Open without nearby Close — likely file-handle leak
#   4. sql.Open without nearby Close — likely connection leak
#   5. naked sync.Mutex copies      — `var m = otherMutex` is a bug
#   6. Goroutine inventory          — informational only: every explicit
#      `go func`/`go ident(` call site is listed for human review, not
#      enforced (no ctx-parameter check, no allow-list). Its findings never
#      fail the gate; an incomplete scan does.
#
# Checks 2, 4, 5 and 6 are text heuristics over PMF_DIRS. Each search runs
# to completion or fails the gate: grep exit status 0 (matches) and 1 (none)
# are results, anything higher (an unreadable file, a missing root) is an
# operational error, and grep's own diagnostic stays visible.
#
# Optional tools (auto-detected; skipped silently if absent):
#   staticcheck . ./internal/...
#   gosec . ./internal/...
#   govulncheck . ./internal/...
#
# Optional tools are advisory by default because not every developer
# or CI image has the same optional scanner set installed. Set
# GOPMGR_STRICT_OPTIONAL_SCANS=1 to make optional scanner findings
# fail this gate.

set -eu
cd "$(dirname "$0")/.."

# Colour helpers (no-op when not a TTY).
if [ -t 1 ]; then
	RED='\033[0;31m'
	YELLOW='\033[1;33m'
	GREEN='\033[0;32m'
	NC='\033[0m'
else
	RED=''
	YELLOW=''
	GREEN=''
	NC=''
fi

# Scope: only scan GoPMgr's own source tree. Other directories
# (vendored libraries, sibling repos accidentally cloned at the root)
# are explicitly excluded so the scan stays focused.
PMF_DIRS=(./main.go ./internal ./tools)
GO_PACKAGES=(. ./internal/... ./tools/update-manifest ./tools/os-open-scan)
NATIVE_ENV=(python3 scripts/native-go-env.py go --)
GO_TAGS="${GOPMGR_GO_TAGS:-production}"

fail=0
scan_tmp=$(mktemp -d)
trap 'rm -rf "$scan_tmp"' EXIT

section() {
	printf '\n%b== %s ==%b\n' "$YELLOW" "$1" "$NC"
}
ok() {
	printf '%bOK%b: %s\n' "$GREEN" "$NC" "$1"
}
fail_msg() {
	printf '%bFAIL%b: %s\n' "$RED" "$NC" "$1"
	fail=1
}
# run_grep OUT ARGS... writes grep's output to OUT. It succeeds when grep
# exits 0 or 1 and fails on any operational error (exit status above 1).
run_grep() {
	local out=$1
	shift
	local status=0
	grep "$@" >"$out" || status=$?
	[ "$status" -le 1 ]
}

# grep_sources OUT ARGS... searches every Go file under PMF_DIRS, always
# printing file names (-H) and skipping vendored code by directory name.
# Roots are checked first: BSD grep applies --include to command-line
# operands, so it skips a missing root without an error.
grep_sources() {
	local out=$1
	shift
	local root
	for root in "${PMF_DIRS[@]}"; do
		if [ ! -e "$root" ]; then
			printf 'memory-safety-scan: source root %s does not exist\n' "$root" >&2
			return 1
		fi
	done
	run_grep "$out" -rnH --exclude-dir=vendor --include='*.go' "$@" "${PMF_DIRS[@]}"
}

optional_fail_msg() {
	if [ "${GOPMGR_STRICT_OPTIONAL_SCANS:-0}" = "1" ]; then
		fail_msg "$1"
	else
		printf '%bwarn%b: %s (set GOPMGR_STRICT_OPTIONAL_SCANS=1 to fail on optional scanner findings)\n' "$YELLOW" "$NC" "$1"
	fi
}

# -------------------------------------------------------------------
# 1. go vet
# -------------------------------------------------------------------
section "go vet -tags $GO_TAGS ${GO_PACKAGES[*]}"
if command -v go >/dev/null 2>&1; then
	if "${NATIVE_ENV[@]}" go vet -tags "$GO_TAGS" "${GO_PACKAGES[@]}" 2>&1; then
		ok "go vet clean"
	else
		fail_msg "go vet reported issues (see above)"
	fi
else
	fail_msg "go not in PATH; required checks cannot complete"
fi

# -------------------------------------------------------------------
# 2. unsafe.Pointer is forbidden
# -------------------------------------------------------------------
section "unsafe.Pointer ban"
if ! grep_sources "$scan_tmp/unsafe" 'unsafe\.Pointer'; then
	fail_msg "unsafe.Pointer scan could not complete (see grep error above)"
elif [ -s "$scan_tmp/unsafe" ]; then
	fail_msg "unsafe.Pointer use is forbidden — refactor or document"
	cat "$scan_tmp/unsafe"
else
	ok "no unsafe.Pointer usage"
fi

# -------------------------------------------------------------------
# 3. os.Open without nearby defer Close
# -------------------------------------------------------------------
# Heuristic: for every line `os.Open(`, scan the following 10 lines
# for `defer .*Close()` or `.Close()`. Misses legitimate one-shot
# uses; flag for human review.
section "os.Open → defer Close()"
if "${NATIVE_ENV[@]}" go run ./tools/os-open-scan "${PMF_DIRS[@]}"; then
	ok "no os.Open findings from the ten-line Close heuristic"
else
	fail_msg "os.Open helper reported findings or could not complete (see above)"
fi

# -------------------------------------------------------------------
# 4. sql.Open / DB.Conn use without Close
# -------------------------------------------------------------------
section "sql.Open → defer Close()"
# For each match, look at the match line and the 29 after it for either
# Close() on the returned handle or the caller storing it in a struct that
# owns Close (Database, Store).
sql_scan_ok=1
: >"$scan_tmp/sql-findings"
if ! grep_sources "$scan_tmp/sql" 'sql\.Open('; then
	sql_scan_ok=0
else
	while IFS=: read -r f n _; do
		case "$n" in
		'' | *[!0-9]*)
			printf 'memory-safety-scan: cannot parse grep match location: %s:%s\n' "$f" "$n" >&2
			sql_scan_ok=0
			continue
			;;
		esac
		if ! window=$(sed -n "${n},$((n + 29))p" "$f"); then
			sql_scan_ok=0
			continue
		fi
		if ! grep -qE '(Close\(\)|&Database\{|&Store\{|return.*Database|return.*Store)' <<<"$window"; then
			printf '%s:%s: sql.Open without nearby Close\n' "$f" "$n" >>"$scan_tmp/sql-findings"
		fi
	done <"$scan_tmp/sql"
fi
if [ "$sql_scan_ok" -ne 1 ]; then
	fail_msg "sql.Open scan could not complete (see error above)"
elif [ -s "$scan_tmp/sql-findings" ]; then
	fail_msg "sql.Open call sites missing Close()"
	cat "$scan_tmp/sql-findings"
else
	ok "every sql.Open is fed to a struct that owns Close"
fi

# -------------------------------------------------------------------
# 5. Mutex copies (sync.Mutex/RWMutex must not be copied by value)
# -------------------------------------------------------------------
section "no value-copies of sync.Mutex / RWMutex"
# go vet already catches these but we double-check by looking for
# explicit assignments like `m := other.mu`.
if ! grep_sources "$scan_tmp/mutex-raw" -E '(:=|=)[[:space:]]*[a-zA-Z_]+\.mu([^.]|$)' ||
	! run_grep "$scan_tmp/mutex" -v '\.mu\.' "$scan_tmp/mutex-raw"; then
	fail_msg "mutex value-copy scan could not complete (see grep error above)"
elif [ -s "$scan_tmp/mutex" ]; then
	fail_msg "possible value-copy of a mutex — review:"
	cat "$scan_tmp/mutex"
else
	ok "no obvious mutex value-copies"
fi

# -------------------------------------------------------------------
# 6. Goroutines explicitly spawned
# -------------------------------------------------------------------
# Match `go <ident>(` or `go func(` at the start of an expression
# (preceded by whitespace, brace, paren, or semicolon) so we don't
# trip on substrings inside comments / package names like `gofpdf`.
section "goroutine inventory (informational)"
if ! grep_sources "$scan_tmp/go-raw" -E '(^|[[:space:]{(;])go (func|[A-Za-z_][A-Za-z0-9_]*\()' ||
	! run_grep "$scan_tmp/go" -vE ':[[:space:]]*//' "$scan_tmp/go-raw"; then
	fail_msg "goroutine inventory could not complete (see grep error above)"
elif [ -s "$scan_tmp/go" ]; then
	printf '%bgoroutines detected%b (must be reviewed):\n' "$YELLOW" "$NC"
	cat "$scan_tmp/go"
else
	ok "no explicit goroutines (Wails runtime is the only spawner)"
fi

# -------------------------------------------------------------------
# 7. Optional: staticcheck
# -------------------------------------------------------------------
section "staticcheck (optional)"
if command -v staticcheck >/dev/null 2>&1; then
	if "${NATIVE_ENV[@]}" staticcheck "${GO_PACKAGES[@]}"; then
		ok "staticcheck clean"
	else
		optional_fail_msg "staticcheck reported issues"
	fi
else
	printf '%bskip%b: staticcheck not installed\n' "$YELLOW" "$NC"
	printf "  Install:  go install honnef.co/go/tools/cmd/staticcheck@latest\n"
fi

# -------------------------------------------------------------------
# 8. Optional: gosec
# -------------------------------------------------------------------
section "gosec (optional)"
if command -v gosec >/dev/null 2>&1; then
	if "${NATIVE_ENV[@]}" gosec -quiet "${GO_PACKAGES[@]}"; then
		ok "gosec clean"
	else
		optional_fail_msg "gosec flagged security issues"
	fi
else
	printf '%bskip%b: gosec not installed\n' "$YELLOW" "$NC"
fi

# -------------------------------------------------------------------
# 9. Optional: govulncheck (known vulns in dependencies)
# -------------------------------------------------------------------
section "govulncheck (optional)"
if command -v govulncheck >/dev/null 2>&1; then
	if "${NATIVE_ENV[@]}" govulncheck "${GO_PACKAGES[@]}"; then
		ok "no known vulnerabilities"
	else
		optional_fail_msg "govulncheck found vulnerabilities"
	fi
else
	printf '%bskip%b: govulncheck not installed\n' "$YELLOW" "$NC"
fi

# -------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------
echo
if [ "$fail" -ne 0 ]; then
	printf '%bMemory-safety gate FAILED.%b See findings above.\n' "$RED" "$NC"
	exit 1
fi
printf '%bMemory-safety gate PASSED.%b\n' "$GREEN" "$NC"
