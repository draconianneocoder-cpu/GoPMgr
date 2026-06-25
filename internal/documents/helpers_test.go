// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"testing"
	"time"
)

// These tests pin the pure computational helpers shared by the document
// PDF renderers (date-window math, cost aggregation, issue
// classification). The fpdf draw calls in each Render*PDF are glue and
// intentionally left to the Render smoke test in documents_test.go.

// ----- parseDate (execution_plan.go) -----

func TestParseDate_Formats(t *testing.T) {
	tests := []struct {
		name string
		in   string
		zero bool
	}{
		{"empty", "", true},
		{"iso date", "2026-05-15", false},
		{"rfc3339", "2026-05-15T08:30:00Z", false},
		{"rfc3339 nano", "2026-05-15T08:30:00.123456789Z", false},
		{"garbage", "not-a-date", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDate(tt.in)
			if got.IsZero() != tt.zero {
				t.Errorf("parseDate(%q).IsZero() = %v, want %v", tt.in, got.IsZero(), tt.zero)
			}
		})
	}
}

// ----- computeProjectWindow (execution_plan.go) -----

func TestComputeProjectWindow_Empty(t *testing.T) {
	w := computeProjectWindow(nil)
	if !w.Start.IsZero() || !w.End.IsZero() || w.Days != 0 {
		t.Errorf("empty window: got %+v, want zero Start/End and Days 0", w)
	}
}

func TestComputeProjectWindow_InclusiveDays(t *testing.T) {
	// Jan 1 through Jan 10 spans 10 calendar days inclusive, not 9.
	tasks := []executionTask{
		{StartDate: "2026-01-01", EndDate: "2026-01-10"},
	}
	w := computeProjectWindow(tasks)
	if w.Days != 10 {
		t.Errorf("Days: got %d, want 10 (inclusive)", w.Days)
	}
	if !w.Start.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Start: got %v, want 2026-01-01", w.Start)
	}
	if !w.End.Equal(time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("End: got %v, want 2026-01-10", w.End)
	}
}

func TestComputeProjectWindow_SpansMultipleTasks(t *testing.T) {
	tasks := []executionTask{
		{StartDate: "2026-02-10", EndDate: "2026-02-15"},
		{StartDate: "2026-01-05", EndDate: "2026-01-20"}, // earliest start
		{StartDate: "2026-03-01", EndDate: "2026-03-31"}, // latest end
	}
	w := computeProjectWindow(tasks)
	if !w.Start.Equal(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Start: got %v, want 2026-01-05 (earliest)", w.Start)
	}
	if !w.End.Equal(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("End: got %v, want 2026-03-31 (latest)", w.End)
	}
}

func TestComputeProjectWindow_StartOnlyExtendsWindow(t *testing.T) {
	// A task with only a start date must still extend maxT (the
	// non-obvious third branch in computeProjectWindow).
	tasks := []executionTask{
		{StartDate: "2026-04-01", EndDate: ""},
	}
	w := computeProjectWindow(tasks)
	if !w.Start.Equal(w.End) {
		t.Errorf("start-only task: Start (%v) should equal End (%v)", w.Start, w.End)
	}
	if w.Days != 1 {
		t.Errorf("start-only task: Days got %d, want 1", w.Days)
	}
}

// ----- cost aggregation (execution_plan.go, procurement_plan.go, project_budget.go) -----

func TestSumExecutionCost(t *testing.T) {
	if got := sumExecutionCost(nil); got != 0 {
		t.Errorf("empty: got %v, want 0", got)
	}
	tasks := []executionTask{{Cost: 100}, {Cost: 250.5}, {Cost: 0}}
	if got := sumExecutionCost(tasks); got != 350.5 {
		t.Errorf("got %v, want 350.5", got)
	}
}

func TestProcurementTotal(t *testing.T) {
	if got := procurementTotal(nil); got != 0 {
		t.Errorf("empty: got %v, want 0", got)
	}
	items := []procurementItem{{Budget: 1000}, {Budget: 2500}}
	if got := procurementTotal(items); got != 3500 {
		t.Errorf("got %v, want 3500", got)
	}
}

