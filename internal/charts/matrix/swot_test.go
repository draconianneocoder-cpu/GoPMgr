// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import "testing"

// ----- ParseSWOT -----

func TestParseSWOT_Empty(t *testing.T) {
	doc, err := ParseSWOT("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if len(doc.Strengths) != 0 || len(doc.Weaknesses) != 0 ||
		len(doc.Opportunities) != 0 || len(doc.Threats) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseSWOT_EmptyObject(t *testing.T) {
	doc, err := ParseSWOT("{}")
	if err != nil {
		t.Fatalf("unexpected error for {}: %v", err)
	}
	if len(doc.Strengths) != 0 {
		t.Error("expected empty doc from {}")
	}
}

func TestParseSWOT_InvalidJSON(t *testing.T) {
	if _, err := ParseSWOT("{not json}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseSWOT_ValidDocument(t *testing.T) {
	raw := `{
		"title": "Q3 Analysis",
		"strengths": ["brand", "team"],
		"weaknesses": ["cash"],
		"opportunities": ["new market"],
		"threats": ["competitor"]
	}`
	doc, err := ParseSWOT(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Q3 Analysis" {
		t.Errorf("Title: got %q, want %q", doc.Title, "Q3 Analysis")
	}
	if len(doc.Strengths) != 2 {
		t.Errorf("Strengths: got %d, want 2", len(doc.Strengths))
	}
	if len(doc.Threats) != 1 || doc.Threats[0] != "competitor" {
		t.Errorf("Threats: got %v, want [competitor]", doc.Threats)
	}
}

// ----- LayoutSWOT -----

func TestLayoutSWOT_FourQuadrantsInCanonicalGrid(t *testing.T) {
	layout := LayoutSWOT(SWOTDocument{})
	if len(layout.Quadrants) != 4 {
		t.Fatalf("expected 4 quadrants, got %d", len(layout.Quadrants))
	}

	// key -> expected (row, col, tone)
	want := map[string]struct {
		row, col int
		tone     string
	}{
		"S": {0, 0, "positive"},
		"W": {0, 1, "negative"},
		"O": {1, 0, "external_positive"},
		"T": {1, 1, "external_negative"},
	}
	for _, q := range layout.Quadrants {
		w, ok := want[q.Key]
		if !ok {
			t.Errorf("unexpected quadrant key %q", q.Key)
			continue
		}
		if q.Row != w.row || q.Col != w.col {
			t.Errorf("quadrant %s: got (row=%d,col=%d), want (row=%d,col=%d)",
				q.Key, q.Row, q.Col, w.row, w.col)
		}
		if q.Tone != w.tone {
			t.Errorf("quadrant %s: tone %q, want %q", q.Key, q.Tone, w.tone)
		}
	}
}

func TestLayoutSWOT_ItemsAndTitlePassThrough(t *testing.T) {
	doc := SWOTDocument{
		Title:      "My SWOT",
		Strengths:  []string{"a", "b"},
		Weaknesses: []string{"c"},
	}
	layout := LayoutSWOT(doc)
	if layout.Title != "My SWOT" {
		t.Errorf("Title: got %q, want %q", layout.Title, "My SWOT")
	}
	byKey := make(map[string]SWOTQuadrant, 4)
	for _, q := range layout.Quadrants {
		byKey[q.Key] = q
	}
	if len(byKey["S"].Items) != 2 {
		t.Errorf("S items: got %d, want 2", len(byKey["S"].Items))
	}
	if len(byKey["W"].Items) != 1 {
		t.Errorf("W items: got %d, want 1", len(byKey["W"].Items))
	}
	if len(byKey["T"].Items) != 0 {
		t.Errorf("T items: got %d, want 0", len(byKey["T"].Items))
	}
}
