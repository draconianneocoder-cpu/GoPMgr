<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->

# Import fixtures

Small hand-written files for `ImportTabular`. Expected values are written by
hand in the tests, not taken from either engine.

- **Text files** (`.csv`, `.tsv`, `.txt`, `.json`, `.ndjson`) were written by
  hand.
- **Parquet files** were written by DuckDB 1.5.6 (through duckdb-go
  v2.10506.0) with `COPY … (FORMAT parquet, COMPRESSION …)`:
  `types-<codec>.parquet` once per codec (uncompressed, snappy, gzip, zstd,
  brotli, lz4), and `many.parquet` with several row groups. A file written by
  another tool exercises the reader against a writer it does not share code
  with.
- **`duckdb-parity.json`** holds the retired DuckDB engine's results for
  every fixture, captured on 2026-11-03 UTC before DuckDB was removed. See
  [the parity record](../../../../docs/testing/duckdb-retirement-parity.md).

These files are test inputs only and are not shipped.
