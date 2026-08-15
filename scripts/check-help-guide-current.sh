#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

# HelpGuide is a shell; its rendered sections live in the sibling help/
# components. Check the complete in-app guide, not only the shell, so a
# refactor cannot make required text look absent.
help_files=(
	"frontend/src/lib/components/HelpGuide.svelte"
	frontend/src/lib/components/help/*.svelte
)
help_label="Help Guide components"

fail() {
	echo "check-help-guide-current: $*" >&2
	exit 1
}

require_contains() {
	local pattern=$1
	if ! rg -q --fixed-strings "$pattern" "${help_files[@]}"; then
		fail "$help_label must mention: $pattern"
	fi
}

require_not_contains() {
	local pattern=$1
	if rg -q --fixed-strings "$pattern" "${help_files[@]}"; then
		fail "$help_label must not contain stale text: $pattern"
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

# 2026-07 user-guide overhaul: keep the new load-bearing sections present.
require_contains "Quick Start: Your First Project"
require_contains "Keyboard Shortcuts"
require_contains "Troubleshooting & FAQ"
require_contains "Command-Line Maintenance"
require_contains "Search help sections"
require_contains "Tornado Ranking"

require_not_contains "libwebkit2gtk-4.0-dev"
require_not_contains "Ubuntu 22.04"
require_not_contains "WebKit2GTK 4.0"

# ── Route coverage ──────────────────────────────────────────────────────
# Every view routed in App.svelte's routeLoaders must be mentioned in the
# Help Guide, so shipping a new view without documenting it fails CI. The
# default expectation is the route id with underscores as spaces (matched
# case-insensitively); routes whose guide wording differs get an explicit
# phrase in route_phrase below. When this check fails for a new route,
# either document the view in the Help Guide components or add a mapping here that
# points at the wording the guide actually uses.
app_file="frontend/src/App.svelte"

route_phrase() {
	case "$1" in
		recovery_reset) printf '%s\n' "recovery code" ;;
		burnup) printf '%s\n' "burn-up" ;;
		burndown) printf '%s\n' "burn-down" ;;
		cause_effect) printf '%s\n' "Cause-and-Effect Diagram" ;;
		sigma_dashboard) printf '%s\n' "DMAIC Pack" ;;
		*) printf '%s\n' "${1//_/ }" ;;
	esac
}

routes="$(sed -n '/const routeLoaders/,/^  };/p' "$app_file" | { grep -oE '^    [a-z_]+:' || true; } | tr -d ' :')"
route_count="$(printf '%s\n' "$routes" | { grep -c . || true; })"
if [ "$route_count" -lt 30 ]; then
	fail "route extraction from $app_file found only $route_count routes — the routeLoaders pattern has likely changed; update this script"
fi

for route in $routes; do
	phrase="$(route_phrase "$route")"
	if ! rg -qi --fixed-strings "$phrase" "${help_files[@]}"; then
		fail "route '$route' is not documented: $help_label has no mention of '$phrase' (document the view or map it in route_phrase)"
	fi
done

echo "check-help-guide-current: HelpGuide covers recent release corrections and all $route_count routed views."
