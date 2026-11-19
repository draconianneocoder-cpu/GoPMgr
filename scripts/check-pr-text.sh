#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Reject pull request text that addresses the owner by the personal name
# on the SPDX copyright line in AGENTS.md. Read the text on stdin.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
line="$(grep -m1 '^SPDX-FileCopyrightText:' "$ROOT/AGENTS.md")"
name="$(printf '%s\n' "$line" | sed -n 's/^SPDX-FileCopyrightText: [0-9][0-9][0-9][0-9] \(.*\) and The GoPMgr Contributors$/\1/p')"
if [ -z "$name" ]; then
	echo "check-pr-text: AGENTS.md has no SPDX personal name to enforce." >&2
	exit 1
fi

text="$(cat)"
if printf '%s' "$text" | grep -F -q -- "$name"; then
	echo "check-pr-text: remove the copyright personal name from the pull request title and body." >&2
	exit 1
fi
