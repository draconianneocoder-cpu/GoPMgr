<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Resource overallocation: legacy contract and test oracle

## Scope

These tests pin what `ResourceUsage`, `DetectOverallocations` and
`DetectOverallocationsWithPlan` return. They were written against the
original detector and are the compatibility reference for any faster one.
Since RES-A-1 (2026-10-27) the detector finds each breach's participants
through an index built once per call instead of rescanning every task. Its
output, flags, demand and capacity resolution are unchanged, and every test
below passes against it unmodified.

- **Pinned source:** `internal/kernel/resources.go` at git blob
  `0826dfc62755a0b7eb50c06680c59650809edad2` (main `28a758d`).
- **Why:** Gantt layout was roughly quadratic in schedule size because
  every overallocated resource-day rescanned every task (`tasksOnDay`;
  ADR-010 S0 finding F3,
  [PR #142](https://github.com/draconianneocoder-cpu/GoPMgr/pull/142)).
  Removing that rescan (PS-04 slice RES-A) needed a reference that does not
  move with the code under change.
- **Authorization:** the maintainer's 2026-10-26 professional-scheduling
  decision ([PR #152](https://github.com/draconianneocoder-cpu/GoPMgr/pull/152),
  not yet merged) approves "developing independent fixtures, reference
  oracles, benchmarks and isolated feasibility prototypes". The maintainer
  approved the RES-A-1 production change on 2026-10-27 (DEC-09, carrying
  DEC-02; [owner decisions](../review/PS-04-owner-decisions-2026-10-27.md)),
  limited to a compatibility-preserving change with no RES-B redesign and no
  scheduling-semantic change.
- **Design sources:** the PS-04 design package received 2026-10-27, which is
  not in this repository. This page uses its IDs (RES-C, RES-A, F-xx).

| Document | SHA-256 |
| --- | --- |
| PS04-ARCH-001, architecture and requirements baseline | `e35cdf31249cacac2e1837515b4913122455020ace7a4cff38e90f4cd23eeaa6` |
| PS04-RES-002, resource overallocation contract and sparse algorithm | `ff24aa7527d33200c4c26549231c0bbd03efb3de0f96d8260ef0a12ed56ffa72` |
| PS04-QUAL-005, unified acceptance matrix | `10f7a307d99b26f23b3c62a4be08736fe6347b40cc8623def04001aacf86a052` |
| PS04-EXEC-006, implementation sequence and agent directive | `5914465d26ee77f7e3b3b087b0f93ed13199904c073648c812a7429c504f848b` |

## Current behavior

- **Records.** One `Overallocation` per resource and integer project day
  on which, at some moment, the units in use exceed the smallest capacity
  among the assignments in use at that moment (time intervals, 2026-11-23;
  see the change below). `Demand` is the day's total, which can be at or
  below `Capacity` when the breach is an overlap within the day.
  `Capacity` is the smallest capacity that day among the participants'
  assignments to the resource. Records are ordered by resource, then day.
  `TaskIDs` holds the printed `Task.ID` of every participating map entry,
  sorted, never truncated: the entries in use at an overloaded moment.
  When every participant covers the whole day (every day of a whole-day
  schedule), this is the day-total comparison `demand <= capacity + 1e-9`
  of earlier versions.
- **Empty result.** No conflicts, including an empty or nil task map,
  returns a nil slice, which marshals to `null`.
- **Flags.** Every input task's `Overallocated` is cleared first, then set on
  each participant. Detection writes nothing else.
- **Demand.** Each assignment occurrence adds its effective units times the
  share of the day the task uses (see Days): `Units <= 0` counts as 1, and
  a positive `MaxUnits` caps the result. Assignments with a
  blank resource are skipped. A task with two assignments to one resource
  adds demand twice but is listed once. Each day's demand is summed from
  zero for that day; it is not carried from the day before.
- **Days.** A task with `Duration > 0` occupies `floor(ES + 1e-9)` through
  `ceil(EF - 1e-9) - 1`, raised to the first day when lower: the days its
  work falls in, the rule the schedule uses to show it. On each of those
  days its share is `max(0, min(EF, d+1) - max(ES, d))`: 1 for a day it
  spans whole, the fraction used on a day it starts or ends part-way
  through, and never negative. A non-empty `WorkDays` replaces that span,
  in any order, and each listed day counts whole (share 1), including for
  a zero-duration task; an empty list, nil or not, means the span.
  Detection does not reorder `WorkDays`. Days before 0 add no demand.
- **Horizon.** Every profile has length (last occupied span day + 1) over all
  tasks with `Duration > 0`, including tasks with no assignments. Split
  days never extend it, and split days outside `[0, horizon)` add no
  demand.
- **Capacity.** Each step replaces the one before it:
  1. the plan default (1 when not positive);
  2. a positive `Capacities[resource]`;
  3. from the calendar named after the resource: a positive default;
  4. a nonnegative `WeeklyCapacity[day mod 7]`;
  5. a nonnegative `Overrides[day]`.

  Since 2026-11-23 (D-CAL) detection and levelling both read an
  assignment's capacity from the calendar its `CalendarID` names (steps 3
  to 5 on that calendar, from the resource's base capacity), and from the
  resource's own calendar when it names none or one that does not exist.
  The frozen copy reads the resource's calendar only.

### Contract change, 2026-11-21 (owner decision)

Until 2026-11-21 a part day counted as a whole one, and a part-day start
counted on the next day: a span ran from `round(ES)` to `ceil(EF) - 1`,
and every day in it took the full units (OR-01-S4: a task from 0.5 to 0.9
counted on day 1, whole). The schedule shows that task on day 0, so its
overallocation badge and histogram sat one day after its bar. The owner
chose to count the fraction used ("count the used fraction"): demand now
lands on the days shown, weighted by the share of each day used, so two
half-day tasks on one person in one day are not an overload. Whole-day
schedules and split tasks are unchanged. The frozen copy (OR-02) was not
edited and keeps the old answers; the hand oracle gives the two rows whose
answer changed (F-20, OR-01-S4) separate expectations for production and
for the frozen copy, and adds production-only rows (OR-01-S4b to S4f).

Counting shares alone let a task use more of a resource at once than it
has whenever the task straddled a day boundary: two people for half of
each of two days on one person totals 1 per day. On 2026-11-23 the owner
chose to treat units as a rate ("always a breach"): an assignment whose
units exceed the day's capacity is a breach on every day the task works
part of, and levelling never places it there (it is reported unplaced).
Whole days and split days already add the full units to the day's total,
so only part days change; a full-time task on a part-time resource stays
flagged, as before 2026-11-21. The rule is per assignment: demand is
counted per day and tasks are not sequenced within a day, so two part-day
tasks may run at the same time on one person when their day's total
fits. Levelling's rate check reads the assignment's calendar and
detection's the resource's, the same divergence as F-09.
Resource levelling books the same shares (part-day levelling, also
2026-11-21), so the leveller and the detector agree. The rate rule and
the day-total rule for part days were superseded on 2026-11-23 (below).

### Contract change, 2026-11-23 (owner decisions D-TIME and D-CAL)

The day-total rule let two part-day tasks run at the same moment on one
person while their day's total fitted. The owner compared it with a rule
over time and chose time intervals (D-TIME, "time intervals"), with the
standing direction that GoPMgr's planning should be more accurate and
precise than comparable programs. A contiguous task uses each assignment's
effective units over `[ES, EF)` on the working-time axis, shrunk by `1e-9`
at both ends so summing noise (0.1 + 0.2 against 0.3) is not an overlap; a
split task uses them over each listed day whole, as often as the day is
listed. A day is a breach when, at some moment, the units in use exceed
the smallest capacity among the assignments in use then; NaN or infinite
units in use, or a NaN capacity, make that moment a breach. The rate rule
is the case of one assignment alone. Two half-day tasks at the same
moment now breach (T3-1), one after the other do not (OR-01-S4c), and only
the tasks in use at an overloaded moment take part (T3-2).

The owner also chose one calendar rule (D-CAL, "assignment calendar"):
detection, like levelling, reads an assignment's capacity from the
calendar it names, and from the resource's own when it names none or one
that does not exist (T3-7). F-09 is resolved: its row keeps the frozen
copy's answer and has a separate production expectation, and
`TestDetectorAndLevelerReadTheSameCalendar` replaces the two-sided
tripwire.

Levelling applies the same rule. It books whole-day totals per day (with
the smallest capacity among them) and lists part-day pieces separately,
checks a start against the shared overload sweep, sums a task's
assignments to one resource (before, each was checked alone, so a task
needing 1 + 1 of a one-person resource was reported levelled and then
flagged), and tries the ends of part-day bookings as well as day
boundaries. `TestLevelledScheduleIsOverallocatedOnlyByUnplacedTasks`
holds the agreement: after levelling, every record names an unplaced
task. Whole-day schedules without assignment calendars give the same
records and the same levelled dates as before.

### Anomalies: current output, normative answer not decided

| Fixture | Current behavior | Status |
| --- | --- | --- |
| F-09 | Detection read the resource's calendar; leveling read the assignment's | Resolved 2026-11-23 (D-CAL): both read the assignment's calendar |
| F-10 | A repeated split day counts demand twice, participant once | Malformed input; policy pending (DEC-05) |
| F-11 | A split day past the span horizon is dropped, so a possible conflict on that day is never reported | Known defect candidate; a fix needs its own change request |
| F-15, F-19 | Two entries with one printed ID, or two keys aliasing one task, both appear in `TaskIDs` | Identity validation pending |
| F-16 | A zero-duration task with `WorkDays` adds demand inside other tasks' horizon but cannot extend it | Malformed input; policy pending |
| F-17 | A far task with no assignments makes every profile as long as the schedule (73,000 days in the test, the most the kernel allocates for since the 50-year schedule limit of 2026-11-10; 1,000,000 before it) | Dense-horizon cost; sparse design (RES-B) scope |
| F-18 | `Units = NaN` produces a record that `encoding/json` cannot marshal | Non-finite input; validation pending |
| F-21 | A negative split day adds no demand but still creates the resource's profile | Malformed input; policy pending |

A faster detector must reproduce every row. Changing one is a separate,
approved behavior change with its own tests.

## Oracles

| Oracle | File | What it proves |
| --- | --- | --- |
| Hand oracle (OR-01) | `internal/kernel/overallocation_contract_test.go` | Hand-computed records, flags and usage. It covers F-01 to F-11 and F-15 to F-21, plus supplementary cases OR-01-S1 to S5 and, since 2026-11-21, production-only part-day cases OR-01-S4b to S4f (a day-long task straddling two days, two half-day tasks sharing a day, a trailing half day, a sliver below a day boundary, units above capacity on a part day), and since 2026-11-23 production-only time-interval cases T3-1 to T3-7 (two half-day tasks at the same moment, only the overlapping two of three taking part, summing noise at a boundary, assignment calendars compared moment by moment, the smaller calendar binding an overlap, NaN units overloading only their own moments, an unknown calendar reading the resource's). F-09, F-20 and OR-01-S4 carry separate production and frozen-copy expectations. It also includes the F-07 threshold, a 16-day capacity-resolution probe (OR-01-S3) and the F-17 horizon, all run against both production and the frozen copy. The JSON-shape, repeated-call and F-09 calendar tests run against production only; the frozen copy has no capacity-map wrapper or leveler. |
| Frozen legacy reference (OR-02) | `internal/kernel/overallocation_legacy_ref_test.go` | A copy of the detection path at the pinned blob, written as free functions. |
| Differential test | `internal/kernel/overallocation_differential_test.go` | Production and the frozen copy agree on 2,000 seeded random whole-day schedules (whole-day starts and durations since 2026-11-21, and no assignment calendars since 2026-11-23, where both contracts agree), and production leaves its inputs unchanged. A parser guard allows the copy to call only its own functions, builtins, `math` and `sort`. A hash pin fails when the copy changes. |
| Part-day grid property | `TestPartDayDemandMatchesAnEighthDayGrid` in the differential file | On 2,000 seeded schedules whose starts and durations are multiples of 1/8 day (from day -1), each resource's usage equals an independent count of eighth-day cells per day times effective units, exactly. |
| Time-interval grid property | `TestDetectionMatchesAnEighthDayGrid` in the differential file | On 2,000 seeded eighth-day schedules with two assignment calendars, an unknown one and a resource calendar, the records (days, demand, capacity, participants) and flags equal an independent count of overloaded eighth-day cells, with capacities stated by the test, exactly. About 3,900 breaches, about 1,000 of them with the day's total within capacity. |
| Levelling agreement | `TestLevelledScheduleIsOverallocatedOnlyByUnplacedTasks` in `resources_test.go` | On 600 random part-day schedules with links, assignment calendars, splitting and both strategies, every record detection finds after levelling names an unplaced task. |
| Participant index | `internal/kernel/overallocation_index_test.go` | A contiguous span is one membership run however long; split days become sorted, deduplicated, merged runs; repeated assignments and aliased keys give one run per entry and resource. Also T2 and T3 benchmarks. |
| Benchmarks | `internal/kernel/overallocation_bench_test.go` | Timing and allocations at T0 and T1, run before and after RES-A-1. |

The frozen copy's SHA-256, with CRLF normalized to LF, is
`68807af1bde66fe8805827809be5fcab9e723f40825acb01967706776905dc88`.
`TestLegacyReferenceIsFrozen` checks it.

**Numeric policy.** `Demand` and `Capacity` are compared bit for bit; no
tolerance is used anywhere.

- F-02's 0.4 + 0.5 + 0.3 and OR-01-S5's 0.1 + 0.2 do not depend on map
  order: every possible order gives the same binary64 sum, and the test
  checks that.
- OR-01-S5 also shows that each day's demand is summed from zero: a running
  sum would leave 0.20000000000000004 on its last day instead of 0.2.
- The random schedules use multiples of 1/8, so any summation order is exact.
  They cannot tell how demand is summed; OR-01-S5 does that. The part-day
  grid property uses multiples of 1/8 day and of 1/8 unit, so every share
  and product is exact and its comparison needs no tolerance either.
- F-07 checks the threshold itself and the values one ULP either side.
- NOT RUN: the design's tolerance-compared sums whose binary64 value depends
  on map order, and threshold cases whose classification depends on it.
  No such fixture exists yet.

The differential test also fails if the generator stops producing any of
these: duplicate IDs, aliases, split gaps, weekly days past day 6, zero
capacity or empty `WorkDays` lists. (Assignment calendars left the
generator on 2026-11-23: production reads them and the frozen copy does
not.)

F-12, F-13, F-14 and F-22 cover the sparse design (RES-B) and are not
implemented here.

## Requirement status

"Pinned" means the legacy behavior is captured by the tests named. The
RES-A-1 column is the result of running those tests, unchanged, against
the new detector locally on darwin/arm64 with Go 1.27.3, at base
`31cac48`. A local run is not a release PASS (PS04-EXEC-006 §6.3); CI runs
the same tests on Linux.

| ID | Legacy behavior pinned by | RES-A-1 |
| --- | --- | --- |
| RES-A01 API, JSON and nil shape | Signature pins, `TestOverallocationJSONShape`, F-16 (empty and nil maps) | Passing locally |
| RES-A02 ordering and flag clearing | F-01, F-08, OR-01-S2, `TestOverallocationRepeatCallClearsFlags` | Passing locally |
| RES-A03 repeated assignments, identity, demand summation | F-02, F-15, OR-01-S5 | Passing locally |
| RES-A04 effective units | F-03 (both cases) | Passing locally |
| RES-A05 split gaps, duplicates, clipping | F-04, OR-01-S1 (unsorted days), F-10, F-11, F-16 (empty list) | Passing locally |
| RES-A06 capacity resolution | F-05, F-06, F-09, OR-01-S3, T3-4, T3-5, T3-7 | Passing locally (assignment calendars read since 2026-11-23) |
| RES-A07 threshold | `TestOverallocationThresholdIsLiteral` (F-07) | Passing locally |
| RES-A08 performance | Paired before-and-after benchmarks at T0 and T1; T2 and T3 for the new detector | Measured (see Benchmarks). RSS and the old detector's T2 and T3 NOT RUN; budgets BLOCKED (OWN-10) |
| RES-A09 aliases, unassigned horizon, part days | F-17, F-19, F-20, OR-01-S4, OR-01-S4b to S4f, T3-1 to T3-6, both grid properties | Passing locally (part-day rule changed 2026-11-21, time intervals 2026-11-23) |
| RES-A10 literal comparison, zero demand | F-18, F-21 | Passing locally |

## Sensitivity

Each seed changed one thing in a disposable worktree, ran the tests, and
was restored byte for byte.

### Seeds against the participant index (RES-A-1)

All 14 seeds were caught by the full suite.

- **Membership runs:**
  - split days not deduplicated;
  - adjacent split days not merged;
  - a span ending one day early;
  - `WorkDays != nil` in place of `len(WorkDays) > 0`.
- **Entries and identity:**
  - entries collapsed by printed ID;
  - entries ordered by key instead of printed ID.
- **Membership and output:**
  - membership given for resources the task is not assigned to;
  - membership applied one day late;
  - the active set not reset between resources;
  - participants left unsorted;
  - flags not set.
- **Run with the differential test alone, five times each:**
  - the missing adjacency merge, which only the differential test catches;
  - the missing deduplication.
- **Negative split days not clipped:** caught only by
  `TestOccupiedRuns`. It is equivalent at the detector's output, because a
  run before day 0 enters and leaves before day 0 is examined.

### Seeds against the original detector

**Production mutations: 27 distinct, all caught by the full suite.**

- Demand, membership and output (16):
  - participant multiplicity;
  - deduplicated printed IDs;
  - participants taken from the contiguous span;
  - unsorted participants;
  - reversed resource order;
  - uncleared flags;
  - deduplicated split demand;
  - a horizon grown by split days;
  - a horizon from assigned tasks only;
  - counting blank resources;
  - a running sum in place of per-day sums;
  - `WorkDays` sorted in place;
  - an empty `WorkDays` treated as a split, in demand and in membership
    (two seeds);
  - a stale participant cache keyed by resource and day;
  - a renamed JSON tag.
- Comparison and result shape (3): threshold `<` instead of `<=`;
  `!(d > t)` in place of `d <= t`, which only NaN distinguishes; and an
  empty slice instead of nil.
- Capacity resolution (6):
  - ignored overrides;
  - a weekly phase without `mod 7`;
  - weekly and override precedence swapped;
  - an ignored calendar default;
  - an accepted negative base;
  - detection switched to the assignment calendar.
- Effective units (2): a dropped `MaxUnits` cap, and zero units kept as zero.

**Restricted run (3 seeds).** Run with only the differential test, it caught
the two effective-units seeds and participants taken from the span. F-03
also catches the first two.

**Checks on the suite itself (4 seeds).**

- A tampered expected `TaskID` fails.
- A tampered frozen copy fails the hash pin, the differential test, and the
  hand oracle's legacy run.
- The parser guard fails when the copy calls a production method.
- It also fails when the copy calls a test helper from another file.

**Against the original kernel tests only.** Each of the 27 production
mutations was run ten times in an untouched worktree, and 22 passed every
run. Five were caught:

- ignored overrides, by the calendar-override and leveling tests;
- the dropped cap, by `TestResourceUsageCapsAssignmentAtMaxUnits`;
- zero units kept as zero, by `TestResourceUsageProfile` and others;
- unsorted participants, by `TestDetectOverallocations`, but only when map
  order happens to put B before A;
- the stale cache, by `TestDetectOverallocations`, but only on its second
  run in one process.

### Seeds against the part-day rule (2026-11-21)

Run on linux/amd64 with Go 1.27.4; all caught:

- a whole day counted for every occupied day (share 1): the hand oracle
  and the grid property;
- `round(ES)` restored as the first day: the hand oracle and the grid
  property;
- the share not clamped at 0: OR-01-S4e;
- the rate rule removed from detection: F-20, OR-01-S4, S4b, S4d, S4f;
- the rate rule applied on a share-0 day: OR-01-S4e;
- the rate rule removed from levelling:
  `TestLevelResourcesUnplaceablePartDayStartStays`.

### Seeds against the time-interval rule (2026-11-23)

Run on linux/amd64 with Go 1.27.4; all caught:

- Detection:
  - no separate count of non-finite units: T3-6;
  - no `1e-9` shrink at occupancy ends: T3-3 (with a runtime sum; the
    constant 0.1+0.2 is exact in Go and hid it);
  - every member of the day taken as a participant: T3-2;
  - the day's smallest capacity in place of the moment's: T3-4;
  - the day-total rule on every day: F-20, OR-01-S4, S4b, S4d;
  - the resource's capacity in place of the assignment's: F-09, T3-5 and
    the time-interval grid property.
- Levelling:
  - booked capacities ignored: the levelling agreement property;
  - part-day pieces ignored: the part-day levelling cases;
  - only the last of a task's assignments to a resource counted: the
    levelling agreement property;
  - booking ends not tried as starts: the part-day levelling cases.
- One seed was not detectable and was not kept as a test: putting starts
  before ends at equal times changes only the summation order, because
  zero-length segments are skipped and the shrink keeps abutting tasks
  apart.

## Benchmarks (indicative)

```sh
go test -run '^$' -bench 'ResourceUsage|DetectOverallocations' -benchmem ./internal/kernel/
```

The fixture is the benchmark files' own seeded schedule:

- Tasks of 1 to 15 days spread over 400 days, with one resource per 40
  tasks.
- 15% of tasks have a second assignment.
- Units are 0.5 or 1.

It is not the S0 generator, and these numbers are not comparable with S0's.

**Setup:** an Apple M4 (darwin/arm64) with Go 1.27.3, on 2026-10-27.
- The before and after runs alternated six times. The "before" side was a
  worktree of `main` at `31cac48`.
- Times are benchstat medians with the spread around them, n=6.
- Breaches, task references and allocations are deterministic for the
  fixture, and are identical before and after.

| Tier | Detector before | Detector after | Change | Bytes per call, after | Allocations, after |
| --- | --- | --- | --- | --- | --- |
| T0, 500 tasks | 4.77 ms ± 4% | 0.233 ms ± 2% | −95.1% | 474 KiB (+1.1%) | 5,173 (−29%) |
| T1, 10,000 tasks | 2.05 s ± 2% | 6.16 ms ± 1% | −99.7% | 10.7 MiB (+0.9%) | 100,814 (−29%) |
| T2, 50,000 tasks | not run | 45.9 ms ± 2% | | 55.8 MiB | 503,705 |
| T3, 100,000 tasks | not run | 99.4 ms ± 1% | | 111 MiB | 1,006,482 |

- T0 has 1,029 breaches and 2,420 task references; T1 has 19,569 and
  47,785; T2 has 97,759 and 238,540; T3 has 194,586 and 477,841.
- `ResourceUsage` is unchanged (T1 about 0.74 ms both sides).
- The remaining cost follows the dense demand profiles and the size of the
  output, which are RES-B's subject.
- Peak RSS was not measured, and the old detector was not run at T2 or T3.
- These figures guide design only; they are not budgets.

**Time intervals, 2026-11-23** (linux/amd64 container, 4 CPUs, Go 1.27.4;
three runs each, ranges shown; indicative, not comparable with the M4
figures above). Before is `a113db8`, after is the time-interval detector:

| Tier | Before | After | Bytes per call, after | Allocations, after |
| --- | --- | --- | --- | --- |
| T0 | 0.91–1.06 ms | 0.83–0.96 ms | 486 KB (−7%) | 5,174 |
| T1 | 27.8–31.0 ms | 23.7–26.4 ms | 11.2 MB (−7%) | 100,815 |
| T2 | 185–208 ms | 160–184 ms | 58.5 MB (−7%) | 503,706 |
| T3 | 397–478 ms | 377–428 ms | 116.8 MB (−7%) | 1,006,483 |

Breaches and task references are unchanged at every tier (whole-day
fixtures take the day-total path). `BenchmarkDetectOverallocationsPartDay`
runs the same fixture in eighths of a day, so most days take the sweep:
T0 1.42–1.60 ms, 506 KB, 1,102 breaches; T1 34.3–35.7 ms, 11.6 MB, 20,990
breaches. `BenchmarkLevelResources` (default horizon): whole-day T1
4.03–4.04 s and 17.0 MB against 3.73–4.27 s and 21.4 MB before (the
ready-queue scan dominates); whole-day T0 21–27 ms against 12–13 ms,
because four fixture tasks that assign one resource twice for more than
it has are now unplaced and search the whole horizon; part-day T1
3.9–4.0 s and 19.6 MB.

## Changing these tests

- Do not edit the frozen reference to follow production. If the detector's
  output is meant to change, record the decision here and update the hand
  oracle row by row. The 2026-11-21 part-day change kept the frozen copy
  and its hash pin: rows whose answer changed carry separate production
  and frozen-copy expectations, and the differential test is limited to
  the inputs on which both contracts agree. The 2026-11-23 time-interval
  and assignment-calendar change did the same.
- A faster detector passes only if every test here passes unchanged, the
  differential test included. RES-A-1 did: it removed `tasksOnDay` and
  `occupiesDay` from production, and the frozen copy keeps its own versions.
