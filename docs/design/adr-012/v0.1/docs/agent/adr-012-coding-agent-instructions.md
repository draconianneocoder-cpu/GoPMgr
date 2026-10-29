<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Coding-agent directive: ADR-012 DataFusion feasibility without architectural compromise

**Status:** Proposed agent instruction; becomes binding only when accepted by the GoPMgr maintainer.  
**Parent:** [ADR-012](../design/ADR-012-datafusion-analytics-replacement.md)  
**Test plan:** [DF0 feasibility specification](../testing/adr-012-datafusion-feasibility-spike.md)  
**Inherited:** ADR-010 v2.2 architecture-preservation agent prompt, repository `AGENTS.md`, `docs/DEPENDENCIES.md`, `docs/TESTING.md`, release gates and current platform support matrix.

## Mission

Determine whether a standalone Rust Apache DataFusion/Arrow worker can **fully replace the functionality currently supplied by DuckDB** within GoPMgr's local-first architecture. Report measured benefits, regressions and limits. Do not assume DataFusion will be faster or better on every dimension. Your goal is high-quality, reproducible evidence for an owner decision, not a predetermined migration.

## Authority limits

You may work on an isolated DF0 prototype **only after the owner explicitly authorizes DF-OWN-02**. Until separately approved, you must not:

- Modify shipped application behavior, project schema or SQLCipher encryption, use production project data, or write the Rust worker into the release packages.
- Add dependencies to the main `go.mod`, alter its build tags or release pipelines, or remove DuckDB code/bindings.
- Replace native gRPC/UDS/named-pipe IPC with TCP, HTTP, custom stdio framing, FFI, or an independent plugin supervisor.
- Change original acceptance criteria, delete negative tests, silently defer work, or reclassify a FAIL/BLOCKED test as PASS because the toolchain is difficult.
- Claim a speedup by excluding Rust spawn, transport, serialization, snapshot creation, file decoding, memory spilling or final UI return costs.
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
