// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
//   - OutputIntent / ICC profile embedding.
//   - veraPDF validation gate.

// XMPSpec is re-exported from pdfmeta so existing export-package
// callers keep their type reference without importing pdfmeta directly.
type XMPSpec = pdfmeta.XMPSpec

// ApplyPDFAMetadata sets the gofpdf-supported metadata fields. Call
// this immediately after pdf := gofpdf.New(...) in any renderer.
//
// The function is a no-op if pdf is nil so callers can chain it
// without nil-checks.
func ApplyPDFAMetadata(pdf *gofpdf.Fpdf, spec XMPSpec) {
	if pdf == nil {
		return
	}
	if spec.Title != "" {
		pdf.SetTitle(spec.Title, true)
	}
	if spec.Author == "" {
		spec.Author = "PMForge"
	}
	pdf.SetAuthor(spec.Author, true)
	if spec.Subject != "" {
		pdf.SetSubject(spec.Subject, true)
	}
	pdf.SetCreator("PMForge "+exportVersion(), true)
	if len(spec.Keywords) > 0 {
		pdf.SetKeywords(strJoin(spec.Keywords, ", "), true)
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

// strJoin avoids dragging in strings just for the single Join call.
func strJoin(xs []string, sep string) string {
	if len(xs) == 0 {
		return ""
	}
	out := xs[0]
	for _, x := range xs[1:] {
		out += sep + x
	}
	return out
}
