#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-linux-target-test.XXXXXX")"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/scripts" "$fixture/build/linux" "$fixture/build/darwin" "$fixture/build/windows" "$fixture/debian" "$fixture/.github/workflows" "$fixture/docs"
cp "$ROOT/scripts/check-linux-runtime-target.sh" "$fixture/scripts/"
cp "$ROOT/build/linux/nfpm.yaml" "$fixture/build/linux/"
cp "$ROOT/debian/control" "$fixture/debian/"
# Other agents own workflow/docs/Debian rules: use an already-converted
# fixture here, so their concurrent edits cannot affect these regression cases.
printf 'ubuntu-26.04\nlibgtk-4-dev libwebkitgtk-6.0-dev\n' >"$fixture/.github/workflows/ci.yml"
printf 'ubuntu-26.04\nos: ubuntu-24.04\nlibgtk-4-dev libwebkitgtk-6.0-dev\n' >"$fixture/.github/workflows/release.yml"
cp "$ROOT/Makefile" "$fixture/"
cp "$ROOT/debian/rules" "$fixture/debian/"
[ ! -f "$ROOT/build/linux/Taskfile.yml" ] || cp "$ROOT/build/linux/Taskfile.yml" "$fixture/build/linux/"
for p in README.md docs/ROADMAP.md docs/INSTALL.md docs/release-preflight.md; do echo 'Ubuntu 24.04+ GTK 4 WebKitGTK 6.0' >"$fixture/$p"; done
cd "$fixture"
# Correct baseline fixture before applying one broken dependency at a time.
python3 - <<'PY'
from pathlib import Path
for p in ['Makefile','debian/rules','debian/control','build/linux/nfpm.yaml']:
 s=Path(p).read_text().replace('webkit2_41','').replace('libgtk-3-dev','libgtk-4-dev').replace('libwebkit2gtk-4.1-dev','libwebkitgtk-6.0-dev').replace('libgtk-3-0','libgtk-4-1').replace('libwebkit2gtk-4.1-0','libwebkitgtk-6.0-4').replace('- gtk3','- gtk4').replace('- webkit2gtk4.1','- webkitgtk6.0')
 s=s.replace('golang-1.26-go (>= 1.26.6)','golang-1.27-go (>= 1.27.3)')
 Path(p).write_text(s)
PY
bash scripts/check-linux-runtime-target.sh >output 2>&1 || {
	cat output
	echo 'FAIL converted Linux target rejected'
	exit 1
}
for mutation in nfpm-gtk3 nfpm-webkit3 debian-dev legacy-build; do
	cp build/linux/nfpm.yaml nfpm.saved
	cp debian/control control.saved
	cp Makefile Makefile.saved
	case "$mutation" in
	nfpm-gtk3) sed 's/libgtk-4-1/libgtk-3-0/g' nfpm.saved >build/linux/nfpm.yaml ;;
	nfpm-webkit3) sed 's/libwebkitgtk-6.0-4/libwebkit2gtk-4.1-0/g' nfpm.saved >build/linux/nfpm.yaml ;;
	debian-dev) sed 's/libgtk-4-dev/libgtk-3-dev/g' control.saved >debian/control ;;
	legacy-build) echo 'GO_TEST_TAGS ?= webkit2_41' >>Makefile ;;
	esac
	if bash scripts/check-linux-runtime-target.sh >output 2>&1; then
		echo "FAIL accepted $mutation"
		exit 1
	fi
	mv nfpm.saved build/linux/nfpm.yaml
	mv control.saved debian/control
	mv Makefile.saved Makefile
	echo "PASS rejects $mutation"
done
echo 'check-linux-runtime-target fixtures passed'
