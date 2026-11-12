#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
SITE_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export GOWORK=off
export PYTHONDONTWRITEBYTECODE=1
exec python3 "$SITE_ROOT/tools/site.py" check "$@"
