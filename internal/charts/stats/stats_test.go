// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"math"
	"strings"
	"testing"
)

func near(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// ----- ParsePareto -----

func TestParsePareto_Empty(t *testing.T) {
	doc, err := ParsePareto("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Items) != 0 {
		t.Error("expected empty Items from empty string")
	}
}

func TestParsePareto_EmptyObject(t *testing.T) {
	doc, err := ParsePareto("{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Items) != 0 {
		t.Error("expected empty Items from {}")
	}
}

func TestParsePareto_InvalidJSON(t *testing.T) {
	_, err := ParsePareto("{invalid}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePareto_ValidDocument(t *testing.T) {
	raw := `{"title":"Defects","items":[{"label":"A","count":50},{"label":"B","count":30}]}`
	doc, err := ParsePareto(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Defects" {
		t.Errorf("Title: got %q, want %q", doc.Title, "Defects")
	}
	if len(doc.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(doc.Items))
	}
	if doc.Items[0].Label != "A" || doc.Items[0].Count != 50 {
		t.Errorf("item 0: got {%q, %v}", doc.Items[0].Label, doc.Items[0].Count)
	}
}

// ----- LayoutPareto: sort -----

func TestLayoutPareto_SortDescendingByCount(t *testing.T) {
	doc := ParetoDocument{
		Items: []ParetoItem{
			{Label: "C", Count: 20},
			{Label: "A", Count: 50},
			{Label: "B", Count: 30},
		},
	}
	layout := LayoutPareto(doc)
	if len(layout.Categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(layout.Categories))
	}
	if layout.Categories[0] != "A" || layout.Categories[1] != "B" || layout.Categories[2] != "C" {
		t.Errorf("wrong sort order: %v", layout.Categories)
	}
}

// ----- LayoutPareto: cumulative percentage -----

func TestLayoutPareto_CumulativePercentageValues(t *testing.T) {
	// Items already in descending order: A=50, B=30, C=20, total=100
	doc := ParetoDocument{
		Items: []ParetoItem{
			{Label: "A", Count: 50},
			{Label: "B", Count: 30},
			{Label: "C", Count: 20},
		},
	}
	layout := LayoutPareto(doc)
	if len(layout.Series) < 2 {
		t.Fatalf("expected at least 2 series, got %d", len(layout.Series))
	}
	pct := layout.Series[1].Values
	near(t, "cumPct[0]", pct[0], 50.0)
	near(t, "cumPct[1]", pct[1], 80.0)
	near(t, "cumPct[2]", pct[2], 100.0)
}

func TestLayoutPareto_LastCumPctIs100(t *testing.T) {
	doc := ParetoDocument{
		Items: []ParetoItem{{Label: "X", Count: 7}, {Label: "Y", Count: 3}},
	}
	layout := LayoutPareto(doc)
	pct := layout.Series[1].Values
	near(t, "last cumPct", pct[len(pct)-1], 100.0)
}

func TestLayoutPareto_ZeroTotal_CumPctAllZero(t *testing.T) {
	doc := ParetoDocument{
		Items: []ParetoItem{{Label: "X", Count: 0}, {Label: "Y", Count: 0}},
	}
	layout := LayoutPareto(doc)
	for i, v := range layout.Series[1].Values {
		if v != 0 {
			t.Errorf("cumPct[%d] = %v, want 0 when total=0", i, v)
		}
	}
}

// ----- LayoutPareto: 80% annotation -----

func TestLayoutPareto_80PercentAnnotation(t *testing.T) {
	layout := LayoutPareto(ParetoDocument{Items: []ParetoItem{{Label: "A", Count: 1}}})
	found := false
	for _, ann := range layout.Annotations {
		if ann.Value == 80 && ann.Dashed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected dashed 80%% annotation, got %v", layout.Annotations)
	}
}

// ----- LayoutPareto: right-axis and kind -----

func TestLayoutPareto_YAxisRightRange(t *testing.T) {
	layout := LayoutPareto(ParetoDocument{})
	if layout.YAxisRight == nil {
		t.Fatal("YAxisRight is nil")
	}
	if layout.YAxisRight.Min == nil || *layout.YAxisRight.Min != 0 {
		t.Error("YAxisRight.Min should be 0")
	}
	if layout.YAxisRight.Max == nil || *layout.YAxisRight.Max != 100 {
		t.Error("YAxisRight.Max should be 100")
	}
}

func TestLayoutPareto_KindAndSeriesTypes(t *testing.T) {
	layout := LayoutPareto(ParetoDocument{Items: []ParetoItem{{Label: "A", Count: 5}}})
	if layout.Kind != "pareto" {
		t.Errorf("Kind: got %q, want %q", layout.Kind, "pareto")
	}
	if len(layout.Series) != 2 {
		t.Fatalf("expected 2 series (bar + line), got %d", len(layout.Series))
	}
	if layout.Series[0].Type != "bar" {
		t.Errorf("Series[0].Type: got %q, want %q", layout.Series[0].Type, "bar")
	}
	if layout.Series[1].Type != "line" {
		t.Errorf("Series[1].Type: got %q, want %q", layout.Series[1].Type, "line")
	}
}

// ----- computeMean -----

