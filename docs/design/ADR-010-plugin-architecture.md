<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010: Local-first interoperable worker architecture, scalable project interchange, and future extension boundaries

**Status:** ACCEPTED — architecture only, v2.2. The maintainer accepted it on 2026-10-24 ([decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md)). **Not implemented or tested.** Only P00, P01/G0 and P02/S0 are authorized.\
**Date:** 2026-10-24.\
**Decider:** GoPMgr maintainer.\
**Repository evidence:** GoPMgr `main` at `28a758d154795bb2a20f3cbeb63a12e9130a88ea`; agent's v2.1 document branch `claude/adr-010-plugin-architecture` at `49ac2bf756e3369ac3e90755f3656bfd13c9659c`. Re-check against HEAD before making changes.\
**Supersedes:** proposed ADR-010 v2.1 for architectural direction (INT-04, accepted 2026-10-24). v2.1 is kept unchanged in [`adr-010/v2.1/`](adr-010/v2.1/ADR-010-plugin-architecture.md). No earlier accepted ADR is implicitly overridden.\
**Implementation companion:** [ADR-010 implementation plan](adr-010-implementation-plan.md).\
**Assurance companion:** [ADR-010 test and acceptance strategy](../testing/adr-010-test-strategy.md).\
**Agent controls:** [Architecture preservation and execution prompt](../agent/ADR-010-architecture-preservation-agent-prompt.md).\
**Detailed review:** [v2.1 → v2.2 change audit](../review/ADR-010-v2.2-review-and-traceability.md).

> **Decision discipline.** “Proposed”, “specified”, “demonstrated”, “implemented”, “tested”, and “released” are distinct states. No document statement or test case should be interpreted as proof that code exists or passed a gate. The maintainer must approve any change to the binding objectives, quality thresholds, security posture, user control, or scope.

## 1. Problem and mission

GoPMgr is a Go/Wails/Svelte local-first project-controls application with authenticated local users, encrypted per-project SQLCipher-compatible `.gopmgr` databases, auditing, a Go scheduling kernel, and PDF/document/reporting features. The product must ingest native Microsoft Project files *without Microsoft Project installed*; import supported Primavera P6, ProjectLibre and other validated systems; retain source project structure and meaningful unsupported data; create, compare, selectively merge and export projects; scale well beyond 10,000 tasks; and offer sophisticated deterministic analytics and optional future AI assistance. It must continue operating offline without an external account or hosted service.

The current MSPDI importer is a limited chart importer (`internal/export/mspdi_import.go`): it skips summary tasks, converts duration at an assumed eight-hour day and flattens assignments to names. `internal/analytics/duckdb.go` limits a materialized tabular result to 10,000 rows, not overall persistent storage. `internal/wbs/model.go` caps *one WBS aggregate* at 8 MiB and 10,000 nodes. The existing chart/audit pathway materializes whole-chart before/after JSON. **These limits are distinct; do not simply raise constants**. Normalized records, paged interfaces, and new audit commands need their own reviewed data architecture.

This ADR selects the integration boundary and non-negotiable functional/security properties. ADR-011 must design and approve normalized project persistence and project-format compatibility before production storage changes. An AI/MCP ADR and third-party extension ADR require separate approvals but must be compatible with this foundation.

## 2. Binding quality invariants (QI)

