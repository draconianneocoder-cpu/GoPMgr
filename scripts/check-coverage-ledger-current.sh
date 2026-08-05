#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# TEST_COVERAGE_LEDGER.md is a hand-written inventory (see its own
# "Methodology and confidence" note): it does not validate what each test
# actually proves, only lists what exists. That weaker claim still needs
# one guarantee to be worth anything -- every test file that exists is at
# least mentioned. Without this check, adding a new *_test.go/*.test.ts
# file and forgetting the ledger update is silent: nothing else in the
# repo notices. This is a cheap textual presence check, not a content
# audit; it catches the common drift (new test file, ledger untouched),
# not stale prose within an existing row.

set -euo pipefail
cd "$(dirname "$0")/.."

ledger="TEST_COVERAGE_LEDGER.md"
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
done < <(find . \
	\( -name node_modules -o -path ./frontend/coverage -o -name .git \) -prune -o \
	\( -name '*_test.go' -o -name '*.test.ts' \) -type f -print0)

if [ "$missing" -ne 0 ]; then
	echo "check-coverage-ledger-current: add the file(s) above to $ledger (what/how/why), or run this again after fixing." >&2
	exit 1
fi

echo "check-coverage-ledger-current: every *_test.go/*.test.ts file is mentioned in $ledger."
