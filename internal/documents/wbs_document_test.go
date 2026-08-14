// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"testing"
	"time"
)

// documents_test.go's smoke test seeds RenderWBSDocumentPDF from
// DefaultContent(), which is zero-valued -- that already exercises the
// len(deliverables)==0 placeholder branch ("No deliverables documented
// yet."), which is why RenderWBSDocumentPDF started at 79.4% rather than
// 0%. It never reaches the wbs_ref banner, the deliverable tree itself,
// or any of wbs_document.go's six pure/draw-only helper functions, all
// of which were at 0%.
//
// Crash-safety check performed before writing any test (not assumed):
// drawWBSNode's cell-width arithmetic (maxWidth := 190 - indent, indent
// growing 8mm per WBS-code depth level) goes negative once depth
// exceeds ~21 -- a WBS code with 22+ dots, reachable from pasted or
// malformed JSON. Probed directly against fpdf v0.9.0 with depths up to
// 50 (well past the crossover), both with and without a populated
// description: no panic, no hang, in any case -- confirmed with a
// goroutine + time.After timeout, not just a synchronous call that could
// have masked an infinite loop. Reading fpdf's MultiCell source explains
// why: its line-wrapping loop scans the input character-by-character
// (i increments every iteration regardless of the target width), so a
// negative or zero wmax just means every character wraps onto its own
// line -- still O(n), not unbounded. TestRenderWBSDocumentPDF_DeepWBSCode_DoesNotHangOrPanic
// pins this as a real crash-safety regression test, not a hypothetical.

