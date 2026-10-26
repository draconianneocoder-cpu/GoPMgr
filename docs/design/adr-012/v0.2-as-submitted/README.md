<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr ADR-012 v0.2 — DataFusion replacement architecture and feasibility bundle

**Date:** 2026-10-26 · **Status:** PROPOSED, not accepted and not implemented. This is a revised design package, not proof that the replacement exists.

## Canonical revised documents

1. [`docs/design/ADR-012-datafusion-analytics-replacement.md`](docs/design/ADR-012-datafusion-analytics-replacement.md) — revised ADR, binding **proposed** architectural decisions, DF-A01–DF-A14, release/license gate, authority and rollback.
2. [`docs/testing/adr-012-datafusion-feasibility-spike.md`](docs/testing/adr-012-datafusion-feasibility-spike.md) — independently executable DF0 plan, **65 DF test cases**, privacy and three-provider benchmarks.
3. [`docs/agent/adr-012-coding-agent-instructions.md`](docs/agent/adr-012-coding-agent-instructions.md) — expanded agent engineering safeguards.
4. [`docs/agent/adr-012-owner-prompt-to-paste.md`](docs/agent/adr-012-owner-prompt-to-paste.md) — **standalone copyable prompt** to send the agent with this archive.
5. [`docs/review/adr-012-v0.2-improvements-and-traceability.md`](docs/review/adr-012-v0.2-improvements-and-traceability.md) — detailed amendment resolution and decisions requiring approval.
6. `reference/v0.1/` — unchanged archival copies of earlier documents for comparison; **not operative**.
7. `SHA256SUMS.txt` — content checksums; verify from the archive root with `sha256sum -c SHA256SUMS.txt` where available.

## Critical corrections from the prior critique

- Independence of **release licensing** (PR #146 TPC-H/TPC-DS compiled DuckDB native code) from **functional DataFusion cutover**. A notice alone is not legal compatibility proof.
- Compare **DuckDB vs DataFusion-for-all vs checked-Go-rollup + DataFusion** before settling ownership.
- ADR-002 remains historically implemented until verified cutover.
- Enforced `SENSITIVE_MEMORY_ONLY` for private project analytics; protected spill requires separate approval.
- Correct money field-presence semantics without silently changing legacy portfolio results.
- Explicit Protobuf/Arrow and exact Wails/JavaScript number contracts, schema compatibility, and stream accounting.
- Host-mediated bounded Parquet seeking as a genuine performance candidate, no file-system delegation to worker.
- Single allowlisted analytical execution façade and independently enforced OS/host/DataFusion restrictions.
- Aggregate worker resource limits, statistically honest p99, absolute **and** relative UX budgets, independent test oracles, cross-platform signed release gates, pinned DataFusion 55.2.0 candidate and TPC-free fixtures.

## Source baseline and authority

GoPMgr repository `main` inspected at `28a758d154795bb2a20f3cbeb63a12e9130a88ea`; ADR-010 v2.2, PR #141 (G0) and #142 (S0), #146 (DuckDB licensing) contextualized. **Recheck live HEAD and owner approvals.** ADR-011 storage/migrations remain independent. The user has requested a draft, not authorized production code or DuckDB dependency removal.

## Execution caution

DF0 may begin only after explicit owner authorization; G1 supervisor and Rust local IPC feasibility must be demonstrated. Do not conflate an unsigned spike with installed/signed product verification. No executable tests or DataFusion benchmarks were run as part of this Markdown authoring.
