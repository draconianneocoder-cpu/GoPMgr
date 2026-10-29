<!--
SPDX-FileCopyrightText: 2026 GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Critical Design Review and Improvement Addendum — GoPMgr ADR-010 v2.0

**Status:** Independent technical critique and proposed amendments; **not accepted or implemented**  
**Review date:** 2026-10-23  
**Reviewed document:** `GoPMgr_ADR-010_Plugin_Architecture_and_Scalable_Interoperability_Plan_v2.md` (1,818 lines, ~24,000 words)  
**Reference application:** `draconianneocoder-cpu/GoPMgr`, with repository code from the current conversation and the ADR's pinned HEAD reference  
**Purpose:** Provide an actionable, risk-ranked amendment backlog for a maintainer or coding agent. **Do not** blindly patch production code from this addendum. Confirm HEAD, accepted ADRs, underlying dependencies, fixtures and migration requirements first.

## Executive verdict

**Decision:** Retain out-of-process MPXJ integration, a Go-owned data and authorization plane, normalized SQLCipher-backed editable schedule entities, an explicitly distinct WBS scope model, typed field-fidelity receipts, paged/virtualized interfaces and separately governed MCP/AI. **Do not authorize general implementation of v2.0 without closing the P0 gaps below.**

**Why:** The plan is strongest at defining authority and prohibitions but under-specifies critical physical/temporal contracts: cross-file publication and crash recovery; longitudinal import identity; realistic MPXJ heap and scratch behavior; authentication of local AI clients; query snapshot/cursor lifetime; and the physical implementation of exact-value storage, materialized hierarchy and incremental derived computations.

## 1. Findings requiring changes before production approval

### P0-01 — Separate source snapshot identity from logical project/entity lineage

**Reference:** v2 §8.3, §8.7, §11.3, §11.5.

The tuple `(source_digest, source_project_key, entity_kind, source_unique_id, source_scope)` can identify a record **within a particular imported snapshot**, but a digest changes whenever one byte of the original file changes. Re-importing revised versions of the same source project then creates a new key even for unchanged tasks, undermining stable matching, 3-way diff, no-duplicate merge and selective update.

**Required revision:** Introduce separate:

- `source_artifact_id`: exact bytes/reader/version, SHA-256 and import timestamp.
- `source_lineage_id`: established from trustworthy native project GUID/UID, producer characteristics and explicit human-confirmed continuity, **never blindly from name or path**.
- `source_entity_key`: producer-specific stable source UID **scoped to lineage and record type**. Preserve task ID/ordinal separately from UID; detect reused GUIDs/UIDs in copied projects.
- `import_mapping_version` and immutable `entity_link` records with validity intervals, mapping confidence category, creation/retirement and human-confirmed overrides.

**Acceptance:** Revised same-project MPP files match stable tasks despite file hash changes; cloned schedules with reused UIDs do **not** merge automatically; added/deleted/reparented tasks and UID reuse are demonstrated with real/synthetic tests.

### P0-02 — Define crash-consistent new-project publication across database and filesystem boundaries

**Reference:** §9.2 step 11; §11.5; §15.4.

A transaction inside a new `.gopmgr` SQLCipher database cannot atomically cover publishing a different file path and updating `system.db` or another project catalog. WAL, database close/checkpoint, rename, fsync and catalog registration introduce separately failing steps. The plan promises too much unless it defines exact recovery states.

**Required revision:** Introduce a persistent **host-owned installation intent journal** with operation ID, target generation, source/stage hashes, schema/cipher format, `PREPARED → SEALED → PUBLISHED → REGISTERED` phases, idempotent restart reconciliation, deterministic orphan/quarantine policy and a committed receipt. Prove that a staged encrypted project is independently openable and passes integrity/FK/audit checks **before** publication. Explicitly account for `-wal`/`-shm` sidecars and platform-specific rename durability. Prefer a single sealed/cold database file before atomic publish, and do not claim a multi-file transaction is atomic merely because SQLCipher transactions succeeded.

**Acceptance:** Inject failures before/after commit, checkpoint, file sync, rename, directory sync, catalog registration and UI notification. Recovery must yield exactly one discoverable complete project or a documented unregistered recoverable artifact, never a silently truncated usable project. Existing project imports remain a distinct single-file transactional protocol.

### P0-03 — Define the real Java MPXJ memory model and file access policy

**Reference:** §6.1, §7.4, §9.2, §11.11.

