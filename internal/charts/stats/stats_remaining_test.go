// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "testing"

// ----- ParseLine -----

func TestParseLine_Empty(t *testing.T) {
	doc, err := ParseLine("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Series) != 0 {
		t.Error("expected empty Series from empty string")
	}
}

func TestParseLine_EmptyObject(t *testing.T) {
	doc, err := ParseLine("{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Series) != 0 {
		t.Error("expected empty Series from {}")
	}
}

func TestParseLine_InvalidJSON(t *testing.T) {
	if _, err := ParseLine("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseLine_ValidDocument(t *testing.T) {
	raw := `{"title":"Velocity","x_str":["W1","W2"],"series":[{"name":"S","y":[3,5]}]}`
	doc, err := ParseLine(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Velocity" {
		t.Errorf("Title: got %q, want %q", doc.Title, "Velocity")
	}
	if len(doc.XStr) != 2 || len(doc.Series) != 1 {
		t.Errorf("XStr=%d Series=%d", len(doc.XStr), len(doc.Series))
	}
}

// ----- LayoutLine -----

func TestLayoutLine_XStrTakesPrecedenceOverX(t *testing.T) {
	doc := LineDocument{
		X:    []float64{1, 2},
		XStr: []string{"Jan", "Feb"},
	}
	layout := LayoutLine(doc)
	if len(layout.Categories) != 2 || layout.Categories[0] != "Jan" {
		t.Errorf("categories: got %v, want [Jan Feb]", layout.Categories)
	}
}

func TestLayoutLine_NumericXConvertedWhenXStrAbsent(t *testing.T) {
	doc := LineDocument{X: []float64{10, 20}}
	layout := LayoutLine(doc)
	if len(layout.Categories) != 2 || layout.Categories[0] != "10" || layout.Categories[1] != "20" {
		t.Errorf("categories: got %v, want [10 20]", layout.Categories)
	}
}

func TestLayoutLine_KindAndSeriesType(t *testing.T) {
	doc := LineDocument{
		XStr: []string{"d1"},
		Series: []struct {
			Name   string    `json:"name"`
			Y      []float64 `json:"y"`
			Color  string    `json:"color,omitempty"`
			Dashed bool      `json:"dashed,omitempty"`
		}{{Name: "Actual", Y: []float64{5}}},
	}
	layout := LayoutLine(doc)
	if layout.Kind != "line" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "line")
	}
	if len(layout.Series) != 1 || layout.Series[0].Type != "line" {
		t.Errorf("Series[0].Type: got %q, want %q", layout.Series[0].Type, "line")
	}
}

// ----- ParseBar -----

func TestParseBar_Empty(t *testing.T) {
	doc, err := ParseBar("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Categories) != 0 {
		t.Error("expected empty Categories from empty string")
	}
}

