// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RiskItem is the shared record behind the 5x5 matrix. Kind distinguishes a
// possible threat, an active issue, and a beneficial opportunity without
// changing the probability x impact scoring contract.
type RiskItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Kind        string `json:"kind,omitempty"` // risk | issue | opportunity
	Probability int    `json:"probability"`
	Impact      int    `json:"impact"`
	Owner       string `json:"owner,omitempty"`
	Status      string `json:"status,omitempty"`
	Mitigation  string `json:"mitigation,omitempty"`
	LinkedTask  string `json:"linked_task,omitempty"`
}

type RiskMatrixDocument struct {
	Items []RiskItem `json:"items"`
}

// RiskCell is one probability/impact coordinate. Cells are emitted in visual
// row order (probability 5 down to 1, impact 1 up to 5) so every renderer can
// draw the canonical matrix without reimplementing axis orientation.
type RiskCell struct {
	Probability int        `json:"probability"`
	Impact      int        `json:"impact"`
	Score       int        `json:"score"`
	Band        string     `json:"band"`
	Items       []RiskItem `json:"items"`
}

type RiskMatrixLayout struct {
	Cells      []RiskCell `json:"cells"`
	Validation Validation `json:"validation"`
}

func ParseRiskMatrix(raw string) (RiskMatrixDocument, error) {
	if raw == "" || raw == "{}" {
		return RiskMatrixDocument{}, nil
	}
	var doc RiskMatrixDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return RiskMatrixDocument{}, err
	}
	return doc, nil
}

// LayoutRiskMatrix validates user-authored records and groups valid items into
// a deterministic 5x5 grid. Invalid entries remain in the saved document for
// correction but are excluded from cells so they cannot distort a report.
func LayoutRiskMatrix(doc RiskMatrixDocument) RiskMatrixLayout {
	out := RiskMatrixLayout{Cells: make([]RiskCell, 0, 25)}
	cellIndex := make(map[[2]int]int, 25)
	for probability := 5; probability >= 1; probability-- {
		for impact := 1; impact <= 5; impact++ {
			score := probability * impact
			cellIndex[[2]int{probability, impact}] = len(out.Cells)
			out.Cells = append(out.Cells, RiskCell{
				Probability: probability,
				Impact:      impact,
				Score:       score,
				Band:        riskBand(score),
				Items:       []RiskItem{},
			})
		}
	}

	seen := make(map[string]bool, len(doc.Items))
	for i, item := range doc.Items {
		item.ID = strings.TrimSpace(item.ID)
		item.Title = strings.TrimSpace(item.Title)
		if item.ID == "" {
			out.Validation.AddIssue(fmt.Sprintf("Item %d requires an ID.", i+1))
			continue
		}
		if seen[item.ID] {
			out.Validation.AddIssue(fmt.Sprintf("Item ID %q is duplicated.", item.ID))
			continue
		}
		seen[item.ID] = true
		if item.Title == "" {
			out.Validation.AddIssue(fmt.Sprintf("%s requires a title.", item.ID))
			continue
		}
		item.Kind = strings.ToLower(strings.TrimSpace(item.Kind))
		if item.Probability < 1 || item.Probability > 5 || item.Impact < 1 || item.Impact > 5 {
			out.Validation.AddIssue(fmt.Sprintf("%s must use probability and impact values from 1 to 5.", item.ID))
			continue
		}
		if item.Kind == "" {
			item.Kind = "risk"
		}
		switch item.Kind {
		case "risk", "issue", "opportunity":
		default:
			out.Validation.AddIssue(fmt.Sprintf("%s has unsupported kind %q.", item.ID, item.Kind))
			continue
		}
		idx := cellIndex[[2]int{item.Probability, item.Impact}]
		out.Cells[idx].Items = append(out.Cells[idx].Items, item)
	}
	return out
}

func riskBand(score int) string {
	switch {
	case score >= 20:
		return "extreme"
	case score >= 13:
		return "severe"
	case score >= 7:
		return "high"
	case score >= 4:
		return "medium"
	default:
		return "low"
	}
}
