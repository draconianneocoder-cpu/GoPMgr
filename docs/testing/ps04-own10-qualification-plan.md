<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PS-04 performance qualification plan (OWN-10)

**Status:** plan. The maintainer approved preparing it on 2026-10-27 ([owner decisions](../review/PS-04-owner-decisions-2026-10-27.md), item 2).

**What it is not:**
- **Not budgets.** Every number in §4 is provisional until representative fixtures and native-platform measurements justify approval.
- **Not results.** Nothing here is a PASS.

**Source:** PS04-QUAL-005 §6–7 (the PS-04 unified acceptance matrix, received 2026-10-27, not in this repository; SHA-256 in [resource-overallocation-contract.md](resource-overallocation-contract.md)). Its measurement rules are adopted below unless marked otherwise.

## 1. Hardware profiles to choose

The owner picks or amends these. Each chosen profile then needs exact models, OS builds and WebView versions recorded.

| Profile | Candidate | Use |
| --- | --- | --- |
| HW-MIN | 4-core current x64, or Apple M1; 8 GB RAM; SSD | Lowest supported machine: memory pressure and usability |
| HW-REC | 8-core current x64, or Apple M2 or later; 16 GB RAM; NVMe | Where the interactive targets apply |
| HW-STRESS | High-memory x64 or ARM64 runner; fast NVMe | T3 and pathological inputs only; never a user-facing claim |
| HW-S0 | 4-vCPU, 16 GB Linux container | Historical S0 comparison only; not desktop evidence |

Development machine used so far (not a candidate profile): Apple M4, 10 cores, 32 GB, macOS 27.0.1.

**Platforms:**
- macOS (WKWebView), Windows x64 (WebView2) and Linux x64 (WebKitGTK), each only where the release officially supports it.
- A platform without a real run is BLOCKED. A cross-compile is not a run.

## 2. What is measured

Each operation has one clock, from the user's action to the correct, complete result. Stage timings (SQL, kernel, Wails bridge, parse, paint) are recorded inside each trace, never added up across operations.

| ID | Operation | Clock stops when |
| --- | --- | --- |
| PERF-OPEN | Open an encrypted project | The format check, decryption and any approved migration are done and the workspace is usable |
| PERF-FIRSTVIEW | First Gantt view (warm, and from a cold launch) | Correct rows and bars are painted, with keyboard focus available |
| PERF-EDIT-TEXT | Edit a name or note | The encrypted commit, audit event and receipt are durable and the row shows as committed |
| PERF-EDIT-SCHED | Edit a duration, link or calendar | The command is durable and the recomputed schedule is visible. Commit and recompute times are also recorded separately |
| PERF-OVERALLOC | Full overallocation detection | Every record, participant ID and flag is produced |
| PERF-CPM | Full CPM | The complete schedule is computed |
| PERF-VP, PERF-PAN | Viewport fetch; pan, zoom and scroll | The requested region is painted |
| PERF-AUDIT | Audit write per edit | The event is durable and verifiable. Bytes per edit are recorded |
| PERF-BACKUP | Backup and restore | The archive is verified, or the restored project reopens |
| PERF-WAL | Mixed reads and edits | WAL growth and checkpoint wait are recorded; integrity verified |
| PERF-IMPORT | Supported full import | The fidelity receipt is written and the first view is valid |
| PERF-EVM | Change the reporting date | The EVM view is correct for the new date |

**Workloads:**
- Task tiers T0 (500), T1 (10,000), T2 (50,000) and T3 (100,000).
- Fixture cohorts:
  - the S0 synthetic generator (seed 20261024);
  - a balanced parallel schedule;
  - a long, sparse horizon;
  - a split- and calendar-heavy schedule;
  - licensed real projects. The real-project cohort is BLOCKED on fixture provenance (DEC-07).

## 3. Sampling and statistics

- **Correctness:** every measured run is checked for correct output first. A fast wrong answer is a failure, not a sample.
- **Frequent operations:** 5 independently launched app sessions × 30 operations each, giving n = 150 per operation, platform and cohort.
- **Cold first view:** at least 30 independent cold launches.
- **p95:**
  - nearest rank over the raw end-to-end samples, with a session-clustered confidence interval;
  - if the interval overlaps the budget, the result is INCONCLUSIVE, not PASS;
  - p99 is descriptive only.
- **Failures and timeouts stay in the denominator.** Completion rate and latency are reported separately.
- **Before-and-after comparisons** run on the same machine with the same toolchain and fixture, with runs interleaved.
- **Slow runs are kept.** One may be excluded only under an infrastructure rule declared before the run, with the record kept.
- **Every raw sample is recorded**, with its trace ID, build SHA, fixture hash, machine, cache state and outcome.

## 4. Provisional targets

These are engineering candidates from PS04-QUAL-005 §6.4. **None is approved.**

| Operation (T2, HW-REC) | Candidate |
| --- | --- |
| PERF-EDIT-TEXT | p95 ≤ 250 ms |
| PERF-EDIT-SCHED | p95 ≤ 1 s to a consistent computed state |
| PERF-FIRSTVIEW | p95 ≤ 1 s |
| PERF-PAN | 30–60 frames per second |
| PERF-OVERALLOC | Under a second on realistic schedules; huge outputs budgeted separately |
| PERF-AUDIT | Bytes per edit grow with the change, not with the schedule |
| PERF-CPM | No regression against the verified baseline |

## 5. Where each operation stands

| Operation | Evidence today | Status |
| --- | --- | --- |
| PERF-OVERALLOC | Kernel benchmark after RES-A-1, on the development machine: T2 45.9 ms, T3 99.4 ms ([contract](resource-overallocation-contract.md)) | Diagnostic only; native end-to-end NOT RUN |
| PERF-CPM | S0: 205 ms at T2 in the S0 container | Diagnostic only |
| PERF-EDIT-TEXT, PERF-AUDIT | S0: 2.5 s p50 and 35.5 MB per edit at T2 | Diagnostic; the normalized store (ADR-011) is not built |
| PERF-FIRSTVIEW, PERF-VP, PERF-PAN | None | NOT RUN; needs a native timing harness |
| PERF-OPEN, PERF-BACKUP, PERF-WAL, PERF-IMPORT, PERF-EVM, PERF-EDIT-SCHED | None | NOT RUN |

**Blocked until:**
- the owner chooses hardware profiles (§1);
- representative fixtures are approved (DEC-07);
- a native timing harness exists, which is an implementation slice needing its own approval.

## 6. Decisions requested

1. Choose or amend HW-MIN and HW-REC, and name the supported platforms and WebViews.
2. Approve the sampling rule in §3, or set a different one.
3. Approve fixture sources for the real-project cohort (DEC-07).

Budgets are set only after measurements on the chosen profiles exist.
