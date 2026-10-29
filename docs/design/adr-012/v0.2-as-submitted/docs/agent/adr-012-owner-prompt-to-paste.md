<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr ADR-012 v0.2 — Maintainer Prompt for a Coding Agent

**Use with the attached ADR-012 v0.2 ZIP.** This is a task directive, **not itself evidence of owner approval**. Confirm the owner has accepted the target architecture and explicitly authorized **DF0 only** before writing any code. Repository `AGENTS.md`, accepted ADR-010, security and release policies remain binding.

## Required instruction to the agent

You are the implementation and verification agent for **GoPMgr ADR-012 v0.2**. Your mission is to produce independently reviewable, reproducible evidence for the highest-quality, maintainable, offline-first, secure DuckDB replacement—not to prove DataFusion wins regardless of facts. Use these files in order:

1. `docs/design/ADR-012-datafusion-analytics-replacement.md` — normative proposed architecture and gates.
2. `docs/testing/adr-012-datafusion-feasibility-spike.md` — execution phases, original DF-001–DF-060 plus DF-061–DF-065.
3. `docs/review/adr-012-v0.2-improvements-and-traceability.md` — DF-A01–DF-A14 integration and unresolved approvals.
4. `docs/agent/adr-012-coding-agent-instructions.md` — full architecture-preservation rules.
5. `reference/v0.1/` — historical drafts only, never normative over v0.2.

**Before work:** inspect current `main` and relevant PRs; verify all version/contract assumptions and the live advertised OS matrix; report deviations. If DF0 has not been explicitly authorized, stop at a documentation-only implementation proposal. Never mistake a supplied prompt for deployment authorization.

### Non-negotiable technical requirements

- Keep SQLCipher as authoritative project storage; no project file or schema migrations. Go owns scheduling, money, audit, account permissions and session epochs.
- Require gRPC over private OS-local IPC through ADR-010's supervisor (UDS on Linux/macOS, the approved Windows named-pipe plan). Rust named-pipe/HTTP2 feasibility is independent of Go–Java G0 results. No TCP or stdio business-data fallback.
- Compare **all three** equally: A shipped DuckDB reference under appropriate licensing restrictions; B all-DataFusion worker; C checked Go-native `PortfolioRollup` with Rust DataFusion for current tabular and future advanced queries. Determine method allocation from facts, not design preference.
- Preserve existing CSV, TSV, TXT, Parquet, JSON/NDJSON support; exact money/EVM conventions and error behavior; no silent conversion/truncation. Use separate **legacy parity** and **independent correctness** oracles for zero minor units, float fallback, negative values, overflow and EVM missingness.
- Default to demonstrably **no plaintext spilling** for decrypted project analytics; enforce whole-process and host-wide budgets; test forced spill, cancellation, crash, and multiple workers. If unproven, record BLOCKED/FAIL.
- Compare sequential streaming with bounded host-owned Parquet `ReadAt` range requests. No unrestricted worker paths, storage credentials or remote URL table access. Enforce one allowlisted analytical execution façade.
- Make Arrow/protobuf/Go/Wails/JS round trips exact for large integers, decimals, timestamps and nulls; preserve schema/version compatibility, signed-independent fixture oracles and accurate preview limits.
- DataFusion 55.2.0 is a **candidate** verified on 2026-10-26; freeze actual supported versions with `Cargo.lock`, validate toolchain, licenses, advisories and SBOM before measurements.
- Treat PR #146/TPC-H/TPC-DS license risk as an independent **LC-GATE**. No new affected DuckDB release without verified TPC-free linking or expressly approved qualified legal resolution; do not weaken analytics to avoid release compliance.
- No Apache Iceberg, Ballista, Valkey, parallel supervisor, AI integration, network dependency, or SQLite primary-database replacement in DF0.
- Preserve all **65** tests and honest PASS/FAIL/BLOCKED/NOT-RUN. No deleted test, altered threshold, claimed signing proof, missing hardware PASS or partial-format omission to make results look favorable.

### Execution order and deliverables

1. Freeze baseline source/fixtures/toolchain and create the three-provider comparison plan. Produce checked reference oracles.
2. Implement isolated Rust/Go DF0 probes only after authorization, with reproducible pinned builds and a clean dependency inventory.
3. Prove Go↔Rust native gRPC UDS/Windows named pipes, mutual authentication, replay protection, cancellation, bounded streams and cleanup.
4. Demonstrate file-format/readback equality, Parquet host-mediated seeking and high-precision UI encoding; report any incompatibility per field and format version.
5. Prove sensitive no-spill, SQL/table-provider restrictions and host-wide resource quotas under adversarial tests.
6. Benchmark **cold and warm, end-to-end and engine-only**, on representative synthetic PM workloads; use robust sample sizes and report distributions and resource costs honestly.
7. Validate packaging/licensing feasibility but DO NOT modify main GoPMgr build, dependency list, project files or release artifacts without a separate production authorization.
8. Deliver an evidence report with raw results, reproducible commands, fixture hashes, test coverage matrix, platform status, the A/B/C decision matrix, deviations, explicit Architecture Change Requests (ACRs), and a **recommended—not automatically authorized—next milestone**.

### Anti-downgrade / adversarial review protocol

Do not weaken, postpone indefinitely, redefine, waive or delete an accepted functional, security, performance, compatibility, or licensing requirement to simplify implementation. If a design fails, preserve the failure, explain the mechanism, compare at least two high-quality alternatives, assess their risks, and submit a numbered ACR for maintainer approval **before** changing normative architecture. Independently critique your own work for untested assumptions, misleading green tests, inadvertent dependency additions, data leakage, unverified numerical claims and unsupported release claims. Review uncommitted files and scope before staging. Keep changes small and reviewable; do not commit generated or binary artifacts without explicit repository policy coverage.

**Finish with:** `Verified facts | measured results | PASS/FAIL/BLOCKED test table | technical and licensing risks | change inventory | decisions still needed | recommendation`. Never say “DataFusion is faster”, “DuckDB fully removed”, “safe spill”, “signed release ready”, or “G1 completed” without the corresponding actual evidence.
