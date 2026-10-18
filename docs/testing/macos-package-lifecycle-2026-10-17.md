<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# macOS package lifecycle evidence, 2026-10-17

The published alpha.2 app was replaced by a locally built candidate through
macOS Installer, reinstalled, removed without its data, and reinstalled into
the empty application location. The same disposable account and saved project
survived all three candidate installations. Final app-only cleanup passed.
No production code change was needed. This is bounded darwin/arm64 evidence,
not approval of the entire release or other platform lifecycles.

## Frozen inputs and pre-install gates

Host: macOS 27.0.1 (26A434), arm64. Candidate source:
`5b74fd194f64894c8149f9314602014bd76a786e`, clean tracked worktree.
`make package-macos-installer` passed with Go 1.27.2, CGO and Wails 2.16.0.

| Input | Artifact | SHA-256 |
| --- | --- | --- |
| Published baseline | `GoPMgr-1.1.0-alpha.2-arm64.dmg`, release `v1.1.0-alpha.2` | `c4a843bd8e72e27ec41bc20f8aa5f9d0186e957081a570ad2bc5424251902ca8` |
| Local candidate | `gopmgr-1.1.0-alpha.2-177-g5b74fd1-darwin-arm64.pkg` | `a4d421b69518564efb55bfcabbae173cd52af3455f963ed571b385bf499e0369` |

