// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import "encoding/json"

// SWOTDocument is the JSON shape stored in db.charts.data for a SWOT
// chart. Each quadrant is a list of strings the user has entered.
type SWOTDocument struct {
	Title         string   `json:"title,omitempty"`
	Strengths     []string `json:"strengths"`
	Weaknesses    []string `json:"weaknesses"`
	Opportunities []string `json:"opportunities"`
	Threats       []string `json:"threats"`
}

// SWOTQuadrant is one rendered pane.
type SWOTQuadrant struct {
	Key   string   `json:"key"`   // "S" | "W" | "O" | "T"
	Title string   `json:"title"` // "Strengths" | ...
	Items []string `json:"items"`
	// Position in the 2×2 grid. (0,0) is top-left.
	Row int `json:"row"`
	Col int `json:"col"`
	// Visual hint for the frontend palette.
	Tone string `json:"tone"` // "positive" | "negative" | "external_positive" | "external_negative"
}

// SWOTLayout is the frontend payload.
type SWOTLayout struct {
	Title     string         `json:"title,omitempty"`
	Quadrants []SWOTQuadrant `json:"quadrants"`
}

// ParseSWOT decodes a JSON blob into a SWOTDocument.
func ParseSWOT(raw string) (SWOTDocument, error) {
	if raw == "" || raw == "{}" {
		return SWOTDocument{}, nil
	}
	var doc SWOTDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return SWOTDocument{}, err
	}
	return doc, nil
}

// LayoutSWOT places the four quadrants in the canonical 2×2 grid:
//
//	Strengths (S)        Weaknesses (W)
//	Opportunities (O)    Threats (T)
//
// The "internal vs external" / "positive vs negative" classification
// is encoded in Tone so the frontend can colour-code without
// hard-coding the palette per quadrant.
func LayoutSWOT(doc SWOTDocument) SWOTLayout {
	return SWOTLayout{
		Title: doc.Title,
		Quadrants: []SWOTQuadrant{
			{Key: "S", Title: "Strengths", Items: doc.Strengths,
				Row: 0, Col: 0, Tone: "positive"},
			{Key: "W", Title: "Weaknesses", Items: doc.Weaknesses,
				Row: 0, Col: 1, Tone: "negative"},
			{Key: "O", Title: "Opportunities", Items: doc.Opportunities,
				Row: 1, Col: 0, Tone: "external_positive"},
			{Key: "T", Title: "Threats", Items: doc.Threats,
				Row: 1, Col: 1, Tone: "external_negative"},
		},
	}
}
