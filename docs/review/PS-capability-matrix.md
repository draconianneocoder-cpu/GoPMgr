<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Professional scheduling: requirements-to-capabilities matrix

**Date:** 2026-10-26.
**Source baseline:** `main` 28a758d, inspected by reading the code. No feature
was exercised in a running app for this matrix.
**Plan:** [professional-scheduling-modernization-plan.md](../design/professional-scheduling-modernization-plan.md).

## Status words

| Status | Meaning |
|---|---|
| **Implemented** | On `main`, with code and tests that do what the row says. The evidence column names the code |
| **Partial** | On `main`, but narrower than the requirement. The gap is named |
| **Proposed** | Designed or planned, with no production code |
| **Blocked** | Needs a decision or resource that is not available |
| **Unverified** | A claim not checked by test or hands-on use |

Branch work such as the ADR-010 G0/G1 spikes and the ADR-012 DF0 spike is
**not** a released capability (PS-00), so those rows say Proposed.

### The ProjectLibre column

It summarizes what the vendor's site and third-party listings say about
ProjectLibre Desktop: Gantt, network diagram and critical path, WBS, resource
assignment and leveling, histograms, earned value, baselines, and Microsoft
Project file compatibility.

- **All of it is Unverified.** QG-06 requires a hands-on comparison against
  a documented ProjectLibre version, and none has run.
- Where the vendor says nothing, the cell says "not stated", not "no".

## 1. Schedule logic and calculation

| Requirement | GoPMgr status | Evidence | ProjectLibre (vendor-stated) | Work package |
|---|---|---|---|---|
| Critical Path Method, forward and backward pass, float | **Implemented** | `internal/kernel/scheduler.go` `CalculateCPM`. S0 measured 24 ms at 10k tasks and 205 ms at 50k (F7) | Critical path stated | — |
| Four link types (FS, SS, FF, SF) with lag | **Partial**: lag is in working days only; no elapsed or percent lag | `scheduler.go` `LinkType`, `Link.Lag` | Not stated | PS-08 |
| Dependency-cycle detection | **Partial**: CPM refuses a cycle (`CalculateCPM` returns false), but no report names the cycle's tasks | `scheduler.go:232`, `:450` | Not stated | PS-02 |
| Constraints | **Partial**: ASAP, ALAP, SNET, FNLT and MFO. MSO, SNLT and FNET are absent. `ConstraintViolated` is flagged | `scheduler.go` `ConstraintType`; `anchor.go` `ApplyConstraintDates` | Not stated | PS-08 |
| Project calendars | **Partial**: a country holiday calendar per project, plus time-zone checks. No custom project calendars, exceptions or per-task calendars | `internal/calendar/calendar.go` `For`, `HolidaysIn` | Calendars stated | PS-01, PS-08 |
| Resource calendars | **Partial**: stored and used in capacity plans | `internal/db/resource_calendars.go`; `kernel/resources.go` `ResourceCalendar` | Not stated | PS-06 |
| Finish-anchored or backward project scheduling | **Proposed** (only task-level ALAP exists) | — | Not stated | PS-08 |
| Activity splitting | **Partial**: `AllowSplitting` in leveling and persisted `PlannedWorkSegments`; no manual split or resume | `scheduler.go` `WorkSegment`; `kernel/resources.go` | Not stated | PS-08 |
| Resource leveling | **Implemented** (kernel and App method); correctness against real projects **Unverified** (the existing ROADMAP says so) | `kernel/resources.go` `LevelResourcesWithOptions`; `App.LevelChartResources` | Leveling stated | PS-06 |
| Overallocation detection | **Implemented.** S0 F3 found it roughly quadratic (Gantt layout 61 s at 50k tasks). RES-A-1 (2026-10-27) removed that rescan with unchanged output: T1 detection 2.05 s → 6.16 ms, T2 45.9 ms. Dense demand profiles remain (RES-B) | `DetectOverallocations*`; S0 §6.1; [overallocation contract](../testing/resource-overallocation-contract.md) | Stated | PS-04 |
| Effort-driven, fixed-work, fixed-units and fixed-duration policies | **Proposed** | — | Not stated | PS-07, PS-08 |
| Monte Carlo schedule risk | **Implemented**, with a report export | `kernel/montecarlo.go`; `App.ExportChartMonteCarloRiskReport` | Not stated | PS-09 |

## 2. Baselines, progress and earned value

| Requirement | GoPMgr status | Evidence | ProjectLibre (vendor-stated) | Work package |
|---|---|---|---|---|
| Schedule baselines and variance | **Implemented** for chart schedules | `kernel/baseline.go` `CompareSchedules`; `App.ListScheduleBaselines`, `CompareScheduleBaseline` | Baselines stated | PS-01 (normalized baselines) |
| Progress: percent complete, actual dates | **Partial**: stored and reported; "never reschedules the plan" by design | `scheduler.go` `Task` comments | Not stated | PS-02, PS-08 |
| Earned value (schedule and portfolio) | **Implemented**: EV, PV and AC in exact minor units, SPI and CPI | `kernel/evm.go` `ComputeEVM`; `App.ComputeScheduleEVM`; `RunPortfolioAnalytics` | Earned value stated | PS-09 |
| Cost baselines | **Implemented** in the cost domain | `App.ListCostBaselines` | Not stated | PS-07 |

## 3. Scope, resources and cost

