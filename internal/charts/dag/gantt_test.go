// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"testing"
	"time"

	"pmforge/internal/kernel"
)

func ganttDoc() LayeredDocument {
	return LayeredDocument{
		Nodes: []LayeredNode{
			{ID: "B", Label: "Build", Duration: 3, PercentComplete: 40},
			{ID: "A", Label: "Design", Duration: 2},
			{ID: "M", Label: "Ship", Duration: 0},
		},
		Edges: []LayeredEdge{
			{From: "A", To: "B", Label: "FS+1"},
			{From: "B", To: "M"},
		},
	}
}

func TestLayoutGantt(t *testing.T) {
	layout, err := LayoutGantt(ganttDoc())
	if err != nil {
		t.Fatalf("LayoutGantt: %v", err)
	}

	// Rows sorted by (ES, ID): A (0), B (3 after FS+1), M (6).
	if len(layout.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(layout.Rows))
	}
	if layout.Rows[0].ID != "A" || layout.Rows[1].ID != "B" || layout.Rows[2].ID != "M" {
		t.Errorf("row order = %s %s %s, want A B M",
			layout.Rows[0].ID, layout.Rows[1].ID, layout.Rows[2].ID)
	}
	if layout.Rows[1].ES != 3 || layout.Rows[1].EF != 6 {
		t.Errorf("B = [%v, %v], want [3, 6] (FS+1)", layout.Rows[1].ES, layout.Rows[1].EF)
	}
	if layout.Horizon != 6 {
		t.Errorf("horizon = %v, want 6", layout.Horizon)
	}
	if !layout.Rows[2].Milestone {
		t.Error("zero-duration row must be a milestone")
	}
	if layout.Rows[1].PercentComplete != 40 {
		t.Errorf("percent lost: %v", layout.Rows[1].PercentComplete)
	}
	// Whole chain is critical.
	for _, r := range layout.Rows {
		if !r.IsCritical {
			t.Errorf("row %s should be critical", r.ID)
		}
	}
	if len(layout.Deps) != 2 || layout.Deps[0].Label != "FS+1" {
		t.Errorf("deps = %+v", layout.Deps)
	}
	if layout.Anchored {
		t.Error("plain layout must not claim anchoring")
	}
}

func TestLayoutGanttScheduled(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // Monday
	layout, err := LayoutGanttScheduled(ganttDoc(), start, weekdaysOnly, nil)
	if err != nil {
		t.Fatalf("LayoutGanttScheduled: %v", err)
	}
	if !layout.Anchored {
		t.Error("scheduled layout must be anchored")
	}
	if layout.Rows[0].StartDate != "2026-06-01" {
		t.Errorf("A.StartDate = %s, want 2026-06-01", layout.Rows[0].StartDate)
	}
	// B: ES=3 (Thu), 3 days Thu+Fri+Mon.
	if layout.Rows[1].StartDate != "2026-06-04" || layout.Rows[1].FinishDate != "2026-06-08" {
		t.Errorf("B dates = %s → %s, want 2026-06-04 → 2026-06-08",
			layout.Rows[1].StartDate, layout.Rows[1].FinishDate)
	}
}

func TestLayoutGanttCycle(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A", Duration: 1}, {ID: "B", Duration: 1}},
		Edges: []LayeredEdge{{From: "A", To: "B"}, {From: "B", To: "A"}},
	}
	if _, err := LayoutGantt(doc); err != ErrCycle {
		t.Errorf("err = %v, want ErrCycle", err)
	}
}

func TestLayoutGanttOverallocationFlag(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{
			{ID: "A", Label: "A", Duration: 2,
				Assignments: []kernel.Assignment{{Resource: "alice"}}},
			{ID: "B", Label: "B", Duration: 2,
				Assignments: []kernel.Assignment{{Resource: "alice"}}},
		},
	}
	layout, err := LayoutGantt(doc)
	if err != nil {
		t.Fatalf("LayoutGantt: %v", err)
	}
	for _, r := range layout.Rows {
		if !r.Overallocated {
			t.Errorf("row %s should be overallocated", r.ID)
		}
	}
}