The baseline came from the [published release](https://github.com/draconianneocoder-cpu/GoPMgr/releases/tag/v1.1.0-alpha.2).
Downloaded bytes matched the release asset digest; disk-image verification
passed. The image was attached read-only and contained the app and the usual
Applications link. No published baseline PKG was available: the baseline
installation was an app copy, not a receipt-tracked PKG installation.

Both apps and the candidate receipt report version `1.1.0`. The filename's
Git-derived version does not change that. These trials establish artifact
replacement and same-package reinstall, **not a version-increasing receipt
upgrade, downgrade refusal, or a resolved release-version policy**.

Before installation, full PKG expansion showed install location
`/Applications`, bundle identifier `dev.gopmgr.GoPMgr`, no installer scripts,
`relocatable="false"`, an empty relocation list and upgrade-bundle semantics.
Relocation refusal was a hard pre-install gate, not inferred from the final
destination. The local `pkgbuild` manual says its relocation default changed
on macOS 27; this observation does not establish behavior on older builders.

The PKG is unsigned. Both app bundles passed local
`codesign --verify --deep --strict` with ad-hoc signatures. No trusted signing,
notarization or Gatekeeper approval is claimed. Packaging exited successfully
but emitted four `write: Permission denied` warnings; their exact cause was
not isolated. Complete expanded/installed bundle comparisons and signature
checks below found no missing or changed bundle member. The warnings remain
a diagnostic limitation, not a suppressed failure or proof of their cause.
The build also emitted Wails `Not found: time.Time` diagnostics and a macOS 13
object / macOS 11 link-target warning. Successful execution on this macOS 27
host does not resolve those diagnostics or verify older deployment targets.

## Isolation and comparison method

The owner approved receipt-tracked installation in `/Applications`, disposable
data and recoverable removal of only the test-owned app. No matching app was
present before the baseline copy. The pre-existing legacy receipt
`dev.pmforge.PMForge` was preserved throughout, including its metadata and file
list. No receipt was forgotten. Administrator authentication stayed in the
native macOS dialogs; no administrator password was collected by automation.

A fresh `scripts/launch-isolated-native.sh` profile provided private HOME,
XDG and temporary directories. Re-entry used the same retained profile,
checked canonical non-symlink paths, ownership and private permissions,
pinned the candidate executable digest and reran the sandbox preflight.
Normal and legacy GoPMgr/PMForge application-data and Documents roots were
denied. This is bounded project-data isolation, not proof of zero unrelated
WebKit cache or preference writes. The old app was never launched against
data already opened by the candidate.

Each stopped-data snapshot covered the entire disposable XDG GoPMgr tree,
including logs and SQLite WAL/SHM sidecars. The comparison checked relative
paths, entry types, modes, file sizes and SHA-256 values. Before/after snapshots
were identical across each installer operation and each app removal, before
any subsequent app launch. App launches can migrate databases and update
login metadata, so byte equality is not claimed across launches.

Complete bundle comparison included the root, directories, resource files,
Info.plist, signature files, executable, modes and confined symlink targets.
It rejected missing, extra or changed members, rather than checking only the
executable. Baseline copy matched all 9 members; each candidate installation
and removed copy matched all 11 expanded candidate members. UID/GID and
extended attributes were not equated with bundle content: installation changed
the app from user-owned to root-owned. BOM AppleDouble entries were consumed
as metadata; no literal top-level `._gopmgr.app` was observed after installation.

A private helper authenticated only the disposable account and unwrapped its
key without logging secrets. It queried the two databases using the repository
SQLCipher driver with `mode=ro` and query-only connections, without application
opening services or migrations. SQLite inspection nevertheless created empty
WAL/SHM sidecars, so snapshots were taken after inspection. Retained logical
checks covered account username/display name, project ID/name/owner/description,
and the document list's IDs, project ownership and titles, not every column.

## Observed lifecycle and acceptance checks

The baseline UI created a non-administrator account and project. The project
description was a unique saved marker. Its project ID remained `prj_c543b523`;
the single Project Charter document remained `doc_5b6e99fd` under that project.
The account, project name, owner and exact description marker were unchanged
in native settings and the logical inspection after every candidate cycle.

| Acceptance check | Result |
| --- | --- |
| Baseline first run, save and clean quit | Passed: published DMG app copied exclusively into `/Applications/GoPMgr.app`; disposable account/project created; exit status 0. |
| Candidate PKG replaces baseline app | Passed: Installer reported success, new `dev.gopmgr.GoPMgr` receipt at `Applications`, install-time `1792243873`; complete bundle and stopped-data checks passed. |
| Same PKG reinstalls over candidate | Passed: Installer success, receipt install-time `1792243980`; complete bundle and stopped-data checks passed. |
| App-only removal preserves data and receipts | Passed: verified stopped test app moved recoverably to private storage; `/Applications/gopmgr.app` absent; data and both receipts unchanged. |
| Reinstall after app absence | Passed: Installer success, receipt install-time `1792244154`; app returned to `/Applications`, not the retained removed copy; complete bundle and stopped-data checks passed. |
| Same-account authentication and project recovery | Passed after all three installations: same-profile sign-in, project reopen and settings marker; logical inspection matched the baseline; each process quit with status 0. |
| Final cleanup | Passed: test-owned app recoverably removed again; no GoPMgr process or matching app left in `/Applications`; data snapshot and both receipts unchanged; baseline image detached. |

The installed path became lowercase `gopmgr.app` on this case-insensitive APFS
volume. Complete bundle comparison was unaffected by the root spelling change.
A shell move of the root-owned app was denied without mutation; Finder's
normal authorized move completed instead. No privilege configuration was
changed. Private test profiles, credentials, snapshots, removed app copies and
the packaging log are not committed. The new candidate receipt intentionally
remains alongside the untouched legacy receipt.

## Validation and remaining boundaries

In addition to the actual package build, native trials and artifact checks:

- Package/version/re-entry shell syntax checks passed.
- `bash scripts/package-version-lib_test.sh` passed.
- `bash scripts/package-macos_test.sh` passed.
- `bash scripts/launch-isolated-native_test.sh` passed.
- A changed-resource counterexample was rejected by the bundle comparator;
  both recoverably removed candidate bundles still matched all 11 members.
- `reuse lint`, `make release-scope`, Gitleaks on the task diff/new document,
  and `git diff --check` passed. The initial sandboxed scope run emitted Go
  cache-access denials despite status 0; a rerun with the required cache access
  completed without those diagnostics. Disposable credential values were also
  checked explicitly and were absent from both documentation files.

No runtime, interface, dependency or packaging-policy change was required.
The code map, API documentation and changelog therefore need no change; the
release backlog links this evidence without closing its cross-platform row.
Full Go/frontend suites, the full release gate, PDF gates and hosted CI were
not rerun for this documentation-only outcome. This run does not verify Linux
or Windows installation, older macOS builders, case-sensitive volumes, trusted
distribution, power-loss/interrupted installation, recovery-code reset, every
legacy schema, version ordering or removal of installer receipts.

Lesson: freeze and hash the exact artifacts, inspect relocation before allowing
installation, compare the whole bundle and stopped data, then authenticate and
reopen saved state. A successful Installer screen or unchanged executable hash
alone cannot establish that lifecycle. Future candidates need their own evidence;
this record must not silently be carried forward to another binary.
