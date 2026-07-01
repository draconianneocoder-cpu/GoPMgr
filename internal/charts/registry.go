// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package charts is the taxonomy and dispatch layer for PMForge's 20
// chart and diagram types.
//
// # Architectural overview
//
// Rather than 20 separate packages, all chart types share four
// engines. Each engine knows how to take a JSON `data` blob and a
// JSON `config` blob and produce (a) an internal layout suitable for
// the Svelte frontend, and (b) a rendering to PDF/PNG/SVG for export:
//
//   - dag     Hierarchical / directed-acyclic-graph layouts: WBS,
//     Network Diagram, PERT, CPM, Fishbone, Cause-and-Effect.
//   - stats   Quantitative data series: Line, Bar, Pareto, Pie,
//     Burn-Up, Burn-Down, Cumulative Flow, Control.
//   - matrix  Grid-based: RACI, SWOT, Stakeholder Analysis, Matrix Diagram.
//   - flow    Process flowcharts: Workflow, Activity.
//
// Each Kind is bound to exactly one engine in the registry below.
package charts

// Kind is the discriminator stored in db.charts.kind.
type Kind string

const (
	// DAG family
	KindWBS            Kind = "wbs"
	KindNetworkDiagram Kind = "network"
	KindPERT           Kind = "pert"
	KindCPM            Kind = "cpm"
	KindGantt          Kind = "gantt"
	KindFishbone       Kind = "fishbone"
	KindCauseAndEffect Kind = "cause_effect"

	// Statistical family
	KindLine           Kind = "line"
	KindBar            Kind = "bar"
	KindPareto         Kind = "pareto"
	KindPie            Kind = "pie"
	KindBurnUp         Kind = "burnup"
	KindBurnDown       Kind = "burndown"
	KindCumulativeFlow Kind = "cumulative_flow"
	KindControl        Kind = "control"

	// Matrix family
	KindRACI          Kind = "raci"
	KindSWOT          Kind = "swot"
	KindStakeholder   Kind = "stakeholder_analysis"
	KindMatrixDiagram Kind = "matrix"

	// Flow family
	KindWorkflow Kind = "workflow"
	KindActivity Kind = "activity"
)

// Engine is the family a Kind belongs to.
type Engine string

const (
	EngineDAG    Engine = "dag"
	EngineStats  Engine = "stats"
	EngineMatrix Engine = "matrix"
	EngineFlow   Engine = "flow"
)

// Definition describes one Kind: its display name, the engine that
// renders it, a short description, and the JSON shape its data uses.
//
// DataExample is a small reference document the GUI shows when the
// user picks "New <Kind>" with no template.
type Definition struct {
	Kind        Kind   `json:"kind"`
	Name        string `json:"name"`
	Engine      Engine `json:"engine"`
	Description string `json:"description"`
	DataExample string `json:"data_example"` // JSON
}

