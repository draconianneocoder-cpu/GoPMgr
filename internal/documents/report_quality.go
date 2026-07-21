// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "sort"

// ReportMode controls how preflight findings affect an export. Draft reports
// remain useful working documents; certified reports require a clean preflight.
type ReportMode string

const (
	ReportModeDraft      ReportMode = "draft"
	ReportModeManagement ReportMode = "management"
	ReportModeCertified  ReportMode = "certified"
)

// ReportProfile maps a project context to a practical document and chart
// baseline. Profiles are guidance, not a claim of certification against a
// referenced standard; users may always select a custom profile.
type ReportProfile struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Industry                 string   `json:"industry"`
	Reference                string   `json:"reference,omitempty"`
	RequiredDocumentKinds    []Kind   `json:"required_document_kinds"`
	RecommendedDocumentKinds []Kind   `json:"recommended_document_kinds"`
	RecommendedChartKinds    []string `json:"recommended_chart_kinds"`
	Customizable             bool     `json:"customizable"`
}

// ReportIssue is a visible, exportable quality finding. EntityID is a document
// or chart identifier when the issue is tied to a specific input.
type ReportIssue struct {
	Severity string `json:"severity"` // error, warning, information
	Code     string `json:"code"`
	Message  string `json:"message"`
	EntityID string `json:"entity_id,omitempty"`
}

// ReportPreflight is returned to the UI before an export and is also embedded
// in the PDF and provenance manifest, preventing silent report degradation.
type ReportPreflight struct {
	Profile ReportProfile `json:"profile"`
	Mode    ReportMode    `json:"mode"`
	Issues  []ReportIssue `json:"issues"`
	Ready   bool          `json:"ready"`
}

// ReportInput is the small persistence-independent shape needed by preflight.
type ReportInput struct {
	ID     string
	Kind   Kind
	Status string
}

var reportProfiles = []ReportProfile{
	{
		ID: "general", Name: "Project controls", Industry: "general",
		Reference:                "ISO 21502:2020 project management guidance",
		RequiredDocumentKinds:    []Kind{KindProjectCharterWord, KindProjectSchedule, KindRiskRegister, KindStatusReport},
		RecommendedDocumentKinds: []Kind{KindWBSDocument, KindRACIDocument, KindProjectBudget, KindStakeholderAnalysis},
		RecommendedChartKinds:    []string{"gantt", "wbs", "raci", "stakeholder_analysis"}, Customizable: true,
	},
	{
		ID: "construction", Name: "Construction information controls", Industry: "construction",
		Reference:                "ISO 19650 information-management framework",
		RequiredDocumentKinds:    []Kind{KindProjectCharterWord, KindProjectSchedule, KindRiskRegister, KindStatusReport},
		RecommendedDocumentKinds: []Kind{KindScopeStatement, KindProcurementPlan, KindProjectBudget, KindStakeholderAnalysis},
		RecommendedChartKinds:    []string{"gantt", "cpm", "wbs", "raci"}, Customizable: true,
	},
	{
		ID: "software", Name: "Software delivery controls", Industry: "software",
		Reference:                "ISO/IEC/IEEE 12207:2026 software life-cycle processes",
		RequiredDocumentKinds:    []Kind{KindProjectCharterWord, KindRequirements, KindProjectSchedule, KindStatusReport},
		RecommendedDocumentKinds: []Kind{KindRiskRegister, KindStakeholderAnalysis, KindCommunicationPlan, KindChangeRequest},
		RecommendedChartKinds:    []string{"gantt", "burndown", "cumulative_flow", "raci"}, Customizable: true,
	},
	{
		ID: "custom", Name: "Custom selection", Industry: "custom",
		Reference:    "User-selected document and chart set",
		Customizable: true,
	},
}

// ReportProfiles returns copies so callers cannot change the built-in policy.
func ReportProfiles() []ReportProfile {
	out := make([]ReportProfile, len(reportProfiles))
	copy(out, reportProfiles)
	return out
}

// ReportProfileFor selects a profile explicitly, or derives the best baseline
// from a project's industry. Unknown choices deliberately fall back to the
// broadly applicable project-controls profile.
func ReportProfileFor(id, industry string) ReportProfile {
	if id != "" {
		for _, profile := range reportProfiles {
			if profile.ID == id {
				return profile
			}
		}
	}
	for _, profile := range reportProfiles {
		if profile.Industry == industry {
			return profile
		}
	}
	return reportProfiles[0]
}

// Preflight checks profile completeness and readiness without knowing about a
// database. Missing chart references are added by the application layer.
func Preflight(profile ReportProfile, mode ReportMode, inputs []ReportInput) ReportPreflight {
	if mode == "" {
		mode = ReportModeDraft
	}
	byKind := make(map[Kind][]ReportInput)
	for _, input := range inputs {
		byKind[input.Kind] = append(byKind[input.Kind], input)
	}
	issues := make([]ReportIssue, 0)
	for _, kind := range profile.RequiredDocumentKinds {
		if len(byKind[kind]) == 0 {
			issues = append(issues, ReportIssue{Severity: "error", Code: "required_document_missing", Message: "Required profile document is not included: " + string(kind)})
		}
	}
	for _, kind := range profile.RecommendedDocumentKinds {
		if len(byKind[kind]) == 0 {
			issues = append(issues, ReportIssue{Severity: "warning", Code: "recommended_document_missing", Message: "Recommended profile document is not included: " + string(kind)})
		}
	}
	if mode == ReportModeCertified {
		for _, input := range inputs {
			if input.Status != "approved" {
				issues = append(issues, ReportIssue{Severity: "error", Code: "document_not_approved", Message: "Certified reports require approved document versions.", EntityID: input.ID})
			}
		}
	} else if mode == ReportModeManagement {
		for _, input := range inputs {
			if input.Status == "draft" {
				issues = append(issues, ReportIssue{Severity: "warning", Code: "draft_document", Message: "Management report includes a draft document.", EntityID: input.ID})
			}
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].Severity < issues[j].Severity
	})
	ready := true
	if mode == ReportModeCertified {
		for _, issue := range issues {
			if issue.Severity == "error" {
				ready = false
				break
			}
		}
	}
	return ReportPreflight{Profile: profile, Mode: mode, Issues: issues, Ready: ready}
}
