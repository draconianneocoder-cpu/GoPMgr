<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-014 Reporting integrity and lifecycle documentation

**Status:** Proposed detailed architecture; documentation integrated 2026-11-13.
**Workstream:** RILD-01. The supplied plan records owner approval of workstream
establishment. That direction is retained separately from approval of this detailed
ADR, persistent schema, policies or any implementation slice. The current user
request authorizes this reconciliation and repository documentation only.
**Source baseline reviewed:** `ac20cbe711d9c03479cf937d82c5b7da991454d3`.

## Authority and document map

AGENTS.md, accepted ADRs and explicit owner decisions control. This ADR supplements
PS-09; it does not replace ADR-010/011, reopen DataFusion, change the file format,
turn an export into an approved record, or authorize production implementation.
The original RILD-01 DOCX is retained byte-for-byte in
[the source archive](reporting-extension/sources/GoPMgr_Reporting_Integrity_Architecture_Plan.docx).
The historical `09afe43e` source claims are distinct from the integration baseline.

- [Execution sequence and decision gates](reporting-extension/implementation-plan.md).
- [Versioned requirement register](reporting-extension/requirements.json), preserving
  all 120 explicit source IDs, eight expanded catalog families and architecture contracts.
- [Metric definitions and acceptance strategy](../testing/reporting-extension-assurance.md).
- [Secure extension proposal](ADR-015-secure-extension-platform.md), an optional
  execution boundary rather than a prerequisite for host-native reports.
- [Integration findings and provenance](../review/reporting-extension-integration.md).

## Verified foundations and gaps

At the reviewed baseline, Go-native analytics, the document registry, export formats,
report quality preflight, project audit and versioned document rows exist. The
financial report already reads its inputs in one transaction
(`internal/desktop/app_financial_report.go`). Combined reporting preflights and
resolves in separate reads and deliberately retains its PDF if manifest creation
fails (`internal/reporting/reporting.go`). Neither implies the immutable issuance
and recovery protocol specified below. Extend the existing transaction pattern.

`internal/db/documents.go` versions the current row and records audit/approval
operations; it does not establish immutable approved revisions or an issued-byte
archive. Status fields can be authored rather than calculated; burndown accepts
arrays rather than a complete work-event history. Preserve these useful capabilities
and label their evidence correctly. Do not fabricate historical events.

PS-02 rules and `App.AuditSchedule` are implemented; the findings panel remains open.
Both KRN-01 and KRN-02 affect the truth of dependent schedule, EVM, baseline and
resource claims. Their remedies require their separately authorized kernel slices;
reporting and read-only audit work cannot change scheduling semantics incidentally.

## Integration invariants

Preserve imported source observations, representations and hashes. Calculate host-
derived values separately under approved domain contracts; never overwrite imported
facts to agree with a host calculation. Keep fractional-JPY compatibility, exact
money, unavailable versus zero, encrypted persistence, existing audit bytes and
PDF render → PDF/A metadata → final PAdES mutation order. An unqualified capability
cannot become qualified through a new renderer or an extension.

Use **RILD evidence E0–E4** only for the reporting evidence ladder. These are unrelated
to **EXT profile E0–E3**, which classify extension execution trust. Always include
the namespace. No engine, template language, SDK, online registry or new security
mechanism is adopted here. Core reports work offline without executable extensions.

The following target contracts retain the substantive source design, with the
baseline and authority corrections above integrated. Target requirements are not
claims that a component is implemented or qualified.

## RILD-ARCH01 System architecture and boundaries

Use a modular monolith for core operations. Preserve pure kernel and domain logic, SQLCipher transaction authority, Go-native analytics and the existing Wails/Svelte boundary. A worker is optional for a bounded heavy/unsafe task only after a separate ADR; no mandatory service or network.