| ID | Requirement | Violation examples |
|---|---|---|
| QI-01 | Local-first and offline; no mandatory cloud service | Network required to parse a schedule or open a project |
| QI-02 | GoPMgr host is sole authority for authentication, DEKs, SQLCipher, storage, audit, scheduling and revisions | Worker opens `.gopmgr`, receives a DEK or commits raw SQL |
| QI-03 | Native gRPC + **typed**, versioned Protocol Buffers for worker business data | Unreviewed custom stdio-framed data protocol or `Any` containing authoritative untyped JSON |
| QI-04 | Local operating-system IPC is preferred and **binding unless expressly excepted** | Silently substituting loopback TCP on Windows |
| QI-05 | Cross-platform release correctness and faithful behavior are gates | Claiming Windows support without tested Java transport |
| QI-06 | Field-level fidelity, source provenance, explicit loss/unknowns and user confirmation | Silent skipping, fabricated source values, “full fidelity” without per-version evidence |
| QI-07 | Project publication is crash-consistent and cannot expose an incomplete import | Staging folder listed as a project, deleting retained source evidence |
| QI-08 | Existing encrypted projects, audit histories and recovery paths remain usable and unmodified unless migrated under a separate approved contract | Old hash rule changed, destructive migration, unsafe downgrade assertion |
| QI-09 | >10,000 editable records without whole-document UI or audit rewrites | Raising `maxImportRows` and sending 100,000 tasks through Wails as one response |
| QI-10 | Minimal **generalized internal plugin contract**, with bundled MPXJ first | MPXJ-specific supervisor or hardcoded transport/API assumptions preventing a second worker |
| QI-11 | AI assistance is optional, permissioned and **proposal-first**; deterministic algorithms remain authoritative | Autonomous edits, AI-generated EVM numbers treated as ground truth |
| QI-12 | Fail closed; no silent reduction of requirements, release scope, fidelity, OS support or tests | Treating a BLOCKED interoperability test as PASS |
| QI-13 | Enforced OS restrictions distinguished from host permissions and observed behavior | Describing unsandboxed bundled Java process as unable to read other files |
| QI-14 | Performance and usability are demonstrated on real supported machines and fixture families | Asserting 50,000 tasks based only on an in-memory synthetic fixture |
| QI-15 | Change control is explicit, traceable, reversible where feasible | Agent edits ADR to make a hard gate optional without maintainer approval |

These invariants override phase shortcuts. **An unproven capability stays unproven; it does not get quietly removed.**

## 3. Decisions

### D-01 — Out-of-process, language-neutral modules

Use **subprocess workers** (Go, Java, Rust and potentially other languages) supervised by Go. Reject Go's `plugin` package for distribution, JNI in the GoPMgr process, and rewriting MPXJ from scratch as the initial strategy. Java failures should be isolated from the GoPMgr process; this **does not mean Java is sandboxed**. The worker has no privileged callback API or direct project/database access.

### D-02 — Internal capability registry exists from the first version

Keep the first *installable* scope narrow: **one bundled, authenticated MPXJ worker**. However, the internal registry and manifest vocabulary must be neutral to worker language and vendor from day one. It shall support: stable worker ID, vendor, executable/artifact location, version, protocol range, capabilities (operation + direction + format/version), trusted provenance, lifecycle status, resource policy, rollout state and per-platform transport availability. A second worker must not require restructuring the supervisor or protocol envelope. No arbitrary third-party code, marketplace, plugin DOM injection or public SDK in G1–G4. Plugin Center presents bundled capabilities now and scales to more providers later.

### D-03 — gRPC is the worker business-data protocol, not custom stdio framing

**Binding architectural preference:** gRPC over local IPC using generated typed protobuf contracts and standard RPC semantics. Do not choose raw protobuf frames on stdin/stdout as the primary data path; avoid custom multiplexing, flow control and cancellation protocols where gRPC provides them. Stdio may only carry a constrained one-time **bootstrap descriptor or emergency diagnostic output**, not project payloads or a fallback worker RPC protocol, and only if its threat model is approved. Prefer host-owned endpoint creation and worker-initiated connection, authenticated per launch. The transport must expose a standard gRPC HTTP/2 connection and use a single generated logical service contract across operating systems.

| Target | Preferred local transport | Implementation state / hard gate |
|---|---|---|
| Linux | UDS | G0 proof with Go + bundled Java runtime / grpc-java, permissions and cleanup |
| macOS (Intel + Apple Silicon as supported) | UDS | G0 native Netty/Java runtime, architecture signing/notarization and cleanup |
| Windows | Windows named pipes | **Candidate, not demonstrated**: Go `go-winio` can use pipes as `net` transport, but Java grpc-java compatible named-pipe HTTP/2 transport must be proven |
| Windows alternative | AF_UNIX / UDS | Only if demonstrated with the selected JDK, Java gRPC transport and supported Windows versions |
| Exception | Authenticated loopback gRPC | **Not approved as default**; written owner-approved exception with compensating controls and product-support disclosure, only after native paths fail evidence-based gates |

