<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Beta Release Backlog

This is the running execution list for the next GoPMgr 1.1.0 Beta
prerelease. The exact `v1.1.0-beta.<n>` tag is selected during release
preflight. `ROADMAP.md` remains the strategic product roadmap; this file is
limited to work needed to make the existing 1.1.0 feature set ready for wider
Beta testing.

## Maintenance rules

- Status is one of `Planned`, `In progress`, `Blocked`, `Done`, or `Deferred`.
- `P0` blocks the Beta release. `P1` should be completed or carried as an
  explicit Beta limitation. `P2` is a candidate that may move to a later
  release without weakening current claims.
- Every completed item records verification evidence. When the work landed in
  an earlier change, record its completing commit. Do not mark an item done
  from source inspection alone when its acceptance criteria require a
  packaged native application or real target.
- Keep new scheduling, analytics, and report capabilities out of this Beta
  stabilization cycle unless they fix data loss, security, or a release
  blocker.

## Beta exit criteria

- [ ] Every `P0` item is `Done`.
- [ ] No open critical security, data-loss, first-run, or installer defect.
- [ ] `make check-release`, `make verify`, `make release-scope`, and
  `make license-check` pass from the release commit.
- [ ] Published artifacts and GitHub asset digests match locally recorded
  SHA-256 values.
- [ ] Installation, first-run, upgrade, and data-preserving removal evidence
  exists for every advertised platform/package format.
- [ ] Release notes and user documentation describe every remaining Beta
  limitation without implying unverified trust or platform support.

## Fixes

### BETA-FIX-001: Enable native macOS green zoom and restore control

- Type: Fix
- Priority: P0
- Status: Done
- Implemented: centralized Wails options now provide explicit macOS options
  with native zoom enabled; focused Go tests preserve the window contract.
- Build evidence: `make package-macos` produced the Apple Silicon DMG; DuckDB
  linkage, strict ad-hoc `codesign`, arm64 inspection, and `hdiutil verify`
  passed in the implementation checkout.
- Native evidence: the packaged app exposed Cocoa's `zoom the window`
  accessibility action and `Window > Zoom`. Both paths enlarged and restored
  the prior frame on each of two active 1920 by 1080 displays. Relaunch from a
  zoomed state was clean; retaining that frame remains the separate
  BETA-IMP-003 behavior decision.
- Scope: macOS main application window

Evaluation found that this is native Wails window configuration, not a missing
Svelte control. GoPMgr's `options.App` leaves `Mac` nil. In Wails 2.13.0,
`internal/frontend/desktop/darwin/window.go` initializes `zoomable` only when
`options.App.Mac` exists; the Cocoa bridge then disables
`NSWindowZoomButton` when `zoomable` is false. GoPMgr already keeps the window
resizable, defines useful minimum dimensions, installs the native macOS Window
menu, and uses Wails' native `zoom:` implementation for maximize and restore.

Planned integration:

1. Centralize construction of the Wails application options so platform
   invariants can be tested without starting the GUI.
2. Supply a non-nil `mac.Options` with zoom enabled. Preserve native title-bar
   chrome, resizing, the 800 by 600 minimum, and the existing Window menu.
3. Do not add a custom HTML/Svelte traffic-light button. Cocoa should retain
   ownership of activation, animation, accessibility, full-screen/zoom
   semantics, and restoration of the prior window frame.
4. Correct Help Guide shortcut wording: F11 is the explicit Windows/Linux
   maximize/restore command, while macOS uses its green control and
   Window > Zoom.

Acceptance criteria:

- The packaged arm64 app exposes an enabled green window control to macOS
  Accessibility.
- From a non-default size and position, activating the green control enlarges
  or full-screens the window according to macOS policy; activating the native
  restore path returns to the exact prior frame within normal display-scaling
  tolerance.
- Window > Zoom toggles between the zoomed and previous frames.
- Close, minimize, resizing, the 800 by 600 minimum, keyboard focus, dialogs,
  and the Windows/Linux F11 behavior remain unchanged.
- Native testing covers the built `.app`, multiple display arrangements when
  available, and relaunch after exiting a zoomed/full-screen state.

Verification plan:

- Focused Go test for the constructed Wails options and platform invariants.
- `go test . ./internal/...`, frontend checks, and `make verify`.
- Packaged-app accessibility-tree inspection and real green-control
  maximize/restore exercise on the M4 Mac.
- `make package-macos`, strict bundle signature verification, and the macOS
  installed-app smoke test.

### BETA-FIX-002: Keep release evidence synchronized

- Type: Fix
- Priority: P0
- Status: Done
- Evidence: `docs/release-preflight.md` now records the completed macOS
  administrator, installation, upgrade, removal, and reinstall lifecycle.

Release-facing checklists must not continue describing completed native tests
as outstanding. Future lifecycle work updates this backlog, release preflight,
and `code-map/recent-decisions.md` in the same change.

### BETA-FIX-003: Prevent silent editor data loss

- Type: Fix
- Priority: P0
- Status: In progress

Registered editors now route navigation, project close, sign-out, and native
window close through Save, Discard, and Cancel choices. Failed saves remain
dirty and block continuation; auto-save no longer treats a resolved `false`
result as success, and close-project errors are no longer ignored. Focused Go,
Svelte, and Vitest coverage passes. Packaged native close behavior and the
remaining stateful editors still require smoke coverage before this item is
`Done`.

## Features

### BETA-FEAT-001: Configure the signed Beta update channel

- Type: Feature
- Priority: P1
- Status: In progress

The application already verifies an Ed25519-signed manifest over HTTPS, but
release builds currently embed neither a manifest URL nor a public key. Plan a
separate build-time release identity derived from the prerelease tag so an
Alpha or Beta binary can compare its real channel version without changing the
clean `1.1.0` macOS bundle version.

