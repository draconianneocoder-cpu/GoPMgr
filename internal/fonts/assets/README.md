<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC0-1.0
-->

# Bundled font binaries

This directory holds the TrueType (`.ttf`) font files that GoPMgr
embeds in generated PDFs. The `go:embed assets` directive in
`../manager.go` bundles whatever `.ttf` files are present here into the
compiled binary, along with `FONT-NOTICES.txt`, the copyright notices and
license text of the committed fonts.

These faces are committed, so every build bundles them:

- **IBM Plex Sans, Serif, Mono, and Sans Condensed**, four faces each
  (Regular, SemiBold, Italic, SemiBold Italic). IBM Plex Sans is the default
  document font and the release-critical PDF/A baseline; IBM Plex Mono sets
  the recovery-code sheet. The app's own interface uses the matching WOFF2
  faces in `frontend/public/fonts/ibm-plex/`.
- **IBM Plex Sans JP, KR, and SC**, Regular and SemiBold, unhinted, so
  documents in Japanese, Korean, and Simplified Chinese print their text.
  They add about 33 MB. CJK type has no italics, so the italic styles use the
  upright faces.
- **Source Sans 3**, four faces, kept as a document font choice.

The app also ships IBM Plex Sans Arabic, Hebrew, Devanagari, and Thai as WOFF2
only (Regular, Medium, SemiBold, Bold), for text in those scripts on screen.
PDFs cannot use them: the PDF engine does not shape these scripts. See
`docs/design/ADR-007-ibm-plex-script-coverage.md` for what is bundled where,
and why IBM Plex Sans TC is not.

Strict archival exports require an embedded TrueType font; without these
files, PDFs would fall back to the PDF core fonts and fail veraPDF.
`make required-font-assets` verifies that every committed font file is
tracked, has a valid signature, and matches its reviewed upstream bytes. See
`docs/design/typography-ibm-plex.md` for which Plex face serves each role.

The other, larger catalog families are not committed. They keep their own
upstream licenses and are downloaded on demand from their upstream projects.
CI and release builds never download them, so release binaries embed only
IBM Plex and Source Sans 3.

## Fetching the fonts

```sh
make fonts          # or: bash scripts/fetch-fonts.sh
```

This downloads every family listed in `../catalog.go` into this directory.
Each source is pinned to an immutable upstream commit, tag, or release
archive, and each download is checked against a SHA-256 recorded in the
script. The committed fonts are fetched from the same commits that
`scripts/check-required-font-assets.sh` verifies; `--force` re-downloads
files that already exist. After fetching, rebuild with `make build` and the
fonts are embedded.

The script downloads font files only, not license files. If you distribute a
build that embeds an optional family, include that family's upstream license
file from the table below.

## Defaults and fallbacks

If an optional family's `.ttf` files are absent at build time, the font
Manager omits that family from `Available()`. A document whose chosen family
is missing, or that has no chosen family, uses IBM Plex Sans. Report exports
always use IBM Plex Sans. No GoPMgr PDF falls back to fpdf's unembedded core
fonts.

## Licenses

Every family is free for commercial AND personal use. Each license allows the
font to be bundled and embedded with any software, including GoPMgr, while
the font itself keeps its own license. Fonts embedded in documents you create
do not place those documents under the font license.

