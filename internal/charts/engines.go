// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"pmforge/internal/charts/dag"
	"pmforge/internal/charts/flow"
	"pmforge/internal/charts/matrix"
	"pmforge/internal/charts/stats"
	"pmforge/internal/kernel"
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

// ErrEngineNotImplemented is the defensive default returned by Layout
// when a kind is in the registry (Get succeeds) but has no switch arm
// below. All 20 shipped kinds have arms, so this is unreachable for
// them; it guards against a new registry entry added without a matching
// renderer. main.go treats it as a non-fatal "skip this chart" signal.
var ErrEngineNotImplemented = errors.New("charts: engine renderer not yet implemented")

// LayoutWithSchedule is Layout plus schedule context for the CPM
// kind: when projectStart is non-zero, every CPM node additionally
// carries StartDate/FinishDate (real dates, non-working days skipped
// via isWorkday), and date-bearing node constraints (SNET/FNLT/MFO)
// are armed and honoured by the CPM passes with violations flagged.
// All other kinds — and a zero projectStart — fall through to plain
// Layout, so callers without project context lose nothing by not
// using this entry point.
func LayoutWithSchedule(kind Kind, rawData string, projectStart time.Time, isWorkday kernel.WorkdayFunc, capacities map[string]float64) (LayoutResult, error) {
	if (kind != KindCPM && kind != KindGantt) || projectStart.IsZero() {
		return Layout(kind, rawData)
	}
	def, ok := Get(kind)
	if !ok {
		return LayoutResult{}, fmt.Errorf("charts: unknown kind %q", kind)
	}
	doc, err := dag.ParseLayered(rawData)
	if err != nil {
		return LayoutResult{}, err
	}
	var layout interface{}
	if kind == KindGantt {
		layout, err = dag.LayoutGanttScheduled(doc, projectStart, isWorkday, capacities)
	} else {
		layout, err = dag.LayoutCPMScheduled(doc, projectStart, isWorkday, capacities)
	}
	if err != nil {
		return LayoutResult{}, err
	}
	return wrapWithAnnotated(def, kind, layout, doc)
}

// Layout dispatches a (kind, raw-JSON-data) pair to the correct
// engine and returns a LayoutResult the frontend can draw directly.
//
// All four families are fully implemented:
//
//	DAG:    WBS, Network Diagram, PERT, CPM, Fishbone, Cause-and-Effect
//	Flow:   Workflow, Activity
//	Matrix: RACI, SWOT, Stakeholder, Generic
//	Stats:  Line, Bar, Pareto, Pie, BurnUp, BurnDown, CumulativeFlow, Control
//
// Adding a kind is a self-contained change: add a function next to the
// family's existing layouts (dag.LayoutWBS / flow.LayoutWorkflow /
// matrix.LayoutRACI / stats.LayoutLine) and add a case to the switch.
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

	case KindGantt:
		doc, err := dag.ParseLayered(rawData)
		if err != nil {
			return LayoutResult{}, err
		}
		layout, err := dag.LayoutGantt(doc)
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
