// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "encoding/json"

// BurnDownDocument is the JSON shape stored in db.charts.data.
// Remaining[i] is the work left at the end of Days[i]. The ideal
// trajectory (straight line from Remaining[0] to 0 across all days)
// is computed server-side so the GUI never has to.
type BurnDownDocument struct {
	Title     string    `json:"title,omitempty"`
	YLabel    string    `json:"y_label,omitempty"`
	Days      []float64 `json:"days"`
	Remaining []float64 `json:"remaining"`
}

// ParseBurnDown decodes the JSON blob.
func ParseBurnDown(raw string) (BurnDownDocument, error) {
	if raw == "" || raw == "{}" {
		return BurnDownDocument{}, nil
	}
	var doc BurnDownDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return BurnDownDocument{}, err
	}
	return doc, nil
}

// LayoutBurnDown emits a two-line chart: actual remaining work and
// the ideal linear trajectory from start to zero. When the actual
// line stays above the ideal, the team is behind schedule.
func LayoutBurnDown(doc BurnDownDocument) StatsLayout {
	ideal := computeIdealBurnDown(doc.Remaining, len(doc.Days))

	return StatsLayout{
		Kind:       "burndown",
		Title:      doc.Title,
		XAxis:      AxisConfig{Label: "Day", Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		Categories: floatsToStrings(doc.Days),
		Series: []Series{
			{Name: "Remaining", Values: append([]float64{}, doc.Remaining...), Type: "line", Color: "#22d3ee"},
			{Name: "Ideal", Values: ideal, Type: "line", Color: "#94a3b8", Dashed: true},
		},
	}
}

// computeIdealBurnDown builds a straight line from remaining[0] (the
// committed scope) down to 0 across n samples. Returns an empty slice
// if there's no committed scope to burn down from.
func computeIdealBurnDown(remaining []float64, n int) []float64 {
	if n == 0 || len(remaining) == 0 {
		return []float64{}
	}
	start := remaining[0]
	if n == 1 {
		return []float64{start}
	}
	out := make([]float64, n)
	step := start / float64(n-1)
	for i := 0; i < n; i++ {
		out[i] = start - step*float64(i)
		if out[i] < 0 {
			out[i] = 0
		}
	}
	return out
}
