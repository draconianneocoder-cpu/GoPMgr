#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Regression cases for scripts/verify-module-notices.sh. A stub `go` serves
# canned `go version -m` output, so the cases need no network and no real
# GoPMgr build; one case uses a real Go binary and the committed notices file.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERIFY="$ROOT/scripts/verify-module-notices.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

work="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-verify-module-notices-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT

real_go="$(command -v go)" || fail "this test needs a Go toolchain on PATH"
mkdir -p "$work/bin"
cat > "$work/bin/go" << 'STUB'
#!/bin/bash
if [ "${1:-}" = version ] && [ "${2:-}" = -m ] && [ -n "${FAKE_GO_VERSION_M:-}" ]; then
	[ "$FAKE_GO_VERSION_M" = fail ] && exit 1
	cat "$FAKE_GO_VERSION_M"
	exit 0
fi
exec "$REAL_GO" "$@"
STUB
chmod +x "$work/bin/go"

binary="$work/gopmgr"
: > "$binary"

# What `go version -m` prints for a binary with one plain and one replaced
# dependency: tab-indented fields, and an "=>" line under the replaced one.
printf '%s\n' \
	"$binary: go1.27.3" \
	$'\tpath\tgopmgr' \
	$'\tmod\tgopmgr\t(devel)\t' \
	$'\tdep\texample.com/a\tv1.0.0\th1:abc=' \
	$'\tdep\texample.com/c/v4\tv4.4.2' \
	$'\t=>\t./third_party/c\t\t' \
	$'\tbuild\t-tags=desktop,production' > "$work/metadata.txt"

notices() {
	printf '%s\n' "Go module notices for GoPMgr" "" "$@" > "$work/notices.txt"
}

# run <name> <expected-exit> <expected-output-substring> [args...]
run() {
	local name="$1" want_status="$2" want_text="$3" status=0 out
	shift 3
	out="$(PATH="$work/bin:$PATH" REAL_GO="$real_go" FAKE_GO_VERSION_M="${FAKE:-$work/metadata.txt}" \
		bash "$VERIFY" "$@" 2>&1)" || status=$?
	if [ "$status" != "$want_status" ]; then
		printf '%s\n' "$out" >&2
		fail "$name: exit $status, want $want_status"
	fi
	case "$out" in
	*"$want_text"*) ;;
	*)
		printf '%s\n' "$out" >&2
		fail "$name: output lacks '$want_text'"
		;;
	esac
}

notices "Module: std" "Module: example.com/a v1.0.0" "Module: example.com/c/v4 v4.4.2"
run "complete notices" 0 "all 2 modules linked" "$binary" "$work/notices.txt"

printf '%s\r\n' "Go module notices for GoPMgr" "" "Module: std" "Module: example.com/a v1.0.0" \
	"Module: example.com/c/v4 v4.4.2" > "$work/notices.txt"
run "CRLF line endings" 0 "all 2 modules linked" "$binary" "$work/notices.txt"

notices "Module: std" "Module: example.com/c/v4 v4.4.2"
run "a missing module" 1 "  example.com/a v1.0.0" "$binary" "$work/notices.txt"

notices "Module: std" "Module: example.com/a v1.0.1" "Module: example.com/c/v4 v4.4.2"
run "a version mismatch" 1 "  example.com/a v1.0.0" "$binary" "$work/notices.txt"

notices "Module: std" "Module: example.com/a v1.0.0-20260101000000-abcdef123456" "Module: example.com/c/v4 v4.4.2"
run "a version with the same prefix" 1 "  example.com/a v1.0.0" "$binary" "$work/notices.txt"

notices "Module: std" "Module: example.com/a" "Module: example.com/c/v4 v4.4.2"
run "a module without its version" 1 "  example.com/a v1.0.0" "$binary" "$work/notices.txt"

notices "Module: example.com/a v1.0.0" "Module: example.com/c/v4 v4.4.2"
run "no standard library section" 1 "no Go standard library section" "$binary" "$work/notices.txt"

