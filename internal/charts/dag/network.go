// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

// Network Diagram is the simplest layered-DAG kind: activities are
// nodes, precedence relationships are edges. No durations or floats
// are computed — that's PERT's and CPM's job.
//
// This file is intentionally short. All the heavy lifting (layering,
// barycenter ordering, coordinate assignment) lives in layered.go;
// the kind-specific work is just deciding what defaults to apply
// and what to surface in the layout result.

// LayoutNetwork builds a frontend-ready Layout for a Network Diagram.
// It does not modify the input document.
func LayoutNetwork(doc LayeredDocument) (Layout, error) {
	return LayoutLayered(doc, DefaultLayeredOptions())
}
