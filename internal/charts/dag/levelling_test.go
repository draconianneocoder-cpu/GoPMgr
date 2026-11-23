// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"strings"
	"testing"

	"gopmgr/internal/kernel"
)

// ctx stands for what levelling read outside the chart (the desktop builds
// it from the project start, calendar and referenced capacity).
const ctx = `{"start":"2026-06-01","calendar":"US"}`

// levelledDoc is a levelled chart: B carries a levelled start.
func levelledDoc() LayeredDocument {
	return LayeredDocument{
		Nodes: []LayeredNode{
			{ID: "A", Label: "Design", Duration: 2, Constraint: "snet", ConstraintDate: "2026-06-02",
				Assignments: []kernel.Assignment{{Resource: "alice", Units: 1, SkillTags: []string{"x", "a"}}}},
			{ID: "B", Label: "Build", Duration: 3, LevelStart: "2026-06-04",
				Assignments: []kernel.Assignment{{Resource: "alice"}}},
		},
		Edges: []LayeredEdge{{From: "A", To: "B", Label: "fs"}},
	}
}

// The digest changes exactly when something levelling reads changes (owner
// decision 2026-11-21: warn when the schedule changed since levelling).
func TestLevellingInputsDigestTracksSchedulingInputsOnly(t *testing.T) {
	base := LevellingInputsDigest(levelledDoc(), ctx)
	if !strings.HasPrefix(base, "v2:") {
		t.Fatalf("digest %q has no version prefix", base)
	}
	same := map[string]func(*LayeredDocument){
		"label":                      func(d *LayeredDocument) { d.Nodes[0].Label = "Renamed" },
		"progress":                   func(d *LayeredDocument) { d.Nodes[0].PercentComplete = 50 },
		"estimate":                   func(d *LayeredDocument) { d.Nodes[0].DurationEstimate.Optimistic = 1 },
		"levelled start":             func(d *LayeredDocument) { d.Nodes[1].LevelStart = "" },
		"split segments":             func(d *LayeredDocument) { d.Nodes[1].WorkSegments = []WorkSegment{{Start: 0, End: 1}} },
		"computed fields":            func(d *LayeredDocument) { d.Nodes[0].ES, d.Nodes[0].ConstraintViolated = 4, true },
		"constraint case and spaces": func(d *LayeredDocument) { d.Nodes[0].Constraint = " SNET " },
		"node order":                 func(d *LayeredDocument) { d.Nodes[0], d.Nodes[1] = d.Nodes[1], d.Nodes[0] },
		"skill tag order":            func(d *LayeredDocument) { d.Nodes[0].Assignments[0].SkillTags = []string{"a", "x"} },
		"empty skill tags":           func(d *LayeredDocument) { d.Nodes[1].Assignments[0].SkillTags = []string{} },
		"assignment with no resource": func(d *LayeredDocument) {
			d.Nodes[1].Assignments = append(d.Nodes[1].Assignments, kernel.Assignment{Units: 1})
		},
		"link label spelling":          func(d *LayeredDocument) { d.Edges[0].Label = "" },
		"stored digest and stale flag": func(d *LayeredDocument) { d.LevelledInputs, d.LevellingStale = "v1:x", true },
	}
	for name, edit := range same {
		doc := levelledDoc()
		edit(&doc)
		if got := LevellingInputsDigest(doc, ctx); got != base {
			t.Errorf("%s changed the digest", name)
		}
	}

	// The context outside the chart is part of what levelling read.
	if LevellingInputsDigest(levelledDoc(), `{"start":"2026-06-08","calendar":"US"}`) == base {
		t.Error("a different project start left the digest unchanged")
	}

	// A date left on a task whose constraint no longer uses one is not read.
	asap := levelledDoc()
	asap.Nodes[0].Constraint = "ASAP"
	asapBase := LevellingInputsDigest(asap, ctx)
	asap.Nodes[0].ConstraintDate = "2026-07-01"
	if LevellingInputsDigest(asap, ctx) != asapBase {
		t.Error("the date of an ASAP task changed the digest")
	}

	differ := map[string]func(*LayeredDocument){
		"duration":         func(d *LayeredDocument) { d.Nodes[0].Duration = 2.5 },
		"milestone":        func(d *LayeredDocument) { d.Nodes[1].Milestone = true },
		"constraint type":  func(d *LayeredDocument) { d.Nodes[0].Constraint = "ALAP" },
		"constraint date":  func(d *LayeredDocument) { d.Nodes[0].ConstraintDate = "2026-06-03" },
		"assignment units": func(d *LayeredDocument) { d.Nodes[0].Assignments[0].Units = 0.5 },
		"assignment":       func(d *LayeredDocument) { d.Nodes[1].Assignments = nil },
		"link type":        func(d *LayeredDocument) { d.Edges[0].Label = "SS" },
		"link lag":         func(d *LayeredDocument) { d.Edges[0].Label = "FS+1" },
		"link removed":     func(d *LayeredDocument) { d.Edges = nil },
		"task added":       func(d *LayeredDocument) { d.Nodes = append(d.Nodes, LayeredNode{ID: "C", Duration: 1}) },
	}
	for name, edit := range differ {
		doc := levelledDoc()
		edit(&doc)
		if LevellingInputsDigest(doc, ctx) == base {
			t.Errorf("%s left the digest unchanged", name)
		}
	}
}

// Levelling is stale only when a digest was stored, the inputs changed
// since, and some task still carries a levelled start or split segments.
func TestLevellingIsStale(t *testing.T) {
	stored := levelledDoc()
	stored.LevelledInputs = LevellingInputsDigest(stored, ctx)

	edited := stored
	edited.Nodes = append([]LayeredNode(nil), stored.Nodes...)
	edited.Nodes[0].Duration = 4

	cleared := edited
	cleared.Nodes = append([]LayeredNode(nil), edited.Nodes...)
	cleared.Nodes[1].LevelStart = ""

	never := edited
	never.LevelledInputs = ""

	// A digest from before the context was added (v1) never matches.
	older := stored
	older.LevelledInputs = "v1:" + strings.TrimPrefix(stored.LevelledInputs, "v2:")

	for name, c := range map[string]struct {
		doc  LayeredDocument
		want bool
	}{
		"unchanged since levelling":    {stored, false},
		"duration edited":              {edited, true},
		"edited, levelled starts gone": {cleared, false},
		"never levelled":               {never, false},
	} {
		if got := LevellingIsStale(c.doc, ctx); got != c.want {
			t.Errorf("%s: stale %v, want %v", name, got, c.want)
		}
	}
}
