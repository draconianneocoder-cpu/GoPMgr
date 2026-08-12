// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

// documents_test.go's smoke test seeds RenderBusinessCasePDF from
// DefaultContent(), which is zero-valued, so every content-gated section
// had never executed under test. Five of the six sections are
// independently gated by their own key(s); the alternatives and
// cost/ROI sections have their own internal nuances tested explicitly
// below rather than assumed, per this package's established habit of
// checking each mutation's individual result before claiming coverage.

func TestGetStringBC(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want string
	}{
		{"present", map[string]interface{}{"name": "Build in-house"}, "name", "Build in-house"},
		{"missing key", map[string]interface{}{}, "name", ""},
		{"wrong type falls back to empty", map[string]interface{}{"name": 42.0}, "name", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStringBC(tt.m, tt.key); got != tt.want {
				t.Errorf("getStringBC(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestRenderBusinessCasePDF_ProblemStatement_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"problem_statement": "Manual reconciliation costs 40 engineer-hours per week.",
	}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only problem_statement populated")
}

func TestRenderBusinessCasePDF_ProposedSolution_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"proposed_solution": "Automate settlement matching with a rules engine.",
	}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only proposed_solution populated")
}

func TestRenderBusinessCasePDF_Recommendation_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"recommendation": "Proceed with the in-house build; ROI breaks even at month 8.",
	}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only recommendation populated")
}

// drawBCAlternative early-returns when name, pros, AND cons are all
// empty (business_case.go:158) -- an alternative object with no usable
// fields must not draw an empty card. Content with one populated
// alternative must grow; content with one entirely-empty alternative
// object must not.
func TestRenderBusinessCasePDF_Alternatives_WrittenWhenPopulated(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"alternatives": []interface{}{
			map[string]interface{}{"name": "Build in-house", "pros": "Full control", "cons": "Longer timeline"},
		},
	}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with a populated alternative")
}

func TestRenderBusinessCasePDF_Alternatives_AllEmptyFieldsSkipsCard(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"alternatives": []interface{}{
			map[string]interface{}{},
		},
	}, "Test Project", RenderBusinessCasePDF)
	// The outer `if len(alts) > 0` still fires (drawing the "Alternatives
	// Considered" heading), so this isn't byte-identical to empty -- only
	// the per-card content must be suppressed. Measured heading-only
	// growth is ~41 bytes; a full card (header bar + two placeholder
	// columns' borders/dividers, drawn regardless of whether pros/cons
	// text is real or the "—" fallback) measured ~209 bytes beyond that.
	// An earlier version of this test compared got against a populated
	// card instead of this ceiling and did not catch the early-return
	// guard being removed: a card filled entirely with "—" placeholders
	// is close enough in size to a real card (same header bar, borders,
	// and column structure; only the body text differs) that "smaller
	// than populated" wasn't a tight enough bound. Confirmed by
	// fault-seeding: removing the guard grew this render from 2226 to
	// 2394 bytes, comfortably under the populated case's 2431 -- so that
	// comparison alone would have missed it.
	const headingOnlyCeiling = 100
	if delta := len(got) - len(empty); delta <= growthTolerance {
		t.Errorf("RenderBusinessCasePDF with one all-empty alternative (%d bytes) is not larger than empty (%d bytes); expected at least the section heading to render", len(got), len(empty))
	} else if delta > headingOnlyCeiling {
		t.Errorf("RenderBusinessCasePDF with one all-empty alternative grew by %d bytes over empty, want <= %d (heading-only); a full card may be rendering despite all fields being empty", delta, headingOnlyCeiling)
	}
}

// drawBCAlternative computes the card's outer border height as
// max(prosY, consY) after both columns are drawn (business_case.go:182-186).
// The single-line pros/cons text above never makes the cons column wrap
// to more lines than pros, so it never actually exercises the `consY >
// endY` branch. A cons value long enough to wrap across several lines
// (while pros stays short) forces cons to end lower than pros, hitting
// the branch the case above missed.
func TestRenderBusinessCasePDF_Alternatives_LongConsExercisesCardHeightBranch(t *testing.T) {
	mustRender(t, map[string]interface{}{
		"alternatives": []interface{}{
			map[string]interface{}{
				"name": "Buy off-the-shelf",
				"pros": "Fast",
				"cons": "Requires a multi-year contract, limited customization, vendor lock-in risk, and a migration cost if we ever need to switch providers later.",
			},
		},
	}, "Test Project", RenderBusinessCasePDF)
}

