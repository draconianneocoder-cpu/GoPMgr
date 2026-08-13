// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

// documents_test.go's smoke test seeds RenderProjectOverviewPDF from
// DefaultContent(), which is zero-valued, so every content-gated section
// had never executed under test. Unlike the prior five files in this
// package's coverage sweep, project_overview.go has no per-file
// getStringPO/getStringSlicePO accessors of its own -- it uses the
// shared getString/getStringSlice from charter.go -- so this file has
// fewer pure-accessor tests and more render/geometry tests.
//
// Three mutations were run against the three gates below, each reverted
// after checking the actual pass/fail result (not assumed from the
// mechanism) -- all 3 caught, though the third took two attempts:
//   - status != "" -> false: drawOverviewStatusBadge is skipped
//     regardless of the "status" field's value, so the populated and
//     empty renders become byte-identical (measured delta: 0). Caught.
//   - highlights len>0 -> false: same shape, same result. Caught.
//   - the milestones/budget/team OR-gate -> unconditional true: the
//     first test written for this gate (the fires-on-any-single-field
//     test below) survived it. drawOverviewGrid fills an empty card with
//     a literal "(not provided)" placeholder, so forcing the gate open
//     still produces a real, nonzero byte delta between a populated
//     field and its placeholder (measured: 35-65 bytes per field, well
//     above growthTolerance) -- but that test only asserts "populated
//     content produced more bytes than this same binary's own
//     empty-content baseline", a claim that stays true whether or not
//     the grid should have drawn anything at all for the empty baseline.
//     A growth-only comparison between two content-varied renders is
//     structurally unable to tell "the gate correctly suppressed the
//     section" apart from "the gate is gone but the section's own
//     content happens to differ from its placeholder". This is a
//     different mechanism than business_case.go's named limitation
//     (there the mutation collapsed both compared operands to identical
//     content) reaching the same conclusion -- that limitation's stated
//     reason turns out to be narrower than its stated conclusion. Unlike
//     that case, though, this gate IS pinnable: a direct ceiling
//     assertion on the all-empty render's absolute size (not a
//     comparison against a second, differently-mutated render) sidesteps
//     the growth-only blind spot entirely and does catch the mutation
//     (measured: 1969 bytes unmutated, 2201 with the gate forced open --
//     see TestRenderProjectOverviewPDF_Grid_NotDrawnWhenAllFieldsEmpty).
//
// Two of the three content-gated sections (status badge, highlights) are
// each guarded by their own independent key and are genuinely isolable
// by content -- confirmed by the mutation results above, not the source
// shape alone. The third -- the milestones/budget/team grid -- is a
// single OR-gate feeding ONE drawOverviewGrid call that always draws all
// three cards, pinned separately (by ceiling, not by growth comparison)
// from proving each field reaches its own card: a mutation swapping
// which field's text lands in which card would still be invisible to
// either assertion style used here.

func TestOverviewStatusColor(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantLabel string
		wantR     int
		wantG     int
		wantB     int
	}{
		{"green", "green", "ON TRACK", 21, 128, 61},
		{"on track synonym", "on track", "ON TRACK", 21, 128, 61},
		{"ok synonym", "ok", "ON TRACK", 21, 128, 61},
		{"healthy synonym, mixed case", "Healthy", "ON TRACK", 21, 128, 61},
		{"yellow", "yellow", "AT RISK", 180, 83, 9},
		{"amber synonym", "amber", "AT RISK", 180, 83, 9},
		{"at risk synonym", "at risk", "AT RISK", 180, 83, 9},
		{"caution synonym", "caution", "AT RISK", 180, 83, 9},
		{"red", "red", "OFF TRACK", 185, 28, 28},
		{"off track synonym", "off track", "OFF TRACK", 185, 28, 28},
		{"blocked synonym", "blocked", "OFF TRACK", 185, 28, 28},
		{"critical synonym", "critical", "OFF TRACK", 185, 28, 28},
		{"complete", "complete", "COMPLETE", 71, 85, 105},
		{"done synonym", "done", "COMPLETE", 71, 85, 105},
		{"closed synonym", "closed", "COMPLETE", 71, 85, 105},
		{"unrecognised status uppercased verbatim", "paused", "PAUSED", 100, 116, 139},
		{"whitespace trimmed before matching", "  green  ", "ON TRACK", 21, 128, 61},
		{"empty string falls back to UNKNOWN", "", "UNKNOWN", 100, 116, 139},
		{"whitespace-only falls back to UNKNOWN", "   ", "UNKNOWN", 100, 116, 139},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, r, g, b := overviewStatusColor(tt.status)
			if label != tt.wantLabel || r != tt.wantR || g != tt.wantG || b != tt.wantB {
				t.Errorf("overviewStatusColor(%q) = (%q, %d, %d, %d), want (%q, %d, %d, %d)",
					tt.status, label, r, g, b, tt.wantLabel, tt.wantR, tt.wantG, tt.wantB)
			}
		})
	}
}

