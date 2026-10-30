<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 DF0 spike

This is the isolated feasibility spike authorized by the maintainer on
2026-10-26
([acceptance record](../../docs/review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)).
It is evidence only:

- It is not part of the GoPMgr build.
- It changes no production code, `go.mod`, build tag or package.

Results are recorded in
[`docs/testing/adr-012-df0-evidence.md`](../../docs/testing/adr-012-df0-evidence.md).

## Isolation

- The spike is its own Go module, `gopmgr/spikes/adr012-datafusion`, so the
  main module ignores this directory.
- It uses `replace gopmgr => ../..`, which lets it import the shipped
  `gopmgr/internal/analytics` engine unchanged.
- The packages the engine compiles from are the same modules at the same
  versions as the app: duckdb-go v2.10506.0 and bindings v0.10506.0.
  `environment.json` records this on every run.

## Layout

| Path | What |
|---|---|
| `internal/fixtures` | Seeded synthetic corpus: 24 portfolio and 70 tabular cases (CSV, TSV, TXT, JSON, NDJSON, Parquet), each carrying the logical truth the generator wrote. No TPC data |
| `internal/oracle` | Independent big-integer portfolio oracle. It keeps the legacy-parity verdict separate from the financial-correctness verdict (DF-A05) |
| `internal/canon` | Engine-neutral cell comparator and parity classes |
| `cmd/df0-fixtures` | Writes the corpus. `-check` compares it with `fixtures/manifest.json` (SHA-256 of every file and of its expected rows) |
| `tools/independent_read.py` | Reads Parquet with pyarrow and JSON with Python's `json` module, for the fixture self-check |
| `cmd/df0-selfcheck` | Proves every fixture against an independent reader before any engine is judged |
| `cmd/df0-baseline` | Runs the shipped DuckDB engine (A) over the corpus |
| `evidence/` | JSONL evidence, synthetic only. It contains no user paths or data |

## Reproduce DF0-A

```sh
python3 -m venv /tmp/df0-venv && /tmp/df0-venv/bin/pip install pyarrow==22.0.0
PYTHON=/tmp/df0-venv/bin/python spikes/adr012-datafusion/scripts/run-df0a.sh
```

The run needs Go 1.27.3 with CGO, as for GoPMgr, and network access the
first time, to download modules.
