<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012: Replace DuckDB analytics with an Apache DataFusion/Arrow worker

**Status:** ACCEPTED by the maintainer on 2026-10-26 ([acceptance record](../review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)) as the binding target architecture, subject to its feasibility gates and reserved owner decisions. The DF0 feasibility spike is authorized; production integration, mandatory dependency changes, SQLCipher migrations, DuckDB removal and release packaging changes are not. ADR-002 stays implemented until a verified cutover. Verification notes: [repository verification](../review/adr-012-repository-verification.md).  
**Draft:** 0.2 · **Date:** 2026-10-26 · **Supersedes:** proposed draft v0.1 (preserved under `reference/v0.1/`)  
**Decider:** GoPMgr maintainer  
**Repository evidence baseline:** `main` at `28a758d154795bb2a20f3cbeb63a12e9130a88ea`, cross-checked against ADR-010 v2.2 on P00 branch `3d00f53` and P02/S0 PR #142. Reconfirm HEAD before execution.  
**Companions:** [feasibility specification](../testing/adr-012-datafusion-feasibility-spike.md), [agent prompt](../agent/adr-012-coding-agent-instructions.md).  
**Related ADRs:** ADR-001 (encrypted project data), ADR-002 (current implemented DuckDB decision), ADR-010 (accepted plugin platform), ADR-011 (reserved/anticipated normalized storage).  
**Decision category:** Analytics backend and local IPC; *not* a project database replacement.

## 0. Executive decision

> **Decision posture:** Approve the objective of removing DuckDB and evaluating a Rust DataFusion/Arrow worker, **not** a predetermined mandate that every analytical operation move into Rust. The work must compare a Go-native deterministic portfolio rollup plus DataFusion for file/columnar analytics with an all-DataFusion worker and the current DuckDB reference. A verified **release-compliance gate** is independent of functional migration: the unresolved DuckDB TPC-H/TPC-DS packaging issue in PR #146 must not be used as permission to remove features or to ship an unreviewed artifact. This ADR remains **PROPOSED** until the maintainer accepts it.

**Review incorporation:** [v0.2 amendment and traceability record](../review/adr-012-v0.2-improvements-and-traceability.md) enumerates DF-A01 through DF-A14. The 60 original acceptance test IDs remain intact, augmented by DF-061–DF-065 and executable test-case requirements.

**Propose DataFusion and Arrow as the preferred target for GoPMgr's future analytics backend, subject to a genuine proof of functional parity, security, portability, performance, and signed packaging.** Replace the existing DuckDB engine in a dedicated early analytics workstream after the common G1 worker foundation is technically ready. Do not wait for S6/S7A's complete portfolio and risk analytics expansion. Do not remove DuckDB merely because an isolated DataFusion SQL query passes.

The application **must continue to use encrypted SQLCipher-backed SQLite as its sole authoritative project database**, Go-owned schedule/financial semantics, and version-controlled audit logic. DataFusion is a cancellable, nonauthoritative, *ephemeral* compute engine. Apache Iceberg is **not** a dependency for this replacement. Removing DuckDB's CGO bindings **does not make GoPMgr CGO-free**: SQLCipher still uses CGO.

**Three distinct events:** (1) feasibility evidence; (2) production integration with contract/feature parity and rollback; (3) verified DuckDB dependency removal. Each requires a separate go/no-go decision.

### 0.1 What has actually been verified in the repo

| Fact at baseline | Evidence | Architectural consequence |
|---|---|---|
| Production Wails builds select `duckdb,webkit2_41` | `Makefile:19` | Must remove/replace the tag across all build, CI, release, and packaging paths at cutover. |
| `internal/analytics.Engine` exposes `PortfolioRollup`, `ImportTabular`, `Available`, `Close` | `internal/analytics/analytics.go` | These methods form a **minimum** compatibility boundary, not a complete scalable analytics platform. |
| DuckDB is in-process, in-memory, with one connection; SQLCipher passes already-authorized rows | `internal/analytics/duckdb.go`; ADR-002 | Rust process adds transport and startup costs and may be slower for small requests. |
| Portfolio outputs use exact `int64` minor-unit totals plus computed float display values; EVM coverage distinguishes unavailable from zero | `analytics.go`, `duckdb.go`, `duckdb_test.go` | Integer-exact, missingness and index conventions require golden regression. |
| `ImportTabular` accepts CSV, TSV, TXT, Parquet, JSON, NDJSON and returns at most 10,000 rows in `Dataset.Rows` | `duckdb.go` | Preserve present behavior while adding explicit `truncated`/`fully_ingested` statuses in a **new versioned API**. |
| Production DuckDB bindings add `duckdb-go/v2` and native platform modules | `go.mod`, `docs/DEPENDENCIES.md` | Remove all only after build, package, license, and user-operation parity. |
| Current analytics code uses DuckDB under `//go:build duckdb`; `!duckdb` has an unavailable stub | `duckdb.go`, `stub.go`, tests | No default no-op degradation may be shipped by accident during transition. |
| G0 proves real Go↔Java local gRPC IPC, **not Rust↔Go named pipes** | P01/G0 PR #141 | Rust transport on Windows is its own hard feasibility gate. |
| S0 shows whole-chart edits/audit/Gantt are actual large-schedule bottlenecks | P02/S0 PR #142 | Engine replacement alone will not resolve them; do not credit those improvements to DataFusion. |

**Scope/verification limit:** This draft is based on repository inspection and upstream published interfaces, not a locally executed Rust integration or fresh GoPMgr benchmark. All proposed measurements remain pending.

