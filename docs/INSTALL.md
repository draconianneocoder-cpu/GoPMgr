<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Installing & running PMForge

PMForge is a local-first desktop app: your projects live in encrypted files on
your own machine — no account, cloud, or network is required.

Each release publishes a native installer per platform. Download the file for
your OS from the project's **Releases** page and follow the steps below.

> **Heads-up — unsigned builds.** Current packages are not code-signed, so
> Windows SmartScreen and macOS Gatekeeper show an "unidentified developer"
> warning the first time you run the app. The packages are safe to install; the
> warning just reflects the absence of a paid signing certificate. Signing is
> planned.

## Which file do I download?

| Platform | File | How you install it |
|---|---|---|
| Windows 10/11 (x86-64) | `PMForge-<version>-amd64-setup.exe` | Guided installer |
| macOS (Apple Silicon) | `PMForge-<version>-arm64.dmg` | Drag to Applications |
| Debian / Ubuntu (x86-64) | `pmforge-<version>-amd64.deb` | `apt` / `dpkg` |
| Fedora / RHEL / openSUSE (x86-64) | `pmforge-<version>-x86_64.rpm` | `dnf` / `rpm` |

## Install

### Windows (`.exe`)

Double-click the installer and follow the prompts. If SmartScreen appears,
choose **More info → Run anyway**.

### macOS Apple Silicon (`.dmg`)

1. Open the `.dmg`.
2. Drag **PMForge** onto the **Applications** shortcut.
3. First launch only: right-click **PMForge** in Applications → **Open** →
   **Open** (or System Settings → Privacy & Security → **Open Anyway**).

### Debian / Ubuntu (`.deb`)

```sh
sudo apt install ./pmforge-<version>-amd64.deb
```

`apt` pulls in the GTK/WebKit runtime automatically. Linux packages target
Ubuntu 24.04+ (`libwebkit2gtk-4.1-0`). On older systems use `sudo dpkg -i …`
followed by `sudo apt -f install` to resolve dependencies; older WebKit2GTK
runtimes are no longer the release target.

### Fedora / RHEL / openSUSE (`.rpm`)

```sh
sudo dnf install ./pmforge-<version>-x86_64.rpm
```

After installing, launch **PMForge** from your applications menu (or run
`pmforge` in a terminal on Linux).

## Run / build from source

Prerequisites:

- **Go** (version in `go.mod`) and **Node** with `npm`.
- The **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`.
- **Linux only:** Wails v2 GTK/WebKit dev packages for Ubuntu 24.04+, e.g. on
  Debian/Ubuntu:
  `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`.
  PMForge builds with the Wails `webkit2_41` tag. Wails v2 still links GTK3;
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
  (needs [`nfpm`](https://nfpm.goreleaser.com/)).
- **macOS** (`.dmg`): `VERSION=<x.y.z> make package-macos`
  (uses `create-dmg`, falls back to a staged `hdiutil` image with an
  Applications shortcut).
- **Windows** (`.exe`): `wails build -platform windows/amd64 -nsis` (needs NSIS).

The release workflow (`.github/workflows/release.yml`) runs all of these on
native runners automatically when you push a `v*` tag.

## Where your data lives

PMForge stores each project as an encrypted database under your user data
folder. Uninstalling the app does **not** delete your projects. See the in-app
**Help → Installing & Running** and **Database Encryption** sections for more.
