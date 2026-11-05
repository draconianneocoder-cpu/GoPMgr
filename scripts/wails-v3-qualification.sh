#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail

repo_root="${GOPMGR_REPO_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$repo_root"
mode=${1:-check}
[[ "$mode" == generate || "$mode" == check ]] || {
	echo 'Usage: wails-v3-qualification.sh [generate|check]' >&2
	exit 2
}
cli=${WAILS3:-$repo_root/.tmp/wails-v3/bin/wails3}
if [[ ! -f "$cli" && -f "$cli.exe" ]]; then cli="$cli.exe"; fi
version=$(awk '$1 == "github.com/wailsapp/wails/v3" {print $2; exit} $1 == "require" && $2 == "github.com/wailsapp/wails/v3" {print $3; exit}' go.mod)
[[ "$version" =~ ^v3\.[0-9]+\.[0-9]+$ ]] || {
	echo 'Cannot determine stable Wails v3 version from go.mod' >&2
	exit 1
}
[[ "$("$cli" version 2>&1)" == "$version" ]] || {
	echo "Wails CLI must match $version" >&2
	exit 1
}
python3 - "$version" <<'PY'
import json, sys
with open('frontend/package.json') as f:
    package = json.load(f)
if package.get('dependencies', {}).get('@wailsio/runtime') != sys.argv[1][1:]:
    sys.exit('Frontend runtime must exactly match the Wails module version')
PY
mkdir -p .tmp
scratch=$(mktemp -d "$repo_root/.tmp/wails-bindings.XXXXXX")
target="$repo_root/frontend/bindings"
cleanup() {
	cleanup_status=$?
	trap - EXIT
	if [[ -d "$scratch/previous" && ! -e "$target" ]]; then mv "$scratch/previous" "$target" || cleanup_status=1; fi
	rm -rf "$scratch" || cleanup_status=1
	exit "$cleanup_status"
}
trap cleanup EXIT
if [[ $(uname -s) == Darwin ]]; then
	export MACOSX_DEPLOYMENT_TARGET=13.0
	export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0"
	export CGO_CXXFLAGS="${CGO_CXXFLAGS:-} -mmacosx-version-min=13.0"
	export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=13.0"
fi
flags=''
if [[ ${GOPMGR_BINDINGS_PRODUCTION:-0} == 1 ]]; then flags='-tags production'; fi
"$cli" generate bindings -ts -i -names -time-type string -f "$flags" -d "$scratch/generated" ./internal/desktop
# Canonicalize generator-owned whitespace reproducibly, never by hand-editing.
python3 - "$scratch/generated" <<'PYTHON'
from pathlib import Path
import sys
for path in Path(sys.argv[1]).rglob('*.ts'):
    lines = path.read_text().splitlines()
    path.write_text('\n'.join(line.rstrip() for line in lines).rstrip() + '\n')
PYTHON
python3 - "$scratch/generated/gopmgr/internal/desktop/app.ts" <<'PY'
import json, re, sys
with open('code-map/app-api.json') as f:
    expected = {method['method'] for method in json.load(f)['methods']}
with open(sys.argv[1]) as f:
    actual = set(re.findall(r'^export function (\w+)\(', f.read(), re.M))
if not expected or actual != expected:
    sys.exit(f'Generated service surface differs: missing={sorted(expected-actual)}, extra={sorted(actual-expected)}')
PY
if [[ "$mode" == check ]]; then
	diff -ru "$target" "$scratch/generated"
else
	if [[ -e "$target" ]]; then mv "$target" "$scratch/previous"; fi
	mv "$scratch/generated" "$target"
fi
echo "wails-v3-qualification: $mode passed ($version)."
