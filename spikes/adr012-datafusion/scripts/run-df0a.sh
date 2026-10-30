#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Reproduce ADR-012 DF0-A/A2 evidence from a clean checkout:
#   1. unit tests for the corpus, comparator and oracle;
#   2. generate the corpus and check it against fixtures/manifest.json;
#   3. prove each fixture with an independent reader (pyarrow, Python json,
#      Go encoding/csv);
#   4. run GoPMgr's shipped DuckDB engine over the corpus.
#
# PYTHON must have pyarrow (the evidence used pyarrow 22.0.0):
#   python3 -m venv /tmp/df0-venv && /tmp/df0-venv/bin/pip install pyarrow==22.0.0
#   PYTHON=/tmp/df0-venv/bin/python spikes/adr012-datafusion/scripts/run-df0a.sh
set -euo pipefail
cd "$(dirname "$0")/.."
python="${PYTHON:-python3}"
corpus="$(mktemp -d "${TMPDIR:-/tmp}/df0-corpus.XXXXXX")"
trap 'rm -rf "$corpus"' EXIT

go test -count=1 ./internal/...
go run ./cmd/df0-fixtures -out "$corpus" -check
"$python" -I tools/independent_read.py "$corpus" > "$corpus/independent-reads.jsonl"
go run ./cmd/df0-selfcheck -corpus "$corpus" -reads "$corpus/independent-reads.jsonl" -out evidence/df0-a
go run -tags duckdb ./cmd/df0-baseline -corpus "$corpus" -out evidence/df0-a