**Do not infer Windows named-pipe grpc-java support** from Go named-pipe support or .NET Kestrel documentation. grpc-java issue `#12027` closed without evidence of an integrated production transport; test real Go↔Java gRPC over HTTP/2 in both directions. If no acceptable Windows local IPC is proven, **block Windows worker shipping** and present the compatibility issue for owner decision; preserve existing Windows core features. Do not silently install raw stdio framing as a workaround.

### D-04 — Direct grpc-go supervisor; reconsider go-plugin only with evidence

Prefer a small host-owned subprocess supervisor with grpc-go (not `hashicorp/go-plugin` runtime dependency in G1) because platform-specific local IPC and explicit host-authenticated bootstrap are first-class requirements. All RPC contracts must be independent of the supervisor, so later use of go-plugin or another process manager is possible under a documented ADR amendment. Generated protobuf code, descriptor sets and IPC transport modules must be version-pinned; no Java reflection-based interchange serializer.

### D-05 — Authenticated, private, ephemeral endpoints

The Go host creates a new unpredictable, operation-scoped IPC endpoint and accepts only the authorized worker connection. UDS directories are private and owner-verified; defend against symlink/hardlink/swap, stale socket reuse, overlong paths and permission confusion. Windows named pipes use an explicit restrictive security descriptor (not an implicit default ACL), reject remote access where applicable, and prevent pipe squatting. Use a one-time secret/credential delivered by a secure bootstrap channel, never in command arguments, persistent manifest, unprotected environment, logs or project records. Require TLS or channel credentials only where meaningful for the chosen IPC and maintain replay-resistant app-level authentication, peer/OS credential verification where available, protocol negotiation and short-lived call authorization. Security must not depend only on endpoint obscurity. Reject connections failing worker identity, protocol version, nonce, capability, or job binding.

### D-06 — Managed worker lifecycle and bounded parallelism

Default first implementation is one job per worker process for fault isolation; a measured session-scoped pool can be added without changing protocol/API. State machine: `DISCOVERED → VERIFIED → STARTING → CONNECTING → AUTHENTICATED → READY → RUNNING → DRAINING → EXITED` with terminal failures and reason codes. All processes, subprocess trees, descriptors, sockets and scratch directories are owned, bounded, cancelled and reaped; no goroutine or process orphaning. Protect host + JVM aggregate memory, simultaneous worker count, CPU time, file size, decompressed bytes, parse depth, source/timephased expansion, stdout/stderr logging and disk usage. Maintain per-job deadline, idempotency key and status. Reused session workers must not survive account/project switch without explicit complete reauthorization and state reset; default is restart.

### D-07 — Strict host authority and permission checking

Workers accept *only explicitly selected source material* and return structured data. All authenticated operations are bound to principal incarnation, session epoch, account, project copy (where relevant), issued project handle, expected revision/generation, operation ID, capability, source digest, worker digest, approval receipt and TTL. Re-check at actual commit/publish, **not only at job admission**. No worker access to DEKs, credentials, SQLCipher handles, project directories or host implementation internals. Passing a file path is disallowed by default; a host-supplied stream or strictly reviewed read-only handle is used. Worker is trusted bundled code but has same-OS-user authority absent effective OS sandboxing; never claim filesystem/network denial unless enforced and tested.

### D-08 — Three identities, plus versioned mapping observations

Separate (a) source artifact ID: exact bytes, digest, parser/version; (b) logical source project lineage: confirmed project identity independent of path/name/hash; and (c) source entity keys scoped to lineage, producer, record type and actual UID namespace. Introduce host entity IDs independent of imported ordinals, names or WBS display codes. Preserve evidence for copied projects, UID reuse, moved/re-parented activities and external links. Identity mappings are append-only facts or are retired by separate events; never mutate or retroactively erase historical observations. Re-import does not auto-merge on a shared file GUID/UID if lineage diverges. Ambiguity requires review. Preserve original UIDs separately from task numeric display IDs.