| Boundary | Contract / existing anchor |
| --- | --- |
| Project repositories | SQLCipher per-project; normalized schedule design under ADR-011; no report writer may mutate source facts. |
| Snapshot builder | Read-only repeatable snapshot, pinned baseline(s), source revision vector and policy versions; typed immutable input. Extend internal/reporting. |
| Domain calculators | internal/kernel and internal/analytics plus domain-specific services; pure typed functions with explicit units, nullability, applicability and errors. |
| Report compiler | Resolve versioned definition, filters, audience and calculations into typed ReportIR; no SQL/HTML supplied by untrusted templates. |
| Validation & policy | Data, schedule, reconciliation, completeness, permissions, freshness and render preflight; findings with stable IDs and source links. |
| Presentation adapters | Existing internal/documents and internal/export; render validated IR; accessible HTML/UI; PDF/A, DOCX, ODT, XLSX, CSV/Parquet where appropriate. |
| Publication and evidence | Private staged package, atomic commit or recoverable journal, content digest, approval and audit; verify and retrieve immutable issue. |

### Direction of dependencies

UI → report application service → snapshot/query ports → authoritative repositories; service → domain calculators → typed IR → renderer; publisher → artifact store/audit. Kernel and calculators must not depend on DB, UI, PDF, clock or filesystem. Dependency inversion is justified only at I/O and test seams.

Never persist derived chart values as a competing source. A cache is keyed by complete input revision and policy identity, is disposable and rebuildable, and cannot survive a failed verification as authoritative.

## RILD-ARCH02 Authoritative domain model

Stable IDs are project-scoped; use a portfolio registry to map shared identities without inferring equality from names. All records carry source, author/import identity, recorded timestamp, applicable effective time, revision and deletion/archive policy. Domain-specific schema decisions follow DEC-FORMAT and DEC-AUDIT.

| Authority / entity group | Required relationships and invariants |
| --- | --- |
| Project / program / portfolio | Objectives, sponsor, phase, membership, strategic contribution, reporting calendar; version membership and coverage. |
| Schedule / WBS | Activities, parent work packages, typed dependencies, calendars, constraints, milestone, assignments, progress events, baseline and scenario revisions. No disconnected duplicate charts as authority. |
| Cost / time / contracts | Ledger entries, classification, reserves, approvals; time-entry ID and assignment; rate effective periods; invoice/credit allocation links; ledger reconciliation. |
| Delivery / quality | Deliverable, requirement, acceptance criterion, inspection/test evidence, acceptance decision, defects, handover, benefits measures. |
| Governance | Risk, issue, assumption, dependency, action, decision, change; owners, dates, disposition and evidence. Typed link table; preserve referential integrity. |
| Agile | Work item, sprint, state transition, estimate and scope event, deployment event; state mapping versions. |
| Documents / reports | Document identity, immutable revision, versioned definition, workflow events, approval, attachment digest, issued artifact, supersession and archive reason. |

### Migration invariants

Map existing chart JSON/document rows to stable identities with loss inventory and source hashes. Retain original source representation for unsupported fields. Reconcile counts, relationships, money and dates before enabling writes; never auto-approve migrated content. Use read-only legacy reader or explicit loss disclosure; do not permit older writers to mutate new-format files. No destructive migration without backup/restore and owner gate.

## RILD-ARCH03 Snapshot, lineage and temporal semantics

Define ReportRequest {definitionID/version, project selection, periodStart, cutoff, baselineIDs, filters, audiencePolicy, locale, timezone, units, requestedFormats}. Reporting cutoff is business-effective time; capturedAt is transaction observation time; generatedAt and issuedAt are distinct. Capture once; render repeatedly without rereading mutable records.

### Single-project and portfolio consistency

Use one read transaction or a durable immutable revision set for each project. Freeze all source references, including linked chart configurations, cost entries and approvals; validate that the resolved snapshot is the same data used by calculations and renderers. For multiple encrypted files, capture a revision vector with per-project observation time, cutoff, completeness and failure state. Do not claim atomic cross-file snapshots. Reject strict synchronized mode if any project changes during capture; permit explicitly labeled best-effort only by policy.

### Temporal and correction rules

Store UTC instants plus original local date/time and time-zone/calendar ID where semantics require. Distinguish effective work date from recorded time; support late-entered corrections as new events with links, never retroactively rewrite issued reports. As-of query must select revisions effective at cutoff and recorded by the selected knowledge time. A report restatement uses a new issue ID with reason and delta.

| Required lineage | Validation |
| --- | --- |
| Source file / import ID / mapping version | Digest original, mapped counts, rejected/unsupported fields, source IDs and import time. |
| Source revision and policy versions | Record source type, ID, immutable version/digest, baseline, currency, rates, calendars, rule definitions. |
| Coverage counters | Expected, selected, accepted, excluded, rejected and unknown; reason per exclusion. Reconcile counts and totals. |
| Provenance graph | Report section → metric → snapshot record IDs; stable audit/evidence link and permission check. |
| Reproduction | Retain original issued bytes; regenerate semantically with exact inputs, tool version and deterministic settings where feasible. |

