#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

# GOPMGR_REPO_ROOT makes the check testable against isolated fixtures. Normal
# callers intentionally resolve from this script so the result is independent
# of the shell's current working directory.
repo_root="${GOPMGR_REPO_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
fail=0

report_missing() {
	echo "check-wails-version: $1 must contain: $2" >&2
	fail=1
}

require_contains() {
	local file=$1
	local expected=$2
	if ! grep -Fq -- "$expected" "$repo_root/$file"; then
		report_missing "$file" "$expected"
	fi
}

require_not_contains() {
	local file=$1
	local forbidden=$2
	if grep -Fq -- "$forbidden" "$repo_root/$file"; then
		echo "check-wails-version: $file must not contain: $forbidden" >&2
		fail=1
	fi
}

# go.mod is the runtime source of truth. Workflows, developer instructions,
# install docs, and failure guidance must install the matching CLI because a
# newer arbitrary Wails CLI can change generated bindings or packaging output.
module_version="$(
	awk '
		$1 == "github.com/wailsapp/wails/v2" { print $2; exit }
		$1 == "require" && $2 == "github.com/wailsapp/wails/v2" { print $3; exit }
	' "$repo_root/go.mod"
)"
if [[ ! "$module_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "check-wails-version: cannot determine a stable Wails module version from go.mod." >&2
	exit 1
fi

install_command="go install github.com/wailsapp/wails/v2/cmd/wails@${module_version}"
install_surfaces=(
	.github/workflows/ci.yml
	.github/workflows/release.yml
	README.md
	docs/INSTALL.md
	DEVELOPER_HANDBOOK.md
	scripts/check-release.sh
	scripts/package-macos-installer.sh
)
for file in "${install_surfaces[@]}"; do
	require_contains "$file" "$install_command"
	require_not_contains "$file" "github.com/wailsapp/wails/v2/cmd/wails@latest"
done

require_contains README.md "Wails: the project uses Wails ${module_version}."
require_contains AGENTS.md "Wails ${module_version}."
require_contains DEPENDENCIES.md "Wails: ${module_version}."
require_contains DEVELOPER_HANDBOOK.md "**Wails ${module_version}**"
require_contains docs/branding.md "Wails ${module_version%.*}"
require_contains build/windows/installer/project.nsi "GoPMgr-owned Wails ${module_version%.*} NSIS entrypoint"

if [[ "${GOPMGR_REQUIRE_WAILS_CLI:-0}" == "1" ]]; then
	if ! command -v wails >/dev/null 2>&1; then
		echo "check-wails-version: installed Wails CLI is required; install with: $install_command" >&2
		fail=1
	else
		installed_version="$(wails version 2>/dev/null | awk '/^v[0-9]+\.[0-9]+\.[0-9]+$/ { print; exit }')"
		if [[ "$installed_version" != "$module_version" ]]; then
			echo "check-wails-version: installed Wails CLI is ${installed_version:-unknown}; go.mod requires ${module_version}." >&2
			echo "check-wails-version: install with: $install_command" >&2
			fail=1
		fi
	fi
fi

if (( fail != 0 )); then
	exit "$fail"
fi

if [[ "${GOPMGR_REQUIRE_WAILS_CLI:-0}" == "1" ]]; then
	echo "check-wails-version: runtime, installed CLI, and current documentation use ${module_version}."
else
	echo "check-wails-version: runtime, declared CLI pins, and current documentation use ${module_version}."
fi
