// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestRenderChartToPDF_Gantt(t *testing.T) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()

	data := `{
		"nodes": [
			{"id":"A","label":"Design","duration":2,"percent_complete":50},
			{"id":"B","label":"Build","duration":3},
			{"id":"M","label":"Ship","duration":0}
		],
		"edges": [
			{"from":"A","to":"B","label":"FS+1"},
			{"from":"B","to":"M"}
		]
	}`
	frame := Frame{X: 10, Y: 10, W: 260, H: 180}

	if err := RenderChartToPDF(pdf, "gantt", data, "Schedule", frame); err != nil {
		t.Fatalf("RenderChartToPDF(gantt): %v", err)
	}
	if pdf.Err() {
		t.Fatalf("fpdf error state: %v", pdf.Error())
	}
}

// TestRenderChartToPDF_GanttSplit renders a task carrying split work_segments
// and confirms the interrupted-bar path draws without error.
func TestRenderChartToPDF_GanttSplit(t *testing.T) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()

	// S is a 3-day task interrupted across relative days 0,2,4.
	data := `{
		"nodes": [
			{"id":"S","label":"Long task","duration":3,
			 "work_segments":[{"start":0,"end":1},{"start":2,"end":3},{"start":4,"end":5}]}
		],
		"edges": []
	}`
	frame := Frame{X: 10, Y: 10, W: 260, H: 180}
	if err := RenderChartToPDF(pdf, "gantt", data, "Schedule", frame); err != nil {
		t.Fatalf("RenderChartToPDF(gantt split): %v", err)
	}
	if pdf.Err() {
		t.Fatalf("fpdf error state: %v", pdf.Error())
	}
}

func TestRenderChartToPDF_GanttEmpty(t *testing.T) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	frame := Frame{X: 10, Y: 10, W: 260, H: 180}

	if err := RenderChartToPDF(pdf, "gantt", `{"nodes":[],"edges":[]}`, "Schedule", frame); err != nil {
		t.Fatalf("empty gantt must render a placeholder, got %v", err)
	}
}

func TestPickGridStep(t *testing.T) {
	if s := pickGridStep(10, 200); s != 1 {
		t.Errorf("wide chart: step = %v, want 1", s)
	}
	if s := pickGridStep(1000, 100); s == 1 {
		t.Error("dense chart must not use 1-day grid")
	}
	if s := pickGridStep(0, 100); s != 0 {
		t.Errorf("zero horizon: step = %v, want 0", s)
	}
}
