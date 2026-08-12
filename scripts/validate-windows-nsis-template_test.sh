#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# validate-windows-nsis-template.sh has no isolated-fixture test of its
# own (its Wails/LICENSES/project.nsi prerequisites are real repo files,
# not something worth fixturing for one narrow fix). This test targets
# just the 2026-08-12 addition: when the `makensis` invocation exits
# nonzero, the script must print a hint naming GOPMGR_SKIP_NSIS_COMPILE
# and preserve the real exit status, rather than the bare, unexplained
# "Abort trap: 6" that cost three prior development cycles before the
# existing (2026-08-04, GOPMGR_SKIP_NSIS_COMPILE itself) fix was
# rediscovered. Uses a stub `makensis` prepended to PATH so the failure
# is deterministic and not tied to the specific host crash this was
# written to explain -- proving the hint fires on ANY nonzero exit, not
# just a detected crash signature, matters exactly as much as proving it
# fires at all.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
checker="$repo_root/scripts/validate-windows-nsis-template.sh"

test_root="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-nsis-hint-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

stub_dir="$test_root/stub-bin"
mkdir -p "$stub_dir"

# A distinctive, non-crash exit code: proves the hint fires on ordinary
# makensis failures (e.g. real template syntax rejection), not only ones
# that look like a crash, and that the script's own exit code is the
# stub's real exit status, not a hardcoded 1.
cat >"$stub_dir/makensis" <<'EOF'
#!/bin/sh
echo "stub makensis: simulated template rejection" >&2
exit 7
EOF
chmod +x "$stub_dir/makensis"

output=""
status=0
# Explicitly unset GOPMGR_SKIP_NSIS_COMPILE rather than assuming it's
# absent -- this test exists specifically to exercise the makensis
# invocation and its failure hint, and a value inherited from the
# invoking shell (e.g. a developer's own `GOPMGR_SKIP_NSIS_COMPILE=1 make
# verify` run) would silently divert the checker to the skip branch
# instead, making every assertion below fail for an unrelated reason.
output="$(env -u GOPMGR_SKIP_NSIS_COMPILE PATH="$stub_dir:$PATH" bash "$checker" 2>&1)" || status=$?

if [ "$status" -ne 7 ]; then
	echo "validate-windows-nsis-template_test: expected exit 7 (the stub's own status), got $status. Output:" >&2
	echo "$output" >&2
	exit 1
fi

if [[ "$output" != *"makensis exited 7"* ]]; then
	echo "validate-windows-nsis-template_test: expected the real exit status quoted in the hint, got:" >&2
	echo "$output" >&2
	exit 1
fi

if [[ "$output" != *"GOPMGR_SKIP_NSIS_COMPILE"* ]]; then
	echo "validate-windows-nsis-template_test: expected a GOPMGR_SKIP_NSIS_COMPILE hint on failure, got:" >&2
	echo "$output" >&2
	exit 1
fi

if [[ "$output" == *"NSIS 3 template compilation passed"* ]]; then
	echo "validate-windows-nsis-template_test: a failing makensis must not report the compilation as passed:" >&2
	echo "$output" >&2
	exit 1
fi

echo "validate-windows-nsis-template_test: all cases passed."
