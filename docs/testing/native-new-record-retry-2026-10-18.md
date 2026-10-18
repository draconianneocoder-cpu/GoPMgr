<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Native new-record failed-save/retry evidence, 2026-10-18

A confirmed Save attempt on a new Backlog work item returned a real database
lock error, retained the draft and pending native quit, and created no row.
After lock release, an explicit retry created exactly one item; its identity
and predefined fields survived quit and relaunch. This is bounded contention
evidence, not general retry/idempotence or release approval.

## Candidate and isolation

- Production inputs: `c45c33e2ad7520a980966af899802cb374730802`.
  Only tests and documentation changed afterward; runtime code is unchanged.
- Executable SHA-256:
  `a86dd3bd9a331fa3fd8f6a1f8c1617ab4603aae9a9881aa393ec92d7978b49a9`.
- macOS 27.0.1 (26A434), darwin/arm64, Go 1.27.2, CGO enabled, Wails 2.16.0.
- `make build` and strict deep ad-hoc signature verification passed. Existing
  Wails `time.Time` and macOS object/deployment-target warnings remain; this
  does not establish Developer ID signing, notarization or older-OS support.

The user authorized a fresh protected disposable account/project. The existing
`scripts/launch-isolated-native.sh` created private HOME/XDG/TMP/CFFIXED paths
and verified denial of normal and legacy GoPMgr/PMForge data roots. Re-entry
and inspection repeated that sandbox preflight and pinned the executable
digest and fresh profile. Recovery export stayed inside the new private HOME.
No normal account, `/Applications` app or installer receipt was changed.

Private helpers reused the earlier inspection/lock patterns with every fixture
constant rebound. Inspection used read-only/query-only connections and a fresh
transaction against the real SQLCipher database/current WAL, not a copied or
immutable snapshot or application migration service. Unfiltered counts, project
ownership and the fields below were checked; query errors were not empty results.
SQLCipher may create empty WAL/SHM sidecars, so whole-profile byte equality is
not claimed. Helpers, credentials, databases and raw captures are not shipped.

## Failure and retry procedure

The separate writer-lock mode used a dedicated read/write connection solely
for fixture identity reads, `BEGIN IMMEDIATE` and `ROLLBACK`. Acquisition had a
5-second context, holding a 120-second deadline, rollback a 3-second context,
and the process a 130-second watchdog. The 2-second expiry test rolled back and
returned failure as intended. Inspections had a 20-second whole-process bound.
Wrong project/record IDs and incompatible zero/one expectations were rejected.

1. Fresh project and typed draft: unfiltered count zero. Captures
   `01-empty-backlog.png`, `02-unsaved-draft.png`.
2. Cmd+Q displayed Save/Discard/Cancel. Count remained zero before Save;
   `03-quit-save-choice.png` records this choice on the initial trial.
3. **Initial trial inconclusive:** an indexed Save interaction was followed by
   a clean exit with zero rows while the lock remained held. No Saving state
   or backend error was observed, so neither Save execution nor its cause was
   established. The lock was explicitly rolled back and joined with status 0.
   This trial is not counted as a pass or attributed to a product defect.
4. Same-fixture repeat used a screenshot-grounded Save target. Disabled
   controls and `Saving…` confirmed entry into Save. The real editor error
   `database is locked` then appeared alongside the guard's failure message;
   app and locker remained alive, all fields remained, and fresh count was
   zero. Capture `04-locked-save-failure.png` was saved and inspected while
   the lock was still held.
5. Explicit lock release completed rollback and process join with status 0.
   Another fresh count was zero before retry. The app still awaited retry;
   `05-released-awaiting-retry.png`. No automatic persistence occurred on release.
6. Explicit Save retry exited with status 0 and persisted exactly one item.
   Its generated ID and fields matched again before and during relaunch;
   `06-relaunch-one-item.png`, `07-reopened-saved-fields.png`.
7. Final unchanged-editor Cmd+Q exited with status 0; count, ID and checked
   fields remained unchanged. No forced app termination was used.
8. A third trial checked the indexed-action discrepancy with a distinct new
   draft, `Indexed save probe 1018`, and the saved row pinned as baseline.
   A fresh full accessibility snapshot immediately preceded indexed Save.
   Saving and the real busy error were observed again, with the original
   row unchanged while locked. Capture `08-indexed-save-busy.png` records it.
   Release/rollback/join succeeded; Cancel preserved the failed draft
   (`09-cancel-retains-failed-draft.png`). A new Cmd+Q and explicit Discard
   (`10-probe-discard-choice.png`) exited with status 0, leaving exactly the
   original saved row and fields. This did not reproduce the initial exit
   and does not establish that initial trial's cause.

The confirmed retry was selected at 09:17:59.339 CDT and exited at
09:17:59.689. The later clean quit exited at 09:19:37.999. Exit is supported
by child status, logs and PID disappearance, not screenshots alone. Current
window/PID was verified before capture; every accepted saved PNG was opened.

| Persisted field | Expected value |
| --- | --- |
| Project / generated work-item ID | `prj_a9f62069` / `wi_6a426a0a` |
| Title | `New item retry 1018` |
| Description | `Retained through failed quit Save 1018` |
| Type / priority / points | `bug` / `urgent` / `13` |
| Assignee | `retry-fixture` |
| State / sprint | `backlog` / empty |

Only these fields and counts were compared, not every database column.

## Automated evidence and limits

The new `WorkItemEditor.test.ts` regression checks both empty-ID, normalized
outbound payloads, every edited field, untouched input props, dirty/pending
state and absence of saved/closed callbacks or a quit permit after failure.
Retry must deliver the exact backend-assigned record and permit quit once.
A temporary false-success mutation made this test fail; production source was
restored and the focused 32 tests passed again. An earlier mutation attempt
restored source too early and was invalid, not counted as mutation evidence.

Also passed: `go test . ./internal/...`; targeted native-guard, timestamp and
App round-trip tests under `-race`; all 612 frontend tests across 81 files;
`svelte-check` (zero errors/warnings); targeted test-file ESLint; runtime smoke
(App rendered); and isolated-launcher tests. The environment-performance advice
printed by Vitest is not a test failure or a performance guarantee.

The guard clearly exposes a failure and an explicit retry, but the precise
database error is behind the modal and its wording still says leaving a view,
not quitting. Focus trapping/restoration and screen-reader behavior were not
verified; screenshots do not establish accessibility compliance. The existing
year-1 zero closure-date display is unchanged.

Not tested: post-write/readback-response loss, crash/power loss, general
disk-full/I/O failures, Kanban as a separate native entry, older macOS,
Linux/Windows GUI, native-C races, installation, trusted signing or hosted CI.
The complete release/PDF/security matrices were not repeated for this test and
evidence change. No production repair or architectural change was justified.

Lesson: distinguish a confirmed Save attempt from a targeted click, and prove
zero rows both while failure is held and after release, before retry. Stable
identity begins after successful creation; a failed insert may consume an ID.
