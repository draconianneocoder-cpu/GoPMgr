// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"encoding/json"
	"errors"
	"fmt"

	"pmforge/internal/charts/dag"
	"pmforge/internal/charts/flow"
	"pmforge/internal/charts/matrix"
	"pmforge/internal/charts/stats"
)

// LayoutResult is the engine-agnostic shape the Wails bridge passes
// to the Svelte frontend. The frontend reads `Engine` and dispatches
// to the appropriate renderer.
type LayoutResult struct {
	Engine Engine          `json:"engine"`
	Kind   Kind            `json:"kind"`
	Title  string          `json:"title"`
	Body   json.RawMessage `json:"body"`
}

// ErrEngineNotImplemented is returned by Layout for kinds that ship
// in the V2 foundation as backend taxonomy entries but do not yet
// have a renderer wired up.
var ErrEngineNotImplemented = errors.New("charts: engine renderer not yet implemented")

// Layout dispatches a (kind, raw-JSON-data) pair to the correct
// engine and returns a LayoutResult the frontend can draw directly.
//
// DAG family — fully implemented in V2.1:
//   WBS, Network Diagram, PERT, CPM, Fishbone, Cause-and-Effect.
//
// Stats / Matrix / Flow families return ErrEngineNotImplemented.
// Implementing one of them is a self-contained change: add a function
// next to dag.LayoutWBS / stats.LayoutLine / matrix.LayoutRACI /
// flow.LayoutWorkflow and add a case to the switch below.
//
// The PERT and CPM cases also re-encode the input document with the
// computed annotations (Expected/Variance/StdDev or ES/EF/LS/LF/Float)
// embedded inline. This lets the frontend render those overlays
// without performing the math twice.
func Layout(kind Kind, rawData string) (LayoutResult, error) {
	def, ok := Get(kind)
	if !ok {
		return LayoutResult{}, fmt.Errorf("charts: unknown kind %q", kind)
	}

	switch kind {
	// --- DAG family: hierarchical / tree ---
	case KindWBS:
		doc, err := dag.Parse(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		dag.Renumber(&doc)
		layout := dag.LayoutWBS(doc, dag.DefaultLayoutOptions())
		return wrap(def, kind, layout)

	// --- DAG family: layered (Network / PERT / CPM) ---
	case KindNetworkDiagram:
		doc, err := dag.ParseLayered(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := dag.LayoutNetwork(doc)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrapWithAnnotated(def, kind, layout, doc)

	case KindPERT:
		doc, err := dag.ParseLayered(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := dag.LayoutPERT(doc)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrapWithAnnotated(def, kind, layout, doc)

	case KindCPM:
		doc, err := dag.ParseLayered(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := dag.LayoutCPM(doc)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrapWithAnnotated(def, kind, layout, doc)

	// --- DAG family: radial / causal ---
	case KindFishbone:
		doc, err := dag.ParseFishbone(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout := dag.LayoutFishbone(doc, dag.DefaultFishboneOptions())
		return wrap(def, kind, layout)

	case KindCauseAndEffect:
		doc, err := dag.ParseCausalTree(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := dag.LayoutCausalTree(doc)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, layout)

	// --- Flow family ---
	case KindWorkflow:
		doc, err := flow.ParseWorkflow(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := flow.LayoutWorkflow(doc, flow.DefaultOptions())
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, layout)

	case KindActivity:
		doc, err := flow.ParseActivity(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := flow.LayoutActivity(doc, flow.DefaultOptions())
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, layout)

	// --- Matrix family ---
	case KindRACI:
		doc, err := matrix.ParseRACI(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, matrix.LayoutRACI(doc))

	case KindSWOT:
		doc, err := matrix.ParseSWOT(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, matrix.LayoutSWOT(doc))

	case KindStakeholder:
		doc, err := matrix.ParseStakeholder(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, matrix.LayoutStakeholder(doc))

	case KindMatrixDiagram:
		doc, err := matrix.ParseGenericMatrix(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, matrix.LayoutGenericMatrix(doc))

	// --- Stats family ---
	case KindLine:
		doc, err := stats.ParseLine(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutLine(doc))

	case KindBar:
		doc, err := stats.ParseBar(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutBar(doc))

	case KindPareto:
		doc, err := stats.ParsePareto(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutPareto(doc))

	case KindPie:
		doc, err := stats.ParsePie(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutPie(doc))

	case KindBurnUp:
		doc, err := stats.ParseBurnUp(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutBurnUp(doc))

	case KindBurnDown:
		doc, err := stats.ParseBurnDown(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutBurnDown(doc))

	case KindCumulativeFlow:
		doc, err := stats.ParseCumFlow(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutCumFlow(doc))

	case KindControl:
		doc, err := stats.ParseControl(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		return wrap(def, kind, stats.LayoutControl(doc))
	}

	return LayoutResult{
		Engine: def.Engine,
		Kind:   kind,
		Title:  def.Name,
	}, ErrEngineNotImplemented
}

// wrap encodes `layout` as JSON and returns a populated LayoutResult.
func wrap(def Definition, kind Kind, layout interface{}) (LayoutResult, error) {
	body, err := json.Marshal(layout)
	if err != nil {
		return LayoutResult{}, err
	}
	return LayoutResult{
		Engine: def.Engine,
		Kind:   kind,
		Title:  def.Name,
		Body:   body,
	}, nil
}

// wrapWithAnnotated bundles the (layout, annotated-document) pair so
// the frontend can read both the geometry and the per-node fields
// (PERT durations, CPM floats) without re-running math.
//
// The body shape on the wire is:
//
//	{ "layout": {...nodes & edges...}, "doc": {...LayeredDocument...} }
func wrapWithAnnotated(def Definition, kind Kind, layout interface{}, doc dag.LayeredDocument) (LayoutResult, error) {
	body, err := json.Marshal(map[string]interface{}{
		"layout": layout,
		"doc":    doc,
	})
	if err != nil {
		return LayoutResult{}, err
	}
	return LayoutResult{
		Engine: def.Engine,
		Kind:   kind,
		Title:  def.Name,
		Body:   body,
	}, nil
}
