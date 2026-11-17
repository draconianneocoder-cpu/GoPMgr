<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Editor errors in the exit guard

One exit-guard error contract, proven for three editors.

Each section below is a record kept as written, with its headings one
level lower and its links updated; nothing else changed.

## What this file holds

- [Sprint save errors in the foreground guard](#sprint-save-errors-in-the-foreground-guard): formerly `docs/testing/sprint-guard-errors.md`, 2026-10-19, from commit `9ab5d91`.
- [Stakeholder errors in the exit guard](#stakeholder-errors-in-the-exit-guard): formerly `docs/testing/stakeholder-guard-errors.md`, undated, from commit `9ab5d91`.
- [Work-item errors in the exit guard](#work-item-errors-in-the-exit-guard): formerly `docs/testing/work-item-guard-errors.md`, 2026-10-20, from commit `9ab5d91`.

## Sprint save errors in the foreground guard

### Scope and cause

Baseline: `d573e164570f5cd0615bf130ac22ad6fd57e29f4`. The sprint editor
caught validation and backend failures and returned `false`; the coordinator
then substituted “The editor reported that the save failed.” The useful error
remained dimmed behind the foreground Save/Discard/Cancel dialog.

The registered sprint callback now forwards its existing nonempty error
through the coordinator's already-supported exception handling. The manual
boolean save function, generic detail-free refusal, shared guard APIs, native
quit permissions, IDs, timestamps, defaults, schemas and styling are unchanged.
No transaction or backend behavior changes are included.

### Risk-to-test evidence

`SprintList.test.ts` asserts the exact `navigation.error`, not merely text
somewhere in the document. Seven strengthened/new assertions failed against
the baseline generic message. The repaired suite includes:

- Existing `Error` rejections, malformed timestamps and in-app name validation.
- New/existing native-close cases with string rejections as returned by Wails,
  Cancel/re-arm, replacement by the current validation error, retained fields,
  identical nine-field retry payloads and no premature quit or extra mutation.
- A pending manual write plus guarded save attempt: no second write, dirty
  draft retained, generic detail-free refusal preserved, no automatic quit
  after the first write finishes, then explicit continuation without another
  write.

All scoped criteria passed: precise current foreground error, dirty/pending
state retained after refusal, Cancel retains the draft, successful retry clears
the error and permits continuation, and compatibility invariants are preserved.

### Fresh native darwin/arm64 trial, October 19, 2026

Executable SHA-256 values bind the observations to the tested bundles:

- Baseline: `c13e8656532e961037511fe8eddfd37fd59b0a05c98e0c874eef694b825ae7a9`.
- Candidate: `e2ec30190bce6becda1e998e90da1f7bfb4ee8b8fdf065f0e1a8ceedcfb716a0`.

The repository's isolated launcher created a fresh protected disposable HOME,
XDG data root and temporary directory, with normal and legacy project roots
sandbox-denied. A fresh local account, blank project and Agile feature were
created through the UI; recovery export stayed inside the disposable HOME.
Executable identity and all four isolation environment values were checked
before native automation and after potentially terminal actions when the
process survived. Actual exits were verified from child status before any
subsequent app observation.

Six native-window screenshots were saved, opened and accepted, matched to
the current automation state and PID-owned 1280 x 832 window:

| Step | Observation | Health |
| --- | --- | --- |
| 1 | Baseline empty-name guarded save shows generic foreground failure and dimmed specific error. | Defect reproduced; quit blocked |
| 2 | Candidate new-sprint refusal shows “Sprint name is required” in the foreground. | Healthy scoped feedback |
| 3 | Cancel retains the empty name and edited goal. | Healthy draft retention; inline error remains outside the editor |
| 4 | Correct name and manual retry create the sprint and show success. | Healthy |
| 5 | Existing-sprint invalid-name edit shows the same specific foreground refusal. | Healthy scoped feedback; quit blocked |
| 6 | Cancel, correction and guarded Save quit cleanly; restart reopens the corrected fields. | Healthy, UI-level persistence evidence |

The reopened record showed `Foreground corrected sprint`, goal `Candidate
retained goal updated`, planning status, dates `2026-10-19` to `2026-11-02`,
capacity `0`. Baseline test-draft Discard exited with status 0 at
15:52:02.779873 CDT. Candidate guarded-save quit exited with status 0 at
17:55:19.167195; final unchanged-editor quit exited with status 0 at
17:56:03.650543. The trial was interrupted by a user pause; these timestamps
do not imply continuous elapsed test work. No app remained running.

These native failures were validation refusals, not fresh SQLCipher lock
failures. No full-row database oracle, normal-profile audit, package install,
receipt modification, force quit or outside-profile launch was performed or
observed. Isolation does not prove untouched macOS preferences or caches.
Private captures, credentials and disposable data are not release artifacts.

### Validation and limits

Completed locally: 53 focused tests; three shuffled focused runs (seeds
20261019, 20261020, 20261021); full frontend suite of 626 tests across 81 files;
Svelte check with zero errors/warnings; official Svelte analysis; frontend and
Go lint; `go test -count=1 . ./internal/... ./tools/...`; matching `go vet`;
module verification; `make build`; strict deep bundle-signature verification.

The final `make verify`, `make frontend-smoke` (607 rendered bytes),
`make release-scope` and REUSE lint passed. A backend-only forwarding mutation
failed all three validation-message cases; restoring the adapter restored the
focused suite. Six jsdom canvas diagnostics occur in both baseline and
candidate suites. Existing Wails
`time.Time` and deployment-target linker warnings remain. None establishes
older macOS support or trusted signing.

No new dependency, backend, export or packaging contract changed, so full
release/PDF/trusted-signing/platform matrices were not rerun. Native in-flight
timing, crash/disk faults, post-commit response loss and assistive technology
remain unverified. Screenshots do not establish contrast, focus handling or
accessibility compliance. The shared dialog still says “leaving this view”
during native quit; other editors' boolean-error adapters remain separate work.
The overall editor-data-loss and release matrices remain open.

Lesson: error presence somewhere in the DOM does not prove useful foreground
feedback. Assert the exact guard message and replacement of old errors, then
verify Cancel/retry and prohibited continuation independently.

## Stakeholder errors in the exit guard

### Contract and acceptance

Baseline: `39676948772b57c2e2b977c99b5a9e277de5937c`.
`StakeholderManager` retained specific inline errors but returned only `false`
to the autosave coordinator, which substituted its generic foreground error.
The registered callback now forwards the existing nonempty message by throwing
an `Error` after an attempted save fails. Absent or busy drafts refuse first,
without interpreting a retained message as a new save failure.

All acceptance criteria passed in the bounded checks below:

- Name validation, unsafe hourly-rate/contract-value minor units and backend
  failures reach `navigation.error` with their existing exact text.
- Failure retains the draft, dirty state and pending destination, without
  navigation or native-close completion. Unsafe money never reaches the writer.
- Cancel/rearm and corrected retry recover; stale backend details do not replace
  a subsequent validation error.
- A busy manual save refuses an overlapping guarded write with the generic
  fallback, even if a late list failure has populated an unrelated inline error.
  Its eventual completion does not itself grant pending native-close permission.
- Existing canonical-money, per-field normalization, immutable payload,
  late-edit/rebase and per-open registration tests remain passing.

Only the private registered callback changes. Manual boolean save behavior,
shared coordinator/session interfaces, payloads, IDs, defaults, schemas,
backend transactions, markup and styling are unchanged. No migration or new
dependency is needed.

### Regression evidence

The stakeholder suite grows from 22 to 28 cases. Eight exact-message assertions
failed against the original callback, then passed with the adapter. Tests check
the foreground `navigation.error`, not merely text elsewhere in the DOM.
New cases cover new-draft Error/string/empty-Error rejections, retry, a previous
backend failure followed by name validation, and a controlled busy save with
an unrelated late refresh error. Existing native-close recovery assertions now
include Cancel/rearm; unsafe-money rejection covers both money fields.

Removing only the registered readiness check in an isolated test transform
made the busy test fail: it received the late list error instead of the generic
refusal. Removing only message forwarding made eight exact-message checks
fail again. The production source was not mutated. The normal focused suite also
passed with shuffled order (seed `20261021`). Deferred test operations are
settled and awaited in `finally`.

Reproduce the focused 48-case component/coordinator check:

```sh
npm --prefix frontend test -- --run src/lib/components/project/StakeholderManager.test.ts src/lib/autosave.test.ts src/lib/session.test.ts src/lib/native-close.test.ts
```

Fresh October 21 validation also passed: 637 frontend tests across 81 files;
Svelte check (zero errors/warnings); frontend lint; `make verify` including
root/internal/tools Go tests and repository contracts; production `make build`;
strict deep bundle-signature verification; frontend runtime smoke (607 rendered
bytes); focused native-close Go race tests; `go mod verify`; dependency audit
(zero advisories); REUSE lint; and release-scope checks. Official Svelte analysis
reported no issues, with an unchanged refresh-effect advisory.

### Browser trial and limits

Six saved, opened screenshots cover baseline entry, invalid name, generic
foreground failure, candidate specific message, Cancel/correction, and guarded
retry to one corrected list row. The private fixture uses the real App, router,
editor and guard with a disposable in-memory backend. Visible money values
remain unchanged. This is not native quit or database commit/reopen evidence.
Rejected fixture setup failures and a replaced intermediate capture are not
accepted product evidence; private screenshots/configuration are not release
artifacts.

Six canvas diagnostics were reproduced by unchanged DORA component tests under
jsdom; they do not establish chart rendering. Existing Wails `time.Time` binding
messages and the macOS 13-object/11-link target warning remain. Ad-hoc signature
verification does not establish publisher trust. No native GUI trial, full Go
race suite, Go advisory scan, Linux/Windows native trial, assistive-technology
audit or complete release/export matrix was run for this frontend-only adapter.

Lesson: preserve error details at the boolean-to-coordinator boundary, and
assert both the current foreground message and prohibited side effects. A busy
refusal must not turn an unrelated retained error into a claimed save failure.

## Work-item errors in the exit guard

### Scope and acceptance

Baseline: `b6e16ed5b543201db10ac1e919c7dc1999906432`. Backlog and Kanban
share `WorkItemEditor`. It caught save failures and retained a specific inline
error, but returned only `false` to the coordinator. The foreground guard
therefore substituted a generic failure above the dimmed actionable error.

The registered callback now forwards the existing nonempty message through
the coordinator's exception handling, only after checking that the draft is
present and idle. This readiness check matters: deletion can retain an earlier
save error, which must not describe a new busy refusal. No shared coordinator,
manual boolean save API, payload, ID, timestamp, default, schema, styling or
backend transaction changes are included.

Acceptance passed in the bounded evidence below: current specific foreground
errors; draft/dirty/pending state retained and no success/close permission on
failure; corrected retry; generic fallback when no details exist; compatibility
with existing payload, late-edit and per-record registration behavior.

### Automated evidence

Seven exact-message assertions failed against the original callback. Five new
cases bring the editor suite from 22 to 27 tests:

- New/existing drafts: a Wails-style string rejection reaches the guard;
  Cancel retains edits; whitespace validation replaces the previous backend
  error without another write; correction retries the exact normalized payload.
- An empty backend error retains the generic refusal and dirty draft.
- A deferred manual save refuses a guarded duplicate without another write;
  its later rejection still appears inline.
- A prior failed save followed by deferred deletion cannot forward the stale
  error on native-close Save. The guard remains pending, issues no extra save
  or delete and grants no quit permission. The deletion is settled explicitly.

Existing native-close tests now assert exact backend messages as well as
retained fields, unchanged source props, retry payloads and prohibited effects.
Removing the callback's readiness check made the deferred-deletion test fail
with the old save error (26 passed, one failed). Forwarding an empty message
made the detail-free backend test fail (26 passed, one failed). Restoring the
intended behavior restored all 47 focused tests. Deferred operations are
settled and awaited in `finally`, including when an assertion fails.
The focused command covers this editor, autosave, session and native-close
tests: 47 passed. `make verify` passed, including root/internal/tools Go tests,
631 frontend tests across 81 files, Svelte check (zero errors/warnings), build
and repository contracts. A verbose full-suite rerun also passed 631 tests.

Also passed: frontend/Go lint (zero Go issues),
`go test -race -count=1 . -run 'TestNativeCloseGuard'`, `go mod verify`,
`make build`, strict deep bundle-signature verification, runtime smoke
(607 rendered bytes), dependency audit (zero advisories), REUSE lint and
release-scope checks. Official Svelte analysis found no issues; its unchanged
effect suggestions concern intentional per-record draft/registration ownership,
not a reason to replace the imperative lifecycle with derived state.

### Fresh native trial, October 20, 2026

The isolated launcher created a protected disposable profile. A fresh local
account and blank project were created through the UI and Agile was enabled.
Normal and legacy project-data roots were sandbox-denied; credentials and
recovery export stayed private. The executable and all four isolation
environment values were checked before automation and after quit-capable
actions when the process survived. Actual process exit was checked before
any further app observation.

- Baseline executable SHA-256: `e2ec30190bce6becda1e998e90da1f7bfb4ee8b8fdf065f0e1a8ceedcfb716a0`.
- Candidate executable SHA-256: `dee97b65d757014b716b1f6ba6e0e235f42e1238af74632f6434efee14b7e3e6`.

The baseline artifact used Go 1.27.2; the fresh candidate build used the
installed Go 1.27.3 with `GOTOOLCHAIN=auto`, while `go.mod` remains at 1.27.2.
No dependency/toolchain files changed. This is not a single-variable binary
experiment; component red/green evidence independently isolates the callback.

Six screenshots were saved, opened and accepted against the current CUA view
and PID-owned 1280 x 832 window:

| Step | Observation | Health |
| --- | --- | --- |
| 1 | Baseline native-quit Save shows generic foreground failure above the dimmed title validation. | Defect reproduced; quit blocked |
| 2 | Candidate new-draft Save shows “Title is required.” in the foreground. | Improved; quit blocked |
| 3 | Cancel retains the blank title and edited description. | Healthy retention |
| 4 | Corrected title and manual Save create one visible Backlog item. | Healthy recovery |
| 5 | Existing-item invalid update produces the same specific foreground error. | Improved; quit blocked |
| 6 | Cancel, correction, re-armed guarded Save and clean quit; protected relaunch reopens the corrected record. | Healthy, bounded persistence |

The reopened item retained the visible ID `wi_5f3fdcd9`, corrected title,
updated description, story/medium/backlog defaults, zero points and no
assignee/sprint. The baseline disposable-draft Discard, candidate saved quit
and final unchanged-editor quit each exited with status 0. No app was left
running. Private captures and disposable data are not release artifacts.

### Limits and lesson

These native failures were validation refusals, not fresh SQLCipher faults.
There was no full-row database oracle, package installation, receipt change,
force quit or observed outside-profile launch. Isolation does not prove that
all macOS preferences/caches were untouched. No Linux/Windows native trial,
full race suite, Go advisory scan, trusted signing, export/release matrix or
assistive-technology audit was performed for this frontend-only fix.

Six canvas diagnostics were traced to unchanged DORA tests under jsdom; they
are not chart-rendering evidence. Existing Wails `time.Time` binding messages
and the macOS 13-object/11-link target warning remain; INSTALL requires macOS
13 or later. Ad-hoc signature verification is not publisher trust evidence.
Native in-flight timing, crash/disk faults, post-commit response loss, broader
editor-data-loss coverage and stakeholder error forwarding remain separate.

Lesson: assert the foreground guard message, not merely error text anywhere
in the DOM. Check readiness before interpreting a retained error as the result
of the current save attempt, and test both recovery and prohibited effects.
