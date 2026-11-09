<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012: DuckDB retirement and provider allocation

**Date:** 2026-11-02
**Decider:** GoPMgr maintainer
**Applies to:** [ADR-012](../design/ADR-012-datafusion-analytics-replacement.md)
and its [v0.2 acceptance](ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md), which
reserved the post-DF0 provider allocation and the cutover for the maintainer.

## Decision

The maintainer directed, in this session's chat, that DuckDB be replaced
"immediately", with macOS the primary platform, Linux second, and Windows not
a priority. Asked to choose between three replacement paths, the maintainer
chose a **Go-native engine for both analytics operations**:

| Operation | Provider | Notes |
|---|---|---|
| `PortfolioRollup` | Go, in the host process | Exact minor-unit sums with checked 128-bit accumulation, the DF0 option C rollup |
| `ImportTabular` (CSV, TSV, TXT, JSON, NDJSON) | Go standard library | Streaming readers |
| `ImportTabular` (Parquet) | Go, `github.com/apache/arrow-go/v18/parquet` | Pure Go and already in GoPMgr's module graph; becomes a direct dependency |

DuckDB is then removed from GoPMgr: no DuckDB module, build tag, native
library, notice file or packaging step remains (DF-G6).

## Paths not chosen

- **Go rollup now, DataFusion worker for imports later** (ADR-012 option C).
  DuckDB would have stayed until the Rust worker clears its macOS containment
  conditions (signed install, per-slot worker copies, condition 5).
- **Go-native without Parquet.** That would have dropped a supported format,
  which DF-G2 does not allow without explicit approval.

## What stays in force

- **DF-G2.** No accepted format or field is silently dropped. Every behavioural
  difference from DuckDB is recorded, with its reason, in the
  [DuckDB retirement parity record](../testing/duckdb-retirement-parity.md).
  Rows beyond the existing 10,000-row cap are now reported (`truncated`), not
  silently omitted.
- **No oracle.** Neither engine is its own correctness oracle (DF-A11).
  Expected values in the new tests are written by hand. DuckDB's outputs are
  kept only as the parity baseline.
- **SQLCipher** stays the system of record; the engine still never opens the
  encrypted project file.
- **The DF0 spike** under `spikes/adr012-datafusion` stays as isolated
  evidence for possible future columnar analytics. (Superseded 2026-11-09: the
  spike was concluded and archived; see the
  [DF0-G report](../testing/adr-012-df0-g-report.md).) Its DuckDB measurement
  commands (provider A) are frozen as history, because the engine they
  measured no longer exists.

## Consequences

- The release-compliance concerns tied to DuckDB end with it: its prebuilt
  native libraries, its notices, and the TPC-H/TPC-DS exclusion checks.
- Arrow-go's Parquet codecs bring new modules into the release binaries. They
  are reviewed in [DEPENDENCIES.md](../DEPENDENCIES.md), listed in the module
  notices and the SBOM, and scanned with `govulncheck`.
