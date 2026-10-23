<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC0-1.0
-->

# Bundled font binaries

This directory holds the TrueType (`.ttf`) font files that GoPMgr
embeds in generated PDFs. The `go:embed assets` directive in
`../manager.go` bundles whatever `.ttf` files are present here into the
compiled binary.

The four Source Sans 3 faces are committed as GoPMgr's release-critical
PDF/A baseline. Strict archival exports require an embedded TrueType font;
without these files, a clean build falls back to PDF core Helvetica and fails
veraPDF. `make required-font-assets` verifies that the baseline is tracked,
has a valid TrueType signature, and matches the reviewed Adobe release bytes
(Source Sans 3 version 3.052).

The other, larger catalog families are not committed. They keep their own
upstream licenses and are downloaded on demand from their upstream projects.
CI and release builds never download them, so release binaries embed only
Source Sans 3.

## Fetching the fonts

```sh
make fonts          # or: bash scripts/fetch-fonts.sh
```

This downloads every family listed in `../catalog.go` into this directory.
Each source is pinned to an immutable upstream commit, tag, or release
archive, and each download is checked against a SHA-256 recorded in the
script. Source Sans 3 is fetched from the same commit that
`scripts/check-required-font-assets.sh` verifies; `--force` re-downloads
files that already exist. After fetching, rebuild with `make build` and the
fonts are embedded.

The script downloads font files only, not license files. If you distribute a
build that embeds an optional family, include that family's upstream license
file from the table below.

## Graceful degradation

If an optional family's `.ttf` files are absent at build time, the font
Manager omits that family from `Available()`. A document export whose chosen
family is missing keeps fpdf's core Helvetica, which is not embedded. Report
exports always register Source Sans 3, which supported release builds always
embed, so archival renderers never reach the core font.

## Licenses

Every family is free for commercial AND personal use. Each license allows the
font to be bundled and embedded with any software, including GoPMgr, while
the font itself keeps its own license. Fonts embedded in documents you create
do not place those documents under the font license.

| Family | Version | License | Upstream copyright notice | License file |
| ------ | ------- | ------- | ------------------------- | ------------ |
| Source Sans 3 (committed) | 3.052 | `OFL-1.1` | © 2010–2024 Adobe, with Reserved Font Name "Source" | [LICENSE.md](https://github.com/adobe-fonts/source-sans/blob/87b37a2daaed80fcb8e8ccb0085c4d72ddade12e/LICENSE.md) |
| Liberation Sans, Serif, Mono | 2.1.5 | `OFL-1.1` | © 2010 Google Corporation (digitized data), with Reserved Font Arimo, Tinos and Cousine; © 2012 Red Hat, Inc., with Reserved Font Name Liberation | [LICENSE](https://github.com/liberationfonts/liberation-fonts/blob/2.1.5/LICENSE) |
| DejaVu Sans | 2.37 | `Bitstream-Vera` | © 2003 Bitstream, Inc.; Arev glyphs © 2006 Tavmjong Bah; DejaVu changes are public domain | [LICENSE](https://github.com/dejavu-fonts/dejavu-fonts/blob/version_2_37/LICENSE) |
| Noto Sans | 2.015 | `OFL-1.1` | © 2022 The Noto Project Authors | [OFL.txt](https://github.com/notofonts/latin-greek-cyrillic/blob/NotoSans-v2.015/OFL.txt) |
| JetBrains Mono | 2.304 | `OFL-1.1` | © 2020 The JetBrains Mono Project Authors | [OFL.txt](https://github.com/JetBrains/JetBrainsMono/blob/v2.304/OFL.txt) |
| Roboto | 3.016 | `OFL-1.1` | © 2011 The Roboto Project Authors | [OFL.txt](https://github.com/googlefonts/roboto-3-classic/blob/v3.016/OFL.txt) |
| Arimo | 1.33 | `OFL-1.1` | © 2020–2026 The Arimo Project Authors | [OFL.txt](https://github.com/googlefonts/arimo/blob/4a6255f269916ae7ad3fc2706b0935e7621396b8/OFL.txt) |
| Cousine | 1.241 | `OFL-1.1` | © 2026 The Cousine Project Authors | [OFL.txt](https://github.com/google/fonts/blob/e7db6ca603fd4862010383ff17ffd6cc4ffac1c0/ofl/cousine/OFL.txt) |
| Ledger | 1.003 | `OFL-1.1` | © 2011–2012 Denis Masharov, with Reserved Font Name "Ledger" | [OFL.txt](https://github.com/google/fonts/blob/e7db6ca603fd4862010383ff17ffd6cc4ffac1c0/ofl/ledger/OFL.txt) |

Notes:

- Roboto, Arimo, and Cousine were once distributed under `Apache-2.0`.
  Their current upstream releases, which the fetch script pins, are
  `OFL-1.1`.
- DejaVu Sans uses the SPDX `Bitstream-Vera` license. Its upstream `LICENSE`
  also includes the Arev Fonts license, which applies the same conditions,
  with Tavmjong Bah's name in place of Bitstream's, to the glyphs taken from
  the Arev fonts.
- The tracked Source Sans 3 faces are accompanied by `LICENSES/OFL-1.1.txt`.
  The same license identifiers and copyright notices are recorded in
  `REUSE.toml`; `TestCatalogLicensesMatchREUSE` keeps `../catalog.go` in step
  with it.
