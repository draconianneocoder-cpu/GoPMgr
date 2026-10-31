<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 DF0 evidence

DF0 is authorized and evidence-only
([acceptance record](../review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)).
This file records results phase by phase. Every result names its command, its
inputs and its evidence file. Historical environment records are observations
from those runs, not current toolchain declarations. The current pin and fresh
build evidence are recorded in [Rust 1.99.1 verification](adr-012-rust-1.99.1.md);
those compiler changes do not rerun or supersede the Linux DF0-A/B/C measurements.

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
| DF0-A baseline freeze | **Done on Linux amd64.** Native macOS engine-A baseline NOT-RUN in the current compiler slice. Windows x64: BLOCKED (no native host) |
| DF0-A2 money oracle | **Done** |
| DF0-B / B2 DataFusion and Go candidates | **Done on Linux amd64** (standalone engine; no IPC yet). The [macOS Rust 1.99.1 functional rerun](adr-012-rust-1.99.1.md) matches; performance qualification NOT-RUN. Windows: BLOCKED (no native host) |
| DF0-C Go↔Rust gRPC local IPC | **Done on Linux amd64** (Unix socket): 97 of 97 scenarios PASS. macOS: full suite NOT-RUN; socket/secret inspection remains BLOCKED because the Darwin inspector is unimplemented. Windows named pipes: BLOCKED (no host; endpoint not yet written) |
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
| Toolchain recorded for this run | rustc 1.97.0; MSRV 1.94. The current build pin is 1.99.1; see [toolchain verification](adr-012-rust-1.99.1.md). |
| Release binary | 139.5 MB unstripped, Linux amd64. Size work is DF0-F |
| Evidence stamp | `environment.json` records source commit 5d238a5 with a clean worktree (the Codex review fixes below). Compared with the first run, at 9a386a1, every verdict and value is the same; only timings and the new `error_class` field differ |
| Reproduce | `spikes/adr012-datafusion/scripts/run-df0b.sh` (needs `rustup`; builds outside the repository) |

### Portfolio (DF-002 to DF-009, DF-054, DF-061)

**B and C both match the legacy oracle on 24 of 24 cases**, overflow
included. Both return a typed overflow error instead of a scan failure.

**Codex review fixes (2026-10-30):**
- **Overflow checks only the final total.** C used to reject a running
  total outside int64 even when the final total fit, so `[MaxInt64, 1, -1]`
  failed or passed depending on project order. C now sums exactly in
  128 bits and checks only the final totals, as the oracle and DuckDB's
  `HUGEINT` SUM do. B already did. No corpus case had cancelling amounts
  that large, so the scores did not change; unit tests now pin it.
- **Scoring separates overflow from failure.** Any error used to pass an
  overflow case, including a crashed or timed-out CLI run or malformed
  output. Records now carry `error_class`, and only `overflow` matches. All
  six overflow passes are typed overflow results.

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
| G-7 | DataFusion detects neither the CSV delimiter nor the header; the spike adds a dialect sniffer (`sniff.rs`). That is code GoPMgr would own. The first version called any first record with a text field a header, so a headerless `1,Alpha,10.50` file lost its first row (Codex review). It now compares the first record with later records column by column, as DuckDB does. No corpus file has headerless mixed text and numbers, so the verdicts did not change; a corpus case should be added at DF1 | `.txt`, `semicolon.csv`, `noheader.csv` | Keep the sniffer, with its tests |
| G-8 | **A path with glob characters is expanded (`q?.csv` reads `q1.csv`), as in A (LD-6).** DataFusion's listing table treats `*`, `?` and `[` as patterns | `datafusion-filesystem.jsonl` | Host-mediated reads (DF-09) remove paths entirely |

**Configuration findings in B, fixed in the candidate:**
- **F-B1.** With more than one partition, `SELECT * … LIMIT` returned
  row groups in any order, so the preview was not the first 10,000 file
  rows. The candidate uses one partition for previews.
- **F-B2.** DataFusion's JSON schema inference returns columns in
  alphabetical order. arrow-json keeps file order only when serde_json's
  `preserve_order` feature is on. The candidate enables it and passes the
  schema in.

**Not yet evaluated (after DF0-B):**
- Go↔Rust gRPC, DF0-C: now evaluated on Linux, below.
- Security and no-spill, DF0-D. DF-030 to DF-038 and DF-062/064 are NOT-RUN.
- Performance, DF0-E. DF-041 to DF-045, DF-050 and DF-065 are NOT-RUN.
- Packaging and licensing, DF0-F. DF-046 to DF-049 are NOT-RUN.

## DF0-C — Go↔Rust gRPC local IPC (Linux amd64)

