// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "encoding/json"

// PieDocument is the JSON shape stored in db.charts.data for a Pie
// chart.
type PieDocument struct {
	Title  string `json:"title,omitempty"`
	Slices []struct {
		Label string  `json:"label"`
		Value float64 `json:"value"`
		Color string  `json:"color,omitempty"`
	} `json:"slices"`
}

// ParsePie decodes the JSON blob.
func ParsePie(raw string) (PieDocument, error) {
	if raw == "" || raw == "{}" {
		return PieDocument{}, nil
	}
	var doc PieDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return PieDocument{}, err
	}
	return doc, nil
}

// LayoutPie computes each slice's percentage of the total and emits
// presentation-ready slice records.
func LayoutPie(doc PieDocument) StatsLayout {
	var total float64
	for _, s := range doc.Slices {
		total += s.Value
	}

	slices := make([]PieSlice, 0, len(doc.Slices))
	for _, s := range doc.Slices {
		var pct float64
		if total > 0 {
			pct = (s.Value / total) * 100
		}
		slices = append(slices, PieSlice{
			Label: s.Label,
			Value: s.Value,
			Pct:   pct,
			Color: s.Color,
		})
	}
	return StatsLayout{
		Kind:   "pie",
		Title:  doc.Title,
		Slices: slices,
	}
}
