// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

// registry is the master list of all 25 PMForge document types.
//
// Editing notes
//
//   - Add a new document by appending to this slice AND adding a Kind
//     constant in registry.go.
//   - Project Charter (charter_word) is the reference implementation:
//     its schema is the most fleshed-out of the 25. Use it as the
//     template when filling out the others.
//   - Fields whose Type is FieldChartRef render in the GUI as a chart
//     picker; clicking the field opens the relevant chart editor.
//   - Two pairs (plan_word/plan_excel, charter_word/charter_excel) are
//     intentionally separate kinds so the user-facing menu matches the
//     conventional Word/Excel distinction. The content schema is the
//     same; only the default export format differs.
var registry = []Definition{
	// ============================================================
	// INITIATION
	// ============================================================
	{
		Kind:        KindProjectCharterWord,
		Name:        "Project Charter (Word)",
		Phase:       PhaseInitiation,
		Description: "Foundational document that formally authorises the project. Captures purpose, objectives, scope, stakeholders, high-level schedule and budget.",
		Fields: []Field{
			{Key: "project_name", Label: "Project Name", Type: FieldString, Required: true},
			{Key: "sponsor", Label: "Project Sponsor", Type: FieldString, Required: true},
			{Key: "project_manager", Label: "Project Manager", Type: FieldString, Required: true},
			{Key: "charter_date", Label: "Charter Date", Type: FieldDate, Required: true},
			{Key: "purpose", Label: "Purpose / Business Need", Type: FieldText, Required: true, Help: "Why is this project being undertaken?"},
			{Key: "objectives", Label: "Objectives", Type: FieldStringArr, Help: "SMART objectives the project will achieve."},
			{Key: "scope_in", Label: "In Scope", Type: FieldStringArr},
			{Key: "scope_out", Label: "Out of Scope", Type: FieldStringArr},
			{Key: "deliverables", Label: "Deliverables", Type: FieldStringArr},
			{Key: "stakeholders", Label: "Key Stakeholders", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "name", Label: "Name", Type: FieldString},
				{Key: "role", Label: "Role", Type: FieldString},
				{Key: "interest", Label: "Interest / Influence", Type: FieldString},
			}},
			{Key: "high_level_schedule", Label: "High-Level Schedule", Type: FieldText, Help: "Major milestones and target dates."},
			{Key: "milestones", Label: "Milestones", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "name", Label: "Milestone", Type: FieldString},
				{Key: "date", Label: "Target Date", Type: FieldDate},
			}},
			{Key: "high_level_budget", Label: "High-Level Budget (USD)", Type: FieldNumber},
			{Key: "assumptions", Label: "Assumptions", Type: FieldStringArr},
			{Key: "constraints", Label: "Constraints", Type: FieldStringArr},
			{Key: "risks", Label: "Initial Risks", Type: FieldStringArr},
			{Key: "success_criteria", Label: "Success Criteria", Type: FieldStringArr},
			{Key: "authorisation", Label: "Authorising Signatures", Type: FieldText, Help: "Names, titles, and dates of approvers."},
		},
	},
	{
		Kind:        KindProjectCharterExcel,
		Name:        "Project Charter (Excel)",
		Phase:       PhaseInitiation,
		Description: "Spreadsheet-oriented project charter. Same content as the Word charter; default export format is XLSX.",
		Fields:      []Field{ /* mirrors KindProjectCharterWord at runtime */ },
	},
	{
		Kind:        KindBusinessCase,
		Name:        "Business Case",
		Phase:       PhaseInitiation,
		Description: "Justifies the project's value to stakeholders by laying out costs, benefits, risks, and alternatives.",
		Fields: []Field{
			{Key: "project_name", Label: "Project Name", Type: FieldString, Required: true},
			{Key: "problem_statement", Label: "Problem / Opportunity", Type: FieldText, Required: true},
			{Key: "proposed_solution", Label: "Proposed Solution", Type: FieldText, Required: true},
			{Key: "alternatives", Label: "Alternatives Considered", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "name", Label: "Alternative", Type: FieldString},
				{Key: "pros", Label: "Pros", Type: FieldText},
				{Key: "cons", Label: "Cons", Type: FieldText},
			}},
			{Key: "benefits", Label: "Expected Benefits", Type: FieldStringArr},
			{Key: "costs_summary", Label: "Cost Summary (USD)", Type: FieldNumber},
			{Key: "roi", Label: "ROI / Payback Period", Type: FieldText},
			{Key: "risks", Label: "Key Risks", Type: FieldStringArr},
			{Key: "recommendation", Label: "Recommendation", Type: FieldText},
		},
	},
	{
		Kind:        KindProjectProposal,
		Name:        "Project Proposal",
		Phase:       PhaseInitiation,
		Description: "Persuasive overview used to win stakeholder buy-in. Less formal than the Charter.",
		Fields: []Field{
			{Key: "project_name", Label: "Project Name", Type: FieldString, Required: true},
			{Key: "executive_summary", Label: "Executive Summary", Type: FieldText, Required: true},
			{Key: "goals", Label: "Goals", Type: FieldStringArr},
			{Key: "approach", Label: "Approach", Type: FieldText},
			{Key: "team", Label: "Proposed Team", Type: FieldStringArr},
			{Key: "timeline", Label: "Timeline Summary", Type: FieldText},
			{Key: "budget_summary", Label: "Budget Summary (USD)", Type: FieldNumber},
			{Key: "ask", Label: "What We're Asking For", Type: FieldText},
		},
	},
	{
		Kind:        KindStakeholderAnalysis,
		Name:        "Stakeholder Analysis Document",
		Phase:       PhaseInitiation,
		Description: "Narrative companion to the Stakeholder Analysis matrix. Documents engagement strategy per stakeholder.",
		Fields: []Field{
			{Key: "matrix_ref", Label: "Linked Matrix", Type: FieldChartRef, ChartKind: "stakeholder_analysis", Help: "Pick the Stakeholder Analysis chart this doc summarises."},
			{Key: "stakeholders", Label: "Stakeholders", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "name", Label: "Name", Type: FieldString},
				{Key: "role", Label: "Role", Type: FieldString},
				{Key: "interest_level", Label: "Interest", Type: FieldString},
				{Key: "influence_level", Label: "Influence", Type: FieldString},
				{Key: "engagement_strategy", Label: "Engagement Strategy", Type: FieldText},
			}},
		},
	},

	// ============================================================
	// PLANNING
	// ============================================================
	{
		Kind:        KindProjectPlanWord,
		Name:        "Project Plan (Word)",
		Phase:       PhasePlanning,
		Description: "Most comprehensive of all PM documents. Compiles scope, schedule, budget, and other planning artefacts.",
		Fields: []Field{
			{Key: "project_name", Label: "Project Name", Type: FieldString, Required: true},
			{Key: "executive_summary", Label: "Executive Summary", Type: FieldText},
			{Key: "scope_ref", Label: "Linked Scope Statement", Type: FieldString, Help: "ID of the Scope Statement doc."},
			{Key: "schedule_ref", Label: "Linked Schedule Chart", Type: FieldChartRef, ChartKind: "cpm"},
			{Key: "wbs_ref", Label: "Linked WBS", Type: FieldChartRef, ChartKind: "wbs"},
			{Key: "budget_ref", Label: "Linked Budget Doc ID", Type: FieldString},
			{Key: "risks_ref", Label: "Linked Risk Register ID", Type: FieldString},
			{Key: "raci_ref", Label: "Linked RACI Chart", Type: FieldChartRef, ChartKind: "raci"},
			{Key: "communication_plan_ref", Label: "Linked Communication Plan ID", Type: FieldString},
			{Key: "narrative_sections", Label: "Narrative Sections", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "heading", Label: "Heading", Type: FieldString},
				{Key: "body", Label: "Body", Type: FieldText},
			}},
		},
	},
	{
		Kind:        KindProjectPlanExcel,
		Name:        "Project Plan (Excel)",
		Phase:       PhasePlanning,
		Description: "Spreadsheet-oriented project plan. Same content as the Word plan; default export is XLSX.",
		Fields:      []Field{ /* mirrors KindProjectPlanWord at runtime */ },
	},
	{
		Kind:        KindProjectSchedule,
		Name:        "Project Schedule",
		Phase:       PhasePlanning,
		Description: "Timeline of every project task with dependencies, resources, and the critical path.",
		Fields: []Field{
			{Key: "schedule_ref", Label: "Linked Schedule Chart", Type: FieldChartRef, ChartKind: "cpm", Required: true},
			{Key: "baseline_date", Label: "Baseline Date", Type: FieldDate},
			{Key: "notes", Label: "Notes", Type: FieldText},
		},
	},
	{
		Kind:        KindWBSDocument,
		Name:        "Work Breakdown Structure",
		Phase:       PhasePlanning,
		Description: "Narrative around the WBS chart: deliverable definitions, work-package owners, and acceptance criteria.",
		Fields: []Field{
			{Key: "wbs_ref", Label: "Linked WBS Chart", Type: FieldChartRef, ChartKind: "wbs", Required: true},
			{Key: "deliverable_descriptions", Label: "Deliverable Descriptions", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "wbs_code", Label: "WBS Code", Type: FieldString},
				{Key: "description", Label: "Description", Type: FieldText},
				{Key: "acceptance_criteria", Label: "Acceptance Criteria", Type: FieldText},
			}},
		},
	},
	{
		Kind:        KindRACIDocument,
		Name:        "RACI Chart Document",
		Phase:       PhasePlanning,
		Description: "Narrative around the RACI matrix: role definitions and effective dates.",
		Fields: []Field{
			{Key: "raci_ref", Label: "Linked RACI Chart", Type: FieldChartRef, ChartKind: "raci", Required: true},
			{Key: "role_definitions", Label: "Role Definitions", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "role", Label: "Role", Type: FieldString},
				{Key: "definition", Label: "Definition", Type: FieldText},
			}},
			{Key: "effective_date", Label: "Effective Date", Type: FieldDate},
		},
	},
	{
		Kind:        KindRiskRegister,
		Name:        "Risk Register",
		Phase:       PhasePlanning,
		Description: "Catalogue of potential risks with probability, impact, owner, and mitigation strategy.",
		Fields: []Field{
			{Key: "risks", Label: "Risks", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "id", Label: "Risk ID", Type: FieldString},
				{Key: "description", Label: "Description", Type: FieldText},
				{Key: "probability", Label: "Probability (1-5)", Type: FieldNumber},
				{Key: "impact", Label: "Impact (1-5)", Type: FieldNumber},
				{Key: "owner", Label: "Owner", Type: FieldString},
				{Key: "mitigation", Label: "Mitigation", Type: FieldText},
				{Key: "status", Label: "Status", Type: FieldString},
			}},
		},
	},
	{
		Kind:        KindScopeStatement,
		Name:        "Scope Statement",
		Phase:       PhasePlanning,
		Description: "Defines exactly what the project will and will not deliver.",
		Fields: []Field{
			{Key: "project_name", Label: "Project Name", Type: FieldString, Required: true},
			{Key: "scope_description", Label: "Scope Description", Type: FieldText, Required: true},
			{Key: "deliverables", Label: "Deliverables", Type: FieldStringArr},
			{Key: "acceptance_criteria", Label: "Acceptance Criteria", Type: FieldStringArr},
			{Key: "exclusions", Label: "Exclusions", Type: FieldStringArr},
			{Key: "constraints", Label: "Constraints", Type: FieldStringArr},
			{Key: "assumptions", Label: "Assumptions", Type: FieldStringArr},
		},
	},
	{
		Kind:        KindProjectBudget,
		Name:        "Project Budget",
		Phase:       PhasePlanning,
		Description: "Estimated costs broken down by category (labor, materials, equipment, ...).",
		Fields: []Field{
			{Key: "currency", Label: "Currency", Type: FieldString},
			{Key: "categories", Label: "Cost Categories", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "category", Label: "Category", Type: FieldString},
				{Key: "amount", Label: "Amount", Type: FieldNumber},
				{Key: "notes", Label: "Notes", Type: FieldText},
			}},
			{Key: "contingency_pct", Label: "Contingency %", Type: FieldNumber},
			{Key: "total", Label: "Total", Type: FieldNumber},
		},
	},
	{
		Kind:        KindCommunicationPlan,
		Name:        "Communication Plan",
		Phase:       PhasePlanning,
		Description: "Who needs what information, in what format, on what cadence.",
		Fields: []Field{
			{Key: "channels", Label: "Communication Channels", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "channel", Label: "Channel", Type: FieldString},
				{Key: "audience", Label: "Audience", Type: FieldString},
				{Key: "purpose", Label: "Purpose", Type: FieldText},
				{Key: "frequency", Label: "Frequency", Type: FieldString},
				{Key: "owner", Label: "Owner", Type: FieldString},
			}},
		},
	},
	{
		Kind:        KindExecutionPlan,
		Name:        "Project Execution Plan",
		Phase:       PhasePlanning,
		Description: "How the project will actually be delivered: tasks, timeline, resources, costs.",
		Fields: []Field{
			{Key: "tasks", Label: "Tasks", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "name", Label: "Task", Type: FieldString},
				{Key: "owner", Label: "Owner", Type: FieldString},
				{Key: "start_date", Label: "Start", Type: FieldDate},
				{Key: "end_date", Label: "End", Type: FieldDate},
				{Key: "resources", Label: "Resources", Type: FieldText},
				{Key: "cost", Label: "Cost", Type: FieldNumber},
			}},
		},
	},
	{
		Kind:        KindStatementOfWork,
		Name:        "Statement of Work",
		Phase:       PhasePlanning,
		Description: "Formal definition of scope, deliverables, timeline, and responsibilities before work begins.",
		Fields: []Field{
			{Key: "background", Label: "Background", Type: FieldText},
			{Key: "objectives", Label: "Objectives", Type: FieldStringArr},
			{Key: "scope", Label: "Scope of Work", Type: FieldText},
			{Key: "deliverables", Label: "Deliverables", Type: FieldStringArr},
			{Key: "schedule", Label: "Schedule", Type: FieldText},
			{Key: "responsibilities", Label: "Responsibilities", Type: FieldText},
			{Key: "acceptance_criteria", Label: "Acceptance Criteria", Type: FieldStringArr},
		},
	},
	{
		Kind:        KindProcurementPlan,
		Name:        "Procurement Plan",
		Phase:       PhasePlanning,
		Description: "What will be procured, how, from whom, and on what schedule.",
		Fields: []Field{
			{Key: "items", Label: "Procurement Items", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "item", Label: "Item", Type: FieldString},
				{Key: "contract_type", Label: "Contract Type", Type: FieldString},
				{Key: "vendor_selection_criteria", Label: "Selection Criteria", Type: FieldText},
				{Key: "target_award_date", Label: "Target Award Date", Type: FieldDate},
				{Key: "budget", Label: "Budget", Type: FieldNumber},
			}},
		},
	},
	{
		Kind:        KindRequirements,
		Name:        "Requirements Document",
		Phase:       PhasePlanning,
		Description: "Specifications the project must meet to satisfy stakeholders.",
		Fields: []Field{
			{Key: "requirements", Label: "Requirements", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "id", Label: "Req ID", Type: FieldString},
				{Key: "description", Label: "Description", Type: FieldText},
				{Key: "type", Label: "Type", Type: FieldString, Help: "functional / non-functional / business / technical"},
				{Key: "priority", Label: "Priority", Type: FieldString},
				{Key: "source", Label: "Source / Stakeholder", Type: FieldString},
			}},
		},
	},
	{
		Kind:        KindTeamCharter,
		Name:        "Team Charter",
		Phase:       PhasePlanning,
		Description: "Roles, responsibilities, deliverables, and resources of the project team.",
		Fields: []Field{
			{Key: "team_purpose", Label: "Team Purpose", Type: FieldText},
			{Key: "members", Label: "Members", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "name", Label: "Name", Type: FieldString},
				{Key: "role", Label: "Role", Type: FieldString},
				{Key: "responsibilities", Label: "Responsibilities", Type: FieldText},
				{Key: "allocation_pct", Label: "Allocation %", Type: FieldNumber},
			}},
			{Key: "ground_rules", Label: "Ground Rules", Type: FieldStringArr},
		},
	},

	// ============================================================
	// EXECUTION
	// ============================================================
	{
		Kind:        KindProjectBrief,
		Name:        "Project Brief",
		Phase:       PhaseExecution,
		Description: "Short, audience-oriented summary of the plan for non-PM stakeholders.",
		Fields: []Field{
			{Key: "summary", Label: "Summary", Type: FieldText, Required: true},
			{Key: "goals", Label: "Goals", Type: FieldStringArr},
			{Key: "roles", Label: "Roles", Type: FieldStringArr},
			{Key: "budget", Label: "Budget", Type: FieldNumber},
			{Key: "timeline", Label: "Timeline", Type: FieldText},
		},
	},
	{
		Kind:        KindProjectOverview,
		Name:        "Project Overview",
		Phase:       PhaseExecution,
		Description: "1-page snapshot: timeline, milestones, budget, status, and key roles.",
		Fields: []Field{
			{Key: "status", Label: "Status", Type: FieldString},
			{Key: "milestones_summary", Label: "Milestones", Type: FieldText},
			{Key: "budget_summary", Label: "Budget", Type: FieldText},
			{Key: "team_summary", Label: "Team", Type: FieldText},
			{Key: "highlights", Label: "Highlights", Type: FieldStringArr},
		},
	},

	// ============================================================
	// MONITORING & CONTROL
	// ============================================================
	{
		Kind:        KindStatusReport,
		Name:        "Status Report",
		Phase:       PhaseMonitoring,
		Description: "Periodic check-in: progress, risks, blockers, and upcoming work.",
		Fields: []Field{
			{Key: "report_date", Label: "Report Date", Type: FieldDate, Required: true},
			{Key: "schedule_ref", Label: "Linked Schedule Chart", Type: FieldChartRef, ChartKind: "cpm"},
			{Key: "overall_status", Label: "Overall Status", Type: FieldString, Help: "green / yellow / red"},
			{Key: "accomplishments", Label: "Accomplishments", Type: FieldStringArr},
			{Key: "in_progress", Label: "In Progress", Type: FieldStringArr},
			{Key: "blockers", Label: "Blockers", Type: FieldStringArr},
			{Key: "upcoming", Label: "Upcoming", Type: FieldStringArr},
			{Key: "schedule_status", Label: "Schedule", Type: FieldString},
			{Key: "budget_status", Label: "Budget", Type: FieldString},
		},
	},
	{
		Kind:        KindIssueLog,
		Name:        "Issue Log",
		Phase:       PhaseMonitoring,
		Description: "Tracks problems that have actually occurred (vs. risks, which haven't yet).",
		Fields: []Field{
			{Key: "issues", Label: "Issues", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "id", Label: "Issue ID", Type: FieldString},
				{Key: "description", Label: "Description", Type: FieldText},
				{Key: "raised_by", Label: "Raised By", Type: FieldString},
				{Key: "raised_at", Label: "Raised", Type: FieldDate},
				{Key: "severity", Label: "Severity", Type: FieldString},
				{Key: "owner", Label: "Owner", Type: FieldString},
				{Key: "status", Label: "Status", Type: FieldString},
				{Key: "resolution", Label: "Resolution", Type: FieldText},
			}},
		},
	},
	{
		Kind:        KindChangeRequest,
		Name:        "Change Request Form",
		Phase:       PhaseMonitoring,
		Description: "Formally proposes a change to scope, budget, schedule, or other project aspects.",
		Fields: []Field{
			{Key: "request_id", Label: "Request ID", Type: FieldString, Required: true},
			{Key: "requested_by", Label: "Requested By", Type: FieldString, Required: true},
			{Key: "requested_at", Label: "Date", Type: FieldDate, Required: true},
			{Key: "description", Label: "Change Description", Type: FieldText, Required: true},
			{Key: "reason", Label: "Reason", Type: FieldText},
			{Key: "scope_impact", Label: "Scope Impact", Type: FieldText},
			{Key: "schedule_impact", Label: "Schedule Impact", Type: FieldText},
			{Key: "cost_impact", Label: "Cost Impact", Type: FieldNumber},
			{Key: "risk_impact", Label: "Risk Impact", Type: FieldText},
			{Key: "decision", Label: "Decision", Type: FieldString, Help: "approved / rejected / deferred"},
			{Key: "decision_rationale", Label: "Rationale", Type: FieldText},
			{Key: "approver", Label: "Approver", Type: FieldString},
		},
	},

	// ============================================================
	// CLOSING
	// ============================================================
	{
		Kind:        KindProjectClosure,
		Name:        "Project Closure",
		Phase:       PhaseClosing,
		Description: "Formal end-of-project record: deliverables accepted, contracts closed, lessons learned.",
		Fields: []Field{
			{Key: "closure_date", Label: "Closure Date", Type: FieldDate, Required: true},
			{Key: "objectives_met", Label: "Objectives Met", Type: FieldText},
			{Key: "deliverables_accepted", Label: "Deliverables Accepted", Type: FieldStringArr},
			{Key: "outstanding_items", Label: "Outstanding Items", Type: FieldStringArr},
			{Key: "contracts_closed", Label: "Contracts Closed", Type: FieldStringArr},
			{Key: "lessons_learned", Label: "Lessons Learned", Type: FieldObjectArr, ObjectShape: []Field{
				{Key: "category", Label: "Category", Type: FieldString},
				{Key: "lesson", Label: "Lesson", Type: FieldText},
				{Key: "recommendation", Label: "Recommendation", Type: FieldText},
			}},
			{Key: "final_budget_actual", Label: "Final Budget (Actual)", Type: FieldNumber},
			{Key: "stakeholder_signoff", Label: "Stakeholder Sign-Off", Type: FieldText},
		},
	},
}