| Requirement | GoPMgr status | Evidence | ProjectLibre (vendor-stated) | Work package |
|---|---|---|---|---|
| WBS hierarchy | **Implemented** (WBS v2, revision-checked, 10,000-node limit) | `internal/wbs/model.go:19` | WBS stated | PS-01 keeps the limit until a tested migration |
| Schedule outline distinct from the governance WBS | **Partial**: the schedule is a chart document, not linked records | `internal/db/charts.go`; S0 F1 | — | PS-01 |
| Resource definitions and assignments with units | **Partial**: per-task resource units inside the chart document; no normalized assignments | `kernel/resources.go`; MSPDI `Assignments` | Stated | PS-01, PS-06 |
| Task Usage and Resource Usage time-phased views | **Proposed**. The kernel computes per-day usage (`ResourceUsage`), but no usage view exists | `kernel/resources.go:242` | Histograms stated | PS-06 |
| Resource rates (standard, overtime, cost per use, effective-dated) | **Partial**: a stakeholder hourly rate feeds committed labor cost; no per-assignment rate tables | `internal/budget/budget.go:150` | Not stated | PS-07 |
| Committed and actual cost, procurement, cost ledger | **Implemented** (the cost-control domain) | `internal/budget`; `CostControlPanel.svelte`; `ProcurementCatalog.svelte` | Not stated | PS-07 links them |
| Shared resource pools across projects | **Proposed** | — | Multi-project pools stated in third-party listings | PS-10 |
| Custom fields | **Proposed** (none on `main`) | — | Custom columns stated | PS-01, PS-03 |

## 4. Views and usability

| Requirement | GoPMgr status | Evidence | ProjectLibre (vendor-stated) | Work package |
|---|---|---|---|---|
| Gantt chart | **Implemented** as a chart editor. S0 measured a 61 s layout at T2, mostly the F3 rescan, which RES-A-1 removed; the layout has not been re-measured at T2 | `charts/GanttEditor.svelte`, `GanttBars.svelte` | Stated | PS-04, PS-05 |
| Network, CPM and PERT diagrams | **Implemented** as separate chart editors | `charts/NetworkEditor.svelte`, `CPMEditor.svelte`, `PERTEditor.svelte` | Stated | PS-05 |
| One connected workspace with synchronized selection | **Proposed**. The views are separate chart documents today | — | Not stated | PS-05 |
| Saved views, filter, group, sort | **Proposed** for the schedule | — | Not stated | PS-05 |
| Revision-aware undo and redo for schedule edits | **Proposed**. Only toast-level undo exists | `ToastContainer.svelte` | Not stated | PS-05 |
| Schedule-quality audit (DCMA-style profile) | **Proposed**. The building blocks (CPM, cycles, constraint violations, overallocations) exist | See §1 | Not stated | **PS-02** |
| 50,000-task editable schedule (T2) | **Blocked** by architecture: 2.5 s p50 per edit and 35.5 MB of audit per edit (S0 F1, F2). The 61 s layout (F3) was largely the overallocation rescan, removed by RES-A-1 | S0 baseline | Not stated | PS-04 (with ADR-011) |

## 5. Interoperability and reporting

| Requirement | GoPMgr status | Evidence | ProjectLibre (vendor-stated) | Work package |
|---|---|---|---|---|
| MSPDI (Microsoft Project XML) import | **Partial**: tasks, durations, milestones, summaries, percent complete, links (type and lag), resources and assignment units. **Dropped:** outline hierarchy, calendars, constraints, actual dates, baselines, costs, custom fields. 64 MiB cap (S0 F4) | `internal/export/mspdi_import.go:58-84` | MS Project compatibility stated | PS-03 (G2/G3) |
| MSPDI export | **Partial**: ID, UID, name, start, finish, duration, milestone, percent complete, links, resources, assignments | `internal/export/mspdi.go` | Stated | PS-03 (G4) |
| MPP, POD and Primavera import | **Proposed** (ADR-010 G2/G3 through MPXJ; G0 transport only on a branch) | ADR-010 plan §2, §4, §6 | MPP stated; fidelity anecdotally imperfect | PS-03 |
| Report exports: PDF/A-2u (signable), DOCX, ODT, XLSX, CSV, HTML, iCal | **Implemented** | `internal/export/*`; ADR-006 | PDF printing stated | PS-09 |
| Tabular analytics import (CSV, TSV, TXT, JSON, NDJSON, Parquet) | **Implemented, with defects**: DuckDB legacy defects LD-1 to LD-9 | `docs/testing/adr-012-df0-evidence.md` (PR #151) | Not stated | PS-09 with ADR-012 |
| Six Sigma and agile (DORA) analytics | **Implemented** (beyond the ProjectLibre scope) | `internal/sigma`, `internal/agile` | Not stated | — |

## 6. Security, offline use and distribution

| Requirement | GoPMgr status | Evidence | Work package |
|---|---|---|---|
| Encrypted project files (SQLCipher), per-user keys, recovery codes | **Implemented** | ADR-001; `internal/crypto`, `internal/db` | QG-05 |
| Fully offline core | **Implemented**; the update channel is disabled by design | ROADMAP current focus | AP-05 |
| Third-party notices, with TPC exclusion enforced | **Proposed** until PRs #144 to #149 merge | PR #146 | QG-07 |
| Signed and notarized packages on every platform | **Blocked**: signing resources are pending (P04-D6) | `docs/release-preflight.md` | QG-07 |

## Summary of gaps that set the priority order

1. **The schedule is a chart document, not normalized records.** This is the
   root of F1 (edit latency), F2 (audit growth), the lack of linked views,
   and the lack of usage views. → PS-01 and PS-04, through ADR-011 S1–S3.
2. **Import fidelity is narrow.** MSPDI drops hierarchy, calendars,
   constraints, actuals, baselines and costs. → PS-03 (G2/G3).
3. **Schedule quality is invisible.** The calculations exist, but no audit
   reports them. → PS-02. It is the cheapest high-value slice, because it is
   read-only and works on today's kernel types.
4. **Several professional features are missing:** usage views, rate tables,
   three constraint types, project calendars. → PS-06 to PS-08, after PS-01.
