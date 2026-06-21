// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "github.com/jung-kurt/gofpdf"

// DrawCompactSignatureBox draws a small, professional signature verification
// block at the current position on the PDF. Intended to be called at the end
// of the last content page for a more integrated "inline" look on key
// documents (SOW, Closure, Change Request, Combined Reports, etc.).
func DrawCompactSignatureBox(pdf *gofpdf.Fpdf, signerName, date string) {
	pdf.Ln(8)
	pdf.SetDrawColor(25, 55, 95)
	pdf.SetFillColor(248, 250, 253)
	pdf.SetTextColor(30, 30, 30)

	startY := pdf.GetY()
	pdf.Rect(20, startY, 170, 22, "DF")

	pdf.SetY(startY + 3)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(170, 5, "DIGITALLY SIGNED", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(15, 50, 85)
	pdf.CellFormat(100, 6, signerName, "", 0, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(70, 6, date, "", 1, "R", false, 0, "")

	pdf.SetY(startY + 14)
	pdf.SetFont("Helvetica", "I", 7)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(170, 5, "PAdES B-B signature — content is tamper-evident", "", 1, "L", false, 0, "")
}
