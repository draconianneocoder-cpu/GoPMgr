// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"github.com/jung-kurt/gofpdf"

	"pmforge/internal/pdfmeta"
)

// PDF/A-3 metadata helpers (gofpdf-side adapter).
//
// The byte-level XMP work (BuildXMPPacket, InjectXMPStream, the
// incremental-update machinery) now lives in the dependency-free
// internal/pdfmeta package so it can be shared by both this package
// and internal/documents without an import cycle. This file keeps the
// thin gofpdf-specific glue: ApplyPDFAMetadata sets the library's
// documented metadata setters on a *gofpdf.Fpdf.
//
// Still NOT provided (V3 milestones, AGENT.md §8):
//   - Font embedding (ship a TTF; switch SetFont calls to it).
//   - OutputIntent / ICC profile embedding — the injection code
//     (InjectOutputIntent + MakePDFA3) is complete. Run `make icc` to
//     fetch the sRGB profile and get full PDF/A-3 conformance.
//   - veraPDF validation gate.

// XMPSpec is re-exported from pdfmeta so existing export-package
// callers keep their type reference without importing pdfmeta directly.
type XMPSpec = pdfmeta.XMPSpec

// ApplyPDFAMetadata delegates to the canonical implementation in pdfmeta
// and then overrides the Creator with the live application version.
func ApplyPDFAMetadata(pdf *gofpdf.Fpdf, spec XMPSpec) {
	pdfmeta.ApplyPDFAMetadata(pdf, spec)
	if pdf != nil {
		pdf.SetCreator("PMForge "+exportVersion(), true)
	}
}

// BuildXMPPacket delegates to pdfmeta. Retained as a thin shim so the
// existing export-package call sites (and tests) keep working.
func BuildXMPPacket(spec XMPSpec) []byte {
	return pdfmeta.BuildXMPPacket(spec)
}

// InjectXMPStream delegates to pdfmeta. Retained as a shim for
// export-package callers.
func InjectXMPStream(pdfBytes, xmpPacket []byte) ([]byte, error) {
	return pdfmeta.InjectXMPStream(pdfBytes, xmpPacket)
}

// MakePDFA3 re-exports the high-level PDF/A-3 (XMP + OutputIntent) helper
// so renderers inside the export package can use it without importing
// pdfmeta directly.
func MakePDFA3(pdfBytes []byte, spec XMPSpec, iccProfile []byte) ([]byte, error) {
	return pdfmeta.MakePDFA3(pdfBytes, spec, iccProfile)
}

// DefaultICCProfile re-exports the embedded sRGB profile accessor.
func DefaultICCProfile() []byte {
	return pdfmeta.DefaultICCProfile()
}

// HasDefaultICC reports whether an ICC profile was embedded at build time.
func HasDefaultICC() bool {
	return pdfmeta.HasDefaultICC()
}

// InjectPAdESSignature re-exports the real PAdES B-B embedding function.
// The signRanges callback will be invoked with the exact byte ranges
// that must be signed for a correct /ByteRange.
func InjectPAdESSignature(pdfBytes []byte, signRanges func([]byte) ([]byte, error)) ([]byte, error) {
	return pdfmeta.InjectPAdESSignature(pdfBytes, signRanges)
}