| Family | Version | License | Upstream copyright notice | License file |
| ------ | ------- | ------- | ------------------------- | ------------ |
| IBM Plex Sans (committed) | 3.005 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans/LICENSE.txt) |
| IBM Plex Serif (committed) | 3.006 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/434af578549afcfdabd281f386e0ff7314fd20b0/packages/plex-serif/LICENSE.txt) |
| IBM Plex Mono (committed) | 2.005 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/2f9ba1b25957d958db71a849e85d72e3ecfb845a/packages/plex-mono/LICENSE.txt) |
| IBM Plex Sans Condensed (committed) | 3.000 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/bb3ab6404e1881ea286f8742dc839e09057db6dd/packages/plex-sans-condensed/LICENSE.txt) |
| IBM Plex Sans JP (committed) | 1.004 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/89cba80dad75561262e758f4b6ddd474c5119796/packages/plex-sans-jp/LICENSE.txt) |
| IBM Plex Sans KR (committed) | 1.003 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans-kr/LICENSE.txt) |
| IBM Plex Sans SC (committed) | 1.000 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/222657a016f31e6d913ede88520de7dec84dc81b/packages/plex-sans-sc/LICENSE.txt) |
| IBM Plex Sans Arabic (app, WOFF2) | 1.005 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans-arabic/LICENSE.txt) |
| IBM Plex Sans Hebrew (app, WOFF2) | 1.3 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans-hebrew/LICENSE.txt) |
| IBM Plex Sans Devanagari (app, WOFF2) | 1.2 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans-devanagari/LICENSE.txt) |
| IBM Plex Sans Thai (app, WOFF2) | 1.2 | `OFL-1.1` | © 2017 IBM Corp., with Reserved Font Name "Plex" | [LICENSE.txt](https://github.com/IBM/plex/blob/1da12f02587b630c07e92692d21492d722f53614/packages/plex-sans-thai/LICENSE.txt) |
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

- The IBM Plex font files carry IBM Corp. copyright lines from 2017 (Mono),
  2018 (Sans, JP, KR, SC), 2019 (Sans Condensed, Arabic, Hebrew, Devanagari,
  Thai), and 2020 (Serif); IBM's license file for every family reads 2017,
  and every package's license file is identical. `REUSE.toml` records the
  full range.
- Roboto, Arimo, and Cousine were once distributed under `Apache-2.0`.
  Their current upstream releases, which the fetch script pins, are
  `OFL-1.1`.
- DejaVu Sans uses the SPDX `Bitstream-Vera` license. Its upstream `LICENSE`
  also includes the Arev Fonts license, which applies the same conditions,
  with Tavmjong Bah's name in place of Bitstream's, to the glyphs taken from
  the Arev fonts.
- The committed faces are accompanied by `LICENSES/OFL-1.1.txt` and by
  `FONT-NOTICES.txt`; the app's web fonts are accompanied by IBM's license in
  `frontend/public/fonts/ibm-plex/OFL.txt`. The same license identifiers and
  copyright notices are recorded in `REUSE.toml`;
  `TestCatalogLicensesMatchREUSE` keeps `../catalog.go` in step with it.
- Every font above sets OS/2 `fsType` 0 (installable), so none restricts
  embedding or subsetting. Liberation 2.1.5 was not checked.

## No proprietary fonts

GoPMgr ships no proprietary fonts. The committed IBM Plex and Source Sans 3
files are the only font files in the repository, in the release binary, and in
the frontend bundle. No PDF, SVG, Office file, or data URI in the repository
embeds a font.

Some outputs name common system fonts without including them, and the viewer
then uses a font it already has:

- Every GoPMgr PDF embeds its fonts, named after the embedded face with a
  subset tag, such as `PVUUJE+IBMPlexSans`. The renderers select fonts
  through fpdf's "Helvetica" family key, and `pdfmeta.RepairFPDFFonts`
  replaces the internal name fpdf derives from it (`utf8helvetica`).
- DOCX, ODT, XLSX, and HTML exports set their text in IBM Plex. Microsoft
  font names remain where GoPMgr's text does not use them: the DOCX font table
  and the DOCX theme's slots for East Asian and other scripts Plex does not
  cover, and the XLSX theme, which Excel applies only to content a user adds
  with theme fonts. The app's interface lists operating-system fonts after
  IBM Plex in its font stacks, as fallbacks for emoji and other characters
  Plex lacks.

Imported fonts belong to the user. `ImportFont` checks that a file is a
TrueType font and refuses one whose OS/2 `fsType` forbids the way GoPMgr
embeds fonts: marked restricted-license, not to be subset, or bitmap-only.
It does not check the font's license itself.
