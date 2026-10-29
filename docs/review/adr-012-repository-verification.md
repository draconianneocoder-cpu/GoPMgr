<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 v0.2: repository and acceptance-contract verification

**Date:** 2026-10-26
**Scope:** checks every factual claim in the accepted ADR-012 v0.2 package
against the repository, the PR stack and upstream sources before DF0 begins.
**Baseline inspected:** `main` at `28a758d154795bb2a20f3cbeb63a12e9130a88ea`,
the commit the package names. It was still `main`'s head on 2026-10-26.
ADR-010 documents are on branch `claude/adr-010-plugin-architecture` and the
G1 branches, not yet on `main`.

None of this changes the accepted architecture. Items marked **Erratum**
correct a factual premise. Items marked **Finding** are facts the package did
not record and that DF0 has to account for. Each one is proposed to the
maintainer. The normative text is unchanged.

## 1. Package integrity and contract shape

| Check | Result |
|---|---|
| `SHA256SUMS.txt` against the 10 archive files | all OK |
| Unique test IDs in the v0.2 specification | 65, DF-001 to DF-065, no duplicates |
| DF-001 to DF-060 against v0.1 | each row is word-for-word identical (ID, requirement, gate), so the "preserved unchanged" claim holds |
| ADR numbers | ADR-001 to ADR-009 are on `main`, ADR-010 is on its branch, ADR-011 is unused (reserved), and ADR-012 was unused before this package |
| `3d00f53` (ADR-010 P00) | exists, on the ADR-010 branches |

## 2. Claims confirmed

| Package claim | Evidence |
|---|---|
| Production Wails builds select `duckdb,webkit2_41` | `Makefile:19` `WAILS_BUILD_TAGS ?= duckdb,webkit2_41` |
| `analytics.Engine` has `PortfolioRollup`, `ImportTabular`, `Available` and `Close` | `internal/analytics/analytics.go:86` |
| In-memory DuckDB with no network extension install | `duckdb.go` `harden`: `SET autoinstall_known_extensions=false` |
| CSV, TSV and TXT go through `read_csv_auto`, Parquet through `read_parquet`, and JSON and NDJSON through `read_json_auto` | `duckdb.go` `tabularReader` |
| At most 10,000 rows | `maxImportRows = 10000`, applied as `LIMIT` |
| Money as `int64` minor units with float fallback; EVM unavailable is distinct from zero | `ProjectMetrics`, `normaliseProjectMetricsMoney`, and the `evm_available` CASE sums |
| SPI = ΣEV/ΣPV and CPI = ΣEV/ΣAC, with 0 meaning n/a | `duckdb.go` after the aggregate |
| A `!duckdb` stub returns unavailable | `stub.go`, `TestStubReportsUnavailable` |
| G0 proved Go↔Java, not Go↔Rust | PR #141. The G1 design names the host as gRPC server and the worker as client (`adr-010-g1-transport-design.md` §3) |
| DataFusion 55.2.0 is the current stable release | crates.io: 55.2.0 published 2026-10-07, `rust_version` 1.94.0, Apache-2.0. 55.1.0 was published 2026-09-11 and 55.0.0 on 2026-08-18 |
| Advertised platforms | `docs/INSTALL.md`: Windows 10/11 x64, macOS 13+ Apple Silicon, Debian/Ubuntu x64 (`.deb`), Fedora/RHEL/openSUSE x64 (`.rpm`). ADR-010 P04-D5 keeps Windows 10 x64 in the target |
| DF-001 baseline | `go test -tags duckdb ./internal/analytics/...` passes all 8 tests, and the stub test passes (Linux amd64, Go 1.27.3, 2026-10-26) |

## 3. Errata

### E-1 — the TPC code is not compiled into GoPMgr's binaries (DF-A01 premise)

The package says, in several places, that the distributed DuckDB bindings
"include TPC-H and TPC-DS benchmark implementations" that are "compiled into
DuckDB static libraries". It also says the production DuckDB build "must not
be newly distributed … until LC-GATE passes". Those places are:

- DF-A01;
- spike §0 and §8A;
- agent safeguard 1;
- the README.

