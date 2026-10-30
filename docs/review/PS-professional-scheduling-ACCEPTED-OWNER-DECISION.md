<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Professional scheduling and project controls: maintainer decision

**Status:** APPROVED by the maintainer, 2026-10-26, as the strategic direction
and priority order. This approval is **not** evidence that anything in it is
implemented or tested.
**Canonical plan:** [professional-scheduling-modernization-plan.md](../design/professional-scheduling-modernization-plan.md)

## Receipt

- Received as `GoPMgr_Professional_Scheduling_Plan.txt`: 24,072 bytes,
  SHA-256 `685558d52119eec2bfb421519bd23aaa024a5fceda22e1a0c8786933b061b685`.
- It is reproduced below exactly as received.

### Gaps in the received text

Two parts of the received file have no content. They were probably tables
lost in export. Nothing has been filled in for them; both are noted as open
items in the plan.

- **AP-01** says "Each domain owns its authoritative records:" and the list
  that should follow is empty.
- **§6, "Risks and Mitigations"**, has a heading and no body.

## Decision text (verbatim)

```text
GoPMgr Professional Scheduling and Project Controls Modernization Plan
Status: MAINTAINER-APPROVED STRATEGIC DIRECTION Date: October 26, 2026 Document version: 1.0 Project: GoPMgr Strategic benchmark: ProjectLibre Desktop and established professional project-scheduling practices
1. Formal Maintainer Approval
As GoPMgr's project owner and maintainer, I formally approve the strategic direction and prioritized roadmap defined in this document.
My objective is to develop GoPMgr into a professional, local-first project-management application that exceeds ProjectLibre Desktop in overall quality, scheduling capabilities, project controls, security, documentation, interoperability, usability, and analytical functionality.
I specifically approve a connected project-management architecture in which scheduling, WBS, resources, assignments, costs, baselines, reporting, risk analysis and other professional workflows operate over consistent, authoritative information.
This approval establishes the project's strategic direction and prioritization. It does not automatically approve unreviewed database migrations, dependency changes, unsupported format claims, security exceptions or production releases.
Existing approvals remain effective within their stated scope, including ADR-010 v2.2, the G1/P04 decisions, the unified WBS design decisions and ADR-012 v0.2 with its authorized DF0 feasibility work.
2. Architectural Principles
AP-01 — Connected information, separate authorities
Use a cohesive logical project model with explicit domain ownership.
A connected project model does not require all domains to share one mutable aggregate or physical table.
Each domain owns its authoritative records:

Connect domains through stable, validated identifiers and explicit relationship records.
Names, row numbers, WBS outline codes and visual positions must never substitute for stable entity identities.
AP-02 — Preserve existing GoPMgr architecture
Maintain:
Go as the authoritative application and scheduling environment.
SQLCipher as encrypted project storage.
Wails/Svelte as the native desktop UI.
Native, authenticated gRPC local IPC for approved worker processes.
MPXJ integration for supported external project file parsing.
Apache DataFusion/Arrow as the independently approved analytical replacement direction, subject to ADR-012 feasibility gates.
Workers must not directly control project databases, authentication keys, project approval decisions or authoritative audit records.
AP-03 — One calculation authority
Each scheduling and financial result must have one authoritative computation policy.
Do not independently calculate competing dates, resource totals, costs or completion percentages in multiple UI components or workers.
Derived views must identify the source revision and calculation version used.
Source-imported values and GoPMgr-calculated values must remain distinguishable wherever they could differ.
AP-04 — Reliability and fidelity outrank feature count
No feature is considered complete merely because its interface renders.
A feature must demonstrate correct calculations, persistence, revision handling, cancellation, accessibility, testing, documentation, interoperability and recovery appropriate to its risk.
Do not sacrifice existing functionality to claim parity with a competing application.
AP-05 — Local-first operation
All core scheduling, resource analysis, project controls, documents, imports, exports and analytical operations must operate offline.
External services, AI services, collaboration servers or distributed infrastructure must not become mandatory dependencies.
3. Prioritized Work Packages
PS-00 — Establish the verified implementation baseline
Priority: Immediate
Before implementing major new functionality:
Reconcile current main, open PRs, accepted architecture records and existing release gates.
Complete the S0 measurement and test-fixture inventory.
Document existing ProjectLibre Desktop capabilities against verified GoPMgr functionality.
Record which features exist, are partially implemented, are experimental, or are only proposed.
Select owner-approved minimum and recommended hardware profiles.
Develop realistic schedule fixtures containing complex calendars, dependencies, resources, costs, baselines, constraints and timephased records.
Identify existing defects that should be corrected independently of modernization.
Exit gate: Approved source-of-truth inventory, dependency graph, representative fixtures, platform matrix and proposed numerical performance budgets.
Do not classify an unmerged prototype as a released capability.
PS-01 — Establish the normalized connected project model
Priority: Critical — foundational
Coordinate with ADR-011/S1–S4 rather than creating a competing schema architecture.
Design stable, typed, normalized entities for:
Activities and source schedule hierarchy.
WBS scope and work-package associations.
Dependencies, constraints and scheduling calendars.
Resources, resource groups and capacity calendars.
Assignments, work quantities and timephased segments.
Cost planning associations and approved financial ledger references.
Schedule, scope and financial baselines.
Progress, actual dates and source observations.
Provenance, custom fields and imported evidence.
The schedule outline and governance WBS remain distinct unless a specifically accepted mapping defines their relationship.
Existing WBS v2 records and their approved size and revision constraints must be preserved. Increasing the 10,000-node WBS limit requires its own tested migration and performance decision; the 50,000-task scheduling objective does not silently remove that limit.
Required implementation properties:
Stable IDs across legitimate edits.
Revision-checked commands.
Normalized transactional writes.
Typed, paged read APIs.
Deterministic ordering.
Atomic updates of related records.
No duplicate editable financial authority.
Encrypted schema migration and backup/restore compatibility.
Strict older-writer protection.
Explicit legacy-project conversion and failure recovery.
Exit gate: ADR-011 approval, schema-version gate, migration recovery evidence, referential integrity tests and an independently reviewed transactional command/query contract.
PS-02 — Implement schedule-quality analysis early
Priority: High — early parallel track
Develop a read-only quality-assurance module using the existing Go scheduling kernel.
The initial audit should examine:
Missing or dangling predecessor/successor relationships.
Unexpected open ends.
Dependency cycles.
Excessively long or negative lags.
High total float.
Excessively long durations.
Hard constraints and their effect on schedule logic.
Milestones lacking meaningful logic.
Invalid or conflicting actual dates.
Calendar or assignment inconsistencies.
Resource overallocations.
Missing or inconsistent progress evidence.
Baseline and reporting-date availability.
Include configurable, named rule profiles. A DCMA-style profile may be provided with properly documented rule definitions and thresholds; do not claim official DCMA certification or treat all rule thresholds as universal.
Each finding must identify the affected record, rule, severity, source revision, explanation and possible correction.
Auditing must remain non-destructive. Suggested repairs require preview and explicit approval.
Provide historical audit-result comparison when normalized persistence is ready; the initial implementation can operate on a bounded, read-only schedule snapshot.
Exit gate: Independent rule oracles, deliberately defective fixtures, false-positive review, deterministic reruns and no modifications to authoritative schedules.
PS-03 — Complete professional file inspection and interoperability
Priority: Critical
Continue ADR-010 G1/G2/G3.
Prioritize native project-file inspection and eventual high-fidelity import for the formats supported by the chosen, verified MPXJ version.
Candidate formats include Microsoft Project MPP/MSPDI and ProjectLibre POD, followed by supported Primavera formats and other useful project-management interchange formats.
For every claimed format and version, independently verify its actual reader and writer support.
Preserve or explicitly disclose:
Stable source identifiers.
Full activity and summary hierarchy.
Calendars and working-time exceptions.
Dependency types, lags and leads.
Constraints, actual dates and progress.
Resources, assignments and availability.
Rates, budgets and costs.
Baselines and timephased records.
Custom fields and source-specific metadata.
Unknown fields and source evidence, where technically possible.
An imported file must not be described as complete if important source fields were omitted without disclosure.
Proceed in stages: inspect, map, review, import as a new project, validate readback, and only later introduce selective merge under S6.
Exit gate: Per-format and per-version fixture matrix, field-fidelity report, signed/offline packaging evidence, safe publication and complete recovery tests.
PS-04 — Correct large-schedule storage, audit and Gantt bottlenecks
Priority: Critical
Use the S0 results as the benchmark baseline.
Implement:
Scoped transactional edits rather than rewriting entire schedules.
Compact, verifiable audit events while preserving historical audit-chain compatibility.
Indexed resource/time-window queries without repeated full-schedule rescans.
Revision-bound paged task, dependency and resource APIs.
Virtualized Gantt rendering and bounded Wails payloads.
Efficient affected-row recalculation and cache invalidation.
Cancellation and bounded resource consumption.
Representative tests at 500, 10,000, 50,000 and 100,000 tasks.
Preserve correctness for calendars, splitting, lags, constraints, cycle detection and all baseline calculations.
Do not introduce incremental scheduling algorithms as authoritative until differential testing establishes equivalence with a verified full recomputation.
Exit gate: Approved T2 usability budgets for genuinely editable 50,000-task schedules, plus documented 100,000-task stress behavior. All relevant functionality must remain available.
PS-05 — Deliver the connected professional scheduling workspace
Priority: High — after the transactional query foundation
Develop one cohesive scheduling workspace with synchronized, task-centered navigation.
Core views:
Configurable task outline/table.
Gantt timeline.
WBS scope hierarchy.
Dependency/network diagram.
Resource assignment summary.
Task details.
Baseline/variance inspection.
Schedule-quality findings.
The user must be able to select a task in one view and navigate to its corresponding representation in another without losing selection, filter state or edit context.
Support:
Saved column sets and views.
Sorting, filtering and grouping.
Critical path and float highlighting.
Summary expand/collapse.
Dependency navigation.
Keyboard-first data entry.
Multirow operations with atomic previews.
Revision-aware undo/redo where applicable.
Clear distinction between unsaved edits and committed changes.
Appropriate tooltips, focus behavior and accessible alternatives to diagrams.
Use existing GoPMgr visual design conventions rather than copying another application's interface.
Exit gate: Identical underlying task identities and values across all connected views; no duplicate storage; accessibility, native desktop and large-project performance tests passing.
PS-06 — Implement professional Task Usage and Resource Usage workspaces
Priority: High
Add synchronized, time-scaled task and resource work views.
Use existing Go resource calculations as a foundation, extending them with normalized assignments and sparse timephased work segments.
Capabilities should include:
Task-by-resource and resource-by-task usage.
Work, remaining work, assigned units and availability.
Calendar-aware timephased allocations.
Overallocated and underutilized resource visibility.
Capacity thresholds.
Hour/day/week/month aggregation.
Assignment details and bounded editing.
Work contour representation when supported.
Previewable resource adjustments and leveling effects.
Initially deliver authoritative read-only usage views, then add controlled editing after revision, calendar and work-effort semantics are verified.
Do not treat a plotted allocation as an authoritative new assignment merely because it appears in a histogram.
Exit gate: Usage totals reconcile with assignments and calendar rules; editing produces deterministic schedule effects; large timephased datasets remain bounded.
PS-07 — Expand resource and financial planning
Priority: High, after PS-01/PS-06 contracts
Introduce professional resource-cost modeling using:
Standard and overtime rates.
Effective-dated rates.
Cost-per-use charges.
Labor and nonlabor resource categories.
Resource groups and optional RBS.
Planned and remaining work.
Cost projections by activity, resource and project.
Controlled links to Cost Control records.
Maintain the distinction between planned resource cost, committed expenditure, actual cost, approved funding and earned-value calculations.
No calculated forecast becomes a posted actual expense or approved commitment without the applicable explicit financial transaction.
Define effort-driven, fixed-work, fixed-units and fixed-duration policies separately; do not silently implement Microsoft Project terminology without matching and verifying its semantics.
Exit gate: Checked monetary arithmetic, currency and precision tests, effective-date boundary tests, reconciliation reports and independent financial approval-state verification.
PS-08 — Extend advanced professional scheduling
Priority: Medium — correctness-dependent
Evaluate and implement, where justified:
Finish-anchored project planning and backward scheduling.
Additional formally specified scheduling constraints.
Manual task splitting and resumption.
Effort-driven duration/work calculations.
Recurring activity templates.
Calendar exceptions and mixed calendars.
Multiple scheduling strategies and controlled resource-leveling previews.
Reliable baseline comparison across supported schedules.
Any feature that changes scheduling semantics requires independent reference tests, especially for partial completion, lead/lag handling, nonworking days, overtime, daylight-saving transitions and constrained critical paths.
Do not infer that an existing CPM backward pass alone establishes complete finish-anchored project scheduling behavior.
Exit gate: Independent scheduling reference corpus, differential tests, backwards compatibility, and no hidden recalculation or irreversible task changes.
PS-09 — Complete analysis, comparison and professional reporting
Priority: High, in parallel where dependencies permit
Coordinate with:
ADR-012/DataFusion replacement.
ADR-010 S6 project comparison and merge.
S7A analytics expansion.
S7B compact audit and replay.
Deliver revision-consistent schedule and resource analysis, project comparison, cost/resource reporting, forecast and risk views, and accessible, repeatable exports.
Connect reporting to authoritative domain services, not independently recomputed UI totals.
Schedule-quality findings should be usable in risk analysis, but must not silently alter statistical assumptions or become unreviewed financial projections.
DataFusion remains an analytical engine, not the scheduling authority.
Exit gate: Financial and scheduling parity, reproducible analysis, auditable source revisions, secure export behavior and full release regression coverage.
PS-10 — Advanced local-first capabilities
Priority: Later, separately approved
Candidate enhancements:
RBS and organizational breakdown views.
Shared local resource pools across projects.
Cross-project resource-capacity previews.
Time-entry and work-approval workflows.
Additional localization.
Controlled AI-assisted analysis.
Optional offline collaboration and conflict-aware sharing.
Do not make real-time cloud collaboration or mandatory enterprise services prerequisites for professional single-user functionality.
Cross-project resource management must first prove identity, locking, ownership, concurrency and reconciliation behavior across separate encrypted project files.
4. Delivery Dependencies
The modernization program must respect the existing ADR-010 dependency graph.
The following streams may progress in parallel when their own gates permit:
Foundation stream: S0 → ADR-011/S1 → S2 storage gates → S3 transactional APIs → S4 mapping and readback.
Interop stream: G0 → G1 registry/supervisor → G2 format inspection → G3 new-project import → G4 packaging/export readiness.
Professional workspace stream: Early read-only schedule audit → S3-backed connected views → S5 virtualization → timephased resource workflows → extended scheduling and financial integration.
Analytics stream: ADR-012 DF0 → separately approved implementation → DuckDB parity/cutover → expanded S7A analytical capabilities.
No team or agent should introduce a second competing schema, plugin framework, scheduling engine or audit authority to accelerate its own workstream.
5. Required Acceptance and Quality Gates
QG-01 — Scheduling correctness
Preserve and test the authoritative Go scheduling model.
Cover dependencies, critical path, float, constraints, leads/lags, all supported calendars, leveling, splitting, baselines, milestones and actual-versus-planned dates.
Independent reference fixtures must supplement existing GoPMgr regression tests.
QG-02 — Data and identity consistency
All connected views must reference the correct authoritative objects and revisions.
Test rename, move, reparent, delete, duplicate IDs, import remapping, concurrent edits and stale handles.
A view must never silently save stale data over newer changes.
QG-03 — Import fidelity and recovery
Every supported input format needs documented field coverage, omission reporting, typed conversions, malformed-input handling, source-provenance validation and encrypted readback.
No partially published project may appear as successfully imported.
QG-04 — Scalability
T0: 100–500 tasks, representative ordinary use.
T1: 10,000 tasks, substantial project use.
T2: 50,000 tasks, mandatory usable/editable product acceptance.
T3: 100,000 or more tasks, defined stress behavior.
Measure p50/p95 user-observed latency, peak memory, allocation growth, query/IPC payload size, edit duration, audit growth, first paint and viewport navigation.
Use owner-approved hardware and numerical targets established from S0, not invented thresholds.
QG-05 — Security and audit
Preserve SQLCipher confidentiality, project authorization, audit integrity, credential protection, process isolation requirements, file-version restrictions, and offline operation.
Test abnormal termination, interruption recovery, malicious inputs and legacy encrypted database handling.
QG-06 — User experience and accessibility
Test realistic professional workflows rather than isolated controls alone.
Require keyboard navigation, screen-reader semantics, stable focus, predictable undo/discard, visible error states, scalable rendering and responsive native behavior.
Include hands-on comparative evaluations using a documented ProjectLibre Desktop version and representative workflows.
QG-07 — Legal and distribution quality
Preserve dependency-license reviews, third-party notices, SBOM expectations, reproducible package inventories, signature verification and platform-specific installation tests.
The TPC-H/TPC-DS exclusion decision remains binding. Follow the corrected technical evidence in PR #146; distinguish bundled archives from executable object code actually linked into distributed artifacts.
QG-08 — Adversarial completion review
Before completion of each work package:
Identify requirements that remain unimplemented.
Search for independent correctness and security failures.
Run negative and boundary tests.
Inspect all changed files and unexpected dependency modifications.
Verify documentation against actual behavior.
Record blocked tests and unresolved assumptions.
Compare measured performance against the baseline.
Confirm all existing architectural invariants remain satisfied.
A passing unit suite does not imply that the complete feature or release gate has passed.
6. Risks and Mitigations

7. Implementation Governance
For every work package, produce a small-slice plan containing:
Current repository commit and dependency baseline.
Exact feature requirements.
Domain ownership and affected interfaces.
Proposed schema/API changes.
Security and data-loss threat review.
Test identifiers and independent oracles.
Performance budgets and benchmark methodology.
Compatibility and migration effects.
Rollback/recovery procedure.
Release and documentation impacts.
Explicit out-of-scope items.
Create narrowly scoped pull requests with passing applicable gates before dependent work is merged.
Never use a large, combined PR to conceal failures or bypass review.
Where an existing accepted ADR differs from this roadmap, produce an Architecture Change Request and obtain my approval. This roadmap does not silently override ADR-010, ADR-011's reserved decisions, ADR-012 or earlier WBS requirements.
8. Maintainer Authorization
APPROVED NOW
I authorize:
Recording this strategic direction in the canonical roadmap and a traceable maintainer decision record.
Completing the capability and architecture gap analysis.
Preparing the detailed design and implementation slices for the work packages.
Continuing already authorized ADR-010 and ADR-012 work.
Developing independent fixtures, reference oracles, benchmarks and isolated feasibility prototypes.
Proposing the early schedule-quality auditing work as a separately reviewable, read-only implementation slice.
SEPARATE APPROVAL REQUIRED
Production schema activation, project-format migrations, financial authority changes, new mandatory dependencies, significant scheduling-semantic changes, and release-support or signing-policy changes require their applicable accepted contracts and owner authorization.
Approval of this roadmap is not evidence that these changes have been implemented or tested.
9. Required Agent Deliverables
The implementation agent shall:
Update docs/ROADMAP.md with the accepted professional scheduling direction.
Create a canonical modernization plan and maintainer decision record.
Build a requirements-to-capabilities matrix distinguishing implemented, partial, proposed, blocked and unverified features.
Map each work package to existing ADR-010/S1–S7 and ADR-012 activities.
Produce the normalized project-domain dependency diagram and identify open ADR-011 decisions.
Prepare the schedule-quality audit specification and isolated implementation plan.
Identify the next highest-value authorized implementation slice, including test plan and risk assessment.
Verify document links, architectural consistency and acceptance traceability.
Preserve historical design and acceptance documents.
Report precisely what was changed, tested, blocked and deferred.
10. Final Maintainer Directive
I approve the goal of making GoPMgr superior to ProjectLibre Desktop as a professional, local-first project-management application.
The central product strategy is one connected project-management experience built upon independently governed, consistent scheduling, scope, resource, financial and audit records.
Professional scheduling correctness, interoperability, usability, security and reliability take priority over superficial feature-count parity.
Agents must prioritize stable foundational engineering, verified calculations, real-world workflows, performance, fidelity and maintainability.
Do not compromise architectural quality to accelerate delivery. Do not misrepresent proposed capabilities as implemented.
Proceed with the approved strategic and design work, preserve existing phase-specific authorization boundaries, and submit the first independently reviewable implementation proposal with its acceptance evidence.
```
