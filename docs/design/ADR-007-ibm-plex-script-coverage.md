<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-007: Which IBM Plex packages GoPMgr bundles

**Status:** Implemented; amended by [ADR-008](ADR-008-localization.md),
which adds IBM Plex Sans JP and KR to the app for a Japanese and Korean
interface.
**Decision date:** 2026-10-23
**Deciders:** GoPMgr maintainer

## Context

GoPMgr bundled IBM Plex Sans, Serif, Mono, and Sans Condensed, which cover
Latin, Greek, and Cyrillic. Text in any other script printed in PDFs as
missing-glyph boxes, and the app fell back to system fonts for it. Users
enter names, tasks, and notes in their own languages, and GoPMgr already ships
a Japanese holiday calendar.

[IBM/plex](https://github.com/IBM/plex) publishes 17 packages. Two engines
use fonts, with different abilities:

- **The app** runs in the platform web view, which shapes every script and
  lays out right-to-left text.
- **PDFs** come from go-pdf/fpdf, which draws one glyph per character from
  the font's cmap format 4 subtable, left to right, with no shaping.

Each candidate was rendered through fpdf, validated with veraPDF, and viewed.

## Decision

| Package | App (WOFF2) | PDFs (TrueType) | Why |
| --- | --- | --- | --- |
| Sans, Mono | Yes | Yes | Already the default text and code faces |
| Serif, Sans Condensed | No | Yes | Document choices; no screen uses them |
| Sans JP, KR, SC | No | Yes, Regular and SemiBold | fpdf renders them correctly; the web view already has system CJK fonts |
| Sans Arabic, Hebrew, Devanagari, Thai | Yes, 4 weights | No | The web view shapes them; fpdf cannot |
| Sans TC | No | No | IBM's TrueType files leave cmap format 4 empty, so fpdf draws only boxes |
| Sans Thai Looped | No | No | An alternative Thai style; Plex Sans Thai matches Plex Sans |
| Sans, Serif, Mono Variable | No | No | fpdf cannot use variable fonts; the app's four static weights suffice |
| Math | No | No | GoPMgr typesets no mathematics |

Display weights (Thin, ExtraLight, Light) and the Text weight stay out for the
reason in the typography note: no role uses them.

## Evidence

- **fpdf with the script families.** Arabic letters did not join, and Arabic
  and Hebrew ran left to right. Devanagari showed a visible virama and misplaced
  vowel signs. Thai tone marks vanished into the vowel below them, which
  changes the word. All of it is legible as glyphs and wrong as text.
- **fpdf with CJK.** Japanese, Korean, and Simplified Chinese render correctly,
  embed as subsets (a charter is under 100 KB), extract as written, and pass
  veraPDF PDF/A-2u. Traditional Chinese drew every character, Latin included,
  as a box: its format 4 cmap has no entries, and only the format 12 table maps
  characters.
- **The web view.** In Chromium each script shapes correctly, and a page loads
  only the faces its text needs, because each script face's `unicode-range`
  covers only its script.

## Options considered for CJK in PDFs

### Option A: bundle Regular and SemiBold (chosen)

| Dimension | Assessment |
| --- | --- |
| Size | About 33 MB more in the repository and the binary |
| Offline | Works with no network, like every other GoPMgr feature |
| Quality | Headings keep IBM's SemiBold weight |

### Option B: download on demand

Smaller binary, but a network path, an integrity check, and a new UI flow in a
local-first app, for fonts users need to print their own language.

### Option C: Regular only

About 17 MB, but headings and table headers lose their weight.

## Consequences

- Documents in Japanese, Korean, or Simplified Chinese print their text when
  the project's document font is set to that language's IBM Plex Sans.
- The app shows Arabic, Hebrew, Devanagari, and Thai text in IBM Plex.
- PDFs still cannot lay out Arabic, Hebrew, Devanagari, or Thai, or show
  Traditional Chinese in IBM Plex. Settings and Help say to export such
  documents as DOCX or ODT.
- The CJK families have no U+FFFD glyph, so an emoji that `pdfdoc.SafeText`
  replaces prints as an empty box rather than �. Copied text still reads �.
- The repository and the binary grow by about 34 MB.

Revisit when:

- IBM ships IBM Plex Sans TC with a populated format 4 cmap.
- GoPMgr gains text shaping. IBM Plex Sans Arabic maps the Arabic
  presentation forms, so joining and right-to-left ordering in GoPMgr would be
  enough for basic Arabic.

## Verification

- `internal/fonts/cmap_test.go` reads each committed face's cmap the way fpdf
  does, and fails on the Traditional Chinese file.
- `internal/documents` renders a charter in each CJK family and checks that
  every character maps to a real glyph.
- `make check-pdfa` validates a Japanese, a Korean, and a Chinese charter.
- `frontend/src/lib/typography.test.ts` checks each script face's
  `unicode-range` and the font stack order.
