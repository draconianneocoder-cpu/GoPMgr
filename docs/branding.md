<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr branding assets

Bobby Beaver is GoPMgr's project-controls mascot. The GoPMgr-owned Bobby
artwork in this repository is licensed under GPL-3.0-or-later, the same
license as the application source. Bobby's visual design is inspired by the
[Go gopher created by Renee French](https://go.dev/brand); that source is
credited under CC BY 4.0. This attribution does not imply endorsement by or
affiliation with the Go project.

## Asset roles

The supplied portrait compositions are **logo lockups**, not compact
wordmarks. They remain whole in the authentication screens, where the layout
has enough vertical room to make both Bobby and the GoPMgr name legible. The
application header uses the no-text square mark beside real text, avoiding an
illegibly narrow lockup at toolbar height.

| Purpose | Source of truth | Consumer |
| --- | --- | --- |
| Supplied masters | `assets/branding/source/` | Provenance and future raster work |
| Dark/light frontend lockups and marks | `frontend/public/branding/` | `Logo.svelte`, selected from `html[data-theme]` |
| 1024px native icon variants | `assets/branding/platform/` | Platform release assembly |
| macOS dark/light lockup and app-icon ICNS exports | `assets/branding/macos/` | macOS distribution/design export set |
| Shipped native icon | `build/appicon.png` | Wails, Linux package metadata, and generated macOS/Windows resources |

The dark native icon is the current package choice. It is a documented crop
of the supplied dark lockup: a 704 × 704 square at `(x=64, y=220)`, scaled to
1024 × 1024. The light icon crops the supplied no-text source to its centered
784 × 784 scene at `(x=0, y=192)`, then scales to 1024 × 1024. The source
masters are retained so those choices can be revised without reconstructing
the originals.

macOS, Windows, and Linux each expose one installed application icon per
package. They cannot switch when GoPMgr's frontend theme changes. Wails v2.15
reads `build/appicon.png`, generates the macOS bundle's
`Contents/Resources/iconfile.icns`, and generates `build/windows/icon.ico`
when needed. Linux packaging installs that same PNG. The dark/light lockup and
app-icon ICNS files are maintained macOS export variants; do not add a tracked
`build/darwin/iconfile.icns`, because Wails does not consume it.

## Maintenance

Run `make brand-assets` after changing branding. It validates all required
raster dimensions, ICNS signatures, the native source copy, and the rule that
`make clean` must retain `build/appicon.png`. On macOS, additionally inspect a
fresh Wails bundle to validate the generated `iconfile.icns` at small sizes.

New Bobby artwork must retain the project identity, this attribution, and the
GPL-3.0-or-later REUSE annotation. Do not reuse the retired SVGs.

The compact mark includes a 128px rendition used through `srcset` at toolbar
size, avoiding a 512px download for a 32px control on common displays.
