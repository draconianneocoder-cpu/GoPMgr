// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"gopmgr/internal/kernel"
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

// TestRenderCSVNeutralizesFormulaInTaskID covers the ID column: an MSPDI
// import keeps each task's UID as its ID, so a crafted file can supply a
// formula there too (2026-11-10 security scan).
func TestRenderCSVNeutralizesFormulaInTaskID(t *testing.T) {
	payload := ReportPayload{
		Tasks: map[string]*kernel.Task{
			"=HYPERLINK(1)": {ID: "=HYPERLINK(1)", Title: "Plain title"},
		},
	}
	out, err := renderCSV(payload, ExportOptions{})
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-1], "'=HYPERLINK(1)") {
		t.Fatalf("expected the task ID to be quote-escaped as text, got:\n%s", out)
	}
}