### D-09 — Multidimensional field-fidelity ledger

Do **not** compress all source-field semantics into one disposition. Each relevant source field/record-group receives independent properties: source presence (`present/absent/unreadable/unknown`); transformation (`exact/lossless/approximate/derived/not_applicable`); representation (`native/typed_extension/opaque_preserved/not_stored`); editability; exportability; validation; warning severity; and provenance/reader limits. This permits, e.g., `lossless + native + editable + export_supported`, or `present + opaque_preserved + read_only`. Known-unreadable fields are not the same as unknowable properties. Strict mode fails when owner-marked essential fields cannot meet **specified fidelity and usability expectations** (exact representation versus merely retained evidence). Reviewed mode demands an accurate grouped report, acknowledgments for material loss and previewed decisions. Inspect mode performs no project mutations. Per-format/version field matrices and a mapping document are required before capability claims.

### D-10 — Host-controlled all-or-nothing new-project import

Staging, validation and a single publication boundary are mandatory for creating a new project from MPXJ. Workers never mutate project data. Staged project records may be written in bounded **invisible** transactions. The staged `.gopmgr` database is encrypted with the authorized user's DEK. Completed project data, the import receipt, audit evidence, and **retained source/field mapping evidence must all be sealed and recoverable** before publication. Do not put retained evidence in a sibling staging directory that is deleted after renaming the database. Use one project-owned sealed folder (or a separately atomic, encrypted, content-addressed store with audited references and its own recovery proof); prefer project-owned by default for self-contained backups. Respect documented user retention choice; if no source copy is kept, say so and do not claim it is recoverable.

The host validates integrity, cipher integrity, foreign keys, counts, referential mappings and audit chain; checkpoints and closes database; prepares private file permissions; fsyncs where available; creates/updates a durable intent journal atomically and fsyncs it; and performs a same-volume no-replace folder rename into the existing `projects/` scan namespace. Publication must have **no new project writes required** to be considered complete. Durable `PREPARED/SEALED/PUBLISHING/PUBLISHED/RECONCILED` transitions and crash recovery are binding, while OS-specific durability guarantees and hard power-loss testing remain candidly bounded. Folder name collision is resolved with atomic no-replace primitives, never check-then-rename. Post-publish verification failure quarantines/flags as appropriate; never silently deletes a possibly valuable published artifact.

Staging reconciliation is restricted to **host-created operation-owned staging and journal records**, not arbitrary legacy projects, which correctly lack ADR-010 journals. Recovery of SEALED-but-unpublished imports requires a fresh authenticated session and **fresh publication authorization**; an expired session approval cannot be reused to publish. Already published operations are reconciled idempotently without duplication. Ensure clean catalog/discovery semantics and backup/restore includes all retained evidence.

### D-11 — Existing project merges require shadow generations; not v1 release

No segmented visible commits into an existing project. Future selective merge uses a shadow normalized schedule generation and one atomic active-generation pointer switch with revision/approval checks, or a demonstrably equivalent transaction model. Comparison is read-only first; mapping and conflicts are user-visible; no unreviewed “source wins” behavior. The first release delivers *inspect → new-project import* but S6 compare and controlled merge remain **binding program deliverables**, not a deleted goal.

### D-12 — ADR-011 owns normalized storage and upgrade safety

Large editable schedules use indexed normalized typed tables for activities, outline/hierarchy, relationships, calendars and exceptions, resources, assignments, costs, constraints, progress, custom fields, baselines, timephased series, notes, external references, source lineage and revisions. Charts and Gantt are projections, not authoritative stores. The existing WBS v2 scope contract remains independent of imported schedule hierarchy; *do not auto-create or expand it beyond limits*. Retain legacy charts for existing files and Quick MSPDI chart import behind clear limitations. ADR-011 must supply migration-ready DDL, indexes, query plans, transaction boundaries, schema- and min-writer-version guard, read-only/unsupported behavior, audited migration and backup/restore, encrypted fixtures, version compatibility matrix, role/admin-escrow tests and rollback/forward recovery.

