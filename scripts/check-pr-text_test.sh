#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Prove check-pr-text.sh rejects text that contains the copyright personal
# name and accepts text that does not.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-pr-text.sh"
line="$(grep -m1 '^SPDX-FileCopyrightText:' "$ROOT/AGENTS.md")"
name="$(printf '%s\n' "$line" | sed -n 's/^SPDX-FileCopyrightText: [0-9][0-9][0-9][0-9] \(.*\) and The GoPMgr Contributors$/\1/p')"
if [ -z "$name" ]; then
	echo "FAIL: could not read the copyright personal name" >&2
	exit 1
fi

if ! printf '%s\n' "Chart revisions follow stored fields." | "$CHECK"; then
	echo "FAIL: text without the personal name was rejected" >&2
	exit 1
fi

if printf '%s\n' "Ask ${name} to decide." | "$CHECK"; then
	echo "FAIL: text with the personal name was accepted" >&2
	exit 1
fi

echo "check-pr-text: self-test passed"
