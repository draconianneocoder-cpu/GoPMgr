<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Connected project model: domains, dependencies and open ADR-011 decisions

**Status:** a design proposal for the
[modernization plan](professional-scheduling-modernization-plan.md) (PS-01).

- It is not a schema.
- ADR-011 (reserved, ADR-010 S1) owns the exact physical design.
- Table families follow ADR-010 implementation plan §5.2.

## 1. Domain ownership (AP-01, proposed for confirmation)

The received decision text leaves AP-01's list of domain owners empty. This
table is the proposal to confirm or correct. Each row has **one** owner of
authoritative writes.

| Domain | Owns | Authority today | Never owned elsewhere |
|---|---|---|---|
| Project | identity, currency, country, calendar policy, format version | `internal/db` project row | — |
| Schedule | generations, activities, outline, links, constraints, schedule calendars, progress, actuals | Chart documents (`internal/db/charts.go`), calculated by `internal/kernel` | Dates and float are computed only by the Go kernel (AP-03) |
| Scope (WBS) | WBS v2 nodes and work-package associations | `internal/wbs` (10,000-node limit) | The schedule outline is not the WBS (PS-01) |
| Resources | definitions, groups, resource calendars, capacity | `internal/db/resource_calendars.go`, plus resources inside charts | — |
| Assignments | task–resource units, work, timephased segments | Inside chart documents | Usage views derive from assignments, never the reverse (PS-06) |
| Cost control | budgets, commitments, actual expenditure, ledger, procurement, cost baselines | `internal/budget`, cost ledger | No forecast becomes an actual or a commitment without a financial transaction (PS-07) |
| Baselines | schedule, scope and financial snapshots | Schedule baselines per chart; cost baselines | — |
| Provenance | source artifacts, lineages, field observations, import receipts | Proposed (ADR-010 D-09) | Imported values and calculated values stay distinct (AP-03) |
| Audit | the hash-chained audit log | `internal/db/audit.go` | Workers never write it |
| Analytics | nonauthoritative computed views | DuckDB today; ADR-012 target | Never a system of record (ADR-012 DF-01) |

## 2. Entity dependencies

```mermaid
flowchart TB
  P[Project] --> SG[Schedule generation]
  SG --> A[Activities / outline]
  SG --> L[Links]
  SG --> C[Schedule calendars + exceptions]
  SG --> K[Constraints]
  SG --> PR[Progress / actuals]
  A --> L
  A --> K
  A --> PR
  P --> W[WBS v2 nodes]
  W -. "accepted mapping only" .- A
  P --> R[Resources]
  R --> RC[Resource calendars]
  A --> AS[Assignments]
  R --> AS
  AS --> TP[Timephased segments]
  P --> CC[Cost control: budgets, commitments, actuals, ledger]
  AS -. "planned cost via rate tables" .-> CC
  SG --> B[Baselines + baseline values]
  CC --> FB[Financial baselines]
  SRC[Source artifacts + lineages] -. provenance .-> A
  SRC -. provenance .-> AS
  AUD[Audit chain] -. "records every command" .- SG
  AUD -. .- CC
  AUD -. .- W
```

Solid arrows mean ownership: the parent's identity scopes the child's.
Dashed lines are references. Every reference uses a stable typed ID,
never a name or a row position.

## 3. What has to hold across domains

1. **One active editable generation** per schedule owner. Edits are
   revision-checked commands (ADR-010 §7.1 `ApplyTaskPatch`).
2. **Usage totals reconcile** with assignments and calendars (the PS-06 exit
   gate).
3. **Money is checked integer minor units**, with currency at the project
   level as today (ADR-009). There is no float authority, matching the
   existing `PortfolioSummaryWire` decimal strings.
4. **Imported values keep provenance.** A calculated value never overwrites
   an imported observation silently.
5. **Audit events are typed and compact** (S7B). Old whole-chart events stay
   verifiable.

## 4. Open ADR-011 decisions

| # | Decision | Why it is open | Evidence or constraint |
|---|---|---|---|
| O-1 | Exact column types, units and precision (durations, work, lag, money) | §5.2 is a reference design, not DDL | SQLite 3.33.0: no `STRICT`, `RETURNING`, JSON1, FTS5 or `DROP COLUMN` (S0 F5) |
| O-2 | ID scheme for host IDs, and how source UIDs are scoped | Stable IDs across edits and imports | ADR-010 §5.2 identity column |
| O-3 | Revision granularity: per generation or per entity | Concurrency and undo cost | §7.1 opaque cursors bound to the revision |
| O-4 | How the schedule outline maps to the WBS | The plan keeps them distinct unless a mapping is accepted | WBS v2 contract |
| O-5 | Raising the 10,000-node WBS limit | Needs a tested migration and a performance decision | `internal/wbs/model.go:19`; the decision text forbids silently removing it |
| O-6 | Calendar model: project, resource and task calendars; exceptions; time zone and DST | Today only country holidays plus resource calendars exist | Matrix §1 |
| O-7 | Timephased representation: sparse segments, contours, precision | Must not explode into per-minute or per-day rows | §5.2 timephased families |
| O-8 | Number of baselines and their value sets | "All supported source baselines" (§5.2) | MPXJ formats carry several baselines |
| O-9 | Custom field typing and versioning | Source type versus modeled type | §5.2 custom field families |
| O-10 | Converting legacy chart schedules and recovering from failure | Existing projects must stay readable (§5.3) | Format gate before every writer |
| O-11 | Typed audit events and how they bridge to the existing chain | S7B; the chain must stay verifiable | S0 F2 (35.5 MB per edit at T2) |
| O-12 | Where rate tables live (resource domain or cost domain) and their effective dates | PS-07 | `internal/budget` hourly rate today |