## 1. Goals and non-goals

### 1.1 Binding end-state goals

- Remove the DuckDB analytical runtime, Go bindings, native dependency modules, `duckdb` build tag, obsolete stub semantics, and related release constraints **only when** all currently shipped functionality remains equivalent or improved.
- Support `PortfolioRollup` with exact currency minor-unit arithmetic, existing EVM availability semantics and undefined-index behavior; never combine different reporting currencies without a governing FX/grouping contract. Account/project scope and session/epoch controls remain Go-owned.
- Support existing user-selected CSV/TSV/TXT, Parquet, JSON and NDJSON import paths, with robust, tested decoding and no silent missing-data claims. Verify format-specific edge cases independently; DataFusion's out-of-box JSON/CSV inference is **not assumed equivalent** to DuckDB's.
- Provide scalable typed analytics through a versioned host API and bounded Arrow-compatible columnar batches or an equivalent rigorously measured IPC representation; preserve responsiveness and avoid Wails whole-table payloads.
- Remain fully local-first, offline-capable, reproducible, accessible, signed-package-compatible on every platform **actually advertised** by GoPMgr.
- Keep engine restarts, cancellations, resource exhaustion, malformed input and aborted operations from altering project authority, audit chains or encrypted project files.
- Preserve architectural flexibility for S7A schedule, resource, portfolio, risk and timephased analytics, without introducing premature distributed services.

### 1.2 Non-goals

- Replacing SQLCipher/SQLite, the WBS store, audit ledger, project-file encryption, or the Go scheduling kernel.
- Replacing MPXJ or the ADR-010 interchange architecture.
- Claiming native `.mpp` import/export, which is a separate G2/G3 concern.
- Introducing Ballista, Iceberg, remote object stores, mandatory online access, Valkey, an analytics server, or user-provided arbitrary SQL as prerequisites.
- Making DataFusion's planner the authority for CPM, EVM, currency, timeline, scheduling or project-change semantics.
- Guaranteeing raw query-engine speed improvements over DuckDB. End-to-end benefits must be measured.

## 2. Architectural decision records (pending owner approval)

### DF-01 — System of record and authority

GoPMgr's host holds project keys and database handles. It validates principal, account, project copy, current revision and user authorizations before exposing any analytical projection. The Rust worker never receives the SQLCipher database handle, DEK, escrow key, unrestricted SQLCipher path, or permission to mutate the project's canonical tables. All worker outputs are untrusted computed results until the host validates schema, provenance, overflow, units and identity.

### DF-02 — Execution boundary: separate, bundled, trusted Rust worker

Implement DataFusion in a versioned Rust binary managed by ADR-010's Go supervisor, *not* in-process Rust FFI. The worker declares an analytics capability in the internal language-neutral plugin registry. It is bundled and release-verified, with per-job/possibly session-bounded lifecycle, predictable memory/CPU/scratch quotas and crash isolation. A process boundary does **not** imply sandboxing. Adopt stronger OS restrictions only when proven; no unearned containment claims.

**Trade-off:** Removes DuckDB Go-linkage/native library burden but adds a separately shipped executable, process startup and gRPC serialization. Evaluate performance with both cold and warm worker lifecycles. The common plugin supervisor must remain independent of Rust or DataFusion.

### DF-03 — Transport: native gRPC on private local IPC

Use the accepted ADR-010 proto/gRPC services, bootstrap, mutual authentication, OS peer identity, endpoint protections and offline-only policy. Linux/macOS: UDS. Windows: the accepted local named-pipe transport, with explicit Rust tonic/tokio/Windows named-pipe adapter proof. **Go↔Java transport evidence does not establish Go↔Rust correctness.** No silent loopback TCP, HTTP server, ad hoc raw-protobuf-over-stdio data transport, or unreviewed new bridge. A one-time secret-bearing stdin bootstrap is permitted under INT-02, never for business records.

Version and capability negotiation must fail closed. Use dedicated analytics proto services or a properly typed extension to the common ADR-010 connection, decided in G1/DF0. Every message has operation ID, schema/protocol versions, unit/currency interpretation, bound lengths and cancellation identity. Authenticated host-generated snapshot descriptors must be scoped to the job.

### DF-04 — Typed analytics API, selectable execution ownership, and adapter

Keep the existing Go-facing `analytics.Engine` functional at the initial cutover, using a thin host adapter to the worker; evolve a new typed, explicitly versioned API for paged/streamed datasets without breaking existing Wails/Go callers. `Dataset{Columns, Rows}` remains bounded to legacy 10,000-row payloads. Prefer `DescribeSource`, `BeginDatasetScan`, `ReadNextBatch`/server streaming, `CloseDataset`, `RunPortfolioRollup`, `CancelJob`, `Health` and `GetCapabilities` as *illustrative* contracts (exact service signatures require G1 contract review).

Do not send unrestricted SQL from the UI or plugin consumer. Permit only vetted, host-defined analytical operations and typed filters/projections in phase 1. An SQL/query-authoring feature requires another permission, parsing and resource-limit review.

### DF-05 — Arrow exchange with bounded frames, no implicit shared memory

DataFusion uses Arrow internally. For large inputs/results, prefer Arrow IPC RecordBatch payloads **carried inside authenticated bounded gRPC chunk envelopes**, with explicit schema fingerprint, batch numbering, compression and checksum rules; the Arrow IPC stream is *not* itself a network listener and *must not bypass* gRPC limits. Implement reassembly with preallocation caps, end-of-stream summary, deterministic ordering (where promised), nullable fields and malicious-length protection.

