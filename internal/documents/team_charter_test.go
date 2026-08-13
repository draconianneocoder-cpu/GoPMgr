// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

// documents_test.go's smoke test seeds RenderTeamCharterPDF from
// DefaultContent() (members: [], ground_rules: [], team_purpose: ""), so
// every branch gated on non-empty content -- the members table, the
// allocation bar, the capacity banner -- has never executed under test.
// These tests populate real content to exercise those branches, following
// the same growthTolerance-based approach as charter_test.go (see that
// file's package comment for why exact/aggregate byte comparisons are
// unsafe here).

// ----- allocationColor: pure function, exact assertions -----

func TestAllocationColor(t *testing.T) {
	tests := []struct {
		name    string
		pct     float64
		r, g, b int
	}{
		{"zero", 0, 71, 85, 105},
		{"slate upper boundary", 25, 71, 85, 105},
		{"cyan lower boundary", 26, 14, 116, 144},
		{"cyan upper boundary", 50, 14, 116, 144},
		{"amber lower boundary", 51, 180, 83, 9},
		{"amber upper boundary", 75, 180, 83, 9},
		{"green lower boundary", 76, 21, 128, 61},
		{"green upper boundary (fully allocated)", 100, 21, 128, 61},
		{"red: just over-allocated", 100.5, 185, 28, 28},
		{"red: heavily over-allocated", 320, 185, 28, 28},
		{"negative (no clamp in this function; falls to default)", -10, 71, 85, 105},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := allocationColor(tt.pct)
			if r != tt.r || g != tt.g || b != tt.b {
				t.Errorf("allocationColor(%v) = (%d,%d,%d), want (%d,%d,%d)", tt.pct, r, g, b, tt.r, tt.g, tt.b)
			}
		})
	}
}

// ----- getStringTC / getFloatTC: pure accessors -----