Streaming host-to-Java bytes and Java-to-host records bounds **IPC transfer** but does **not** bound MPXJ's parser's internal object graph, decoded timephased records, JVM heap or the need for seekable source input. Confidential scratch copies and repeated hashing/copying may dominate elapsed time and disk footprint.

**Required revision:** Probe actual source path/stream/seek support for each promised reader; benchmark peak JVM heap, combined host+worker RSS, temp bytes and parsing time by format. Provide two *explicit* source strategies with threat models: (a) streamed bytes to a scoped private read-only seekable worker scratch; (b) where sandbox and file-handle support are actually proven, a controlled already-open read-only source handle/path without redundant copy. Do not pass unrestricted user paths to unconfined third-party code. JVM `-Xmx`, cgroup/job object memory limits, max timephased expansion, CPU/deadline and worker-count limits are independent. Note that MPXJ 16.0+ JSON output omits timephased data by default; configure and validate necessary writer/export flags rather than assuming defaults.

**Acceptance:** Explicit worst-case fixture capacity plan and refusal path for resources too large for the target machine. Verified that streaming does not hide unbounded parser memory.

### P0-04 — Threat-model local MCP consumers and credential lifecycle, not just exposed tools

**Reference:** §10.2, §7.6.

An external AI application may start/connect to a GoPMgr MCP endpoint while the GUI is locked, logged out, switched users, or not running. `stdio` transport alone does not establish which human user/project authorized the request. Treating a port/stdio channel as a logged-in session invites a confused-deputy cross-project access bug.

**Required revision:** Design a **host-mediated client grant**: explicit account/project/copy scope; read-only capability set; short expiry, revocation, secure handling of bearer material; binding to OS process/channel where supported; auditable connect/disconnect; reauthentication on GUI sign-in changes. Decide whether MCP is **embedded in the running authenticated app** or a separately launched broker—do not permit both without distinct protocols. Remote client use requires a separate decision. Apply current MCP transport security including Origin validation if Streamable HTTP is introduced.

**Acceptance:** Revoke on sign-out, project switch, expired grant, GUI exit, admin-account switch, worker restart and stale tool continuation. No plaintext key in command line, ordinary logs or permanent config.

### P0-05 — Resolve contradictory atomicity language for segmented imports

**Reference:** §0 rule 3; §11.5 first bullet.

The plan promises all-or-nothing import but proposes 'segmented import with independent atomic checkpoints' if one transaction becomes too big. Independent committed segments violate all-or-nothing visibility unless a generation/staging mechanism prevents partial records from becoming authoritative.

**Required revision:** Choose **one** of:

1. For new-project import: build a complete staged encrypted DB in many internal bounded transactions, validate it, then **publish only the complete sealed database**. No partial imported project ever becomes discoverable/active.
2. For existing project: use an immutable shadow schedule generation plus atomic generation-pointer switch if a single logical write transaction would exceed measured budgets. Preserve the old generation until the new one is complete, validated and approved. This needs a separate migration/garbage-collection design.

If neither mechanism is implemented, explicitly cap an import and **reject with a clear resource-limit error**. Never label segmented visible commits as all-or-nothing.

### P0-06 — Define fidelity as an evidence-backed per-field matrix and strict modes

**Reference:** §§8.4–8.6, 9.2, 11.13.

A list of unsupported fields is necessary but does not establish 'full fidelity'; the parser itself can omit unsupported source fields, some derived attributes are not persisted by the source, and the reader may silently normalize. Original bytes preserve evidence, not lossless semantic round-trip of edits.

**Required revision:** Add `ImportCoverageMode`: `inspect_only`, `strict_required_fields`, `reviewed_best_effort`. Mark each supported entity/field with `supported_exact`, `supported_transformed_exact`, `derived`, `unsupported_by_reader`, `unmodeled_host`, `unresolved`, `user_excluded`, `failed`. Require source schema/field catalogs where available, invariant checks, and a reproducible comparison matrix per **format and version family**. Do not promise perfect detection of unknown binary attributes MPXJ itself cannot expose. `strict_required_fields` fails before commit when designated essential fields cannot be preserved.

**Acceptance:** Explicit records/field counts and samples from original native application (where legally available), MPXJ, transfer wire, stage, readback and export. Every claim names checked dimensions and unsupported areas.

### P0-07 — Specify the old/new project schema's authority and versioned compatibility boundary

**Reference:** §§4.5, 11.2–11.6, 15.4.