Do not assume Arrow's in-process zero-copy advantage survives Go↔Rust IPC: encoding/copying, process boundary and column conversion must be measured. For small portfolio metrics, typed protobuf repeated rows may be simpler/faster; benchmark both. Schema compatibility is versioned separately from worker binary version.

### DF-06 — Financial and EVM parity is non-negotiable

All money aggregate inputs/outputs in wire and analytical form use signed integral minor units (or a validated wider representation if overflow permits) and an explicit currency/unit policy; never use `f64` as the authoritative monetary total. Accumulate with checked overflow; return a typed overflow error rather than wrapping. Maintain exact counts and existing behavior for unavailable EVM, zero numerator/denominator, missing metrics, consistent reporting dates and mixed-currency rejection **at the host boundary**. The current `normaliseProjectMetricsMoney` float-to-minor-unit fallback is legacy compatibility; capture it as a fixture, document its rounding behavior, and forbid new ambiguous float-only monetary contracts.

Do not silently switch to per-row ratios averaged together. Portfolio SPI = total EV / total PV and CPI = total EV / total AC, with the same undefined-index convention as the existing host API unless separately approved.

### DF-07 — Tabular reader compatibility is tested, not assumed

For `.csv`, `.tsv`, `.txt`, `.parquet`, `.json`, `.ndjson`, verify input classification, header/column names, duplicate field names, nulls, empty/large files, quoting, BOM, encoding, Unicode, delimiter choice, decimal precision, timestamps/time zones, nested JSON, JSON arrays vs line-delimited objects, row groups, compressed Parquet, missing/corrupt files, inference determinism and adversarial inputs. DataFusion's JSON reader normally targets line-delimited records; a standalone JSON array may need an adapter. Either preserve existing supported results, explicitly reject previously accepted unsupported shapes with a user-facing explanation *and owner approval*, or add a safe preprocessing step. Do not silently reduce formats or field fidelity.

The old `ImportTabular` returns a bounded result, not proof a source file was fully ingested. Introduce `truncated` and `fully_ingested` as separate flags with truthful provenance. If total row count is expensive, count is `unknown`, never fabricated.

### DF-08 — Host-created, immutable source references, and revision-bound analytical snapshots

For current portfolio inputs, marshal authorized `ProjectMetrics` provided by Go; for future normalized projects, use a stable, bounded, encrypted-project read snapshot owned by the host. Do not grant the worker access to arbitrary account directories, database files, other project copies or foreign projects. Bind snapshots to account, principal, copy identity, generation/revision, currency/reporting date, authorization, expiry and cancellation. Keep SQLite reads short-lived; a long-running calculation should receive a bounded materialized projection or a resumable, revision-consistent export so it cannot hold a WAL reader open indefinitely. Never cache raw decrypted data on disk without an approved protection/retention design.

### DF-09 — Path and filesystem restrictions

An explicit user-selected file is opened and verified by GoPMgr. The default worker mode receives its contents over bounded authenticated streaming, not the original absolute path. Host performs canonicalization and symlink/reparse-point checks and enforces user selection, file quotas and time-of-check/use safeguards. Host-mediated range reads are a mandatory feasibility candidate under DF-A07; a private immutable snapshot remains a future security-reviewed alternative. Do not let the worker freely open arbitrary filesystem paths. Use bounded scratch space for spills; disable unapproved `SessionContext::enable_url_table`, SQL DDL/DML/statement classes, remote object store integrations and extension auto-install; mount/file system permissions enforce these controls where possible.

### DF-10 — Memory/CPU/spill and cancellation

Enforce both DataFusion query-memory pools and OS process limits; DataFusion's pool limit is not the same as total Rust-process RSS. Set explicit thread count/partition count, task concurrency, spill disk quota, scratch ACL, safe deletion, query deadline and gRPC per-stream backpressure. Spill files may contain decrypted project data: enforce `SENSITIVE_MEMORY_ONLY` for all decrypted project projections per DF-A04; never allow plaintext spill or claim an unproven encrypted lifecycle. Never log raw rows, expressions containing data, file paths, query literals, keys or credentials by default.

### DF-11 — Provider availability and failure model

`Available()` should reflect verified worker/runtime capability, not merely a compile tag or existence of a file. Startup errors, invalid contracts, missing runtime, malformed input, canceled jobs, quota exhaustion and worker crashes map to typed host error codes and ADR-008 localized UI messages. During a staged transition, the currently shipped DuckDB engine remains usable **only where explicitly configured and tested**; no automatic engine fallback that hides a failed DataFusion execution. After final cutover, the shipped app must not silently substitute a no-op stub for the missing replacement.

### DF-12 — Performance must be compared end-to-end

Benchmark old DuckDB **as shipped** and candidate DataFusion with equivalent inputs, logic and memory policies. Separately record startup, host snapshot prep, serialization, IPC transfer, parser/scan, SQL plan/execute, return conversion and UI result. Warm-query speed alone is insufficient. Predict that small rollups may be slower with an out-of-process engine; accept a measured loss only through documented maintainer decision, without changing acceptance tests. The replacement may be justified by dependency, modularity and future capability gains if current UX stays within approved latency budgets.

### DF-13 — Build, distribution and rollback

