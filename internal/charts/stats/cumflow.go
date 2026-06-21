// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"encoding/json"
	"sort"
)

// CumFlowDocument is the JSON shape stored in db.charts.data for a
// Cumulative Flow Diagram. Days is the x-axis. States is a map
// keyed by state name (e.g. "todo", "doing", "done") containing the
// WIP count for each day; all state arrays must have the same length
// as Days.
//
// StateOrder optionally controls the stacking order from bottom to
// top. When empty, states are stacked alphabetically.
type CumFlowDocument struct {
	Title      string               `json:"title,omitempty"`
	YLabel     string               `json:"y_label,omitempty"`
	Days       []float64            `json:"days"`
	States     map[string][]float64 `json:"states"`
	StateOrder []string             `json:"state_order,omitempty"`
}

// ParseCumFlow decodes the JSON blob.
func ParseCumFlow(raw string) (CumFlowDocument, error) {
	if raw == "" || raw == "{}" {
		return CumFlowDocument{}, nil
	}
	var doc CumFlowDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return CumFlowDocument{}, err
	}
	return doc, nil
}

// LayoutCumFlow produces a stacked area chart with one series per
// state, ordered bottom-to-top per StateOrder (or alphabetically
// when StateOrder is empty).
func LayoutCumFlow(doc CumFlowDocument) StatsLayout {
	// Resolve series order.
	var order []string
	if len(doc.StateOrder) > 0 {
		// Use explicit order, filtering out names not in States.
		for _, n := range doc.StateOrder {
			if _, ok := doc.States[n]; ok {
				order = append(order, n)
			}
		}
	} else {
		for name := range doc.States {
			order = append(order, name)
		}
		sort.Strings(order)
	}

	series := make([]Series, 0, len(order))
	for _, name := range order {
		values := doc.States[name]
		series = append(series, Series{
			Name:   name,
			Values: append([]float64{}, values...),
			Type:   "area",
		})
	}

	return StatsLayout{
		Kind:       "cumulative_flow",
		Title:      doc.Title,
		XAxis:      AxisConfig{Label: "Day", Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		Categories: floatsToStrings(doc.Days),
		Series:     series,
		Stacked:    true,
	}
}
