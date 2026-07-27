<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: CC0-1.0
-->

# Bundled font binaries

This directory holds the TrueType (`.ttf`) font files that PMForge
embeds in generated PDFs. The `go:embed assets` directive in
`../manager.go` bundles whatever `.ttf` files are present here into the
compiled binary.

The four Source Sans 3 faces are committed as PMForge's release-critical
PDF/A baseline. Strict archival exports require an embedded TrueType font;
without these files, a clean build falls back to PDF core Helvetica and fails
veraPDF. `make required-font-assets` verifies that the baseline is tracked,
has a valid TrueType signature, and matches the reviewed Adobe release bytes.

The other, larger catalog families are not committed. They retain their own
upstream licenses and are fetched on demand from their canonical sources.

## Fetching the fonts

```sh
make fonts          # or: scripts/fetch-fonts.sh
```

This downloads every family listed in `../catalog.go` into this directory.
Source Sans 3 is fetched from the immutable upstream commit recorded in the
fetch and verification scripts; `--force` refreshes the tracked baseline.
After fetching, rebuild with `make build` and the fonts are embedded.

## Graceful degradation

If an optional family's `.ttf` files are absent at build time, the font
Manager omits that family from `Available()` and renderers fall back to the
next available family. Source Sans 3 is always embedded in supported release
builds so archival renderers never reach fpdf's non-embeddable core Helvetica.

## Licenses

Every bundled family is free for commercial AND personal use and
GPL-3.0-compatible:

| Family            | License        |
| ----------------- | -------------- |
| Liberation (Sans/Serif/Mono) | OFL-1.1 |
| DejaVu Sans       | Bitstream-Vera |
| Noto Sans         | OFL-1.1        |
| Source Sans 3     | OFL-1.1        |
| JetBrains Mono    | OFL-1.1        |
| Roboto            | Apache-2.0     |
| Arimo             | Apache-2.0     |
| Cousine           | Apache-2.0     |
| Ledger            | OFL-1.1        |

The tracked Source Sans 3 faces are accompanied by
`LICENSES/OFL-1.1.txt`. See `LICENSES.md` for how to fetch the Apache-2.0 and
Bitstream Vera texts when auditing optional downloaded families.
