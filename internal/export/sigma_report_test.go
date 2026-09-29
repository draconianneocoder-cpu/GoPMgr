// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"strings"
	"testing"

	"gopmgr/internal/sigma/domain"
)

// GenerateSigmaReportPDF is what the app exports through the save dialog:
// it returns PDF bytes and a suggested name and touches no files.
func TestGenerateSigmaReportPDFReturnsAPDFAndItsName(t *testing.T) {
	pdfBytes, filename, err := GenerateSigmaReportPDF(
		domain.Project{Title: "Line 3 / Scrap", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusActive},
		nil, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("report does not start with a PDF header: %q", pdfBytes[:min(len(pdfBytes), 8)])
	}
	if !strings.HasPrefix(filename, "sigma_report_") || !strings.HasSuffix(filename, ".pdf") || strings.ContainsAny(filename, `/\`) {
		t.Fatalf("filename = %q, want sigma_report_<title>_<time>.pdf with no path separators", filename)
	}
}
