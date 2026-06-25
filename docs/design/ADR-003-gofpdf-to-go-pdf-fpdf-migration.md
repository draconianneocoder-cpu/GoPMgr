<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-003: PDF library migration — jung-kurt/gofpdf → go-pdf/fpdf

**Status:** Implemented (2026-06-25)
**Date:** 2026-06-25
**Deciders:** James L. Burns (project owner)

## Context

PMForge uses a PDF generation library for 25 document kinds, all chart PDF
renderers, the PDF/A-3 export pipeline, and the PAdES signing adapter. As of
2026-06-25, the dependency was `github.com/jung-kurt/gofpdf v1.16.2`.

`jung-kurt/gofpdf` has been **archived** (read-only) since 2021. It receives
no security patches, no compatibility fixes for new Go toolchain versions, and
no bug fixes. Continuing to depend on an archived library:

- blocks `govulncheck` from detecting future CVEs in the library's transitive
  dependencies
- creates a maintenance cliff when a future Go toolchain version changes
  behaviour in code the archived library relies on
- sends a misleading signal to contributors and users about the health of
  PMForge's dependency tree

## Decision

Migrate from `github.com/jung-kurt/gofpdf` to `github.com/go-pdf/fpdf`, the
officially-blessed community continuation of the same library, maintained
under the `go-pdf` GitHub organisation.

**Chosen version:** `v0.9.0` (latest stable as of 2026-06-25).

## Options Considered

### A — Stay on jung-kurt/gofpdf

Archived, no maintenance path. Rejected outright.

### B — Migrate to go-pdf/fpdf (chosen)

The `go-pdf/fpdf` project is the direct continuation of `jung-kurt/gofpdf`.
It is referenced in the archived repo's own README as the recommended migration
target. Active commit history, community maintainership, MIT licence
(compatible with PMForge's GPL-3.0-or-later), no CGO dependency.

**Trade-offs:**
- Breaking change in package name: `gofpdf` → `fpdf`. All import paths and
  call-site selectors must be updated mechanically.
- Version number is `v0.9.0`, lower than the archived `v1.16.2` — this is
  cosmetic, not a regression. The library continues from the same code base.
- The exported method `(Style).GofpdfStyle()` in `internal/fonts/catalog.go`
  was renamed to `FpdfStyle()` for consistency.
- Transitive dependencies shift slightly (adds `boombuler/barcode`,
  `phpdave11/gofpdi`, `ruudk/golang-pdf417`). These were already indirect
  requirements via jung-kurt; `go mod tidy` resolves them correctly.

### C — Migrate to a different PDF library (UniPDF, maroto, gopdf, etc.)

The codebase has deep integration: 38 Go files call into the library for
vector primitives (Polygon, Line, Rect, Cell, MultiCell, Image, Transform*),
UTF-8 font loading, and PDF metadata injection. A full rewrite to a different
API would require weeks and would regress the PDF/A-3 and PAdES pipelines
before they could be rebuilt. Deferred to future consideration only if
go-pdf/fpdf maintenance lapses.

## Consequences

### Positive

- PMForge's dependency tree no longer contains an archived library.
- `govulncheck` can now detect and surface CVEs in the library's transitive
  dependencies.
- The `vuln` CI job (blocking govulncheck gate) is meaningful end-to-end.
- Future PDF features can be built on a maintained foundation.

### Negative / Required Follow-up

1. **`go mod tidy` required after checkout:** the `go.sum` entries for
   `jung-kurt/gofpdf` were removed and `go-pdf/fpdf v0.9.0` entries must be
   added. Run `go mod tidy` once after pulling this commit. CI will fail on
   missing sum entries until this is done.

2. **API surface check:** the full API used by PMForge was verified against the
   v0.9.0 source (`SetFont`, `Cell`, `MultiCell`, `Rect`, `Line`, `Polygon`,
   `Image`, `AddUTF8FontFromBytes`, `SetXY`, `GetStringWidth`, `TransformBegin`,
   `TransformEnd`, `TransformRotate`, `InitType`, `Fpdf`). All methods are
   present and signature-compatible.

3. **Renamed method:** `(Style).GofpdfStyle()` → `(Style).FpdfStyle()`.
   Any external code calling `GofpdfStyle()` must be updated.

4. **go-pdf/fpdf maintenance monitoring:** if go-pdf/fpdf activity stalls for
   more than 12 months, re-evaluate option C (alternative library). Track via
   GitHub repository pulse.

## Files Changed

- `go.mod` — replaced `github.com/jung-kurt/gofpdf v1.16.2` with
  `github.com/go-pdf/fpdf v0.9.0`
- `go.sum` — removed jung-kurt entries; new go-pdf/fpdf hashes added by
  `go mod tidy`
- 38 `.go` source files in `internal/charts/pdfrender/`,
  `internal/documents/`, `internal/export/`, `internal/pdfmeta/` —
  import path and package selector updated
- `internal/fonts/catalog.go` — `GofpdfStyle()` renamed to `FpdfStyle()`
- `internal/fonts/manager.go` — call sites updated to `FpdfStyle()`
- `internal/fonts/fonts_test.go` — test updated for `FpdfStyle()`
- All comment references to "gofpdf" updated to "fpdf" for accuracy
