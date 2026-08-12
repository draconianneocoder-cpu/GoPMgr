// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

// documents_test.go's smoke test seeds RenderProjectBriefPDF from
// DefaultContent(), which is zero-valued, so every content-gated section
// had never executed under test. Four of the five content keys
// (summary, goals, roles, and the budget/timeline pair together) gate
// distinct sections, so each is independently isolable by content -- but
// budget and timeline share ONE outer gate (drawBriefKPIStrip fires if
// EITHER is present) before splitting into their own inner tiles, which
// is tested explicitly below rather than assumed.

func TestRenderProjectBriefPDF_Summary_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{
		"summary": "A short-lived initiative to cut settlement time from days to minutes.",
	}, "Test Project", RenderProjectBriefPDF)
	assertGrew(t, got, empty, "RenderProjectBriefPDF with only summary populated")
}

func TestRenderProjectBriefPDF_Goals_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{
		"goals": []interface{}{"Cut settlement time by 90%", "Eliminate manual reconciliation"},
	}, "Test Project", RenderProjectBriefPDF)
	assertGrew(t, got, empty, "RenderProjectBriefPDF with only goals populated")
}

func TestRenderProjectBriefPDF_Roles_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{
		"roles": []interface{}{"Sponsor: Jane Sponsor", "PM: Jo PM"},
	}, "Test Project", RenderProjectBriefPDF)
	assertGrew(t, got, empty, "RenderProjectBriefPDF with only roles populated")
}

// Statement coverage of drawBriefChips's wrap-to-next-row branch, same
// caveat as project_proposal.go's team-chip wrap test: this proves the
// branch executes under enough long input, not that the wrap itself
// renders at the geometrically correct position (see that file's test
// comment for why pinning the wrap position isn't pursued here).
func TestRenderProjectBriefPDF_RoleChips_ManyLongRolesExerciseWrapBranch(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{
		"roles": []interface{}{
			"Sponsor: Alexandra Stakeholder-Extraordinaire",
			"PM: Bartholomew Reviewer-Extraordinaire",
			"Lead: Christopher Approver-Extraordinaire",
			"QA: Dominique Sponsor-Extraordinaire",
			"Ops: Evangeline Auditor-Extraordinaire",
		},
	}, "Test Project", RenderProjectBriefPDF)
	assertGrew(t, got, empty, "RenderProjectBriefPDF with wrapping role chips")
}

// The KPI strip (drawBriefKPIStrip) fires if EITHER budget>0 OR
// timeline!="" -- a single outer OR-gate wrapping two independently
// gated inner tiles. Each of the three cases below is tested explicitly
// rather than assumed: budget alone, timeline alone, and neither (which
// must NOT fire the strip at all).

func TestRenderProjectBriefPDF_KPIStrip_BudgetAloneWritesStrip(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{"budget": 250000.0}, "Test Project", RenderProjectBriefPDF)
	assertGrew(t, got, empty, "RenderProjectBriefPDF with only budget populated")
}

func TestRenderProjectBriefPDF_KPIStrip_TimelineAloneWritesStrip(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{"timeline": "Q3 2026"}, "Test Project", RenderProjectBriefPDF)
	assertGrew(t, got, empty, "RenderProjectBriefPDF with only timeline populated")
}

// Zero budget and empty timeline must not fire the KPI strip at all --
// matching charter.go/project_proposal.go's established zero-guard
// pattern of asserting the delta stays within jitter range, not just
// "smaller than some populated case."
//
// Known limitation, checked by fault-seeding rather than assumed: this
// test does NOT independently pin the outer `budget > 0 || timeline !=
// ""` gate itself. drawBriefKPIStrip's own two inner guards (budget > 0,
// timeline != "") already prevent any visible tile from being drawn when
// both are absent, so removing the OUTER gate and always calling
// drawBriefKPIStrip produces byte-for-byte identical output in that
// case -- confirmed by mutation. The outer gate's real, narrower purpose
// is to skip drawBriefKPIStrip's unconditional trailing
// `pdf.SetXY(20, startY+tileH+5)`, which otherwise shifts the page
// cursor down ~27mm of wasted whitespace even when nothing was drawn.
// That's a real layout difference a reader would notice, but it changes
// vertical position, not byte count, so a length-based test structurally
// cannot see it -- the same class of limitation as the team-chip/role-chip
// wrap tests elsewhere in this package.
func TestRenderProjectBriefPDF_KPIStrip_NeitherPresent_StripNotWritten(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectBriefPDF)
	got := mustRender(t, map[string]interface{}{"budget": 0.0, "timeline": ""}, "Test Project", RenderProjectBriefPDF)
	if delta := len(got) - len(empty); delta > growthTolerance || delta < -growthTolerance {
		t.Errorf("RenderProjectBriefPDF(budget=0, timeline=\"\") vs empty: delta = %d bytes, want within +/-%d (the KPI strip must not draw visible content when neither value is present)", delta, growthTolerance)
	}
}