// registry is the master table. Keep this sorted by family for
// readability; the GUI re-sorts to its own preference.
var registry = []Definition{
	// -------- DAG family --------
	{
		Kind:        KindWBS,
		Name:        "Work Breakdown Structure",
		Engine:      EngineDAG,
		Description: "Hierarchical decomposition of project scope into deliverables and work packages.",
		DataExample: `{"root":{"id":"1","title":"Project","children":[{"id":"1.1","title":"Phase 1"},{"id":"1.2","title":"Phase 2"}]}}`,
	},
	{
		Kind:        KindNetworkDiagram,
		Name:        "Network Diagram",
		Engine:      EngineDAG,
		Description: "Activity-on-node diagram showing the precedence relationships among tasks.",
		DataExample: `{"nodes":[{"id":"A"},{"id":"B"}],"edges":[{"from":"A","to":"B"}]}`,
	},
	{
		Kind:        KindPERT,
		Name:        "PERT Chart",
		Engine:      EngineDAG,
		Description: "Program Evaluation and Review Technique. Activities annotated with optimistic, most likely, and pessimistic durations.",
		DataExample: `{"nodes":[{"id":"A","o":2,"m":3,"p":5}],"edges":[]}`,
	},
	{
		Kind:        KindCPM,
		Name:        "Critical Path Method Chart",
		Engine:      EngineDAG,
		Description: "Activity nodes annotated with ES/EF/LS/LF and critical-path highlighting.",
		DataExample: `{"nodes":[{"id":"A","duration":3}],"edges":[]}`,
	},
	{
		Kind:        KindGantt,
		Name:        "Gantt Chart",
		Engine:      EngineDAG,
		Description: "Schedule bars over a time axis with dependencies, critical path, progress, and baseline overlay. Shares the CPM data model.",
		DataExample: `{"nodes":[{"id":"A","label":"Design","duration":3}],"edges":[]}`,
	},
	{
		Kind:        KindFishbone,
		Name:        "Fishbone (Ishikawa) Diagram",
		Engine:      EngineDAG,
		Description: "Root-cause analysis. Categories (people, process, tools, ...) branch from a central effect.",
		DataExample: `{"effect":"Defect","categories":[{"name":"People","causes":["Training gap"]}]}`,
	},
	{
		Kind:        KindCauseAndEffect,
		Name:        "Cause-and-Effect Diagram",
		Engine:      EngineDAG,
		Description: "Generic cause/effect tree. Less rigid than Fishbone; supports nested causes.",
		DataExample: `{"effect":"Outcome","root":{"id":"c1","label":"Cause 1"}}`,
	},

	// -------- Statistical family --------
	{
		Kind:        KindLine,
		Name:        "Line Chart",
		Engine:      EngineStats,
		Description: "One or more series of values plotted against a continuous x-axis (typically time).",
		DataExample: `{"x":[1,2,3],"series":[{"name":"actual","y":[10,12,15]}]}`,
	},
	{
		Kind:        KindBar,
		Name:        "Bar Chart",
		Engine:      EngineStats,
		Description: "Categorical comparison. Vertical or horizontal bars per category.",
		DataExample: `{"categories":["Q1","Q2"],"series":[{"name":"revenue","values":[100,140]}]}`,
	},
	{
		Kind:        KindPareto,
		Name:        "Pareto Chart",
		Engine:      EngineStats,
		Description: "Bar chart sorted descending with a cumulative-percentage line. Surfaces the vital few.",
		DataExample: `{"items":[{"label":"A","count":40},{"label":"B","count":20}]}`,
	},
	{
		Kind:        KindPie,
		Name:        "Pie Chart",
		Engine:      EngineStats,
		Description: "Part-to-whole composition over a small number of categories.",
		DataExample: `{"slices":[{"label":"Done","value":60},{"label":"Open","value":40}]}`,
	},
	{
		Kind:        KindBurnUp,
		Name:        "Burn-Up Chart",
		Engine:      EngineStats,
		Description: "Cumulative scope completed against total scope over time. Distinguishes scope changes from progress.",
		DataExample: `{"days":[1,2,3],"completed":[2,5,8],"scope":[10,10,12]}`,
	},
	{
		Kind:        KindBurnDown,
		Name:        "Burn-Down Chart",
		Engine:      EngineStats,
		Description: "Remaining work over time, with an ideal-trajectory reference line.",
		DataExample: `{"days":[1,2,3],"remaining":[10,7,4]}`,
	},
	{
		Kind:        KindCumulativeFlow,
		Name:        "Cumulative Flow Diagram",
		Engine:      EngineStats,
		Description: "Stacked area chart of work-in-progress by workflow state over time.",
		DataExample: `{"days":[1,2,3],"states":{"todo":[5,4,2],"doing":[3,3,2],"done":[2,3,6]}}`,
	},
	{
		Kind:        KindControl,
		Name:        "Control Chart",
		Engine:      EngineStats,
		Description: "Time series with upper and lower control limits. Surfaces out-of-control points.",
		DataExample: `{"x":[1,2,3,4],"y":[5.1,5.3,4.8,7.9],"mean":5.0,"ucl":7.0,"lcl":3.0}`,
	},

	// -------- Matrix family --------
	{
		Kind:        KindRACI,
		Name:        "RACI Matrix",
		Engine:      EngineMatrix,
		Description: "Responsibility assignment: Responsible, Accountable, Consulted, Informed per (task, role).",
		DataExample: `{"roles":["PM","Dev"],"tasks":[{"id":"t1","title":"Plan"}],"assignments":{"t1":{"PM":"A","Dev":"R"}}}`,
	},
	{
		Kind:        KindSWOT,
		Name:        "SWOT Matrix",
		Engine:      EngineMatrix,
		Description: "2x2 grid of Strengths / Weaknesses / Opportunities / Threats.",
		DataExample: `{"strengths":[],"weaknesses":[],"opportunities":[],"threats":[]}`,
	},
	{
		Kind:        KindStakeholder,
		Name:        "Stakeholder Analysis Matrix",
		Engine:      EngineMatrix,
		Description: "Stakeholders plotted by Power and Interest to drive engagement strategy.",
		DataExample: `{"stakeholders":[{"name":"Sponsor","power":"high","interest":"high"}]}`,
	},
	{
		Kind:        KindMatrixDiagram,
		Name:        "Matrix Diagram",
		Engine:      EngineMatrix,
		Description: "Generic m×n grid for relating any two dimensions (used for requirements traceability, etc.).",
		DataExample: `{"rows":["R1"],"cols":["C1"],"cells":[[""]]}`,
	},

	// -------- Flow family --------
	{
		Kind:        KindWorkflow,
		Name:        "Workflow Diagram",
		Engine:      EngineFlow,
		Description: "Process flow with decisions, gates, and parallel paths.",
		DataExample: `{"nodes":[{"id":"start","type":"start"}],"edges":[]}`,
	},
	{
		Kind:        KindActivity,
		Name:        "Activity Diagram",
		Engine:      EngineFlow,
		Description: "UML-style activity flow with swimlanes.",
		DataExample: `{"swimlanes":[{"name":"User","activities":[]}]}`,
	},
}

// All returns every registered chart Definition, copied so callers
// cannot mutate the master table.
func All() []Definition {
	out := make([]Definition, len(registry))
	copy(out, registry)
	return out
}

// Get returns the Definition for a Kind, or (zero, false) if unknown.
func Get(k Kind) (Definition, bool) {
	for _, d := range registry {
		if d.Kind == k {
			return d, true
		}
	}
	return Definition{}, false
}

// ByEngine returns every Kind that the given engine handles.
func ByEngine(e Engine) []Definition {
	var out []Definition
	for _, d := range registry {
		if d.Engine == e {
			out = append(out, d)
		}
	}
	return out
}
