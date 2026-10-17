<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Native editor Discard and failed-save recovery, 2026-10-16

Four saved-record cases passed in the production macOS app: Cmd+Q Discard
and a real blocked-write failure followed by explicit retry, separately in
`StakeholderManager` and Backlog's `WorkItemEditor`. No production change was
needed. This extends the [October 12 checks](native-close-macos-2026-10-12.md)
and [timestamp-boundary repair](work-item-timestamp-boundary.md); it does not
close the entire editor-data-loss or release-readiness backlog.

## Build and isolation

Source: `c8f09f8f8e0f63ef296d7f9265e5ec3d17de2e63`, clean tracked worktree.
Host: macOS 27.0.1 (26A434), arm64, Go 1.27.2, CGO enabled, Wails 2.16.0.
`make build` passed. Local ad-hoc-signed executable SHA-256:
`a86dd3bd9a331fa3fd8f6a1f8c1617ab4603aae9a9881aa393ec92d7978b49a9`.
This is neither installer lifecycle nor trusted-distribution evidence.

The explicitly approved disposable account/project was reused. Each launch
checked the exact executable digest and retained profile's ownership, private
permissions, canonical paths and absence of symlinks, then reran its sandbox
preflight. Normal and legacy GoPMgr/PMForge data and Documents roots remained
denied. No real account/project was opened. The sandbox is bounded project-root
isolation, not proof of zero WebKit cache or preference writes elsewhere.

Only the verified owned process received native input through the existing
automation. Fresh accessibility snapshots supplied control indices; real text
input events updated bindings. The user-approved macOS window-ID capture was
restricted to that process's verified window. Each saved PNG was opened and
matched against the current UI before acceptance. No historical image was used.

## Failure injection and persisted-state checks

A private helper used the repository SQLCipher driver without `InitDB`, project
opening services, migrations or credential updates. It authenticated only the
disposable account through a read-only system connection, unwrapped its key
without logging secrets, and checked the exact project identity. Inspection
used `mode=ro`, query-only connections and a fresh read transaction over the
unfiltered stakeholder and work-item lists, including their project ownership.

Lock mode used `mode=rw`, one dedicated connection and `BEGIN IMMEDIATE`, with
no row changes. It acknowledged acquisition before the native save attempt.
The app's existing 5000 ms busy timeout produced a real `database is locked`
error. After observing failure, inspection confirmed the baseline while the
lock was still held, before release or retry. Explicit release rolled back;
the helper exited successfully before retry. A 120-second deadline invalidated
a trial rather than treating expiry as successful release. Both real trial
locks were released before that deadline and exited status 0.

Helper checks also demonstrated second-writer refusal, non-successful deadline
expiry with cleanup, and successful explicit release. An initial read-only
inspection overlapped a rebuild and emitted a missing-binary warning; it was
excluded. The private re-entry checks were changed to fail explicitly, and a
deliberately wrong digest was refused before inspection/launch. All accepted
trials ran after build completion. These helpers, keys and profiles are private
test material, not shipped hooks or committed artifacts.

## Observed flow

Counts remained one stakeholder and one work item in project `prj_6f5de68b`.
Their IDs remained `stk_75e2c5b2` and `wi_d271721b` throughout all four cases.
Inspection compared those IDs, project ownership, name/title and list counts.
Other columns were not compared; preservation claims below cover only these
inspected fields, not whole-row equivalence.

| Step | Screen / action | Result and health |
| --- | --- | --- |
| 1 | Stakeholder changed to `Discard stakeholder 1016-A`; native Cmd+Q | Healthy: explicit Save/Discard/Cancel dialog retained the draft. Capture `01-stakeholder-discard.png`. |
| 2 | Choose Discard; authenticate and reopen the same project | Passed: status-0 exit; original `Baseline stakeholder 1016` returned, with work-item title `Native bridge update 1016` unchanged. Capture `02-stakeholder-discard-relaunched.png`. |
| 3 | Fresh stakeholder edit `Retry stakeholder 1016-B`; lock, Cmd+Q, Save and continue | Passed failure handling: real backend error, app/draft/quit dialog retained, saved name still the baseline before retry. Capture `03-stakeholder-save-failed.png`. |
| 4 | Release lock, explicitly retry, relaunch | Passed: status-0 exit; exact retry name under the original ID, same counts and unchanged work-item ID, ownership and title. Capture `04-stakeholder-retry-relaunched.png`. |
| 5 | Work item changed to `Discard work item 1016-C`; native Cmd+Q | Healthy: explicit Save/Discard/Cancel dialog retained the draft. Capture `05-work-item-discard.png`. |
| 6 | Choose Discard; authenticate and reopen Backlog | Passed: status-0 exit; original `Native bridge update 1016` returned, with stakeholder `Retry stakeholder 1016-B` unchanged. Capture `06-work-item-discard-relaunched.png`. |
| 7 | Fresh work-item edit `Retry work item 1016-D`; lock, Cmd+Q, Save and continue | Passed failure handling: real backend error, app/draft/quit dialog retained, saved title still the baseline before retry. Capture `07-work-item-save-failed.png`. |
| 8 | Release lock, explicitly retry, relaunch | Passed: status-0 exit; exact retry title under the original ID, same counts and unchanged stakeholder ID, ownership and name. Capture `08-work-item-retry-relaunched.png`. |

Persistence was checked immediately after each exit, after each authenticated
relaunch, and after the final clean status-0 quit. Neither Discard marker was
persisted. Both explicit retries updated the original row, not a duplicate.
No task app or writer-lock process was left running.

## UX observations and evidence limits

The guard offers distinct Cancel, Discard and Save choices. Failed saves remain
visible and retain the retry action instead of dismissing work. The specific
backend error sits behind the dimmed confirmation, while the confirmation says
only that saving failed; the preserved draft and explicit retry were usable,
but the layered error detail is harder to read. This is an observed clarity
limitation, not a demonstrated data-loss defect. Screenshots do not establish
screen-reader announcements, focus trapping, contrast compliance or full
keyboard accessibility; none is claimed.

`npm --prefix frontend test -- src/lib/native-close.test.ts
src/lib/components/project/StakeholderManager.test.ts
src/lib/components/agile/WorkItemEditor.test.ts` passed 53 tests.
`go test -race . -run 'Test(NativeCloseGuard|CompleteNativeClose)' -count=1`
and `bash scripts/launch-isolated-native_test.sh` passed. Svelte analysis found
no concrete issues; existing lifecycle/effect suggestions did not justify
unrelated refactoring. Licensing, release-scope and secret checks passed for
the documentation change. No new component or Go test was necessary because no
production behavior changed; these native trials supply the missing evidence.

Not tested: disk-full, I/O faults, crash/power loss, native mid-save editing,
new-record quit, Kanban as a separate entry point, older macOS, Linux/Windows
GUI, native-C races, installation/upgrade/removal, signing/notarization or
hosted CI. Disabled controls were not bypassed to claim native late-edit proof.
The full release gate was not rerun for this evidence-only change. The existing
Wails `time.Time` diagnostics, macOS 13 object / 11 link-target warning and
year-1 closure-date presentation remain outside this slice.

Lesson: a validation refusal is not backend-failure evidence. Keep the writer
lock acknowledged through the visible failure, inspect saved data before retry,
and verify identities and counts after relaunch, not just the displayed label.
