<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Linux minimum target: Ubuntu 26.04

**Date:** 2026-11-09
**Decider:** GoPMgr maintainer
**Replaces:** Ubuntu 24.04 as the Linux minimum target set with the
[Wails v3 and GTK 4 migration](../design/wails-v3-migration-design.md).

## Decision

The maintainer raised the minimum supported Linux to **Ubuntu 26.04 LTS**.
Linux release packages (`.deb` and `.rpm`) are built on Ubuntu 26.04 and
declare GTK 4 (`libgtk-4-1`) and WebKitGTK 6.0 (`libwebkitgtk-6.0-4`).
Ubuntu 24.04 is no longer a release target.

## What changes

- The release workflow builds the Linux packages on `ubuntu-26.04`, as CI
  already does. `scripts/check-linux-runtime-target.sh` requires that pin and
  fails if `ubuntu-24.04` returns.
- Current-state documentation names Ubuntu 26.04 as the Linux release gate.
  Dated evidence records keep their 24.04 history.

## Findings on 2026-11-09

- On an Ubuntu 26.04.1 host, the archive provides `libgtk-4-1` and
  `libgtk-4-dev` 4.22.4 and `libwebkitgtk-6.0-4` and `libwebkitgtk-6.0-dev`
  2.52.6, so the runtime and build dependencies resolve.
- The archive has **no `golang-1.27-go`**, which `debian/control` requires to
  build the Debian source package (the Launchpad PPA route). That route cannot
  build from the stock 26.04 archive until a Go 1.27 package is available to
  it. The release workflow's `.deb`/`.rpm` route uses the pinned Go toolchain
  and is not affected.

## Release gate

The minimum-target execution gate is now runnable on an available Ubuntu
26.04 host, but it has not passed: installed-package install, upgrade and
removal lifecycle remain unqualified. macOS 26 Tahoe execution remains a
separate mandatory gate.
