// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import "encoding/json"

// LineDocument is the JSON shape stored in db.charts.data for a Line
// chart. X may be numeric (e.g. day indices) or already string-shaped.
type LineDocument struct {
	Title  string    `json:"title,omitempty"`
	XLabel string    `json:"x_label,omitempty"`
	YLabel string    `json:"y_label,omitempty"`
	X      []float64 `json:"x,omitempty"`
	XStr   []string  `json:"x_str,omitempty"` // alternative when x is categorical text
	Series []struct {
		Name   string    `json:"name"`
		Y      []float64 `json:"y"`
		Color  string    `json:"color,omitempty"`
		Dashed bool      `json:"dashed,omitempty"`
	} `json:"series"`
}

// ParseLine decodes the JSON blob.
func ParseLine(raw string) (LineDocument, error) {
	if raw == "" || raw == "{}" {
		return LineDocument{}, nil
	}
	var doc LineDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return LineDocument{}, err
	}
	return doc, nil
}

// LayoutLine maps a LineDocument onto the unified StatsLayout shape.
// String-typed XStr takes precedence when both are supplied.
func LayoutLine(doc LineDocument) StatsLayout {
	var categories []string
	if len(doc.XStr) > 0 {
		categories = append([]string{}, doc.XStr...)
	} else {
		categories = floatsToStrings(doc.X)
	}

	series := make([]Series, 0, len(doc.Series))
	for _, s := range doc.Series {
		series = append(series, Series{
			Name:   s.Name,
			Values: s.Y,
			Type:   "line",
			Color:  s.Color,
			Dashed: s.Dashed,
		})
	}

	return StatsLayout{
		Kind:       "line",
		Title:      doc.Title,
		XAxis:      AxisConfig{Label: doc.XLabel, Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		Categories: categories,
		Series:     series,
	}
}
