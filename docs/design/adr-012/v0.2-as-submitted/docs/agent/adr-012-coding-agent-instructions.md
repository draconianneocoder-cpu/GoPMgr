<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Coding-agent directive: ADR-012 v0.2 DataFusion feasibility without architectural compromise

**Status:** Proposed agent instruction; becomes binding only when accepted by the GoPMgr maintainer.  
**Parent:** [ADR-012](../design/ADR-012-datafusion-analytics-replacement.md)  
**Test plan:** [DF0 feasibility specification](../testing/adr-012-datafusion-feasibility-spike.md)  
**Inherited:** ADR-010 v2.2 architecture-preservation agent prompt, repository `AGENTS.md`, `docs/DEPENDENCIES.md`, `docs/TESTING.md`, release gates and current platform support matrix.

## Governing interpretation

This document **does not grant permission to implement**; the owner must accept ADR-012 and separately authorize DF0. The authoritative v0.2 plan supersedes the *draft* v0.1 text; preserved reference copies are historical. Every earlier DF-001–DF-060 test remains valid, and DF-061–DF-065 are added. Treat LC-GATE (distribution licensing) as independent of DF-G0–DF-G6 (technical/functional migration).

## Mission

Determine whether a standalone Rust Apache DataFusion/Arrow worker can **fully replace the functionality currently supplied by DuckDB** within GoPMgr's local-first architecture. Report measured benefits, regressions and limits. Do not assume DataFusion will be faster or better on every dimension. Your goal is high-quality, reproducible evidence for an owner decision, not a predetermined migration.

## Authority limits

You may work on an isolated DF0 prototype **only after the owner explicitly authorizes DF-OWN-02**. Until separately approved, you must not:

- Modify shipped application behavior, project schema or SQLCipher encryption, use production project data, or write the Rust worker into the release packages.
- Add dependencies to the main `go.mod`, alter its build tags or release pipelines, or remove DuckDB code/bindings.
- Replace native gRPC/UDS/named-pipe IPC with TCP, HTTP, custom stdio framing, FFI, or an independent plugin supervisor.
- Change original acceptance criteria, delete negative tests, silently defer work, or reclassify a FAIL/BLOCKED test as PASS because the toolchain is difficult.
- Claim a speedup by excluding Rust spawn, transport, serialization, snapshot creation, file decoding, memory spilling or final UI return costs. Benchmark DuckDB, all-DataFusion, **and Go exact-rollup + DataFusion** as equal candidates; do not assume the preferred candidate wins.
- Change canonical project money, EVM, schedule or audit semantics to match DataFusion defaults.
- Claim a worker is sandboxed merely because it is out-of-process; claim encryption of spill files without verified design and tests.
- Introduce Apache Iceberg, Ballista, Valkey, new caching infrastructure or a mandatory network service into the replacement MVP.
- Modify accepted ADR-002, ADR-010, ADR-011 or their historical archived versions. New architecture decisions are additive/superseding, not retroactive edits.

## Required execution discipline

1. **Check live HEAD, current PR stack and repo instructions**, including `AGENTS.md`. State exactly which branch/commit was inspected. If the assumptions in ADR-012 have drifted, document differences before work.
2. **Freeze the baseline** by executing the existing DuckDB and stub tests and discovering every call-site/build-tag/release dependency. Do not invent a list of callers or assume a path is unused.
3. **Choose pinned reproducible toolchains** and produce a full license/SBOM/security assessment. Do not use an unreleased DataFusion version as an unacknowledged baseline. Latest verified stable may change.
4. **Prove Go↔Rust native gRPC** on each supported transport. Windows named pipes must be demonstrated *with Rust tonic or the approved Rust gRPC transport*, not inferred from Java G0 tests. Treat production G1 security and the host's existing peer/auth rules as binding.
5. **Establish independent numeric oracles** for integer money totals, currency boundaries and portfolio index semantics. No float-only authoritative totals and no hidden rounding differences.
6. **Probe every format** currently supported by `ImportTabular` and preserve exact visible behavior where contractually required; keep one explicit disposition for each difference and do not claim full import when only 10,000 rows are displayed.
7. **Instrument end-to-end latency**, CPU, RSS, memory pool, on-disk scratch/spill and worker launch costs. Warm- and cold-query results must be shown separately. Use robust sampling and actual owner-approved machines when available.
8. **Attack the trust boundary:** forged credentials, wrong peer, second named-pipe instance, unauthorized path, SQL/URL-table escape, malicious Arrow IPC frame, cancellation, crash, quota exhaustion and redaction detection.
9. **Preserve blocked evidence.** For every unavailable Windows desktop, macOS minimum-version, notarization, Linux/Windows architecture or signing test, report BLOCKED and the specific missing resource. A cross-compile is not an actual target run.
10. **Adversarially review** the evidence and design before reporting success: how might DataFusion be slower? Can a file reader read outside the selected path? Can spill contain plaintext? Can an operation leave a worker? Are ratios rounded differently? What does the UI see on partial read?
11. **Do not proceed to production** merely because a spike completes. Submit a go/no-go report, owner decisions needed, explicit risks, compatibility evidence and a narrowly sequenced implementation proposal.

