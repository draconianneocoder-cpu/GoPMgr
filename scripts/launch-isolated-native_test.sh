#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Exercise the native-launch isolation harness with a fake bundle only. The
# test never launches GoPMgr or references the invoking user's data root.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
SOURCE="$ROOT/scripts/launch-isolated-native.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-native-isolation-test.XXXXXX")"
FIXTURE="$(cd "$FIXTURE" && pwd -P)"
trap 'rm -rf "$FIXTURE"' EXIT

fail() {
	echo "launch-isolated-native test: $*" >&2
	exit 1
}

assert_fails_with() {
	local expected="$1"
	shift
	local output
	if output="$("$@" 2>&1)"; then
		fail "expected failure containing: $expected"
	fi
	grep -Fq "$expected" <<<"$output" ||
		fail "failure did not contain '$expected': $output"
}

repo="$FIXTURE/repo"
mkdir -p "$repo/scripts" "$repo/build/bin"
cp "$SOURCE" "$repo/scripts/launch-isolated-native.sh"
chmod 0700 "$repo/scripts/launch-isolated-native.sh"
launcher="$repo/scripts/launch-isolated-native.sh"

stub_bin="$FIXTURE/stub-bin"
mkdir -m 0700 "$stub_bin"
cat >"$stub_bin/dscl" <<'EOF'
#!/bin/bash
set -euo pipefail
[[ -n "${GOPMGR_TEST_DSCL_HOME:-}" ]] || exit 1
printf 'NFSHomeDirectory: %s\n' "$GOPMGR_TEST_DSCL_HOME"
EOF
chmod 0700 "$stub_bin/dscl"
cat >"$stub_bin/pgrep" <<'EOF'
#!/bin/bash
if [[ "${GOPMGR_TEST_PGREP_RUNNING:-0}" == "1" ]]; then
	exit 0
fi
exit 1
EOF
chmod 0700 "$stub_bin/pgrep"
# Portable form deliberately avoids `sed -i`: BSD sed (macOS) requires a
# backup-suffix argument (`-i ''`) that GNU sed (Linux CI) instead treats as
# the script text, silently shifting the real script and target path into
# filename arguments -- this line runs before the Darwin guard below, so it
# executed (and failed) on every platform, not just macOS.
sed "s|/usr/bin/dscl|$stub_bin/dscl|" "$launcher" >"$launcher.tmp"
mv "$launcher.tmp" "$launcher"
chmod 0700 "$launcher"

make_bundle() {
	local base="$1"
	local executable_name="${2:-gopmgr}"
	local identifier="${3:-dev.gopmgr.GoPMgr}"
	local exit_status="${4:-0}"
	local executable="$base/gopmgr.app/Contents/MacOS/gopmgr"
	mkdir -p "$(dirname "$executable")"
	cat >"$base/gopmgr.app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>$executable_name</string>
<key>CFBundleIdentifier</key><string>$identifier</string>
</dict></plist>
EOF
	cat >"$executable" <<EOF
#!/bin/bash
set -euo pipefail
printf '%s\n' "\$HOME" >"\$GOPMGR_CAPTURE/home"
printf '%s\n' "\$XDG_DATA_HOME" >"\$GOPMGR_CAPTURE/xdg"
printf '%s\n' "\$TMPDIR" >"\$GOPMGR_CAPTURE/tmp"
printf '%s\n' "\$CFFIXED_USER_HOME" >"\$GOPMGR_CAPTURE/cfhome"
for protected_root in \
    "\$GOPMGR_EXPECTED_HOME/Library/Application Support/GoPMgr" \
    "\$GOPMGR_EXPECTED_HOME/Library/Application Support/PMForge" \
    "\$GOPMGR_EXPECTED_HOME/Documents/GoPMgr" \
    "\$GOPMGR_EXPECTED_HOME/Documents/PMForge"; do
    if /usr/bin/stat "\$protected_root" >/dev/null 2>&1; then
        exit 86
    fi
done
exit "$exit_status"
EOF
	chmod 0700 "$executable"
	printf '%s\n' "$executable"
}

if [[ "$(uname -s)" != "Darwin" ]]; then
	assert_fails_with "macOS (Darwin) is required" bash "$launcher" --app /tmp/missing
	echo "launch-isolated-native tests passed (non-Darwin refusal)."
	exit 0
fi

test_home="$FIXTURE/Actual home 'quoted'"
mkdir -p "$test_home/Library/Application Support/GoPMgr" \
	"$test_home/Library/Application Support/PMForge" \
	"$test_home/Documents/GoPMgr" \
	"$test_home/Documents/PMForge"
for denied in \
	"$test_home/Library/Application Support/GoPMgr" \
	"$test_home/Library/Application Support/PMForge" \
	"$test_home/Documents/GoPMgr" \
	"$test_home/Documents/PMForge"; do
	printf 'sentinel\n' >"$denied/sentinel"
done

launcher_tmp="$FIXTURE/launcher-tmp"
mkdir -m 0700 "$launcher_tmp"

run_launcher() {
	env GOPMGR_TEST_DSCL_HOME="$test_home" \
		GOPMGR_TEST_PGREP_RUNNING=0 \
		GOPMGR_EXPECTED_HOME="$test_home" \
		TMPDIR="$launcher_tmp" \
		PATH="$stub_bin:$PATH" \
		bash "$launcher" "$@"
}

app="$(make_bundle "$FIXTURE/fake-bundle")"
make_bundle "$repo/build/bin" >/dev/null
capture="$FIXTURE/capture"
mkdir -m 0700 "$capture"

