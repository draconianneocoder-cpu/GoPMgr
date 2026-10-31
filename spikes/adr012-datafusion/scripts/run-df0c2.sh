#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
# DF0-C2 native UDS comparison. Evidence destination must be explicitly selected.
set -euo pipefail
cd "$(dirname "$0")/.."
case "$(uname -s)" in Darwin|Linux) ;; *) echo "DF0-C2 requires a native UDS host" >&2; exit 1;; esac
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-${TMPDIR:-/tmp}/gopmgr-df0-c2-target}"
mkdir -p "$CARGO_TARGET_DIR"
CARGO_TARGET_DIR="$(cd "$CARGO_TARGET_DIR" && pwd -P)"
export CARGO_TARGET_DIR
bash scripts/with-rust-toolchain.sh cargo build --manifest-path rust/Cargo.toml --locked --release -p df0-worker
export DF0_WORKER="$CARGO_TARGET_DIR/release/df0-worker"
go test -race -count=1 -v ./internal/ipchost -run '^TestNative(ReadAt|ExistingOperations)$'