**Pre-gate builds cannot retroactively reject future file versions.** Shipping a format gate a release ahead improves safety only for gate-enabled releases; older pre-gate binaries require specific user-facing compatibility warnings and tested migration/backup policies. Check every writer entry point (GUI, CLI, backup, repair, migration, admin, restore). Do not imply universal downgrade safety. Verify actual bundled SQLite/SQLCipher capabilities (repo baseline SQLite 3.33.0), rather than relying on modern SQLite features that may not exist. Preserve SQLCipher cipher/layout compatibility unless an explicitly approved upgrade passes existing gates.

### D-13 — Audit continuity with compact future events

Do not rewrite old audit event bytes or change existing event hashes/canonicalization. New typed domain commands and data changes append compatible records to the existing verified chain, using string encodings for exact money, large integral values and revisions where current JSON canonicalization would lose precision. Separate short-lived idempotency receipts, undo/redo history, immutable forensic evidence and retention policies. Large blobs/evidence referenced by audit entries must have immutable digests and verifiable lifecycle. Any checkpoint and replay path requires threat-modeled, reproducible proof, not a claim based on shorter payloads alone. No migration silently truncates existing audit events.

### D-14 — Scalable queries, UI and deterministic analytics

Never send whole 50,000-task schedules through Wails or render every row as DOM/SVG. Build keyset-paged typed query APIs with revision-bound opaque cursors, virtualized tree grid, viewport/time-window Gantt data, clip-correct relationships, server-side search/filter/sort, no long-lived SQLite read transactions across Wails calls, accessible keyboard/screen-reader equivalents and preserved navigation anchors. Expensive analytics are cancellable jobs; scheduling/EVM/exact-money kernel remains authoritative. Benchmark selective denormalized projections, cache invalidation, timephased interval aggregations, and an optional Arrow/DataFusion engine **only when measured beneficial**. DuckDB replacement remains a separate owner-approved decision; do not silently replace it or reintroduce it as the only analytics engine. `maxImportRows` remains a per-result cap until pagination replaces it; expose explicit truncation status and never mislabel a partial result as an imported dataset.

### D-15 — Usability is a release-quality requirement

G2/G3 provide a five-stage accessible import assistant (Choose → Inspect → Map → Review → Import), format/version and capability explanations, material-loss summary, explicit essential-field selection, memory/disk estimates labeled as estimates, import receipt/history, job center (progress counters, retry, cancellation boundary, recovery), Plugin Center (bundled module status + read/write asymmetry), simple Quick Chart Import explanation and recoverable errors. G4/S5 provide large-schedule search, sticky columns, breadcrumb, dependency jumps, saved views, minimap, source-vs-calculated date toggle, virtualized Gantt and keyboard/IME behavior, all under ADR-008 localization and release accessibility checks. S6 provides side-by-side compare, filtered selections, visible off-screen counts, conflict explanation and scoped apply preview. Do not make inaccessible virtualization a performance shortcut.

### D-16 — Future AI/MCP and community extensions stay governed, not eliminated

Reserve extension contracts for model providers, analytics jobs and controlled MCP tool/resource adapters. No AI model modifies projects autonomously: tools are read-only first, writes produce proposals with deterministic previews, human approval, current epoch/revision, audit and rollback semantics. Cloud is optional and off without informed consent; user never needs AI to operate GoPMgr. MCP transport, grants, revocation and threat model require a separate ADR. Arbitrary third-party binary plugins are deferred to G7 pending per-OS executable confinement, distribution/signature/revocation and consent gates. Keep the **internal** registry neutral now; defer the **public** SDK/marketplace.

### D-17 — Packaging, reproducibility and legally sound distribution