## Binding v0.2 safeguards

1. **Release licensing:** PR #146 reports TPC-H/TPC-DS compiled into DuckDB static libraries. Do not declare the current DuckDB artifact redistributable because a notice exists; do not silently ship it or remove analytics to evade compliance. Produce separate LC-GATE evidence and qualified-review requests.
2. **Go rollup candidate:** A clean, checked Go minor-unit aggregator is an equal candidate for existing `PortfolioRollup`. Do not force small math through Rust to satisfy a preconceived “all DataFusion” architecture. Select methods only after owner review.
3. **Money:** Preserve the actual legacy float-to-minor fallback in parity evidence, but separately establish correct explicit-zero and presence semantics; independent oracle is essential.
4. **Confidentiality:** The `SENSITIVE_MEMORY_ONLY` profile is the default. DataFusion’s configured memory pool is *not* sufficient proof that no plaintext spills to disk. Force a spill scenario and monitor filesystem/OS behavior. If not proved, fail/block.
5. **Parquet source ownership:** Benchmark host-mediated bounded `ReadAt` vs pure stream; don't pass worker unrestricted file paths or manufacture a faster benchmark with different permissions.
6. **Frontend correctness:** Assert exact `int64`/Decimal128/Decimal256 across Arrow/protobuf/Go/Wails/JS. No JavaScript `number` conversion where precision would be lost.
7. **Resource governance:** Enforce combined host-wide CPU/RSS/jobs, and measure cold/warm startup and G1 gRPC overhead. Do not fabricate p99 from n=30.
8. **Non-substitutable tests:** Preserve DF-001–DF-060 and add DF-061–DF-065; report FAIL, BLOCKED, NOT-RUN distinctly. A spike PASS is not a signed-app release PASS.
9. **Scope separation:** Iceberg, Valkey, Ballista, a new database, and autonomous AI stay deferred. Existing SQLCipher authority and ADR-010 local IPC are not negotiable.
10. **Status truth:** Acceptance of the target ADR does not supersede ADR-002's *implemented historical* status. Only a validated cutover and owner decision may report replacement complete.

## Architecture Change Request (ACR) requirement

If a requested requirement conflicts with actual technical capabilities, submit an ACR that contains:

1. The exact requirement, ID and affected decision.
2. Reproduction and source/fixture evidence showing the conflict.
3. At least two viable alternatives, including retaining DuckDB/deferring cutover.
4. Comparative security, compatibility, functionality, performance, usability, maintenance, licensing and release impact.
5. Test cases that will still prove all required guarantees, or a transparent proposed relaxation.
6. Recommended option, independent adversarial critique and explicit maintainer approval field.

**No ACR approval means no architectural downgrade.** The correct response to an unsolved problem is a recorded FAIL/BLOCKED result and a sound alternative plan—not silently lessening the target.

## Required final outputs

- Updated DF0 evidence, exact revision and commands.
- Raw test matrix DF-001 through DF-060, PASS/FAIL/BLOCKED/NOT-RUN.
- Source and golden fixture manifest with hashes.
- DataFusion/DuckDB cold/warm performance comparison with uncertainty and baseline provenance.
- Rust dependency license/security/SBOM and packaged size estimates.
- Go↔Rust UDS/named-pipe proof and security negative tests.
- Explicit owner decisions and strictly scoped next PR instructions.
- Confirmation that no production DuckDB behavior, SQLCipher file, build tag, installer or database migration changed in the feasibility PR.

**Proposed immediate step:** request owner acceptance of ADR-012 direction and DF-OWN-02 DF0-only authorization. Once authorized, run baseline freeze (DF0-A) and independent Rust/IPC feasibility (DF0-B/C), not a mass migration.