No row cap, pagination or query error may silently reduce totals. Partial data is visibly labeled and cannot satisfy a complete-report gate.

## RILD-ARCH04 Controlled lifecycle documentation

Separate authored records from generated reports. An authored charter, requirement or meeting minute records human decisions; a generated register summarizes those records. A hybrid report has immutable computed sections and separately attributable narrative. A source document cannot be changed by editing a generated report.

| State / transition | Required behavior |
| --- | --- |
| Draft → submitted | Save draft revisions; submission freezes a content hash, source references and reviewer assignment. |
| Submitted → reviewed | Record review outcome, comments, reviewer identity, time and material findings; rejection creates correction task. |
| Reviewed → approved | Check policy, authority and separation of duties where required. Approve exact revision hash, not mutable identity. |
| Approved → issued | Issue exact artifact bytes, manifest and approval; sign final bytes if configured; issue ID and audit event. |
| Issued → superseded / withdrawn | Keep original bytes and records; new issue references prior one, reason, authority and delta. No overwriting. |
| Archive | Explicit reason, actor, time, retention class and audit. Preserve discoverability, links and legal-hold flags. Authorized disposition only under approved policy. |

### Document composition and versioning

Create content schemas with required/optional fields, units, validation, linked record pickers and field-level help. Extend registry metadata with schema version, lifecycle applicability, audience, authority and evidence requirements. Convert existing “approved” status to legacy approval evidence, not new certification. Protect approved revisions and historical baseline references from delete; record tombstones for withdrawn identities. Keep old approved template versions renderable or retain original bytes.

Meetings, actions, decisions, risks, issues, changes, requirements and lessons should have durable typed records with authors, owners, dates, status transitions and evidence. Avoid duplicated tables maintained independently of project records.

## RILD-ARCH05 Schedule semantics and calculation contracts

Scheduling is a report dependency, not just a display. KRN-01 and KRN-02 are correctness blockers for claims dependent on affected schedule semantics. Complete the user-facing PS-02 audit and implement verified constraint semantics under separate authorization; do not change results within the read-only audit slice.

| Requirement | Contract |
| --- | --- |
| Network logic | Typed FS/SS/FF/SF, positive/negative lag, calendar-aware offsets, explicit hard/soft constraints, cycle and disconnected detection. Unsupported imported semantics must block affected claims; never silently coerce to ASAP. |
| Activity types | Milestone zero duration; task, summary, level of effort and hammock distinguished where supported. Summary rollups must not double count leaf effort/cost. |
| Calendars | Project/task/resource calendar precedence, working intervals, holidays, DST and exceptions; explicit time units and date boundary conventions. |
| Progress | Separate duration, effort, physical and cost completion; actual start/finish, remaining work/duration, status date, out-of-sequence progress policy and accepted deliverables. |
| Baseline | Immutable named approved baseline, scope membership, calendar and cost versions; original versus current plan and approved changes shown separately. |
| Resource leveling | Preserve logical constraints, capacity and splits; record priority, leveling policy and applied delays; show unlevelled vs levelled forecast. |
| Risk | Monte Carlo input distributions, dependence assumptions, seed/algorithm, iteration count and convergence evidence; P50/P80/P90 and driver ranking. Deterministic plan is not a probabilistic guarantee. |

Critical path/float: report path definition and threshold; negative float and multiple critical paths are visible. Finish variance uses approved calendar and sign convention. EVM SV is monetary, never translated directly into days. Schedule audit rules identify unsupported fields, invalid dates, missing links and incoherent actuals before issuing affected forecasts.

## RILD-ARCH06 Schedule catalog and interchange

Support schedule creation, analysis, comparison and publication as first-class workflows. Use one typed schedule view model: a graph, work/calendar time, baseline and scenario revisions, derived criticality and assignments. The same inputs produce all views; UI and print are not competing schedule engines.