func TestBudgetSubtotal(t *testing.T) {
	if got := budgetSubtotal(nil); got != 0 {
		t.Errorf("empty: got %v, want 0", got)
	}
	cats := []map[string]interface{}{
		{"amount": 1200.0},
		{"amount": 300.0},
		{"name": "no amount key"}, // missing key contributes 0
	}
	if got := budgetSubtotal(cats); got != 1500 {
		t.Errorf("got %v, want 1500", got)
	}
}

// ----- issue classification (issue_log.go) -----

func TestIsIssueResolved(t *testing.T) {
	resolved := []string{"resolved", "Closed ", "  DONE", "complete", "Completed"}
	for _, s := range resolved {
		if !isIssueResolved(s) {
			t.Errorf("isIssueResolved(%q) = false, want true", s)
		}
	}
	open := []string{"", "open", "in progress", "investigating"}
	for _, s := range open {
		if isIssueResolved(s) {
			t.Errorf("isIssueResolved(%q) = true, want false", s)
		}
	}
}

func TestIssueSeverityOrder(t *testing.T) {
	tests := []struct {
		sev  string
		want int
	}{
		{"critical", 0},
		{"High", 1}, // case-insensitive
		{" medium ", 2}, // trimmed
		{"low", 3},
		{"unknown", 4}, // default
		{"", 4},
	}
	for _, tt := range tests {
		if got := issueSeverityOrder(tt.sev); got != tt.want {
			t.Errorf("issueSeverityOrder(%q) = %d, want %d", tt.sev, got, tt.want)
		}
	}
}

func TestPartitionIssues_SplitAndSort(t *testing.T) {
	issues := []issue{
		{ID: "1", Severity: "medium", Status: "open"},
		{ID: "2", Severity: "critical", Status: ""},       // empty status -> open
		{ID: "3", Severity: "high", Status: "Closed"},     // resolved
		{ID: "4", Severity: "low", Status: "open"},
		{ID: "5", Severity: "critical", Status: "done"},   // resolved
	}
	open, resolved := partitionIssues(issues)

	// Empty status counts as open: ids 1, 2, 4.
	if len(open) != 3 {
		t.Fatalf("open: got %d, want 3", len(open))
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved: got %d, want 2", len(resolved))
	}
	// Open sorted by severity ascending order value: critical(0) < medium(2) < low(3).
	wantOpenOrder := []string{"2", "1", "4"}
	for i, id := range wantOpenOrder {
		if open[i].ID != id {
			t.Errorf("open[%d].ID = %q, want %q (severity sort)", i, open[i].ID, id)
		}
	}
	// Resolved sorted: critical(0) before high(1).
	if resolved[0].ID != "5" || resolved[1].ID != "3" {
		t.Errorf("resolved order = [%s, %s], want [5, 3]", resolved[0].ID, resolved[1].ID)
	}
}

// ----- accessor default-branch behaviour (representative) -----
//
// The per-document normalise*/getStringX/getFloatX helpers share one
// pattern: a type assertion that falls back to a zero value on a missing
// or wrong-typed key. One representative test pins that contract rather
// than replicating it across all ~20 near-identical copies.

func TestNormaliseExecutionTasks_DefaultsOnBadInput(t *testing.T) {
	raw := []map[string]interface{}{
		{"name": "Design", "owner": "Ana", "cost": 500.0},
		{"name": 123, "cost": "not a number"}, // wrong types -> zero values
		{}, // missing keys -> zero values
	}
	got := normaliseExecutionTasks(raw)
	if len(got) != 3 {
		t.Fatalf("got %d tasks, want 3", len(got))
	}
	if got[0].Name != "Design" || got[0].Owner != "Ana" || got[0].Cost != 500 {
		t.Errorf("row 0 mismapped: %+v", got[0])
	}
	// Wrong-typed values fall back to zero string / zero float.
	if got[1].Name != "" || got[1].Cost != 0 {
		t.Errorf("row 1 should default bad types to zero: %+v", got[1])
	}
	if got[2].Name != "" || got[2].Cost != 0 {
		t.Errorf("row 2 should default missing keys to zero: %+v", got[2])
	}
}