// drawBCColumn substitutes an em-dash placeholder when a column's body
// is empty (business_case.go:209-211) -- an alternative with a name but
// no pros/cons must still render two placeholder columns, not panic or
// silently omit them.
func TestRenderBusinessCasePDF_Alternatives_EmptyProsConsUsePlaceholder(t *testing.T) {
	mustRender(t, map[string]interface{}{
		"alternatives": []interface{}{
			map[string]interface{}{"name": "Do nothing"},
		},
	}, "Test Project", RenderBusinessCasePDF)
}

// costs_summary/roi share one outer gate (`cost != 0 || roi != ""`)
// wrapping two independently-guarded inner blocks -- same shape as
// project_brief.go's KPI strip, tested the same way: each alone, then
// neither. Unlike project_brief.go's `budget > 0` (excludes negative),
// this guard is `cost != 0` (includes negative) -- verified explicitly
// since a negative cost is a plausible real input (e.g. a
// cost-avoidance business case) and the two files' near-identical shape
// makes it easy to assume they share the same boundary when they don't.
func TestRenderBusinessCasePDF_CostRoi_CostAloneWritesSection(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{"costs_summary": 250000.0}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only costs_summary populated")
}

func TestRenderBusinessCasePDF_CostRoi_NegativeCostWritesSection(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{"costs_summary": -50000.0}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with a negative costs_summary")
}

func TestRenderBusinessCasePDF_CostRoi_RoiAloneWritesSection(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{"roi": "Breaks even at month 8"}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only roi populated")
}

// Known limitation, checked by fault-seeding rather than assumed: this
// test does NOT independently pin the outer `cost != 0 || roi != ""`
// gate. Widening that gate to an unconditional `if true` makes
// RenderBusinessCasePDF draw the "Cost & ROI Summary" heading for BOTH
// operands of this comparison -- the empty baseline above (also called
// with the mutated code) picks up the same heading, so the measured
// delta stays near zero even though the gate is broken. A test built
// only from comparing two content-varied renders of the same mutated
// code structurally cannot detect a mutation that removes
// content-dependence from the gate entirely. What IS independently
// pinned: the two INNER guards (cost != 0, roi != "") in the tests
// above, and specifically that this file's `!= 0` boundary (unlike
// project_brief.go's `> 0`) genuinely admits negative values -- verified
// by mutating it to `cost > 0` and confirming the negative-cost test
// catches that.
func TestRenderBusinessCasePDF_CostRoi_NeitherPresent_SectionNotWritten(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{"costs_summary": 0.0, "roi": ""}, "Test Project", RenderBusinessCasePDF)
	if delta := len(got) - len(empty); delta > growthTolerance || delta < -growthTolerance {
		t.Errorf("RenderBusinessCasePDF(costs_summary=0, roi=\"\") vs empty: delta = %d bytes, want within +/-%d (the Cost & ROI section must not draw when neither value is present)", delta, growthTolerance)
	}
}

// benefits/risks share one outer gate (`len(benefits) > 0 || len(risks)
// > 0`) wrapping drawBCTwoColumn, which -- unlike project_brief.go's KPI
// strip -- has NO per-side inner guard: it always draws both column
// headings once called, regardless of whether either slice is empty.
// That structural difference means (unverified until tested) the outer
// gate here may be the ONLY thing suppressing output when both are
// empty, unlike project_brief.go where the inner guards already did
// that job and the outer gate's removal went undetected by a
// byte-length test. Tested explicitly rather than assumed either way.
func TestRenderBusinessCasePDF_BenefitsRisks_BenefitsAloneWritesSection(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"benefits": []interface{}{"Faster settlement", "Fewer manual errors"},
	}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only benefits populated")
}

func TestRenderBusinessCasePDF_BenefitsRisks_RisksAloneWritesSection(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"risks": []interface{}{"Vendor lock-in", "Migration downtime"},
	}, "Test Project", RenderBusinessCasePDF)
	assertGrew(t, got, empty, "RenderBusinessCasePDF with only risks populated")
}

// Same limitation class as TestRenderBusinessCasePDF_CostRoi_NeitherPresent
// above: widening this outer gate to an unconditional `if true` draws
// both column headings for both operands of the comparison (including
// the empty baseline this test computes), so the delta stays near zero
// even though the gate is broken. Not independently pinned; see that
// test's comment for the full reasoning.
func TestRenderBusinessCasePDF_BenefitsRisks_NeitherPresent_SectionNotWritten(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderBusinessCasePDF)
	got := mustRender(t, map[string]interface{}{
		"benefits": []interface{}{}, "risks": []interface{}{},
	}, "Test Project", RenderBusinessCasePDF)
	if delta := len(got) - len(empty); delta > growthTolerance || delta < -growthTolerance {
		t.Errorf("RenderBusinessCasePDF(benefits=[], risks=[]) vs empty: delta = %d bytes, want within +/-%d (the Benefits vs Risks section must not draw when both are empty)", delta, growthTolerance)
	}
}
