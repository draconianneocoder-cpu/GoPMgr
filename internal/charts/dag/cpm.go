// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import "pmforge/internal/kernel"

// CPM (Critical Path Method) annotates each node with ES/EF/LS/LF,
// computes Float, and marks IsCritical=true for any node whose float
// is zero. The math lives in internal/kernel, so this file is a thin
// adapter between the LayeredDocument shape (used by the chart layer)
// and the kernel.Task shape (used by the scheduler).
//
// LayoutCPM mutates the input document in place.
func LayoutCPM(doc LayeredDocument) (Layout, error) {
	// Convert LayeredDocument → kernel task map.
	tasks := make(map[string]*kernel.Task, len(doc.Nodes))
	for _, n := range doc.Nodes {
		tasks[n.ID] = &kernel.Task{
			ID:       n.ID,
			Title:    n.Label,
			Duration: n.Duration,
		}
	}
	for _, e := range doc.Edges {
		if t, ok := tasks[e.To]; ok {
			t.Precedents = append(t.Precedents, e.From)
		}
	}

	// Compute. If the graph has a cycle the kernel returns false and
	// we propagate ErrCycle so the GUI can show a useful message.
	if ok := kernel.CalculateCPM(tasks); !ok {
		return Layout{}, ErrCycle
	}

	// Copy the kernel's results back into the LayeredDocument.
	for i := range doc.Nodes {
		t, ok := tasks[doc.Nodes[i].ID]
		if !ok {
			continue
		}
		doc.Nodes[i].ES = t.ES
		doc.Nodes[i].EF = t.EF
		doc.Nodes[i].LS = t.LS
		doc.Nodes[i].LF = t.LF
		doc.Nodes[i].Float = t.Float
		doc.Nodes[i].IsCritical = t.IsCritical
	}

	return LayoutLayered(doc, DefaultLayeredOptions())
}
