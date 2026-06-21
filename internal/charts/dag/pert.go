// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import "math"

// PERT (Program Evaluation and Review Technique) annotates each node
// with three time estimates — Optimistic, Most Likely, Pessimistic —
// and computes:
//
//	Expected duration  E = (O + 4M + P) / 6
//	Activity variance  V = ((P - O) / 6)^2
//	Standard deviation σ = sqrt(V)
//
// These are the textbook beta-distribution approximations.
//
// LayoutPERT mutates the input document in place to fill the computed
// fields (Expected/Variance/StdDev) so the frontend can render them
// alongside the node label.
func LayoutPERT(doc LayeredDocument) (Layout, error) {
	for i := range doc.Nodes {
		annotatePERT(&doc.Nodes[i])
	}
	return LayoutLayered(doc, DefaultLayeredOptions())
}

func annotatePERT(n *LayeredNode) {
	o := n.Optimistic
	m := n.MostLikely
	p := n.Pessimistic
	if o == 0 && m == 0 && p == 0 {
		return // user hasn't filled the durations yet — leave alone
	}
	e := (o + 4*m + p) / 6
	v := math.Pow((p-o)/6, 2)
	n.Expected = e
	n.Variance = v
	n.StdDev = math.Sqrt(v)
	// PERT also fills the generic Duration field so a node can be
	// fed into the CPM engine downstream without re-derivation.
	n.Duration = e
}