Every advertised OS/architecture requires reproducible Rust target, pinned Cargo.lock/toolchain, legal notices/SBOM, offline package, signing/notarization checks, Microsoft Defender/Windows security checks where relevant, and update/rollback compatibility. Preserve the existing SQLCipher project file format; worker has no project-file migration authority. Provide a staged, isolated feature flag and rollback to the prior executable until complete release parity. After removing DuckDB, verify clean build/tag matrix, modules, `go.sum`, release scopes, installer payload size, startup/RSS, ASLR/signatures and absence of DuckDB native artifacts.

### DF-14 — Iceberg deferred; S7A remains a separately owned milestone

Do **not** depend on Apache Iceberg, Ballista, remote catalogs, Valkey, LanceDB or a lakehouse for replacement of current DuckDB methods. Iceberg is an independently benchmarked, security-reviewed option for later analytical snapshots and archival datasets. S7A still owns expanded portfolio/risk/timephased analytics and cannot be marked complete merely because DuckDB was replaced. S5 large Gantt performance and S7B audit reductions are independent workstreams.

### DF-15 — Change governance, ADR supersession, and audit trail

Acceptance of this ADR approves a **conditional target** to replace ADR-002's DuckDB engine selection; it does **not** operationally supersede the already implemented ADR-002 until the verified, owner-approved cutover. ADR-002 remains an unchanged historical record. It inherits all binding ADR-010 invariants and the architecture-preservation agent prompt. All test IDs, negative tests, resource/security requirements, release claims and permitted format coverage stay binding unless the maintainer accepts a documented Architecture Change Request with equal-or-better objective evidence. Source code changes are authorized by separate small PR gates, never by accepting this draft alone.

## 2A. Binding v0.2 architectural corrections (DF-A01–DF-A08)

### DF-A01 — Separate DuckDB release compliance from DataFusion cutover