The proposed schema is intentionally incomplete. It does not yet define full FKs, same-project boundary checks, immutable source vs editable state, migration truth, baseline protection, or how old chart editors are prevented from writing divergent schedule copies. It also introduces new table names without a reversible schema-version contract.

**Required revision:** A separate accepted storage ADR must include executable migration DDL plus sample fixture queries; authoritative schedule generation pointer; read-only legacy chart adapter; hard block on legacy mutators once a schedule is normalized; exactly specified new-file/minimum-reader/minimum-writer version fields; validated cross-platform encrypted migration/backup and admin escrow paths; and project-copy identity semantics. `PRAGMA foreign_keys=ON` must be verified **for each SQLite connection**. Compatibility with GoPMgr's pinned SQLCipher-backed SQLite version must be tested—do not assume new SQLite features are available.

**Acceptance:** Old files open unchanged; normalized files cannot be corrupted by older executables; rollback/forward repair is rehearsed; no editor can write two authoritative representations.

### P0-08 — Reconsider the initial plugin framework's scope and acquisition decision

**Reference:** §§0, 3, 5–6, G0–G2.

A full general-purpose dynamically installed plugin framework is not necessary to make one trusted bundled Java converter work. Early abstraction across hypothetical future plugin types can overwhelm the most important interoperability/data tasks.

**Required revision:** Use a narrow **bundled worker** MVP with an adapter interface and a versioned transport. Spike `hashicorp/go-plugin` versus custom supervisor with fixed time/evidence budget; its non-Go handshake is documented but compatibility **must be demonstrated**. Implement only launch/stop/health/cancel/metrics, digests/manifest validation, capability negotiation, and bounded streaming. Defer plugin marketplace, independent updates, generalized UI insertion, third-party SDK and broad registry until proven demand. Freeze transport schema only after a Go↔Java golden fixture and Windows/macOS/Linux packaged smoke.

**Acceptance:** A reviewer can describe the launch and trust chain in one page; disabling MPXJ does not affect core; no premature public plugin marketplace infrastructure.

## 2. High-value performance and data engineering improvements

### P1-01 — Physical schema/index plan, not just a table inventory

**Reference:** §§11.3–11.4, S1–S3.

Create a workload-specific schema benchmark plan including: stable surrogate IDs vs long composite keys; scoped unique constraints and partial indexes; adjacency (`parent_task_id`) plus separately measured materialized flattening for outline viewport; two directed relationship indexes; time-range series access; sparse typed custom fields; database statistics/`ANALYZE`; FTS5 optional encrypted full-text search. Benchmark normal rowid and `WITHOUT ROWID` only on suitable small composite rows; never assume one is globally faster. Record `EXPLAIN QUERY PLAN` and WAL/write amplification.

### P1-02 — Make query cursors robust against changing data and long-lived reads

**Reference:** §11.7.

A cursor `snapshot_revision` is a *logical* consistency token, not proof that an SQLite read snapshot stays pinned across separate Wails RPC requests. Long-lived WAL readers can prevent checkpoints and grow the WAL. For paged UI use short read transactions plus a stable revisioned order projection or re-query/restart on revision change; use keyset pagination over a stable final tie-breaker, and encode filter/sort/copy/revision expiry in an authenticated host cursor. Design count semantics (`exact`, `estimated`, `unknown`) and accessible stable scroll anchors.

### P1-03 — Optimize derived computations and cache invalidation by affected scope

**Reference:** §§11.7–11.9.

Define a change-impact graph for task edits, predecessor/successor paths, assigned resources, working calendars, WBS links, EVM and risk caches. Some CPM/date changes can propagate widely; prove equivalence to full recomputation before using incremental algorithms as authoritative. Maintain `(project_copy,schedule_generation,algorithm_version,calendar_revision,options_digest)` cache keys, bounded caches and explicit stale/refresh states. Queue expensive kernel/risk/portfolio computations as cancellable jobs with progress; never block a single-row save on a full Monte Carlo run.

### P1-04 — Control write amplification, WAL and large staging transactions

**Reference:** §11.5, §11.11.

SQLite WAL has one writer; avoid unbounded writer locks. Profile bulk import via reusable prepared statements, bounded batches, indexing strategy, encrypted temporary staging and candidate-generation build. Document WAL checkpoint policy and reader lifetime, disk headroom, `SQLITE_FULL` faults, cancel and restart. Compare staging-in-memory vs SQLCipher-encrypted stage based on actual footprint, avoiding plaintext spill and relying on measured reclaim/checkpoint behavior.

### P1-05 — Treat timephased records as a separate scalability regime

