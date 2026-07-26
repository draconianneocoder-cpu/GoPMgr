#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Validate that every release-installer tool is immutable at tag build time.
# This is a static contract check: native runner integration still belongs to
# an RC tag, but the source tag must select the same tool versions every time.

set -euo pipefail
ROOT="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
WORKFLOW="$ROOT/.github/workflows/release.yml"
VERSIONS="$ROOT/scripts/release-tool-versions.env"
PACKAGE_LINUX="$ROOT/scripts/package-linux.sh"

fail=0
require_literal() {
	local file="$1"
	local literal="$2"
	local message="$3"
	if ! rg -Fq "$literal" "$file"; then
		echo "installer-tool-pins: $message" >&2
		fail=1
	fi
}

read_version() {
	local name="$1"
	awk -F= -v name="$name" '$1 == name { print $2 }' "$VERSIONS"
}

line_of_literal() {
	local file="$1"
	local literal="$2"
	# awk exits successfully when the literal is absent, allowing the caller to
	# report all contract failures instead of set -e stopping at the first one.
	awk -v literal="$literal" 'index($0, literal) { print NR; exit }' "$file"
}

for required in "$WORKFLOW" "$VERSIONS" "$PACKAGE_LINUX"; do
	if [ ! -f "$required" ]; then
		echo "installer-tool-pins: required file missing: ${required#"$ROOT/"}" >&2
		exit 1
	fi
done

NFPM_VERSION="$(read_version NFPM_VERSION)"
NSIS_VERSION="$(read_version NSIS_VERSION)"

if [[ ! "$NFPM_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "installer-tool-pins: NFPM_VERSION must be a v-prefixed stable semantic version." >&2
	fail=1
fi
if [[ ! "$NSIS_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "installer-tool-pins: NSIS_VERSION must be a stable three-part version." >&2
	fail=1
fi

require_literal "$WORKFLOW" \
	"grep -E '^(NFPM_VERSION|NSIS_VERSION)=' scripts/release-tool-versions.env >> \"\$GITHUB_ENV\"" \
	"workflow must load release-tool-versions.env before installing tools."
require_literal "$WORKFLOW" \
	'go install github.com/goreleaser/nfpm/v2/cmd/nfpm@"${NFPM_VERSION}"' \
	"nFPM install must use the declared NFPM_VERSION."
require_literal "$WORKFLOW" \
	'choco install nsis --version="${NSIS_VERSION}" -y --no-progress' \
	"NSIS install must use the declared NSIS_VERSION."
require_literal "$WORKFLOW" \
	'actual_nfpm_version="$(go version -m "$(command -v nfpm)" | awk' \
	"workflow must verify nFPM module build metadata."
require_literal "$WORKFLOW" \
	'installed_nsis_version="$(choco list --exact nsis --limit-output' \
	"workflow must verify the installed Chocolatey NSIS version."
require_literal "$PACKAGE_LINUX" \
	'echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@$NFPM_VERSION"' \
	"package-linux must recommend the declared nFPM version."
require_literal "$PACKAGE_LINUX" \
	'actual_nfpm_version="$(go version -m "$nfpm_binary"' \
	"package-linux must reject an nFPM binary with mismatched module metadata."

# Presence alone is insufficient: GITHUB_ENV values become available only to
# following steps. Enforce ordering so a visually plausible workflow edit
# cannot run either installer before its version reaches the environment.
loader_line="$(line_of_literal "$WORKFLOW" "grep -E '^(NFPM_VERSION|NSIS_VERSION)='")"
nfpm_install_line="$(line_of_literal "$WORKFLOW" 'go install github.com/goreleaser/nfpm/v2/cmd/nfpm@"${NFPM_VERSION}"')"
nsis_install_line="$(line_of_literal "$WORKFLOW" 'choco install nsis --version="${NSIS_VERSION}"')"
if [ -n "$loader_line" ] && {
	{ [ -n "$nfpm_install_line" ] && [ "$loader_line" -ge "$nfpm_install_line" ]; } ||
		{ [ -n "$nsis_install_line" ] && [ "$loader_line" -ge "$nsis_install_line" ]; }
}; then
	echo "installer-tool-pins: workflow must load installer versions before native tool installation." >&2
	fail=1
fi

# create-dmg is opt-in locally because it drives Finder through AppleScript.
# The release workflow uses package-macos.sh's deterministic hdiutil default,
# so installing create-dmg there would add an unused, mutable network input.
if rg -q '^[[:space:]]*(run:[[:space:]]*)?brew install create-dmg([[:space:]]|$)' "$WORKFLOW"; then
	echo "installer-tool-pins: release workflow must not install create-dmg; hdiutil is the release path." >&2
	fail=1
fi
if rg -q 'PMFORGE_FANCY_DMG[=:][[:space:]]*["'\'']?1' "$WORKFLOW"; then
	echo "installer-tool-pins: release workflow must not enable the optional create-dmg path." >&2
	fail=1
fi

if rg -n '@latest|choco install nsis -y|brew install create-dmg' "$WORKFLOW" "$PACKAGE_LINUX" >/dev/null; then
	echo "installer-tool-pins: mutable or unversioned installer-tool installation remains." >&2
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit "$fail"
fi

echo "installer-tool-pins: nFPM $NFPM_VERSION and NSIS $NSIS_VERSION are pinned; macOS uses hdiutil."