**Independent release blocker:** PR [#146](https://github.com/draconianneocoder-cpu/GoPMgr/pull/146) reports that the currently distributed DuckDB native bindings include TPC-H and TPC-DS benchmark implementations with unresolved license compatibility. The maintainer directed that those implementations not be distributed in new artifacts absent a separately approved legal determination. **A notice file alone is not evidence of license compatibility.** Verify affected linked archives, symbols, and final release executables on each architecture; retain the other third-party notice controls. A preexisting signed release is not automatically compliant merely because it is a useful functional baseline. Keep exact past-artifact inventory and seek qualified legal review where necessary.

- Controlled **internal** DuckDB reference benchmarks may remain only subject to applicable license conditions and organization policy; never assume public redistribution of TPC-based queries/generators/results is permitted.
- A release may proceed only with **(i)** a proven TPC-free DuckDB binary retaining required functionality, **or (ii)** a fully qualified DataFusion/Go replacement, **or (iii)** an expressly approved legal resolution. No unapproved stub/no-analytics fallback.
- Packaging/compliance is **LC-GATE**; technical replacement is **DF-G0 through DF-G6**. They are independently required, cannot be merged into one artificial PASS, and must each have a recorded owner decision.
- Source-generated tests must use independently authored synthetic schedules/portfolios rather than TPC-H/TPC-DS fixture generators unless license conditions are separately verified. Keep release-safe provenance for fixture data.

### DF-A02 — Compare the hybrid allocation as a genuine competitor

A simple portfolio rollup does not inherently require a columnar SQL engine. Compare **A: shipped DuckDB**, **B: all-DataFusion**, and **C: deterministic Go rollup + DataFusion tabular analytics** under the same semantic contracts and security limits. Go rollups may eliminate worker startup and RPC for a tiny operation while retaining DataFusion for Parquet/CSV/JSON and future S7A. Measure end-to-end p50/p95, CPU, peak RSS, exact-money correctness and binary size for all three. Owner must accept the method-to-provider allocation before DF-P03 production design. No architecture claim that “one backend for everything” is intrinsically superior.

### DF-A03 — Correct the ADR-002 chronology

Acceptance of ADR-012 changes the **approved target** only; it does not retroactively erase implemented ADR-002. Until cutover ADR-002 remains **Implemented—pending replacement**; after DF-P06 acceptance, DF-G5/G6 proofs and release switch it becomes **Historically implemented—superseded for analytics-engine selection**. SQLCipher authority remains unchanged. Tag removal, source deletion, build reconfiguration and external release publication are independently verified events; no unverified statement of completion.

### DF-A04 — Two explicit confidentiality profiles; no unprotected spill

1. `SENSITIVE_MEMORY_ONLY` (mandatory default for decrypted project projections): enforce no operator spill **by a proven runtime policy and OS/file monitoring**, bounded query memory and overall RSS; if the operation cannot complete without disk, return typed `RESOURCE_LIMIT` **without creating plaintext spill**. Merely setting a temp-directory quota to zero or relying on DataFusion's pool configuration is insufficient until proven by failure injection. If DataFusion cannot enforce true no-spill for an operator, **block that operator or data class**, not the test.
2. `PROTECTED_SPILL` (future, explicit security/owner approval): choose an encryption architecture and key custody, per-job quotas, restrictive ACLs, crash recovery, deletion/retention and tests under memory pressure; no worker project DEK and no optimistic “encrypted temp” wording without proof. It is **not** authorized by ADR-012 v0.2.
3. Non-sensitive synthetic DF0 benchmark fixtures may use isolated bounded scratch, **never** as precedent for sensitive production data.

Enforce process-level CPU/RSS/child counts and aggregate host-wide admission budgets independently of DataFusion pool accounting. Detect temp files before/after all normal, cancelled, crashed and OOM paths. Disk absence is evidence from controlled instrumentation, not an assumption.

### DF-A05 — Preserve legacy money behavior without perpetuating ambiguity

Existing `ProjectMetrics` uses **zero minor units as both a legitimate amount and a potential legacy fallback signal** when a major-unit float is nonzero. Characterize this ambiguity with independent reference tests. Keep the existing public wire result compatible for the initial cutover where possible; design a **v2** typed input with `present`/`valid` flags, currency code, scale and signed integral values (or checked 128-bit/wider accumulator), while forbidding ambiguous zero-as-absent inference in new contracts. If old DuckDB behavior conflicts with financial correctness, label **LEGACY_PARITY** and **CORRECTNESS_ORACLE** separately and seek an approved migration/UX choice; never change a customer-visible amount silently. Reject mixed currencies before aggregation. EVM absence, defined zero, negative and undefined denominators stay distinct in tests.

### DF-A06 — Prove an exact wire/API and UI numeric contract

Before production DF-P03, publish versioned Protobuf and Arrow schema descriptors, compatibility rules, fixtures, and test implementations for:

- `principal/account`, `project_copy`, `source_identity`, `schedule_generation`, `revision`, `job_id`, `dataset_id`, TTL/expiry, consent scope; durable revocation at sign-out and project switch.
- `DescribeSource`, `Open/ReadAt/CloseSource`, `BeginDataset`, `ReadNext` or server streaming, `CloseDataset`, `PortfolioRollup`, `CancelJob`, `Health` and `Capabilities`, including retry/idempotency, EOF/error precedence, ordering, count exactness and max dataset lifetime.
- Arrow IPC framing in bounded authenticated gRPC chunks, schema hash, monotonic sequence numbers, per-batch count/checksum, final digest/receipt, defensive reassembly and version rejection. A chunk is **not** an independent Arrow record; publish clear batch boundaries and schema rules.
- `int64` values exceeding JavaScript safe integer range, `Decimal128/256`, precision/scale, null/absent, timestamps/time zones/DST, nested records, infinity/NaN, strings/binary and overflow behavior across **Rust → gRPC → Go → Wails → JS**. Encode large exact values as validated decimal **strings** or schema-defined tagged records for UI; never silently coerce them into `number`.
- `Available()` is a low-cost, side-effect-free capability snapshot, with separate `Health`/worker launch and explicit invalidation. Do not spawn a worker from a property getter.

Bound legacy `Dataset.Rows` to its 10,000 rows with **independent** `truncated`, `fully_ingested` and `total_rows_known` semantics in a versioned endpoint. The legacy endpoint may remain behavior-compatible; it may not claim full ingestion when it returned a preview.

### DF-A07 — Authorize efficient seeking without granting raw filesystem authority

User-selected local files remain host-owned. Compare three approaches on identical Parquet/CSV/JSON fixtures: (1) sequential stream; (2) **bounded host-mediated `ReadAt(offset,length)`** requests backed by a pinned open file descriptor and controlled parallelism; (3) explicitly protected immutable snapshot (only after approval). Parquet footer reads, row-group pruning and metadata inspection require efficient range access. Implement an audited, job-scoped `ObjectStore`/reader adapter when using DataFusion/Parquet APIs; do **not** assume a standard DataFusion file provider supports arbitrary remote byte ranges over gRPC without such a bridge. Enforce source byte identity/length/digest and detect mutation during a job; decline or restart if a consistent view cannot be guaranteed. Record IO requests, bytes transferred, ranges, cache effects and excess read amplification. Workers never receive arbitrary local paths.

### DF-A08 — Single restricted analytical façade and enforcement matrix

All user- or plugin-influenced queries must pass one host/worker **allowlisted execution façade**. Restrict allowed operations/table providers, reject DDL/DML/statement classes, disable URL table access, remote object stores, uncontrolled UDFs and arbitrary host-file registration. DataFusion's `SQLOptions` and `sql_with_options` apply only when invoked on the relevant query path; **audit all DataFrame/SQL/table registration entry points**, not just SQL text. Host issues limited capability tokens, immutable datasets and typed filters, not raw unrestricted SQL. Maintain a test-backed enforcement matrix with columns `host`, `DataFusion`, `OS`, `test-only assertion`; distinguishing “no TCP observed” from “network blocked at OS level”. Treat malicious decoded Arrow and parser inputs as untrusted and impose nesting/decompression/CPU limits.

---

## 3. Alternatives and decision matrix

The scores below are design judgments, not measured query-speed scores. Higher is better, 1–5.

| Candidate | Architectural modularity | Current small-query efficiency | Encrypted-authority isolation | Packaging ease | Future extensibility | Decision |
|---|---:|---:|---:|---:|---:|---|
| Keep current in-process DuckDB | 3 | 5 | 4 | 3 | 3 | Reliable rollback/baseline; not preferred target |
| **Rust DataFusion/Arrow worker over ADR-010 gRPC** | **5** | 2–3 (unmeasured) | **5** | 3 | **5** | **Preferred conditional target** |
| Rust DataFusion in-process via C ABI | 2 | 4 | 2 | 2 | 4 | Reject: FFI/CGO, crash and linking risks |
| Go-only analytical implementation | 3 | Unknown | 5 | 5 | 2 | Consider for isolated tiny rollups only, not broad analytics replacement |
| Replace SQLCipher with DataFusion/Iceberg | 1 | N/A | 1 | 1 | 3 | Reject: no OLTP authority/format protection |
| Remote analytical service | 4 | Network-dependent | 1 | 1 | 5 | Reject: violates local-first default and operation/privacy constraints |

**A Go-native deterministic portfolio rollup is a required first-class feasibility candidate, not an afterthought.** Compare: (A) existing DuckDB as shipped; (B) a DataFusion Rust worker for both operations; (C) **Go exact portfolio rollup + DataFusion Rust worker for tabular and advanced columnar analytics**. A Go rollup uses the same already-authorized `ProjectMetrics`, checked exact minor-unit sums, EVM semantics, and single host authority. It does not create a competing financial source of truth. Keep routing policy explicit and versioned; no silent fallback from a failed DataFusion file operation to another engine. The final assignment of methods is a **post-DF0 owner decision**. The DuckDB retirement gate is satisfied only if *all* existing capabilities are replaced and proven, regardless of which authorized component executes them.

## 3A. Workload classification, lifecycle, and benchmarking obligations (DF-A09–DF-A12)

**DF-A09 — Ownership and aggregate admission.** Compare fresh-per-job, bounded session-scoped worker reuse, and Go-only small rollup. Reuse requires account/copy separation, cancellation and memory scrubbing, no cached cross-user data, worker restart on sign-out/switch and explicit expiry. Host tracks *aggregate* CPU, RSS, concurrent jobs, spawn rate, gRPC buffers, open files and scratch; a per-worker memory cap is not sufficient. Use deterministic admission/rejection, not OS thrashing.

**DF-A10 — Performance gate has two dimensions.** (a) absolute user-observed response-time and memory/CPU/installer budgets set by owner from representative hardware and S0; (b) relative to the current shipped DuckDB baseline for exactly equivalent output and privacy policy. Keep engine-only, startup, host source prep, IPC/batch encoding, scan/query, conversion and Wails/UI results separately. Preliminary 1.5× rollup p95 and 1.25× preview p95 are only **proposals** and cannot be treated as pass standards until approved. A few milliseconds difference may be negligible, whereas a 500 ms UI regression matters; explicit owner acceptance is needed. Report p99 only with enough samples to justify tail claims: for cheap operations target ≥1,000 iterations across multiple cold/warm sessions, annotate uncertainty; for expensive stress runs report max and raw distribution without asserting a statistically stable p99 from n=30.

**DF-A11 — Independent correctness oracles and realistic workloads.** Characterize current actual workload separately from 50k-project stress. Mark each fixture `legacy parity`, `plausible production`, `S7A future`, or `adversarial`. A 50k-*task* schedule and 50k-*project* portfolio are distinct. Use checked hand/reference aggregators, CSV/JSON golden schemas, independent Parquet reader or manually validated fixture generation; neither DuckDB nor DataFusion is its own correctness oracle. Require all observed schema/type/inference differences in a signed disposition ledger; cannot declare success by omitting difficult formats.

**DF-A12 — Evidence layers and OS matrix.** DF0 is isolated and evidence-only; DF1/DF2 require accepted G1 production supervisor, separately approved dependency changes and approved contract; signed installed-app tests are release gates. Preserve minimum OS commitments, test the same privacy/resource rules in benchmark and release candidate, and mark each unrun target **BLOCKED**, not PASS. A spike running offline is not proof a signed release is offline/contained. No generalization from Go↔Java pipe evidence to Rust.

---

## 4. Proposed delivery dependencies

```text
ADR-010 G0 evidence ──> DF0 isolated DataFusion/Arrow/Rust IPC spike
                             │
ADR-010 G1 approved + integrated host supervisor ──> DF1 contract/worker adapter
                             │                         │
Existing DuckDB golden suite ─────────────────────────┤
                             ├──> DF2 semantic/file-format/security parity
                             ├──> DF3 performance and release validation
                             └──> DF4 approved cutover, DuckDB removal, rollback proof

ADR-011 S1/S2/S3 typed normalized read projections ──> S7A *expanded* analytics
DataFusion cutover does NOT imply ADR-011 complete; ADR-011 is not a blocker
for replacing the current two in-memory operations.
```

DF0 is evidence-only and can proceed immediately in its own modules, independently of G1 production code. DF1 **must not fork or bypass** ADR-010's supervisor or introduce an unreviewed Windows TCP/stdio fallback. ADR-011 and DataFusion share typed snapshot standards but do not share schema migration authority.

## 4A. Dependency and benchmark licensing (DF-A13)

**Candidate pin at drafting:** upstream DataFusion **55.2.0**, released 2026-10-07 (Apache release catalog and docs.rs); do not claim it is the latest on a later execution date without rechecking. DF0 must lock DataFusion/Arrow/tonic/tokio/prost/Rust versions (`Cargo.lock`), inspect default features, MSRV, advisories, licenses, native library/loading requirements, signing, update size, and reproducibility. A newer release is accepted only by another pinned evidence run; Cargo registry resolution must never silently float comparative samples.

TPC-H/TPC-DS benchmark generators, official queries, and license-encumbered datasets **are not part of the primary fixture corpus** and must not be copied into the repository or redistributed absent independent license review. Use original GoPMgr synthetic workloads; label any supplemental third-party benchmark and obey its attribution/result-publication terms. No speed claim may be generalized from an unrelated TPC-like workload.

---

## 5. Functional and nonfunctional acceptance gates

| Gate | Requirement | Must be true before |
|---|---|---|
| DF-G0 | Reproducible isolated Rust DataFusion engine, real Go↔Rust local gRPC IPC on each claimed OS, bounded streams, source format corpus; failures classified PASS/FAIL/BLOCKED | Production engine ADR authorization |
| DF-G1 | All present DuckDB public methods/behavior understood, golden fixtures frozen, currency/EVM parity and error map stable | Production API adapter merge |
| DF-G2 | Exact accounting/coverage for every accepted input format and declared version, no silent truncation, hostile-files/security tests | Replace file-import implementation |
| DF-G3 | Cancellation, process teardown, mutual auth, ACLs, memory/CPU/scratch/spill bounds, offline and leak tests | Enable candidate in app |
| DF-G4 | Representative cold/warm and large-load UX benchmarks vs shipped DuckDB with immutable reports; owner accepts any regression | Cutover proposal |
| DF-G5 | Signed/notarized offline installers, versioned client protocol, worker-disabled behavior, release test matrix and rollback | Product release |
| DF-G6 | No DuckDB code/import/build tags/native bindings/artifacts across the applicable supported build and package matrix; clean dependency/license inventory | Claim DuckDB removed |

**Do not invent numeric budgets.** The companion spike proposes provisional acceptance policy and instructs the maintainer to choose thresholds after baselines. A benchmark failure must remain a failure until the owner makes a documented design/UX trade-off, not until the agent raises the threshold.

## 6. Risks and mitigations

| ID | Risk | Severity | Mitigation |
|---|---|---|---|
| DF-R1 | Small rollup slower because new Rust process starts per operation | High | Mandatory DuckDB/all-DataFusion/Go-rollup+DataFusion comparison; choose provider allocation only from measured evidence and an owner decision |
| DF-R2 | Windows Rust named-pipe gRPC transport not viable with existing G1 transport | Critical | DF0 Go↔Rust named-pipe interoperability proof; no stealth TCP |
| DF-R3 | Source file format semantics differ between engines | High | Golden cross-engine fixture matrix; no automatic parity assumption |
| DF-R4 | Inexact financial or EVM aggregation | Critical | Integer-exact signed values, overflow checks, mixed-currency and missingness tests |
| DF-R5 | Decrypted rows spill to unprotected disk | Critical | Secure bounded scratch policy; sensitive operations fail closed when policy cannot be met |
| DF-R6 | Hidden dependency on DuckDB tags/bindings remains | High | AST/search/build/installer proof under every target, `go mod tidy` review |
| DF-R7 | Double work from using a parallel plugin/supervisor stack | High | Reuse G1 gRPC, registry, code generation and process policies |
| DF-R8 | Performance measured only in-engine rather than from user action | High | Capture total latency and cold/warm worker startup separately |
| DF-R9 | DataFusion memory pool excludes RSS/allocator/runtime overhead | High | Measure process RSS and OS quotas in addition to DataFusion pool |
| DF-R10 | Snapshot/cache exposes another account's project | Critical | Host provenance, scoped session/copy handles, cross-user negative tests |
| DF-R11 | Build becomes too large or unsigned Rust executable blocks installer | High | OS-specific signed package tests, SBOM, update/rollback rehearsal |
| DF-R12 | Misconstrues 50,000 tasks as an engine-performance achievement | High | Separate S0 Gantt/chart/audit baselines; do not attribute unrelated fixes to DataFusion |

## 6A. Additional review and assurance requirements (DF-A14)

The original 60 `DF-001`–`DF-060` tests are preserved unchanged in intent and identifiers. Five binding additions `DF-061`–`DF-065` cover exact-zero money, protected spill failure injection, host-mediated Parquet seeking, aggregate resource admission and Arrow-to-Wails exact numerics. Every P0 case must record the independent oracle, exact fixture hash, test setup, input, expected result, resource budget, supported-platform evidence and explicit PASS/FAIL/BLOCKED/NOT-RUN state. Absence of instrumentation is BLOCKED, never PASS. The companion [DF0 test strategy](../testing/adr-012-datafusion-feasibility-spike.md) owns execution specifics and test artifacts.

---

## 7. Maintainer decisions required

- [ ] **DF-OWN-01:** Accept conditional DuckDB removal target **and** require impartial three-way evaluation (DuckDB, all-DataFusion, Go-rollup + DataFusion). Production responsibility allocation remains unapproved until DF0 report.
- [ ] **DF-OWN-02:** Authorize isolated DF0 spike only; no production engine/code/dependency changes.
- [ ] **DF-OWN-03:** Accept the nonauthoritative Rust worker and native gRPC local-IPC boundary; Windows named-pipe proof mandatory.
- [ ] **DF-OWN-04:** Accept compatibility requirements, including CSV/TSV/TXT, Parquet, JSON and NDJSON; decide any *identified* ambiguous JSON/type semantics by evidence.
- [ ] **DF-OWN-05:** Set final performance/UX budgets after full DuckDB/DataFusion comparative baseline; regressions require explicit approval.
- [ ] **DF-OWN-06:** Approve default **enforced `SENSITIVE_MEMORY_ONLY`** profile for all decrypted project data. Future protected spilling requires a separate owner/security approval; unknown spill safety blocks production.
- [ ] **DF-OWN-07:** Confirm ADR-002 remains historically implemented until owner-approved cutover; architecture acceptance is not implementation. SQLCipher stays authoritative. Release-compliance restriction for TPC-H/TPC-DS is **independent** of this cutover.
- [ ] **DF-OWN-08:** Confirm S7A analytical expansion and ADR-011 storage remain separate gated programs; no Iceberg in baseline.

- [ ] **DF-OWN-09:** Approve the independent **LC-GATE** for PR #146/TPC-H/TPC-DS linked release artifacts; do not approve shipping unresolved code or replacing it with a no-analytics stub.
- [ ] **DF-OWN-10:** Approve the **Go-native portfolio-rollup candidate** as mandatory DF0 comparison, with provider allocation decided from evidence.
- [ ] **DF-OWN-11:** Approve host-mediated bounded range reads as a required Parquet source-access experiment; no blanket worker path access.
- [ ] **DF-OWN-12:** Approve source-data `SENSITIVE_MEMORY_ONLY` as default and DF-061–DF-065 without weakening the original DF-001–DF-060 cases.

## 8. Implementation deliverables / proposed PR sequence

| PR | Work product | Required tests | Excluded |
|---|---|---|---|
| DF-P00 | ADR-012 + spike + agent prompt + owner acceptance | Docs/crosslinks | Production code |
| DF-P01 | Isolated Rust DataFusion engine **plus independent Go exact-rollup comparator**, parity fixtures, 55.2.0 candidate lock, profiling harness, crate/license audit | DF-G0 evidence, DF-A02/A05/A13 | GoPMgr module changes |
| DF-P02 | Go↔Rust gRPC local IPC, Windows named pipes, UDS, **host-mediated ReadAt Parquet**, offline/cancel/fault tests | DF-G0/DF-G3 subset, DF-A07/A08 | Production listener |
| DF-P03 | Approved method allocation, versioned Protobuf/Arrow/schema and Wails numeric contract, Go adapter, test-only worker | Golden contract, G1 registry, DF-A06 and DF-065 | DuckDB deletion |
| DF-P04 | Functional reader/accounting/financial parity, fuzz/regression | DF-G1/DF-G2 | New UI feature scope |
| DF-P05 | No-spill security and aggregate budgets, signed offline installer, independent LC-GATE compliance | DF-G3/DF-G4/DF-G5, DF-062/064 | Unreviewed benchmark relaxations |
| DF-P06 | Separate owner cutover review, feature selection, release rollback | Evidence bundle | Universal cutover without approval |
| DF-P07 | Remove DuckDB deps/build tag/obsolete implementation and update docs | DF-G6 + full release matrix | SQLCipher CGO removal or schema migration |
| DF-P08+ | S7A expanded analytics on approved normalized read projections | S7A acceptance | Treating replacement as S7A completion |

### Rollback

Until DF-P07, preserve the historical DuckDB functional reference behind an explicitly selected **controlled test** configuration. **An affected DuckDB artifact may not be distributed** without independently passing LC-GATE; a historical baseline is not authorization to ship it. Never auto-run both against user data as an unconsented shadow mode. Read-only differential tests use synthetic or authorized fixtures, isolated processes and no persistent project writes. If DF-P07 fails release quality, revert the code/dependency selection and ship the previously validated binary; do not modify or convert project files. The project format must remain readable by the previous release. After cutover, rollbacks still use previous signed releases within the published support/compatibility window and never assume a downgraded binary can open future project formats.

## 9. Reference sources (verify pinned versions when executing)

### Project evidence

- [`internal/analytics/analytics.go`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/28a758d/internal/analytics/analytics.go), `duckdb.go`, `stub.go`, `duckdb_test.go`, `stub_test.go`
- [`Makefile`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/28a758d/Makefile), [`go.mod`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/28a758d/go.mod), [`docs/DEPENDENCIES.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/28a758d/docs/DEPENDENCIES.md)
- [`ADR-002`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/28a758d/docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md), ADR-010 v2.2 (feature branch), [P02/S0 PR #142](https://github.com/draconianneocoder-cpu/GoPMgr/pull/142)

### Upstream technology / independently benchmarked evidence

- [Apache DataFusion 55 release](https://datafusion.apache.org/blog/output/2026/08/25/datafusion-55.0.0/) (55.0.0 released August 2026; do not infer unreleased 56 is usable)
- [Securing DataFusion](https://datafusion.apache.org/library-user-guide/securing-datafusion.html) (SQL, tables, memory, spill policy)
- [DataFusion configuration](https://datafusion.apache.org/user-guide/configs.html)
- [DataFusion benchmarks repository](https://github.com/apache/datafusion-benchmarks) (queries derived from TPC-H/TPC-DS are not official TPC results)
- [Arrow Rust](https://github.com/apache/arrow-rs); [DataFusion Rust](https://github.com/apache/datafusion)
- [tonic](https://github.com/hyperium/tonic); [tokio](https://github.com/tokio-rs/tokio); [Windows named-pipe API](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes)
- [Iceberg Rust](https://github.com/apache/iceberg-rust), deferred

**Important:** Performance claims from third-party benchmarks are context-specific; no upstream published benchmark may substitute for measuring GoPMgr's actual end-to-end analytical operations and worker IPC.

## 10. v0.2 execution-quality gates and evidence precedence

1. `LC-GATE` (distribution compliance) and `DF-G0`–`DF-G6` (technical cutover) are separate PASS/FAIL/BLOCKED decisions. License uncertainty cannot be “fixed” by copying notices or silently removing the file-import feature.
2. Owner accepts only a **target architecture and experimental authority** when approving DF0. No production deployment or main `go.mod` mutation follows automatically.
3. The **source of truth** is accepted ADR-010 and repo `AGENTS.md`, then this approved ADR (if/when accepted), then separately approved transport/data/storage ADRs, then DF0 execution evidence. Draft snippets never override live, tested implementation constraints.
4. All test and platform claims reference immutable run IDs, exact source revision, fixture checksums, sample count and evidence file. Do not infer a product result from a Rust crate benchmark or an unsigned spike.
5. An owner-approved functional change requires a versioned interface and a visible migration/disposition plan; incompatibilities are never silently accepted as “engine differences”.
