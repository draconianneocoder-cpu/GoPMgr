// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"testing"

	"pmforge/internal/kernel"
)

func TestParseLinkLabel(t *testing.T) {
	cases := []struct {
		in  string
		typ kernel.LinkType
		lag float64
	}{
		{"", kernel.FinishToStart, 0},
		{"FS", kernel.FinishToStart, 0},
		{"fs", kernel.FinishToStart, 0},
		{"FS+2", kernel.FinishToStart, 2},
		{"SS-1", kernel.StartToStart, -1},
		{"ss + 1.5", kernel.StartToStart, 1.5},
		{"FF+0.5", kernel.FinishToFinish, 0.5},
		{"SF-2", kernel.StartToFinish, -2},
		{"+3", kernel.FinishToStart, 3},
		{"-1", kernel.FinishToStart, -1},
		{"hello", kernel.FinishToStart, 0},  // free text fails soft
		{"FS+abc", kernel.FinishToStart, 0}, // bad number fails soft
		{"SS2", kernel.FinishToStart, 0},    // missing sign fails soft
	}
	for _, c := range cases {
		typ, lag := ParseLinkLabel(c.in)
		if typ != c.typ || lag != c.lag {
			t.Errorf("ParseLinkLabel(%q) = (%s, %v), want (%s, %v)",
				c.in, typ, lag, c.typ, c.lag)
		}
	}
}

func TestLayoutCPM_HonoursEdgeLabels(t *testing.T) {
	nodes := []LayeredNode{
		{ID: "A", Label: "A", Duration: 5},
		{ID: "B", Label: "B", Duration: 2},
	}
	doc := LayeredDocument{
		Nodes: nodes,
		Edges: []LayeredEdge{{From: "A", To: "B", Label: "SS+1"}},
	}
	if _, err := LayoutCPM(doc); err != nil {
		t.Fatalf("LayoutCPM: %v", err)
	}

	// SS+1: B starts at A.ES(0)+1, no longer waits for A to finish.
	if nodes[1].ES != 1 {
		t.Errorf("B.ES = %v, want 1 (SS+1 must be honoured)", nodes[1].ES)
	}
	if nodes[1].EF != 3 {
		t.Errorf("B.EF = %v, want 3", nodes[1].EF)
	}
}
