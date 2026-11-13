#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
SITE_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export GOWORK=off
export PYTHONDONTWRITEBYTECODE=1
HUGO=$(python3 "$SITE_ROOT/tools/provision_hugo.py")
export HUGO
build=$(bash "$SITE_ROOT/tools/build.sh")
printf '%s\n' "$build"
report=$(printf '%s\n' "$build" | python3 -c 'import json,sys; print(json.load(sys.stdin)["report"])')
bash "$SITE_ROOT/tools/check.sh" --report "$report"
