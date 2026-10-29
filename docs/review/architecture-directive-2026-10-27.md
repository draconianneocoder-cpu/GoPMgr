<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Maintainer architecture directive, 2026-10-27: addendum and baseline

**Status:** APPROVED by the maintainer, James L. Burns, on 2026-10-27, as architectural direction and staged implementation, subject to its own verification, compatibility and release gates. The directive's text is recorded verbatim in §4.

**Repository state when recorded:** `main` at `b6570e1`.

**Relation to earlier records:** this refines, and does not replace:
- the [PS-04 owner decisions](PS-04-owner-decisions-2026-10-27.md) from earlier the same day;
- ADR-010 v2.2 and ADR-012 v0.2 with their accepted owner decisions, on [PR #140](https://github.com/draconianneocoder-cpu/GoPMgr/pull/140) and [PR #150](https://github.com/draconianneocoder-cpu/GoPMgr/pull/150), both unmerged.

Historical ADR text stays as written. §1 lists the amendments this directive makes.

**Later G1 authorization (2026-10-28):** see [§5](#5-g1-authorization-clarification-2026-10-28).
The recorded baseline and verbatim directive remain unchanged.

## 1. Amendments to existing records

| Record | Amendment | Effect |
| --- | --- | --- |
| [ADR-011](../design/ADR-011-normalized-schedule-store.md) O-2 | **Decided:** cryptographically generated 128-bit IDs for new authoritative records (UUIDv4 preferred), database-enforced uniqueness, no fallback to short IDs. Existing IDs and references are kept | Replaces the draft's "16-byte `newID` body" proposal |
| ADR-011 O-13 | **Decided, layered:** a distinct next-generation format and extension; an isolated storage location older builds do not traverse; the internal format and minimum-writer gate; explicit migration; verified encrypted recovery copies; positive evidence that older writers cannot modify migrated projects. Illustrative names: `projects/.../project.gopmgr` (legacy), `projects_v2/.../project.gopmgr2` (new); final names to be tested and documented | Replaces the draft's three open options |
| ADR-011 §5.2 | **Changed:** read-only operations must stop migrating, including on legacy files (directive item 9, §4). The draft had kept today's open-and-migrate behavior for legacy files on read paths | Slice B makes read paths non-mutating, with readers that tolerate missing additive columns |
| ADR-011 status | Implementation is authorized in small, gated stages. No migration is activated before its acceptance and recovery gates pass | ADR-011 itself stays Proposed until reviewed |
| ADR-012 v0.2 | **Target allocation approved:** native Go for deterministic rollups and checked financial calculations; a supervised Rust DataFusion/Arrow worker for advanced, columnar and file-based workloads. DF0 evidence still decides each workload's owner. Production integration still requires DF0-C to DF0-G | Narrows v0.2's three candidates to the hybrid; does not mark DF0 passed |
| ADR-012 / ADR-002 | DuckDB's retirement is approved, through a separate cutover (Slice J) after the replacement qualifies. ADR-002 is not superseded until that cutover succeeds | No DuckDB removal now |
| ADR-010 | The Rust worker runs under ADR-010's supervisor and registry, over authenticated gRPC: Unix sockets on Linux and macOS, named pipes on Windows. Go↔Java evidence does not count for Go↔Rust | Windows named pipes are required, not optional |
| PS-04 sequence | Reaffirmed: RES-A-1, then ADR-011 storage, compact audit, revision-bound queries and virtualized views | Unchanged |
| PR #147 | Separate the Wails upgrade from the DuckDB upgrade, and recheck CI at the current commit | See §2.6 |

## 2. Implementation baseline (at `b6570e1`)

### 2.1 Implemented on `main`

- Go scheduling kernel, SQLCipher project files with the patched `go-sqlcipher` 4.4.2 driver (SQLite 3.33.0), and the Wails/Svelte desktop.
- **RES-A-1** (PR #158): overallocation participants are indexed once per call, and the legacy-contract tests pass unchanged.
- DuckDB analytics (`-tags duckdb`): `PortfolioRollup` and `ImportTabular` over CSV, TSV, TXT, Parquet, JSON and NDJSON.
- Design documents: the ADR-011 draft (PR #159, Proposed), the OWN-10 plan (PR #160), and the PS-04 owner decisions (PR #157).

### 2.2 Accepted decisions that are not on `main`

- **ADR-010 v2.2** and its decision record are only on PR #140.
- **ADR-012 v0.2** and its decision record are only on PR #150.
- The professional-scheduling strategic decision is only on PR #152.

Merging them is a separate, authorized step for each PR.

### 2.3 Authorized and not yet done

| Slice | State |
| --- | --- |
| A, evidence and reconciliation | This record |
| B, non-mutating read paths | Not started |
| C, 128-bit IDs | Not started |
| D, RES-A-1 | **Done** (PR #158) |
| E, format gate | Designed (ADR-011 §5), not implemented |
| F, new-format migration | Designed (ADR-011 §7; O-13 now decided), not implemented |
| G, DF0 completion | DF0-A, B and C done on Linux amd64 only. DF0-C on macOS can run on the maintainer's Mac. The Windows named-pipe endpoint is not written. DF0-C2 to DF0-G not run |
| H to J, hybrid analytics, fidelity, DuckDB removal | Not started; gated on G |
| K, scalable scheduling workspace | Gated on E, F and the audit bridge (DEC-04, not approved) |

### 2.4 Experimental only (on unmerged branches)

These must not be treated as production:
- the S0 benchmarks (PR #142);
- the G0 Go↔Java gRPC spike (PR #141);
- the G1-1 dependencies (PR #143);
- the DF0 spikes, including the Rust worker (PRs #151, #153 and #154).

### 2.5 Open PRs and their dependencies

**Historical baseline:** the OWN-11 permission assertion below is superseded
by the [2026-10-28 clarification](ADR-010-G1-authorization-clarification-2026-10-28.md).
Conditional engineering permission is not qualification or merge approval.

| Stack | Order, base first | Notes |
| --- | --- | --- |
| ADR-010 | #140 → #142 → #152; #140 → #141 → #143 | #143 adds gRPC and protobuf as production dependencies, which OWN-11 has not yet approved for G1 |
| Release notices | #144 → #145 → #146 → #149 | #146 holds the TPC-exclusion gate, proven only for DuckDB 1.5.6 |
| ADR-012 | #150 → #151 → #153 → #154 | Evidence in `docs/testing/adr-012-df0-evidence.md` on #154 |
| Dependabot | #147 (Go), #148 (frontend) | #147 bundles Wails 2.17 with DuckDB 2.0 (§2.6) |

Each child PR must be retargeted before its base merges with branch deletion.

### 2.6 Confirmed defects

| ID | Defect | Evidence |
| --- | --- | --- |
| BL-1 | **Read paths migrate project files.** `ProjectsOverview`, `RunPortfolioAnalytics` and `projectDeletionRequest` open each project with `InitEncryptedDB`, which runs `Migrate()`. The administrator's read-only copy is migrated before it is reopened read-only | ADR-011 §5.3; `internal/db/sqlite.go`, `internal/db/encryption.go` |
| BL-2 | **Short IDs.** `newID` (`internal/db/ids.go`, 21 call sites) and a copy in `internal/agile` draw 32 random bits; a 50,000-record set has about a 25% chance of a collision | ADR-011 O-2 |
| BL-3 | **DuckDB 2.0 links TPC-H/TPC-DS generator code.** PR #147's darwin-arm64 build has 207 TPC symbols (99 code, e.g. `tpch::DBGenWrapper::LoadTPCHData`); `main`'s has 0. #147's CI passes only because the TPC gate is in unmerged #146 | `nm` on both `make build` outputs, 2026-10-27; Linux and Windows not checked |
| LD-1 to LD-9 | Shipped DuckDB analytics defects: wrong Parquet decimals; big integers rounded; `1e400` read as infinity; phantom column; malformed files accepted; **glob characters expand to unselected files**; decimals shown as JSON objects; unsafe JavaScript numbers; the preview reports 10,000 rows with no truncation flag | DF0-A evidence on PR #154 |
| G-1 to G-8 | Gaps in the DataFusion candidate, including the same glob expansion (G-8) and a silently accepted unterminated quote (G-4) | DF0-B evidence on PR #154 |

### 2.7 Acceptance blockers

- **OWN-10:** hardware profiles and budgets.
- **DEC-07:** representative fixtures.
- **DEC-04:** the audit bridge, before any normalized write path.
- **Native platform proof:** macOS and Windows evidence for DF0 and for migration publication.
- **A TPC-exclusion proof for DuckDB 2.0,** if it is ever shipped.

### 2.8 Conflicts among documents, and how they resolve

| Conflict | Resolution |
| --- | --- |
| The ADR-011 draft kept read-path migration for legacy files; this directive removes it | This directive governs (§1) |
| The ADR-011 draft proposed a 16-byte `newID`; this directive prefers UUIDv4 | This directive governs; Slice C chooses the encoding with evidence, in UUIDv4 form or an equivalent |
| ADR-012 v0.2 left the allocation to evidence among three candidates; this directive approves the hybrid | The hybrid is the target, and DF0 still decides each workload's owner |
| ADR-002 keeps DuckDB | Remains in force until the Slice J cutover |
| The PS-04 handoff said ADR-011 schema work was not authorized | Now authorized in gated stages; activation is still gated |

## 3. Order of work

Following directive §18, the next slices are:
1. **#147 split:** Wails 2.17 on its own; DuckDB 2.0 held until it has a TPC-exclusion proof.
2. **Slice B:** non-mutating inspection and opening.
3. **Slice C:** the ID contract.
4. **Slice G:** DF0-C on macOS (runnable on the maintainer's Mac), then the Windows named-pipe endpoint.
5. **Slice E:** the format gate.

## 4. Directive text (verbatim)

```text
GoPMgr — Maintainer-Approved Architecture and Controlled Implementation Directive
Project: GoPMgr
Decision authority: Project owner and maintainer
Decision status: APPROVED architectural direction and staged implementation, subject to the mandatory verification, compatibility, and release gates below.
Repository: `draconianneocoder-cpu/GoPMgr`
1. Authoritative Maintainer Decision
I formally approve the following long-term architecture for GoPMgr:

1. Retain Go as the authoritative application, scheduling, resource-calculation, and financial-policy environment.
2. Retain SQLCipher-backed SQLite as the sole authoritative encrypted project database.
3. Replace DuckDB with a hybrid analytical architecture:
   * Native Go for deterministic portfolio rollups, checked financial calculations, and lightweight analytical operations.
   * A separately supervised Rust Apache DataFusion / Apache Arrow worker for advanced, columnar, large-scale, and file-based analytical workloads.
4. Integrate the Rust worker through the approved, language-neutral ADR-010 native gRPC local-IPC architecture.
5. Preserve Wails/Svelte as the native desktop application interface.
6. Implement ADR-011 normalized, revision-controlled transactional schedule storage without introducing another authoritative database.
7. Adopt layered protection against older GoPMgr versions modifying migrated projects.
8. Adopt cryptographically generated 128-bit identifiers for new authoritative records.
9. Eliminate implicit schema migration from read-only operations.
10. Continue the independently approved large-schedule optimization strategy, beginning with RES-A-1.

These decisions approve the architecture and authorize the engineering work needed to implement it in small, independently reviewable stages.
They do not waive testing, safety, licensing, compatibility, recovery, or release requirements. They do not constitute permission to publish an unqualified migration, merge failing PRs, remove shipped capabilities, or claim that feasibility gates have passed.
Where this decision changes an existing ADR's sequencing or previously reserved owner choice, record an explicit, traceable amendment before implementing the affected production behavior. Preserve historical ADR content.
2. Mandatory Initial Repository Inspection
Before changing production code:

* Inspect the current `main` commit, active branches, open PRs, and their dependency relationships.
* Read ADR-010 v2.2 and its accepted owner decisions.
* Read ADR-012 v0.2 and its accepted owner decisions.
* Read the full DF0 acceptance matrix, evidence, and unresolved findings.
* Read the ADR-011 design work and its unresolved decisions.
* Read the six PS-04 architectural, implementation, and acceptance documents.
* Inspect the current SQLCipher connection, migration, audit, ID generation, portfolio, deletion, backup, restore, and analytics code.
* Inspect Wails bindings, frontend consumers, build tags, Go/Cargo dependencies, CI, packaging, and release scripts.
* Review all affected existing tests and preservation requirements.

At the time of this directive, the relevant PRs include #140–#154. Their state, checks, and contents must be reverified; do not infer that an open draft is merged or production-ready.
Produce a concise implementation-baseline report identifying:

* Implemented behavior.
* Accepted architectural decisions.
* Authorized work.
* Unimplemented or experimental functionality.
* Active PR dependencies.
* Confirmed defects.
* Remaining acceptance blockers.
* Potential conflicts among architectural documents.

Do not build on assumptions that can be resolved by inspecting the repository.
3. Non-Negotiable Architecture
3.1 SQLCipher remains authoritative
All authoritative project data remains within the encrypted, transactionally managed project-storage architecture.
This includes scheduling records, assignments, calendars, baselines, financial references, revision metadata, audit records, and governed project state.
The Rust worker must never possess:

* The project's database encryption key or recovery credentials.
* An unrestricted authoritative database handle.
* Permission to modify authoritative project tables.
* Authority to approve financial or project transactions.
* Authority to append directly to the canonical audit chain.
* Arbitrary filesystem or network access.

3.2 Go remains the calculation authority
The existing Go scheduling kernel remains authoritative for:

* Critical-path calculations.
* Scheduling dates, float, and constraints.
* Resource demand and capacity semantics.
* Resource leveling.
* Calendar interpretation.
* Scheduling baselines and variances.
* Governed EVM and financial calculation policies.

Do not reimplement these calculations independently in Rust, DataFusion SQL, or frontend components.
DataFusion may analyze Go-authorized results and immutable analytical projections.
3.3 Analytics must not block ordinary project work
The Rust worker must not be required to:

* Open a project.
* Save a task.
* Modify a dependency.
* Perform normal scheduling calculations.
* Validate an ordinary task revision.
* Append an authoritative audit event.
* Display an ordinary Gantt viewport.

Failure or absence of the analytical worker must not corrupt authoritative data or disable unrelated core scheduling workflows.
4. Correct the Read-Only Database Boundary
Investigate and eliminate all instances where operations intended to inspect or read project data implicitly change database schemas.
In particular, inspect:

* `InitEncryptedDB()`.
* `initDBWithDSN()`.
* `Migrate()`.
* `ProjectsOverview()`.
* Project deletion preparation.
* Backup and restoration inspection.
* Administrative read-only copies.
* Portfolio and reporting operations.
* CLI, headless, and repair paths.

Distinguish filename-only discovery, which may already be non-mutating, from database-opening paths.
Introduce explicit connection and operation contracts equivalent to:

* InspectProject: Existing-file, non-mutating, version-aware inspection.
* OpenProjectForWrite: Authorized, format-checked, writer-version-checked opening.
* MigrateProject: Explicit, controlled, audited, recoverable migration.

Actual names and API shapes should follow the repository's conventions.
Read-only opening must not execute schema migrations, change journal mode, create database files, modify metadata, or publish replacement files.
Use genuinely read-only SQLCipher/SQLite connection modes where supported and verified. Do not rely solely on `PRAGMA query_only`.
Test existing WAL files, live writers, damaged files, absent files, invalid keys, permissions, symlinks, and legacy formats.
Do not use SQLite immutable mode on a changing database with an active WAL.
5. ADR-011: Approve the Layered O-13 Migration Strategy
Adopt the following migration-protection architecture:

1. A distinct next-generation project-file format and extension.
2. An isolated project-storage location not traversed by normal discovery in historical builds.
3. Internal project-format and minimum-writer-version enforcement.
4. Explicit, separately controlled migration.
5. Verified encrypted recovery copies.
6. Positive evidence that incompatible writers cannot accidentally modify migrated projects.

Illustrative naming, subject to implementation compatibility review:

* Legacy: `projects/.../project.gopmgr`
* New: `projects_v2/.../project.gopmgr2`

The exact names must be documented, tested, and consistently applied.
New applications must discover and manage both supported legacy and new-format projects.
Historical builds cannot be retroactively modified. File extensions and directory isolation provide defense in depth, not an absolute security guarantee. Test actual historical binaries, including explicit-path opening, restoration, import, and other reachable paths.
Migration must:

* Verify account ownership and authorization.
* Acquire appropriate locks.
* Confirm source format and encryption compatibility.
* Preserve the complete source identity and audit history.
* Create an encrypted destination without an ordinary plaintext intermediate.
* Validate the destination's schema, data, references, and audit chain.
* Preserve an independently recoverable source.
* Publish safely using tested filesystem operations.
* Prevent ambiguous dual writable authorities.
* Recover deterministically from interruption at every migration stage.

Account for SQLite WAL and SHM handling, checkpointing, file permissions, locking, atomic publication, directory synchronization, and Windows/macOS/Linux behavior.
Do not silently migrate projects on startup, project-list refresh, portfolio inspection, or ordinary file opening.
Do not activate a production migration until its acceptance and recovery gates have passed.
6. Replace Short IDs with Durable Identity Contracts
The current `internal/db/ids.go` generates identifiers using four random bytes.
Investigate all existing ID-generation paths before changing the scheme.
For new authoritative entities, adopt a clearly documented, cryptographically secure 128-bit identifier scheme, preferably UUIDv4 or an equivalent with independently verified properties.
Requirements:

* Stable identity across legitimate edits.
* Database-enforced uniqueness.
* Deterministic collision handling and bounded retry/error behavior where applicable.
* No silent fallback to short random identifiers.
* No identity based on task name, outline position, row number, or display order.
* Explicit namespace handling for imported source IDs.
* Consistent serialization through Wails, exports, and import mappings.
* Preserve existing identifiers and all historical references unless an approved migration specifically requires a change.

Test collisions, entropy failure, source-ID duplication, cross-project identity, import replay, backup/restore, revision history, and audit references.
Do not confuse authoritative GoPMgr IDs with external Microsoft Project or Primavera IDs.
7. Rust DataFusion / Arrow Worker Architecture
Implement a separate, versioned Rust worker behind the approved ADR-010 supervisor and capability registry.
The worker must be a nonauthoritative analytical component, not a second application core.
7.1 Transport
Use authenticated native gRPC over private local IPC:

* Linux: Unix domain sockets.
* macOS: Unix domain sockets.
* Windows: native named pipes using a verified compatible Rust/Go transport implementation.

Do not substitute loopback TCP, unauthenticated sockets, an HTTP listener, or a stdio-only data protocol.
Implement and independently verify:

* Protocol and capability negotiation.
* Worker binary verification and lifecycle ownership.
* Authenticated bootstrap and peer restrictions.
* Endpoint ACLs and permissions.
* Session, project-copy, revision, and job binding.
* Request correlation and deterministic cancellation.
* Timeouts, heartbeat/liveness handling, and process supervision.
* Bounded message sizes and streaming backpressure.
* Worker crashes, orphan cleanup, and predictable restart behavior.
* Rejection of incompatible, stale, or unauthorized requests.

Do not treat the existing Go↔Java gRPC tests as proof of Go↔Rust Windows named-pipe support.
7.2 Arrow and analytical exchange
Use versioned, typed schemas and bounded Arrow-compatible batches or an independently justified equivalent.
Avoid unnecessary JSON serialization, repeated row-by-row allocation, and unbounded materialization.
Explicitly represent:

* Nullability and missingness.
* Integer widths and overflow.
* Decimal precision and scale.
* Currency and measurement units.
* Dates, timestamps, time zones, and ordering.
* Source project and revision.
* Truncation and completeness.
* Calculation provenance.

Do not convert authoritative monetary values or large integers through lossy JavaScript numbers or floating-point intermediates.
7.3 Resource security
The default confidentiality profile for decrypted project-derived data is SENSITIVE_MEMORY_ONLY.
No unprotected disk spill is permitted.
Protect against:

* DataFusion operator spill.
* Temporary plaintext files.
* Worker logs containing sensitive data.
* Unbounded RSS or CPU consumption.
* Process spawning outside the approved lifecycle.
* Unrestricted filesystem discovery.
* Network access.
* Inadvertent retention after cancellation or crashes.

Enforce process-level resource limits as well as engine-level memory controls. A DataFusion memory-pool setting alone is not a complete memory boundary.
Do not introduce encrypted disk spill without a separate reviewed design and approval.
7.4 Data access
Prefer host-mediated, explicitly authorized data transfer.
The Go host must control source opening, selection, verification, project authorization, and allowed data ranges.
Support bounded streaming and, where required, verified host-mediated random-access reads.
Never let a filename containing `*`, `?`, or other glob characters expand to additional files.
Reject paths, symlinks, reparse-point escapes, and other inputs that violate the selected-source boundary.
8. Implement the Hybrid Analytics Allocation
Use native Go for portfolio rollups and other lightweight deterministic aggregates where the evidence favors it.
Use DataFusion/Arrow for advanced analytical workloads where its query and columnar execution capabilities justify the added worker complexity.
Preserve the existing application-facing analytical capability, including:

* `PortfolioRollup`.
* `ImportTabular`.
* Current portfolio UI functionality.
* Existing EVM and currency behavior.
* Supported CSV, TSV, TXT, Parquet, JSON, and NDJSON formats.

Extend these through versioned APIs where necessary.
For financial values:

* Prefer checked integer minor units or approved exact decimal representations.
* Preserve meaningful zero versus missing values.
* Detect arithmetic overflow.
* Preserve governed rounding rules.
* Preserve reporting-date and currency policies.
* Calculate SPI and CPI from authoritative aggregate values, not averages of individual ratios.
* Use independent mathematical oracles.

DataFusion's BIGINT aggregation overflow behavior must be explicitly tested. A wrapping aggregate is unacceptable.
Use the existing DF0 comparison framework to select execution ownership on evidence, not preference.
Do not force fast Go-native calculations into Rust simply to increase usage of DataFusion.
9. Correct Confirmed Analytics Defects Without Weakening Fidelity
Review every recorded DF0 legacy defect and candidate gap, including LD-1 through LD-9 and G-1 through G-8.
The desired correction policy is:

* Never silently change exact stored values.
* Never silently read files the user did not select.
* Never silently accept malformed data as complete.
* Never silently lose integer or decimal precision.
* Never display structured decimal values as meaningless objects.
* Never describe a truncated preview as the complete input.
* Never silently substitute zero for invalid numerical values.
* Never discard unsupported source fields while claiming complete fidelity.

For malformed, ambiguous, or unsupported inputs, prefer typed errors and explicit diagnostics over false success.
Preserve legitimate input formats and document any intentional compatibility changes.
A correction must have its own independent test, before/after evidence, migration or UI implications, and traceable approved disposition.
Do not weaken a parser or acceptance test merely to imitate a defect in DuckDB.
10. Complete DF0 Before Production Cutover
Retain the accepted ADR-012 tests DF-001 through DF-065.
Complete and independently review:

* DF0-C: Go↔Rust native IPC.
* DF0-C2: Host-mediated bounded reads.
* DF0-D/D2: Security, memory, confidentiality, and admission controls.
* DF0-E/E2: End-to-end performance and numeric fidelity.
* DF0-F/F2: Packaging, dependencies, licensing, and platform validation.
* DF0-G: Final comparative recommendation and qualification report.

Treat previously recorded standalone Linux results as preliminary evidence, not production qualification.
Require native Windows and macOS evidence for their advertised configurations.
Performance tests must measure the complete operation, including:

* SQLCipher reads and authorized snapshot preparation.
* Worker startup or reuse.
* Serialization and IPC.
* Parsing and conversion.
* Analytical execution.
* Return transport.
* UI-facing result preparation.
* Memory and CPU behavior.
* Cancellation and failure recovery.

Use the approved T0/T1/T2/T3 workload tiers and representative real-world fixtures.
Do not invent performance budgets or claim that preliminary measurements satisfy OWN-10.
11. Retire DuckDB Through a Separate Verified Cutover
Do not remove DuckDB immediately merely because its retirement is approved.
Execute the removal when the replacement has passed its applicable functional, security, performance, and platform gates.
The cutover must inspect and update:

* `internal/analytics` implementation and tests.
* Legacy `duckdb` build tags.
* `go.mod` and `go.sum`.
* Transitive DuckDB native bindings.
* Makefile targets.
* Wails build and release flags.
* CI and vulnerability scanning.
* Packaging scripts.
* Installer payloads.
* Licensing notices and SBOMs.
* Documentation and historical ADR status.

Do not leave dead dependencies or a default no-op analytics engine masquerading as a successful replacement.
Preserve separate evidence for the release-compliance obligations associated with PR #146 while DuckDB remains in any build or supported release.
Do not mark ADR-002 operationally superseded until the cutover has succeeded and is documented.
Keep compatibility benchmarks isolated and legally governed.
Review PR #147 carefully. Separate the Wails upgrade from the DuckDB upgrade and investigate any CI failure at the current commit rather than relying on an earlier check result.
12. Preserve the PS-04 Large-Schedule Architecture
Do not allow the analytics migration to distract from the verified 50,000-task bottlenecks.
Maintain the approved sequence:

1. RES-A-1 compatible resource-detector optimization.
2. ADR-011 normalized transactional storage.
3. Compact audit events and legacy-chain continuity.
4. Revision-bound typed queries.
5. Virtualized task and Gantt views.
6. Later selective scheduling optimization only where independently proven equivalent.

Do not:

* Replace the Go CPM engine with DataFusion.
* Make SQLCipher an analytical cache instead of the source of truth.
* Reintroduce full-document rewrites for ordinary task edits.
* Reintroduce full-schedule Wails payloads.
* Reintroduce repeated full-task rescans for conflict detection.
* Activate unproven incremental CPM logic.
* Raise WBS v2 limits without separate authorization.

13. Required Implementation Order and PR Isolation
Create small, dependency-ordered implementation slices.
Recommended work packages:
Slice A — Evidence and ADR reconciliation: Record owner-approved decisions, baseline the repository, reconcile PR dependencies, and document explicit amendments.
Slice B — Non-mutating read paths: Implement strict database inspection/open modes and tests. No new-format migration in this slice.
Slice C — Identifier modernization: Implement the new-ID contract and compatibility tests without altering historical identities.
Slice D — RES-A-1: Complete the independent, compatibility-preserving resource optimization and measured benchmarks.
Slice E — ADR-011 format gate: Implement and verify writer-version restrictions and format recognition before activating new schema behavior.
Slice F — New-format migration: Implement isolated encrypted conversion, complete verification, safe publication, and crash recovery. Keep it inactive until qualification.
Slice G — DataFusion feasibility completion: Finish native IPC, host-mediated reads, security, resource budgets, and platform proof.
Slice H — Hybrid analytics implementation: Introduce Go-native rollups and the qualified Rust worker behind a stable host façade.
Slice I — Analytics fidelity and UI compatibility: Complete supported-format parity, exact-value handling, errors, bounded output, and frontend integration.
Slice J — DuckDB removal: Remove its runtime, build flags, dependencies, packaging remnants, and obsolete tests only after the replacement is qualified.
Slice K — Scalable scheduling workspace: Continue normalized commands, compact audit, revision-bound queries, and viewport rendering in their independently governed PS-04 slices.
Parallelize independent design, fixtures, and feasibility work where safe.
Do not merge unrelated changes merely to reduce the number of PRs.
Do not merge stacked branches without verifying their base relationships and complete relevant checks.
14. Mandatory Testing and Quality Controls
For every implementation slice:

1. Define its invariants and corresponding acceptance IDs before modifying code.
2. Create or preserve independent reference oracles.
3. Test both ordinary and adversarial cases.
4. Use deterministic fixtures and recorded random seeds.
5. Test concurrency, cancellation, malformed inputs, and resource exhaustion where applicable.
6. Test security and data-loss failure modes.
7. Benchmark the actual affected end-to-end operation.
8. Check compatibility with existing encrypted projects.
9. Preserve current production tests and prevent regressions.
10. Verify documentation and generated artifacts match actual behavior.

Use the unified PS-04 acceptance framework, ADR-012 DF-001–DF-065, and all relevant existing project tests.
Every result must be recorded as one of:

* PASS.
* FAIL.
* BLOCKED.
* NOT-RUN.

A rerun is not a substitute for diagnosing a flaky failure.
Do not delete tests, weaken thresholds, remove adversarial fixtures, modify oracles to agree with the implementation, suppress errors, or quarantine failures to obtain green CI.
If tests fail, investigate the root cause and add regression protection.
Cross-platform compilation is not equivalent to running native-platform tests.
15. Mandatory Adversarial Review
Before presenting a slice as complete, independently challenge the design and implementation.
Specifically attempt to demonstrate:

* Older GoPMgr builds can modify migrated files.
* Read-only operations write to project storage.
* A malformed project can trigger unauthorized migration.
* Short or duplicate IDs corrupt references.
* Analytics can access unauthorized project information.
* The Rust worker can read unintended files.
* Worker failure can affect authoritative data.
* Memory limits are bypassable.
* Sensitive data reaches plaintext temporary files or logs.
* IPC authentication or version negotiation can be bypassed.
* Cancellation leaves running or orphaned work.
* Overflow produces incorrect financial totals.
* A malformed file is accepted as complete.
* A preview misrepresents its completeness.
* An upgrade silently changes imported data semantics.
* DuckDB remains transitively linked after cutover.
* Large-schedule performance regresses.
* Audit verification fails after migration.
* Legacy backup and restore lose information.
* A missing test is incorrectly reported as passing.

Correct demonstrated failures before requesting acceptance.
Document unresolved findings explicitly.
16. Required Agent Behavior
For each slice:

* Inspect current repository state and unrelated work before modifying files.
* Prepare a concise design and dependency analysis.
* Identify relevant decisions and acceptance requirements.
* Implement only the authorized scope.
* Avoid speculative abstractions, duplicate frameworks, generated boilerplate, and unrelated refactors.
* Run relevant tests and measure affected performance.
* Inspect all changed files.
* Perform a self-critique and an adversarial critique.
* Correct defects and add preventative tests.
* Update architecture, code maps, migration, release, and developer documentation where required.
* Record exact test commands and observed results.
* Stage and commit only intended changes.
* Preserve a recoverable work state.
* Identify the next highest-value authorized slice.

Do not silently simplify the architecture to make implementation easier.
Do not weaken gRPC security, encrypted storage, audit fidelity, compatibility, or test coverage.
Do not substitute caches, extra databases, cloud services, or different language runtimes for the approved architecture without a documented owner-approved change.
Do not make automatic merge, release, or production-migration decisions merely because CI passes.
17. Required Completion Report
At the end of every slice provide:

1. Current base commit, branch, and PR.
2. Exact files changed.
3. Architectural decisions applied.
4. Requirements implemented and deferred.
5. Independent-oracle results.
6. Tests performed, with PASS/FAIL/BLOCKED/NOT-RUN.
7. Performance results with hardware and fixture provenance.
8. Security and privacy verification.
9. Compatibility and migration evidence.
10. Adversarial findings and their disposition.
11. Unresolved risks.
12. Rollback or recovery procedure.
13. Documentation updates.
14. Next authorized implementation slice.

Never claim that an unexecuted test passed or that a feature is production-ready because a prototype works.
18. First Action
Begin by reconciling the live repository against the accepted ADRs and current PRs.
Record the approved architectural decisions in a traceable maintainer-decision addendum.
Then prioritize the smallest safe production corrections and independently authorized feasibility work:

* Non-mutating project inspection and database-opening safeguards.
* Durable identity-contract implementation.
* RES-A-1 resource-detector optimization.
* Completion of ADR-012 DF0, including Rust gRPC transport and cross-platform proof.
* ADR-011 format-gate and migration design.

Advance through the remaining slices only as their dependencies and acceptance gates are satisfied.
The objective is not merely to replace DuckDB. The objective is to make GoPMgr a dependable, secure, performant, maintainable, local-first professional project-management application capable of handling large schedules without sacrificing existing correctness or functionality.
Preserve architectural quality, mathematical correctness, project-data integrity, historical compatibility, security, and independently verifiable evidence above implementation speed.
```

## 5. G1 authorization clarification, 2026-10-28

The maintainer reaffirmed conditional G1 / P04 approval after reviewing
the conflict between the 2026-10-26 P04 record and §2.5's OWN-11 assertion.
The [dated clarification](ADR-010-G1-authorization-clarification-2026-10-28.md)
governs G1 permission only, including the shared language-neutral foundation
and independent Java/Rust qualification. All P04 conditions, this directive's
gates and the qualified Slice J cutover remain binding. No PR is approved
for merge by this notice. §4 is preserved verbatim.