func TestComputeMean_KnownValues(t *testing.T) {
	near(t, "mean [1,2,3]", computeMean([]float64{1, 2, 3}), 2.0)
	near(t, "mean [4,2]", computeMean([]float64{4, 2}), 3.0)
}

func TestComputeMean_EmptySlice(t *testing.T) {
	near(t, "mean nil", computeMean(nil), 0.0)
}

// ----- computeStdDev -----

func TestComputeStdDev_KnownValues(t *testing.T) {
	// sample std dev of [1,2,3] with mean=2: sqrt((1+0+1)/2) = 1.0
	near(t, "stddev [1,2,3]", computeStdDev([]float64{1, 2, 3}, 2.0), 1.0)
}

func TestComputeStdDev_SingleElement_ReturnsZero(t *testing.T) {
	near(t, "stddev single", computeStdDev([]float64{5}, 5.0), 0.0)
}

func TestComputeStdDev_EmptySlice_ReturnsZero(t *testing.T) {
	near(t, "stddev nil", computeStdDev(nil, 0.0), 0.0)
}

// ----- ParseControl -----

func TestParseControl_Empty(t *testing.T) {
	doc, err := ParseControl("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Y) != 0 {
		t.Error("expected empty Y from empty string")
	}
}

func TestParseControl_EmptyObject(t *testing.T) {
	doc, err := ParseControl("{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Y) != 0 {
		t.Error("expected empty Y from {}")
	}
}

func TestParseControl_InvalidJSON(t *testing.T) {
	_, err := ParseControl("{bad json}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ----- LayoutControl: auto-compute limits -----

func TestLayoutControl_AutoComputesLimits(t *testing.T) {
	// Y=[1,2,3]: mean=2, sample stddev=1, ucl=5, lcl=-1
	doc := ControlDocument{Y: []float64{1, 2, 3}, X: []float64{1, 2, 3}}
	layout := LayoutControl(doc)

	if len(layout.Annotations) < 3 {
		t.Fatalf("expected 3 annotations, got %d", len(layout.Annotations))
	}
	near(t, "Mean annotation", layout.Annotations[0].Value, 2.0)
	near(t, "UCL annotation", layout.Annotations[1].Value, 5.0)
	near(t, "LCL annotation", layout.Annotations[2].Value, -1.0)
}

// ----- LayoutControl: explicit limits preserved -----

func TestLayoutControl_ExplicitLimits_NotOverridden(t *testing.T) {
	doc := ControlDocument{Y: []float64{12}, X: []float64{1}, Mean: 10, UCL: 15, LCL: 5}
	layout := LayoutControl(doc)
	near(t, "Mean", layout.Annotations[0].Value, 10.0)
	near(t, "UCL", layout.Annotations[1].Value, 15.0)
	near(t, "LCL", layout.Annotations[2].Value, 5.0)
}

// ----- LayoutControl: flagging -----

func TestLayoutControl_PointAboveUCL_Flagged(t *testing.T) {
	doc := ControlDocument{
		Y: []float64{3, 10}, X: []float64{1, 2},
		Mean: 5, UCL: 8, LCL: 2,
	}
	layout := LayoutControl(doc)
	if len(layout.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(layout.Flags))
	}
	f := layout.Flags[0]
	if f.Point != 1 {
		t.Errorf("flagged point index: got %d, want 1", f.Point)
	}
	if !strings.Contains(f.Reason, "Above UCL") {
		t.Errorf("reason %q does not contain 'Above UCL'", f.Reason)
	}
}

func TestLayoutControl_PointBelowLCL_Flagged(t *testing.T) {
	doc := ControlDocument{
		Y: []float64{1, 5}, X: []float64{1, 2},
		Mean: 5, UCL: 8, LCL: 2,
	}
	layout := LayoutControl(doc)
	if len(layout.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(layout.Flags))
	}
	f := layout.Flags[0]
	if f.Point != 0 {
		t.Errorf("flagged point index: got %d, want 0", f.Point)
	}
	if !strings.Contains(f.Reason, "Below LCL") {
		t.Errorf("reason %q does not contain 'Below LCL'", f.Reason)
	}
}

func TestLayoutControl_PointsWithinLimits_NoFlags(t *testing.T) {
	doc := ControlDocument{
		Y: []float64{3, 5, 7}, X: []float64{1, 2, 3},
		Mean: 5, UCL: 8, LCL: 2,
	}
	layout := LayoutControl(doc)
	if len(layout.Flags) != 0 {
		t.Errorf("expected no flags, got %d: %v", len(layout.Flags), layout.Flags)
	}
}

func TestLayoutControl_EmptyY_NoFlags(t *testing.T) {
	layout := LayoutControl(ControlDocument{})
	if len(layout.Flags) != 0 {
		t.Errorf("expected no flags for empty Y, got %d", len(layout.Flags))
	}
}

// ----- LayoutControl: categories from X -----

func TestLayoutControl_CategoriesFromX(t *testing.T) {
	doc := ControlDocument{
		Y: []float64{1, 2}, X: []float64{10, 20},
		Mean: 1.5, UCL: 3, LCL: 0,
	}
	layout := LayoutControl(doc)
	if len(layout.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(layout.Categories))
	}
	if layout.Categories[0] != "10" || layout.Categories[1] != "20" {
		t.Errorf("categories: got %v, want [10 20]", layout.Categories)
	}
}