Pin and verify MPXJ (initial candidate **v16.10.0**, not an unchangeable requirement), full resolved dependency tree, Java LTS runtime used to build a jlink image, grpc-java and native transports, protoc/codegen, Go dependencies and platform-specific binaries; produce an SBOM, provenance/REUSE/SPDX notices, reproducible package inventory and signed installers. The pinned MPXJ release must pass upstream vulnerability, licensing, third-party transitive license and native-library review. LGPL-2.1-or-later obligations require **legal review against the actual distribution architecture**. A vendor-signed, digest-pinned path and a separate explicitly user-authorized replacement/development mechanism must be reconciled—never silently weaken the production integrity path or categorically claim licensing requirements are solved. Runtime bundles and JNI/native transport artifacts need notarization/Windows signing evidence; exact JDK and transport versions are chosen by G0 evidence, not by guessed compatibility.

### D-18 — Measurable performance and safety targets

Targets: T0 (100–500 tasks), T1 (10,000), T2 (50,000: product acceptance), T3 (100,000+ tasks and million-plus timephased points: stress). Benchmarks include representative MPP/MSPDI/XER/POD formats, trees, calendar exceptions, resources, dependencies, cost/earned value, custom fields and irregular timephased records. On owner-approved minimum/recommended hardware profiles, measure full pipeline: cold JVM start, bytes read, MPXJ heap + native memory, transfer, staging, seal, publish, first paint, scroll, search, edit, recalculation, analytics, export, backup and restore. Distinguish warm/cold and p50/p95 with adequate sample counts; do not assert performance on fewer than documented repetitions or synthetic-only sources. Budget failures block agreed release gates; budget values remain proposals until G0/S0 owner approval.

### D-19 — Explicit plan governance and stop conditions

Approved decisions are **not independently overridable by coding agents**. Every work package traces to this ADR, the implementation plan and unique test IDs. To alter a hard requirement, submit a separate, clearly labeled Architecture Change Request identifying existing decision, proposed alternative, user-impact, security/fidelity/performance/OS tradeoffs, evidence, rejected options, and rollback, and obtain maintainer authorization *before* modifying normative specifications or production scope. Technical infeasibility causes a **BLOCKED** outcome, never self-authorized scope reduction, silent fallback, fake PASS or permanent deferral. Keep original reference files immutable; update only the canonical proposed documents and change log. External claims/versions require primary-source validation; tests require actual execution before being labeled PASS.

## 4. Rejected and conditional alternatives

| Option | Disposition | Explanation |
|---|---|---|
| In-process Go plugins | REJECT | Windows and build compatibility, crash containment |
| In-process JNI JVM | REJECT | Reliability/security/packaging coupling |
| Rewrite MPXJ in Go/Rust now | REJECT | Duplicates substantial parser maintenance, delays interoperability |
| Custom protobuf stdio as primary IPC | REJECT for this design | User explicitly requires native gRPC local IPC and long-term cross-language contract |
| HashiCorp go-plugin as mandatory dependency | DEFER | Useful technology; inspect only if it meets cross-platform local IPC policies |
| gRPC over Linux/macOS UDS | ACCEPT TARGET, gated by G0 | Java/native transport/packaging must pass |
| gRPC over Windows named pipes | ACCEPT TARGET, **unproven** | Requires real Java HTTP/2 transport over named pipes |
| Windows AF_UNIX gRPC | EXPERIMENTAL ALTERNATE | JDK OS support ≠ grpc-java transport support |
| Authenticated loopback TCP | OWNER EXCEPTION ONLY | Not a covert fallback; review security and release claims |
| Persistent JVM by default | DEFER pending measurement | One-job-per-process simplifies first implementation |
| Embedded Java file parser | REJECT | Violates host/worker boundary |
| Automatic re-import merge | REJECT | Conflicts with identity/approval model |
| General third-party plugin marketplace in G1 | REJECT | Increases attack surface before bundled worker assurance |
| Source deletion or fidelity downgrade for convenience | REJECT | Violates provenance and explicit user retention policy |

## 5. Completion semantics