| Output family | Required variations and information |
| --- | --- |
| Master / detail | Integrated master schedule, WBS summary, work-package detail, phase/rolling-wave look-ahead, milestone-only executive schedule. |
| Time and network | Gantt with baseline bars, critical path, dependency network, PERT, calendar/day-week-month-quarter view; readable legends and working time. |
| Variance / trend | Baseline-to-current/actual dates, milestone trend across frozen issues, float changes, late/overdue tasks, constraints, recovery plan and what-if comparison. |
| Resource / effort | Task Usage, Resource Usage, histogram and capacity heat map, leveling comparison, multi-project conflicts. |
| Risk / assurance | Schedule quality audit, missing/unsupported data, risk S-curve, tornado, sensitivity, deterministic vs probabilistic finish. |
| Interchange / data | MSPDI XML, supported import formats through approved worker design, CSV/XLSX, Parquet; fidelity report with unmapped fields, units, counts and source hashes. |

### Import and interoperability

PS-03/DEC-INTEROP governs expanded MS Project and other format support. Read and preserve raw imported artifacts within the project security policy; validate schema, file size, archive expansion and hostile input. Map stable source UIDs, custom fields, calendars, dependencies, progress, resources and baseline fields. Unknown features receive structured loss codes; block “complete import” when material fields are not represented. No silent conversion or defaulting.

Test with independently authored fixtures and actual exported reference files, not only round trips generated by GoPMgr. Compare the intended scheduling rules, not uncritical vendor parity. A 10,000-row tabular result cap must not become a schedule/report truncation policy.

## RILD-ARCH07 Financial, investment and commercial reporting

Retain existing Cost Control ledger separation from legacy Budget context. Extend via approved domain contracts, not duplicate financial fields. Report the accounting basis (commitment, accrual, cash, recognized revenue), cutoff, currency and inclusion rules. Use exact integer minor units or governed decimal scale; reject overflow and unsupported conversion. Preserve legacy fractional JPY without silently rounding.

| Metric / report | Required semantics |
| --- | --- |
| Budget / funding | Base plan + contingency = cost baseline (policy); management reserve separately; authorized funding per approved policy; commitments and actuals distinct; no addition of overlapping categories. |
| EVM | PV, EV, AC and BAC from reconciled performance baseline; SV=EV−PV; CV=EV−AC; SPI=EV/PV; CPI=EV/AC; denominator zero gives unavailable unless defined otherwise. |
| Forecast | ETC, EAC, VAC=BAC−EAC; label selected method, assumption, confidence and reconciliation; BAC/CPI only when applicable, not universal. Time-phased commitments and cash flow separate. |
| Investment | Scenario cash flows, NPV=Σ CFt/(1+r)^t, ROI by stated basis, BCR, payback and IRR where meaningful; multiple/no IRR roots flagged; nominal/real rate consistency. |
| Commercial | Recognized revenue − attributable costs; margin divided by recognized revenue; invoiced, collected, unbilled and outstanding receivable distinct; billing eligibility and credit notes. |
| Benefits | Expected vs realized, KPI unit, measurement source/date, attribution, benefit owner, nonfinancial outcomes; never monetize without stated method. |

Cost Control Phase 2 must define reserve drawdowns, adjustments, allocations, forecasts and EVM authority. Integrate actuals using explicit linked entries and reconciliation. Portfolio FX, if approved, is presentation-only with rate source, date, rounding and original currency totals; mixed currencies otherwise remain separate. No ERP/general ledger or payroll scope.

## RILD-ARCH08 Resource, workforce and time accounting

Resource capacity is not inferred from task counts. A named resource has stable identity, role/skill, calendar, availability, nonproject allocation, effective-dated rate and optional equipment/material units. Shared identities across projects require authorized mapping. Demand is time-phased assignment effort; actuals are approved time entries, not inferred from duration.

| Record or report | Contract |
| --- | --- |
| Assignments | Activity/resource IDs, period, planned effort, units, calendar, rate reference, constraint and scenario. Reconcile leaf allocations; summary rows are derived. |
| Time entry | Person, project, activity/issue/general category, work date, duration in exact units, billable eligibility, evidence, status, submit/approve/reject/correct events. Corrections reverse or supersede approved entries. |
| Timesheet | Period, submitted/approved/pending/rejected totals, billable vs nonbillable, corrections, approval and exceptions; group by resource/project/category. |
| Usage / workload | Available capacity = calendar capacity less leave and nonproject allocations; demand/capacity ×100 when capacity >0; explicit zero-capacity overload; demand minus capacity. |
| Planned vs actual | Compare same scope, unit, period and approved time cutoff; include variance, remaining estimate and reason; do not treat pending time as zero. |
| Skills and resource plan | Role/skill demand, verified availability, gap, equipment/material quantities, planned allocation and conflicts; protect personal data. |

