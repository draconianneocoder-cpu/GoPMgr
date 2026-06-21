// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "encoding/json"

// BarDocument is the JSON shape stored in db.charts.data for a Bar
// chart. Categories are mandatory; series.values must align 1:1.
type BarDocument struct {
	Title      string   `json:"title,omitempty"`
	XLabel     string   `json:"x_label,omitempty"`
	YLabel     string   `json:"y_label,omitempty"`
	Categories []string `json:"categories"`
	Series     []struct {
		Name   string    `json:"name"`
		Values []float64 `json:"values"`
		Color  string    `json:"color,omitempty"`
	} `json:"series"`
}

// ParseBar decodes the JSON blob.
func ParseBar(raw string) (BarDocument, error) {
	if raw == "" || raw == "{}" {
		return BarDocument{}, nil
	}
	var doc BarDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return BarDocument{}, err
	}
	return doc, nil
}

// LayoutBar maps a BarDocument onto StatsLayout.
func LayoutBar(doc BarDocument) StatsLayout {
	series := make([]Series, 0, len(doc.Series))
	for _, s := range doc.Series {
		series = append(series, Series{
			Name:   s.Name,
			Values: s.Values,
			Type:   "bar",
			Color:  s.Color,
		})
	}
	return StatsLayout{
		Kind:       "bar",
		Title:      doc.Title,
		XAxis:      AxisConfig{Label: doc.XLabel, Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		Categories: append([]string{}, doc.Categories...),
		Series:     series,
	}
}
