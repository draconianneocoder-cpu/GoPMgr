// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import "testing"

func TestParseRiskMatrixRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseRiskMatrix("{bad}"); err == nil {
		t.Fatal("ParseRiskMatrix accepted invalid JSON")
	}
}

func TestLayoutRiskMatrixBuildsCanonicalFiveByFiveGrid(t *testing.T) {
	layout := LayoutRiskMatrix(RiskMatrixDocument{Items: []RiskItem{
		{ID: "R-1", Title: "Supplier delay", Kind: "risk", Probability: 4, Impact: 5},
		{ID: "O-1", Title: "Early delivery", Kind: "opportunity", Probability: 2, Impact: 3},
	}})

	if len(layout.Cells) != 25 {
		t.Fatalf("Cells = %d, want 25", len(layout.Cells))
	}
	if layout.Cells[0].Probability != 5 || layout.Cells[0].Impact != 1 {
		t.Fatalf("first cell = %+v, want probability 5 impact 1", layout.Cells[0])
	}
	if layout.Cells[0].Items == nil {
		t.Fatal("empty cell Items is nil, want [] for a stable frontend JSON contract")
	}
	if layout.Cells[24].Probability != 1 || layout.Cells[24].Impact != 5 {
		t.Fatalf("last cell = %+v, want probability 1 impact 5", layout.Cells[24])
	}
	if got := findRiskCell(t, layout, 4, 5); len(got.Items) != 1 || got.Items[0].ID != "R-1" || got.Band != "extreme" {
		t.Fatalf("P4/I5 cell = %+v, want R-1 in extreme band", got)
	}
	if got := findRiskCell(t, layout, 2, 3); len(got.Items) != 1 || got.Items[0].ID != "O-1" || got.Band != "medium" {
		t.Fatalf("P2/I3 cell = %+v, want O-1 in medium band", got)
	}
}

func TestLayoutRiskMatrixReportsInvalidCoordinatesAndDuplicateIDs(t *testing.T) {
	layout := LayoutRiskMatrix(RiskMatrixDocument{Items: []RiskItem{
		{ID: "R-1", Title: "Valid", Probability: 3, Impact: 3},
		{ID: "R-1", Title: "Duplicate", Probability: 2, Impact: 2},
		{ID: "R-3", Title: "Outside grid", Probability: 0, Impact: 6},
	}})

	if layout.Validation.ErrorCount != 2 {
		t.Fatalf("ErrorCount = %d, want 2; issues=%v", layout.Validation.ErrorCount, layout.Validation.Issues)
	}
	if got := findRiskCell(t, layout, 3, 3); len(got.Items) != 1 {
		t.Fatalf("valid cell items = %d, want 1", len(got.Items))
	}
}

func findRiskCell(t *testing.T, layout RiskMatrixLayout, probability, impact int) RiskCell {
	t.Helper()
	for _, cell := range layout.Cells {
		if cell.Probability == probability && cell.Impact == impact {
			return cell
		}
	}
	t.Fatalf("cell P%d/I%d not found", probability, impact)
	return RiskCell{}
}
