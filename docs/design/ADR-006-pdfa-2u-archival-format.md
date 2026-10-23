<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-006: PDF/A-2u as the archival PDF format

**Status:** Implemented
**Decision date:** 2026-10-23
**Deciders:** GoPMgr maintainer

## Context

GoPMgr labelled every archival PDF PDF/A-3b. Three facts made that label the
wrong one:

- **PDF/A-3 adds only attachments.** ISO 19005-3 differs from ISO 19005-2 in
  one respect: it allows embedded files of any format. GoPMgr embeds none.
- **Archives prefer PDF/A-2 for documents.** NARA's transfer guidance
  (updated August 2025) lists PDF/A-1 and PDF/A-2 as preferred formats for
  born-digital textual records. It lists PDF/A-3 only for case files, and its
  digitized-paper guidance says not to use PDF/A-3 attachments.
- **Level B promises appearance only.** Level U adds that every character
  maps to Unicode, so text can be searched, copied, and read aloud.

Validating real output with veraPDF also showed that GoPMgr could not claim
level U as it stood, and that signed files met no level:

- go-pdf/fpdf writes one ToUnicode bfrange, `<0000> <FFFF> <0000>`, for every
  UTF-8 font. ISO 32000-1 9.10.3 lets a bfrange vary only the last byte, so
  strict readers map no character above U+00FF: dashes, curly quotes,
  bullets, the euro sign, Greek, Cyrillic.
- `InjectPAdESSignature` wrote its trailer without `/ID`, which ISO 19005-2
  6.1.3 requires, so every signed PDF failed PDF/A at every level. The PDF/A
  gate validated only unsigned files, and the PAdES gate signed a file that
  was not PDF/A, so neither gate could see it.

## Decision

Declare PDF/A-2u (`pdfaid:part` 2, `pdfaid:conformance` U) for every archival
PDF, and fix both defects in GoPMgr's own PDF/A step.

## Options considered

### Option A: PDF/A-2u (chosen)

| Dimension | Assessment |
| --- | --- |
| Archive acceptance | Preferred by NARA for textual records |
| Fit | Claims exactly what GoPMgr produces: no attachments |
| Work | Font repair and trailer fix, needed for any level U |
| Limitation | An export that attaches files needs part 3 or PDF/A-4f |

### Option B: PDF/A-3u

| Dimension | Assessment |
| --- | --- |
| Archive acceptance | Not in NARA's preferred list for textual records |
| Fit | Claims an attachment capability GoPMgr does not use |
| Work | The same as Option A |
| Advantage | Ready for attachments, which nothing plans |

### Option C: keep PDF/A-3b

Rejected. It leaves signed files invalid, keeps text outside Latin-1
unmapped, and keeps the weaker archive fit.

PDF/A-2a and 3a need tagged PDF structure, which fpdf cannot write. PDF/A-4
drops the B and U levels, and the Library of Congress notes that no PDF
processor yet claims PDF/A-4 conformance.

## Pipeline

```text
renderer: *pdfdoc.PDF            every string passes through SafeText
   │                             (emoji become U+FFFD; fpdf cannot encode them)
   ▼
raw fpdf output
   │
   ▼ pdfmeta.MakePDFA (called through pdfmeta.TagPDFA)
   │  1. binary header comment
   │  2. RepairFPDFFonts   incremental update: valid ToUnicode CMaps, and
   │                       fonts named "ABCDEF+IBMPlexSans", not "utf8helvetica"
   │  3. XMP identifier    part 2, conformance U
   │  4. OutputIntent      embedded sRGB profile
   ▼
optional signing.ApplyPAdES      final incremental update; keeps /ID
```

## Trade-off analysis

- **Part 2 or part 3.** Both parts apply the same rules to a file without
  attachments, so the choice changes only what the file claims. The narrower
  claim matches the files and the archives' preference.
- **Repair or fork fpdf.** Forking fpdf is a dependency decision
  (DEPENDENCIES.md). The repair is GoPMgr-owned code in `internal/pdfmeta`,
  next to the other PDF/A steps, and a test fails if fpdf changes the CMap it
  writes.
- **Incremental update or rewrite.** Appending new revisions of the CMap and
  font objects reuses the tested incremental-update code and keeps fpdf's
  bytes unchanged. The superseded objects stay in the file, a few kilobytes.

## Consequences

- Archives that prefer PDF/A-2 accept GoPMgr's PDFs, and signed documents are
  valid PDF/A-2u.
- Text in any script the font covers copies, searches, and reads aloud
  correctly. PDF font lists show the real font, for example
  `PVUUJE+IBMPlexSans`.
- A future export that attaches files must declare part 3, or PDF/A-4f, and
  needs its own gate sample.
- Revisit when go-pdf/fpdf changes its font output (the repair test pins
  it), or when PDF/A-4 processors become common.

## Verification

- `make check-pdfa` validates six samples against veraPDF's PDF/A-2u profile,
  including a PAdES-signed document and one with Greek, Cyrillic, Vietnamese,
  symbols, and emoji.
- `internal/pdfmeta` tests check every repaired CMap against ISO 32000-1
  9.10.3 and the PDF/A-2 forbidden values, the font names, idempotence, and
  the signed trailer's `/ID`; each fails when its fix is reverted.
- `internal/pdfdoc` tests drive every text method with emoji, and
  `boundary_test.go` fails if a renderer bypasses `pdfdoc.PDF`.