**MVP (G0–G4):** documented and approved contracts, verified cross-platform transport for advertised systems, bounded worker, MPXJ inspect + high-fidelity import as *new project*, encrypted/crash-consistent publication with evidence, required migration gate, real packaged installer test, and usable import/recovery UI. Export only format/version combinations that pass regression fixtures; no native MPP write claim.

**Program completion:** MVP plus S5 scalable interactive editing, S6 comparison/approved selective merge, S7A sophisticated query/analytics, S7B compact audit/replay, optional subsequent AI/MCP G5/G6 and audited third-party G7 once independently approved. Deferred work remains a live tracked requirement and cannot be marked complete by shipping G4.

**Release approval:** Maintainer acceptance of D-01…D-19; all assigned phase tests PASS or formally documented blocking exception; supported-platform matrix; legal/license review; durable recovery demonstrations; no uncontrolled privacy/security or usability regression.

## 6. Open decisions explicitly reserved for evidence

1. Which *tested* Windows Go↔Java gRPC local transport implementation is viable, including named-pipe JNI/Netty adapter cost and AF_UNIX compatibility; reject unsupported claims.
2. Whether transport bootstrap credentials can be passed securely by inherited descriptor/pipe on each platform; choose measured implementation without sending project data via stdio.
3. Chosen Java LTS vendor/version and native Netty dependencies, bundle size, signed macOS/Windows package behavior.
4. Security/implementation shape of an explicit LGPL user replacement path and vendor integrity path, subject to legal review.
5. Normative schema, format gate, migrations and historical file protections in ADR-011.
6. Hardware profiles, performance budgets and supported fixture families after G0/S0.
7. Whether storing source bytes in project DB or encrypted project-owned chunk store has better measured restore/performance behavior.
8. Capability-specific permission scopes and any separately approved cloud/AI or third-party install model.

None of these open decisions authorizes reducing the stated requirement. A failed feasibility spike changes release readiness, not user intent.

## 7. Normative source and precedence

1. Approved GoPMgr ADRs and security/release invariants applicable to their scope.
2. Maintainer-accepted ADR-010 v2.2 decisions (accepted 2026-10-24, with the clarifications in the [decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md)), followed by accepted ADR-011, AI ADR, and later supplemental decisions within their scope.
3. This ADR's implementation and test companions; the more restrictive applicable security/data-integrity rule prevails until the maintainer resolves a conflict.
4. Current repository code and reproducible tests establish **implemented behavior**, not authority to weaken an approved goal.
5. Archived earlier v1/v2.0/addendum and agent v2.1 drafts document context. Their binding requirements remain unless explicitly superseded in an accepted decision record.
6. Upstream primary documents for specific external technologies and licenses. Revalidate before an implementation PR.

**External reference anchors (verified 2026-10-24):**

- MPXJ upstream releases: https://github.com/joniles/mpxj/releases ; https://github.com/joniles/mpxj
- grpc-java repo and transport description: https://github.com/grpc/grpc-java ; Java Windows pipe discussion: https://github.com/grpc/grpc-java/issues/12027
- Microsoft's Go Windows pipe transport: https://github.com/microsoft/go-winio
- Java AF_UNIX JEP 380: https://openjdk.org/jeps/380
- SQLite WAL/foreign keys/PRAGMA documentation: https://www.sqlite.org/wal.html ; https://www.sqlite.org/foreignkeys.html ; https://www.sqlite.org/pragma.html
- GoPMgr source and existing ADRs: https://github.com/draconianneocoder-cpu/GoPMgr

## 8. Document history

