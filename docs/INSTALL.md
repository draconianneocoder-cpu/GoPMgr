<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Installing & running GoPMgr

GoPMgr is a local-first desktop app: your projects live in encrypted files on
your own machine — no account, cloud, or network is required.

Packaged releases will publish a native installer per platform, but **no
GitHub Release has been published yet** — there is currently nothing to
download. Until the first release is cut, build from source instead (see
[Run / build from source](#run--build-from-source) below), or build the
installers yourself (see [Build the installers
yourself](#build-the-installers-yourself)). The table and per-platform steps
below describe what installing a packaged release will look like once one
exists.

> **Heads-up — unsigned builds.** Current packages are not code-signed, so
> Windows SmartScreen and macOS Gatekeeper show an "unidentified developer"
> warning the first time you run the app. The warning reflects the absence of
> an Apple Developer ID or Windows Authenticode signature; signing is planned.
> Confirm that the downloaded file's SHA-256 digest matches the digest GitHub
> displays for that release asset before bypassing the platform warning.

## Which file do I download?

| Platform | File | How you install it |
|---|---|---|
| Windows 10/11 (x86-64) | `GoPMgr-<version>-amd64-setup.exe` | Guided installer |
| macOS 13 Ventura or later (Apple Silicon) | `GoPMgr-<version>-arm64.dmg` | Drag to Applications |
| Debian / Ubuntu (x86-64) | `gopmgr-<version>-amd64.deb` | `apt` / `dpkg` |
| Fedora / RHEL / openSUSE (x86-64) | `gopmgr-<version>-x86_64.rpm` | `dnf` / `rpm` |

## Install

### Windows (`.exe`)

Double-click the installer and follow the prompts. If SmartScreen appears,
choose **More info → Run anyway**.

### macOS Apple Silicon (`.dmg`)

Requires macOS 13 Ventura or later; the Go toolchain GoPMgr is built with does
not support earlier macOS releases.

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

`apt` pulls in the GTK/WebKit runtime automatically. Linux packages target
Ubuntu 24.04+ (`libwebkit2gtk-4.1-0`). On older systems use `sudo dpkg -i …`
followed by `sudo apt -f install` to resolve dependencies; older WebKit2GTK
runtimes are no longer the release target.

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

- **Go** (version in `go.mod`) and **Node** with `npm`.
- The **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`.
- **Linux only:** Wails v2 GTK/WebKit dev packages for Ubuntu 24.04+, e.g. on
  Debian/Ubuntu:
  `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`.
  GoPMgr builds with the Wails `webkit2_41` tag. Wails v2 still links GTK3;
  true GTK4/WebKitGTK 6.0 support requires a future Wails migration.

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
> `make build` is the production path and includes the embedded DuckDB
> analytics engine. Explicit untagged developer builds are possible, but those
> builds show the analytics-unavailable fallback and are not release artifacts.

## Build the installers yourself

On the matching OS, after `make build`:

- **Linux** (`.deb` / `.rpm`): `VERSION=<x.y.z> bash scripts/package-linux.sh`
  (requires the nFPM version recorded in
  `scripts/release-tool-versions.env`; the script prints exact installation
  guidance when it is missing or mismatched).
- **macOS** (`.dmg`): `VERSION=<x.y.z> make package-macos`
  (uses the built-in `hdiutil` image path with an Applications shortcut;
  `GOPMGR_FANCY_DMG=1` opts into a locally installed `create-dmg`).
- **Windows** (`.exe`): run `make windows-installer-scaffold`, then
  `wails build -platform windows/amd64 -tags duckdb -nsis` (requires NSIS
  3.12.0 for parity with the release workflow).

The release workflow (`.github/workflows/release.yml`) runs all of these on
native runners automatically when you push a `v*` tag.

## Where your data lives

GoPMgr stores each project as an encrypted database under your user data
folder. Uninstalling the app does **not** delete your projects. See the in-app
**Help → Installing & Running** and **Database Encryption** sections for more.
