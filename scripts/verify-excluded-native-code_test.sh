#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Regression cases for scripts/verify-excluded-native-code.sh: synthetic
# binaries and trees, every marker in the committed list, and a positive
# control against the real TPC-H archive in the pinned DuckDB module.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERIFY="$ROOT/scripts/verify-excluded-native-code.sh"
LIST="$ROOT/third_party/EXCLUDED-NATIVE-CODE.txt"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

work="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-verify-excluded-native-code-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# run <name> <expected-exit> <expected-output-substring> [args...]
run() {
	local name="$1" want_status="$2" want_text="$3" status=0 out
	shift 3
	out="$(bash "$VERIFY" "$@" 2>&1)" || status=$?
	if [ "$status" != "$want_status" ]; then
		printf '%s\n' "$out" >&2
		fail "$name: exit $status, want $want_status"
	fi
	if ! grep -Fq -- "$want_text" <<< "$out"; then
		printf '%s\n' "$out" >&2
		fail "$name: output lacks \"$want_text\""
	fi
	echo "ok: $name"
}

clean="$work/clean"
# DuckDB core keeps extension names for autoloading; they are not markers.
printf 'ELF\0duckdb core\0dbgen\0dsdgen\0tpch_queries\0tpcds\0' > "$clean"
run "clean binary passes, autoload names are not markers" 0 "contains none of the 8 excluded-code markers" "$clean"
run "several clean binaries pass" 0 "contains none" "$clean" "$clean"

# Every marker in the committed list must fail a binary on its own.
count=0
while IFS= read -r marker; do
	count=$((count + 1))
	bin="$work/marker-$count"
	{ cat "$clean"; printf '%s\0' "$marker"; cat "$clean"; } > "$bin"
	run "marker $count fails a binary" 1 "  $marker" "$bin"
done <<< "$(tr -d '\r' < "$LIST" | sed -n 's/^marker //p')"
[ "$count" -eq 8 ] || fail "the committed list has $count markers, want 8"
run "one bad binary among clean ones fails" 1 "do not distribute this build" "$clean" "$work/marker-1" "$clean"

run "no arguments" 2 "usage:"
run "missing binary" 1 "binary not found" "$work/nope"
run "--tree without a directory" 2 "usage:" --tree
run "--tree with a missing directory" 1 "not a directory" --tree "$work/nope"

tree="$work/tree"
mkdir -p "$tree/github.com/duckdb/duckdb-go-bindings/lib/linux-amd64"
cp "$clean" "$tree/github.com/duckdb/duckdb-go-bindings/lib/linux-amd64/libduckdb_bundle.a"
run "clean tree passes" 0 "holds none of the excluded archives" --tree "$tree"
: > "$tree/github.com/duckdb/duckdb-go-bindings/lib/linux-amd64/libtpcds_extension.a"
run "an excluded archive fails a tree, even empty" 1 "holds excluded archive" --tree "$tree"
rm "$tree/github.com/duckdb/duckdb-go-bindings/lib/linux-amd64/libtpcds_extension.a"
cp "$work/marker-3" "$tree/renamed.o"
run "a renamed file with a marker fails a tree" 1 "renamed.o contains excluded code markers" --tree "$tree"

printf 'archive liba.a\r\nmarker CRLF-MARK\r\n' > "$work/crlf.txt"
printf 'xx CRLF-MARK xx' > "$work/crlf-bin"
run "a CRLF list still matches" 1 "  CRLF-MARK" --list "$work/crlf.txt" "$work/crlf-bin"
printf '# only comments\narchive liba.a\n' > "$work/nomarker.txt"
run "a list without markers fails" 1 "names no marker or no archive" --list "$work/nomarker.txt" "$clean"
run "a missing list fails" 1 "exclusion list missing or empty" --list "$work/none.txt" "$clean"
run "--list without a file" 2 "usage:" --list

# Tool failures must not turn incomplete scans into clean results.
printf 'archive liba.a\nmarker m\nunknown value\n' > "$work/malformed.txt"
run "malformed exclusion entry" 1 "invalid exclusion entry" --list "$work/malformed.txt" "$clean"
mkdir -p "$work/fail-grep" "$work/partial-grep" "$work/fail-find" "$work/fail-sort"
for mode in fail-grep partial-grep; do
	cat > "$work/$mode/grep" <<'MOCK'
#!/bin/bash
if [ "${1:-}" = -a ]; then
	case "$0" in *partial*) printf 'partial output\n' ;; esac
	exit 2
fi
exec /usr/bin/grep "$@"
MOCK
	chmod +x "$work/$mode/grep"
done
printf '#!/bin/bash\nexit 1\n' > "$work/fail-find/find"
printf '#!/bin/bash\nprintf "partial output\\n"\nexit 2\n' > "$work/fail-sort/sort"
chmod +x "$work/fail-find/find" "$work/fail-sort/sort"
PATH="$work/fail-grep:$PATH" run "binary scan error" 1 "scan failed" "$clean"
PATH="$work/partial-grep:$PATH" run "binary scan error after partial output" 1 "scan failed" "$clean"
PATH="$work/fail-find:$PATH" run "tree traversal error" 1 "tree traversal failed" --tree "$work/tree"
PATH="$work/fail-sort:$PATH" run "sort failure after partial output" 1 "sorting scan results failed" "$clean"
ln -s "$clean" "$tree/link"
run "linked tree entry is not silently skipped" 1 "nonregular tree entry" --tree "$tree"
rm "$tree/link"

# Positive control: the real excluded archive in the pinned module.
module="github.com/duckdb/duckdb-go-bindings/lib/linux-amd64"
dir="$(cd "$ROOT" && go mod download -json "$module" | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",$/\1/p')"
[ -n "$dir" ] || fail "go mod download $module reported no directory"
run "the real TPC-H archive fails" 1 "N6duckdb13TpchExtensionE" "$dir/libtpch_extension.a"
run "the real TPC-DS archive fails" 1 "Transaction Processing Performance Council (TPC)" "$dir/libtpcds_extension.a"
run "the pinned module tree fails" 1 "holds excluded archive" --tree "$dir"

echo "verify-excluded-native-code_test: all cases passed"