func TestParseBar_EmptyObject(t *testing.T) {
	if _, err := ParseBar("{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBar_InvalidJSON(t *testing.T) {
	if _, err := ParseBar("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ----- LayoutBar -----

func TestLayoutBar_KindCategoriesSeriesType(t *testing.T) {
	doc := BarDocument{
		Categories: []string{"Q1", "Q2"},
		Series:     []BarSeries{{Name: "Revenue", Values: []float64{100, 200}}},
	}
	layout := LayoutBar(doc)
	if layout.Kind != "bar" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "bar")
	}
	if len(layout.Categories) != 2 || layout.Categories[0] != "Q1" {
		t.Errorf("Categories: got %v", layout.Categories)
	}
	if len(layout.Series) != 1 || layout.Series[0].Type != "bar" {
		t.Errorf("Series[0].Type: got %q, want %q", layout.Series[0].Type, "bar")
	}
}

func TestLayoutBar_PreservesMixedOverlaySeries(t *testing.T) {
	doc := BarDocument{
		Categories: []string{"Day 1", "Day 2"},
		Series: []BarSeries{
			{Name: "alice", Values: []float64{1.5, 0.5}},
			{Name: "alice capacity", Values: []float64{1, 1}, Type: "line", Color: "#f59e0b", Dashed: true},
		},
	}

	layout := LayoutBar(doc)

	if len(layout.Series) != 2 {
		t.Fatalf("Series = %d, want 2", len(layout.Series))
	}
	if layout.Series[0].Type != "bar" {
		t.Errorf("demand Type = %q, want bar", layout.Series[0].Type)
	}
	capacity := layout.Series[1]
	if capacity.Type != "line" || !capacity.Dashed || capacity.Color != "#f59e0b" {
		t.Errorf("capacity series = %+v, want dashed amber line", capacity)
	}
}

// ----- ParsePie -----

func TestParsePie_Empty(t *testing.T) {
	doc, err := ParsePie("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Slices) != 0 {
		t.Error("expected empty Slices from empty string")
	}
}

func TestParsePie_EmptyObject(t *testing.T) {
	if _, err := ParsePie("{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePie_InvalidJSON(t *testing.T) {
	if _, err := ParsePie("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ----- LayoutPie -----

func TestLayoutPie_ZeroTotal_AllPctZero(t *testing.T) {
	doc := PieDocument{
		Slices: []struct {
			Label string  `json:"label"`
			Value float64 `json:"value"`
			Color string  `json:"color,omitempty"`
		}{{Label: "A", Value: 0}, {Label: "B", Value: 0}},
	}
	layout := LayoutPie(doc)
	for i, s := range layout.Slices {
		if s.Pct != 0 {
			t.Errorf("Slices[%d].Pct = %v, want 0 when total=0", i, s.Pct)
		}
	}
}

func TestLayoutPie_SingleSlice_100Percent(t *testing.T) {
	doc := PieDocument{
		Slices: []struct {
			Label string  `json:"label"`
			Value float64 `json:"value"`
			Color string  `json:"color,omitempty"`
		}{{Label: "All", Value: 42}},
	}
	layout := LayoutPie(doc)
	if len(layout.Slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(layout.Slices))
	}
	near(t, "single slice pct", layout.Slices[0].Pct, 100.0)
}

func TestLayoutPie_MultiSliceProportional(t *testing.T) {
	// A=50, B=30, C=20; total=100 -> 50%, 30%, 20%
	doc := PieDocument{
		Slices: []struct {
			Label string  `json:"label"`
			Value float64 `json:"value"`
			Color string  `json:"color,omitempty"`
		}{{Label: "A", Value: 50}, {Label: "B", Value: 30}, {Label: "C", Value: 20}},
	}
	layout := LayoutPie(doc)
	if len(layout.Slices) != 3 {
		t.Fatalf("expected 3 slices, got %d", len(layout.Slices))
	}
	near(t, "A pct", layout.Slices[0].Pct, 50.0)
	near(t, "B pct", layout.Slices[1].Pct, 30.0)
	near(t, "C pct", layout.Slices[2].Pct, 20.0)
}

func TestLayoutPie_KindAndNoSeries(t *testing.T) {
	layout := LayoutPie(PieDocument{})
	if layout.Kind != "pie" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "pie")
	}
	if len(layout.Series) != 0 {
		t.Errorf("Pie layout should have no Series, got %d", len(layout.Series))
	}
}

// ----- ParseBurnUp -----

func TestParseBurnUp_Empty(t *testing.T) {
	doc, err := ParseBurnUp("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Days) != 0 {
		t.Error("expected empty Days from empty string")
	}
}

func TestParseBurnUp_EmptyObject(t *testing.T) {
	if _, err := ParseBurnUp("{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBurnUp_InvalidJSON(t *testing.T) {
	if _, err := ParseBurnUp("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ----- LayoutBurnUp -----

func TestLayoutBurnUp_KindAndTwoSeries(t *testing.T) {
	doc := BurnUpDocument{
		Days:      []float64{1, 2, 3},
		Completed: []float64{0, 5, 10},
		Scope:     []float64{20, 20, 20},
	}
	layout := LayoutBurnUp(doc)
	if layout.Kind != "burnup" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "burnup")
	}
	if len(layout.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(layout.Series))
	}
}

func TestLayoutBurnUp_CompletedSeriesColor(t *testing.T) {
	layout := LayoutBurnUp(BurnUpDocument{Days: []float64{1}})
	if layout.Series[0].Name != "Completed" {
		t.Errorf("Series[0].Name: got %q, want %q", layout.Series[0].Name, "Completed")
	}
	if layout.Series[0].Color != "#22d3ee" {
		t.Errorf("Series[0].Color: got %q, want #22d3ee", layout.Series[0].Color)
	}
}

func TestLayoutBurnUp_ScopeSeriesDashedAmber(t *testing.T) {
	layout := LayoutBurnUp(BurnUpDocument{Days: []float64{1}})
	s := layout.Series[1]
	if s.Name != "Scope" {
		t.Errorf("Series[1].Name: got %q, want %q", s.Name, "Scope")
	}
	if s.Color != "#f59e0b" {
		t.Errorf("Series[1].Color: got %q, want #f59e0b", s.Color)
	}
	if !s.Dashed {
		t.Error("Series[1] (Scope) should be Dashed")
	}
}

// ----- ParseBurnDown -----

func TestParseBurnDown_Empty(t *testing.T) {
	doc, err := ParseBurnDown("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Remaining) != 0 {
		t.Error("expected empty Remaining from empty string")
	}
}

func TestParseBurnDown_EmptyObject(t *testing.T) {
	if _, err := ParseBurnDown("{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBurnDown_InvalidJSON(t *testing.T) {
	if _, err := ParseBurnDown("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ----- computeIdealBurnDown -----

func TestComputeIdealBurnDown_ZeroN_ReturnsEmpty(t *testing.T) {
	got := computeIdealBurnDown([]float64{10}, 0)
	if len(got) != 0 {
		t.Errorf("n=0: expected empty slice, got %v", got)
	}
}

func TestComputeIdealBurnDown_EmptyRemaining_ReturnsEmpty(t *testing.T) {
	got := computeIdealBurnDown(nil, 5)
	if len(got) != 0 {
		t.Errorf("empty remaining: expected empty slice, got %v", got)
	}
}

func TestComputeIdealBurnDown_N1_ReturnsSingleStart(t *testing.T) {
	got := computeIdealBurnDown([]float64{10}, 1)
	if len(got) != 1 {
		t.Fatalf("n=1: expected 1 element, got %d", len(got))
	}
	near(t, "n=1 value", got[0], 10.0)
}

func TestComputeIdealBurnDown_N5_KnownTrajectory(t *testing.T) {
	// start=10, n=5: step=2.5 -> [10, 7.5, 5, 2.5, 0]
	got := computeIdealBurnDown([]float64{10}, 5)
	if len(got) != 5 {
		t.Fatalf("n=5: expected 5 elements, got %d", len(got))
	}
	want := []float64{10, 7.5, 5, 2.5, 0}
	for i, w := range want {
		near(t, "ideal["+string(rune('0'+i))+"]", got[i], w)
	}
}

// ----- LayoutBurnDown -----

func TestLayoutBurnDown_KindAndTwoSeries(t *testing.T) {
	doc := BurnDownDocument{
		Days:      []float64{1, 2, 3},
		Remaining: []float64{10, 7, 4},
	}
	layout := LayoutBurnDown(doc)
	if layout.Kind != "burndown" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "burndown")
	}
	if len(layout.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(layout.Series))
	}
}

func TestLayoutBurnDown_RemainingAndIdealSeriesNames(t *testing.T) {
	doc := BurnDownDocument{
		Days:      []float64{1, 2},
		Remaining: []float64{8, 4},
	}
	layout := LayoutBurnDown(doc)
	if layout.Series[0].Name != "Remaining" {
		t.Errorf("Series[0].Name: got %q, want %q", layout.Series[0].Name, "Remaining")
	}
	if layout.Series[1].Name != "Ideal" {
		t.Errorf("Series[1].Name: got %q, want %q", layout.Series[1].Name, "Ideal")
	}
	if !layout.Series[1].Dashed {
		t.Error("Ideal series should be Dashed")
	}
}

// ----- ParseCumFlow -----

func TestParseCumFlow_Empty(t *testing.T) {
	doc, err := ParseCumFlow("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Days) != 0 {
		t.Error("expected empty Days from empty string")
	}
}

func TestParseCumFlow_EmptyObject(t *testing.T) {
	if _, err := ParseCumFlow("{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCumFlow_InvalidJSON(t *testing.T) {
	if _, err := ParseCumFlow("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ----- LayoutCumFlow -----

func TestLayoutCumFlow_KindAndStacked(t *testing.T) {
	doc := CumFlowDocument{
		Days:   []float64{1, 2},
		States: map[string][]float64{"done": {5, 8}},
	}
	layout := LayoutCumFlow(doc)
	if layout.Kind != "cumulative_flow" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "cumulative_flow")
	}
	if !layout.Stacked {
		t.Error("Stacked should be true")
	}
}

func TestLayoutCumFlow_SeriesTypeArea(t *testing.T) {
	doc := CumFlowDocument{
		Days:   []float64{1},
		States: map[string][]float64{"todo": {3}},
	}
	layout := LayoutCumFlow(doc)
	if len(layout.Series) != 1 || layout.Series[0].Type != "area" {
		t.Errorf("Series[0].Type: got %q, want area", layout.Series[0].Type)
	}
}

func TestLayoutCumFlow_AlphabeticalOrderWhenNoStateOrder(t *testing.T) {
	doc := CumFlowDocument{
		Days:   []float64{1},
		States: map[string][]float64{"todo": {3}, "doing": {2}, "done": {5}},
	}
	layout := LayoutCumFlow(doc)
	if len(layout.Series) != 3 {
		t.Fatalf("expected 3 series, got %d", len(layout.Series))
	}
	// sorted: doing, done, todo
	if layout.Series[0].Name != "doing" || layout.Series[1].Name != "done" || layout.Series[2].Name != "todo" {
		t.Errorf("series order: got %v/%v/%v, want doing/done/todo",
			layout.Series[0].Name, layout.Series[1].Name, layout.Series[2].Name)
	}
}

func TestLayoutCumFlow_ExplicitStateOrderFiltersAbsent(t *testing.T) {
	doc := CumFlowDocument{
		Days:       []float64{1},
		States:     map[string][]float64{"todo": {3}, "done": {5}},
		StateOrder: []string{"done", "missing", "todo"},
	}
	layout := LayoutCumFlow(doc)
	if len(layout.Series) != 2 {
		t.Fatalf("expected 2 series (absent name filtered), got %d", len(layout.Series))
	}
	if layout.Series[0].Name != "done" || layout.Series[1].Name != "todo" {
		t.Errorf("series order: got %v/%v, want done/todo", layout.Series[0].Name, layout.Series[1].Name)
	}
}
