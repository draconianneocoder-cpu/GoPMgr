// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"log"

	"gopmgr/internal/pdfmeta"
)

// tagPDFA makes a rendered document PDF/A-2u. If tagging fails, the
// document still goes out, untagged, and the failure is logged: a readable
// document is more useful than none, and the file never claims PDF/A it
// lacks.
func tagPDFA(raw []byte, spec pdfmeta.XMPSpec) []byte {
	tagged, err := pdfmeta.TagPDFA(raw, spec)
	if err != nil {
		log.Printf("[documents] PDF/A tagging failed, exporting an untagged PDF: %v", err)
	}
	return tagged
}
