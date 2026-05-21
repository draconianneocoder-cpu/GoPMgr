// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import (
	"encoding/json"
	"math"
	"sort"
)

// Stakeholder is one entry in the Power × Interest analysis.
//
// Power and Interest are stored as strings ("low"/"high") so the user
// can edit them with a 2×2 dropdown in the GUI without having to
// reason about coordinates. The layout step converts them to
// (x, y) positions within the appropriate quadrant.
type Stakeholder struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
	Power    string `json:"power"`    // "low" | "high"
	Interest string `json:"interest"` // "low" | "high"
	Strategy string `json:"strategy,omitempty"`
	Note     string `json:"note,omitempty"`
}

// StakeholderDocument is the JSON shape stored in db.charts.data.
type StakeholderDocument struct {
	Stakeholders []Stakeholder `json:"stakeholders"`
}

// PlotPoint is one stakeholder positioned for the 2×2 visualisation.
type PlotPoint struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Role     string  `json:"role,omitempty"`
	Power    string  `json:"power"`
	Interest string  `json:"interest"`
	Strategy string  `json:"strategy"`
	X        float64 `json:"x"` // 0..1 within the canvas
	Y        float64 `json:"y"`
}

// QuadrantLabel describes one of the four engagement strategies.
type QuadrantLabel struct {
	Power    string `json:"power"`    // "low" | "high"
	Interest string `json:"interest"`
	Title    string `json:"title"`
	Strategy string `json:"strategy"`
}

// StakeholderLayout is the frontend payload.
type StakeholderLayout struct {
	Points    []PlotPoint     `json:"points"`
	Quadrants []QuadrantLabel `json:"quadrants"`
}

// ParseStakeholder decodes the JSON blob.
func ParseStakeholder(raw string) (StakeholderDocument, error) {
	if raw == "" || raw == "{}" {
		return StakeholderDocument{}, nil
	}
	var doc StakeholderDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return StakeholderDocument{}, err
	}
	return doc, nil
}

// LayoutStakeholder places each stakeholder inside the appropriate
// quadrant of a Power × Interest plot. Within a quadrant, points are
// distributed in a small grid to avoid label overlap when several
// stakeholders share the same classification.
//
// The output canvas is a unit square (0..1 in both axes). The
// frontend scales to its actual pixel size. Quadrant assignments:
//
//	(Power=low,  Interest=low)  → bottom-left   "Monitor"
//	(Power=low,  Interest=high) → bottom-right  "Keep Informed"
//	(Power=high, Interest=low)  → top-left      "Keep Satisfied"
//	(Power=high, Interest=high) → top-right     "Manage Closely"
//
// Each Stakeholder's Strategy field is overwritten with the canonical
// quadrant strategy so the frontend can render it without lookup.
func LayoutStakeholder(doc StakeholderDocument) StakeholderLayout {
	out := StakeholderLayout{
		Quadrants: []QuadrantLabel{
			{Power: "low", Interest: "low", Title: "Low Power · Low Interest", Strategy: "Monitor"},
			{Power: "low", Interest: "high", Title: "Low Power · High Interest", Strategy: "Keep Informed"},
			{Power: "high", Interest: "low", Title: "High Power · Low Interest", Strategy: "Keep Satisfied"},
			{Power: "high", Interest: "high", Title: "High Power · High Interest", Strategy: "Manage Closely"},
		},
	}

	// Bucket stakeholders by quadrant. Keys: "ll", "lh", "hl", "hh".
	buckets := make(map[string][]Stakeholder, 4)
	for _, s := range doc.Stakeholders {
		key := keyFor(s.Power, s.Interest)
		buckets[key] = append(buckets[key], s)
	}
	// Stable order within each bucket so layout is deterministic.
	for k := range buckets {
		sort.SliceStable(buckets[k], func(i, j int) bool {
			return buckets[k][i].Name < buckets[k][j].Name
		})
	}

	// Quadrant centres in the 0..1 canvas.
	centres := map[string][2]float64{
		"ll": {0.25, 0.75}, // bottom-left
		"lh": {0.75, 0.75}, // bottom-right
		"hl": {0.25, 0.25}, // top-left
		"hh": {0.75, 0.25}, // top-right
	}
	strategies := map[string]string{
		"ll": "Monitor",
		"lh": "Keep Informed",
		"hl": "Keep Satisfied",
		"hh": "Manage Closely",
	}

	for key, sl := range buckets {
		centre := centres[key]
		n := len(sl)
		if n == 0 {
			continue
		}
		// Lay each bucket out in a sqrt(n) × sqrt(n) micro-grid.
		cols := int(math.Ceil(math.Sqrt(float64(n))))
		if cols < 1 {
			cols = 1
		}
		const spread = 0.18 // half-width of the quadrant we'll fill
		stepX := (2 * spread) / float64(cols+1)
		for i, s := range sl {
			row := i / cols
			col := i % cols
			x := centre[0] - spread + stepX*float64(col+1)
			y := centre[1] - spread + stepX*float64(row+1)
			out.Points = append(out.Points, PlotPoint{
				ID:       s.ID,
				Name:     s.Name,
				Role:     s.Role,
				Power:    s.Power,
				Interest: s.Interest,
				Strategy: strategies[key],
				X:        x,
				Y:        y,
			})
		}
	}

	return out
}

func keyFor(power, interest string) string {
	p := "l"
	if power == "high" {
		p = "h"
	}
	i := "l"
	if interest == "high" {
		i = "h"
	}
	return p + i
}
