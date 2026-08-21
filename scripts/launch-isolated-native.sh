#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Launch a built macOS GoPMgr bundle with a disposable data environment while
# denying access to the invoking account's current and legacy project roots.
# This is a developer verification harness, not an end-user launcher.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
DEFAULT_APP="$ROOT/build/bin/gopmgr.app/Contents/MacOS/gopmgr"

usage() {
	cat <<'EOF'
Usage:
  scripts/launch-isolated-native.sh [--app /absolute/path/to/gopmgr]

Launches a darwin GoPMgr app bundle in a retained, disposable HOME,
XDG_DATA_HOME, TMPDIR, and CFFIXED_USER_HOME. A macOS sandbox denies the
invoking account's current and legacy GoPMgr/PMForge project-data roots.

The default app is build/bin/gopmgr.app/Contents/MacOS/gopmgr. The temporary
root is retained after exit so a manual edit/relaunch test can reuse it. Quit
the app before removing that printed root.
EOF
}

fail() {
	echo "launch-isolated-native: $*" >&2
	exit 1
}

app="$DEFAULT_APP"
while (($# > 0)); do
	case "$1" in
		--app)
			(($# >= 2)) || fail "--app requires an absolute bundle executable path"
			app="$2"
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1"
			;;
	esac
done

[[ "$(uname -s)" == "Darwin" ]] || fail "macOS (Darwin) is required"
[[ -x /usr/bin/sandbox-exec ]] || fail "/usr/bin/sandbox-exec is required"

if pgrep -x gopmgr >/dev/null 2>&1 || pgrep -x GoPMgr >/dev/null 2>&1; then
	fail "GoPMgr is already running; quit it before starting an isolated verification"
fi

username="$(/usr/bin/id -un)" || fail "could not determine the invoking account"
if ! real_home="$(/usr/bin/dscl . -read "/Users/$username" NFSHomeDirectory 2>/dev/null | awk 'NR == 1 { sub(/^[^:]*:[[:space:]]*/, ""); print; exit }')"; then
	fail "could not determine the invoking account home directory"
fi
[[ -n "$real_home" ]] || fail "could not determine the invoking account home directory"
[[ "$real_home" == /* && "$real_home" != "/" ]] ||
	fail "invoking account home directory must be an absolute non-root path"
[[ ! -L "$real_home" ]] || fail "refusing a symlinked invoking account home directory"

canonical_root="$real_home/Library/Application Support/GoPMgr"
legacy_application_root="$real_home/Library/Application Support/PMForge"
legacy_documents_gopmgr_root="$real_home/Documents/GoPMgr"
legacy_documents_pmforge_root="$real_home/Documents/PMForge"

tmp_parent="${TMPDIR:-/tmp}"
[[ "$tmp_parent" == /* && -d "$tmp_parent" && ! -L "$tmp_parent" ]] ||
	fail "TMPDIR must be an existing, absolute, non-symlinked directory"
tmp_parent="$(cd "$tmp_parent" && pwd -P)"
for denied_root in "$canonical_root" "$legacy_application_root" \
	"$legacy_documents_gopmgr_root" "$legacy_documents_pmforge_root"; do
	if [[ "$tmp_parent" == "$denied_root" || "$tmp_parent" == "$denied_root/"* ]]; then
		fail "TMPDIR must not be inside a protected GoPMgr or PMForge data root"
	fi
done

[[ "$app" == /* ]] || fail "app path must be absolute"
[[ -f "$app" && -x "$app" ]] || fail "app executable is not a regular executable file: $app"
[[ ! -L "$app" ]] || fail "refusing a symlinked app executable"
[[ "$app" == *.app/Contents/MacOS/gopmgr ]] ||
	fail "app must be a gopmgr bundle executable ending in .app/Contents/MacOS/gopmgr"

bundle="${app%/Contents/MacOS/gopmgr}"
info_plist="$bundle/Contents/Info.plist"
[[ -f "$info_plist" && ! -L "$info_plist" ]] || fail "bundle Info.plist is missing or symlinked"
bundle_executable="$(/usr/bin/plutil -extract CFBundleExecutable raw -o - "$info_plist" 2>/dev/null)" ||
	fail "bundle Info.plist has no readable CFBundleExecutable"
bundle_identifier="$(/usr/bin/plutil -extract CFBundleIdentifier raw -o - "$info_plist" 2>/dev/null)" ||
	fail "bundle Info.plist has no readable CFBundleIdentifier"
[[ "$bundle_executable" == "gopmgr" ]] || fail "bundle CFBundleExecutable must be gopmgr"
[[ "$bundle_identifier" == "dev.gopmgr.GoPMgr" ]] ||
	fail "bundle CFBundleIdentifier must be dev.gopmgr.GoPMgr"

isolation_root="$(/usr/bin/mktemp -d "$tmp_parent/gopmgr-native-isolation.XXXXXX")" ||
	fail "could not create an isolated temporary root"
isolation_root="$(cd "$isolation_root" && pwd -P)"
chmod 0700 "$isolation_root"

home="$isolation_root/home"
xdg="$isolation_root/xdg"
tmp="$isolation_root/tmp"
allowed_probe="$isolation_root/allowed-probe"
denied_probe="$isolation_root/denied probe 'quoted'"
mkdir -m 0700 "$home" "$xdg" "$tmp" "$allowed_probe"
printf 'denied probe\n' >"$denied_probe"
chmod 0600 "$denied_probe"

protected_probe_roots=()
for protected_root in "$canonical_root" "$legacy_application_root" \
	"$legacy_documents_gopmgr_root" "$legacy_documents_pmforge_root"; do
	if [[ -e "$protected_root" ]]; then
		protected_probe_roots+=("$protected_root")
	fi
done

profile="$isolation_root/profile.sb"
cat >"$profile" <<'EOF'
(version 1)
(allow default)
(deny file-read* (subpath (param "GOPMGR_CANONICAL_ROOT")))
(deny file-write* (subpath (param "GOPMGR_CANONICAL_ROOT")))
(deny file-read* (subpath (param "GOPMGR_LEGACY_APPLICATION_ROOT")))
(deny file-write* (subpath (param "GOPMGR_LEGACY_APPLICATION_ROOT")))
(deny file-read* (subpath (param "GOPMGR_LEGACY_DOCUMENTS_GOPMGR_ROOT")))
(deny file-write* (subpath (param "GOPMGR_LEGACY_DOCUMENTS_GOPMGR_ROOT")))
(deny file-read* (subpath (param "GOPMGR_LEGACY_DOCUMENTS_PMFORGE_ROOT")))
(deny file-write* (subpath (param "GOPMGR_LEGACY_DOCUMENTS_PMFORGE_ROOT")))
(deny file-read* (subpath (param "GOPMGR_DENIED_PROBE")))
(deny file-write* (subpath (param "GOPMGR_DENIED_PROBE")))
EOF
chmod 0600 "$profile"

sandbox=(
	/usr/bin/sandbox-exec -f "$profile"
	-D "GOPMGR_CANONICAL_ROOT=$canonical_root"
	-D "GOPMGR_LEGACY_APPLICATION_ROOT=$legacy_application_root"
	-D "GOPMGR_LEGACY_DOCUMENTS_GOPMGR_ROOT=$legacy_documents_gopmgr_root"
	-D "GOPMGR_LEGACY_DOCUMENTS_PMFORGE_ROOT=$legacy_documents_pmforge_root"
	-D "GOPMGR_DENIED_PROBE=$denied_probe"
)

preflight="$isolation_root/preflight.sh"
cat >"$preflight" <<'EOF'
#!/bin/bash
set -euo pipefail

allowed_probe="$1"
denied_probe="$2"
shift 2

printf 'allowed probe\n' >"$allowed_probe/write-ok"
[[ -f "$allowed_probe/write-ok" ]] || {
	echo "sandbox preflight could not write the allowed probe" >&2
	exit 1
}

for denied_path in "$denied_probe" "$@"; do
	if denied_output="$(/usr/bin/stat "$denied_path" 2>&1)"; then
		echo "sandbox preflight unexpectedly read denied path: $denied_path" >&2
		exit 1
	fi
	if ! grep -Fqi "Operation not permitted" <<<"$denied_output"; then
		echo "sandbox preflight denied path without EPERM: $denied_path: $denied_output" >&2
		exit 1
	fi
done

if { printf 'blocked write\n' >"$denied_probe"; } 2>/dev/null; then
	echo "sandbox preflight unexpectedly wrote the denied probe" >&2
	exit 1
fi
EOF
chmod 0700 "$preflight"

preflight_args=(/bin/bash "$preflight" "$allowed_probe" "$denied_probe")
if ((${#protected_probe_roots[@]} > 0)); then
	preflight_args+=("${protected_probe_roots[@]}")
fi
if ! "${sandbox[@]}" "${preflight_args[@]}"; then
	fail "sandbox preflight failed; no app was launched"
fi

echo "launch-isolated-native: app=$app"
echo "launch-isolated-native: data_root=$xdg/GoPMgr"
echo "launch-isolated-native: retained_root=$isolation_root"
echo "launch-isolated-native: the app may take focus; no account or project is created automatically"

set +e
"${sandbox[@]}" /usr/bin/env \
	HOME="$home" \
	XDG_DATA_HOME="$xdg" \
	TMPDIR="$tmp" \
	CFFIXED_USER_HOME="$home" \
	"$app"
status=$?
set -e

echo "launch-isolated-native: app exited with status $status; retained_root=$isolation_root"
echo "launch-isolated-native: quit GoPMgr before removing the retained root"
exit "$status"
