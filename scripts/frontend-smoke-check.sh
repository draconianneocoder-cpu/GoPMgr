#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Runtime smoke check: confirm the frontend's root component actually
# loads and renders through the real Vite + Svelte compiler. Catches
# load-time crashes (e.g. a $state rune in a plain .ts) that svelte-check
# and `vite build` pass but that leave #app empty in the browser.

set -eu
cd "$(dirname "$0")/../frontend"

node scripts/smoke-mount.mjs
