// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import (
	"encoding/json"
	"fmt"
)

// RACI cell assignment values. Empty string = unassigned.
const (
	AssignResponsible = "R"
	AssignAccountable = "A"
	AssignConsulted   = "C"
	AssignInformed    = "I"
)

// RACITask is one row of the RACI matrix.
type RACITask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Note  string `json:"note,omitempty"`
}

// RACIDocument is the JSON shape stored in db.charts.data for a RACI
// chart.
//
// Assignments is keyed taskID → roleName → "R" | "A" | "C" | "I" | "".
// Roles are stored as plain strings (rather than IDs) because role
// names are visible to the user and editing one shouldn't break the
// reference; the backend's validation tolerates roles in Assignments
// that aren't in Roles by ignoring them.
type RACIDocument struct {
	Roles       []string                     `json:"roles"`
	Tasks       []RACITask                   `json:"tasks"`
	Assignments map[string]map[string]string `json:"assignments"`
}

// RACICell is one positioned cell ready for the frontend grid renderer.
type RACICell struct {
	TaskID string `json:"task_id"`
	Role   string `json:"role"`
	Value  string `json:"value"` // "R" | "A" | "C" | "I" | ""
}

// RACILayout is the frontend payload.
type RACILayout struct {
	Roles      []string   `json:"roles"`
	Tasks      []RACITask `json:"tasks"`
	Cells      []RACICell `json:"cells"`
	Validation Validation `json:"validation"`
}

// ParseRACI decodes a JSON blob into a RACIDocument.
func ParseRACI(raw string) (RACIDocument, error) {
	if raw == "" || raw == "{}" {
		return RACIDocument{}, nil
	}
	var doc RACIDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return RACIDocument{}, err
	}
	if doc.Assignments == nil {
		doc.Assignments = map[string]map[string]string{}
	}
	return doc, nil
}

// LayoutRACI turns a RACIDocument into a frontend-renderable payload
// and runs the standard RACI sanity checks:
//
//   - Each task must have exactly one Accountable role. Zero is a
//     "no owner" error; two or more is a "shared accountability"
//     error (a common RACI anti-pattern).
//   - Each task should have at least one Responsible role. (Warning,
//     not error.)
//
// Both error and warning lines go into the same Validation tray;
// future versions can separate severity if needed.
func LayoutRACI(doc RACIDocument) RACILayout {
	out := RACILayout{
		Roles: append([]string{}, doc.Roles...),
		Tasks: append([]RACITask{}, doc.Tasks...),
	}

	for _, t := range doc.Tasks {
		rolesForTask := doc.Assignments[t.ID]
		var aCount, rCount int
		for _, role := range doc.Roles {
			v := rolesForTask[role]
			out.Cells = append(out.Cells, RACICell{
				TaskID: t.ID,
				Role:   role,
				Value:  v,
			})
			switch v {
			case AssignAccountable:
				aCount++
			case AssignResponsible:
				rCount++
			}
		}

		if aCount == 0 {
			out.Validation.AddIssue(fmt.Sprintf(
				"Task %q has no Accountable role.", labelOrID(t)))
		} else if aCount > 1 {
			out.Validation.AddIssue(fmt.Sprintf(
				"Task %q has %d Accountable roles — exactly one is required.",
				labelOrID(t), aCount))
		}
		if rCount == 0 {
			out.Validation.AddIssue(fmt.Sprintf(
				"Task %q has no Responsible role.", labelOrID(t)))
		}
	}

	return out
}

func labelOrID(t RACITask) string {
	if t.Title != "" {
		return t.Title
	}
	return t.ID
}