notices "Module: std" "Module: example.com/a v1.0.0" "Module: example.com/c/v4 v4.4.2"
FAKE=fail run "a binary without build information" 1 "carries no Go build information" "$binary" "$work/notices.txt"

run "a missing notices file" 1 "notices file missing or empty" "$binary" "$work/absent.txt"
: > "$work/empty.txt"
run "an empty notices file" 1 "notices file missing or empty" "$binary" "$work/empty.txt"
run "a missing binary" 1 "binary not found" "$work/absent-binary" "$work/notices.txt"
run "no arguments" 2 "usage:"

# Complete notices do not excuse linked TPC code: the exclusion check runs
# on every binary this script accepts.
run "a clean binary passes the exclusion check" 0 "contains none of the 8 excluded-code markers" \
	"$binary" "$work/notices.txt"
printf 'ELF\0N6duckdb13TpchExtensionE\0' > "$work/tpc-binary"
run "a binary carrying excluded TPC code" 1 "contains excluded code" "$work/tpc-binary" "$work/notices.txt"

# A binary linking a DuckDB native library module also needs the DuckDB
# native notices to cover that module at that version.
printf '%s\n' "$(cat "$work/metadata.txt")" \
	$'\tdep\tgithub.com/duckdb/duckdb-go-bindings/lib/linux-amd64\tv0.10506.0\th1:d=' > "$work/metadata-duckdb.txt"
notices "Module: std" "Module: example.com/a v1.0.0" "Module: example.com/c/v4 v4.4.2" \
	"Module: github.com/duckdb/duckdb-go-bindings/lib/linux-amd64 v0.10506.0"
printf '%s\n' "DuckDB native library notices" \
	"Covers: github.com/duckdb/duckdb-go-bindings/lib/linux-amd64 v0.10506.0" > "$work/native.txt"
FAKE="$work/metadata-duckdb.txt" run "DuckDB native notices that cover the library" 0 \
	"covers DuckDB's native libraries (github.com/duckdb/duckdb-go-bindings/lib/linux-amd64 v0.10506.0)" \
	"$binary" "$work/notices.txt" "$work/native.txt"
printf '%s\n' "DuckDB native library notices" \
	"Covers: github.com/duckdb/duckdb-go-bindings/lib/linux-amd64 v0.10505.0" > "$work/native.txt"
FAKE="$work/metadata-duckdb.txt" run "DuckDB native notices for another version" 1 \
	"does not cover github.com/duckdb/duckdb-go-bindings/lib/linux-amd64 v0.10506.0" \
	"$binary" "$work/notices.txt" "$work/native.txt"
FAKE="$work/metadata-duckdb.txt" run "no DuckDB native notices" 1 \
	"links DuckDB's native libraries, but" "$binary" "$work/notices.txt" "$work/absent-native.txt"
notices "Module: std" "Module: example.com/a v1.0.0" "Module: example.com/c/v4 v4.4.2"
run "no DuckDB library, no native notices needed" 0 "all 2 modules linked" \
	"$binary" "$work/notices.txt" "$work/absent-native.txt"

# A real Go binary checked against the committed notices file, with the real
# `go version -m`: proves the parsing matches the toolchain's actual format.
mkdir -p "$work/src"
printf 'package main\n\nfunc main() {}\n' > "$work/src/main.go"
(cd "$work/src" && CGO_ENABLED=0 GOFLAGS= "$real_go" build -o "$work/real" main.go) ||
	fail "could not build the real Go binary fixture"
out="$(bash "$VERIFY" "$work/real" 2>&1)" || {
	printf '%s\n' "$out" >&2
	fail "a real binary against the committed notices file was refused"
}
case "$out" in
*"all 0 modules linked"*) ;;
*)
	printf '%s\n' "$out" >&2
	fail "a real binary with no dependencies did not report 0 modules"
	;;
esac

echo "verify-module-notices tests passed."
