#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

help_file="frontend/src/lib/components/HelpGuide.svelte"

fail() {
	echo "check-help-guide-current: $*" >&2
	exit 1
}

require_contains() {
	local pattern=$1
	if ! rg -q --fixed-strings "$pattern" "$help_file"; then
		fail "$help_file must mention: $pattern"
	fi
}

require_not_contains() {
	local pattern=$1
	if rg -q --fixed-strings "$pattern" "$help_file"; then
		fail "$help_file must not contain stale text: $pattern"
	fi
}

require_contains "integer minor units"
require_contains "DuckDB-backed"
require_contains "Resource Capacity"
require_contains "weekly capacity and day overrides"
require_contains "dashed capacity lines"
require_contains "over-allocation warnings"
require_contains "Earned Value"
require_contains "libwebkit2gtk-4.1-dev"
require_contains "webkit2_41"
require_contains "GTK4/WebKitGTK 6.0 support requires a future Wails migration"

require_not_contains "libwebkit2gtk-4.0-dev"
require_not_contains "Ubuntu 22.04"
require_not_contains "WebKit2GTK 4.0"

echo "check-help-guide-current: HelpGuide covers recent release corrections."
