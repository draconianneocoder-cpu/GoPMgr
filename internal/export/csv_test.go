// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"pmforge/internal/kernel"
)

// TestRenderCSVNeutralizesFormulaInjection locks in F-3 from the 2026-06-29
// security review: a user-controlled task title that begins with a formula
// trigger must be written as text, not as a live spreadsheet formula
// (CWE-1236).
func TestRenderCSVNeutralizesFormulaInjection(t *testing.T) {
	payload := ReportPayload{
		Tasks: map[string]*kernel.Task{
			"t1": {ID: "t1", Title: "=cmd|'/c calc'!A1"},
		},
	}
	out, err := renderCSV(payload, ExportOptions{})
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	s := string(out)
	if strings.Contains(s, ",=cmd") {
		t.Fatalf("CSV contains an unneutralized formula cell:\n%s", s)
	}
	if !strings.Contains(s, "'=cmd") {
		t.Fatalf("expected the title to be quote-escaped as text, got:\n%s", s)
	}
}
