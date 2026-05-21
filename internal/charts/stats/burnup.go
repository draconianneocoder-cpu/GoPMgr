// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "encoding/json"

// BurnUpDocument is the JSON shape stored in db.charts.data for a
// Burn-Up chart. Days are the x-axis ticks; Completed is the
// cumulative work delivered; Scope is the total work in the backlog
// at each point (which can grow when scope creeps).
type BurnUpDocument struct {
	Title     string    `json:"title,omitempty"`
	YLabel    string    `json:"y_label,omitempty"` // e.g. "story points"
	Days      []float64 `json:"days"`
	Completed []float64 `json:"completed"`
	Scope     []float64 `json:"scope"`
}

// ParseBurnUp decodes the JSON blob.
func ParseBurnUp(raw string) (BurnUpDocument, error) {
	if raw == "" || raw == "{}" {
		return BurnUpDocument{}, nil
	}
	var doc BurnUpDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return BurnUpDocument{}, err
	}
	return doc, nil
}

// LayoutBurnUp produces a two-line chart: completed work (cyan) and
// total scope (amber). When completed catches up to scope the lines
// converge at the top.
func LayoutBurnUp(doc BurnUpDocument) StatsLayout {
	return StatsLayout{
		Kind:       "burnup",
		Title:      doc.Title,
		XAxis:      AxisConfig{Label: "Day", Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		Categories: floatsToStrings(doc.Days),
		Series: []Series{
			{Name: "Completed", Values: append([]float64{}, doc.Completed...), Type: "line", Color: "#22d3ee"},
			{Name: "Scope", Values: append([]float64{}, doc.Scope...), Type: "line", Color: "#f59e0b", Dashed: true},
		},
	}
}
