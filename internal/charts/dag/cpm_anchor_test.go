// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"testing"
	"time"

	"pmforge/internal/kernel"
)

func weekdaysOnly(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

func TestAnchorCPMDates_WritesRealDates(t *testing.T) {
	nodes := []LayeredNode{
		{ID: "A", Label: "A", Duration: 2},
		{ID: "B", Label: "B", Duration: 3},
	}
	doc := LayeredDocument{
		Nodes: nodes,
		Edges: []LayeredEdge{{From: "A", To: "B"}},
	}
	if _, err := LayoutCPM(doc); err != nil {
		t.Fatalf("LayoutCPM: %v", err)
	}

	// Friday 2026-06-05; weekend skipped.
	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	AnchorCPMDates(&doc, start, weekdaysOnly)

	want := [][2]string{
		{"2026-06-05", "2026-06-08"}, // A: Fri + Mon
		{"2026-06-09", "2026-06-11"}, // B: Tue..Thu
	}
	for i, n := range doc.Nodes {
		if n.StartDate != want[i][0] || n.FinishDate != want[i][1] {
			t.Errorf("%s: got (%s, %s), want (%s, %s)",
				n.ID, n.StartDate, n.FinishDate, want[i][0], want[i][1])
		}
	}
}

func TestAnchorCPMDates_NilAndEmptyAreNoops(t *testing.T) {
	AnchorCPMDates(nil, time.Now(), nil)                // must not panic
	AnchorCPMDates(&LayeredDocument{}, time.Now(), nil) // must not panic
}

func TestLayoutCPMScheduled_HonoursConstraintsAndDates(t *testing.T) {
	nodes := []LayeredNode{
		{ID: "A", Label: "A", Duration: 1},
		{ID: "B", Label: "B", Duration: 2,
			Constraint: "snet", ConstraintDate: "2026-06-04"},
		{ID: "C", Label: "C", Duration: 4,
			Constraint: "MFO", ConstraintDate: "2026-06-02"},
	}
	doc := LayeredDocument{
		Nodes: nodes,
		Edges: []LayeredEdge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
		},
	}

	// Monday 2026-06-01.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if _, err := LayoutCPMScheduled(doc, start, weekdaysOnly, nil); err != nil {
		t.Fatalf("LayoutCPMScheduled: %v", err)
	}

	// B: SNET Thursday (case-insensitive "snet") beats link ES=1.
	if nodes[1].StartDate != "2026-06-04" {
		t.Errorf("B.StartDate = %s, want 2026-06-04 (SNET)", nodes[1].StartDate)
	}
	if nodes[1].ConstraintViolated {
		t.Error("B's satisfiable SNET must not be flagged")
	}
	// C: 4 days after A cannot finish on Tuesday — violation flagged.
	if !nodes[2].ConstraintViolated {
		t.Error("C's impossible MFO must set ConstraintViolated")
	}
}

func TestLayoutCPM_FlagsOverallocatedNodes(t *testing.T) {
	nodes := []LayeredNode{
		{ID: "A", Label: "A", Duration: 2,
			Assignments: []kernel.Assignment{{Resource: "alice"}}},
		{ID: "B", Label: "B", Duration: 1,
			Assignments: []kernel.Assignment{{Resource: "alice"}}},
		{ID: "C", Label: "C", Duration: 1},
	}
	doc := LayeredDocument{Nodes: nodes}
	if _, err := LayoutCPM(doc); err != nil {
		t.Fatalf("LayoutCPM: %v", err)
	}

	// A and B run in parallel on alice (capacity 1): both flagged.
	if !nodes[0].Overallocated || !nodes[1].Overallocated {
		t.Error("A and B must be flagged overallocated")
	}
	if nodes[2].Overallocated {
		t.Error("C has no assignments and must not be flagged")
	}
}

func TestLayoutCPM_PlainPathIgnoresDateConstraints(t *testing.T) {
	nodes := []LayeredNode{
		{ID: "A", Label: "A", Duration: 1,
			Constraint: "SNET", ConstraintDate: "2026-06-04"},
	}
	doc := LayeredDocument{Nodes: nodes}
	if _, err := LayoutCPM(doc); err != nil {
		t.Fatalf("LayoutCPM: %v", err)
	}
	if nodes[0].ES != 0 {
		t.Errorf("un-anchored LayoutCPM must ignore SNET; ES = %v", nodes[0].ES)
	}
}
