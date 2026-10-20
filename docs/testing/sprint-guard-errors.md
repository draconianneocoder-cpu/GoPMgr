<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Sprint save errors in the foreground guard

## Scope and cause

Baseline: `d573e164570f5cd0615bf130ac22ad6fd57e29f4`. The sprint editor
caught validation and backend failures and returned `false`; the coordinator
then substituted “The editor reported that the save failed.” The useful error
remained dimmed behind the foreground Save/Discard/Cancel dialog.

The registered sprint callback now forwards its existing nonempty error
through the coordinator's already-supported exception handling. The manual
boolean save function, generic detail-free refusal, shared guard APIs, native
quit permissions, IDs, timestamps, defaults, schemas and styling are unchanged.
No transaction or backend behavior changes are included.

## Risk-to-test evidence

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

## Fresh native darwin/arm64 trial, October 19, 2026

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

## Validation and limits

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
