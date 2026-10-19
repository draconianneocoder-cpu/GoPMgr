<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Sprint save feedback, 2026-10-19

Opening a new or existing sprint editor now clears the previous success
message. A genuine successful save still displays `Sprint saved.`. This
prevents a previous save from appearing to confirm the current unsaved or
failed draft without changing persistence or the shared quit guard.

## Cause and scope

`SprintList` keeps its status across editor sessions. `openEditor` already
cleared the previous error but left the previous success intact. The fix adds
one status reset at that same boundary, shared by new and existing editors.
It adds no timer, reactive effect, dependency or feedback framework. Save
payloads, IDs, defaults, schemas, backend transactions and quit decisions are
unchanged. Error placement and generic guard wording remain separate work.

The four new component cases first perform a genuine successful save, then
open a new/existing draft through manual/native-quit paths. They assert exact
nine-field payloads, all five retained editable values, no success during a
pending or rejected save, retained dirty/quit state, Cancel/re-arming, and
success only after explicit retry succeeds. Backend rejection is mocked;
these assertions are not fresh database rollback evidence. New-record replies
use a generated ID and valid creation timestamp. Deferred failures are settled
even if the pending-state assertions fail.

All four cases failed against the baseline at the stale-success assertion.
They passed after the fix. Removing just the reset failed all four again;
the reset was restored and all 40 focused tests passed. Existing late-edit,
in-flight-save, Discard and timestamp regressions remain intact.

Adversarial review found that hiding status only while editing could pass the
initial cases but resurrect old success after Cancel. The tests now close an
untouched new/existing draft and assert success stays absent without another
save, then reopen before the failure/retry checks. All four strengthened cases
reject that hide-only mutation; the actual reset was restored afterward.

## Native evidence

Baseline: `a974057c1ccb23e35abe6fb97e8b777355e643cf`, executable SHA-256
`47dc95ee2157380cd6b903489161fb957a36b542f4ebacf3ef7ada9c14f8f035`.
Candidate: that baseline plus the one-line production change and tests,
executable SHA-256
`c13e8656532e961037511fe8eddfd37fd59b0a05c98e0c874eef694b825ae7a9`.
`make build` and strict deep signature verification passed. The signature is
ad-hoc, not distribution trust. Host: macOS 27.0.1 arm64, Go 1.27.2/CGO,
Wails 2.16.0.

The baseline and candidate used the same freshly created protected disposable
account/project. The standard isolated native launcher denied normal and
legacy project-data roots. Candidate re-entry additionally pinned its digest,
canonical private paths/ownership, singleton PID, executable and all four
HOME/XDG/TMP/CFFIXED overrides. A wrong-PID probe failed closed. Account,
project, sprint, save and quit interactions used native automation. Recovery
export stayed within the fresh HOME. No normal-profile audit, `/Applications`
installation or installer receipt change was performed.

Every Cmd+Q, guarded Save and terminal Discard used an action-only call;
process checks preceded any subsequent app observation. Actual exits were
verified from child status/logs, without observing a closed app. Baseline
Discard returned status 0 at 07:48:09.542977 CDT; candidate clean quits returned
status 0 at 12:33:33.150165 and 12:34:22.713928 CDT on October 19. No test app
remained running. Protected-root isolation does not establish untouched macOS
preferences or caches.

Ten numbered native-window captures were saved, opened and accepted in this
run. Each OS capture was checked against the current native automation state
and the PID-owned 1280 x 832 window. Private captures and credentials are not
release artifacts.

| Step | Observation | Health |
| --- | --- | --- |
| 1 | Baseline genuine save displays success. | Healthy |
| 2 | Baseline reopened editor still shows that old success. | Defect reproduced |
| 3 | Baseline guarded validation refusal shows old success alongside failure. | Defect reproduced; quit correctly blocked |
| 4 | Candidate genuine save displays success. | Healthy |
| 5 | Candidate existing-editor entry clears old success. | Healthy |
| 6 | Candidate guarded validation refusal shows no old success and blocks quit. | Healthy feedback lifetime; detailed error remains behind guard |
| 7 | Cancel retains the empty name and edited goal without false success. | Healthy retention; error placement still limited |
| 8 | Correct name and explicit manual retry restore success and clear the error. | Healthy |
| 9 | New-editor entry clears that success; untouched new draft closes without a save. | Healthy |
| 10 | Same-profile restart reopens the corrected saved fields. | Healthy, UI-level persistence evidence |

The saved candidate fields were `Feedback corrected 1019`, goal
`Candidate unsaved goal`, dates `2026-10-19` to `2026-11-02`, capacity `0`,
planning status. The goal's literal wording is a test marker, not a claim that
the reopened value remains unsaved. No full-row database oracle or fresh
native SQLCipher busy trial was run here. Earlier contention evidence remains
bound to its [original candidate](native-sprint-update-2026-10-19.md).

## Acceptance and validation

All scoped criteria passed: previous success clears on both editor-entry
paths; pending/rejected saves do not display false success; failed saves keep
the draft dirty and native quit pending; Cancel/re-arm/retry remains usable;
and genuine success is still shown after a successful save.

Passed locally in this task:

- Focused SprintList/native-close tests: 40, including 30 SprintList cases;
  three shuffled runs (seeds 20261019, 20261020, 20261021) also passed.
- Full frontend suite: 623 tests across 81 files; `npm --prefix frontend run
  check` reported zero errors/warnings; frontend lint and official Svelte
  analysis passed.
- `go test -count=1 . ./internal/... ./tools/...`, matching `go vet`, Go lint
  and targeted sprint/native-close race tests passed; module verification passed.
- `make verify`, `make frontend-smoke` (607 rendered bytes), `make release-scope`
  and REUSE lint passed.

Six jsdom canvas diagnostics also occur in the baseline suite; this is not
native chart-rendering evidence. Existing Wails `time.Time` and newer-object/
older-link-target build warnings remain; older macOS support is not established.
Full dependency-advisory, PDF/export, installation, trusted-signing and release
matrices were not rerun: this slice changes no dependency, Go, export or
packaging contract. Other platforms, native in-flight timing, crash/disk faults,
post-commit response loss and assistive-technology behavior are not verified.
Screenshots do not establish contrast, focus management or accessibility
compliance. The overall editor-data-loss/release matrix remains open.

Lesson: feedback must expire at the next editing session, not merely be hidden
after an error. Regression tests must first establish a real prior success and
also prove that the next genuine success remains visible.
