// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package stats implements PMForge's Stats-family chart engine.
//
// The family covers eight quantitative chart kinds:
//
//   - Line, Bar, Pareto, Pie     (categorical / one-dimensional)
//   - Burn-Up, Burn-Down         (cumulative progress vs time)
//   - Cumulative Flow            (stacked WIP by state vs time)
//   - Control                    (time-series with UCL/LCL bands)
//
// Unlike DAG / Flow / Matrix, Stats charts produce no geometry; their
// "layout" is a normalised data shape the frontend hands to Chart.js.
// This package therefore does NO rendering — it parses raw JSON,
// applies kind-specific math (sort + cumulative %, ideal trajectory,
// control limits, ...), and emits a StatsLayout the GUI consumes.
package stats

import "strconv"

// Series is one named series of y-values aligned to the chart's
// categories or x-points. `Type` is optional and only used by charts
// that mix series shapes — e.g., Pareto layers a "line" series for
// the cumulative-percentage curve on top of a "bar" series of counts.
type Series struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
	Type   string    `json:"type,omitempty"`   // "" | "line" | "bar" | "area"
	Color  string    `json:"color,omitempty"`  // optional hex, frontend defaults if empty
	YAxis  string    `json:"y_axis,omitempty"` // "" | "left" | "right" — Pareto uses right for %
	Dashed bool      `json:"dashed,omitempty"`
}

// AxisConfig configures one chart axis.
type AxisConfig struct {
	Label string   `json:"label,omitempty"`
	Type  string   `json:"type,omitempty"` // "category" | "linear" | "time"
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

// Annotation marks a horizontal line on the chart — used for control
// limits (UCL/LCL/Mean) and any future reference-line decorations.
type Annotation struct {
	Type   string  `json:"type"` // "horizontal_line"
	Value  float64 `json:"value"`
	Label  string  `json:"label,omitempty"`
	Color  string  `json:"color,omitempty"` // hex; frontend defaults if empty
	Dashed bool    `json:"dashed,omitempty"`
}

// PointFlag highlights a single (seriesIdx, pointIdx) — used by the
// Control chart to mark out-of-control samples.
type PointFlag struct {
	Series int    `json:"series"`
	Point  int    `json:"point"`
	Color  string `json:"color"`
	Reason string `json:"reason,omitempty"`
}

// PieSlice is one wedge in a Pie chart. The frontend ignores Pct on
// input and uses the backend-computed value so all eight kinds carry
// presentation-ready data.
type PieSlice struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Pct   float64 `json:"pct"`
	Color string  `json:"color,omitempty"`
}

// StatsLayout is the unified wire format. The frontend reads `Kind`
// and dispatches to the right Chart.js config builder.
//
// Field rules:
//
//   - Categories and Series are populated for Line/Bar/Pareto/BurnUp/
//     BurnDown/CumulativeFlow/Control.
//   - Slices is populated for Pie only.
//   - Annotations is non-empty for Control.
//   - Flags is non-empty for Control when there are out-of-control
//     samples.
//   - Stacked is true only for Cumulative Flow.
type StatsLayout struct {
	Kind  string `json:"kind"`
	Title string `json:"title,omitempty"`

	// Cartesian charts (everything except Pie)
	XAxis      AxisConfig  `json:"x_axis"`
	YAxis      AxisConfig  `json:"y_axis"`
	YAxisRight *AxisConfig `json:"y_axis_right,omitempty"` // Pareto only
	Categories []string    `json:"categories,omitempty"`
	Series     []Series    `json:"series,omitempty"`
	Stacked    bool        `json:"stacked,omitempty"`

	// Pie only
	Slices []PieSlice `json:"slices,omitempty"`

	// Reference lines and point highlights
	Annotations []Annotation `json:"annotations,omitempty"`
	Flags       []PointFlag  `json:"flags,omitempty"`
}

// floatsToStrings converts a slice of floats to their string
// representations using a compact format. Used when a chart's x-axis
// is numeric (e.g. day numbers) but the frontend expects category
// labels.
func floatsToStrings(xs []float64) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = strconv.FormatFloat(x, 'g', -1, 64)
	}
	return out
}