Acceptance criteria:

- Release automation signs and publishes a bounded manifest without exposing
  the private key.
- Builds embed the HTTPS manifest URL, Ed25519 public key, and exact
  prerelease identity.
- Invalid signatures, non-HTTPS URLs, oversized responses, downgrades, and
  channel mismatches fail closed.
- App Settings and `gopmgr --update` distinguish unconfigured, current,
  update-available, and verification-failed states.
- An installed Beta detects a controlled newer Beta while offline use remains
  fully supported.

Implementation now uses strict SemVer prerelease ordering, signed channel and
target metadata, HTTPS artifact URLs, SHA-256 digests, and exact tag identity
in macOS and Windows release builds. The release workflow can generate signed
manifests without placing the private key on a command line. Configure the
public/private key variables and validate a controlled Beta-to-Beta update
before marking this item `Done`. Linux remains unconfigured until one signed
manifest can represent both `.deb` and `.rpm` choices safely.

### BETA-FEAT-002: Safe project backup and restore

- Type: Feature
- Priority: P1
- Status: Done
- Evidence: focused archive, restore, hostile-entry, encryption, Go, Svelte,
  and frontend tests pass; `make verify`, `make check-release`, release scope,
  lint, and REUSE 523/523 pass in the implementation checkout.

Project Settings exposes integrity-checked `.pmba` creation. The project
picker restores an archive as a new project after schema, entry count, size,
path, type, digest, database integrity, identity, and current-account unlock
validation. Restore never overwrites data and does not automatically import
certificates.

### BETA-FEAT-003: Beta Center and project discovery

- Type: Feature
- Priority: P2
- Status: Done

Application Settings now shows exact build identity, update-channel state,
known Beta limitations, digest evidence, and existing diagnostic actions.
The project picker adds local name/path search without changing persistence.

## Improvements

### BETA-IMP-001: Native installer trust

- Type: Improvement
- Priority: P1
- Status: Blocked
- Blocker: Apple Developer ID/notarization and Windows Authenticode
  credentials are not configured.

Enable hardened-runtime Developer ID signing, notarization, stapling, and
Gatekeeper assessment for macOS. Sign the Windows installer and executable,
then verify SmartScreen/AuthentiCode metadata. Unsigned Beta publication
remains possible only if release notes retain the current warnings.

### BETA-IMP-002: Keyboard and assistive-technology smoke matrix

- Type: Improvement
- Priority: P1
- Status: Planned

Exercise first-run account creation, sign-in, project creation, navigation,
dialogs, chart/report editors, and the native window controls with keyboard
navigation. Run VoiceOver smoke coverage on macOS and the nearest available
native screen-reader coverage on Windows. Record defects separately rather
than treating type checks as accessibility evidence.

### BETA-IMP-003: Restore the normal window frame across relaunch

- Type: Improvement
- Priority: P2
- Status: Planned

This is separate from BETA-FIX-001, which restores the pre-zoom frame within a
running session. Evaluate persisting the last normal, non-minimized,
non-full-screen frame across launches. Any implementation must clamp an
off-screen frame to the current display's visible bounds and handle removed or
rearranged monitors.

## Native release validation

### BETA-QA-001: macOS installation lifecycle

- Type: Validation
- Priority: P0
- Status: Done
- Evidence: Published Alpha 1 was installed to `/Applications`, upgraded to a
  current-main arm64 build, removed, reinstalled, and relaunched on an M4 Mac.
  The isolated administrator and eight recovery codes survived every
  transition; normal GoPMgr data was unchanged. Lifecycle evidence commit:
  `268da8b`.

### BETA-QA-002: Windows NSIS lifecycle

- Type: Validation
- Priority: P0
- Status: Planned

On a supported Windows x86-64 test machine, verify installer launch,
installation, first-administrator creation, restart and authentication,
upgrade, and data-preserving uninstall/reinstall. Confirm the pure-Go Windows
Launchpad decision path and embedded DuckDB in the installed binary.

### BETA-QA-003: Fedora-family RPM lifecycle

- Type: Validation
- Priority: P0
- Status: Planned

Install the published RPM on a supported Fedora-family target and validate
GTK3/WebKitGTK 4.1 dependency resolution, first launch, administrator
creation, project persistence, upgrade, and removal. Do not claim Fedora RPM
runtime support until this passes.

### BETA-QA-004: Trusted PAdES and Acrobat evidence

- Type: Validation
- Priority: P1
- Status: Blocked
- Blocker: A real trusted signing certificate/TSA chain and Acrobat validation
  environment are not configured.

Run `make check-pades-trusted` against a release-certificate sample and archive
Acrobat trust-panel evidence. Until then, keep the current explicit limitation:
deterministic structure, CMS, veraPDF, pdfsig, DSS, and self-signed
PAdES-BASELINE-T evidence do not establish a publicly trusted chain.

### BETA-QA-005: Ubuntu DEB lifecycle

- Type: Validation
- Priority: P0
- Status: Done
- Evidence: On Ubuntu 26.04 LTS x86-64, `make verify`, the Wails production
  build, tarball packaging, and nFPM `.deb` and `.rpm` creation passed. The
  generated `1.1.0-alpha.1` DEB installed to `/usr/bin/gopmgr`, launched with
  isolated data under GTK3/WebKitGTK 4.1, and handled `SIGINT` with a clean
  shutdown. The package and temporary test dependencies were removed after
  verification. The host's NVIDIA EGL configuration required selecting Mesa
  for Xvfb; this was a virtual-display host quirk, not an application change.
