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
  where `demand <= capacity + 1e-9` is false. Records are ordered by
  resource, then day. `TaskIDs` holds the printed `Task.ID` of every
  participating map entry, sorted, never truncated.
- **Empty result.** No conflicts, including an empty or nil task map,
  returns a nil slice, which marshals to `null`.
- **Flags.** Every input task's `Overallocated` is cleared first, then set on
  each participant. Detection writes nothing else.
- **Demand.** Each assignment occurrence adds its effective units: `Units <= 0`
  counts as 1, and a positive `MaxUnits` caps the result. Assignments with a
  blank resource are skipped. A task with two assignments to one resource
  adds demand twice but is listed once. Each day's demand is summed from
  zero for that day; it is not carried from the day before.
- **Days.** A task with `Duration > 0` occupies `round(ES)` through
  `ceil(EF) - 1`, raised to the first day when lower. A non-empty `WorkDays`
  replaces that span, in any order; an empty list, nil or not, means the
  span. Detection does not reorder `WorkDays`.
- **Horizon.** Every profile has length (last occupied span day + 1) over all
  tasks, including tasks with no assignments. Split days outside
  `[0, horizon)` add no demand.
- **Capacity.** Each step replaces the one before it:
  1. the plan default (1 when not positive);
  2. a positive `Capacities[resource]`;
  3. from the calendar named after the resource: a positive default;
  4. a nonnegative `WeeklyCapacity[day mod 7]`;
  5. a nonnegative `Overrides[day]`.

  Detection ignores `Assignment.CalendarID`; leveling honours it.

### Anomalies: current output, normative answer not decided

| Fixture | Current behavior | Status |
| --- | --- | --- |
| F-09 | Detection reads the resource's calendar; leveling reads the assignment's | Divergence; harmonizing it needs a semantic decision |
| F-10 | A repeated split day counts demand twice, participant once | Malformed input; policy pending (DEC-05) |
| F-11 | A split day past the span horizon is dropped, so a possible conflict on that day is never reported | Known defect candidate; a fix needs its own change request |
| F-15, F-19 | Two entries with one printed ID, or two keys aliasing one task, both appear in `TaskIDs` | Identity validation pending |
| F-16 | A zero-duration task with `WorkDays` adds demand inside other tasks' horizon but cannot extend it | Malformed input; policy pending |
| F-17 | A far task with no assignments makes every profile 1,000,000 days long | Dense-horizon cost; sparse design (RES-B) scope |
| F-18 | `Units = NaN` produces a record that `encoding/json` cannot marshal | Non-finite input; validation pending |
| F-21 | A negative split day adds no demand but still creates the resource's profile | Malformed input; policy pending |

A faster detector must reproduce every row. Changing one is a separate,
approved behavior change with its own tests.

## Oracles

| Oracle | File | What it proves |
| --- | --- | --- |
| Hand oracle (OR-01) | `internal/kernel/overallocation_contract_test.go` | Hand-computed records, flags and usage. It covers F-01 to F-11 and F-15 to F-21, plus five supplementary cases (OR-01-S1 to S5). It also includes the F-07 threshold, a 16-day capacity-resolution probe (OR-01-S3) and the F-17 horizon, all run against both production and the frozen copy. The JSON-shape, repeated-call and two-sided F-09 tests run against production only; the frozen copy has no capacity-map wrapper or leveler. |
| Frozen legacy reference (OR-02) | `internal/kernel/overallocation_legacy_ref_test.go` | A copy of the detection path at the pinned blob, written as free functions. |
| Differential test | `internal/kernel/overallocation_differential_test.go` | Production and the frozen copy agree on 2,000 seeded random schedules, and production leaves its inputs unchanged. A parser guard allows the copy to call only its own functions, builtins, `math` and `sort`. A hash pin fails when the copy changes. |
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
  They cannot tell how demand is summed; OR-01-S5 does that.
- F-07 checks the threshold itself and the values one ULP either side.
- NOT RUN: the design's tolerance-compared sums whose binary64 value depends
  on map order, and threshold cases whose classification depends on it.
  No such fixture exists yet.

The differential test also fails if the generator stops producing any of
these: duplicate IDs, aliases, split gaps, weekly days past day 6, zero
capacity, assignment calendars or empty `WorkDays` lists.

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
| RES-A06 capacity resolution | F-05, F-06, F-09, OR-01-S3 | Passing locally |
| RES-A07 threshold | `TestOverallocationThresholdIsLiteral` (F-07) | Passing locally |
| RES-A08 performance | Paired before-and-after benchmarks at T0 and T1; T2 and T3 for the new detector | Measured (see Benchmarks). RSS and the old detector's T2 and T3 NOT RUN; budgets BLOCKED (OWN-10) |
| RES-A09 aliases, unassigned horizon, rounding | F-17, F-19, F-20, OR-01-S4 | Passing locally |
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

## Changing these tests

- Do not edit the frozen reference to follow production. If the detector's
  output is meant to change, record the decision here, update the hand
  oracle row by row, and update the hash pin in the same change.
- A faster detector passes only if every test here passes unchanged, the
  differential test included. RES-A-1 did: it removed `tasksOnDay` and
  `occupiesDay` from production, and the frozen copy keeps its own versions.