UX: allocation heat map, task/resource usage views, drill-down to assignment and calendar, guided time entry with approval and correction, saved views by week/month/custom period. Optional timers may aid capture but are not evidence of actual effort without review. Never infer performance or rank individuals solely from recorded hours, utilization or story points.

Access: resource managers may see appropriate capacity without private leave details; client exports omit internal rates, personally identifying data and nonbillable detail unless authorized.

## RILD-ARCH09 Agile, quality and delivery analytics

The existing statistical charts and work-item/sprint views are valuable renderers and workflow inputs; they do not establish historical analytics. Capture append-only work-item events with sequence, effective time, recorded time, prior/new state, actor, estimate, sprint, membership and reason. Version the workflow-state mapping and sprint commitment.

| Report | Calculation and integrity |
| --- | --- |
| Burndown / burnup | At sprint start freeze scope and estimate basis. Replay events at daily cutoff; show remaining, completed, added/removed scope, reopened work and original/adjusted ideal reference. Do not fabricate pre-event history. |
| Velocity | Sum accepted completed points or count at sprint close per stated team definition; show committed vs completed, rolling window and sample count; no cross-team employee scoring. |
| Cumulative flow | Count unique items in mapped workflow state at each cutoff; bands reconcile to in-scope total; show added/removed scope and reopened items. |
| Flow / delivery | Throughput, cycle time and lead time distributions; explicit start/end states and censored unfinished items. DORA: deployments, lead time, failures, restore time with scope and denominator. |
| Quality / Six Sigma | Defect/inspection evidence, criteria, pass/fail, sample size, control limits and method; use existing DMAIC/Pareto/control-chart foundation. Distinguish observed defect rate from inferred capability. |
| Acceptance / traceability | Requirement → deliverable → criterion → verification evidence → acceptance decision; changes and exceptions linked. Closure cannot assert acceptance without evidence. |

Migration: preserve old user-entered charts as manually curated artifacts with provenance. Start event-derived reporting only at a defensible observed-history date unless imported event history passes validation. Reconcile current board state to replay; late corrections create restated reports rather than modifying closed sprint history.

Use team-level metrics to improve workflow and planning, not surveillance. Quality and delivery metrics require clear applicability by methodology; predictive and hybrid projects use the same underlying deliverable acceptance records.

## RILD-ARCH10 Portfolio and PMO decision support

Create a local portfolio index with project/program memberships, reporting cadence, strategic objectives, pipeline stages, funding/benefits, decision needs, resource identity mappings and authorized views. Project files remain independently encrypted; index contains minimal metadata or explicitly approved snapshots. No uncontrolled cross-project extraction.

| Report or control | Required behavior |
| --- | --- |
| Portfolio dashboard / review | Coverage list with included, excluded, inaccessible, stale and non-EVM projects; summary by program, health, investment, risk and decisions; drill-down with authorization. |
| Pipeline / prioritization | Proposal → assessment → approval → execution → closure; scored criteria and weights versioned; strategic contribution, risk, capacity and investment; ties and manual rationale visible. |
| Roadmap / team planning | Milestone/phase view, cross-project dependencies, aggregate demand vs common resource capacity; no double count shared resources. |
| History / developments | Compare immutable period snapshots; movements and reasons, newly escalated decisions, changes since last issue, milestone trend and benefits. |
| EVM aggregation | Use ΣEV/ΣPV and ΣEV/ΣAC for applicable same-currency projects; disclose exclusions and denominator; do not average ratios. |
| Selection / escalation | Configurable thresholds and inclusion reasons; red projects, sampled green/yellow, risk, variance, strategic priority and investment; retain exact selection and sample seed. |

Review workflows: configurable PMO report calendar, submitter/reviewer roles, freshness and coverage checks, decision log and post-meeting action report. Provide compact executive page plus drill-down, printable detail, offline interactive HTML or native view. An HTML export is a data disclosure; sanitize and omit restricted fields. No public hosting by default.

