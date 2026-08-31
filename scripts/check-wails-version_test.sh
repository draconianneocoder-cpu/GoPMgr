#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
checker="$repo_root/scripts/check-wails-version.sh"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-wails-version-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

fail() {
	echo "check-wails-version_test: $*" >&2
	exit 1
}

make_fixture() {
	local root=$1
	mkdir -p "$root/.github/workflows" "$root/docs" "$root/scripts" "$root/build/windows/installer"

	cat >"$root/go.mod" <<'EOF'
module example.test/gopmgr

require github.com/wailsapp/wails/v2 v2.13.0
EOF

	for workflow in ci.yml release.yml; do
		cat >"$root/.github/workflows/$workflow" <<'EOF'
steps:
  - run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
EOF
	done

	cat >"$root/README.md" <<'EOF'
Wails: the project uses Wails v2.13.0.
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
EOF
	cat >"$root/AGENTS.md" <<'EOF'
go.mod pins Go 1.26.5 and Wails v2.13.0.
EOF
	cat >"$root/DEPENDENCIES.md" <<'EOF'
Wails: v2.13.0.
EOF
	cat >"$root/docs/INSTALL.md" <<'EOF'
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
EOF
	cat >"$root/DEVELOPER_HANDBOOK.md" <<'EOF'
Current runtime: **Wails v2.13.0**.
Install with go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0.

Historical note: GoPMgr previously upgraded from Wails v2.9.2 to v2.12.0.
EOF
	cat >"$root/docs/branding.md" <<'EOF'
Wails v2.13 reads build/appicon.png.
EOF
	cat >"$root/build/windows/installer/project.nsi" <<'EOF'
# GoPMgr-owned Wails v2.13 NSIS entrypoint.
EOF
	for script in check-release.sh package-macos-installer.sh; do
		cat >"$root/scripts/$script" <<'EOF'
echo "Install with go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0."
EOF
	done
}

make_wails_stub() {
	local root=$1
	local version=$2
	mkdir -p "$root"
	cat >"$root/wails" <<EOF
#!/usr/bin/env bash
echo "$version"
EOF
	chmod +x "$root/wails"
}

expect_failure() {
	local fixture=$1
	local expected=$2
	local output
	if output="$(GOPMGR_REPO_ROOT="$fixture" bash "$checker" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	if [[ "$output" != *"$expected"* ]]; then
		fail "failure did not contain '$expected': $output"
	fi
}

happy="$test_root/happy"
make_fixture "$happy"
GOPMGR_REPO_ROOT="$happy" bash "$checker" >/dev/null

matching_cli="$test_root/matching-cli"
make_wails_stub "$matching_cli" "v2.13.0"
GOPMGR_REPO_ROOT="$happy" GOPMGR_REQUIRE_WAILS_CLI=1 \
	PATH="$matching_cli:$PATH" bash "$checker" >/dev/null

stale_cli="$test_root/stale-cli"
make_wails_stub "$stale_cli" "v2.12.0"
if output="$(
	GOPMGR_REPO_ROOT="$happy" GOPMGR_REQUIRE_WAILS_CLI=1 \
		PATH="$stale_cli:$PATH" bash "$checker" 2>&1
)"; then
	fail "expected installed CLI mismatch"
fi
if [[ "$output" != *"installed Wails CLI is v2.12.0"* ]]; then
	fail "installed CLI mismatch was not actionable: $output"
fi

workflow_mismatch="$test_root/workflow-mismatch"
make_fixture "$workflow_mismatch"
perl -0pi -e 's/wails\@v2\.13\.0/wails\@v2.12.0/' "$workflow_mismatch/.github/workflows/release.yml"
expect_failure "$workflow_mismatch" ".github/workflows/release.yml"

documentation_mismatch="$test_root/documentation-mismatch"
make_fixture "$documentation_mismatch"
perl -0pi -e 's/Wails v2\.13\.0/Wails v2.12.0/' "$documentation_mismatch/AGENTS.md"
expect_failure "$documentation_mismatch" "AGENTS.md"

branding_mismatch="$test_root/branding-mismatch"
make_fixture "$branding_mismatch"
perl -0pi -e 's/Wails v2\.13/Wails v2.12/' "$branding_mismatch/docs/branding.md"
expect_failure "$branding_mismatch" "docs/branding.md"

installer_mismatch="$test_root/installer-mismatch"
make_fixture "$installer_mismatch"
perl -0pi -e 's/Wails v2\.13/Wails v2.12/' "$installer_mismatch/build/windows/installer/project.nsi"
expect_failure "$installer_mismatch" "build/windows/installer/project.nsi"

unpinned_guidance="$test_root/unpinned-guidance"
make_fixture "$unpinned_guidance"
perl -0pi -e 's/wails\@v2\.13\.0/wails\@latest/' "$unpinned_guidance/scripts/package-macos-installer.sh"
expect_failure "$unpinned_guidance" "scripts/package-macos-installer.sh"

missing_module="$test_root/missing-module"
make_fixture "$missing_module"
perl -0pi -e 's#github\.com/wailsapp/wails/v2 v2\.13\.0#example.test/other v1.0.0#' "$missing_module/go.mod"
expect_failure "$missing_module" "cannot determine"

echo "check-wails-version_test: all cases passed."
