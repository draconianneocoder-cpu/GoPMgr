<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Release pre-flight checklist

Run through this before pushing a `v*` tag. It captures the static pre-flight
audit of `release.yml` and the packaging scripts (2026-06-23). The first
release candidate exercised the pipeline and the `.deb` and `.rpm` build
cleanly; treat each new tag's run as the integration test.

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
- **macOS** `.app` discovery is glob-based (`build/bin/*.app`) with a
  `create-dmg → hdiutil` fallback, so it survives create-dmg flaking in CI.
- **Windows** installer collection picks the newest `*installer*.exe`
  explicitly and fails loudly if none is found (hardened 2026-06-23).
- `package-macos-installer.sh` is a separate **local `.pkg`** path
  (`make package-macos-installer`), intentionally not used by the release `.dmg`.

## Known caveats to verify on real targets (not pipeline failures)

- **`.deb` WebKit version.** Built on `ubuntu-22.04`, the binary links
  `libwebkit2gtk-4.0-37`, which is **absent on Ubuntu 24.04+** (moved to 4.1).
  The `.deb` installs cleanly on 22.04/Debian 11/12 era; 24.04 users need the
  4.0 compat lib. Documented in `nfpm.yaml`. Revisit when moving to a
  `webkit2_41`-tagged build.
- **`.rpm` cross-distro.** The rpm wraps an Ubuntu-built dynamically-linked
  binary; `gtk3`/`webkit2gtk3` names are correct for Fedora, but **runtime on
  Fedora is unverified**. Test on a real Fedora box before claiming rpm support.
- **Windows NSIS scaffold.** `build/windows/` is not committed; `wails build
  -nsis` auto-generates default templates, so the first build produces a
  **default-branded** installer. After the first successful Windows build,
  commit `build/windows/` for deterministic, customizable branding.
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

So tag `v1.1.0` for a GA release and all three read `1.1.0`. For a pipeline
smoke-test, tag `v1.1.0-rc.1`: packages read `1.1.0-rc.1` while the app/bundle
read `1.1.0` (cosmetic, fine for an rc — nfpm maps the `-rc.1` prerelease to a
valid rpm version). Marketing codenames (e.g. "V1 Expansion") live in the
GitHub release notes, never in the version number.

## Tag procedure

1. Confirm `main` is green in CI (verify, lint, **vuln**, build, analytics-duckdb).
2. Confirm the version of record (channel 1 above) is the semver you intend to
   ship, then tag it exactly (prefixed with `v`):

   ```sh
   git tag v1.1.0 && git push origin v1.1.0          # GA
   # or, to exercise the pipeline first:
   git tag v1.1.0-rc.1 && git push origin v1.1.0-rc.1
   ```

3. Watch the **Release** workflow. Expect first-run friction on the Windows NSIS
   step (scaffold); the per-OS matrix legs are isolated (`fail-fast: false`), so
   one failing leg still lets the others publish.
4. After a green run, download each artifact and smoke-test install on a real
   machine per platform before announcing.
