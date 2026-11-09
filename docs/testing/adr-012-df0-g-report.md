<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 DF0-G: go/no-go report

**Date:** 2026-11-09
**Decider:** GoPMgr maintainer
**Decision:** **No-go.** DataFusion is not adopted. The DF0 feasibility spike
is concluded and archived.

DF0-G is the last phase of the
[DataFusion feasibility spike](adr-012-datafusion-feasibility-spike.md): the
report that asks the maintainer to adopt the candidate or not. It had not run
when the spike stopped.

## Why

- **The question changed.** DF0 compared DataFusion (candidate B) and a Go
  rollup with DataFusion (candidate C) against DuckDB (engine A). On
  2026-11-02 the maintainer replaced DuckDB with a Go-native engine for both
  analytics operations
  ([owner decision](../review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md)).
  The portfolio rollup and the 10,000-row data import now run in Go, with
  exact money and a declared parity record. No current feature needs a
  columnar engine.
- **The open gates are large.** Adopting the Rust worker would still need
  OS-wide spill tracing (DF-062), aggregate memory and CPU enforcement
  (DF-064), controlled performance qualification (DF0-E), complete licensing
  and native-code review (DF0-F, LC-GATE), Linux and Windows transport parity,
  and signed, installed testing on macOS 26.
- **Keeping it costs upkeep.** The spike carried 374 vendored crate files,
  needed its own Zstd security override
  ([assessment](adr-012-df0-zstd-assessment.md)) and kept Rust toolchain and
  source pins under `make config-check`, although nothing shipped it.

## What DF0 established

| Phase | Result |
|---|---|
| DF0-A, A2 | Baseline and money oracle done (Linux amd64) |
| DF0-B, B2 | DataFusion and Go candidates matched the baseline (Linux amd64; macOS functional rerun) |
| DF0-C | Go↔Rust gRPC over a Unix socket: 97 of 97 scenarios passed (Linux amd64) |
| DF0-C2 | Host-mediated Parquet reads passed on macOS arm64 |
| DF0-D, D2 | Partial: no-spill controls, per-child resource accounting, admission checks, App Sandbox observer and memory-only experiments on macOS; DF-062 blocked, DF-064 unqualified |
| DF0-E, E2 | Partial: 306 oracle-checked portfolio calls and a typed-cell fidelity harness; controlled performance not run |
| DF0-F, F2 | Partial: dependency inventory, clean advisory scan, SPDX build-input SBOM, native-source notice review; complete licensing not done |
| DF0-G | This report: no-go |

The [evidence record](adr-012-df0-evidence.md) has every result, command and
limit. The raw records are in [adr-012-df0-records](adr-012-df0-records/).
Findings that still hold for any future columnar work: exact decimals and
large integers must cross the wire as text; NaN and infinities must not reach
the Wails bridge; a worker needs host-mediated reads, no-spill controls and
real aggregate resource limits before it can handle user files.

## What happens now

- The spike's code is removed from the active tree and preserved, complete,
  at the annotated tag
  [`archive/adr012-datafusion-spike`](https://github.com/draconianneocoder-cpu/GoPMgr/tree/archive/adr012-datafusion-spike/spikes/adr012-datafusion).
  To restore it: `git checkout archive/adr012-datafusion-spike -- spikes/adr012-datafusion`.
- The evidence documents stay in `docs/testing/` as records. The
  [containment decision request](../review/ADR-012-macos-containment-ACR.md)
  is closed without adoption.
- The SPDX 3.0.1 validator the spike used moves to
  `tools/sbom/validate_spdx3.py`, because release SBOMs use it too.
- ADR-012 stays accepted as the analytics architecture record. Reopening a
  columnar engine needs a new owner decision tied to a defined need in PS-09
  (analysis, comparison and reporting), such as data beyond the 10,000-row
  preview or columnar analysis of large schedules.