func TestGetStringTC(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want string
	}{
		{"present", map[string]interface{}{"name": "Alex"}, "name", "Alex"},
		{"missing key", map[string]interface{}{}, "name", ""},
		{"wrong type falls back to empty", map[string]interface{}{"name": 42.0}, "name", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStringTC(tt.m, tt.key); got != tt.want {
				t.Errorf("getStringTC(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestGetFloatTC(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want float64
	}{
		{"present", map[string]interface{}{"allocation_pct": 75.0}, "allocation_pct", 75.0},
		{"missing key", map[string]interface{}{}, "allocation_pct", 0},
		{"wrong type falls back to zero", map[string]interface{}{"allocation_pct": "75"}, "allocation_pct", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFloatTC(tt.m, tt.key); got != tt.want {
				t.Errorf("getFloatTC(%v, %q) = %v, want %v", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

// ----- truncTC -----
//
// truncTC used to slice by byte index (s[:n-1]) with no rune-boundary
// check, which could split a multi-byte UTF-8 rune and produce invalid
// UTF-8 for non-ASCII input (confirmed at the time: truncTC("Zoë
// Müller-Åström the Third Extraordinaire", 4) returned "Zo\xc3…", a
// lone lead byte with no continuation byte). That finding was deferred
// rather than fixed in the increment that found it, and independently
// duplicated across eight more per-renderer trunc* helpers in this
// package, all with the same defect and none with any test coverage at
// all. Fixed by extracting one shared truncDoc (charter.go) that every
// trunc* function in the package -- including this one -- now delegates
// to. truncDoc keeps the same byte budget as the old implementations
// (an early rune-count-based draft was rejected: see truncDoc's doc
// comment for why) and just adds the missing rune-boundary check, so
// the historical example above now returns "Zo…" -- dropping the
// 2-byte "ë" whole, since it doesn't fit in the byte budget remaining
// after "Zo", rather than splitting it. See TestTruncDoc in
// charter_test.go for the shared function's own coverage (including a
// regression case using this exact historical example) and
// TestTruncWrappersDelegate for confirmation that all nine per-file
// wrappers actually call through to it. The cases below stay ASCII plus
// one non-ASCII passthrough case; the multi-byte truncation behavior
// itself is covered once, on the shared function, rather than re-proven
// per wrapper.
func TestTruncTC(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than limit passes through unchanged", "Alex", 10, "Alex"},
		{"exact-length boundary passes through unchanged", "Alexandra", 9, "Alexandra"},
		{"longer than limit is truncated with an ellipsis", "Alexandra Extraordinaire", 10, "Alexandra…"},
		{"non-ASCII input shorter than limit passes through unchanged", "Zoë Müller-Åström", 30, "Zoë Müller-Åström"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncTC(tt.s, tt.n); got != tt.want {
				t.Errorf("truncTC(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// ----- normaliseMembers -----

func TestNormaliseMembers_EmptyInput_ReturnsEmptySlice(t *testing.T) {
	got := normaliseMembers(nil)
	if len(got) != 0 {
		t.Errorf("normaliseMembers(nil) = %v, want empty", got)
	}
}

func TestNormaliseMembers_MapsFieldsAndDefaultsMissingOnes(t *testing.T) {
	raw := []map[string]interface{}{
		{"name": "Alex Stakeholder", "role": "Product Owner", "responsibilities": "Roadmap", "allocation_pct": 60.0},
		// Second row is missing every key -- normaliseMembers must default
		// via getStringTC/getFloatTC rather than panicking on a nil lookup.
		{},
	}
	got := normaliseMembers(raw)
	if len(got) != 2 {
		t.Fatalf("normaliseMembers() returned %d members, want 2", len(got))
	}
	first := got[0]
	if first.Name != "Alex Stakeholder" || first.Role != "Product Owner" || first.Responsibilities != "Roadmap" || first.AllocationPct != 60.0 {
		t.Errorf("normaliseMembers()[0] = %+v, want the mapped fields from raw[0]", first)
	}
	second := got[1]
	if second.Name != "" || second.Role != "" || second.Responsibilities != "" || second.AllocationPct != 0 {
		t.Errorf("normaliseMembers()[1] = %+v, want all-zero-value defaults for a row with no keys", second)
	}
}

// ----- RenderTeamCharterPDF: populated content -----

func TestRenderTeamCharterPDF_TeamPurpose_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderTeamCharterPDF)
	got := mustRender(t, map[string]interface{}{
		"team_purpose": "Deliver the ledger migration safely and on schedule.",
	}, "Test Project", RenderTeamCharterPDF)
	assertGrew(t, got, empty, "RenderTeamCharterPDF with only team_purpose populated")
}

func TestRenderTeamCharterPDF_GroundRules_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderTeamCharterPDF)
	got := mustRender(t, map[string]interface{}{
		"ground_rules": []interface{}{"Stand-up at 9am", "PRs need one approval"},
	}, "Test Project", RenderTeamCharterPDF)
	assertGrew(t, got, empty, "RenderTeamCharterPDF with only ground_rules populated")
}

// tcHeading, drawTeamTable, and drawTeamCapacityBanner all fire together
// inside the same `if len(members) > 0` block (team_charter.go:60-62),
// gated on the same `members` slice -- no combination of content can
// isolate one from the others the way charter.go's independently-gated
// stakeholder/milestone tables could.
//
// Fault-seeding measured this test's real assurance: baseline delta (all
// three intact) vs. empty is ~570 bytes. Gutting drawTeamTable alone still
// leaves a ~77-byte delta (heading + banner survive on their own); gutting
// tcHeading alone leaves ~550; gutting drawTeamCapacityBanner alone leaves
// ~519. All three individually-gutted deltas comfortably clear
// growthTolerance (12) against the *empty* baseline this test actually
// compares to -- so this test proves "populating members produces
// meaningfully more output than not populating them," not "each of the
// three calls independently fired." A defect confined to any single one
// of the three could survive this test. Isolating them individually would
// require restructuring RenderTeamCharterPDF's members branch into
// separately callable pieces, which is out of scope for a coverage-only
// increment (no speculative rewrites) -- recorded here and in the ledger
// rather than silently overclaimed.
func TestRenderTeamCharterPDF_MembersTable_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderTeamCharterPDF)
	got := mustRender(t, map[string]interface{}{
		"members": []interface{}{
			map[string]interface{}{"name": "Alex Stakeholder", "role": "Product Owner", "responsibilities": "Roadmap", "allocation_pct": 60.0},
			map[string]interface{}{"name": "Sam Reviewer", "role": "Compliance", "responsibilities": "Sign-off", "allocation_pct": 25.0},
		},
	}, "Test Project", RenderTeamCharterPDF)
	assertGrew(t, got, empty, "RenderTeamCharterPDF with only members populated")
}

// Regression/crash-safety: allocation_pct outside [0, 100] must not panic.
// drawAllocationBar clamps negative values to 0 before use; values over
// 100 are flagged red by allocationColor and clamped only for the filled
// bar's width, not the stored/displayed percentage.
func TestRenderTeamCharterPDF_OutOfRangeAllocation_DoesNotPanic(t *testing.T) {
	mustRender(t, map[string]interface{}{
		"members": []interface{}{
			map[string]interface{}{"name": "Over-allocated", "role": "Lead", "allocation_pct": 320.0},
			map[string]interface{}{"name": "Negative (malformed input)", "role": "QA", "allocation_pct": -15.0},
		},
	}, "Test Project", RenderTeamCharterPDF)
}
