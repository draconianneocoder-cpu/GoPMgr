#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
runner="$(cd "$(dirname "$0")" && pwd)/wails-v3-qualification.sh"
fixture=$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-bindings-test.XXXXXX")
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/frontend/bindings" "$fixture/code-map" "$fixture/bin"
printf 'module gopmgr\nrequire github.com/wailsapp/wails/v3 v3.1.0\n' > "$fixture/go.mod"
printf '{"dependencies":{"@wailsio/runtime":"3.1.0"}}' > "$fixture/frontend/package.json"
printf '{"methods":[{"method":"AccountSetup"}]}' > "$fixture/code-map/app-api.json"
printf 'existing output\n' > "$fixture/frontend/bindings/stale.ts"
cat > "$fixture/bin/wails3" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ "$1" == version ]]; then echo "${TEST_VERSION:-v3.1.0}" >&2; exit; fi
while [[ $# -gt 0 ]]; do
  if [[ "$1" == -d ]]; then out=$2; shift 2; else shift; fi
done
mkdir -p "$out/gopmgr/internal/desktop"
printf 'export function AccountSetup(): void {} \t\n\n' > "$out/gopmgr/internal/desktop/app.ts"
if [[ "${TEST_FAIL:-0}" == 1 ]]; then exit 19; fi
EOF
chmod +x "$fixture/bin/wails3"
run() { GOPMGR_REPO_ROOT="$fixture" WAILS3="$fixture/bin/wails3" bash "$runner" "$@"; }
fail() { echo "wails-v3-qualification_test: $*" >&2; exit 1; }
if TEST_VERSION=v2.17.0 run generate > "$fixture/out" 2>&1; then fail 'accepted wrong CLI'; fi
grep -q 'CLI.*v3.1.0' "$fixture/out" || fail 'wrong-version diagnostic missing'
if TEST_FAIL=1 run generate > "$fixture/out" 2>&1; then fail 'accepted failed generator'; fi
[[ $(cat "$fixture/frontend/bindings/stale.ts") == 'existing output' ]] || fail 'failed generation replaced previous bindings'
run generate
[[ ! -e "$fixture/frontend/bindings/stale.ts" ]] || fail 'stale output survived generation'
printf 'export function AccountSetup(): void {}\n' > "$fixture/expected.ts"
cmp "$fixture/expected.ts" "$fixture/frontend/bindings/gopmgr/internal/desktop/app.ts" || fail 'generated whitespace was not canonicalized'
run check
printf 'stale\n' > "$fixture/frontend/bindings/stale.ts"
if run check > "$fixture/out" 2>&1; then fail 'check accepted stale output'; fi
[[ -f "$fixture/frontend/bindings/stale.ts" ]] || fail 'check mutated existing bindings'
echo 'wails-v3-qualification_test: all cases passed.'