Portfolio snapshots use per-project revisions and observed times, not a fictitious simultaneous global database. Block material missing information for complete issue; allow explicit partial view with coverage banner and no misleading portfolio totals.

## RILD-ARCH11 Program scheduling and cross-project control

Programs coordinate interdependent outcomes and interface dates; portfolios govern investments and may contain independent projects. Do not conflate the two. Extend the existing local portfolio index with explicit program membership, interface records and versioned targets; do not make a portfolio aggregate the write authority for project schedules.

| Record / operation | Required semantics and failure behavior |
| --- | --- |
| Program schedule | Stable program ID and revision; program WBS, program milestones, approved target dates, accountable owner, project membership, predecessor/successor relationships and benefit links. Derived dates reference source-project activity revisions; manually controlled targets are distinct. |
| Cross-project interface | Producer and consumer project/activity/deliverable IDs, relationship type, handoff/need date, calendar, status, owner, source revision and confidence. Resolve authorization before reading; inaccessible source becomes unknown, never “complete.” |
| Scheduling authority | Project maintains its activity network and approved baseline. Program publishes interface requirements and scenario proposals; project owners accept changes via governed commands. A program schedule cannot directly overwrite source-project constraints or actuals. |
| Dependency analysis | Build authorized interface graph over immutable project snapshots. Detect cycles, missing references, incompatible calendars, external dependencies and inconsistent cutoffs. Calculate propagation using defined semantics; if unavailable, show unresolved impact rather than invented dates. |
| Scenario coordination | Compare candidate resource reallocations, milestone commitments, funding, cost and dependencies in isolated scenario revisions. An approved decision produces explicit successor revisions; no write-through to baselines. |
| Program health and issue | Show target vs forecast, critical interfaces, milestone trends, delivery confidence, decision needed, shared capacity and benefits. Explain whether a program forecast is computed or a management commitment. |

Independent acceptance: create three projects, A hands off to B and B to C. A forecast moves +5 working days while approved program targets stay fixed. Confirm affected interface warnings, permission-filtered data, baseline preservation and no automatic source edits. Add C→A cycle, inaccessible project and calendar mismatch; block complete program schedule claim until resolved.

## RILD-ARCH12 Monitoring and control: closed-loop architecture

Monitoring runs concurrently with delivery. Establish a MonitoringPlan with cadence, scope, data owners, reporting cutoff, baseline identities, metric definitions, tolerances, escalation and recipients. The system records observations and exceptions; it never silently mutates approved scope or schedules in response to an exception.

| Control stage | Required contract and resulting record |
| --- | --- |
| 1. Observe | Capture revision-checked actuals, milestone status, time, cost, risk, quality and compliance evidence. Track completeness, source and recorded/effective date. |
| 2. Compare | Calculate deviations against the selected approved baseline and threshold version; identify exceptions and trends, not merely red/green. |
| 3. Assess | Review risk/issue and impact; link causes only when evidence supports them; distinguish potential from realized exposure. |
| 4. Communicate | Issue audience-specific report from one frozen snapshot; show decisions, actions, exclusions and assessment rationale. |
| 5. Respond | Create corrective action for observed deviation or preventive action for prospective risk; require accountable owner, target, due date, approval where material. |
| 6. Verify | Track approved change implementation against authorized scope, schedule, cost and risk; verify effectiveness with evidence; close or reopen; preserve original baseline and report. |

ControlRecord fields: ID, project/period, triggering observation and threshold, affected baseline/revision, variance, risk/issue/change references, classification, owner, due date, proposed response, approval, implementation evidence, verification criteria, outcome and closure/reopen event. Separate issue, risk, change and action entities; link instead of duplicating. The control loop is complete only when implementation and effectiveness are verified, not when a status is marked done.

Use daily operational updates, weekly status and monthly performance reviews as configurable defaults, not imposed schedules. A project monitoring plan is itself a versioned controlled document. Offline reminders may run while the app is available; disclose scheduling limitations when closed. No external communications or automatic approval by default.

## RILD-ARCH13 Report definitions, catalog and customization

A ReportDefinition is a versioned declarative contract, not executable user code. It names decision purpose, audience, applicable methodology, authoritative sources, required fields, calculations, default period, filters, grouping, sections, quality rules, permitted formats and approval policy. Each definition has stable ID, semantic version, supersession and tests.

