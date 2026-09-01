#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# TEST_COVERAGE_LEDGER.md is a hand-written inventory (see its own
# "Methodology and confidence" note): it does not validate what each test
# actually proves, only lists what exists. That weaker claim still needs
# one guarantee to be worth anything -- every test file that exists is at
# least mentioned. Without this check, adding a new test file and
# forgetting the ledger update is silent: nothing else in the repo
# notices. This is a cheap textual presence check, not a content audit;
# it catches the common drift (new test file, ledger untouched), not
# stale prose within an existing row.
#
# Recognizes five test-file shapes: Go's `*_test.go` (the only shape
# `go test` itself ever discovers -- there is no `.spec.go` equivalent),
# and four frontend shapes -- `*.test.ts`, `*.spec.ts`, `*.test.js`,
# `*.spec.js` -- matching frontend/vitest.config.ts's own `include`
# pattern (`src/**/*.{test,spec}.{ts,js}`). That match is maintained by
# hand, not enforced: if vitest.config.ts's include pattern is ever
# widened further (a new extension, a different suffix), this script's
# find expression needs the same update or it silently stops guaranteeing
# anything about the new shape -- the same pass-by-omission failure mode
# check-coverage-ledger-drift.sh's header describes for its own headings.
#
# GOPMGR_REPO_ROOT overrides the repo root, for
# check-coverage-ledger-current_test.sh to run this against isolated
# fixtures instead of the real repository.

set -euo pipefail
ROOT="${GOPMGR_REPO_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"

ledger="docs/TEST_COVERAGE_LEDGER.md"
if [ ! -f "$ledger" ]; then
	echo "check-coverage-ledger-current: $ledger is missing." >&2
	exit 1
fi

missing=0
while IFS= read -r -d '' f; do
	base="$(basename "$f")"
	if ! grep -qF -- "$base" "$ledger"; then
		echo "check-coverage-ledger-current: $f is not mentioned in $ledger." >&2
		missing=1
	fi
# .claude/worktrees holds full duplicate checkouts from parallel agent
# sessions (git-worktree add), each on its own branch/commit with its own
# TEST_COVERAGE_LEDGER.md. Scanning them here checks a worktree's test
# files against a ledger snapshot that isn't tracking the same tree -- a
# false pass today (subset of current tests) and false failures the
# moment the two diverge, neither of which says anything about this repo.
done < <(find . \
	\( -name node_modules -o -path ./frontend/coverage -o -name .git -o -path ./.claude/worktrees \) -prune -o \
	\( -name '*_test.go' -o -name '*.test.ts' -o -name '*.spec.ts' -o -name '*.test.js' -o -name '*.spec.js' \) -type f -print0)

if [ "$missing" -ne 0 ]; then
	echo "check-coverage-ledger-current: add the file(s) above to $ledger (what/how/why), or run this again after fixing." >&2
	exit 1
fi

echo "check-coverage-ledger-current: every *_test.go/*.{test,spec}.{ts,js} file is mentioned in $ledger."
