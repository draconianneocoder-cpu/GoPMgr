<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr — ADR-012 DataFusion replacement design bundle

**Created:** 2026-10-26  
**Status:** **PROPOSED; not accepted, coded, or measured.**  
**Repository baseline:** GoPMgr `main` `28a758d` with ADR-010 v2.2 and PRs #141/#142 considered. Reinspect HEAD before execution.

## Contents

1. [`docs/design/ADR-012-datafusion-analytics-replacement.md`](docs/design/ADR-012-datafusion-analytics-replacement.md) — detailed architecture decision, rationale, contracts, risk register, staged cutover and owner gates.
2. [`docs/testing/adr-012-datafusion-feasibility-spike.md`](docs/testing/adr-012-datafusion-feasibility-spike.md) — independently runnable spike specification, strict fixture/benchmark/security plan and **60 unique acceptance cases**.
3. [`docs/agent/adr-012-coding-agent-instructions.md`](docs/agent/adr-012-coding-agent-instructions.md) — architecture-preservation instruction for a coding agent. No unauthorized downgrades or premature DuckDB removal.
4. `SHA256SUMS.txt` — exact file hashes for this bundle.

## Priority conclusion

DataFusion is favored **for long-term architecture**, *not* for promised query speed. GoPMgr currently uses DuckDB only for `PortfolioRollup` and bounded `ImportTabular` reads. The Rust worker will likely incur extra cold-start/IPC overhead for small rollups. A successful replacement must preserve all current behavior and demonstrate acceptable end-to-end performance and security on advertised desktops.

**Earliest technical start:** DF0 isolated Rust engine and Go↔Rust local IPC spike, in parallel with ADR-010 G1 evidence. **Earliest production integration:** after the common G1 worker supervisor and the exact analytics/protocol contracts are approved. **DuckDB removal:** after all functional, performance, safety, cross-platform packaging and rollback gates pass, under a separate owner cutover decision. **ADR-011 normalized storage is not a prerequisite** for replacing the *existing* two analytics methods; it is needed for later S7A expansion.

## Repository placement (proposal only)

Place the ADR under `docs/design/` once its identifier is confirmed unused, the spike under `docs/testing/`, and the agent guidance under `docs/agent/`. ADR-011 remains reserved for normalized encrypted schedule storage. The currently implemented ADR-002 is retained historically and superseded for analytics-engine selection only upon an approved, successfully verified cutover.

## What has and has not been done

- Done: reviewed existing analytics source/test API, `Makefile`, `go.mod`, ADR-002 and accepted ADR-010 plan, PR #142 source baseline and upstream DataFusion guidance. Authored architecture, spike and safeguards.
- Not done: built DataFusion, executed Rust or Go benchmarks, proved Rust named-pipe IPC, installed signed packages, run project migrations or edited the GitHub repository.
- Never infer experimental performance improvement or release readiness from this planning bundle.

## References

- [Apache DataFusion](https://datafusion.apache.org/), [security guide](https://datafusion.apache.org/library-user-guide/securing-datafusion.html)
- [Apache Arrow Rust](https://github.com/apache/arrow-rs)
- [GoPMgr repository](https://github.com/draconianneocoder-cpu/GoPMgr)
- [GoPMgr P01/G0 gRPC spike](https://github.com/draconianneocoder-cpu/GoPMgr/pull/141) and [P02/S0 performance baseline](https://github.com/draconianneocoder-cpu/GoPMgr/pull/142)