func TestRenderProjectOverviewPDF_StatusBadge_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectOverviewPDF)
	got := mustRender(t, map[string]interface{}{
		"status": "green",
	}, "Test Project", RenderProjectOverviewPDF)
	assertGrew(t, got, empty, "RenderProjectOverviewPDF with only status populated")
}

// A garbage/unrecognised status string still reaches drawOverviewStatusBadge
// via the default branch (uppercased verbatim into a fixed-width badge) --
// a plausible real input (free-typed status text) that must not panic or
// silently drop the badge, same precedent as team_charter.go's
// out-of-range-allocation crash-safety case.
func TestRenderProjectOverviewPDF_UnrecognisedStatus_DoesNotPanicAndStillDraws(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectOverviewPDF)
	got := mustRender(t, map[string]interface{}{
		"status": "some totally unrecognised free-typed status value",
	}, "Test Project", RenderProjectOverviewPDF)
	assertGrew(t, got, empty, "RenderProjectOverviewPDF with an unrecognised status")
}

// status = "   " passes the raw `status != ""` gate in RenderProjectOverviewPDF
// (whitespace is not the empty string) but overviewStatusColor trims it down
// to the UNKNOWN default -- the badge still draws, just with the fallback
// label, not a silently blank one.
func TestRenderProjectOverviewPDF_WhitespaceOnlyStatus_StillDrawsBadge(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectOverviewPDF)
	got := mustRender(t, map[string]interface{}{
		"status": "   ",
	}, "Test Project", RenderProjectOverviewPDF)
	assertGrew(t, got, empty, "RenderProjectOverviewPDF with whitespace-only status")
}

func TestRenderProjectOverviewPDF_Highlights_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectOverviewPDF)
	got := mustRender(t, map[string]interface{}{
		"highlights": []interface{}{
			"Go-live moved up two weeks after vendor confirmed early delivery.",
			"Budget tracking 4% under forecast through Q2.",
		},
	}, "Test Project", RenderProjectOverviewPDF)
	assertGrew(t, got, empty, "RenderProjectOverviewPDF with only highlights populated")
}

// Gives drawOverviewGrid statement coverage with each of the three
// fields populated alone. On its own this does NOT pin the OR-gate
// itself -- a mutation forcing the gate unconditionally open survives
// this test alone, see the file-level comment for the measured reason --
// that gap is closed separately by the ceiling test below. This test
// also does NOT prove each field's text lands specifically in its own
// card; all three share one drawOverviewGrid call.
func TestRenderProjectOverviewPDF_Grid_FiresOnAnySingleField(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectOverviewPDF)

	cases := []struct {
		name    string
		content map[string]interface{}
	}{
		{"milestones only", map[string]interface{}{"milestones_summary": "Design freeze complete; build starts Monday."}},
		{"budget only", map[string]interface{}{"budget_summary": "$142k spent of $200k approved."}},
		{"team only", map[string]interface{}{"team_summary": "6 engineers, 1 designer, 1 PM."}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := mustRender(t, tt.content, "Test Project", RenderProjectOverviewPDF)
			assertGrew(t, got, empty, "RenderProjectOverviewPDF with "+tt.name+" populated")
		})
	}
}

// Pins the OR-gate itself, which the fires-on-any-single-field test above
// cannot: with all three summary fields empty, drawOverviewGrid must not
// be called at all -- the grid section (three cards, even placeholder-
// filled ones) must not appear on the page. Measured directly against
// the gate-forced-open mutation before picking the ceiling: an unmutated
// all-empty render is 1969 bytes; forcing `if milestones != "" || ... ` to
// an unconditional `if true` grows it to 2201 bytes (the three
// placeholder-filled cards). 2050 sits comfortably between the two --
// far above the footer timestamp's few-byte jitter, far below the
// mutated size -- so this test fails under that mutation and passes
// against the real implementation.
//
// Unlike every other assertion in this file, this is an absolute-size
// check, not a relative one -- it's only valid while no embedded font is
// registered via SetFontApplier (fonts.go is the only caller; no test in
// this package calls it, so newDocPDF falls back to fpdf's core
// Helvetica and 1969/2201 are deterministic today). Re-measure both
// numbers if that changes: a future font that shrinks output would leave
// this test passing without actually discriminating the mutation again.
func TestRenderProjectOverviewPDF_Grid_NotDrawnWhenAllFieldsEmpty(t *testing.T) {
	const ceiling = 2050
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderProjectOverviewPDF)
	if len(empty) >= ceiling {
		t.Errorf("RenderProjectOverviewPDF with no milestones/budget/team content = %d bytes, want < %d (the grid must not draw at all when every field is empty)", len(empty), ceiling)
	}
}

