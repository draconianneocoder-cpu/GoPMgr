<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Work-item errors in the exit guard

## Scope and acceptance

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

## Automated evidence

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

## Fresh native trial, October 20, 2026

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

## Limits and lesson

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