| Date | Version | Change |
|---|---|---|
| 2026-10-23 | v1, v2.0, addendum | External planning bundle; kept unchanged in [`adr-010/`](adr-010/README.md) |
| 2026-10-24 | v2.1 | Agent consolidation (`49ac2bf`); kept unchanged in [`adr-010/v2.1/`](adr-010/v2.1/ADR-010-plugin-architecture.md) |
| 2026-10-24 | v2.2 | Maintainer bundle: gRPC over local IPC, internal registry, project-owned evidence, QI-01…QI-15, D-01…D-19 |
| 2026-10-24 | v2.2 integrated | Verified against `28a758d`; corrections listed in the [integration review](../review/ADR-010-v2.2-integration-review.md). No requirement weakened; status stays PROPOSED |
| 2026-10-24 | v2.2 accepted | The maintainer accepted D-01…D-19, QI-01…QI-15, OWN-01…OWN-11 and INT-01…INT-04 ([decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md)). Architecture only: P00, P01/G0 and P02/S0 authorized; nothing implemented. Earlier versions kept unchanged |

## 9. Maintainer acceptance

On 2026-10-24 the maintainer accepted D-01…D-19 and QI-01…QI-15 as the binding target architecture. The [decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md) is authoritative; this section summarizes it. **Acceptance is architecture only.** It is not evidence that any implementation, transport, licence position, performance target or test has been met.

| ID | Disposition | What it binds |
|---|---|---|
| OWN-01 | Approved: target design | D-01…D-19 and QI-01…QI-15, with the record's clarifications; no unapproved downgrade |
| OWN-02 | Approved: transport policy | gRPC/HTTP/2 and typed Protocol Buffers over private UDS (Linux, macOS); Windows restricted named pipes primary, proven Windows AF_UNIX acceptable; real Go↔Java interoperability on each advertised OS is a G0 gate |
| OWN-03 | Approved: no implicit fallback | No protobuf-over-stdio business protocol, no silent loopback TCP; TCP only through an accepted ACR |
| OWN-04 | Approved | Minimal language-neutral internal registry from G1; MPXJ only at first; no public SDK or marketplace before G7 |
| OWN-05 | Approved | Evidence and reports retained with the project, encrypted and restorable; keeping the original file is the recommended default, with a visible opt-out |
| OWN-06 | Approved | A SEALED import found in a new session needs re-authentication, ownership and integrity checks, and fresh approval |
| OWN-07 | Approved: phased delivery | First release is Inspect → Review → Import as new project; S6, S7A and S7B stay mandatory program deliverables |
| OWN-08 | Approved | ADR-011 defines and validates writer gates, normalized storage, encrypted migrations, legacy paths and downgrade disclosures |
| OWN-09 | Approved for G0 investigation; final choices reserved | MPXJ 16.10.0, Java LTS and jlink, native transports, signing, SBOM and LGPL distribution are assessed; release pins need evidence, legal review and approval |
| OWN-10 | Approved: goal and method; budgets reserved | Usable, editable 50,000-task schedules (T2) and T3 stress; hardware profiles and numeric budgets follow S0 |
| OWN-11 | Approved for P01 experiments only | grpc-go, Protocol Buffers and go-winio may be evaluated in isolated spikes; adding them to `go.mod` needs G0 results, a `docs/DEPENDENCIES.md` review and an authorized G1 slice |
| INT-01 | Approved | The intent journal and seal manifest are HMAC-authenticated and verified before recovery may authorize publication. A valid MAC is not freshness, so rollback is detected or handled by independent state checks. The algorithm, key derivation and anti-rollback design are reserved for a security-reviewed G3 contract |
| INT-02 | Approved as a G0 candidate | A parent-owned, private, one-shot stdin pipe may carry the bootstrap credential and bounded endpoint metadata; business data still uses gRPC over local IPC |
| INT-03 | Approved: evidence-based, non-destructive storage policy | Live SQLCipher/WAL storage is not declared safe on unvalidated sync-managed or network filesystems. Existing locations are never relocated, deleted or blocked from reading by heuristic; only new publication may warn or block, under an approved policy. A default-root change needs its own migration decision |
| INT-04 | Approved | v2.2 is the authoritative accepted architecture; v2.1 is superseded and preserved unchanged |

**Authorized now:** P00 documentation, P01/G0 isolated feasibility, and P02/S0 evidence (record §5.1). **Not authorized:** G1 onward, production dependencies, ADR-011 implementation or migrations, publication, a default-root change, signed releases, and G5–G7 (record §5.2).
