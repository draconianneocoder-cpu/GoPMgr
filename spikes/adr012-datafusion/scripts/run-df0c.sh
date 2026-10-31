#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Reproduce ADR-012 DF0-C evidence (Linux): the Go host serves gRPC on a
# private Unix socket; the Rust DataFusion worker connects as the client.
#   1. Rust tests and the locked release build of df0-worker;
#   2. Go unit tests, including the host's refusal tests, under -race;
#   3. generate the corpus (for the rollup cases);
#   4. run every DF0-C scenario against the real worker.
#
# Build output goes to CARGO_TARGET_DIR, outside the repository by default.
set -euo pipefail
cd "$(dirname "$0")/.."
target="${CARGO_TARGET_DIR:-${TMPDIR:-/tmp}/df0-rust-target}"
corpus="$(mktemp -d "${TMPDIR:-/tmp}/df0-corpus.XXXXXX")"
trap 'rm -rf "$corpus"' EXIT

(cd rust && CARGO_TARGET_DIR="$target" bash ../scripts/with-rust-toolchain.sh cargo test --locked --release)
(cd rust && CARGO_TARGET_DIR="$target" bash ../scripts/with-rust-toolchain.sh cargo build --locked --release -p df0-worker)
go test -count=1 -race ./internal/...
go run ./cmd/df0-fixtures -out "$corpus" -check
bash scripts/with-rust-toolchain.sh go run ./cmd/df0-ipc -worker "$target/release/df0-worker" -corpus "$corpus" -out evidence/df0-c
