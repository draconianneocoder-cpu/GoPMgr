// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"errors"

	"github.com/go-pdf/fpdf"
)

// RecoverySheet is a printable page of one account's recovery codes.
type RecoverySheet struct {
	Username string
	Created  string // YYYY-MM-DD
	Codes    []string
}

// Recovery sheet wording. The sign-in link label must match Login.svelte.
const (
	recoverySheetTitle = "GoPMgr recovery codes"
	recoverySheetUse   = "If you forget your password, a recovery code lets you set a new one and keep your encrypted projects. " +
		"On the GoPMgr sign-in screen, choose \"Forgot password? Use a recovery code\", then enter your username and one code."
	recoverySheetOnce = "Each code works once. Tick it off when you use it."
	recoverySheetKeep = "Keep this sheet somewhere safe, away from this computer: anyone with your username and a code can reset your password. " +
		"If you saved this sheet as a file to print it, delete that file now."
	recoverySheetRenew = "When you create new recovery codes in App Settings, under Account, these codes stop working. Destroy this sheet then."
)

// RenderRecoverySheetPDF lays the codes out on one portrait page, each with
// a box to tick off.
//
// Unlike project documents it does not use newDocPDF: that stamps PDF/A
// metadata and swaps in the user's embedded font only while a project is
// open, and this sheet is often made at account creation, before any. It
// uses fpdf's standard Helvetica and Courier, which every PDF viewer has,
// and makes no PDF/A claim: the sheet is meant to be printed, not archived.
// The content fits within the margins of both A4 and US Letter.
func RenderRecoverySheetPDF(sheet RecoverySheet) ([]byte, error) {
	if len(sheet.Codes) == 0 {
		return nil, errors.New("recovery sheet needs at least one code")
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(false, 20)
	pdf.SetTitle(recoverySheetTitle, true)
	pdf.SetCreator("GoPMgr", true)
	pdf.AddPage()
	width, _ := pdf.GetPageSize()
	textWidth := width - 40

	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 9, recoverySheetTitle)
	pdf.Ln(11)
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 6, "Account: "+sheet.Username)
	pdf.Ln(6)
	pdf.Cell(0, 6, "Created: "+sheet.Created)
	pdf.Ln(10)
	pdf.MultiCell(textWidth, 5.5, recoverySheetUse, "", "L", false)
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 6, recoverySheetOnce)
	pdf.Ln(10)

	pdf.SetFont("Courier", "", 14)
	for _, code := range sheet.Codes {
		x, y := pdf.GetXY()
		pdf.Rect(x, y+1.5, 5, 5, "D")
		pdf.SetX(x + 10)
		pdf.Cell(0, 8, code)
		pdf.Ln(10)
	}

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(textWidth, 5, recoverySheetKeep, "", "L", false)
	pdf.Ln(2)
	pdf.MultiCell(textWidth, 5, recoverySheetRenew, "", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
