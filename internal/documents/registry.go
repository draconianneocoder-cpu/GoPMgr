// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package documents owns the 25 PMForge document types: their
// taxonomy, JSON schemas, default content templates, and the
// validation / rendering plumbing that turns a row in db.documents
// into a polished PDF/DOCX/ODT.
//
// # Architecture
//
// Every document type is registered in the master `registry` slice in
// templates.go. Each entry binds a Kind to its display metadata
// (name, phase, description) and a Definition that specifies the
// JSON shape of its content. The validator and renderer dispatch on
// Kind. The frontend reads All() to populate its "new document" menu
// and ByPhase() to organise documents by lifecycle phase.
package documents

// Kind is the discriminator stored in db.documents.kind.
type Kind string

// The 25 document kinds. Identifier convention: snake_case, prefixed
// where helpful to disambiguate (e.g. "plan_word" vs "plan_excel").
const (
	KindProjectPlanWord     Kind = "plan_word"
	KindProjectCharterWord  Kind = "charter_word"
	KindBusinessCase        Kind = "business_case"
	KindProjectSchedule     Kind = "schedule"
	KindWBSDocument         Kind = "wbs_doc"
	KindRACIDocument        Kind = "raci_doc"
	KindRiskRegister        Kind = "risk_register"
	KindScopeStatement      Kind = "scope_statement"
	KindProjectBudget       Kind = "budget"
	KindCommunicationPlan   Kind = "communication_plan"
	KindExecutionPlan       Kind = "execution_plan"
	KindStatusReport        Kind = "status_report"
	KindStatementOfWork     Kind = "statement_of_work"
	KindProjectClosure      Kind = "closure"
	KindProjectProposal     Kind = "proposal"
	KindProcurementPlan     Kind = "procurement_plan"
	KindProjectPlanExcel    Kind = "plan_excel"
	KindIssueLog            Kind = "issue_log"
	KindChangeRequest       Kind = "change_request"
	KindProjectBrief        Kind = "brief"
	KindRequirements        Kind = "requirements"
	KindProjectCharterExcel Kind = "charter_excel"
	KindProjectOverview     Kind = "overview"
	KindTeamCharter         Kind = "team_charter"
	KindStakeholderAnalysis Kind = "stakeholder_analysis_doc"
)

// Phase is one of PMI's process groups. Used to organise documents by
// where in the project lifecycle they're typically created.
type Phase string

const (
	PhaseInitiation Phase = "initiation"
	PhasePlanning   Phase = "planning"
	PhaseExecution  Phase = "execution"
	PhaseMonitoring Phase = "monitoring"
	PhaseClosing    Phase = "closing"
)

// FieldKind is the data type of one Schema field. The frontend's
// generic form renderer reads this to pick the right input widget.
type FieldKind string

const (
	FieldString    FieldKind = "string"
	FieldText      FieldKind = "text" // multi-line
	FieldNumber    FieldKind = "number"
	FieldDate      FieldKind = "date"
	FieldBool      FieldKind = "bool"
	FieldStringArr FieldKind = "string_array"
	FieldObjectArr FieldKind = "object_array"
	FieldChartRef  FieldKind = "chart_ref" // a charts.id pointer
)

// Field describes one slot in a document's content JSON.
//
// ChartKind is meaningful only when Type is FieldChartRef: when set,
// the GUI restricts the chart picker to charts of that kind (e.g.
// "wbs" for wbs_ref, "raci" for raci_ref). Leave empty to allow any
// chart in the project.
type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldKind `json:"type"`
	Help        string    `json:"help,omitempty"`
	Required    bool      `json:"required,omitempty"`
	ObjectShape []Field   `json:"object_shape,omitempty"` // for FieldObjectArr
	ChartKind   string    `json:"chart_kind,omitempty"`   // for FieldChartRef
}

// Definition describes one document kind end-to-end.
type Definition struct {
	Kind        Kind    `json:"kind"`
	Name        string  `json:"name"`
	Phase       Phase   `json:"phase"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
}

// All returns a defensive copy of the master registry.
func All() []Definition {
	out := make([]Definition, len(registry))
	copy(out, registry)
	return out
}

// Get returns the Definition for a Kind, or (zero, false).
func Get(k Kind) (Definition, bool) {
	for _, d := range registry {
		if d.Kind == k {
			return d, true
		}
	}
	return Definition{}, false
}

// ByPhase returns every Definition whose Phase matches.
func ByPhase(p Phase) []Definition {
	var out []Definition
	for _, d := range registry {
		if d.Phase == p {
			out = append(out, d)
		}
	}
	return out
}
