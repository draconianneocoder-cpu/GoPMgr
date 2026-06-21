// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import (
	"math"
	"testing"
)

// ----- ParseStakeholder -----

func TestParseStakeholder_Empty(t *testing.T) {
	doc, err := ParseStakeholder("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if len(doc.Stakeholders) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseStakeholder_EmptyObject(t *testing.T) {
	doc, err := ParseStakeholder("{}")
	if err != nil {
		t.Fatalf("unexpected error for {}: %v", err)
	}
	if len(doc.Stakeholders) != 0 {
		t.Error("expected empty doc from {}")
	}
}

func TestParseStakeholder_InvalidJSON(t *testing.T) {
	if _, err := ParseStakeholder("{bad}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseStakeholder_ValidDocument(t *testing.T) {
	raw := `{"stakeholders":[
		{"id":"s1","name":"Ada","power":"high","interest":"high"},
		{"id":"s2","name":"Bob","power":"low","interest":"low"}
	]}`
	doc, err := ParseStakeholder(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Stakeholders) != 2 {
		t.Fatalf("expected 2 stakeholders, got %d", len(doc.Stakeholders))
	}
	if doc.Stakeholders[0].Name != "Ada" {
		t.Errorf("first name: got %q, want Ada", doc.Stakeholders[0].Name)
	}
}

// ----- keyFor -----

func TestKeyFor_AllCombinations(t *testing.T) {
	cases := []struct {
		power, interest, want string
	}{
		{"low", "low", "ll"},
		{"low", "high", "lh"},
		{"high", "low", "hl"},
		{"high", "high", "hh"},
		// Anything not exactly "high" is treated as low.
		{"", "", "ll"},
		{"medium", "unknown", "ll"},
		{"HIGH", "high", "lh"}, // case-sensitive: "HIGH" != "high"
	}
	for _, c := range cases {
		if got := keyFor(c.power, c.interest); got != c.want {
			t.Errorf("keyFor(%q,%q) = %q, want %q", c.power, c.interest, got, c.want)
		}
	}
}

// ----- LayoutStakeholder -----

func TestLayoutStakeholder_AlwaysFourQuadrantLabels(t *testing.T) {
	layout := LayoutStakeholder(StakeholderDocument{})
	if len(layout.Quadrants) != 4 {
		t.Errorf("expected 4 quadrant labels, got %d", len(layout.Quadrants))
	}
	if len(layout.Points) != 0 {
		t.Errorf("empty doc should yield no points, got %d", len(layout.Points))
	}
}

func TestLayoutStakeholder_SinglePointSitsAtQuadrantCentre(t *testing.T) {
	// With n=1 in a bucket, the micro-grid places the point exactly at
	// the quadrant centre. Verify each quadrant maps to its centre.
	cases := []struct {
		power, interest string
		wantX, wantY    float64
		wantStrategy    string
	}{
		{"low", "low", 0.25, 0.75, "Monitor"},
		{"low", "high", 0.75, 0.75, "Keep Informed"},
		{"high", "low", 0.25, 0.25, "Keep Satisfied"},
		{"high", "high", 0.75, 0.25, "Manage Closely"},
	}
	for _, c := range cases {
		doc := StakeholderDocument{Stakeholders: []Stakeholder{
			{ID: "s1", Name: "Solo", Power: c.power, Interest: c.interest},
		}}
		layout := LayoutStakeholder(doc)
		if len(layout.Points) != 1 {
			t.Fatalf("power=%s interest=%s: expected 1 point, got %d", c.power, c.interest, len(layout.Points))
		}
		p := layout.Points[0]
		if math.Abs(p.X-c.wantX) > 1e-9 || math.Abs(p.Y-c.wantY) > 1e-9 {
			t.Errorf("power=%s interest=%s: got (%.3f,%.3f), want (%.3f,%.3f)",
				c.power, c.interest, p.X, p.Y, c.wantX, c.wantY)
		}
		if p.Strategy != c.wantStrategy {
			t.Errorf("power=%s interest=%s: strategy %q, want %q",
				c.power, c.interest, p.Strategy, c.wantStrategy)
		}
	}
}

func TestLayoutStakeholder_StrategyIsOverwritten(t *testing.T) {
	// A stale Strategy on the input must be replaced by the canonical
	// quadrant strategy.
	doc := StakeholderDocument{Stakeholders: []Stakeholder{
		{ID: "s1", Name: "X", Power: "high", Interest: "high", Strategy: "wrong"},
	}}
	layout := LayoutStakeholder(doc)
	if layout.Points[0].Strategy != "Manage Closely" {
		t.Errorf("strategy: got %q, want Manage Closely", layout.Points[0].Strategy)
	}
}

func TestLayoutStakeholder_PointsStayWithinUnitCanvas(t *testing.T) {
	// Pack several stakeholders into one bucket and confirm every plotted
	// point stays inside the 0..1 canvas.
	var sl []Stakeholder
	for i := range 9 {
		sl = append(sl, Stakeholder{
			ID:    string(rune('a' + i)),
			Name:  string(rune('a' + i)),
			Power: "high", Interest: "high",
		})
	}
	layout := LayoutStakeholder(StakeholderDocument{Stakeholders: sl})
	if len(layout.Points) != 9 {
		t.Fatalf("expected 9 points, got %d", len(layout.Points))
	}
	for _, p := range layout.Points {
		if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			t.Errorf("point %s outside unit canvas: (%.3f,%.3f)", p.Name, p.X, p.Y)
		}
	}
}

func TestLayoutStakeholder_DeterministicSortByName(t *testing.T) {
	// Input out of order; same bucket. Output order within the bucket
	// must be stable (sorted by Name) regardless of input order.
	doc := StakeholderDocument{Stakeholders: []Stakeholder{
		{ID: "s3", Name: "Charlie", Power: "low", Interest: "low"},
		{ID: "s1", Name: "Alice", Power: "low", Interest: "low"},
		{ID: "s2", Name: "Bob", Power: "low", Interest: "low"},
	}}
	layout := LayoutStakeholder(doc)
	if len(layout.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(layout.Points))
	}
	wantOrder := []string{"Alice", "Bob", "Charlie"}
	for i, name := range wantOrder {
		if layout.Points[i].Name != name {
			t.Errorf("point[%d]: got %q, want %q", i, layout.Points[i].Name, name)
		}
	}
}
