<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Release pre-flight checklist

Run through this before pushing a `v*` tag. It captures the static pre-flight
audit of `release.yml` and the packaging scripts (2026-06-23), plus the native
runner evidence from the published `v1.1.0-alpha.1` workflow on 2026-07-27.
That workflow built and published all four expected assets. Installation,
first-launch, and uninstall behavior remain platform-specific evidence. The
published DMG's administrator, installation, upgrade, removal, and reinstall
lifecycle has now passed on an M4 Mac; the remaining platform checks are
listed below.

## No pre-generation blockers

The Linux (`.deb` / `.rpm`), macOS (`.dmg`), and Windows (`.exe`) legs build
straight from the tag — there is no file to generate beforehand. (The portable
AppImage format was dropped; `.deb` and `.rpm` cover Linux.)

## Verified correct by the audit (no action)

- **Filename contract is consistent** across scripts and `docs/INSTALL.md`:
  `pmforge-<v>-amd64.deb`, `pmforge-<v>-x86_64.rpm`, `PMForge-<v>-arm64.dmg`,
  `PMForge-<v>-amd64-setup.exe`.
- **Binary name** `pmforge` matches `wails.json` (`outputfilename`) and every
  script + `nfpm.yaml` path.
- **Tracked build assets exist**: `build/appicon.png`, `build/linux/pmforge.desktop`
  (valid `Office;ProjectManagement;` categories), `build/linux/nfpm.yaml`,
  `build/darwin/Info.plist`.
- **PDF/A font baseline is deterministic.** Four tracked Source Sans 3 faces
  replace the non-embeddable PDF core fonts in clean builds.
  `make required-font-assets` checks tracking, TrueType signatures, and
  reviewed upstream checksums before the release spends time compiling.
  Native workflows do not fetch optional font families, so packages cannot
  vary with CDN availability.
- **macOS** `.app` discovery is glob-based (`build/bin/*.app`). Tag builds use
  the built-in staged `hdiutil` path and expose `PMForge.app` beside an
  `Applications` shortcut. `create-dmg` remains an explicit local opt-in, not a
  release-workflow dependency. After the DuckDB app build, the hosted workflow
  clears disposable Go/npm caches to leave `hdiutil` enough staging space; the
  local packaging script does not alter developer caches.
- **DuckDB analytics ships in installer builds.** `make build` passes the
  `duckdb` tag to Wails, and Linux release builds also pass `webkit2_41` for
  Ubuntu 24.04+ WebKit2GTK 4.1. `scripts/verify-duckdb-linked.sh` checks the
  built binary metadata before release/package claims.
- **Windows** installer collection picks the newest `*installer*.exe`
  explicitly and fails loudly if none is found (hardened 2026-06-23).
- **Windows installer inputs are source-owned.**
  `build/windows/installer/project.nsi`, `build/windows/info.json`, and
  `build/windows/wails.exe.manifest` provide PMForge branding, version
  metadata, DPI behavior, GPL display, and a data-preserving uninstall path.
  Wails regenerates only its pinned macro file, WebView bootstrapper, and icon.
  `make windows-installer-scaffold` exercises isolated drift cases and compiles
  a harmless NSIS fixture when `makensis` is available. The fixture translates
  its dummy executable to an absolute Windows path under Git Bash so the
  pre-build check exercises native `makensis.exe`, not only Homebrew NSIS.
- `make package-macos` builds the shareable drag-to-Applications `.dmg`.
  `package-macos-installer.sh` remains a separate **local `.pkg`** path
  (`make package-macos-installer`), intentionally not used by the release `.dmg`.
- **Installer tool selection is immutable.**
  `scripts/release-tool-versions.env` pins nFPM v2.47.0 and the Chocolatey NSIS
  package at 3.12.0. The workflow verifies the installed nFPM module metadata
  and Chocolatey package record before packaging. On Windows it also verifies
  `makensis.exe` at Chocolatey's explicit installation directory and exports
  that directory through `GITHUB_PATH`; the installation process cannot rely
  on an in-process PATH refresh. `make installer-tool-pins` rejects mutable or
  bypassed installs before a tag build starts.

## Known caveats to verify on real targets (not pipeline failures)

- **macOS native window zoom.** The published and current-main arm64 apps
  previously exposed a disabled green zoom/full-screen control. Current source
  now supplies the explicit Wails macOS options block with zoom enabled and a
  focused options-contract test. The packaged app's native Accessibility zoom
  action and `Window > Zoom` enlarged and restored the window on both attached
  displays; clean relaunch from the zoomed state also passed. `BETA-FIX-001` is
  complete.
- **macOS package trust.** The published DMG passed `hdiutil verify`, its
  ad-hoc signature passed strict `codesign` verification, and its complete
  administrator/install/upgrade/removal/reinstall lifecycle passed on an M4
  Mac with isolated data. Gatekeeper warnings remain expected because the app
  is not Developer ID signed or notarized.
- **`.deb` WebKit version.** Built on `ubuntu-24.04` with the Wails
  `webkit2_41` tag, the binary links the Ubuntu 24.04+ WebKit2GTK 4.1 runtime.
  This fixes the earlier WebKit runtime dependency gap on newer Ubuntu. Wails v2
  still links GTK3; GTK4/WebKitGTK 6.0 remains a future framework migration.
  A current-source `1.1.0-alpha.1` DEB completed install, isolated launch,
  clean shutdown, and removal on Ubuntu 26.04 LTS x86-64.