assert_fails_with "unknown argument" run_launcher --unexpected
assert_fails_with "app path must be absolute" run_launcher --app relative.app/Contents/MacOS/gopmgr
assert_fails_with "not a regular executable" run_launcher --app /tmp/missing.app/Contents/MacOS/gopmgr
assert_fails_with "TMPDIR must not be inside" \
	env GOPMGR_TEST_DSCL_HOME="$test_home" GOPMGR_TEST_PGREP_RUNNING=0 \
	GOPMGR_EXPECTED_HOME="$test_home" TMPDIR="$test_home/Library/Application Support/GoPMgr" \
	PATH="$stub_bin:$PATH" bash "$launcher" --app "$app"

bad_identifier="$(make_bundle "$FIXTURE/bad-identifier" gopmgr example.invalid)"
assert_fails_with "CFBundleIdentifier" run_launcher --app "$bad_identifier"
bad_executable="$(make_bundle "$FIXTURE/bad-executable" not-gopmgr)"
assert_fails_with "CFBundleExecutable" run_launcher --app "$bad_executable"
ln -s "$app" "$FIXTURE/symlink.app"
assert_fails_with "refusing a symlinked app executable" run_launcher --app "$FIXTURE/symlink.app"

assert_fails_with "GoPMgr is already running" \
	env GOPMGR_TEST_DSCL_HOME="$test_home" GOPMGR_TEST_PGREP_RUNNING=1 \
	GOPMGR_EXPECTED_HOME="$test_home" TMPDIR="$launcher_tmp" PATH="$stub_bin:$PATH" \
	bash "$launcher" --app "$app"

output="$(
	GOPMGR_CAPTURE="$capture" \
	run_launcher --app "$app"
)"

retained_root="$(awk -F= '/^launch-isolated-native: retained_root=/ { print $2; exit }' <<<"$output")"
[[ -n "$retained_root" && -d "$retained_root" ]] || fail "launcher did not retain the disposable root"
[[ "$(stat -f '%Lp' "$retained_root")" == "700" ]] || fail "retained root mode is not 0700"
for directory in "$retained_root/home" "$retained_root/xdg" "$retained_root/tmp"; do
	[[ "$(stat -f '%Lp' "$directory")" == "700" ]] || fail "$directory mode is not 0700"
done

[[ "$(cat "$capture/home")" == "$retained_root/home" ]] || fail "child HOME escaped the disposable root"
[[ "$(cat "$capture/xdg")" == "$retained_root/xdg" ]] || fail "child XDG_DATA_HOME escaped the disposable root"
[[ "$(cat "$capture/tmp")" == "$retained_root/tmp" ]] || fail "child TMPDIR escaped the disposable root"
[[ "$(cat "$capture/cfhome")" == "$retained_root/home" ]] || fail "child CFFIXED_USER_HOME escaped the disposable root"
[[ -f "$retained_root/allowed-probe/write-ok" ]] || fail "sandbox did not permit the allowed probe"
for denied in \
	"$test_home/Library/Application Support/GoPMgr/sentinel" \
	"$test_home/Library/Application Support/PMForge/sentinel" \
	"$test_home/Documents/GoPMgr/sentinel" \
	"$test_home/Documents/PMForge/sentinel"; do
	[[ "$(cat "$denied")" == "sentinel" ]] || fail "denied root sentinel changed: $denied"
done

default_capture="$FIXTURE/default-capture"
mkdir -m 0700 "$default_capture"
GOPMGR_CAPTURE="$default_capture" run_launcher >/dev/null
[[ -s "$default_capture/home" ]] || fail "default bundle was not launched"

failing_app="$(make_bundle "$FIXTURE/failing-bundle" gopmgr dev.gopmgr.GoPMgr 23)"
mkdir -m 0700 "$FIXTURE/failing-capture"
assert_fails_with "app exited with status 23" \
	env GOPMGR_TEST_DSCL_HOME="$test_home" GOPMGR_TEST_PGREP_RUNNING=0 \
	GOPMGR_EXPECTED_HOME="$test_home" TMPDIR="$launcher_tmp" PATH="$stub_bin:$PATH" \
	GOPMGR_CAPTURE="$FIXTURE/failing-capture" \
	bash "$launcher" --app "$failing_app"

assert_fails_with "could not determine the invoking account home directory" \
	env GOPMGR_TEST_DSCL_HOME='' GOPMGR_TEST_PGREP_RUNNING=0 \
	GOPMGR_EXPECTED_HOME="$test_home" TMPDIR="$launcher_tmp" PATH="$stub_bin:$PATH" GOPMGR_CAPTURE="$capture" \
	bash "$launcher" --app "$app"

faulted_launcher="$FIXTURE/faulted-launcher.sh"
cp "$launcher" "$faulted_launcher"
perl -0pi -e 's/\Q(deny file-read* (subpath (param "GOPMGR_CANONICAL_ROOT")))\E\n//' "$faulted_launcher"
if grep -Fq '(deny file-read* (subpath (param "GOPMGR_CANONICAL_ROOT")))' "$faulted_launcher"; then
	fail "fault seed did not remove the canonical read-denial clause"
fi
assert_fails_with "sandbox preflight failed" \
	env GOPMGR_TEST_DSCL_HOME="$test_home" GOPMGR_TEST_PGREP_RUNNING=0 \
	GOPMGR_EXPECTED_HOME="$test_home" TMPDIR="$launcher_tmp" PATH="$stub_bin:$PATH" GOPMGR_CAPTURE="$capture" \
	bash "$faulted_launcher" --app "$app"

echo "launch-isolated-native tests passed."
