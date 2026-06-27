// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "encoding/json"

// BarSeries is one series in a stored BarDocument. Type and style
// fields are optional so ordinary bar charts stay compact, while
// generated overlays can render reference lines on top of bars.
type BarSeries struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
	Color  string    `json:"color,omitempty"`
	Type   string    `json:"type,omitempty"`
	YAxis  string    `json:"y_axis,omitempty"`
	Dashed bool      `json:"dashed,omitempty"`
}

// BarDocument is the JSON shape stored in db.charts.data for a Bar
// chart. Categories are mandatory; series.values must align 1:1.
type BarDocument struct {
	Title      string      `json:"title,omitempty"`
	XLabel     string      `json:"x_label,omitempty"`
	YLabel     string      `json:"y_label,omitempty"`
	Categories []string    `json:"categories"`
	Series     []BarSeries `json:"series"`
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
		seriesType := s.Type
		if seriesType == "" {
			seriesType = "bar"
		}
		series = append(series, Series{
			Name:   s.Name,
			Values: s.Values,
			Type:   seriesType,
			Color:  s.Color,
			YAxis:  s.YAxis,
			Dashed: s.Dashed,
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
