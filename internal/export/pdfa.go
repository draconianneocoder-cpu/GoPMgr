// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"gopmgr/internal/pdfmeta"
)

// PDF/A-2u metadata helpers (fpdf-side adapter).
//
// The byte-level work (font repair, XMP, OutputIntent, and the
// incremental-update machinery) lives in the dependency-free
// internal/pdfmeta package so internal/documents and this package share
// it without an import cycle. Export renderers call tagPDFA (pdf.go),
// which wraps pdfmeta.TagPDFA.

// XMPSpec is re-exported from pdfmeta so export-package renderers can
// describe their metadata without importing pdfmeta directly.
type XMPSpec = pdfmeta.XMPSpec