- **`.rpm` cross-distro.** The rpm wraps an Ubuntu-built dynamically-linked
  binary; `gtk3`/`webkit2gtk4.1` names are expected for Fedora, but **runtime on
  Fedora is unverified**. Test on a real Fedora box before claiming rpm support.
- **Windows native execution.** The published alpha's Windows runner completed
  the real Wails/CGO build, verified embedded DuckDB metadata, compiled the
  source-owned template with NSIS 3.12, and uploaded the installer. The
  installer has not yet been launched on a Windows test machine, so
  installation, first-run account creation, and data-preserving uninstall
  remain unverified.
- **Windows decision engine.** The Launchpad uses the same embedded JDM rule
  table on every platform. On Windows it evaluates its exact-match and
  fallback rows in Go because the Zen FFI archive targets MSVC while the
  SQLCipher driver requires MinGW. This keeps the NSIS build on one C toolchain.
- **Unsigned everywhere.** macOS Gatekeeper / Windows SmartScreen warnings are
  expected; the signing hooks in `package-macos.sh` activate when
  `MACOS_SIGN_IDENTITY` (and notarization creds) are set. Covered in
  `docs/INSTALL.md`.

## Version channels — keep all three identical

The version appears in three independent places. For them to read the same
number, the **git tag must equal the version of record**:

1. **Version of record** — `internal/cli/parser.go` `const Version` **and**
   `wails.json` `productVersion`. These two must be equal (enforced by
   `scripts/check-release.sh` step 1) and must be a valid package version
   (clean semver; rpm forbids `-` in its `Version` field). Currently `1.1.0`.
2. **macOS bundle** (`CFBundleVersion` / `CFBundleShortVersionString`) —
   `build/darwin/Info.plist` uses `{{.Info.ProductVersion}}`, which Wails fills
   from `wails.json` `productVersion` at build time. Tracks channel 1
   automatically.
3. **Package version + every artifact filename** (deb/rpm/dmg/exe) — derived
   from the git tag via `${GITHUB_REF_NAME#v}` → nfpm `version`.

For a GA tag, all three channels read the same version. For a pipeline
smoke-test, use a tag shaped like `v<version-of-record>-<prerelease>`; packages
include the prerelease suffix while the app/bundle retain the clean version of
record. This cosmetic difference is acceptable for prerelease testing, and
nFPM maps a valid SemVer suffix to an RPM-safe version. Marketing codenames live
in published GitHub release notes, never in the version number. The tag
preflight enforces either `v<version-of-record>` or a SemVer prerelease of that
exact base version; build metadata is intentionally rejected.

## PAdES evidence before public trust claims

`make check-pades` proves PMForge's deterministic local signature structure;
`make check-pades-external` adds the installed external validators and records
fresh fixture provenance. Neither command proves a public certificate chain
because the fixture signer and TSA are intentionally self-signed.
`make pades-harness-tests` runs the local generator and the deterministic
external, parallel-locking, and trusted-source regression matrices. It is
required by both pre-merge CI and `make check-release`; its controlled
trusted-source output tests classification behavior, not a real trust chain.

For a release-certificate sample, run:

```sh
PMFORGE_TRUSTED_SIGNED_PDF=/absolute/path/to/trusted-signed.pdf \
PMFORGE_PADES_TRUSTED_REQUIRED=1 make check-pades-trusted
```

Required mode succeeds only when the report ends in `status=TRUST_VERIFIED`.
The harness validates the supplied PDF without modifying it, clears stale
derived evidence, and writes hashes plus checkout and UTC provenance to
`.tmp/pmforge-pades-trusted-source/trusted-source-validation-report.txt`.
`STRUCTURE_VALID_TRUST_INDETERMINATE` means the signature structure is valid
but the local CLI certificate store did not prove trust; it is not a passing
required-mode result. `NOT_CONFIGURED`, `INPUT_INVALID`,
`VALIDATION_INCOMPLETE`, and `VALIDATION_FAILED` are likewise non-passing in
required mode. Archive an Acrobat signature-panel screenshot or validation
report separately; CLI verification does not substitute for Acrobat's trust
policy and interoperability evidence.

## Tag procedure

1. Confirm `main` is green in CI (verify, PAdES harnesses, lint, **vuln**,
   build, analytics-duckdb). From the exact commit being tagged, run
   `make check-release` locally as an early signal. The tag-triggered workflow
   reruns that full gate on Ubuntu and blocks all installer jobs until it
   passes. Its strict PDF/A check uses the pinned official
   `verapdf/cli:v1.30.2` image when Docker is available. It also verifies the
   tracked Source Sans 3 baseline, so ignored optional font downloads cannot
   influence conformance. Also confirm
   `bash scripts/verify-duckdb-linked.sh` passes after `make build`.
2. Confirm the version of record (channel 1 above) is the semver you intend to
   ship, then tag it exactly (prefixed with `v`):

   ```sh
   git tag "v<version-of-record>"                    # GA
   git push origin "v<version-of-record>"
   # Or use "v<version-of-record>-<prerelease>" to exercise the pipeline.
   ```

3. Watch the **Release** workflow. Its **Tag preflight** job must pass before
   the package matrix starts. The per-OS matrix legs are isolated
   (`fail-fast: false`), so one failing leg still lets the others build, but
   publication waits for the entire matrix. Preserve the Windows Wails/CGO,
   scaffold, and DuckDB linkage checks that passed for `v1.1.0-alpha.1`. Tags
   with a SemVer suffix are explicitly published with GitHub's prerelease
   classification; only a clean `v<product-version>` tag is eligible for GA.
4. After a green run, download each artifact and smoke-test install on a real
   machine per platform before announcing.
