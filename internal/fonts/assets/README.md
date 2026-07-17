<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: CC0-1.0
-->

# Bundled font binaries

This directory holds the TrueType (`.ttf`) font files that PMForge
embeds in generated PDFs. The `go:embed assets` directive in
`../manager.go` bundles whatever `.ttf` files are present here into the
compiled binary.

**The font binaries are NOT committed to the repository.** They are
large binaries with their own upstream licenses, so they are fetched on
demand from their canonical sources. This README is the placeholder
that keeps the `go:embed` pattern valid before the fonts are fetched.

## Fetching the fonts

```sh
make fonts          # or: scripts/fetch-fonts.sh
```

This downloads every family listed in `../catalog.go` from the URLs in
each family's `Source` field, into this directory. After fetching,
rebuild with `make build` and the fonts are embedded.

## Graceful degradation

If a family's `.ttf` files are absent at build time, the font Manager
omits that family from `Available()` and renderers fall back to the
next available family (ultimately to fpdf's built-in Helvetica). The
application always builds and runs, fonts or no fonts.

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

License texts are not committed here; see `LICENSES.md` for how to fetch
`OFL-1.1.txt`, `Apache-2.0.txt`, and `LicenseRef-Bitstream-Vera.txt` into
`LICENSES/`.
