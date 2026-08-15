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
| Prevent silent editor data loss | In progress | The shared guard protects registered editors on navigation, project close, sign-out, and window close; failed saves and edits made during a save remain visible and block unsafe continuation. The 2026-08-15 native-close guard freezes renderer interaction after a clean, saved, or discarded decision, then has Go grant and consume its one-shot permit while invoking Wails quit; `main_test.go` and `frontend/src/lib/native-close.test.ts` cover the source contract. `WorkItemEditor` now registers only its current open draft (with timed auto-save disabled); a late edit survives the first save and requires another successful save before close. Local drafts in `SprintList` and `StakeholderManager` remain unregistered native-close P0 gaps. Native project-close/sign-out/window-close still need real packaged-app evidence, not just source inspection. |
| Validate package lifecycles | Planned | A real target validates installation, first run, upgrade, removal, and reinstall for every advertised package format. |

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