func TestOverviewCardHeight(t *testing.T) {
	pdf := newDocPDF("P")
	pdf.AddPage()

	t.Run("all empty bodies floor at 35", func(t *testing.T) {
		h := overviewCardHeight(pdf, []string{"", "", ""}, 50)
		if h != 35 {
			t.Errorf("overviewCardHeight(empty bodies) = %v, want 35 (the documented floor)", h)
		}
	})

	t.Run("a short body still floors at 35", func(t *testing.T) {
		h := overviewCardHeight(pdf, []string{"Short.", "", ""}, 50)
		if h != 35 {
			t.Errorf("overviewCardHeight(short body) = %v, want 35 (3-line minimum not exceeded)", h)
		}
	})

	// Each explicit newline adds strings.Count(body, "\n") to the estimated
	// line count, which the formula then multiplies by a fixed 4.5mm --
	// but the SAME formula also feeds the newline-inclusive string into
	// GetStringWidth, whose font-dependent value floors into the line
	// estimate too, so the two terms aren't separable in general. A
	// throwaway development-time probe (not checked in) measured
	// overviewCardHeight across 0-5 "\n"-joined repeats of a short line:
	// n=0..2 all held at the 35mm floor (the two terms together still
	// under 3 lines), and n=3..5 -- used below -- increased by exactly
	// 4.5mm per additional repeat once both terms pushed past the floor.
	// This pins the per-line increment in that regime; it does not claim
	// the formula's two terms are independently separable below it.
	t.Run("each additional newline-separated line adds exactly 4.5mm once past the floor", func(t *testing.T) {
		short := "one short line under the wrap width"
		repeat := func(n int) string {
			body := short
			for i := 0; i < n; i++ {
				body += "\n" + short
			}
			return body
		}
		h3 := overviewCardHeight(pdf, []string{repeat(3)}, 200)
		h4 := overviewCardHeight(pdf, []string{repeat(4)}, 200)
		h5 := overviewCardHeight(pdf, []string{repeat(5)}, 200)
		if h3 <= 35 {
			t.Fatalf("repeat(3) height = %v, want > 35 (test setup should already be past the floor)", h3)
		}
		if delta := h4 - h3; delta != 4.5 {
			t.Errorf("height delta from repeat(3) to repeat(4) = %v, want 4.5", delta)
		}
		if delta := h5 - h4; delta != 4.5 {
			t.Errorf("height delta from repeat(4) to repeat(5) = %v, want 4.5", delta)
		}
	})

	// A long single-line body forces GetStringWidth-based wrapping to
	// estimate more than the 3-line floor. The exact line count is
	// font-dependent (newDocPDF applies whatever embedded font is
	// currently registered as "Helvetica" via SetFontApplier), so this
	// only asserts the relational property -- taller than the floor --
	// not a hardcoded number of lines.
	t.Run("a long single-line body exceeds the floor", func(t *testing.T) {
		long := "This is a deliberately long single-line summary meant to wrap across several lines once GetStringWidth measures it against a narrow card width, exercising the line-count-derived branch of the height formula rather than the fixed 3-line floor."
		h := overviewCardHeight(pdf, []string{long}, 40)
		if h <= 35 {
			t.Errorf("overviewCardHeight(long body) = %v, want > 35 (should exceed the 3-line floor)", h)
		}
	})

	t.Run("the tallest of several bodies wins", func(t *testing.T) {
		short := "Short."
		long := "one\ntwo\nthree\nfour\nfive\nsix"
		hShortAlone := overviewCardHeight(pdf, []string{short, "", ""}, 200)
		hMixed := overviewCardHeight(pdf, []string{short, long, ""}, 200)
		if hMixed <= hShortAlone {
			t.Errorf("overviewCardHeight with one tall body = %v, want > %v (short-only case)", hMixed, hShortAlone)
		}
	})
}
