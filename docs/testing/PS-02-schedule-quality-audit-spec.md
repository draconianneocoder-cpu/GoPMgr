<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PS-02 schedule-quality audit: specification and slice plan

**Status:** PROPOSED. Slice 1 needs your separate implementation
authorization (decision §8). Nothing here is implemented.
**Plan:** [modernization plan](../design/professional-scheduling-modernization-plan.md) §3, §6.

## 1. Scope of slice 1

- A **read-only** audit of one chart schedule.
- It loads **exactly the chart named by `chartID`**, checks that the chart
  belongs to the open project, and records the chart's revision. It does
  not use `loadCurrentProjectSchedule`. That portfolio helper takes a
  project ID, picks the newest CPM chart (`chs[0]`) and can fall back to the
  legacy tasks table, so it could audit a different schedule from the one
  the findings name.
- On **a copy** of that chart's tasks, it arms date constraints with
  `kernel.ApplyConstraintDates` (the project start date and the country work
  calendar) and then runs `CalculateCPM`, as `scheduleProjectTasks` does for
  reports. Loading alone leaves SNET, FNLT and MFO dates inactive
  (`ConstraintArmed` stays false), which would hide SQ-09 violations and give
  SQ-10 and SQ-11 the wrong float. Without a parseable project start date,
  SQ-08's binding report and SQ-09 to SQ-11 are reported **not evaluated**.
- It returns findings. It never writes, repairs or reorders anything.
- No schema change, no new dependency, no calculation change.

**Out of scope for slice 1:**
- history comparison, which needs S3 persistence;
- suggested repairs, which need preview and approval;
- resource-calendar consistency, which needs normalized assignments;
- UI beyond one read-only findings panel.

## 2. Inputs available on `kernel.Task` today

The audit uses these fields:

- `ID` and `Title`;
- `Duration`;
- `Precedents` and typed `Links` (type and lag in working days);
- `Constraint` and `ConstraintDate`; after CPM, `ConstraintViolated`;
- `Milestone`, `PercentComplete`, `ActualStart` and `ActualFinish`;
- `Assignments` and `Overallocated`;
- the CPM outputs `ES`, `EF`, `LS`, `LF`, `Float` and `IsCritical`.

There are no summary tasks: the schedule is flat. The project start comes
from `Project.StartDate`. The reporting date is passed in, as for EVM.

## 3. Rules

**Profiles.** Every threshold is configurable, and each named profile records
its values.

- **GoPMgr default** is the shipped profile.
- **DCMA-style** uses the thresholds commonly published for the DCMA 14-point
  assessment. It is labelled "DCMA-style", **never** "DCMA certified" or
  "compliant". Its percentage tests report a metric and a pass/fail against
  the threshold.

| ID | Rule | Definition on today's data | Default severity | DCMA-style threshold |
|---|---|---|---|---|
| SQ-01 | Missing predecessor | A non-milestone task, other than the first, with no incoming link | warning | ≤ 5 % of tasks |
| SQ-02 | Missing successor | A non-milestone task, other than the last, with no outgoing link | warning | ≤ 5 % (with SQ-01) |
| SQ-03 | Dangling reference | A link or precedent naming an ID that is not in the schedule | error | 0 |
| SQ-04 | Dependency cycle | The cycle's member tasks, found by Tarjan's SCC. `CalculateCPM` only reports that a cycle exists | error | 0 |
| SQ-05 | Lead (negative lag) | `Lag < 0` | warning | 0 % of links |
| SQ-06 | Long lag | `Lag >` profile limit in working days | info | lags ≤ 5 % of links |
| SQ-07 | Non-FS relationships | The share of links that are not FS, reported | info | FS ≥ 90 % |
| SQ-08 | Hard constraint | `MFO` or `FNLT`; which constraints bind is reported from `ConstraintViolated` | warning | ≤ 5 % |
| SQ-09 | Constraint violated | `ConstraintViolated` after CPM | error | 0 |
| SQ-10 | High total float | `Float >` profile limit (working days) | info | > 44 days, ≤ 5 % |
| SQ-11 | Negative float | `Float < 0` | error | 0 |
| SQ-12 | Long duration | `Duration >` profile limit, milestones excluded | info | > 44 days, ≤ 5 % |
| SQ-13 | Milestone without logic | A milestone with no incoming and no outgoing link | warning | — |
| SQ-14 | Invalid actuals | `ActualStart` or `ActualFinish` after the reporting date; finish before start; a finish without a start; `PercentComplete` of 100 without `ActualFinish`, or above 0 without `ActualStart` | error | 0 invalid dates |
| SQ-15 | Missing progress evidence | `ES` is before the reporting date and `PercentComplete` is 0 with no `ActualStart` | warning | — |
| SQ-16 | Overallocation | `Overallocated` from the existing detector | warning | — |
| SQ-17 | Unresourced task | A non-milestone task with no assignment | info | ≤ 5 % (DCMA "resources") |
| SQ-18 | Reporting context | Missing project start date, missing reporting date, or no baseline available | info | — |