**Reference:** §8.2, §11.3 and T3.

Preserve original irregular intervals and original units. Index `(series_id,start,end)` and independent entity/series lookup. Use lazy interval aggregation at requested timeline granularity, cache short date ranges and compute compressed run/segment representations only if round-trip preservation is proven. Do not explode irregular segments into minute/day rows at import unless unavoidable and budgeted. Benchmark 1 million points and realistic sparse/dense source contours separately.

### P1-06 — Materialized derived Gantt/outline views with revision-aware changes

**Reference:** §11.7.

Separate `ListVisibleOutlineRows` from time-based `GetTimelineViewport`; use a `visible_outline_index` per schedule/revision where warranted, tested against deep/wide expansion, filter and user pinning. Return vertical row windows and horizontal date windows; preserve focus and selected task ID on sort, filter, folding, zoom and refresh. Use Canvas for dense bars only if benchmarked; maintain DOM/ARIA semantics for visible editable rows. Dependency routing should be clipped and selectively resolved, not globally rendered.

### P1-07 — Analytics query portability and per-project confidentiality

**Reference:** §§11.8–11.9.

First define *domain queries* and exact-money operations against indexed SQLCipher records. Route expensive scans through bounded streaming, optionally Arrow batches after benchmark; do not choose DataFusion/Iceberg/other engine in this ADR. Portfolio aggregation across multiple separately encrypted `.gopmgr` files requires authenticated per-project snapshot acquisition and explicit user selection, never a broad bypass of account isolation. Validate engine results against GoPMgr deterministic kernel and money calculations. Keep datasets' row count separate from API page cap and status.

### P1-08 — Audit domain-command lifecycle, compaction policy and forensic evidence

**Reference:** §11.10, §15.5.

Keep old event bytes and `eventHash` unchanged. Define new typed canonical delta formats in the old JSON envelope only for new schedule commands, with exact decimal/typed serialization, immutable snapshot anchors and referenced evidence. The existing canonicalizer decodes generic JSON; enforce large integer/decimal values as strings or validate number handling before treating event hashes as numerical provenance. Specify checkpoint frequency by both edits and evidence bytes, retention, backup/export, independent replay, and what 'undo' does **not** guarantee. Preserve old 256 receipt limit as distinct from permanent audit evidence. Avoid one full chart serialization for every single-task edit.

### P1-09 — Two-level performance objective rather than one ungrounded latency target

**Reference:** §11.11.

Publish a reproducible hardware baseline and both a **minimum supported** and a **recommended** hardware profile. Separate cold-open, file conversion, commit, first-viewport, scrolling p95, indexed search, individual edit, dependency recalculation, risk simulation, full export, snapshot/restore and worst-case RAM+scratch. Measure JVM and Go RSS together. Distinguish performance budgets (proposed) from demonstrated guarantees; do not claim T2/T3 support without named platform/fixture evidence.

## 3. Practical user-facing improvements worth incorporating

### UX-01 — Import assistant with quality preview and explicit modes (High)

Five steps: **Select file → Inspect → Map/validate → Review impact → Commit**. Show format and producer/version confidence, source file size, number of summary/leaf tasks, relationship/calendar/resource/assignment/baseline/timephased/custom-field coverage, estimated local disk/temp/heap requirements, and remaining limitations. Offer **Import as new project** by default; only show merge when supported. Present `Strict` and `Reviewed best-effort` modes, with preview-only available even when current project scale limits prevent commit. Preserve chosen mapping profile explicitly, not automatically across unrelated producers.

### UX-02 — Import field-fidelity inspector (High)

Provide an accessible searchable grid: source field/category, record affected, source representation, destination representation, transformation, severity and whether editable/exportable. Filters: `Needs attention`, `Unsupported`, `Preserved but read-only`, `Converted`, `Exact`. Let the user export a privacy-aware import report without automatically exposing notes, local paths or confidential field values. Tooltips explain duration units, work calendars and why imported source dates differ from GoPMgr recalculation.

### UX-03 — 2-way/3-way visual compare and selective merge (High, deferred)

Side-by-side comparison with grouped change counts (added, removed, moved, edited, conflicted), WBS/outline tree navigation, dependency impact preview and calendar/cost changes. Provide an identity-match confidence indicator; ambiguous identities require confirmation. Checkbox selection across visible/filtered changes must express whether it includes offscreen records. Add `Select safe changes only`, `Review high impact`, and immutable reviewed change-set summary. Require explicit approval of exact revision/digest. No name-based auto-merge.

