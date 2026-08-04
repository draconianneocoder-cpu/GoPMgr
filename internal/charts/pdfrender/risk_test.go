// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestRenderChartToPDFRiskMatrixProducesPDF(t *testing.T) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	err := RenderChartToPDF(
		pdf,
		"risk_matrix",
		`{"items":[{"id":"R-1","title":"Supplier delay","kind":"risk","probability":4,"impact":5}]}`,
		"Project exposure",
		Frame{X: 15, Y: 15, W: 267, H: 170},
	)
	if err != nil {
		t.Fatalf("RenderChartToPDF(risk_matrix): %v", err)
	}
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output: %v", err)
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
		t.Fatalf("output prefix = %q, want PDF header", out.Bytes()[:5])
	}
}
