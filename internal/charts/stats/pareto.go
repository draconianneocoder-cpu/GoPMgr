// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"encoding/json"
	"sort"
)

// ParetoItem is one (label, count) pair the user enters.
type ParetoItem struct {
	Label string  `json:"label"`
	Count float64 `json:"count"`
}

// ParetoDocument is the JSON shape stored in db.charts.data.
type ParetoDocument struct {
	Title  string       `json:"title,omitempty"`
	YLabel string       `json:"y_label,omitempty"`
	Items  []ParetoItem `json:"items"`
}

// ParsePareto decodes the JSON blob.
func ParsePareto(raw string) (ParetoDocument, error) {
	if raw == "" || raw == "{}" {
		return ParetoDocument{}, nil
	}
	var doc ParetoDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return ParetoDocument{}, err
	}
	return doc, nil
}

// LayoutPareto sorts items descending by count and emits a mixed
// chart: a bar series for the counts (left y-axis) and a line series
// for the running cumulative percentage (right y-axis, 0-100).
//
// The 80/20 rule's vital-few cut-off is rendered as an annotation
// horizontal line at 80% on the right axis.
func LayoutPareto(doc ParetoDocument) StatsLayout {
	items := append([]ParetoItem{}, doc.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	cats := make([]string, len(items))
	counts := make([]float64, len(items))
	var total float64
	for i, it := range items {
		cats[i] = it.Label
		counts[i] = it.Count
		total += it.Count
	}

	cumPct := make([]float64, len(items))
	if total > 0 {
		var running float64
		for i, it := range items {
			running += it.Count
			cumPct[i] = (running / total) * 100
		}
	}

	right := AxisConfig{Label: "Cumulative %", Type: "linear"}
	zero, hundred := 0.0, 100.0
	right.Min = &zero
	right.Max = &hundred

	return StatsLayout{
		Kind:       "pareto",
		Title:      doc.Title,
		XAxis:      AxisConfig{Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		YAxisRight: &right,
		Categories: cats,
		Series: []Series{
			{Name: "Count", Values: counts, Type: "bar", YAxis: "left"},
			{Name: "Cumulative %", Values: cumPct, Type: "line", YAxis: "right", Color: "#ef4444"},
		},
		Annotations: []Annotation{
			{Type: "horizontal_line", Value: 80, Label: "80%", Color: "#f59e0b", Dashed: true},
		},
	}
}
