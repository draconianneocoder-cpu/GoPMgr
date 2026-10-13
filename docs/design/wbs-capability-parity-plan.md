<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Connected WBS capability and usability plan

Status: Product direction and first pure model/validation slice approved; runtime
integration remains proposed. On 2026-10-13
the user selected local-first file-based sharing, approved designing the unified
WBS, and permitted requiring the new version for upgraded WBS files. Exact
production persistence/API integration, migration enforcement and financial
policies remain separately gated.
This is a capability assessment and delivery plan, not completed parity.
Baseline: `5e97f75`, inspected 2026-10-13; initially clean worktree.

## Target and evidence boundary

Deliver the proposed capability set, subject to scope approval, described by the
[ProjectManager WBS guide](https://www.projectmanager.com/guides/work-breakdown-structure),
with a coherent GoPMgr workflow rather than disconnected duplicate entry.
The guide is advertised capability evidence, not independent verification of
the competitor's application. Its examples and recommended depth are not hard
schema restrictions. Source inspection establishes the findings below; no
native UX audit, competitor account test, or comparative user study ran.

The target is project managers and schedulers using the existing Go/Wails/Svelte
desktop application. Keep its existing industrial/utilitarian design language.
Analytical job: scope hierarchy with connected execution detail. Tree and outline
are complementary views, not replacements for the separate schedule graph.
Native desktop is the delivery target; narrow-window/keyboard behavior requires
validation, without claiming a mobile application exists.

## Capability map

P1 means core workspace; P2 identity/dictionary; P3 execution connections;
P4 cost/reporting connections; P5 integrated assurance. Stages are proposals.
“Not established” means no corresponding connected behavior was established in
the inspected paths, not proof that every file in the repository lacks it.

| Capability | Current evidence and gap | Intended outcome / stage | Acceptance evidence |
| --- | --- | --- | --- |
| Hierarchy, levels and codes | WBS tree, child/sibling/delete and derived numbering exist | Preserve IDs through move, indent/outdent, reorder and undo / P1–P2 | Same entities and content after edit, save and reopen |
| Deliverable/phase/process/time organization | Arbitrary tree titles can represent these; semantic classifications not established | Optional classifications without mandatory phases; approved technical resource bounds apply / P2 | Existing trees retain meaning and all content |
| Dictionary and scope boundaries | Separate WBS document has chart reference, free-text codes, descriptions and acceptance criteria | Node-linked definitions and exclusions, without duplicate entry / P2 | Renumbering cannot disconnect definitions |
| Planning packages and control/cost accounts | Explicit linked semantics not established | Define package/control boundaries, not extra copies of scope or money / P2–P4 | Identity and reconciliation tests against approved rules |
| Tree, outline and Gantt | Tree exists; Gantt uses a separate nodes/edges document | Shared scope views plus explicitly linked schedule / P1–P3 | Selection and scope information agree across views |
| Tasks, subtasks, descriptions and owners | WBS title/note/free-text owner/effort; schedule tasks separate | Connect executable activities without treating every scope node as an activity / P3 | No duplicated activity after retry or rename |
| Priority and duration | Schedule duration exists; WBS priority and duration integration not established | Edit in owning task record; distinguish duration from effort / P3 | Correct units and cross-view updates |
| Dependencies and critical path | Existing kernel supports FS/SS/FF/SF and CPM | Reuse engines; containment never implies precedence / P3 | All link types and cycle/error paths tested |
| Dates and milestones | Calendar-anchored scheduling and Gantt milestones exist separately | WBS-linked execution detail / P3 | Calendars and milestone behavior retained |
| Resource assignment and workload | Resource capacity/leveling paths exist separately | Link existing resource planning, not another engine / P3 | Overload and leveling results remain consistent |
| Logged hours | Connected WBS actual-hours workflow not established | Define time-entry ownership and units before implementation / P3 | Correct revisions, totals and correction behavior |
| Status and progress | Schedule percent-complete exists separately | Approved summary equations; missing values remain unknown / P3 | Boundary, weighting and milestone tests |
| Notes and communications | WBS notes exist; connected communication history not established | Define local annotations versus team communication scope / P1–P3 | History and access behavior match agreed scope |
| Dashboards and filtered reports | Separate analytical/reporting facilities exist | WBS-scoped views with source revisions and explicit filters / P4 | Filters and exports agree with stored source |
| Costs and estimates | Exact-money ledger exists without WBS identity fields | Explicit associations and reconciled totals / P4 | Each contribution counted once; overflow rejected |
| Templates and spreadsheet exchange | WBS example/seed and separate schedule exchange exist | WBS-specific import preview and round-trip exports / P2–P4 | No ambiguous-ID overwrite or lost fields |
| Scope baseline and plan documents | WBS narrative, generic baseline storage/scenarios and schedule-baseline APIs exist; a reviewed WBS scope-baseline workflow is not established | Define reviewed scope versions and linked document snapshots / P2–P4 | Historical snapshots remain immutable and identifiable |
| Related breakdown structures | Connected risk/cost/resource/product/organization hierarchies not established | Decide required classifications/views; do not silently omit them from parity / definition gate | Accepted dispositions and representative workflows |

### Repository evidence anchors

- `frontend/src/lib/components/charts/WBSEditor.svelte` and
  `internal/charts/dag/wbs.go`: scope tree and current editing/layout contract.
- `internal/documents/templates.go`, `wbs_document.go`: independent narrative
  records. Template description mentions owners, but its current row fields are
  code, description and acceptance criteria; do not claim an owner dictionary.
- `frontend/src/lib/components/charts/GanttEditor.svelte`,
  `internal/charts/dag/gantt.go`, `cpm.go`, `internal/kernel/scheduler.go`, and
  `app_charts.go`: existing schedule, resource and baseline components.
- `internal/db/cost_control.go` and
  [project cost ledger scope](project-cost-ledger-scope.md): existing financial
  ownership and exact-money boundary.

## Model direction and remaining integration gates

See the [unified model contract](wbs-unified-model-contract.md) for the reviewed
identity, write, migration and sharing proposals and their verification gates.

WBS owns scope containment, display codes and dictionary content. Schedule
records own precedence, dates, calendars, assignments and execution progress.
Cost records own amounts and financial states. Project-scoped references connect
them; “one WBS” must not mean merging those authorities into one mutable blob.

Existing valid node IDs must survive moves and migration. Validate uniqueness
within the agreed identity scope; copy/import remapping is explicit. Display
codes are not reference keys. Decide chart/node identity scope, link cardinality,
selection among multiple schedules/WBS charts, edit ownership, and unlink/delete
behavior before implementing persistence. Reparenting must not create a
dependency or silently change dates.

Legacy dictionary records need a reviewed reconciliation path. Preserve original
rows and source context, surface missing/duplicate/conflicting matches, and never
bind historical text using current renumbered codes alone. Decide precedence
among multiple documents and whether new documents are live projections or
versioned snapshots.

Define JSON versioning, unknown-field preservation, older-reader/writer policy,
atomic upgrades, interruption recovery, and backup/restore before migration.
Use plaintext and SQLCipher fixtures. No silent destructive conversion.

## Financial and calculation constraints

Approve cost association/cardinality and direct-versus-descendant rollups before
coding them. Each ledger contribution enters a total once. Planned, commitment
and actual stay separate; unmapped amounts remain visible and reconcile to the
project ledger. Shared attribution needs an explicit rule, never guessed splits.
Control accounts and category views cannot create another amount-bearing copy.
Resource estimates do not become posted expenses.

Preserve checked minor-unit arithmetic, decimal-string transport, existing JPY
restrictions, and immutable financial snapshots. Ordinary WBS cost reporting is
not automatically blocked on financial roles. Reserve movements, funding
allocation and forecasting remain subject to the separately proposed
[Phase 2 contract](cost-control-phase-2-lifecycle.md). This plan does not approve
that contract or introduce FX.

Define effort units, summary progress weights, missing-data handling, milestone
treatment and baseline binding. Scope-completeness checks support human review;
they cannot prove that every necessary deliverable was identified.

## Delivery plan and gates

1. Local-first file sharing and designing the connected-WBS direction are
   approved. Upgraded WBS files may require the new version. Agree remaining
   capability dispositions; deferred capabilities remain gaps until accepted.
   Actual old-version protection must be demonstrated before enabling upgrades.
2. Approve domain authority, identities, associations, legacy reconciliation and
   migration behavior. Produce visual workspace options using the existing
   application as grounding; select a direction before UI implementation.
3. Deliver P1–P2 as bounded slices: readable tree/outline, search, collapse/expand,
   keyboard actions, reorder, undo, clear saving/dirty/error feedback, and linked
   dictionary content. Preserve current write-ordering safeguards.
4. Deliver P3 by connecting existing scheduling/resource engines. Approve actual
   hours, communications and progress semantics first. No cloned schedulers.
5. Deliver P4 after association/rollup approval: exact reconciled cost views,
   versioned scope reporting, import preview, export and templates.
6. P5: run migration, negative-input, cross-project/stale-link, concurrency,
   failure/retry, reopen, audit and export tests. Complete a disposable project
   journey through native Wails on supported platforms; report unavailable
   platform evidence rather than claiming it passed.

## Defining “more user friendly”

Proposed workflow tasks: create a realistic breakdown, find a work package,
reorganize without losing links, complete its definition, connect activities,
inspect costs/progress, recover from save failure, and export a reviewed plan.
Agree participant groups, representative project sizes and success thresholds
before evaluation. Measure task completion, duplicate entry, errors, recovery
and elapsed task time against current GoPMgr. Validate keyboard-only and
assistive-technology paths, full-title access, narrow windows and large trees.
Do not invent benchmark results. Superiority over ProjectManager additionally
requires a matched participant comparison with that product; guide inspection
alone cannot establish it.

## Review and current outcome

Self-critique and an independent read-only plan review identified incomplete
capability accounting, blurred ownership, cost double counting and unsafe legacy
code matching. The revised plan addresses those as explicit contracts and gates.
The user's scope/compatibility-policy choices are recorded above. The approved
pure model foundation is implemented in `internal/wbs`; see the unified model
contract for its limits and exclusions. Persistence, migration enforcement,
sharing and workspace UI are not implemented by that slice.
No prototype or production feature changed. The previous planning pass ran the
current WBS component baseline: 16 tests passed; this is historical evidence, not
validation of the proposed model.
