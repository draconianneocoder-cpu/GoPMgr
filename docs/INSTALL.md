<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Installing & running GoPMgr

GoPMgr is a local-first desktop app: your projects live in encrypted files on
your own machine — no account, cloud, or network is required.

The limited engineering-validation prerelease
[v1.1.0-alpha.2](https://github.com/draconianneocoder-cpu/GoPMgr/releases/tag/v1.1.0-alpha.2)
provides native installers for evaluation. It is not a beta, stable release,
or supported-platform claim. Read its platform evidence, trust limitations,
digests, and backup instructions before downloading. Building from source
remains available below, as does building the installers yourself.

> **Heads-up — unsigned builds.** Current packages are not code-signed, so
> Windows SmartScreen and macOS Gatekeeper show an "unidentified developer"
> warning the first time you run the app. The warning reflects the absence of
> an Apple Developer ID or Windows Authenticode signature; signing is planned.
> Confirm that the downloaded file's SHA-256 digest matches the digest GitHub
> displays for that release asset to detect a transfer mismatch. A matching
> digest does not establish publisher identity or replace Developer ID signing,
> notarization, Authenticode, or the platform's trust checks. Evaluate that
> limitation before choosing whether to override a platform warning.

## Migration source status (2026-11-03)

The current source targets Wails v3.1.0, including npm `@wailsio/runtime`
3.1.0, GTK 4 and WebKitGTK 6.0. Published alpha.2 assets above are historical
release artifacts; they do not qualify this migration. Source/frontend and
native macOS bridge checks have passed, but package/install/upgrade and
Windows NSIS qualification remain pending. macOS 26 Tahoe and Ubuntu 24.04 remain
mandatory release targets; execution on those unavailable hosts is deferred
to the release gate, not claimed as passed. See the
[migration design](design/wails-v3-migration-design.md).

## Which file do I download?

| Platform | File | How you install it |
|---|---|---|
| Windows 10/11 (x86-64) | `GoPMgr-<version>-amd64-setup.exe` | Guided installer |
| macOS 26 Tahoe or later (Apple Silicon) | `GoPMgr-<version>-arm64.dmg` | Drag to Applications |
| Debian / Ubuntu (x86-64) | `gopmgr-<version>-amd64.deb` | `apt` / `dpkg` |
| Fedora / RHEL / openSUSE (x86-64) | `gopmgr-<version>-x86_64.rpm` | `dnf` / `rpm` |

## Install

### Windows (`.exe`)

Double-click the installer and follow the prompts. If SmartScreen appears,
choose **More info → Run anyway**.

### macOS Apple Silicon (`.dmg`)

Current source requires macOS 26 Tahoe or later under the owner-approved
support policy. Earlier macOS versions are no longer supported. Historical
release assets above retain their original requirements.

1. Open the `.dmg`.
2. Drag **GoPMgr** onto the **Applications** shortcut.
3. First launch only: right-click **GoPMgr** in Applications → **Open** →
   **Open** (or System Settings → Privacy & Security → **Open Anyway**).

To upgrade a DMG installation, quit GoPMgr and drag the newer app onto
Applications, replacing the existing bundle when Finder asks. Removing or
replacing `GoPMgr.app` does not remove accounts or projects because they live
in the application data directory. To uninstall both the application and its
data, back up any project files you need before separately removing that
directory.

### Debian / Ubuntu (`.deb`)

```sh
sudo apt install ./gopmgr-<version>-amd64.deb
```

For the Wails v3 migration, Debian/Ubuntu packages require `libgtk-4-1` and
`libwebkitgtk-6.0-4`. `apt` resolves declared dependencies; verify the built
package and its runtime requirements on the minimum target before release.
Ubuntu 24.04 remains the minimum-target release gate, with no current
execution evidence. Do not install an incompatible package by substituting
GTK 3 or older WebKit runtimes. The published alpha.2 package has its own
historical dependency/evidence record.

### Fedora / RHEL / openSUSE (`.rpm`)

```sh
sudo dnf install ./gopmgr-<version>-x86_64.rpm
```

After installing, launch **GoPMgr** from your applications menu (or run
`gopmgr` in a terminal on Linux).

## Clean-test reset

To repeat first-launch and administrator-creation testing, quit GoPMgr and run
the repository helper:

```sh
make reset-clean-test
```

The command does not delete projects or accounts. It atomically moves the
active `GoPMgr` data directory to a timestamped sibling such as
`GoPMgr.clean-test-backup-20260728T120000Z` and prints the exact backup path.
The next launch creates a fresh `system.db` and offers first-user
administrator creation.

To return to the saved state, quit GoPMgr, reset the temporary clean-test
state if one was created, then restore the original backup:

```sh
make reset-clean-test
bash scripts/reset-clean-test.sh --restore "/absolute/path/to/GoPMgr.clean-test-backup-<timestamp>"
```

The helper refuses to move data while GoPMgr is running, refuses broad or
symlinked targets, and will not overwrite an existing active data directory or
backup. Use `--data-root "/absolute/path/to/GoPMgr"` to exercise an isolated
test root instead of the platform default.

## Run / build from source

Prerequisites:

- **Go** (version in `go.mod`), **Node** with `npm`, and **Python 3** for
  build-metadata rendering and release-version queries.
- The **Wails CLI**, pinned to the runtime and installed into the local tool
  directory from the repository root:

  ```sh
  mkdir -p .tmp/wails-v3/bin
  GOBIN="$PWD/.tmp/wails-v3/bin" go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
  ```

- **Linux only:** GTK 4 and WebKitGTK 6.0 development packages, for example
  `sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev pkg-config` on
  Debian/Ubuntu. The standard Wails v3 CGO backend needs no legacy backend tag.

Then:

```sh
# 1. Install the exact frontend deps — use npm ci, NOT npm install.
cd frontend && npm ci && cd ..

# 2. Build a desktop binary/app for your platform (output in build/bin/).
make build

# …or run in hot-reload development mode:
make dev
```

> **Important:** always install the frontend with `npm ci`. A fresh
> `npm install` resolves a newer Svelte that breaks the pinned Vite plugin and
> fails the build (`svelte-check` still passes, which hides it).
>
> `make build` is the production path.

## Build the installers yourself

On the matching OS, after `make build`:

- **Linux** (`.deb` / `.rpm`): `VERSION=<x.y.z> bash scripts/package-linux.sh`
  (requires the nFPM version recorded in
  `scripts/release-tool-versions.env`; the script prints exact installation
  guidance when it is missing or mismatched).
- **macOS** (`.dmg`): `VERSION=<x.y.z> make package-macos`
  (uses the built-in `hdiutil` image path with an Applications shortcut;
  `GOPMGR_FANCY_DMG=1` opts into a locally installed `create-dmg`).
- **Windows**: on a native Windows host, run
  `GOOS=windows GOARCH=amd64 bash scripts/wails-build.sh`, then
  `GOARCH=amd64 bash scripts/package-windows.sh`. The second command consumes
  `build/bin/gopmgr.exe` and writes `build/bin/gopmgr-amd64-installer.exe`.
  NSIS 3.12.0 is the pinned release tool. The v3 source/macro path has local
  fixture coverage, including compilation with NSIS 3.13; that does not
  qualify NSIS 3.12.0 or native Windows install/upgrade/remove and trust checks.
  Those remain release gates. Do not use retired v2 installer flags.

The release workflow (`.github/workflows/release.yml`) records a native
platform packaging matrix. During migration, run checks locally. Do not push
a tag or launch hosted automation to obtain missing qualification evidence.

### Local macOS PKG testing

`VERSION=<local-label> make package-macos-installer` builds a local-test-only
Apple Silicon `.pkg`; it is not an advertised release asset. The target runs
`make macos-installer-tests` first, then rebuilds the app. The PKG always
targets `/Applications/gopmgr.app`: install/reinstall does not search for or
update copies moved elsewhere. Quit GoPMgr before installation. The filename
label does not change the bundle or receipt version.

The package is unsigned and its app is ad-hoc signed, not Developer ID signed
or notarized. Packaging validates the generated metadata before replacing an
existing output PKG. See [installer policy and bounded verification](testing/macos-installer-policy.md)
for failure handling, test commands and unverified lifecycle boundaries.

## Where your data lives

GoPMgr stores each project as an encrypted database under your user data
folder. Uninstalling the app does **not** delete your projects. See the in-app
**Help → Installing & Running** and **Database Encryption** sections for more.
