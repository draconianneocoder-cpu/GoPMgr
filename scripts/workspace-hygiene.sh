#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Report repository-local generated material without deleting it. .tmp contains
# both disposable scratch and retained validation evidence, so name/age/ignore
# status alone is never deletion authority. Producers own automatic cleanup of
# their successful temporary runs; this report surfaces everything else for a
# deliberate maintainer decision.

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: scripts/workspace-hygiene.sh

Classify repository .tmp entries and report their sizes. This command is
read-only. It never removes scratch, release evidence, caches, or user data.
EOF
}

fail() {
	echo "workspace-hygiene: $*" >&2
	exit 1
}

case "${1:-}" in
"") ;;
-h | --help)
	usage
	exit 0
	;;
*) fail "unknown argument: $1" ;;
esac

ROOT="${GOPMGR_REPO_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
DU_COMMAND="${GOPMGR_HYGIENE_DU_COMMAND:-du}"
FIND_COMMAND="${GOPMGR_HYGIENE_FIND_COMMAND:-find}"
[ -n "$ROOT" ] || fail "repository root is empty"
[ "$ROOT" != "/" ] || fail "refusing filesystem root"
[ -d "$ROOT" ] || fail "repository root is not a directory: $ROOT"
ROOT="$(cd "$ROOT" && pwd -P)"
[ -f "$ROOT/go.mod" ] || fail "repository root is missing go.mod: $ROOT"
[ -f "$ROOT/Makefile" ] || fail "repository root is missing Makefile: $ROOT"
grep -Eq '^module[[:space:]]+gopmgr$' "$ROOT/go.mod" || fail "repository root is not the GoPMgr module: $ROOT"

SCRATCH="$ROOT/.tmp"
if [ -L "$SCRATCH" ]; then
	fail "refusing symlinked scratch root: $SCRATCH"
fi
if [ ! -e "$SCRATCH" ]; then
	echo "workspace-hygiene: .tmp is absent; nothing to classify."
	exit 0
fi
[ -d "$SCRATCH" ] || fail "scratch root is not a directory: $SCRATCH"

classify() {
	local name="$1"
	case "$name" in
	*.lock) echo "active-or-abandoned-lock" ;;
	verapdf) echo "retained-validator-cache" ;;
	gopmgr-pades-test | gopmgr-pdfa-test | gopmgr-pades-trusted-source)
		echo "retained-validation-evidence"
		;;
	coverage-go-default.out | coverage-go-default.pkgs | coverage-go-duckdb.out | coverage-go-duckdb.pkgs | coverage-ledger-drift-default.log | coverage-ledger-drift-duckdb-analytics.log)
		echo "legacy-coverage-output"
		;;
	coverage-go-default.* | coverage-go-duckdb.* | coverage-ledger-drift.* | gopmgr-pades-test.build.*)
		echo "coverage-or-validation-run"
		;;
	*) echo "unclassified-preserve" ;;
	esac
}

incomplete=0
if total_output="$("$DU_COMMAND" -sk "$SCRATCH" 2>/dev/null)"; then
	total_kib="$(awk '{print $1}' <<<"$total_output")"
else
	total_kib="unavailable"
	incomplete=1
fi
echo "workspace-hygiene: .tmp uses ${total_kib:-unknown} KiB"
printf 'classification\tsize-kib\tpath\n'

if "$FIND_COMMAND" "$SCRATCH" -mindepth 1 -maxdepth 1 -print0 | (
	entry_incomplete=0
	while IFS= read -r -d '' entry; do
		name="$(basename "$entry")"
		if entry_output="$("$DU_COMMAND" -sk "$entry" 2>/dev/null)"; then
			entry_kib="$(awk '{print $1}' <<<"$entry_output")"
		else
			entry_kib="unavailable"
			entry_incomplete=1
		fi
		printf '%s\t%s\t%q\n' "$(classify "$name")" "${entry_kib:-unavailable}" "$entry"
	done
	exit "$entry_incomplete"
); then
	:
else
	incomplete=1
fi

if [ "$incomplete" -ne 0 ]; then
	echo "workspace-hygiene: report incomplete; entries could not be enumerated or sized consistently." >&2
	exit 1
fi
echo "workspace-hygiene: report only; no files removed."
