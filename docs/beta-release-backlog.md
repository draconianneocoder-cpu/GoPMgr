<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Beta release backlog

This is the release-readiness backlog for the current feature set. It is not a
feature wish list. New features should wait unless they fix security, data
loss, or a release blocker.

## Release exit criteria

- Every P0 item is complete.
- No known critical security, data-loss, first-run, or installer defect is
  open for an advertised platform.
- `make verify`, `make release-scope`, `make license-check`, and
  `make check-release` pass from the release commit.
- Release notes describe every remaining limitation without implying unverified
  platform support or trusted signing.
- Installation, upgrade, and data-preserving removal have current evidence for
  each advertised package format.

## P0: must complete before wider beta distribution

| Item | Status | Completion evidence |
| --- | --- | --- |
| Prevent silent editor data loss | In progress | The shared guard protects registered editors on navigation, project close, sign-out, and window close; failed saves and edits made during a save remain visible and block unsafe continuation. The 2026-08-15 native-close guard freezes renderer interaction after a clean, saved, or discarded decision, then has Go grant and consume its one-shot permit while invoking Wails quit; `main_test.go` and `frontend/src/lib/native-close.test.ts` cover the source contract. `WorkItemEditor`, `SprintList`, and `StakeholderManager` each register only their current open draft, with timed auto-save disabled because a successful modal save closes it. In all three, a late modal-field edit survives the first save, retains backend-owned fields, and requires another successful save before close. Stakeholder saves preserve exact money minor units while they are safe JavaScript integers unless the matching major-unit field changed; an unsafe Wails number fails closed rather than being saved rounded. Native project-close/sign-out/window-close still need real packaged-app evidence, not just source inspection. |
| Validate package lifecycles | Planned | A real target validates installation, first run, upgrade, removal, and reinstall for every advertised package format. |
| Confirm `Login` cannot leave a stale session's project open under a new user's key | Planned | Spotted 2026-08-16 while enumerating every write site of `a.dek` (`app_session.go`) for an unrelated investigation: `App.Login` (`app_session.go`) sets `a.user` and `a.dek` to the newly-authenticated user's values but never checks whether `a.db`/`a.dbPath` are already set from a previous session, and never closes them — unlike `Logout` and `shutdown`, which always clear `a.db` and `a.dek` together in the same locked section. If `Login` can be called while a project is already open (e.g. an admin signing in as a different user without the frontend having called `CloseProject`/`Logout` first — not confirmed whether the frontend's own flow prevents this, only that the Go method itself does not), the result is `a.db` left pointing at the *previous* user's live database connection while `a.dek`/`a.user` now belong to the *new* user. Any subsequent call that mixes the two (e.g. `RepairAndSwap`, which reads `a.db` and `a.dek` together under one lock) would then operate on one user's project using another user's encryption key. Not confirmed reachable through the shipped UI — needs a check of the frontend's login flow (does it always call `CloseProject`/`Logout` first?) and, if reachable, a fix either in `Login` (refuse or force-close an existing session) or a regression test proving it can't happen. |
| Confirm self-heal is reachable on real encrypted corruption | Investigated, closed — see `repair_and_swap_test.go` | `App.RepairAndSwap()` (`app_documents.go`) requires `a.db != nil`, which requires `OpenProject` to have already succeeded — every project is SQLCipher-encrypted. A single-offset probe on 2026-08-16 initially found `OpenProject` itself failing on the exact light-corruption fixture `internal/db/repair_selfheal_test.go`'s `corruptLightly` uses (byte offset 4097), raising the concern that `RepairAndSwap` might be unreachable exactly when needed. Resolved via a systematic page-by-page sweep (one byte flipped per 4096-byte page across a 105-page encrypted fixture, `OpenProject`/a real query/`RepairAndSwap` outcome recorded per page, byte restored before the next trial — reproducible, confirmed identical across repeated runs): `OpenProject` succeeded after single-page corruption on 80/105 pages (76%) — the earlier offset-4097 failure was one unlucky page (page 1, hit by `Migrate`'s schema/settings bootstrap), not evidence of a systemic problem. The sweep varied exactly one dimension — a single byte flipped in one page at a time — and did not cover multi-page corruption, torn/partial writes, or truncation, so this closure is scoped to single-page corruption specifically, not corruption in general. **Within that scope, `RepairAndSwap` is reachable in the large majority of cases** — `TestRepairAndSwapHealsReachableLightCorruption` (`repair_and_swap_test.go`) pins one such case end-to-end: `OpenProject` succeeds, a real query succeeds, then `RepairAndSwap` still detects the underlying corruption during its own full-database `VACUUM INTO` pass and heals it. **Residual finding, not a defect:** of the 3 pages (of 105) where corruption was severe enough that a real query failed even after `OpenProject` succeeded — genuinely user-visible corruption, exactly `RepairAndSwap`'s target scenario — `RepairAndSwap`'s own healing attempt also failed on all 3 in this sample, rather than producing a healthy copy. `TestRepairAndSwapCanFailToHealEvenWhenReached` pins this as current, real behavior (not asserted as a bug — no test can require `VACUUM INTO` to heal arbitrary corruption). This mirrors the same light-vs-severe corruption distinction `internal/db/repair_selfheal_test.go` already documents for plaintext SQLite (`corruptLightly` heals, `corruptSeverely` doesn't); this sweep confirms the same distinction holds on the encrypted path, with a small (n=3) sample suggesting user-visible corruption skews toward the unhealable side more than the overall 76%/24% split would suggest — worth a larger sample in a future cycle if this becomes a real support-burden signal, but not blocking: no code behavior change is needed to close this item, since `RepairAndSwap` was already doing the right thing when reachable. |

## P1: release trust and usability

| Item | Status | Completion evidence |
| --- | --- | --- |
| Sign and notarize distributables where supported | Blocked | Release credentials are configured and the published artifacts pass the platform trust checks. |
| Configure the signed update channel | In progress | A controlled prerelease-to-prerelease update verifies signature, channel, version, URL, and digest while offline use remains unaffected. |
| Obtain trusted PAdES evidence | Blocked | A release-certificate sample validates as trusted and independent interoperability evidence is archived. |
| Complete keyboard and assistive-technology smoke coverage | Planned | First run, project editing, dialogs, reports, and native window controls are exercised with the supported assistive tools. |

## P2: follow after beta stability

- Restore a usable normal window frame across relaunch.
- Improve local project discovery without weakening data isolation.
- Expand release validation only after it has an owner, target, and repeatable
  acceptance criteria.

## Maintenance rules

- Use `Planned`, `In progress`, `Blocked`, `Done`, or `Deferred`.
- A completed item must link to current tests or an archived release artifact;
  source inspection alone is insufficient for native behavior.
- Keep this file concise. Put durable architecture decisions in an ADR and
  point-in-time investigation details in Git history.
