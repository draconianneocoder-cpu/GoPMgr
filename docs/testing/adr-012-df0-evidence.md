<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 DF0 evidence

DF0 is authorized and evidence-only
([acceptance record](../review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)).
This file records results phase by phase. Every result names its command, its
inputs and its evidence file.

- Code: [`spikes/adr012-datafusion/`](../../spikes/adr012-datafusion/README.md).
- Raw records: `spikes/adr012-datafusion/evidence/`.

Status words follow the specification:

- **PASS** and **FAIL** were measured.
- **BLOCKED** could not run here, and the missing resource is named.
- **NOT-RUN** has not been attempted yet.

DuckDB is engine A, the shipped baseline. DataFusion is B. The Go rollup plus
DataFusion is C.

## Phase status

| Phase | Status |
|---|---|
| DF0-A baseline freeze | **Done on Linux amd64.** macOS arm64 and Windows x64 are BLOCKED: no native host here |
| DF0-A2 money oracle | **Done** |
| DF0-B / B2 DataFusion and Go candidates | **Done on Linux amd64** (standalone engine; no IPC yet). macOS and Windows: BLOCKED |
| DF0-C Go↔Rust gRPC local IPC | NOT-RUN |
| DF0-C2 host-mediated Parquet `ReadAt` | NOT-RUN |
| DF0-D / D2 security, no-spill, admission | NOT-RUN |
| DF0-E / E2 performance, numeric fidelity | NOT-RUN |
| DF0-F / F2 packaging, licensing | NOT-RUN |
| DF0-G report | NOT-RUN |

## DF0-A — baseline freeze (Linux amd64)

### Environment

| Item | Value |
|---|---|
| Source | `main` 28a758d plus the ADR-012 docs (`claude/adr-012-datafusion-proposal`) |
| Host | Linux amd64, 4 vCPU Intel Xeon @ 2.10 GHz, 15 GiB RAM; container, root user |
| Toolchain | Go 1.27.3 (CGO, gcc) |
| Engine A | `gopmgr/internal/analytics` with `-tags duckdb`, unmodified |
| Engine A modules | `go list -deps` of that package in the spike gives the same modules and versions as the app: duckdb-go v2.10506.0, duckdb-go-bindings and lib/linux-amd64 v0.10506.0, mapstructure v2.5.0, uuid v1.6.0 |
| Independent readers | pyarrow 22.0.0 (Arrow C++), Python 3.13 `json`, Go `encoding/csv` |
| Reproduce | `PYTHON=<python with pyarrow 22.0.0> spikes/adr012-datafusion/scripts/run-df0a.sh` |

### Steps

1. **Existing tests (DF-001).**
   - `go test -tags duckdb ./internal/analytics/...` passes 8 of 8.
   - `go test ./internal/analytics/...` (the stub) passes 1 of 1.
2. **Call sites.** The only callers are:
   - `App.RunPortfolioAnalytics` (`app_foundation.go:894`);
   - `App.ImportDatasetForAnalysis` (`app_foundation.go:1072`);
   - `Portfolio.svelte`, which calls both and renders them.

   The `duckdb` tag is used by `Makefile` `WAILS_BUILD_TAGS` and by the CI
   build, vuln and analytics-duckdb jobs. Each call opens a new engine and
   closes it (finding F-3 in the [verification](../review/adr-012-repository-verification.md)).
3. **Corpus.**
   - 94 seeded synthetic cases: 24 portfolio, 70 tabular. One 1,000,000-row
     CSV is written only for performance runs.
   - `fixtures/manifest.json` pins every file's SHA-256 and a digest of its
     expected rows.
   - Two independent generations were byte-identical
     (`df0-fixtures -check`).
   - No TPC generator, query or data is used.
4. **Fixture self-check.**
   - 69 of 69 tabular fixtures are reproduced exactly by readers independent
     of every engine: pyarrow for Parquet, Python `json` for JSON and NDJSON,
     and Go `encoding/csv` for delimited text.
   - One reader limit is recorded: Python's `json` stops at its recursion
     limit on the 500-level nesting case, which `encoding/json` parses.
   - Evidence: `fixture-selfcheck.jsonl`.
5. **Engine A over the corpus.** Evidence: `duckdb-portfolio.jsonl`,
   `duckdb-tabular.jsonl`, `duckdb-filesystem.jsonl` and `environment.json`.

### DF0-A2 — portfolio money oracle

The oracle (`internal/oracle`) is written from the specification of
`normaliseProjectMetricsMoney` and the rollup SQL. It does not call the app's
code:

- exact `big.Int` sums;
- exact rational SPI and CPI;
- separate verdicts for **legacy parity** and **financial correctness**.

A differential test proves its legacy rule equals `money.FromMajorFloat` on
200,000 random and edge-case floats, including every bit pattern class
(NaN, ±Inf, subnormals).

