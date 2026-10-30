#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Reproduce ADR-012 DF0-B/B2 evidence from a clean checkout:
#   1. Rust engine tests and the locked release build of df0-cli;
#   2. Go unit tests (corpus, comparator, oracle, Go rollup candidate);
#   3. generate the corpus and check it against fixtures/manifest.json;
#   4. run candidates B (DataFusion) and C (Go rollup) over the corpus,
#      next to engine A's committed DF0-A verdicts.
#
# The Rust toolchain comes from rust-toolchain.toml. Build output goes to
# CARGO_TARGET_DIR, outside the repository by default, so `reuse lint`
# never sees generated files.
set -euo pipefail
cd "$(dirname "$0")/.."
target="${CARGO_TARGET_DIR:-${TMPDIR:-/tmp}/df0-rust-target}"
corpus="$(mktemp -d "${TMPDIR:-/tmp}/df0-corpus.XXXXXX")"
trap 'rm -rf "$corpus"' EXIT

(cd rust && CARGO_TARGET_DIR="$target" cargo test --locked --release)
(cd rust && CARGO_TARGET_DIR="$target" cargo build --locked --release -p df0-cli)
go test -count=1 ./internal/...
go run ./cmd/df0-fixtures -out "$corpus" -check
go run ./cmd/df0-candidates -corpus "$corpus" -cli "$target/release/df0-cli" -out evidence/df0-b