func TestGetStringWBS(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want string
	}{
		{"present", map[string]interface{}{"wbs_code": "1.2"}, "wbs_code", "1.2"},
		{"missing key", map[string]interface{}{}, "wbs_code", ""},
		{"wrong type falls back to empty", map[string]interface{}{"wbs_code": 12.0}, "wbs_code", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStringWBS(tt.m, tt.key); got != tt.want {
				t.Errorf("getStringWBS(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestNormaliseDeliverables(t *testing.T) {
	t.Run("empty input returns empty slice", func(t *testing.T) {
		got := normaliseDeliverables(nil)
		if len(got) != 0 {
			t.Errorf("normaliseDeliverables(nil) = %v, want empty", got)
		}
	})

	t.Run("maps fields and defaults missing keys to empty string", func(t *testing.T) {
		raw := []map[string]interface{}{
			{"wbs_code": "1.1", "description": "Design the schema", "acceptance_criteria": "Peer-reviewed"},
			{"wbs_code": "1.2"}, // description and acceptance_criteria omitted
		}
		got := normaliseDeliverables(raw)
		if len(got) != 2 {
			t.Fatalf("normaliseDeliverables() returned %d entries, want 2", len(got))
		}
		want0 := deliverable{Code: "1.1", Description: "Design the schema", AcceptanceCriteria: "Peer-reviewed"}
		if got[0] != want0 {
			t.Errorf("got[0] = %+v, want %+v", got[0], want0)
		}
		want1 := deliverable{Code: "1.2", Description: "", AcceptanceCriteria: ""}
		if got[1] != want1 {
			t.Errorf("got[1] = %+v, want %+v", got[1], want1)
		}
	})
}

func TestWBSDepth(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{"", 0},
		{"1", 0},
		{"1.2", 1},
		{"1.2.3", 2},
		{"1.2.3.4", 3},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := wbsDepth(tt.code); got != tt.want {
				t.Errorf("wbsDepth(%q) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestParseSegment(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"1", 1},
		{"10", 10},
		{"007", 7},
		{"", 0},
		{"a", 0},
		{"1a", 1}, // numeric prefix only; the trailing "a" is not consumed
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := parseSegment(tt.s); got != tt.want {
				t.Errorf("parseSegment(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

// parseSegment's accumulator (n = n*10 + digit) has no overflow guard.
// A pathologically long numeric segment (tried here at 24 nines, far
// beyond any real WBS numbering scheme) silently wraps rather than
// panicking -- Go's defined behavior for int overflow, confirmed
// directly rather than assumed. The only user-visible effect is a wrong
// sort position for that one malformed code; recorded in
// TEST_COVERAGE_LEDGER.md as an accepted, evidenced defect rather than
// fixed, per this package's convention for findings that are real but
// not worth the scope to close in a
// coverage-only increment (matching truncTC's now-fixed precedent, but
// without that finding's crash-adjacency -- this one can't corrupt
// output, only misorder one pathological row).
func TestParseSegment_LongNumericSegmentOverflowsWithoutPanicking(t *testing.T) {
	// The wrapped value itself isn't meaningful (two's-complement
	// overflow of an unbounded accumulator), but pinning the exact
	// observed value -- measured by running this input directly, not
	// derived -- turns "doesn't panic" into a real regression pin
	// rather than a tautology that would pass even if the function's
	// overflow behavior changed to something else non-panicking.
	// The pinned constant is 64-bit-int specific (int is the accumulator
	// type); this repo only builds for 64-bit targets, but a 32-bit
	// build would wrap to a different value and fail here.
	const in = "999999999999999999999999" // 24 nines
	const want = 2003764205206896639
	if got := parseSegment(in); got != want {
		t.Errorf("parseSegment(%q) = %d, want %d (the specific wrapped value observed for this input)", in, got, want)
	}
}

func TestWBSChipColor(t *testing.T) {
	tests := []struct {
		depth   int
		r, g, b int
	}{
		{0, 30, 58, 138},
		{1, 59, 130, 246},
		{2, 14, 116, 144},
		{3, 100, 116, 139},
		{4, 148, 163, 184},  // default band
		{10, 148, 163, 184}, // default band, far beyond the named cases
	}
	for _, tt := range tests {
		r, g, b := wbsChipColor(tt.depth)
		if r != tt.r || g != tt.g || b != tt.b {
			t.Errorf("wbsChipColor(%d) = (%d, %d, %d), want (%d, %d, %d)", tt.depth, r, g, b, tt.r, tt.g, tt.b)
		}
	}
}

// wbsCodeLess's expected ordering below was computed by running
// sort.SliceStable with this exact comparator over this exact fixture
// list and reading back the result (not derived by hand), per this
// package's established discipline after two prior increments'
// hand-computed expectations turned out wrong.
func TestWBSCodeLess(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"equal codes: neither is less", "1.2", "1.2", false},
		{"numeric segment comparison: 9 < 10, not lexical '9' > '1'", "1.9", "1.10", true},
		{"numeric segment comparison, reverse", "1.10", "1.9", false},
		{"shorter prefix sorts before its own extension", "1", "1.1", true},
		{"extension sorts after its own prefix", "1.1", "1", false},
		{"empty code sorts first", "", "1", true},
		{"empty code, reverse", "1", "", false},
		// Both segments parse to the same number (1), so the fallback
		// lexical comparison on the raw segment strings decides.
		{"numeric tie falls back to lexical comparison", "1a", "1b", true},
		{"numeric tie falls back to lexical comparison, reverse", "1b", "1a", false},
		{"different top-level numbers", "1.2", "2.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wbsCodeLess(tt.a, tt.b); got != tt.want {
				t.Errorf("wbsCodeLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestRenderWBSDocumentPDF_ChartBanner_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderWBSDocumentPDF)
	got := mustRender(t, map[string]interface{}{
		"wbs_ref": "wbs-chart-001",
	}, "Test Project", RenderWBSDocumentPDF)
	assertGrew(t, got, empty, "RenderWBSDocumentPDF with only wbs_ref populated")
}

func TestRenderWBSDocumentPDF_Deliverables_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderWBSDocumentPDF)
	got := mustRender(t, map[string]interface{}{
		"deliverable_descriptions": []interface{}{
			map[string]interface{}{
				"wbs_code":            "1.1",
				"description":         "Design the settlement matching schema.",
				"acceptance_criteria": "Reviewed and approved by the data team.",
			},
			map[string]interface{}{
				"wbs_code":            "1.2",
				"description":         "Implement the matching engine.",
				"acceptance_criteria": "Passes the full regression suite.",
			},
		},
	}, "Test Project", RenderWBSDocumentPDF)
	assertGrew(t, got, empty, "RenderWBSDocumentPDF with only deliverable_descriptions populated")
}

// drawWBSNode's description branch writes a fixed "(no description)"
// placeholder when empty, otherwise the real description via MultiCell
// -- structurally the same shape as project_overview.go's
// "(not provided)" card placeholder. Both mutation directions of that
// branch's guard (d.Description == "") were run, not assumed from the
// shape alone: forcing it to always take the placeholder branch (as if
// `if true`) is caught here, because both sides of this pairwise
// empty-vs-populated comparison then render the same placeholder text
// and assertGrew's delta collapses (measured: -3 bytes, i.e. no growth,
// well under growthTolerance). Forcing it to always take the MultiCell
// branch (as if `if false`) -- so an empty description calls
// MultiCell(descW, 5, "", ...) instead of drawing "(no description)" --
// SURVIVES this test: MultiCell on an empty string renders less than
// the 14-character placeholder would, so the "empty" side shrinks
// rather than staying flat, and the populated-vs-empty delta stays
// comfortably positive either way. Closed by
// TestRenderWBSDocumentPDF_EmptyDescription_ShowsPlaceholderNotBlank
// below, which pins the empty-description render's absolute size
// instead of comparing it against a second, differently-populated
// render -- the same technique project_overview.go's
// Grid_NotDrawnWhenAllFieldsEmpty test used for an analogous
// growth-only blind spot.
func TestRenderWBSDocumentPDF_DeliverableDescription_WrittenWhenPresent(t *testing.T) {
	base := func(desc string) map[string]interface{} {
		return map[string]interface{}{
			"deliverable_descriptions": []interface{}{
				map[string]interface{}{
					"wbs_code":    "1.1",
					"description": desc,
				},
			},
		}
	}
	empty := mustRender(t, base(""), "Test Project", RenderWBSDocumentPDF)
	got := mustRender(t, base("A specific, non-trivial deliverable description with real content."), "Test Project", RenderWBSDocumentPDF)
	assertGrew(t, got, empty, "RenderWBSDocumentPDF with a populated deliverable description vs. an empty one")
}

// Pins the description-empty branch itself (see the comment above for
// why the pairwise growth comparison can't): an empty description must
// render the "(no description)" placeholder, not a blank MultiCell call.
// Uses three empty-description deliverables rather than one so the
// real-vs-mutated gap multiplies ~3x, pushing it well clear of the
// footer timestamp's RFC3339Nano jitter (growthTolerance = 12 elsewhere
// in this package) -- a single-deliverable version of this test measured
// real=2200/mutated=2169 (31 bytes) and held across 50 repeated runs,
// but a floor sitting only ~1 byte outside the package's own declared
// jitter bound is fragile on principle, not just on this one measurement.
// Also valid only while no embedded font is registered via
// SetFontApplier: unlike the ceiling tests elsewhere in this package
// (where a font that renders *smaller* would falsely pass), a floor test
// like this one is undermined by a font that renders *larger* -- it
// would silently stop discriminating the mutation in the other
// direction. Measured directly against the `if false` mutation before
// picking the floor: real behavior renders 2405 bytes for this fixture,
// the mutation renders 2355 (a 50-byte gap, vs. 31 bytes for the
// single-deliverable version this replaced) -- 2380 sits at the
// midpoint, comfortably clear of growthTolerance on both sides.
func TestRenderWBSDocumentPDF_EmptyDescription_ShowsPlaceholderNotBlank(t *testing.T) {
	const floor = 2380
	deliverable := map[string]interface{}{
		"wbs_code":    "1.1",
		"description": "",
	}
	got := mustRender(t, map[string]interface{}{
		"deliverable_descriptions": []interface{}{deliverable, deliverable, deliverable},
	}, "Test Project", RenderWBSDocumentPDF)
	if len(got) <= floor {
		t.Errorf("RenderWBSDocumentPDF with 3 empty deliverable descriptions = %d bytes, want > %d (the \"(no description)\" placeholder must render, not a blank cell)", len(got), floor)
	}
}

func TestRenderWBSDocumentPDF_AcceptanceCriteria_WrittenWhenPresent(t *testing.T) {
	base := func(criteria string) map[string]interface{} {
		return map[string]interface{}{
			"deliverable_descriptions": []interface{}{
				map[string]interface{}{
					"wbs_code":            "1.1",
					"acceptance_criteria": criteria,
				},
			},
		}
	}
	empty := mustRender(t, base(""), "Test Project", RenderWBSDocumentPDF)
	got := mustRender(t, base("Must pass code review and integration tests before merge."), "Test Project", RenderWBSDocumentPDF)
	assertGrew(t, got, empty, "RenderWBSDocumentPDF with populated acceptance criteria vs. none")
}

// A WBS code with an empty string still renders (as "—", drawWBSNode's
// own placeholder for that field) rather than an empty or malformed
// chip -- a plausible real input (a deliverable added before its code
// was assigned). Growth-vs-fully-empty-baseline alone wouldn't pin the
// "—" substitution specifically (confirmed by running the codeLabel==""
// guard forced to `if false`, i.e. passing "" straight to the chip
// instead of substituting "—": that mutation still survives a plain
// assertGrew, since a deliverable card renders either way). Pinned
// instead with an absolute-size floor, using three empty-wbs_code
// deliverables rather than one so the real-vs-mutated gap multiplies
// ~3x, clear of the footer timestamp's RFC3339Nano jitter
// (growthTolerance = 12 elsewhere in this package) -- a single-
// deliverable version of this test first measured a floor from a
// differently-shaped probe fixture (no description field) than the
// actual fixture, which put the floor below both the real and mutated
// sizes and let the mutation survive undetected; corrected by
// re-measuring against this exact fixture, and now widened again for
// jitter margin rather than trusting an 11-byte gap. Also valid only
// while no embedded font is registered via SetFontApplier: a floor
// test like this one is undermined by a font that renders *larger*,
// the opposite failure direction from this package's ceiling tests.
// Measured directly against the `if false` mutation before picking the
// floor: real behavior renders 2449 bytes for this fixture, the
// mutation renders 2408 (a 41-byte gap, vs. 11 bytes for the
// single-deliverable version this replaced) -- 2428 sits at the
// midpoint, comfortably clear of growthTolerance on both sides.
func TestRenderWBSDocumentPDF_EmptyWBSCode_StillRendersPlaceholderChip(t *testing.T) {
	const floor = 2428
	deliverable := map[string]interface{}{
		"wbs_code":    "",
		"description": "A deliverable whose code hasn't been assigned yet.",
	}
	got := mustRender(t, map[string]interface{}{
		"deliverable_descriptions": []interface{}{deliverable, deliverable, deliverable},
	}, "Test Project", RenderWBSDocumentPDF)
	if len(got) <= floor {
		t.Errorf("RenderWBSDocumentPDF with 3 empty wbs_codes = %d bytes, want > %d (the \"—\" placeholder chip must render, not an empty one)", len(got), floor)
	}
}

// Crash-safety regression test -- see this file's top comment for the
// probe that established fpdf v0.9.0 degrades gracefully (no panic, no
// hang) rather than assumed safe. A WBS code with enough dots pushes
// drawWBSNode's computed maxWidth negative (crossover around depth 21:
// indent = 20 + depth*8 exceeds the 190mm page width budget).
func TestRenderWBSDocumentPDF_DeepWBSCode_DoesNotHangOrPanic(t *testing.T) {
	deepCode := ""
	for i := 0; i < 30; i++ {
		if i > 0 {
			deepCode += "."
		}
		deepCode += "1"
	}
	// The error is carried out on a channel rather than called via
	// t.Errorf inside the goroutine: if this test's own case (a hang)
	// ever occurred, t.Fatal below would return from the test function
	// while the goroutine was still blocked, and a later t.Errorf from
	// that still-running goroutine would panic ("Log in goroutine after
	// test has completed") instead of reporting a clean failure.
	errCh := make(chan error, 1)
	go func() {
		_, err := RenderWBSDocumentPDF(map[string]interface{}{
			"deliverable_descriptions": []interface{}{
				map[string]interface{}{
					"wbs_code":            deepCode,
					"description":         "A deliverable nested far deeper than any real WBS would go.",
					"acceptance_criteria": "Must not hang or panic regardless of depth.",
				},
			},
		}, "Test Project")
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("RenderWBSDocumentPDF with a 30-level-deep WBS code: unexpected error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RenderWBSDocumentPDF with a 30-level-deep WBS code HUNG")
	}
}