| Result | Count |
|---|---|
| Cases where DuckDB equals the legacy oracle, field by field (SPI/CPI within 2 ULP of the exact ratio) | **24 of 24** |
| Of those, correctness `exact` | 19 |
| Correctness `ambiguous_input`: a zero minor-unit field replaced by a nonzero float (DF-061) | 3 |
| Correctness `invalid_input`: a non-finite float becomes 0, or a huge float saturates | 2 |
| Repeat call identical (DF-009) | 21 of 21 non-error cases |

Legacy behavior that every candidate must reproduce, or change only with
your approval:

- **Overflow.** A total outside int64 fails with a scan error, which is
  typed but unfriendly. It never wraps. DuckDB sums in HUGEINT; scanning the
  result into `int64` fails.
- **EVM exclusion.** An overflowing AC on an EVM-unavailable project is
  excluded from the sum and does not fail.
- **Float fallback rounding is binary, not decimal.**
  - 1.005 becomes 100 minor units.
  - 2.675 becomes 268, because the float64 product is exactly 267.5, a tie.
    Exact decimal rounding of the stored value gives 267.
  - NaN and ±Inf become 0.
  - 1e300 saturates to `MaxInt64`, and the sum then overflows.
- **SPI/CPI** are computed from float64 major units. They are 0 for "n/a".
- **Currency and reporting date** are host checks (verification F-2). The
  engine never sees them, so DF-007 and DF-010 are host-level legacy
  behavior.

### DF0-A tabular baseline (engine A)

| Verdict | Count | Cases |
|---|---|---|
| match | 38 | — |
| match_with_notes | 15 | DuckDB typed the text: integers, doubles, booleans, dates; `007` becomes 7; empty field becomes NULL |
| degenerate_ok | 5 | latin1 CSV rejected; `.ndjson` holding an array read; empty JSON array; deep nesting read intact; scalar JSON read as one row |
| ambiguous | 1 | duplicate JSON keys (DuckDB rejects them) |
| rejected (correct) | 2 | corrupt Parquet footer; text file named `.parquet` |
| **loss** | 5 | see LD-1 to LD-4 below |
| **silent_accept** | 3 | see LD-5 below |

**Legacy defects in the shipped engine.** Each is confirmed against
independent truth. A replacement must not copy them silently; each correction
needs your approval (directive §4.1).

