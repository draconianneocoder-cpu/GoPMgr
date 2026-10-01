// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

var (
	pdfStreamPattern = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfTextPattern   = regexp.MustCompile(`\(((?:\\.|[^\\)])*)\)\s*Tj`)
	pdfPagePattern   = regexp.MustCompile(`/Type /Page\b[^s]`)
)

// recoverySheetText returns the sheet's shown text in page order, joined by
// spaces. The sheet uses core fonts only, so each string is a literal
// "(...) Tj" in an inflated content stream.
func recoverySheetText(t *testing.T, pdf []byte) string {
	t.Helper()
	var words []string
	for _, m := range pdfStreamPattern.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // not a compressed content stream
		}
		content, err := io.ReadAll(r)
		if err != nil {
			continue
		}
		for _, tj := range pdfTextPattern.FindAllSubmatch(content, -1) {
			unescaped := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`).Replace(string(tj[1]))
			words = append(words, unescaped)
		}
	}
	return strings.Join(words, " ")
}

func sampleRecoverySheet() RecoverySheet {
	return RecoverySheet{
		Username: "alice",
		Created:  "2026-10-01",
		Codes: []string{
			"AAAAAAAA-BBBBBBBB", "CCCCCCCC-DDDDDDDD", "EEEEEEEE-FFFFFFFF", "GGGGGGGG-HHHHHHHH",
			"IIIIIIII-JJJJJJJJ", "KKKKKKKK-LLLLLLLL", "MMMMMMMM-NNNNNNNN", "OOOOOOOO-PPPPPPPP",
		},
	}
}

func TestRenderRecoverySheetPDFShowsEveryCodeAndHowToUseThem(t *testing.T) {
	sheet := sampleRecoverySheet()
	pdf, err := RenderRecoverySheetPDF(sheet)
	if err != nil {
		t.Fatalf("RenderRecoverySheetPDF: %v", err)
	}
	if out := os.Getenv("GOPMGR_RECOVERY_SHEET_SAMPLE"); out != "" {
		if err := os.WriteFile(out, pdf, 0o600); err != nil {
			t.Fatalf("write sample: %v", err)
		}
	}
	if pages := len(pdfPagePattern.FindAll(pdf, -1)); pages != 1 {
		t.Fatalf("sheet has %d pages, want 1", pages)
	}

	text := strings.Join(strings.Fields(recoverySheetText(t, pdf)), " ")
	for _, want := range append([]string{
		"GoPMgr recovery codes",
		"Account: alice",
		"Created: 2026-10-01",
		`choose "Forgot password? Use a recovery code"`,
		"Each code works once. Tick it off when you use it.",
		"anyone with your username and a code can reset your password",
		"delete that file now",
		"these codes stop working",
	}, sheet.Codes...) {
		if !strings.Contains(text, want) {
			t.Errorf("sheet text is missing %q\ntext: %s", want, text)
		}
	}
	// Codes appear in the order given, so ticking them off matches the list.
	last := -1
	for _, code := range sheet.Codes {
		i := strings.Index(text, code)
		if i <= last {
			t.Fatalf("code %s is out of order", code)
		}
		last = i
	}
}

func TestRenderRecoverySheetPDFRefusesAnEmptySheet(t *testing.T) {
	if _, err := RenderRecoverySheetPDF(RecoverySheet{Username: "alice", Created: "2026-10-01"}); err == nil {
		t.Fatal("rendered a sheet with no codes")
	}
}
