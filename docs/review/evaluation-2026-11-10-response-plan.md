<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Response plan for the 2026-11-10 independent evaluation

**Date:** 2026-11-11.
**Evaluation:** [independent-evaluation-2026-11-10.md](independent-evaluation-2026-11-10.md),
filed unchanged (SHA-256 `fefa7c44ff2777584fc8517a1e4fe32c96e6620cebb522347e9839b02e64a507`).
It is an independent assessment, not an accepted decision. It inspected `main`
at `1ca5ffa` by reading; it built and ran nothing.
**This plan** records which findings were checked against the repository, the
owner's decisions of 2026-11-11, and the order of work. It authorizes only
what the decisions below name. Current status lives in
[STATUS.md](../STATUS.md).

## 1. Owner decisions, 2026-11-11

| Evaluation item | Decision |
| --- | --- |
| PS-02 slice 1 | **Authorized.** Implement the read-only schedule-quality audit as specified in [PS-02](../testing/PS-02-schedule-quality-audit-spec.md). Later slices stay unauthorized. |
| DEC-SECRET (SEC-03, NEW-07) | **Exclude the signing certificate file from backups by default.** It is included only when the user asks for it when creating the backup. |
| DEC-UNITS (NEW-03) | **Keep the limit as 18,250 working days.** On any calendar that is at least 50 years: about 70 on a Monday-to-Friday calendar, exactly 50 if every day is worked. Documents must say "working days". |
| DEC-ROADMAP (NEW-06) | **Accepted.** Keep a current-status register ([STATUS.md](../STATUS.md)) and store the evaluation in the repository. Historical records are not rewritten. |
| Go toolchain and zen-go | **Go 1.27.4 required; `gorules/zen-go` removed** (pull request 199). Not an evaluation item; decided the same day. |

Still undecided, and blocking the work named beside each:

| Item | Blocks |
| --- | --- |
| DEC-AUDIT (ADR-011 DEC-04, compact audit bridge) | ARC-03, ARC-04, PS-04 |
| DEC-FORMAT (new file format, conversion, older-writer protection) | ARC-01, ARC-02, PS-01 |
| DEC-HARDWARE (hardware profiles, licensed fixtures, numeric budgets) | ARC-05, VAL-02, PS-00 |
| DEC-INTEROP (G1 to G3 production worker slices) | PS-03 |
| DEC-WINDOWS (support intent and real-host testing) | SEC-05, VAL-01 for Windows |

## 2. Findings checked against the repository

Checked on 2026-11-11 against `main` plus pull request 199.

| Finding | Result |
| --- | --- |
| SEC-01 (PR 199 open) | The pull request now also requires Go 1.27.4 and removes zen-go. Both of its engine flags are tested with negative controls. Closed when it merges. |
| SEC-02 (restored tables trusted) | **Confirmed open.** Restore refuses triggers, views and virtual tables; tables the archive defines itself still pass `CREATE TABLE IF NOT EXISTS`. |
| SEC-03 (private key in backups) | **Confirmed.** `admin.SecureArchive` adds the configured `.p12` to every backup. Restore never imports it, and since PR 198 it clears signing settings, so the bundled key serves only manual recovery. |
| SEC-06 (C advisories) | **Confirmed open.** Nothing compares the vendored SQLite or SQLCipher version with published advisories. |
| SEC-09 (minimatch, deepmerge) | Unchanged; rechecked 2026-11-10 in [DEPENDENCIES.md](../DEPENDENCIES.md). |
| DOC-01 (ARCHITECTURE says Wails v2) | **Confirmed.** Also names `wails build` and a `wails-window.d.ts` file that no longer exists. |
| DOC-02 (plan still routes PS-09 through DataFusion) | **Confirmed** in the PS-09 row and the delivery diagram. |
| DOC-04 (certificate finding marked open) | Already corrected in PR 199. |
| DOC-05 (dashboard R3/R4 row stale) | **Partly wrong.** The backlog records R3/R4 as done in the row after the one quoted. The earlier row's wording is clarified so it cannot be read alone. |
| DOC-07 (20 to 500 tasks versus 50,000) | **Confirmed** as an unstated relationship, not a contradiction. |
| "Go 1.27.3" baseline | Superseded: 1.27.4 since PR 199. |
| Evaluation §2, "50 years" wording | Resolved by DEC-UNITS above. |

Not rechecked here, and taken as stated: ARC-01 to ARC-08, SEC-04, SEC-05,
SEC-07, SEC-08, VAL-01 to VAL-07, DOC-03, DOC-06, DOC-08. Each has a row in
[STATUS.md](../STATUS.md).

## 3. Order of work

Problems first, then roadmap work. Each stage ends in one reviewable pull
request with its own evidence. A stage that needs an undecided item stops
there and asks.

### Stage A: reconcile and secure what exists (no further decision needed)

| Step | Work | Closes |
| --- | --- | --- |
| A1 | This plan, the status register, and corrections to ARCHITECTURE, the modernization plan (as a dated amendment), the backlog row, VISION and the limit's wording | DOC-01, DOC-02, DOC-05, DOC-07, DEC-UNITS, DEC-ROADMAP |
| A2 | Backups exclude the signing certificate unless the user opts in | SEC-03, NEW-07 |
| A3 | Restore accepts only the table definitions GoPMgr itself has created, checked read-only before anything is written; hostile fixtures | SEC-02, start of NEW-01 |
| A4 | A check that fails when the vendored SQLite or SQLCipher version has a published advisory or is no longer the reviewed one | SEC-06 |
| A5 | Triage of the 16 low-severity scan items: each proved or refuted, with a disposition | SEC-04 |

### Stage B: the authorized product slice

| Step | Work | Closes |
| --- | --- | --- |
| B1 | PS-02 slice 1: kernel audit rules with independent expected results, one bound read-only API, one findings panel | PS-02 slice 1 |

### Stage C: needs decisions before it starts

| Work | Needs |
| --- | --- |
| ADR-011 design completion: schema, identity, units, migration and recovery matrices (evaluation stage B) | DEC-FORMAT, DEC-AUDIT |
| Inactive storage, format gate, compact audit, typed commands (stages C and D) | The design above, then per-slice approval |
| 50,000-activity qualification (stage E) | DEC-HARDWARE and the stages above |
| MPXJ worker slices and import fidelity (stage F) | DEC-INTEROP |
| Windows qualification of the engine and installer | DEC-WINDOWS and a Windows host |
| Native qualification of Wails v3 on macOS 26 and Ubuntu 26.04, signing, notarization, update channel | Release hosts and signing material, not code |

### Evaluation proposals not taken up now

NEW-02 (operation and provenance envelope) and NEW-09 (SQLite feature policy)
belong to the ADR-011 design. NEW-04 (power-loss laboratory), NEW-05
(diagnostic bundle), NEW-08 (export fidelity corpus) and NEW-10 (mutation
drills as a programme) are recorded in [STATUS.md](../STATUS.md) as proposed.
Fault seeding of every new guard is already the project's practice.

## 4. Rules for this work

- Nothing here activates a schema, migrates a file, changes scheduling
  results or changes release or signing policy.
- Every new guard is fault-seeded: the test must fail with the guard removed.
- A status in [STATUS.md](../STATUS.md) changes only with a commit or evidence
  record to point at.
