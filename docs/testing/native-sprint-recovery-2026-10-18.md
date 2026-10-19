<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Native new-sprint quit recovery, 2026-10-18

Confirmed Cmd+Q saves against a real SQLCipher writer lock retained the new
sprint draft and pending quit without creating a row. Explicit retry after
release created exactly one record. A second failed new draft was cancelled,
then discarded without changing that saved record. No production repair was
needed. This is bounded contention evidence, not release approval.

## Candidate and method

Runtime source: `4de3eacff7f4067e1c867b8fa070f61b9aadef93`. Subsequent changes
are tests/documentation only. Executable SHA-256:
`47dc95ee2157380cd6b903489161fb957a36b542f4ebacf3ef7ada9c14f8f035`.
Host: macOS 27.0.1 (26A434), arm64, Go 1.27.2 with CGO, Wails 2.16.0.
`make build` and independent strict deep signature verification passed.
The bundle is ad-hoc signed; existing Wails `time.Time` and macOS deployment
target warnings do not establish trusted signing or older-OS compatibility.

A fresh disposable account/project used the protected native launcher with
private HOME/XDG/TMP/CFFIXED paths. Re-entry pinned the binary digest, canonical
paths, ownership/permissions and sandbox preflight denying normal and legacy
GoPMgr/PMForge data roots. Account/project/edit/quit interactions used native
automation. Recovery export stayed in the fresh HOME. No `/Applications`
installation or receipt was changed. Raw captures, helpers, credentials and
databases remain private rather than becoming release artifacts.

**Isolation incident:** after the first successful Discard, a following UI
observation unexpectedly found a different process with normal HOME, without
the protected launch overrides. The protected child had exited with status 0.
Automatic activation by the observer is the suspected mechanism, not a proven
cause. The new copy showed sign-in; no account was signed into or project
opened. It was closed, testing stopped, and the user approved safer continuation.
Normal startup may have written metadata/logs; normal data roots cannot be
claimed untouched, and no normal-profile audit was authorized or performed.
All subsequent launches were through the protected harness with process
environment verification before UI actions. Final quit actions had no following
UI observation: child status, logs and PID disappearance proved exit instead.
No further unexpected launch was observed; no app remained running at the end.

Inspection used fresh read-only/query-only transactions against the actual
SQLCipher database/current WAL, not a copy or immutable snapshot. Unfiltered
counts, project ownership and all nine sprint fields were checked. Errors were
not interpreted as empty results. No migrations or credential mutations ran.
WAL/SHM sidecars may be created, so whole-profile byte equality is not claimed.

The independent locker used fixture identity reads, `BEGIN IMMEDIATE` and
`ROLLBACK`, with no DML. Bounds were acquisition 5 seconds, hold 120 seconds,
rollback 3 seconds, process watchdog 130 seconds; expiry invalidated a trial.
Both real contention trials required `LOCK_HELD`, observed disabled controls
and `Saving…`, then the real `database is locked` error with both processes
alive. Each lock was explicitly released and joined with status 0 and
`LOCK_ROLLED_BACK`. No forced application termination was used.

## Native results

| Criterion / counterexample | Result |
| --- | --- |
| Empty project, dirty draft, Cmd+Q Cancel | Guard appeared; name, goal, dates and capacity remained; zero rows. |
| Re-armed Cmd+Q Discard | Protected child exited status 0; zero rows afterward and after protected relaunch. |
| Real failed new-sprint Save | Error and pending guard remained; every draft field retained; zero rows while locked. |
| Lock release without retry | Rollback/join succeeded; fresh count still zero, guard still waiting. |
| Explicit Save and continue retry | Child exited status 0; exactly one generated record matched the independent oracle below. |
| Protected relaunch | Same account/project showed the saved marker; full record snapshot matched exactly. |
| Second distinct new draft, real locked Save | Failed draft retained; existing record snapshot unchanged while locked and after release. |
| Cancel after failure, re-arm, Discard | Cancel retained all fields; second Cmd+Q prompted again; Discard exited status 0 with only the unchanged baseline row. |
| Final protected relaunch and clean quit | Only the saved sprint was visible; exact snapshot unchanged before/after final status-0 quit. |
| Inspector/locker false positives | Wrong project, wrong saved ID and incompatible zero/one counts rejected; 2-second lock-expiry probe rolled back and failed as required. |

