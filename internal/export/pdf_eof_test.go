// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"testing"

	"gopmgr/internal/kernel"
)

func TestSchedulePDFHasNoPostEOFData(t *testing.T) {
	out, err := GenerateArchivalReport(ReportPayload{
		Tasks: map[string]*kernel.Task{
			"A": {ID: "A", Title: "Task A", Duration: 5, ES: 0, EF: 5, LS: 0, LF: 5, Float: 0, IsCritical: true},
			"B": {ID: "B", Title: "Task B", Duration: 3, ES: 5, EF: 8, LS: 5, LF: 8, Float: 0, IsCritical: true},
		},
	}, ExportOptions{Format: FormatPDF, Title: "PDF/A-2u Test Schedule"})
	if err != nil {
		t.Fatalf("GenerateArchivalReport(pdf): %v", err)
	}

	idx := bytes.LastIndex(out, []byte("%%EOF"))
	if idx < 0 {
		t.Fatal("schedule PDF missing EOF marker")
	}
	after := out[idx+len("%%EOF"):]
	if !bytes.Equal(after, []byte("\n")) && len(after) != 0 {
		t.Fatalf("schedule PDF has post-EOF data %q", after)
	}
}

// TestSchedulePDFAcceptsEmojiInTaskNames is the regression for the crash an
// emoji in a task name caused: fpdf panicked while writing the schedule
// table, which draws task titles with single-line cells.
func TestSchedulePDFAcceptsEmojiInTaskNames(t *testing.T) {
	out, err := GenerateArchivalReport(ReportPayload{
		Tasks: map[string]*kernel.Task{
			"A": {ID: "A", Title: "Launch 🚀 prep", Duration: 5, EF: 5, LF: 5, IsCritical: true},
			"B": {ID: "B", Title: "Celebrate 🎉👩\u200D💻", Duration: 1, ES: 5, EF: 6, LS: 5, LF: 6, IsCritical: true},
		},
	}, ExportOptions{Format: FormatPDF, Title: "Release 🚀 plan"})
	if err != nil {
		t.Fatalf("GenerateArchivalReport(pdf): %v", err)
	}
	if !bytes.Contains(out, []byte("<pdfaid:part>2</pdfaid:part>")) {
		t.Fatal("schedule with emoji is not PDF/A-2u")
	}
}
