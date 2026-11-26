// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"encoding/json"
	"fmt"
	"math"
)

// ControlDocument is the JSON shape stored in db.charts.data for a
// Control Chart. X is the sample index (often time), Y is the
// measured value. Mean / UCL / LCL can be supplied explicitly. A Mean of 0
// is computed from the samples, and UCL and LCL that are both 0 become
// mean ± 3σ (see LayoutControl).
type ControlDocument struct {
	Title  string    `json:"title,omitempty"`
	YLabel string    `json:"y_label,omitempty"`
	X      []float64 `json:"x"`
	Y      []float64 `json:"y"`
	Mean   float64   `json:"mean,omitempty"`
	UCL    float64   `json:"ucl,omitempty"`
	LCL    float64   `json:"lcl,omitempty"`
}

// ParseControl decodes the JSON blob.
func ParseControl(raw string) (ControlDocument, error) {
	if raw == "" || raw == "{}" {
		return ControlDocument{}, nil
	}
	var doc ControlDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return ControlDocument{}, err
	}
	return doc, nil
}

// LayoutControl produces a single-series line chart annotated with
// horizontal lines for Mean, UCL, and LCL. Points that fall outside
// [LCL, UCL] are flagged with a red marker and an explanatory reason
// (used by the frontend to render an outlier badge).
//
// When the user hasn't supplied Mean/UCL/LCL, we compute mean and the
// standard ±3σ control limits ourselves so the chart is useful right
// after the first data entry.
func LayoutControl(doc ControlDocument) StatsLayout {
	mean := doc.Mean
	ucl := doc.UCL
	lcl := doc.LCL
	// A field left at 0 is computed, as the editor says. The two limits are
	// computed together, as mean ± 3σ of the samples, and only when both
	// are 0: one limit of 0 is a real value (a count's LCL). Before, a
	// chart with only a UCL drew its mean line at 0.
	if len(doc.Y) > 0 {
		sampleMean := computeMean(doc.Y)
		if mean == 0 {
			mean = sampleMean
		}
		if ucl == 0 && lcl == 0 {
			sigma := computeStdDev(doc.Y, sampleMean)
			ucl = mean + 3*sigma
			lcl = mean - 3*sigma
		}
	}

	flags := make([]PointFlag, 0)
	for i, y := range doc.Y {
		switch {
		case y > ucl:
			flags = append(flags, PointFlag{
				Series: 0,
				Point:  i,
				Color:  "#ef4444",
				Reason: fmt.Sprintf("Above UCL (%.3g)", ucl),
			})
		case y < lcl:
			flags = append(flags, PointFlag{
				Series: 0,
				Point:  i,
				Color:  "#ef4444",
				Reason: fmt.Sprintf("Below LCL (%.3g)", lcl),
			})
		}
	}

	return StatsLayout{
		Kind:       "control",
		Title:      doc.Title,
		XAxis:      AxisConfig{Type: "category"},
		YAxis:      AxisConfig{Label: doc.YLabel, Type: "linear"},
		Categories: floatsToStrings(doc.X),
		Series: []Series{
			{Name: "Measurement", Values: append([]float64{}, doc.Y...), Type: "line", Color: "#22d3ee"},
		},
		Annotations: []Annotation{
			{Type: "horizontal_line", Value: mean, Label: "Mean", Color: "#94a3b8"},
			{Type: "horizontal_line", Value: ucl, Label: "UCL", Color: "#ef4444", Dashed: true},
			{Type: "horizontal_line", Value: lcl, Label: "LCL", Color: "#ef4444", Dashed: true},
		},
		Flags: flags,
	}
}

func computeMean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func computeStdDev(xs []float64, mean float64) float64 {
	if len(xs) <= 1 {
		return 0
	}
	var sq float64
	for _, x := range xs {
		d := x - mean
		sq += d * d
	}
	return math.Sqrt(sq / float64(len(xs)-1))
}
