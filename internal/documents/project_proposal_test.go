// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

// documents_test.go's smoke test seeds RenderProjectProposalPDF from
// DefaultContent(), which is zero-valued, so every one of its seven
// content-gated sections had never executed under test. Unlike
// team_charter.go's members-table trio (which all share one `if` block),
// every section here is gated by its OWN distinct content key
// (executive_summary, goals, approach, team, timeline, budget_summary,
// ask) -- so each can be genuinely isolated by populating only that one
// field, and fault-seeding below confirms each isolation actually works
// rather than assuming it from the source shape (the lesson from the
// team_charter.go increment).

func TestRenderProjectProposalPDF_ExecutiveSummary_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"executive_summary": "This project reduces settlement time from days to minutes.",
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with only executive_summary populated")
}

func TestRenderProjectProposalPDF_Goals_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"goals": []interface{}{"Cut settlement time by 90%", "Eliminate manual reconciliation"},
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with only goals populated")
}

func TestRenderProjectProposalPDF_Approach_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"approach": "Phased rollout starting with the highest-volume settlement corridor.",
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with only approach populated")
}

func TestRenderProjectProposalPDF_Team_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"team": []interface{}{"Alex Stakeholder", "Sam Reviewer"},
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with only team populated")
}

// A second team-chips test with enough long names to reach
// drawProposalTeamChips's wrap-to-next-row branch (x+w > rightEdge) for
// statement coverage -- the single-row case above doesn't reach it.
//
// This does NOT independently pin the wrap condition itself. Fault-seeding
// confirmed that: replacing the condition with `if false` still leaves
// every RoundedRect chip drawn (just unwrapped, running off the right
// edge instead of starting a new row) -- the byte-length delta this test
// measures is against the *empty* baseline, and unwrapped chips still
// produce comfortably more output than empty, so the mutation survives.
// Detecting the actual wrap (a Y-coordinate shift inside a compressed PDF
// stream) is the same fragility class the growthTolerance approach was
// built to avoid, so it isn't pursued here. The test's honest claim is
// "many long team names exercise the wrap code path without panicking or
// losing content," not "wrapping is verified correct."
func TestRenderProjectProposalPDF_TeamChips_ManyLongNamesExerciseWrapBranch(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"team": []interface{}{
			"Alexandra Stakeholder-Extraordinaire",
			"Bartholomew Reviewer-Extraordinaire",
			"Christopher Approver-Extraordinaire",
			"Dominique Sponsor-Extraordinaire",
			"Evangeline Auditor-Extraordinaire",
		},
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with wrapping team chips")
}

func TestRenderProjectProposalPDF_Timeline_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"timeline": "Kickoff March, pilot June, full rollout September.",
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with only timeline populated")
}

func TestRenderProjectProposalPDF_Ask_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectProposalPDF)
	got := mustRender(t, map[string]interface{}{
		"ask": "Approval for a $250,000 budget and two engineers for Q3.",
	}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, got, empty, "RenderProjectProposalPDF with only ask populated")
}

// budget_summary uses `budget > 0` (not `!= 0`), so unlike charter.go's
// FieldNumber guard this also excludes negative values, not just zero.
// Both boundary cases are asserted against the same jitter-safe pattern
// used for internal/documents' other zero/negative-guard tests.
func TestRenderProjectProposalPDF_BudgetTile_WrittenOnlyWhenPositive(t *testing.T) {
	zero := mustRender(t, map[string]interface{}{"budget_summary": 0.0}, "Test Project", RenderProjectProposalPDF)
	negative := mustRender(t, map[string]interface{}{"budget_summary": -500.0}, "Test Project", RenderProjectProposalPDF)
	if delta := len(negative) - len(zero); delta > growthTolerance || delta < -growthTolerance {
		t.Errorf("renderProjectProposalPDF(budget_summary=-500) vs (budget_summary=0): delta = %d bytes, want within +/-%d (a negative budget must be skipped exactly like zero, not rendered as a negative KPI tile)", delta, growthTolerance)
	}

	positive := mustRender(t, map[string]interface{}{"budget_summary": 250000.0}, "Test Project", RenderProjectProposalPDF)
	assertGrew(t, positive, zero, "RenderProjectProposalPDF with a positive budget_summary vs zero")
}
