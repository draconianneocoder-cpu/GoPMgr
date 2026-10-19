<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Saved-sprint native update recovery, 2026-10-19

Real locked updates retained the complete draft and blocked Cmd+Q. Cancel
re-armed the guard; Discard preserved the original saved row through restart.
A separate explicit retry updated exactly one row without changing its ID,
project, status or creation time. This closes the saved-update contention
evidence gap, not the whole editor-data-loss or release matrix. No production
repair was needed; the existing atomic save contract remains unchanged.

## Candidate and safety

Runtime source: `9185f40e1e5be63834185d2b08c462aef93f2186`; subsequent changes
are tests/documentation only. `make build` and strict deep signature verification
passed. Executable SHA-256:
`47dc95ee2157380cd6b903489161fb957a36b542f4ebacf3ef7ada9c14f8f035`.
Host: macOS 27.0.1 (26A434), arm64, Go 1.27.2/CGO, Wails 2.16.0. The ad-hoc
signature is not distribution trust. Existing Wails `time.Time` and deployment
target warnings remain; older macOS support is not established.

The fresh disposable account/project used the protected native launcher and
private HOME/XDG/TMP/CFFIXED paths. Re-entry checked canonical paths, ownership,
permissions, executable digest and denial preflight for normal/legacy data
roots. Account, project, edit, save and quit interactions used native automation;
the recovery export stayed in the fresh HOME. No `/Applications` or installer
receipt changes were made. Helpers, credentials, databases and raw captures
remain private. No normal-profile audit or untouched-macOS-cache claim is made.

The [prior observer-triggered launch incident](native-sprint-recovery-2026-10-18.md)
informed the safeguard: every Cmd+Q, guarded Save and Discard was an action-only
call, followed by process checks before any further UI observation. Protected
PID, executable and all four environment overrides were checked. Expected exits
were verified from child status, logs and PID disappearance, never a post-quit
app observation. No outside-profile launch or forced termination was observed
in this run. No test app or locker remained running at completion.

Inspection used fresh read-only/query-only transactions against the actual
SQLCipher file/current WAL, with no migration, copied snapshot or credential
mutation. All nine sprint fields and unfiltered counts were checked; errors
were not empty results. WAL/SHM sidecars may be created. The separate locker
used fixture reads, `BEGIN IMMEDIATE` and `ROLLBACK`, without DML. Its bounds
were acquisition 5 seconds, hold 120 seconds, rollback 3 seconds and watchdog
130 seconds. Both real locks were acknowledged, explicitly released and joined
with status 0 and `LOCK_ROLLED_BACK`; expiry was not accepted as success.

## Independent record oracle

Project: `prj_8405062d`. The baseline was created through the UI and pinned
before editing. Exactly one sprint existed throughout the saved-update trials.

| Field | Baseline | Explicit retry |
| --- | --- | --- |
| ID | `sprint_0c04af01` | unchanged |
| Project | `prj_8405062d` | unchanged |
| Name | `Saved sprint 1018` | `Updated sprint 1018` |
| Goal | `Original saved goal 1018` | `Updated exact goal 1018` |
| Status | `planning` | unchanged |
| Start | `2026-10-19` | `2026-10-20` |
| End | `2026-11-02` | `2026-11-03` |
| Capacity | `5.25` | `13.75` |
| Created at | `2026-10-19T11:53:02.231Z` | unchanged |

| Acceptance criterion | Observed result |
| --- | --- |
| Dirty saved update, Cmd+Q Cancel | All five edited fields retained; saved row unchanged; next Cmd+Q prompted again. |
| First locked guarded Save | Fresh `write sprint: database is locked` error and pending guard; draft intact; full baseline unchanged while locked. |
| Release without retry | Rollback/join succeeded; full baseline still unchanged. |
| Cancel after failure, re-arm, Discard | All fields retained until explicit Discard; status-0 quit; full baseline unchanged afterward and after protected relaunch. |
| Second locked update | Fresh busy error after a restart with no old error; full baseline unchanged while locked and after release. |
| Explicit retry | Status-0 quit; one exact updated row with original ID, project, status and creation time. |
| Relaunch and final clean quit | Updated card and reopened editor matched all five values; complete row snapshot unchanged through final status-0 quit. |
| False-positive oracles | Wrong project, saved ID, timestamp, zero/one expectation and baseline/update expectation were rejected; 2-second lock expiry rolled back and failed. |

Save entry is supported by newly displayed backend errors absent before each
attempt, not merely a targeted click. The transient Saving/disabled-control
state was not captured in either trial and is not claimed as native evidence.
Capture files `01-saved-baseline.png` through `09-exact-updated-fields.png`
were saved and opened against the current owned window. A partial Cancel
capture was rejected and recaptured. Card rounding (5.3/13.8) did not change
exact editor/database values (5.25/13.75). Comparisons cover sprint rows, not
every project table. Host logs recorded Discard at 06:56:10.131600, retry quit
at 06:58:59.404881 and final quit at 07:00:34.654161 CDT on October 19.

## Automated evidence, findings and limits

The weak saved-retry component case, which checked call counts and a name with
an empty creation timestamp, now covers retry and Discard after failure,
Cancel and re-arming. It uses a valid historical timestamp, exact immutable
nine-field payloads, all five retained inputs, no premature quit/success/clean
state, no extra save/delete on Discard, and unchanged source/other-row fixtures.
These mock assertions are not database preservation evidence. A false-success
catch-return-true mutation failed both cases at the retained-dirty assertion;
production source was restored and all 36 focused tests passed. SprintList
now has 26 tests; existing late-edit and in-flight-save tests remain intact.

Passed locally in this task: uncached Go tests for root/internal/tools; targeted
root native-close/DTO/App race tests; all 16 sprint atomicity fault cases under
race; 619 frontend tests across 81 files; warning-clean Svelte checks; Go vet;
Go/frontend lint; official Svelte analysis; build/signature verification;
isolated-launcher tests; runtime smoke (607 bytes rendered); module integrity;
REUSE lint; release-scope checks; and `make verify`. Three shuffled focused runs
(seeds 20261019, 20261020, 20261021) each passed all 36 tests.

UX follow-up: a prior `Sprint saved` message persisted behind the first failure
guard; after restart the second failure had no stale success text. The precise
error remains behind the modal, whose copy says leaving a view rather than
quitting. These are existing feedback issues, deferred from this persistence
assurance slice. No contrast, focus trapping/restoration, screen-reader or zoom
assessment was performed. Screenshots do not certify accessibility.

Not covered: response loss after commit, crash/power loss, arbitrary disk/I/O
faults, other quit triggers as separate trials, activation/completion's multiple
saves, older macOS, Linux/Windows GUI or native-C races. Installation, trusted
signing, full dependency-advisory/PDF/release matrices and hosted CI were not
rerun because no runtime, export, dependency or packaging contract changed.
No API/schema/default/migration change or architecture/code-map edit was needed.

Lesson: strengthen an existing weak test instead of duplicating it; pin the
original identity/time independently; separate retained draft state from durable
state; and use process checks between quit-capable actions and UI observations.