The independent successful-record oracle was:

| Field | Value |
| --- | --- |
| Project / generated ID | `prj_dfa647e7` / `sprint_dafa1285` |
| Name / goal | `Sprint retry 1018` / `Exact retained retry goal 1018` |
| Status | `planning` |
| Start / end date | `2026-10-19` / `2026-11-02` |
| Capacity | `8.5` |
| Created at | `2026-10-19T01:54:20.945Z` |

Identity was checked as backend-generated and creation time as nonzero/recent,
then both were pinned with every other field through later snapshots. The
discarded markers were `Cancelled sprint 1018` and `Discard sprint 1018`;
neither persisted. Dates follow the observed UTC-derived defaults, not the
host's October 18 local date. Snapshot comparisons cover sprint rows, not every
database table. Clean exits after retry, failed-draft Discard and final quit
were logged at 20:54:20.955939, 20:55:55.564603 and 20:56:40.928431 CDT.

Eight current-run window captures were saved and opened. Key captures are
`04-retry-locked-failure.png`, `05-retry-persisted.png`,
`06-discard-locked-failure.png`, `07-cancel-after-failure.png` and
`08-final-baseline.png`; owned process/window identity was checked before capture.

## Automated evidence and remaining limits

One new `SprintList.test.ts` case checks failed new-sprint quit Save followed
by Cancel, re-arm and Discard: exact normalized payload, retained fields/dirty
state, no premature quit permit, no second save/delete and unchanged supplied
existing-record object. Native row inspections, not that mock assertion, prove
database preservation. A false-success mutation (`save` catch returning true)
failed at the no-quit-permit assertion; production source was restored and
the focused 35 tests passed again. SprintList now has 25 tests.

Passed in this task: uncached `go test -count=1 . ./internal/... ./tools/...`;
targeted timestamp/App/native-close tests under `-race`; 618 frontend tests
across 81 files; warning-clean Svelte checks; Go vet; Go/frontend lint;
official Svelte analyzer; isolated-launcher tests; build/signature verification;
runtime smoke (App rendered); REUSE lint; and `make verify` (configuration,
packaging/toolchain/font/native-launch/reset/cleanup, Go tests, code-map,
frontend stability/build budgets, ledger inventory and static guards).
`go mod verify` and `make release-scope` also passed. Three shuffled focused
runs (seeds 1018, 1019, 1020) each passed all 35 tests. The independent
read-only adversarial review examined the complete change, private helper
sources/logs/snapshots and all eight captures, returning no findings. It did
not replay native trials or rerun the broad test suites.

Visual findings: the guard exposes failure and explicit retry, Cancel retains
the complete draft, and the final view shows only the saved record. The exact
database error remains behind the modal, and guard copy says leaving a view
rather than quitting. Focus trapping/restoration, screen-reader behavior and
contrast were not verified; screenshots are not accessibility certification.

Not tested: saved-sprint update contention, other native quit triggers as
separate trials, post-write/readback-response loss, crash/power loss, arbitrary
disk/I/O faults, older macOS, Linux/Windows GUI or native-C races. Installation,
trusted signing, the complete PDF/security/release matrices and hosted CI were
not repeated for this test/evidence slice. The broader editor-data-loss backlog
remains open. No API, schema, migration, defaults, identity or runtime change
was justified; no architecture/code-map update was necessary.

Lesson: prove Save entry and fresh row counts before retry, pin all returned
fields after success, and never observe a closed app through an automation API
that can activate it outside the protected launcher.
