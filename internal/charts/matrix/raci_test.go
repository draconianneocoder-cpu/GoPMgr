// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import (
	"strings"
	"testing"
)

// ----- ParseRACI -----

func TestParseRACI_Empty(t *testing.T) {
	doc, err := ParseRACI("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if len(doc.Roles) != 0 || len(doc.Tasks) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseRACI_EmptyObject(t *testing.T) {
	// "{}" is treated like an empty string — returns early before the
	// nil-Assignments guard, so the result is an all-zero RACIDocument.
	doc, err := ParseRACI("{}")
	if err != nil {
		t.Fatalf("unexpected error for {}: %v", err)
	}
	if len(doc.Roles) != 0 || len(doc.Tasks) != 0 {
		t.Error("expected empty doc from {}")
	}
}

func TestParseRACI_InvalidJSON(t *testing.T) {
	_, err := ParseRACI("{invalid}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseRACI_ValidDocument(t *testing.T) {
	raw := `{
		"roles": ["Alice", "Bob"],
		"tasks": [{"id": "t1", "title": "Deploy"}],
		"assignments": {"t1": {"Alice": "R", "Bob": "A"}}
	}`
	doc, err := ParseRACI(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(doc.Roles))
	}
	if len(doc.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(doc.Tasks))
	}
	if doc.Assignments["t1"]["Alice"] != "R" {
		t.Error("expected Alice=R for task t1")
	}
	if doc.Assignments["t1"]["Bob"] != "A" {
		t.Error("expected Bob=A for task t1")
	}
}

// ----- LayoutRACI: cell grid -----

func TestLayoutRACI_CellGridCoversAllTaskRolePairs(t *testing.T) {
	doc := RACIDocument{
		Roles: []string{"Dev", "QA", "PM"},
		Tasks: []RACITask{
			{ID: "t1", Title: "Implement"},
			{ID: "t2", Title: "Test"},
		},
		Assignments: map[string]map[string]string{
			"t1": {"Dev": "R", "QA": "C", "PM": "A"},
			"t2": {"Dev": "C", "QA": "R", "PM": "A"},
		},
	}
	layout := LayoutRACI(doc)

	wantCells := len(doc.Roles) * len(doc.Tasks)
	if len(layout.Cells) != wantCells {
		t.Errorf("expected %d cells (roles × tasks), got %d", wantCells, len(layout.Cells))
	}
}

// ----- LayoutRACI: validation — Accountable rules -----

func TestLayoutRACI_NoAccountable_IsIssue(t *testing.T) {
	doc := RACIDocument{
		Roles: []string{"Alice"},
		Tasks: []RACITask{{ID: "t1", Title: "Deploy"}},
		Assignments: map[string]map[string]string{
			"t1": {"Alice": "R"}, // no A
		},
	}
	layout := LayoutRACI(doc)
	if layout.Validation.ErrorCount == 0 {
		t.Error("expected at least one validation issue for missing Accountable")
	}
	found := false
	for _, issue := range layout.Validation.Issues {
		if strings.Contains(issue, "no Accountable") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'no Accountable' message, got %v", layout.Validation.Issues)
	}
}

func TestLayoutRACI_MultipleAccountable_IsIssue(t *testing.T) {
	doc := RACIDocument{
		Roles: []string{"Alice", "Bob"},
		Tasks: []RACITask{{ID: "t1", Title: "Deploy"}},
		Assignments: map[string]map[string]string{
			"t1": {"Alice": "A", "Bob": "A"}, // two A — anti-pattern
		},
	}
	layout := LayoutRACI(doc)
	if layout.Validation.ErrorCount == 0 {
		t.Error("expected validation issue for multiple Accountable")
	}
	found := false
	for _, issue := range layout.Validation.Issues {
		if strings.Contains(issue, "Accountable roles") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Accountable roles' message, got %v", layout.Validation.Issues)
	}
}

func TestLayoutRACI_ExactlyOneAccountable_NoAccountableIssue(t *testing.T) {
	doc := RACIDocument{
		Roles: []string{"Alice", "Bob"},
		Tasks: []RACITask{{ID: "t1", Title: "Deploy"}},
		Assignments: map[string]map[string]string{
			"t1": {"Alice": "A", "Bob": "R"},
		},
	}
	layout := LayoutRACI(doc)
	for _, issue := range layout.Validation.Issues {
		if strings.Contains(issue, "Accountable") {
			t.Errorf("unexpected Accountable issue for valid assignment: %q", issue)
		}
	}
}

// ----- LayoutRACI: validation — Responsible rules -----

func TestLayoutRACI_NoResponsible_IsIssue(t *testing.T) {
	doc := RACIDocument{
		Roles: []string{"Alice"},
		Tasks: []RACITask{{ID: "t1", Title: "Deploy"}},
		Assignments: map[string]map[string]string{
			"t1": {"Alice": "A"}, // A but no R
		},
	}
	layout := LayoutRACI(doc)
	found := false
	for _, issue := range layout.Validation.Issues {
		if strings.Contains(issue, "no Responsible") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'no Responsible' message, got %v", layout.Validation.Issues)
	}
}

// ----- LayoutRACI: valid complete assignment -----

func TestLayoutRACI_ValidMatrix_NoIssues(t *testing.T) {
	doc := RACIDocument{
		Roles: []string{"Dev", "Lead"},
		Tasks: []RACITask{
			{ID: "t1", Title: "Code"},
			{ID: "t2", Title: "Review"},
		},
		Assignments: map[string]map[string]string{
			"t1": {"Dev": "R", "Lead": "A"},
			"t2": {"Dev": "C", "Lead": "A"},
		},
	}
	// t2 has no Responsible — add one.
	doc.Assignments["t2"]["Dev"] = "R"
	layout := LayoutRACI(doc)
	if layout.Validation.ErrorCount != 0 {
		t.Errorf("expected no issues, got %d: %v",
			layout.Validation.ErrorCount, layout.Validation.Issues)
	}
}

// ----- LayoutRACI: empty document -----

func TestLayoutRACI_EmptyDocument_NoIssues(t *testing.T) {
	layout := LayoutRACI(RACIDocument{})
	if layout.Validation.ErrorCount != 0 {
		t.Errorf("empty document should have no issues, got %d", layout.Validation.ErrorCount)
	}
	if len(layout.Cells) != 0 {
		t.Errorf("empty document should have no cells, got %d", len(layout.Cells))
	}
}

// ----- Validation.AddIssue -----

func TestValidationAddIssue(t *testing.T) {
	var v Validation
	v.AddIssue("first problem")
	v.AddIssue("second problem")

	if v.ErrorCount != 2 {
		t.Errorf("ErrorCount: got %d, want 2", v.ErrorCount)
	}
	if len(v.Issues) != 2 {
		t.Errorf("len(Issues): got %d, want 2", len(v.Issues))
	}
}
