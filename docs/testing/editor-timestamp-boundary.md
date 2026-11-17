<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Editor timestamp boundary

One timestamp-boundary contract, proven for two editors.

Each section below is a record kept as written, with its headings one
level lower and its links updated; nothing else changed.

## What this file holds

- [Work-item timestamp boundary, 2026-10-16](#work-item-timestamp-boundary-2026-10-16): formerly `docs/testing/work-item-timestamp-boundary.md`, 2026-10-16, from commit `9ab5d91`.
- [Sprint timestamp boundary, 2026-10-18](#sprint-timestamp-boundary-2026-10-18): formerly `docs/testing/sprint-timestamp-boundary.md`, 2026-10-18, from commit `9ab5d91`.

## Work-item timestamp boundary, 2026-10-16

### Defect and repair

A production macOS build from `873b92d198a82a2480b9aba8409b8c7a8e835ddf`
refused a new Backlog work item before `App.SaveWorkItem` ran. The native
error was `error parsing arguments: parsing time "" ... cannot parse "" as
"2006"`. Backlog and Kanban initialize `created_at` and `updated_at` with
empty strings; the shared editor forwarded them to Go `time.Time` fields.
The component tests' accepting bridge mocks concealed the incompatibility.

`WorkItemEditor` now maps only empty timestamp strings to Go's canonical
zero value, `0001-01-01T00:00:00Z`, in a separate outbound payload. The
submitted draft snapshot is unchanged, preserving late-edit detection.
Nonempty values, including fractional seconds and offsets, are forwarded
unchanged. Optional `closed_at` stays absent when omitted; an explicitly
empty value becomes zero. Go's parser, method signatures, database schema,
server-owned timestamps and existing completion semantics are unchanged.
No migration or dependency change is required.

### Regression evidence

- Before the repair, three new component cases failed: manual save, guarded
  save and mixed empty/supplied timestamps. The same cases passed afterward.
- Five added component cases cover both save paths, unchanged input props,
  supplied fractional/offset timestamps, optional omission, and malformed
  timestamp rejection retaining the draft and pending navigation without
  successful-save or close callbacks. Existing late-edit tests still pass.
- `TestWorkItemTimestampJSON` exercises the actual Go DTO: canonical zero
  and omitted closure retain zero semantics; supplied timestamps retain their
  instant; each of the three fields independently rejects empty and malformed
  strings. It does not pin parser error wording or relax validation.

Focused frontend verification passed 40 tests across the editor, Backlog,
Kanban and native-close tests. Focused Go JSON/App round-trip tests passed
under the race detector. `make verify` passed, including all 81 frontend test
files / 611 tests, ordinary Go tests, type checking, frontend build and
code-map currency. `make build` produced the local production app.
`make frontend-smoke lint-all license-check release-scope` passed, including
REUSE coverage of 963 files. `make coverage-ledger-drift` passed: all 46
package headings matched and all 49 ledger headings were accounted for.
Svelte analysis found no concrete issues; its generic lifecycle/effect
suggestions did not justify unrelated changes.
Independent read-only adversarial review inspected all seven changed files
and four native screenshots and returned no findings. It did not rerun the
tests or independently observe the process exits or database inspections.

### Native evidence

Host: macOS 27.0.1 (26A434), arm64, Go 1.27.2 with CGO. Both builds were
local and ad-hoc signed, not trusted-distribution or installer evidence.
Executable SHA-256 before repair:
`d903c499a8b4d912b3573a383bb5b75f926cbe69ad0ff03a81d926a953302f12`.
After repair:
`a86dd3bd9a331fa3fd8f6a1f8c1617ab4603aae9a9881aa393ec92d7978b49a9`.

All trials used an explicitly approved disposable account/project. Initial
launch and retained-profile re-entry passed the protective sandbox preflight;
normal and legacy project roots remained denied. Re-entry verified canonical
paths, ownership, private permissions and the exact executable digest. Only
the owned app received native input. No real account or project was opened.

| Step | Observed outcome |
| --- | --- |
| 1. Original Backlog creation | Save displayed the bridge timestamp error and retained the draft. Read-only inspection found zero work items. |
| 2. Repaired Backlog creation | Save closed the editor and displayed the exact creation marker. Read-only inspection found one generated work-item ID and the exact title. |
| 3. Update through native Cmd+Q | Editing that record and choosing Save and continue saved the exact update marker and exited status 0. |
| 4. Same-profile relaunch | Authentication and reopening the project showed the update marker. Read-only inspection retained the same ID, one work item and the unchanged stakeholder. Clean quit also exited status 0. |

Inspection used the repository SQLCipher driver with `mode=ro` and query-only
connections, a fresh read transaction, and the exact disposable project ID.
It authenticated the disposable password with existing crypto code and
unwrapped its key without logging secrets. It did not run migrations or mutate
account credentials. Temporary inspection/re-entry helpers remain private and
are not shipped.

Native window captures were saved and inspected: `03-work-item-bridge-error`,
`04-native-create-success`, `05-native-guarded-update`, and
`06-native-update-relaunched` (PNG). The automation's returned screenshot bytes
were blank/partial and rejected; the user authorized macOS window-ID capture.
Only captures matching the verified owned window and current UI were accepted.

### Limits and follow-up

This confirms Backlog creation and one update/relaunch path through the real
bridge. Kanban's shared editor path is component-tested, not separately native
verified. It is not full P0 editor-data-loss closure: native Discard and real
backend-failure/retry cases for both editors remain unverified. Disabled input
controls were not bypassed to claim native mid-save editing coverage.

The existing editor displays a zero closure timestamp as `closed at 0001-01-01`;
that presentation issue is deferred, not a changed storage/completion rule.
Existing Wails `time.Time` binding diagnostics, macOS 13 object / 11 link-target
warnings and test-environment canvas notices remain. No older-macOS,
Windows/Linux GUI, native-C race, crash/power-loss, accessibility compliance,
trusted-signature, hosted-CI or release-approval claim follows from this slice.
The full all-package race/PDF/signing release gate was not repeated for this
frontend repair; the affected Go boundary/App tests ran under the race
detector, and no production Go, PDF, signing or storage code changed.

Lesson: typed backend method tests and accepting UI mocks can both pass while
the serialized desktop boundary rejects a value. Verify that boundary in the
real application and assert exact persisted identities and values after exit.

## Sprint timestamp boundary, 2026-10-18

### Defect and repair

`SprintList.openNew` initializes `created_at` to an empty string. Its shared
manual/guarded save forwarded that value into `agile.Sprint.CreatedAt`, a Go
`time.Time`. Wails v2.16.0's `BoundMethod.ParseArgs` uses `json.Unmarshal`,
which rejected the payload before `App.SaveSprint` ran. Accepting frontend
mocks and typed App tests concealed the mismatch.

The editor now converts only an empty `created_at` to
`0001-01-01T00:00:00Z` in a separate outbound object, following the existing
work-item convention. The dirty snapshot and late-edit comparison keep the
original draft. Nonempty timestamps pass through byte-for-byte, including
malformed values that Go must continue to reject. No parser, API, schema,
identity, default, activation/completion, dependency or visual change is made.
The database still owns creation timestamps; accepting an input timestamp
does not mean persisting that timestamp. No migration is needed.

### Automated evidence

The original 19 sprint component tests passed before this task. Five added
cases cover exact manual/guarded new-sprint payloads, realistic generated-ID
responses, clean closure, transient rejection and explicit retry, retained
fields/dirty state/pending quit, unchanged supplied timestamps and source
objects, and malformed-input rejection without a quit permit. The three
empty-timestamp payload cases failed against the original implementation and
passed after the repair. Expected payloads are independently constructed
under a fixed clock. Existing late-edit/rebase tests still pass.

`TestSprintTimestampJSON` independently checks the real Go DTO: zero,
omission and null retain zero semantics; a fractional-offset timestamp keeps
its instant; empty and malformed strings are rejected. The App round-trip
now starts from decoded JSON, checks all fields, backend-generated ID,
open-project ownership and nonzero creation time, then verifies an update
retains that identity and database timestamp. These Go contract tests passed
before the frontend fix; they are not themselves red/green repair evidence.

Current local checks passed:

- Focused sprint/native-close component tests: 34; full frontend: 81 files,
  617 tests, including the browser-storage compatibility run.
- `go test -count=1 . ./internal/... ./tools/...`; focused JSON/App tests
  under `-race`, including three shuffled iterations.
- `go vet . ./internal/... ./tools/...`, `make lint-all` (zero Go issues),
  warning-clean Svelte checks, official Svelte analyzer (no issues/suggestions).
- `make build`, independent strict bundle signature verification,
  `make frontend-smoke` (607 bytes), code-map currency, ledger inventory,
  release-scope checks and `go mod verify`.
- Coverage-ledger drift: all 46 package headings match, all 49 headings
  accounted for.
- `reuse lint`: 975/975 files have licensing/copyright information;
  task-diff secret scans found no leaks.
- `make verify`: configuration, packaging/toolchain/font/native-launch/reset,
  cleanup/hygiene, Go tests, code-map, frontend stability/build budgets,
  inventory and timestamp/import static guards passed.

The read-only adversarial review examined all seven changed files, the four
native captures and persistence snapshots, and returned no findings. The
reviewer independently reran the 34 focused frontend tests, Go decoder/App
tests, Svelte analysis and whitespace checks; broad gates and native trials
were inspected, not rerun. An additional focused adversarial run passed all
six timestamp/late-edit cases, challenging strict malformed passthrough,
failed-save quit blocking, false late edits and preservation of server fields.

### Native evidence

Host: macOS 27.0.1 arm64, Go 1.27.2 with CGO. Local production bundles were
ad-hoc signed, not trusted-distribution artifacts. Baseline source was
`1695743281ac1c5ae0e4b05f84dca7c7fbe9023c`. Executable SHA-256:

- Before repair: `a334d8f40154d71b55bbca7a39e5f7bc93c851318a52cb4bf67dcffa2fb02fa4`.
- Repaired candidate: `47dc95ee2157380cd6b903489161fb957a36b542f4ebacf3ef7ada9c14f8f035`.

The approved fresh disposable account and blank project used the repository's
protected native launcher. Normal and legacy project roots were sandbox-denied;
retained-profile re-entry verified canonical private paths, ownership,
permissions, protective preflight and the exact executable digest. All account,
project, draft, save and quit interactions used native automation. No normal
account/project or `/Applications` installation was involved.

| Trial | Observed result |
| --- | --- |
| Baseline manual creation | Exact timestamp argument error displayed; draft retained. Cmd+Q Discard exited status 0. Fresh read-only inspection found zero sprints. |
| Repaired manual creation | Editor closed with “Sprint saved.” Exactly one generated ID, `sprint_4a06ad11`, and predefined fields were stored. |
| Distinct new sprint via Cmd+Q | Real Unsaved changes dialog appeared; Save and continue exited status 0 and created `sprint_60241aee`. Exactly two sprints existed. |
| Same-profile relaunch | Same-account authentication and reopening the project showed both markers. Full read-only record snapshots matched before/after relaunch and after final clean quit (status 0). No test app remained running. |

Project identity was `prj_0b2722c0`. Both records retained planning status and
dates `2026-10-18` through `2026-11-01`. Manual name/goal were
`Native sprint manual 1018` / `Exact manual goal 1018`, capacity `3.5`,
creation time `2026-10-18T20:08:27.978Z`. Guarded name/goal were
`Native sprint guarded 1018` / `Exact guarded goal 1018`, capacity `8.25`,
creation time `2026-10-18T20:09:15.480Z`. Independent assertions checked all
nine fields and exact unfiltered counts, not just visible names. Card rounding
to `8.3` did not change the editor's or database's exact `8.25`.

The inspector used the repository SQLCipher driver, read-only/query-only
connections and a fresh read transaction, authenticated only the disposable
account and unwrapped its key without logging secrets. It ran no migrations
or credential updates. Profiles, credentials, inspection/re-entry helpers and
raw captures remain private and are not shipped. Native window-ID captures
`01-baseline-rejection`, `02-manual-success`, `03-guarded-new`, and
`04-relaunch` were inspected against the owned window and observed UI.
Automation occasionally delivered partial typed input; values were corrected
and verified before saving, not treated as application persistence defects.

### Lessons and limits

A permissive mocked bridge can report success for JSON that never reaches the
real method. Pair exact component payload assertions with independent DTO
decoding and a native bridge trial. Preserve transport conversion outside the
snapshot used to detect late edits.

The bounded UX check confirmed a clear successful-save state and an explicit
native quit choice. The baseline error appears behind the editor overlay;
this task repairs the creation failure, not error presentation. No broad UX
research, accessibility-compliance or chart-accuracy claim is made.

Native failed-save/retry, Cancel/Discard on the repaired new-sprint flow,
mid-save timing, loss of a successful response, disk faults, crash/power loss,
older macOS and Linux/Windows GUI behavior remain unverified here. Automated
failure/retry and late-edit coverage is not native fault-injection evidence.
Multi-save sprint activation remains separate. Existing `time.Time` binding
diagnostics, macOS 13 object / 11 link-target warnings and jsdom canvas notices
remain; no diagnostics were suppressed.
Full release/PDF/trusted-signing/update/installer gates and hosted checks were
not run for this frontend-only repair. This is not complete P0 editor-data-loss
closure or authorization to tag/publish a release.