| Contract component | Design |
| --- | --- |
| Typed ReportIR | Report header, sections, tables, chart series, metrics with units/state/precision, evidence links, narrative attribution, accessibility labels and validation findings; immutable after compilation. |
| Query and filter | Allowlisted typed fields, joins, units, stable sorting, bounded pagination and streaming; parameterized queries; totals over full filtered set not current page. |
| Definition editor | Choose fields/metrics, groups, filters, visualizations, page layout and reusable configuration. Validate types, permissions, estimated size and semantic compatibility before save. |
| Audience profiles | Executive one-page, PM detail, team, finance, auditor, client; same source values but different disclosure and granularity. User may tailor optional sections, not suppress required findings. |
| Methodology packs | General project controls, Agile, hybrid, construction, administrative, engineering, software, DMAIC. Map pack to applicable sources and report templates; never claim standards certification. |
| Narrative | Human-authored or deterministic generated statements tied to metric IDs and evidence. Optional AI only with consent, separately labeled, no invented facts, no source-of-truth authority. |

Do not create one renderer per report when semantic sections can compose reusable table, chart and narrative components. Retain bespoke layouts with real value. Add new definitions only if they answer a distinct question or materially improve a lifecycle workflow. Support user-defined variants without multiplying untested code paths.

## RILD-ARCH14 Report truth policy and publication modes

A metric is a typed result, never an ambiguous number. Define MetricValue {metricID, formulaVersion, value, unit, state, precision, scope, cutoff, baselineID, sourceRefs, findingIDs}. State is Known, Unknown, NotApplicable, Unsupported, Invalid or Suppressed. Zero is a Known value with proof; unknown data does not become zero, green, or an empty table. Suppression is a permission decision, not a factual absence.

| Report policy / state | Required behavior |
| --- | --- |
| Draft | Allow incomplete working output with visible watermark/state, scope and findings. No approval, sign-off or complete-report claim. Missing optional data remains explicit. |
| Management preview | Validate data and calculations for selected scope; display severity and source. Material errors block issue, not necessarily preview. Any intentional partial scope has coverage and reason. |
| Validated / reviewed | Automated checks passed for the frozen data and definition; independent human review records outcome. Review cannot change computed facts or bypass errors. |
| Approved for issue | Authorized approval binds snapshot hash, IR hash, definition version, audience, findings and final publication policy. Require any material warning disposition; approval invalidated when inputs change. |
| Issued / certified | Issued only after complete durable artifact package and audit record. Do not label as certified by software. External certification, if ever supported, is separate, signed by authorized certifier under explicit policy. |
| Override and RAG | Threshold policy identifies metric, comparison direction, units, scope, effective version and precedence. Calculated RAG and management assessment coexist; overrides need reason/evidence/actor/expiry and cannot suppress invalidity. |
| Freshness and completeness | Definition-specific expected data, cutoff, age tolerance and required source fields. Show expected/received/rejected/excluded counts and reconcile totals. Strict complete issue blocks missing material evidence. |

Acceptance: a project is forecast late, cost data missing, management marks green and one material import row is unsupported. Preview shows calculated red and cost unknown, management green with rationale, explicit loss and unresolved blockers. Issue is refused. Correct data under a new revision, rerun preflight against same snapshot, approve and publish. Historical draft and prior issued report are retained. No AI or external network may invent evidence.

## RILD-ARCH15 Report Center and usability design

A single Report Center must make the common path simple and the advanced path powerful. Integrate the existing searchable document and chart catalogs rather than introducing parallel navigation. The user starts with a question, not a database table or technical chart identifier.

| Step | Default interaction and error behavior |
| --- | --- |
| 1. Choose | Search by decision or artifact; recommended reports by project stage/methodology; recent and favorites. Explain purpose and data prerequisites. |
| 2. Configure | Preselect project, reporting period, baseline and audience. Show scope and source freshness. Advanced filter/group/template controls progressively disclosed. |
| 3. Validate | Show blocking errors vs warnings; each issue links to affected record or import. Explain unknown data and remedy; do not auto-fill facts. Preserve draft. |
| 4. Preview | Render exact page/data preview from frozen snapshot; inspect metric formula, source records and exclusions. Preview charts and accessible data table. |
| 5. Review/issue | Submit to authorized reviewer, approve exact revision, export to chosen location, retrieve issued record. No overwrite; explicit sign/share choice. |