### UX-04 — Large-project navigation tools (High)

Fast search by task ID/UID/WBS path/text; breadcrumb and jump-to-task; saved filters/views; collapsible outline with 'expand this branch' instead of indiscriminate expand-all; sticky task-name columns; Gantt minimap and zoom-to-selection; keyboard-first row editing; go-to-parent/predecessor/successor; clearly displayed loaded/total/estimated row counts; screen-reader row-position announcements.

### UX-05 — Plugin Center with honest status (Medium)

Only for **bundled modules** initially: status, version, supported file types *and read/write asymmetry*, enabled/disabled, locally installed runtime, offline availability, last self-test, disk footprint, release provenance, permissions and `Run diagnostics`. Actionable recovery such as 'MPXJ unavailable—existing XML importer remains usable'. No broad third-party install affordance before Tier 2 is approved. No false 'sandboxed' badges.

### UX-06 — Background job center and safe cancel (High)

Progress phases and meaningful counters (bytes received, tasks validated, assignments staged, commit/publish state), cancel where safe, and clear boundaries after which a user-visible cancel may not undo an already durable commit. Preserve an error/recovery receipt and offer `View details`/`Retry inspection` only where idempotent; no misleading '100%' during long commit/validation. Distinguish running from queued and memory/disk quota rejection.

### UX-07 — Imported vs recalculated schedule comparison (High)

Toggle source values vs GoPMgr-computed values, calendar exceptions, time zones, critical path differences, and an explicit 'Recalculate using GoPMgr' action with before/after preview. Do not silently replace source dates or baselines. Show a trace/explanation for high-impact discrepancies.

### UX-08 — Safe change history and undo/restore semantics (Medium)

Per-task human-readable change timeline, actor/time/source, field-level differences, baseline protection, reversal proposals where supported; clear distinction between `Undo uncommitted edit`, `Revert committed change` (new audited transaction), `Restore backup`, and `Reimport original`. Avoid promising unlimited undo from a compact audit chain alone.

### UX-09 — Local AI readiness and privacy controls (Future)

An AI capability page should show model/provider location (on-device, LAN-local or remote), exactly what project fields are exposed, current grants, ability to revoke, estimated context size, and approval mode. Default read-only and disabled. Include 'Explain analysis', source links, calculated versus model-generated indicators, and 'No project edits without approval'. Offer no AI setup prerequisite for core usage.

## 4. Proposed incremental amendment order

| Patch | Planned additions | Depends on | Blocks |
|---|---|---|---|
| A0 | Fix document numbering (`12` then repeated `8` in §0); reference/decision inventory, explicit unverified flags | None | Document acceptance |
| A1 | Logical lineage vs artifact identity, safe import-match contract | Current §8 and fixtures | S4/S6/G3 production import |
| A2 | Crash recovery/publish intent journal, multi-file boundaries, authoritative schedule generation | S1 proposal and existing DB lifecycle | S2/G3 activation |
| A3 | Typed protocol schema including complete envelope, error details and MPXJ parse RSS/scratch test | G0 proof | G1/G2 |
| A4 | Strict import fidelity modes and golden field matrix per format | Source and MPXJ fixture corpus | G3 full-fidelity claims |
| A5 | Authenticated local MCP connection/grant design | New AI ADR, not G1 | G5 |
| A6 | Physical schema/indices, keyset pagination, snapshot expiry, WAL/FTS benchmark plan | S1/S2 | S5 scale claim |
| A7 | Incremental computation semantics and per-domain cache keys | Kernel contract/invalidation tests | S7 analytics/large edit claim |
| A8 | Audit semantics and replay/retention design separated from analytics | Separate audit review | compact audit production |
| A9 | Import wizard, fidelity inspector, job center, large project navigation, compare UX | G2/S3/S5 as appropriate | G3/G4 public UX |

**Restructure S7** into **S7A — analytics/large tabular ingest** and **S7B — compact audit evidence/replay**, because they are unrelated high-risk changes and should not form a single hard release gate. Likewise distinguish **plugin worker process MVP** from **future third-party plugin platform**. Preserve current S0–S6 names and original G phases to minimize documentation churn.

## 5. Required new regression/assurance cases