**What runs.** The Go host (`internal/ipchost`) is the gRPC server and the
Rust worker (`rust/worker`) is the client, on one bidirectional stream, as
ADR-010 G0/G1 sets out. For every job the host:

- creates a fresh `0700` directory with a `0600` socket, and refuses any
  pre-placed file, link or stale socket;
- launches one worker with an **empty environment**, writing a one-time
  32-byte credential on its **stdin**, never in argv;
- admits only a connection whose SO_PEERCRED PID is that worker, and only
  one. Connections that arrive before the PID is known are held, not
  refused;
- accepts the credential once, before any business message, and not after
  it expires.

The worker checks, before sending a byte, that the socket is served by its
parent under its own user. This makes the authentication mutual. Business
data is Arrow IPC inside gRPC messages, capped at 4 MiB each way. There is
no TCP connector and no stdio business channel.

| Item | Value |
|---|---|
| Contract | `proto/df0ipc.proto`, spike-only. Go and Rust code generated by `scripts/generate-df0ipc.sh` with protoc 25.9, protoc-gen-go v1.36.12, protoc-gen-go-grpc 1.6.2 and tonic-prost-build 0.14.6, and committed |
| Go modules | grpc v1.84.0 and protobuf v1.36.12. The first run used grpc v1.83.2; the root module now requires v1.84.0 (G1-1, #143), which the spike reaches through its `replace`. Engine A's dependency set is unchanged |
| New crates | tonic 0.14.6 (`channel`, `codegen` only: no server, no TLS), tonic-prost 0.14.6, prost 0.14.4, hyper 1.12.0, hyper-util 0.1.21, h2 0.4.20, tower 0.5.3, sha2 0.10.9 and 22 transitive crates; no existing crate changed version |
| Evidence | `evidence/df0-c/ipc-linux-amd64.jsonl` (97 records) and `environment.json`: commit f29f10c, clean worktree. This rerun includes the Codex review fixes below; the first run was b859fd4 |
| Reproduce | `spikes/adr012-datafusion/scripts/run-df0c.sh` |

### Results

Every launch also checks the invariants: no TCP or UDP socket in the
worker or host, no credential in argv or environment, an empty worker
environment, private modes, the endpoint removed, the worker reaped. A
**detector self-test** runs first. It shows the socket detector seeing a
planted TCP listener and UDP socket, and the leak detector seeing planted
argv and environment sentinels. Only then are "none found" results
trusted.

| Scenario | Result |
|---|---|
| 1, 64 and 1,024 batches | PASS. Every row is recomputed by the host |
| Batch over the frame cap | PASS. The worker splits it (largest frame 4,116,552 bytes) |
| 1.08 GB (1,076,082,576 bytes) streamed, 104,858 rows | PASS in 3.47 s. Host heap peak 67 MB; worker peak RSS 103 MB. Nothing is held whole |
| Full duplex: 64 MiB to the worker while rows stream back | PASS. The worker's source digest matches |
| Portfolio rollup over gRPC | PASS: **24 of 24** equal the legacy oracle |
| Slow consumer (2 ms per batch) | PASS. The worker waited on flow control 67 times; RSS 57 MB |
| Cancel while the worker is blocked on a full stream | PASS. `USER_CANCELLED` after 50 ms; clean exit after 83 ms |
| Cancel after the rows are done but before the upload is (duplex) | PASS. The host stops uploading after 4 chunks and cancels; `USER_CANCELLED` after 1.8 ms, clean exit after 3.9 ms. The previous worker build waited for the rest of the upload and was killed at the 10 s deadline (Codex review) |
| Sequence gap; repeat; changed Arrow schema; wrong digest | PASS. Each is refused and the worker killed (fail closed) |
| Worker exits mid-stream | PASS. Reported, reaped (exit 9) |
| Worker hangs and ignores Cancel and Finish | PASS. Killed at the 5 s job deadline |
| Wrong credential | PASS. Refused before any business message; the worker exits 3 |
| Expired bootstrap | PASS. The worker refuses it and never connects |
| Foreign process dials first | PASS. Held, then refused; it receives 0 bytes; the worker then completes |
| Endpoint served by a process that did not launch the worker | PASS. The worker refuses it and sends 0 bytes |
| Pre-placed link at the endpoint path | PASS. Refused |
| Host killed with SIGKILL mid-job | PASS. The kill now waits until the host has verified its first batch, so the worker is streaming. It is dead 1.0 ms later (parent-death signal). The killed host leaves its own endpoint directory: the production supervisor must sweep these at start. The scenario removes only that directory |
| 50 fresh cancel cycles | PASS. Host fds 9→9, threads 10→10, goroutines 1→1, no endpoint left |

Latency, Linux amd64 (n is small; p99 is not estimable and not reported):

| Measure | n | min | p50 | p90 | max |
|---|---|---|---|---|---|
| Spawn to Hello | 90 | 4.1 ms | 4.8 ms | 5.8 ms | 8.8 ms |
| Cancel to `USER_CANCELLED` | 50 | 0.70 ms | 1.8 ms | 3.3 ms | 6.3 ms |
| Cancel to worker exit | 50 | 2.7 ms | 4.2 ms | 5.8 ms | 10.0 ms |

The host's refusal paths are also unit-tested in `ipchost_test.go`, against
real sockets and processes:
- a fake worker (the test binary re-executed) sends a wrong protocol
  major, a wrong PID in Hello, a batch before Hello, garbage Arrow and an
  oversize frame;
- the proof is shown to be refused when wrong, missing, reused or expired.

All of it passes under `-race`, and one full scenario run under `-race`
also passes.

### Findings

- **F-C1.** Without IPv6, `/proc/net/tcp6` and `udp6` do not exist. The
  detector records them as absent tables (no such socket can exist). It
  does not fail silently or pass silently.
- **F-C2.** `kill(pid, 0)` succeeds on a zombie. An orphan check must read
  the process state; the first version measured init's reaping (1.6 s)
  instead.
- **F-C3.** tonic reports connector failures as "transport error" and hides
  the cause. The worker prints the whole error chain, so refusals are
  diagnosable.
- **F-C4, for G1.** The Rust worker connects about 4 ms after spawn. The
  G0 listener refused any connection that arrived before the worker's PID
  was recorded; with a fast worker that is a race. The DF0-C listener holds
  early connections until the PID is known, then decides.
- **F-C6, Codex review (2026-10-30).** Five harness and worker defects,
  all fixed in f29f10c and covered by this rerun:
  - `df0-ipc` exited 0 with failed scenarios. It now exits 1 after writing
    the evidence.
  - A rollup overflow case passed on any error, including a crashed worker.
    Only a completed run carrying the worker's overflow result now passes.
  - The orphan scenario removed every DF0 endpoint directory, including
    another live job's. It now removes only its own.
  - The orphan host was killed before the job started, so the scenario could
    pass without testing host death mid-job. The kill now waits for a
    verified batch.
  - The worker's duplex path waited for the whole source upload before
    honoring Cancel. That wait, and the rollup source read, now observe
    Cancel.
- **F-C5, for DF0-F.** tonic's `channel` feature still links hyper's TCP
  connector and `socket2`. They are unused: every run shows no IP socket.
  But they are in the binary, so "no TCP" rests on code paths, not on what
  is linked.

### Test status after DF0-C

| Test | Linux amd64 | macOS | Windows |
|---|---|---|---|
| DF-021 Linux UDS without TCP | **PASS** | — | — |
| DF-022 macOS UDS | — | **BLOCKED**: no macOS host. Host and worker code compile for darwin; process inspection is not implemented there | — |
| DF-023 Windows named pipe | — | — | **BLOCKED**: no Windows host, and the named-pipe endpoint (go-winio host, tokio `NamedPipeClient` worker) is not written yet |
| DF-024 mutual authentication, replay refused | **PASS** | BLOCKED | BLOCKED |
| DF-025 Windows pipe ownership, DACL, anti-squat | — | — | **BLOCKED** |
| DF-026 private mode, peer PID, stale-socket refusal | **PASS** | BLOCKED | — |
| DF-027 bounded frames; bad sequence and schema refused | **PASS** | BLOCKED | BLOCKED |
| DF-028 backpressure with bounded memory, then cancel | **PASS** | BLOCKED | BLOCKED |
| DF-029 crash, hang, kill, process tree | **PASS** for crash, hang, host kill and 50 cycles. OOM is **NOT-RUN** (DF0-D2) | BLOCKED | BLOCKED |
| DF-037 nothing accepted before the peer is known | **PASS** (held, then refused) | BLOCKED | BLOCKED |
| DF-039 deterministic cancel and timeout | **PASS** | BLOCKED | BLOCKED |
| DF-040 version incompatibility refused | **PASS**: protocol major (unit test) and Arrow schema change | BLOCKED | BLOCKED |
| DF-036 argv/env leak (partial) | Launch credential: **PASS**, with detector self-test. The sentinel corpus over logs and errors is DF0-D | — | — |

**Not claimed.** This is a spike host, not the G1 supervisor. Signed and
packaged execution, and G1 itself, are later gates (DF0-C item 7). The
remaining phases are NOT-RUN:
- DF0-C2: host-mediated reads, which also remove G-8;
- DF0-D: security and no-spill;
- DF0-E: performance across A, B and C;
- DF0-F: packaging and licensing.