**Notes on the rules:**
- "First" and "last" in SQ-01 and SQ-02 mean **designated** start and finish
  anchors: task IDs that the profile or the chart names explicitly, such as
  the project-start and project-finish milestones. They are never derived
  from computed dates. In an unconstrained schedule, `CalculateCPM` gives
  ES 0 to every task without a predecessor, so an ES-based exemption would
  excuse exactly the tasks SQ-01 must find. With no designated anchor, every
  start and end of every chain is reported.
- SQ-15 depends on today's dates. Its finding states the reporting date it
  used.
- When SQ-04 finds a cycle, CPM-based rules (SQ-09 to SQ-11, SQ-15) do not
  run and are reported as **not evaluated**, never as passing.

## 4. Finding format

Each finding carries:

- the rule ID and profile;
- the severity;
- the affected record: task ID and title, or link (predecessor → successor);
- the observed value and the threshold;
- an explanation;
- a possible correction, stated as advice;
- the source chart ID, chart revision or updated-at stamp, and the audit
  engine version.

Results are ordered deterministically: severity, rule, task ID, then link.
Percentage metrics are reported with numerator and denominator.

## 5. Correctness method

- **Independent oracles.** Each rule has a small hand-computed fixture with
  its expected findings written by hand. The audit's own code is not used to
  produce them. Cycle and float expectations come from hand-drawn graphs.
- **Defective fixtures.** One per rule, for example a 3-task cycle, a link
  to a missing ID, `Lag -2`, an MFO that CPM cannot meet, 100 % complete
  without an actual finish, and an orphan milestone. There is also a clean
  fixture that must produce **zero** findings, to check for false positives.
- **Non-destructive proof.** The input tasks are deep-copied and compared
  before and after (`reflect.DeepEqual`). The chart row, revision and audit
  log are unchanged after an audit call, checked against a real encrypted
  project.
- **Determinism.** Two runs give byte-identical results, including under
  `-race` with concurrent calls.
- **Scale.** At T2 (50,000 tasks, the S0 fixture), every rule except SQ-16
  must be O(tasks + links). SQ-16 reuses the existing detector. Since RES-A-1
  (2026-10-27), that detector finds breach participants through an index
  instead of the S0 F3 rescan: 45.9 ms at T2 in the
  [overallocation contract](resource-overallocation-contract.md) benchmarks.
  Its remaining cost follows the dense demand profiles (RES-B) and is
  reported, not hidden. The audit can be cancelled through `context`.

## 6. Test IDs

| ID | Requirement |
|---|---|
| SQA-001 to SQA-018 | Rule SQ-nn fires on its defective fixture with the expected finding fields |
| SQA-019 | The clean fixture yields zero findings |
| SQA-020 | Input tasks and the stored chart are unchanged (non-destructive) |
| SQA-021 | Output is deterministic, including under `-race` |
| SQA-022 | A cycle marks the CPM-dependent rules "not evaluated" |
| SQA-023 | Profiles: each threshold change moves exactly the expected findings |
| SQA-024 | T2 runtime and memory are recorded against the S0 hardware profile. The budget comes from OWN-10; none is invented here |
| SQA-025 | Cancellation stops the audit within a bounded time |
| SQA-026 | Labels: no output text claims DCMA certification |
| SQA-027 | A schedule with a second, disconnected chain (A→B→C plus D→E), with A and E designated as anchors, reports D under SQ-01 and C under SQ-02, although CPM gives D ES 0. With no anchors designated, it reports A and D, then C and E |
| SQA-028 | A violated SNET, FNLT or MFO date is reported by SQ-09, and float in SQ-10 and SQ-11 reflects the armed constraints; without a project start date those rules read "not evaluated" |
| SQA-029 | Auditing an older chart of a project with several CPM charts audits that chart, and the findings carry its ID and revision |

## 7. PR breakdown (each needs authorization)

1. **PS-02a:** the `internal/schedaudit` package (rules, profiles, finding
   type) with SQA-001 to 023 and 026 to 028. No App or UI change.
2. **PS-02b:** the read-only `App.AuditSchedule(chartID, reportingDate,
   profile)` method, with tests on a real encrypted project (SQA-020), chart
   selection (SQA-029) and the T2 measurement (SQA-024, 025).
3. **PS-02c:** a read-only findings panel with keyboard navigation and screen
   reader semantics, and an export of the findings into the existing report
   composer.

## 8. Risks

| Risk | Mitigation |
|---|---|
| False positives erode trust | The clean fixture; the false-positive review in the exit gate; info severity for judgment rules (SQ-06, 10, 12, 17) |
| Thresholds treated as universal | Named profiles; values shown in every finding |
| A certification claim implied | SQA-026; "DCMA-style" wording only |
| SQ-16 inherits the detector's remaining cost | Reported separately. RES-A-1 removed the S0 F3 rescan; the dense-profile cost is RES-B's subject |
| The wrong schedule or inactive constraints are audited | Load the named chart only (SQA-029); arm constraints before CPM (SQA-028) |
| The flat schedule hides summary-level issues | Recorded as a known limit until S1/S3 adds hierarchy |
| Dates depend on calendars | Use the kernel's `DayOffset` and country calendars only. No second calendar implementation (AP-03) |