### Usability acceptance

Test representative novice PM, scheduler, finance analyst, executive, administrator and auditor personas on real workflows: first weekly report, baseline comparison, capacity conflict, missing data correction, historical retrieval and controlled export. Track completion, errors, time-on-task, assistance, accessibility defects and user confidence. Set targets after baseline and owner approval; do not assert unmeasured percentages.

Use keyboard-first navigation, screen-reader semantics, focus management, visible text beyond RAG color, scalable typography and data tables for charts. Offer plain-language guidance and examples, saved configurations, batch report sets, scheduled local reminders when supported, progress/cancel and useful offline help. Preview and exports must identify if a report is incomplete or stale.

## RILD-ARCH16 Rendering, publication and evidence packages

Render only validated ReportIR. Define format capabilities explicitly: PDF/PDF-A for issue and print; DOCX/ODT for editable documents; XLSX/CSV for structured data; HTML/native dashboard for interactive views; Parquet for typed large data. Not every report needs every format. The definition declares supported formats and parity tests.

| Concern | Required contract |
| --- | --- |
| Professional presentation | Letter/A4, portrait/landscape, repeated table headers, page numbering, navigable TOC/bookmarks, legible typography, accessible chart legends and labels, units and source notes. No clipping or hidden omitted rows. |
| Semantic parity | Canonical IR digest, ordered row IDs/counts, metric IDs/values/units and finding set; parse outputs and compare. XLSX formula injection protection; no unsupported render silently substitutes data. |
| Issue package | report ID, original bytes, manifest, frozen source revision refs, canonical IR, calculation and definition versions, validation findings, approvals, output hash, policy/locale/time metadata. Minimize duplicated sensitive content. |
| Atomic publication | Stage private files; verify hashes and format; commit durable issue record only when package complete. Journal/recover interrupted writes; no “issued” success on partial PDF/manifest. Export copy is distinct from issued archive. |
| Signatures | PDF/A transformation before PAdES final mutation; verify signed bytes and trust chain separately. Signature proves integrity/identity under trust policy, not accuracy. No certification label from preflight alone. |

Evidence retention: encrypted internal store; export sanitized manifest or complete evidence package only with permission. Store exact issued artifact, not solely a recipe. Semantic reproduction should be tested; byte-identical regeneration is optional unless explicitly required and supported. Verify external exports and prevent path traversal, collisions, symlink races and accidental plaintext leakage.

## RILD-ARCH17 Security, privacy and migration gates

Report generation expands the attack surface through document formats, imported schedules, custom definitions, attachments and cross-project queries. Threat model the local application, authorized user, hostile file, compromised plugin/worker, untrusted viewer and physical device loss.

| Threat / failure | Control and required evidence |
| --- | --- |
| Disclosure | Least privilege at source, filter and export boundaries; audience field policy and explicit redaction. Separate identity/rate details from executive summaries. Avoid secrets in logs or manifests. Test denied access and cross-project inference. |
| Untrusted content | Schema and bounds validation, safe escaping, no executable report templates, external references or macros by default; protect CSV/XLSX formula injection, HTML XSS, XML/ZIP bombs and unsafe paths. |
| Tampering / rollback | Hash chain and approved revision digest; independently verify issue package. Threat model hash chain against local privileged attacker; optional external signed checkpoint by owner policy. Never claim tamper-proof. |
| Failure / concurrency | Revision-checked writes; immutable snapshot; cancel/timeout resource limits; private staging with cleanup and recovery. Fault injection at each durable boundary. |
| Migration / restore | Encrypted backup, schema validation, forward/reader gates, rollback or read-only legacy access; validate record counts, money and histories, test power loss and hostile files. |
| Supply chain | Pinned dependencies, license review, reproducible builds, security scan disposition and native platform qualification; no new service or engine merely for feature count. |

Respect existing per-user encryption and recovery semantics. Audit authorization, approvals, export, signature, archive and disposition; no unlogged bypass. User-controlled export to external media is a separate confidentiality decision. Native qualification remains open; do not claim Windows, macOS or Linux release assurance on source evidence.
