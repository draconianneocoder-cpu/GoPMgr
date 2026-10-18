<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Native new-record quit evidence, 2026-10-18

Cancel, Discard and Save passed through native Cmd+Q while a new Backlog
work item had no saved ID. Cancel retained an editable draft, Discard created
no record, and Save created exactly one record that survived relaunch with
the same identity and checked fields. No production repair was needed.
This closes the new-record-quit evidence gap for this macOS Backlog path,
not the entire editor-data-loss or release-readiness backlog.

## Candidate and isolation

Source: `8d5280a6ed4de52402ae1ca1dbf8ca5d03bc1a37`, clean tracked worktree.
Host: macOS 27.0.1 (26A434), arm64, Go 1.27.2 with CGO, Wails 2.16.0.
`make build` and `codesign --verify --deep --strict` passed. Executable
SHA-256: `a86dd3bd9a331fa3fd8f6a1f8c1617ab4603aae9a9881aa393ec92d7978b49a9`.
This is a local ad-hoc-signed build, not installer or trusted-signing evidence.
The build repeated the existing Wails `Not found: time.Time` diagnostics and
macOS 13 object/macOS 11 link-target warning; neither was suppressed or fixed.

The owner explicitly approved a fresh protected disposable account/project
and isolated-window capture. The existing isolation launcher passed its
preflight and denied normal and legacy GoPMgr/PMForge data roots. A new
account and blank project were created through the UI; the Software-Dev Pack
was enabled only in that project. No normal account/project, `/Applications`
bundle or installer receipt was modified. Isolation is bounded project-data
protection, not proof of zero unrelated WebKit cache or preference writes.

The recovery export dialog remembered a previous test destination. Before
saving, that destination was replaced with the new private profile's home
and its parent menu was checked. The resulting file was task-owned, mode
`0600`, inside that profile. Credentials, recovery material, profiles and
inspection helpers remain private and are not committed.

Each re-entry checked canonical non-symlink paths, ownership, private
permissions and the exact executable digest, then reran the protective
preflight. Native actions used fresh accessibility observations of the
verified process/window. Accepted window-only PNGs were opened and matched
against the current UI before use. The trials began October 17; the saved
record was reopened October 18 after the app had exited, without rebuilding
or changing the fixture.

## Method and acceptance checks

The read-only inspector used the repository SQLCipher driver and existing
password/key-unwrapping routines, not application opening services or
migrations. Accepted checkpoints ran under the protective sandbox. Every
observation opened read-only/query-only connections and a fresh transaction
against the actual database and current WAL. No immutable mode or copied
database was used. SQLCipher inspection can create empty WAL/SHM sidecars;
no byte-for-byte whole-profile preservation claim is made.

Queries inspected the unfiltered work-item list and required exact counts
and project ownership. Query, scan and iteration failures were errors, not
empty results. Project identity was pinned after initial fixture enrollment.
Subsequent saved-record observations also pinned the generated work-item ID.
The helper refused deliberately wrong project IDs, wrong saved IDs and a
zero-record expectation after Save. Inspections had a 20-second deadline;
dialog/exit deadlines were 30 seconds. All quit dialogs appeared and all
requested exits completed within one second, with actual child status 0
and PID disappearance checked before re-entry. No forced termination was used.

The Cancel/Discard draft used a distinct title/description, task type, high
priority, points field `08` (entered as 8), and assignee `fixture-owner`.
All field displays survived Cancel; changing its description afterward
proved editing still worked. Backlog state and no sprint were explicit defaults.

The saved record had these predefined values, independently asserted after
Save, relaunch, UI reopening and final clean quit:

| Field | Expected value |
| --- | --- |
| Project / generated work-item ID | `prj_0cbbc23f` / `wi_d6a06313` |
| Title | `Guarded new item 1017-B` |
| Description | `Saved by native quit 1017-B` |
| Type / priority / points | `bug` / `urgent` / `13` |
| Assignee | `native-fixture` |
| State / sprint | `backlog` / empty |

Only those record fields and counts were compared, not every database column.

| Step | Outcome and screenshot |
| --- | --- |
| 1. Empty Backlog | Passed: UI empty; fresh protected inspection found zero work items. `01-empty-backlog.png`. |
| 2. New unsaved draft, native Cmd+Q | Passed: zero records after typing and again with the confirmation open; Cancel/Discard/Save choices visible. `02-new-unsaved-draft.png`, `03-native-quit-dialog.png`. |
| 3. Cancel quit | Passed: confirmation dismissed, every entered field remained, description could be edited, app stayed alive and persisted count remained zero. `04-cancel-retains-editable-draft.png`. |
| 4. Native Cmd+Q, Discard | Passed: zero records before Discard; status-0 exit; zero records immediately afterward. `05-discard-dialog.png`. |
| 5. Same-profile relaunch after Discard | Passed: same account/project reopened with an empty Backlog; fresh unfiltered count zero. `06-discard-relaunched-empty.png`. |
| 6. New Save draft, native Cmd+Q | Passed: predefined fields displayed; zero records both after typing and with the confirmation open, before selecting Save. `07-new-save-draft.png`, `08-native-create-save-dialog.png`. |
| 7. Save and continue | Passed: status-0 exit, exactly one generated work-item ID and all predefined values persisted. |
| 8. Same-profile relaunch after Save | Passed: Backlog showed one item; opening it displayed the same ID and fields; fresh inspection matched. `09-saved-item-relaunched.png`. |
| 9. Final clean quit | Passed: unchanged editor quit without a new dirty prompt, status 0; final inspection retained the same ID, fields and count. No task app remained running. |

Screenshots and raw inspection results are retained privately, not shipped.
Process exit and persistence are supported by process/database observations,
not screenshots alone.

## Automated checks and limits

The focused frontend command passed 40 tests across `native-close.test.ts`,
`WorkItemEditor.test.ts`, `Backlog.test.ts` and `KanbanBoard.test.ts`.
Native-close concurrency/permit, timestamp JSON and App work-item round-trip
tests passed under `go test -race`. The isolation-launcher tests passed.
No new permanent test hook or regression test was needed because production
behavior did not change; native execution supplied the missing evidence.

The confirmation clearly separated the three choices and retained the draft
after Cancel. Its wording says leaving a view rather than quitting the app.
A background control retained a visible focus ring while the confirmation
was open; keyboard trapping, focus restoration and screen-reader behavior
were not tested. These observations are not an accessibility-compliance claim.
The reopened item still displayed the known zero closure date as year 1.

Not tested: new-record backend-failure/retry, Kanban as a separate native
entry point, native mid-save editing, disk-full/arbitrary I/O failures,
crash/power loss, older macOS, Linux/Windows GUI, native-C races, installer
lifecycle, signing/notarization or hosted CI. The full release/PDF/security
matrices were not repeated for this evidence-only change. Earlier
[saved-record failure/retry evidence](native-editor-discard-failure-2026-10-16.md)
does not automatically prove the empty-ID creation case.

Lesson: prove zero records before the native Save choice, then one exact ID
and its predefined values after exit and relaunch. A visible item alone
cannot exclude early persistence, duplicate creation or incorrect ownership.