| ID | Defect | Evidence |
|---|---|---|
| LD-1 | **A wrong value with no warning.** A Parquet `decimal256(60,10)` column is read as float64 with a value unrelated to the stored one: 3.47e63 for 1.23e49. pyarrow reads the same file exactly | `pq-decimals` |
| LD-2 | One out-of-int64 value makes a CSV or JSON column DOUBLE, so in-range integers above 2^53 change value (9007199254740993 becomes …992) | `csv-big-int`, `json-big-int` |
| LD-3 | JSON `1e400` is read as `+Inf` with no error | `json-decimals` |
| LD-4 | An empty CSV yields a phantom column `column0` | `csv-empty` |
| LD-5 | **Malformed files are accepted silently.** An unterminated CSV quote returns 2 rows, ragged CSV rows return 1 row of 4 columns, and JSON with a trailing comma returns 1 row | `csv-malformed-quote`, `csv-ragged`, `json-invalid` |
| LD-6 | **Files the user did not select are read.** The path goes to `read_csv_auto` as a glob. Selecting a file named `*.csv` returns the rows of every CSV in that folder (3 rows from 3 files); `q?.csv` also reads `q1.csv`; `**.csv` reads 4 files | `duckdb-filesystem.jsonl`: `fs-glob-*` |
| LD-7 | **Parquet decimals and nested values reach the UI as JSON objects.** `json.Marshal(duckdb.Decimal)` is `{"Width":…,"Scale":…,"Value":…}`, and `Portfolio.svelte` renders cells with `String(cell)`. So every Parquet DECIMAL cell, a common type for money, displays as `[object Object]`. That is 10,000 of 30,000 preview cells in `pq-rows-100000`. Found by measuring the wire bytes and reading the code; not exercised in the running app | `duckdb-tabular.jsonl` `wire.object_cells` |
| LD-8 | Integers above 2^53 cross the Wails JSON boundary as numbers that JavaScript rounds (DF-065 legacy). Here they were already doubles because of LD-2 | `wire.js_unsafe_numbers` |
| LD-9 | The preview reports "10000 rows" for larger files (verification F-4) | `csv-rows-50000`: 10,000 rows of 50,000, with no truncation flag in the API |
| LD-10 | **A NaN or infinite cell makes the import unencodable, and Wails then exits the app.** `json.Marshal(Dataset)` fails with `unsupported value: NaN` (or `+Inf`). Wails v2.17.0 marshals every call result with `encoding/json`, and on failure calls `Logger.Fatal`, which runs `os.Exit(1)` (`internal/frontend/dispatcher/calls.go`). `App.ImportDatasetForAnalysis` (`app_foundation.go`) returns the Dataset as is. So importing such a file should close GoPMgr with no message. Found by recording the marshal error (Codex review of #151) and reading Wails' code; not exercised in the running app | `duckdb-tabular.jsonl` `wire.error`: `pq-scalars`, `pq-scalars-nulls` (NaN), `json-decimals` (`+Inf`, from LD-3) |

Other observed behavior:
- A symlink to a file outside the folder is followed. `allowed_directories`
  is set to the selected file's own folder, so it would not stop this.
  ADR-012 DF-09 needs host-side canonicalization instead.
- The source file is unchanged after import (DF-020, observed).
- Missing files, directories, `.xlsx` files and files with no extension get
  typed errors.
- **BLOCKED:** the unreadable-file case. The container runs as root, which
  bypasses file permissions; it needs an unprivileged run.

### Test status after DF0-A

The DF0 tests judge the candidates (B, C) against the frozen baseline (A) and
the oracle. Below, "characterized" means engine A's legacy result is
recorded for the comparison.

| Test | A (DuckDB) | B | C |
|---|---|---|---|
| DF-001 shipped tests reproduced | **PASS** (Linux amd64); macOS/Windows **BLOCKED** | — | — |
| DF-002, 003, 004, 005, 006, 008, 009, 054, 061 portfolio semantics | characterized; equals the legacy oracle on 24 of 24 | **PASS** 24 of 24 (DF0-B, standalone) | **PASS** 24 of 24 |
| DF-007, 010 currency and reporting date | host behavior characterized (F-2) | NOT-RUN | NOT-RUN |
| DF-011, 012, 013, 014 formats | characterized: 69 cases with verdicts | **FAIL**: 3 valid files rejected (G-1 to G-3) and 1 silent accept; better than A on 7 cases | same as B (C uses B's importer) |
| DF-015, 016 truncation | characterized: no flag (LD-9) | **PASS**: the truncation flag is correct on 54 of 54 applicable cases | same as B |
| DF-017 typed errors | characterized; LD-5 and LD-6 found; one case BLOCKED (root) | **FAIL**: glob expansion (`q?.csv`) as in A; otherwise typed errors | same as B |
| DF-018 ordering | file order is preserved in every match case | **PASS** after a fix: one partition, and NDJSON columns in file order (F-B1, F-B2) | same as B |
| DF-020 source file unchanged | observed: unchanged | NOT-RUN | NOT-RUN |
| DF-065 wire and UI numeric contract | characterized: LD-7, LD-8; 3 of 69 Datasets cannot be JSON-encoded (LD-10) | NOT-RUN | NOT-RUN |
| All others (DF-019, DF-021 to DF-053, DF-055 to DF-060, DF-062 to DF-064) | — | NOT-RUN | NOT-RUN |

### Decisions this raises for the maintainer (not needed until DF1)

1. **LD-1, LD-5, LD-6 and LD-10** are defects in the shipped app,
   independent of ADR-012. LD-10 is the most severe: it closes the app. I
   propose small production fixes, each needing its own authorization:
   - convert NaN and infinite cells to strings or nulls (with a flag)
     before the Dataset leaves Go;
   - escape glob characters, or open the file in Go and pass the reader;
   - turn off lenient CSV/JSON error handling, or surface the rejected-row
     count;
   - reject or flag decimals above 38 digits.
2. **LD-7 and LD-9** are UI-visible today. They could be fixed in the
   current engine too: render decimals as strings and add a truncation flag.
3. For the replacement, the parity target for each LD item is either "keep
   legacy" or "correct, with approval". The evidence supports correcting all
   of them.

## DF0-B / B2 — candidates B and C, standalone (Linux amd64)

**What runs:**
- **B** is a Rust DataFusion engine (`spikes/adr012-datafusion/rust`) called
  as a CLI per operation. It reads files from paths; the host-mediated
  design (DF-09, DF-A07) comes in DF0-C2.
- **C** is the native Go rollup (`internal/gorollup`) plus B's importer.
- The host applies the shipped money normalization before B sees minor
  units (DF-06).
- Both are scored by the same oracle and comparator as A.
- Evidence: `evidence/df0-b/` (`portfolio.jsonl`, `datafusion-tabular.jsonl`,
  `datafusion-filesystem.jsonl`, `environment.json`).

| Item | Value |
|---|---|
| Crates (Cargo.lock) | DataFusion 55.2.0, arrow and parquet 59.3.2, tokio 1.54.1, serde_json 1.0.154 (`raw_value`, `preserve_order`); 264 crates in total |
| DataFusion features | `parquet`, `compression`, `sql`, `recursive_protection`, `nested_expressions`. Default features off; no avro, remote object stores or encryption |
| Toolchain | rustc 1.97.0 (pinned in `rust-toolchain.toml`; MSRV 1.94) |
| Release binary | 139.5 MB unstripped, Linux amd64. Size work is DF0-F |
| Evidence stamp | `environment.json` records source commit 9a386a1 with a clean worktree. A rerun from that commit reproduced every verdict and value; only timings differ |
| Reproduce | `spikes/adr012-datafusion/scripts/run-df0b.sh` (needs `rustup`; builds outside the repository) |

### Portfolio (DF-002 to DF-009, DF-054, DF-061)

**B and C both match the legacy oracle on 24 of 24 cases**, overflow
included. Both return a typed overflow error instead of a scan failure.

**Finding F-B0, critical for any DataFusion money path.** A plain DataFusion
`SUM` over `BIGINT` **wraps silently**: `9223372036854775807 + 1` returns
`-9223372036854775808` with no error (`environment.json`
`datafusion_bigint_sum_overflow`). Candidate B avoids this by summing over
`DECIMAL(38,0)` and range-checking the result. Any production DataFusion
money aggregation must follow the same rule, with a test.

Single-run wall times (spawn included; DF0-E will measure properly):
- 50,000 projects: B 62 ms, C 3.7 ms.
- The overflow case: B 7.6 ms, C 0.013 ms.

These are observations, not performance results.

### Tabular formats (DF-011 to DF-018)

| Verdict | A (DuckDB) | B / C (DataFusion) |
|---|---|---|
| match | 38 | 39 |
| match with notes | 15 | 14 |
| loss | 5 | **1** |
| silent accept | 3 | **1** |
| valid file rejected | 0 | **3** |
| rejected (correct) | 2 | 4 |
| degenerate ok | 5 | 6 |
| ambiguous | 1 | 1 |

**Where B is better than A:**
- LD-1 is fixed: decimal256 is read exactly.
- LD-2 is fixed for CSV: big integers are kept exact as text.
- LD-4 is fixed: an empty CSV gives no phantom column.
- LD-5 is fixed for ragged CSV and invalid JSON: both are rejected.
- The truncation flag is correct in all 54 applicable cases.

**Parity gaps in B.** Each needs a fix or your approval before DF1:

| ID | Gap | Case | Possible fix |
|---|---|---|---|
| G-1 | A CSV header with duplicate names is rejected. DuckDB renames them `a`, `a_1` | `csv-duplicate-names` | Rename in the host or worker before inference |
| G-2 | A JSON field holding mixed types (int, string, float, bool) is rejected; arrow-json cannot decode it | `json-mixed-types` | Read such columns as JSON text |
| G-3 | One JSON number beyond binary64 (`1e400`) rejects the whole file. DuckDB reads `+Inf`, which is LD-3 | `json-decimals` | Decide: reject with a clear message (safer), or read the column as text |
| G-4 | An unterminated CSV quote is accepted silently, as in A | `csv-malformed-quote` | Strict quote checking in the reader |
| G-5 | A JSON integer beyond 2^53 in a float column loses precision, as in A (LD-2) | `json-big-int` | Infer DECIMAL or text for out-of-range integers |
| G-6 | Zero-padded codes need the spike's sniffer to stay text; DuckDB keeps them by default | `csv-leading-zeros` | Kept in the candidate (`zero_padded_columns`) |
| G-7 | DataFusion detects neither the CSV delimiter nor the header; the spike adds a dialect sniffer (`sniff.rs`). That is code GoPMgr would own | `.txt`, `semicolon.csv`, `noheader.csv` | Keep the sniffer, with its tests |
| G-8 | **A path with glob characters is expanded (`q?.csv` reads `q1.csv`), as in A (LD-6).** DataFusion's listing table treats `*`, `?` and `[` as patterns | `datafusion-filesystem.jsonl` | Host-mediated reads (DF-09) remove paths entirely |

**Configuration findings in B, fixed in the candidate:**
- **F-B1.** With more than one partition, `SELECT * … LIMIT` returned
  row groups in any order, so the preview was not the first 10,000 file
  rows. The candidate uses one partition for previews.
- **F-B2.** DataFusion's JSON schema inference returns columns in
  alphabetical order. arrow-json keeps file order only when serde_json's
  `preserve_order` feature is on. The candidate enables it and passes the
  schema in.

**Not yet evaluated:**
- Go↔Rust gRPC, DF0-C. DF-021 to DF-029 and DF-039/040 are NOT-RUN.
- Security and no-spill, DF0-D. DF-030 to DF-038 and DF-062/064 are NOT-RUN.
- Performance, DF0-E. DF-041 to DF-045, DF-050 and DF-065 are NOT-RUN.
- Packaging and licensing, DF0-F. DF-046 to DF-049 are NOT-RUN.
