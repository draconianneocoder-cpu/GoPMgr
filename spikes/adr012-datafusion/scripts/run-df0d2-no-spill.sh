#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
# Native Darwin no-spill experiment, not full DF-062 qualification.
set -euo pipefail
cd "$(dirname "$0")/.."
if [ "$(uname -s)" != Darwin ]; then
 echo "DF0-D2 write-denial experiment requires native Darwin; no unguarded fallback" >&2
 exit 1
fi
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-${TMPDIR:-/tmp}/gopmgr-df0-d2-target}"
mkdir -p "$CARGO_TARGET_DIR"
CARGO_TARGET_DIR="$(cd "$CARGO_TARGET_DIR" && pwd -P)"
export CARGO_TARGET_DIR
bash scripts/with-rust-toolchain.sh cargo build --manifest-path rust/Cargo.toml --locked --release -p df0-worker
export DF0_WORKER="$CARGO_TARGET_DIR/release/df0-worker"
go test -race -count=1 -v ./internal/ipchost -run '^TestNative(NoSpillPressure|WriteDenialControl|ReadAt|ExistingOperations)$'
