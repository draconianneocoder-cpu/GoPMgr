<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Local macOS PKG policy

The approved local-test PKG policy is a fixed
`/Applications/gopmgr.app` destination. Moved copies are left untouched, not
searched for as update targets. Application data, receipt retention, signing
and release versioning are unchanged. The published macOS format remains DMG.

## Packaging contract

`build/darwin/installer-components.plist` explicitly sets
`BundleIsRelocatable=false`. The local `pkgbuild` manual describes a default
change for packages built on macOS 27 or later; relying on that default would
make the policy builder-dependent. This is a prevention measure, not evidence
of an observed older-builder installation failure.

The script uses `--root` with an app-only payload so it can supply the
component policy. Control files stay outside the payload. Root mode otherwise
defaults the receipt version to `0`, so identifier and version are explicitly
read from the staged app's `CFBundleIdentifier` and `CFBundleVersion`.
Strict bundle identity, version checking and upgrade replacement remain enabled.
The Git-derived filename label is independent of the receipt version.

Before publication, package expansion must succeed, contain no installer
scripts, and report the expected bundle, `/Applications` destination,
`relocatable="false"`, empty relocation list, strict identity, version-check
and upgrade entries, and the app's receipt version. XML checks require both
successful parser execution and the expected result. Invalid metadata is an
error, not a warning.

The candidate package is built in a private temporary directory inside
`build/packages`, outside the payload. Only a validated candidate replaces the
final PKG by a same-filesystem rename. Failed packaging, expansion, validation
or publication leaves a prior output untouched. Symlink and non-regular final
targets are refused. Temporary payload/publication directories are cleaned on
normal success or failure. This is not a concurrent-build or power-loss
durability guarantee. The existing build step still replaces `build/bin`;
preserve any generated app needed for comparison before invoking it.

## Verification

Run on macOS:

```sh
make macos-installer-tests
bash scripts/package-version-lib_test.sh
bash scripts/package-macos_test.sh
shellcheck -x -P scripts scripts/package-macos-installer.sh scripts/package-macos-installer_test.sh
VERSION=<local-label> make package-macos-installer
pkgutil --expand-full build/packages/gopmgr-<local-label>-darwin-arm64.pkg <new-scratch-directory>
```

The PKG target runs the isolated matrix before building. Its 21 cases use
copied scripts in paths containing spaces and apostrophes, deterministic tool
doubles and the real macOS plist/XML parsers. They exercise both relocation
defaults, policy drift, malformed metadata, scripts, version mismatch,
plausible parser stdout with failure status, build/expansion/publication
failures, symlink/directory output refusal, prior-output preservation and
staging cleanup. These doubles do not prove Installer behavior.
In private copies, removing the explicit relocation key broke the success
case; bypassing the XML boolean-result check broke the missing-policy negative
case. Both faults were detected without changing the live implementation.

On 2026-10-17, macOS 27.0.1 arm64, Go 1.27.2 and Wails 2.16.0 built
`gopmgr-local-pkg-policy-e13722a-darwin-arm64.pkg` from base `e13722a` plus
this packaging change. SHA-256:
`5a98a3abb07e3197bed56731900750e9c95ed90d52279ebf53f5f3b3d5d00898`.
This working-tree artifact is not a frozen release candidate. Full expansion
had only `gopmgr.app` at the payload root, no scripts, receipt/bundle identity
`dev.gopmgr.GoPMgr`, version `1.1.0`, the fixed destination and no relocation.
Contents and permission modes matched an independently staged app with the
required SQLCipher notice and ad-hoc signature. Signature verification passed.
BOM AppleDouble entries were metadata, not extra expanded payload files.

Packaging emitted four `write: Permission denied` warnings also recorded in
the [earlier lifecycle trial](macos-package-lifecycle-2026-10-17.md); their
cause remains unisolated. Complete payload comparison found no missing or
changed app member. Wails also repeated `Not found: time.Time` diagnostics
and the macOS 13 object/macOS 11 link-target warning. They were not suppressed
or resolved by this change. The PKG signature check reports no signature,
as expected for this local format; ad-hoc app verification is not publisher
authentication, notarization or Gatekeeper approval.

No new system installation, moved-copy trial, receipt mutation or application
launch was performed. Earlier native lifecycle evidence belongs to its own
artifact, not this one. Older macOS builders, version-increasing upgrades,
downgrades and the complete release gate remain unverified here. No Go,
frontend, dependency or project-file format changed; the production package
build compiled the app, but full runtime/race/PDF matrices were not repeated.