| ID | Scenario | Deterministic acceptance |
|---|---|---|
| REVIEW-001 | Reimport same logical project after one source byte changes | Stable UID lineage retained, no duplicate entities |
| REVIEW-002 | Clone project with identical UID space | Unsafe merge prohibited, user must decide lineage |
| REVIEW-003 | Process crash immediately after staged DB commit/before project publication | App restart reconciles intent, publishes exactly once or quarantines |
| REVIEW-004 | Crash after rename but before catalog registration | Recovery finds verified committed project without duplicate |
| REVIEW-005 | T2 MPP Java import with seekable scratch | JVM heap + host RSS + scratch measured, resource cap enforced |
| REVIEW-006 | Timephased actual/cost data in JSON/MSPDI export | Required flags enabled and exact slices represented or reported |
| REVIEW-007 | MCP connect during logout/project switch | Old client loses project access immediately; never inherits new session |
| REVIEW-008 | Cursor across interleaved writes | No silent duplicate/skip; restart required or stable version-backed view |
| REVIEW-009 | Slow paged reader and active imports | WAL checkpoint progress monitored, no unbounded WAL growth |
| REVIEW-010 | Real 50k schedule per-field edit | Bounded write count, no giant chart JSON, correct affected cache invalidation |
| REVIEW-011 | Old executable opens new schema project | Explicit safe refusal/read-only without mutation |
| REVIEW-012 | Existing audit and new typed numeric delta | Old chain hashes unchanged; exact-money/large int semantics valid |
| REVIEW-013 | 2-way/3-way comparison with filter and offscreen selections | Selection cardinality visible; no unseen rows silently included |
| REVIEW-014 | Screen reader and keyboard on 50k tree | Focus, row position and navigation remain deterministic |
| REVIEW-015 | Project source fields not understood by MPXJ | Unsupported-by-reader claimed honestly, no fictitious complete coverage |
| REVIEW-016 | Resource exhaustion mid-import/restore | Existing project unchanged; staged files cleaned or explicitly recoverable |
| REVIEW-017 | Plugin unavailable, disabled or incompatibly upgraded | Existing XML/core editing remains fully usable |
| REVIEW-018 | User cancels at every job transition | No ambiguous final state; committed receipt visible after late cancel |
| REVIEW-019 | Portfolio query across encrypted user projects | Only approved account/project snapshots used; no stale-user leakage |
| REVIEW-020 | Gantt compares imported dates vs native recompute | Preserves source baseline, explains calendar/constraint discrepancies |

All test IDs need real command/fixture/hardware logs; their presence in a document does **not** establish a PASS.

## 6. Documentation and governance recommendations

1. Keep ADR-010 a concise **architecture decision** (~decisions, alternatives, consequences, constraints, approval status). Move execution details into one versioned `docs/plans/` implementation plan; field catalog into `docs/interoperability/`; physical schema and migration into a **separate** ADR; MCP authorization into its future ADR. A single 24,000-word ADR is hard to maintain as the application changes.
2. Add a dependency graph and decision register that differentiates `Proposed`, `Accepted`, `Implemented`, `Verified`, `Blocked` and `Superseded`. Never use an assumption to satisfy an approval gate.
3. Each gated pull request must include: current HEAD, affected contracts, invariants preserved, source/fixtures, tests run/blocked, performance delta, license/SBOM effects, rollback procedure and exact user-visible functionality changes.
4. Do not transform this addendum into permission for an agent to implement all gates at once. Start with a small G0/S0 evidence-only pull request, owner-approved S1 physical-schema proposal, and cross-language MPXJ proof.

## 7. Authoritative references for the coding agent

- GoPMgr repository: <https://github.com/draconianneocoder-cpu/GoPMgr>
- MPXJ supported formats: <https://www.mpxj.org/supported-formats/>
- MPXJ changelog (including format-specific correctness and security fixes): <https://www.mpxj.org/CHANGELOG/>
- HashiCorp `go-plugin` non-Go guide: <https://github.com/hashicorp/go-plugin/blob/main/docs/guide-plugin-write-non-go.md>
- SQLite WAL and reader/writer/checkpoint behavior: <https://www.sqlite.org/wal.html>
- SQLite foreign-key configuration: <https://www.sqlite.org/foreignkeys.html>
- SQLite WITHOUT ROWID tradeoffs: <https://www.sqlite.org/withoutrowid.html>
- Official MCP Go SDK: <https://go.sdk.modelcontextprotocol.io/>
- MCP Streamable HTTP transport security: <https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/transports/streamable-http.mdx>

**Provenance note:** These are suggested fixes based on inspected architecture text and targeted source checks, not statements that implementation performance or interoperability tests passed. There were no repository mutations, application builds or tests as part of this review.
