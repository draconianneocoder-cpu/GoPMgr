<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Work-item timestamp boundary, 2026-10-16

## Defect and repair

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

## Regression evidence

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

## Native evidence

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

## Limits and follow-up

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
