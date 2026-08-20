// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"gopmgr/internal/pdfmeta"
)

// PDF/A-3 metadata helpers (fpdf-side adapter).
//
// The byte-level XMP work (BuildXMPPacket, InjectXMPStream, the
// incremental-update machinery) lives in the dependency-free
// internal/pdfmeta package so it can be shared by both this package
// and internal/documents without an import cycle. This file re-exports
// the pieces export-package renderers (pdf.go, sigma_report.go,
// montecarlo_report.go) actually call, so they don't need to import
// pdfmeta directly.
//
// PDF/A-3 support is complete: fonts are embedded (internal/fonts,
// registered in pdf.go), OutputIntent/ICC profile embedding is wired
// (InjectOutputIntent + MakePDFA3, with the sRGB profile committed and
// embedded at build time in internal/pdfmeta), and veraPDF validation
// runs as a release gate (make check-pdfa).

// XMPSpec is re-exported from pdfmeta so existing export-package
// callers keep their type reference without importing pdfmeta directly.
type XMPSpec = pdfmeta.XMPSpec

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