That premise came from the first version of PR #146, which has since been
corrected. The audit in
[`TPC-benchmark-exclusion-decision.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/claude/release-duckdb-notices/docs/review/TPC-benchmark-exclusion-decision.md)
(PR #146) shows the following:

- DuckDB builds `tpch` and `tpcds` with `DONT_LINK`. They ship as separate
  archives that nothing references, so no binary contains their code.
- This holds on all five platform modules, in v0.10505.0 and v0.10506.0. No
  symbol the TPC archives define is referenced by any linked archive or by the
  bindings' 632 cgo names.
- The v1.1.0-alpha.2 `.deb`, `.rpm`, `.dmg` and Windows binaries contain 0 of
  the 8 TPC code markers, and do contain DuckDB.
- The `dbgen` and `dsdgen` strings come from DuckDB core's extension-autoload
  name table. That table is data, not code.

**Consequence:** LC-GATE stays exactly as accepted: it is independent and
release-blocking. Its condition (i), "a proven TPC-free DuckDB binary
retaining required functionality", now has per-artifact evidence and a CI
gate for binaries. The maintainer still has to decide whether that satisfies
the decision. The vendored Debian source package remains blocked, because
`go mod vendor` would copy the TPC archives. The urgency of the DuckDB
replacement therefore rests on ADR-012's own merits, not on a licensing
emergency in the shipped binaries.

## 4. Findings DF0 must account for

**F-1 — the Wails boundary already sends portfolio money as decimal strings.**
- `app_portfolio_wire.go` `PortfolioSummaryWire` converts every total with
  `money.Amount.DecimalFor(currency)` and computes `remaining` exactly.
- So DF-065's JavaScript precision risk does not apply to portfolio money
  today, and DF0 must not "fix" it there.
- The live exposure is `ImportTabular`: `Dataset.Rows [][]any` carries raw
  driver values (`int64`, HUGEINT as `*big.Int`, DECIMAL, timestamps, nested
  values) through `encoding/json` to JavaScript numbers.
- DF0-A characterizes the exact current output per type as legacy behavior.

**F-2 — mixed currencies and the reporting date are already enforced in the host.**
- `RunPortfolioAnalytics` (`app_foundation.go:894`) rejects a second currency
  with `portfolioMixedCurrencyError` before any aggregation.
- It evaluates every project at one UTC reporting date.
- So DF-007 and DF-010 are legacy behavior to preserve at the host, not new
  engine work. The engine never sees currency codes.

**F-3 — the shipped baseline is cold on every call, and the engine is a small part of the work.**
- `RunPortfolioAnalytics` and `ImportDatasetForAnalysis` each call
  `analytics.New()`, which runs `sql.Open("duckdb", "")`, and `Close` on every
  call.
- `RunPortfolioAnalytics` also opens and decrypts every project, loads its
  schedule, and runs `kernel.ComputeEVM` in Go before the rollup. The rollup
  itself is seven sums over one row per project.
- Benchmarks must measure the whole `RunPortfolioAnalytics` path, with its
  breakdown, not just the engine. This is the misattribution risk DF-R8/DF-R12
  warn about.
- A "warm DuckDB" number does not describe what ships.

**F-4 — the import preview already misstates large files (existing defect).**
- `Portfolio.svelte` prints "{columns} columns × {rows.length} rows", but rows
  are capped at 10,000. A 50,000-row file is reported as "10000 rows".
- `ImportTabular` cannot tell truncation apart, because it uses `LIMIT 10000`
  with no count.
- This is the defect DF-015 and DF-016 guard against, and it exists in
  production now, independent of any engine.
- Proposed separate fix: a small production PR that requests `LIMIT 10001`,
  reports `truncated`, and labels the UI. It needs its own authorization,
  because DF0 does not touch production code.

**F-5 — unreadable projects are skipped silently in the rollup.**
- `RunPortfolioAnalytics` `continue`s past a project that fails to open or
  read ("matching ProjectsOverview"), so `project_count` excludes it without
  saying so.
- This is legacy behavior to preserve or explicitly change. DF0 records it in
  the parity ledger.

**F-6 — DuckDB's directory restriction is best-effort, and may be inactive (to verify in DF0-A).**
- `ImportTabular` runs `SET allowed_directories=[...]` and discards the
  error.
- DuckDB applies `allowed_directories` only when external access is disabled,
  and the code does not disable it. This understanding is to be confirmed
  against v1.5.6.
- If the restriction is inactive, baseline A's filesystem confinement is
  weaker than its comment says. The DF0 security comparison must use the
  measured posture of each candidate, not the intended one.

**F-7 — overflow behavior of the shipped rollup (to verify in DF0-A).**
- DuckDB `sum(BIGINT)` returns HUGEINT. Scanning it into `int64` should fail
  with an out-of-range error rather than wrap.
- DF0-A records the actual behavior per sum as the DF-005 legacy result.

**F-8 — the unsupported-type error lists 4 of the 6 extensions.**
- The message says "want .csv/.tsv/.parquet/.json", but `.txt` and `.ndjson`
  are also accepted. It is a minor copy defect, recorded for the parity
  ledger.

**F-9 — the Rust stack needs its own dependency review before DF1.**
- ADR-010 P04-D2 conditionally approved grpc-go, protobuf-go and go-winio
  only.
- The DataFusion, Arrow, tonic, tokio and prost crate graph is new. DF0 may
  build it in isolation (DF-A13). Production use needs a review like P04-D2
  and a Cargo notices pipeline, matching the Go-module, DuckDB and frontend
  notice gates PRs #144–#149 added.

**F-10 — DataFusion's repository has TPC-derived benchmark code.**
- Whether any crate that DF0 links ships TPC-derived code or data is to be
  checked in DF0-F2. Inspect the packaged crate contents, not the repository.
- The TPC exclusion gate (`third_party/EXCLUDED-NATIVE-CODE.txt`) is
  DuckDB-specific. A Rust worker needs equivalent markers.

## 5. Isolation plan for DF0 (how DF-051 and DF-060 are met)

- All DF0 code lives under `spikes/adr012-datafusion/`, in its own Go module,
  `gopmgr/spikes/adr012-datafusion`, plus a separate Cargo workspace.
  - Because its import path starts with `gopmgr/`, the Go spike can import
    `gopmgr/internal/analytics` through a `replace gopmgr => ../..`
    directive. That is how it runs the shipped DuckDB engine unmodified.
  - Its directory has its own `go.mod`, so the main module's packages,
    `go.mod`, `go.sum`, `make verify`, the code map and the release scope do
    not change.
- Fixtures are synthetic, generated by seed-pinned code, and listed in a
  manifest with SHA-256 hashes. No TPC generator, query or dataset is used.
- Evidence records are JSONL under `spikes/adr012-datafusion/evidence/`, with
  no user paths or data.
- Platforms this container cannot run are BLOCKED until they run elsewhere:
  - native macOS arm64;
  - Windows 10/11 x64;
  - signed and installed packages.
