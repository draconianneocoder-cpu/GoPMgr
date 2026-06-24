<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Release pre-flight checklist

Run through this before pushing the first `v*` tag. It captures the static
pre-flight audit of `release.yml` and the packaging scripts (2026-06-23) so the
first release has the best chance of going green on the first try. The pipeline
has never executed end-to-end, so treat the first tag as the integration test.

## Hard blocker — do this first

- [ ] **Generate the pinned AppImage tool digests.** `scripts/package-appimage.sh`
      verifies `linuxdeploy` against `build/linux/appimage-tools.sha256` and
      **fails closed** if it is missing. Because `package-linux.sh` builds the
      `.deb` and `.rpm` *before* the AppImage, a missing digest file fails the
      whole Linux job and you get **no Linux artifacts at all** (deb/rpm
      included). Generate and commit it once, on a trusted network:

      ```sh
      # Linux box / CI, or via Docker on macOS:
      docker run --rm -v "$PWD":/w -w /w ubuntu:22.04 bash -c \
        "apt-get update && apt-get install -y curl && \
         APPIMAGE_TOOLS_REFRESH=1 bash scripts/package-appimage.sh"
      git add build/linux/appimage-tools.sha256 && git commit -m "build: pin AppImage tool digests"
      ```

## Verified correct by the audit (no action)

- **Filename contract is consistent** across scripts and `docs/INSTALL.md`:
  `pmforge-<v>-amd64.deb`, `pmforge-<v>-x86_64.rpm`,
  `PMForge-<v>-x86_64.AppImage`, `PMForge-<v>-arm64.dmg`,
  `PMForge-<v>-amd64-setup.exe`.
- **Binary name** `pmforge` matches `wails.json` (`outputfilename`) and every
  script + `nfpm.yaml` path.
- **Tracked build assets exist**: `build/appicon.png`, `build/linux/pmforge.desktop`
  (valid `Office;ProjectManagement;` categories), `build/linux/nfpm.yaml`,
  `build/darwin/Info.plist`.
- **macOS** `.app` discovery is glob-based (`build/bin/*.app`) with a
  `create-dmg → hdiutil` fallback, so it survives create-dmg flaking in CI.
- **Windows** installer collection now picks the newest `*installer*.exe`
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

## Tag procedure

1. Confirm `main` is green in CI (verify, lint, **vuln**, build, analytics-duckdb).
2. Confirm `build/linux/appimage-tools.sha256` is committed (hard blocker above).
3. Bump/confirm the app version where the project tracks it, then:

   ```sh
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```

4. Watch the **Release** workflow. Expect first-run friction on the Windows NSIS
   step (scaffold) and AppImage GTK bundling; both are isolated per-OS matrix
   legs (`fail-fast: false`), so one failing leg still lets the others publish.
5. After a green run, download each artifact and smoke-test install on a real
   machine per platform before announcing.
