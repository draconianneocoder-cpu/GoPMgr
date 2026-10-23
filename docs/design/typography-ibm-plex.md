<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Typography: the IBM Plex superfamily

**Status:** Implemented.
**Scope:** The app interface, PDF documents and reports, and the HTML, DOCX,
ODT, and XLSX exports.

GoPMgr bundles the Latin members of the IBM Plex superfamily and uses them by
default everywhere it draws or writes text. This note records which Plex style
serves each role and why, so later changes keep the system consistent.

## Bundled families and versions

| Family | Version | Role | Where it ships |
| --- | --- | --- | --- |
| IBM Plex Sans | 3.005 | Default text everywhere | App (WOFF2) and PDFs (TTF) |
| IBM Plex Mono | 2.005 | Codes, identifiers, code samples | App (WOFF2) and PDFs (TTF) |
| IBM Plex Serif | 3.006 | Selectable document font | PDFs (TTF) |
| IBM Plex Sans Condensed | 3.000 | Selectable document font for dense tables | PDFs (TTF) |

The files come from IBM's release tags in
[github.com/IBM/plex](https://github.com/IBM/plex) (TTF) and the matching npm
packages (WOFF2). Each WOFF2 face has the same version, glyph set, character
map, advance widths, and outlines as its TTF, so the app and its PDFs show the
same letterforms. `scripts/check-required-font-assets.sh` pins every committed
file by SHA-256.

The script families (Arabic, Devanagari, Hebrew, Thai, and the Japanese,
Korean, and Chinese families) are not bundled. The PDF engine cannot shape
Arabic, Indic, or Thai text or lay out right-to-left lines, and the CJK
families are several megabytes per weight.

## PDF documents and reports

Renderers select a font by family key: `"Helvetica"` for all text and
`"Courier"` for the recovery-code sheet. GoPMgr registers Plex faces under those
keys, so no renderer names a font itself. The engine offers four styles per
family, mapped as follows.

| Renderer style | Use in GoPMgr PDFs | Plex face |
| --- | --- | --- |
| Regular (`""`) | Body text, table cells, field values | Regular (400) |
| Bold (`"B"`) | Headings, table headers, labels, totals | SemiBold (600) |
| Italic (`"I"`) | Notes, captions, footers | Italic (400) |
| Bold italic (`"BI"`) | Emphasized notes | SemiBold Italic (600) |

Every Plex family in the catalog uses the same mapping.

Why these faces:

- **Regular for text.** Plex Sans Regular sets only 1.7% wider than the
  Helvetica metrics the renderers were laid out against, so existing column
  widths and wrapping still fit. Its large x-height (0.74 of the cap height)
  keeps 7–9 pt table text legible in print and on screen.
- **SemiBold for "bold".** SemiBold is IBM's own heading weight. It sets about
  1% narrower than Helvetica Bold, so headings and table headers keep fitting
  their columns. Plex Bold sets about the same width but looks heavier, which
  crowds dense tables.
- **Figures line up.** Plex's default digits are tabular, 600 units wide in
  every weight of every family. Costs, dates, and percentages align in columns,
  even where a bold total sits under regular values.
- **Mono for recovery codes.** Recovery codes use Base32 (A–Z, 2–7), which
  contains the look-alike pairs S/5, Z/2, and G/6. Plex Mono keeps them
  distinct, and its advance width matches Courier's, so the sheet's layout is
  unchanged.

Defaults:

- Project documents use the project's chosen document font. When none is
  chosen, or the chosen font is not available in the build, they use IBM Plex
  Sans. They never fall back to an unembedded PDF standard font.
- Report exports (schedule PDF, Monte Carlo, Six Sigma) always use IBM Plex
  Sans, which is also the PDF/A baseline.
- The recovery-code sheet uses IBM Plex Sans for text and IBM Plex Mono for the
  codes.

## The app

`frontend/src/app.css` defines the Tailwind font tokens, so every existing
utility class resolves to a Plex face without per-component edits.

| Utility or element | Use | Plex face |
| --- | --- | --- |
| Default text, `font-normal` | Body text, table cells, inputs | Sans Regular (400) |
| `font-medium` | Buttons, tabs, field labels | Sans Medium (500) |
| `font-semibold` | Section headings | Sans SemiBold (600) |
| `font-bold`, `<strong>` | Page titles, key figures | Sans Bold (700) |
| `italic`, `<em>` | Secondary notes | Sans Italic (400) |
| `font-mono`, `<code>`, `<pre>` | Paths, identifiers, hashes, code | Mono Regular, Medium, SemiBold, Bold |

The app uses four weights, so each one gets its own designed face rather than
a weight the browser synthesizes. Monospace text inherits the weight of the
label around it, so Mono ships in the same four weights. Charts drawn on canvas
use Plex Sans explicitly; SVG charts inherit it from the page.

The four Sans weights are preloaded in `frontend/index.html`, so the first
screen renders in Plex without a flash of a fallback font. Mono loads when a
screen first uses it.

Serif and Sans Condensed are not shipped to the app, because no screen uses
them. To use one, add its WOFF2 faces and a `--font-serif` or similar token in
`app.css`.

## Other exports

| Export | Font setting |
| --- | --- |
| HTML schedule report | `"IBM Plex Sans"` first in the font stack, headings and table headers at 600 |
| XLSX | Workbook default font IBM Plex Sans |
| DOCX, ODT documents | The project's document font (default IBM Plex Sans) for body and headings |
| DOCX, ODT reports | IBM Plex Sans |

These formats name the font but do not embed it. A recipient without IBM Plex
installed sees a similar font chosen by their software. DOCX and ODT files
record whether the font is sans-serif, serif, or monospaced to guide that
choice.

## Changing the system

- Keep the 400/600 mapping for every PDF family, so documents stay consistent
  when a user switches between Plex families.
- When updating a Plex version, update the TTF and WOFF2 files together, check
  that they still match, and update the SHA-256 values in
  `scripts/check-required-font-assets.sh` and `scripts/fetch-fonts.sh`.
- Do not use the weights 100–300 for body text; they are display weights.
- Measure text in the current font instead of assuming a width. fpdf does not
  clip cell text, so a fixed label column that fits one face overflows a wider
  one; `kvLabelWidth` widens key/value labels for that reason.
