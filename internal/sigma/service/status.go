// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"pmforge/internal/sigma/domain"
)

// ToolStatus represents the completion state of a single tool.
type ToolStatus struct {
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Status string `json:"status"` // "completed", "active", "not_started"
}

// PhaseTools holds all tool statuses for a given phase.
type PhaseTools struct {
	Phase string       `json:"phase"`
	Tools []ToolStatus `json:"tools"`
}

// GetToolStatus returns the completion status of all tools for the current phase.
func (s *ProjectService) GetToolStatus(projectID string, phase string) PhaseTools {
	var tools []ToolStatus

	switch phase {
	case "define":
		charter, _ := s.GetCharter(projectID)
		sipoc, _ := s.GetSIPOC(projectID)

		charterStatus := "not_started"
		if charter != nil && len(charter.ProblemStatement) >= 20 {
			charterStatus = "completed"
		} else if charter != nil && len(charter.ProblemStatement) > 0 {
			charterStatus = "active"
		}

		sipocStatus := "not_started"
		if sipoc != nil && len(sipoc.Elements) >= 5 {
			sipocStatus = "completed"
		} else if sipoc != nil && len(sipoc.Elements) > 0 {
			sipocStatus = "active"
		}

		tools = []ToolStatus{
			{Name: "Project Charter", Icon: "📝", Status: charterStatus},
			{Name: "SIPOC Diagram", Icon: "🔄", Status: sipocStatus},
			{Name: "Voice of Customer", Icon: "🗣️", Status: "not_started"},
		}

	case "measure":
		tools = []ToolStatus{
			{Name: "Data Collection Plan", Icon: "📊", Status: "not_started"},
			{Name: "Descriptive Statistics", Icon: "📈", Status: "not_started"},
			{Name: "Process Capability", Icon: "📏", Status: "not_started"},
		}

	case "analyze":
		fb, _ := s.GetFishbone(projectID)

		fishboneStatus := "not_started"
		if fb != nil {
			hasCauses := false
			for _, b := range fb.Branches {
				if len(b.Causes) > 0 {
					hasCauses = true
					break
				}
			}
			if hasCauses {
				fishboneStatus = "completed"
			}
		}

		tools = []ToolStatus{
			{Name: "Pareto Chart", Icon: "📉", Status: "not_started"},
			{Name: "Fishbone Diagram", Icon: "🐟", Status: fishboneStatus},
			{Name: "5 Whys", Icon: "❓", Status: "not_started"},
		}

	case "improve":
		solutions, _ := s.GetSolutions(projectID)

		solutionStatus := "not_started"
		if len(solutions) > 0 {
			hasSelected := false
			for _, sol := range solutions {
				if sol.Selected {
					hasSelected = true
					break
				}
			}
			if hasSelected {
				solutionStatus = "completed"
			} else {
				solutionStatus = "active"
			}
		}

		tools = []ToolStatus{
			{Name: "Solution Matrix", Icon: "✅", Status: solutionStatus},
			{Name: "Pilot Plan", Icon: "🧪", Status: "not_started"},
		}

	case "control":
		controlPlan, _ := s.GetControlPlan(projectID)

		cpStatus := "not_started"
		if len(controlPlan) > 0 {
			hasOwner := false
			for _, item := range controlPlan {
				if len(item.Owner) > 0 {
					hasOwner = true
					break
				}
			}
			if hasOwner {
				cpStatus = "completed"
			} else {
				cpStatus = "active"
			}
		}

		tools = []ToolStatus{
			{Name: "Control Plan", Icon: "🛡️", Status: cpStatus},
			{Name: "SOP Builder", Icon: "📖", Status: "not_started"},
		}

	default:
		tools = []ToolStatus{}
	}

	return PhaseTools{Phase: phase, Tools: tools}
}

// GetProjectReportData assembles all phase data for export.
func (s *ProjectService) GetProjectReportData(projectID string) (domain.Project, *domain.Charter, *domain.SIPOCData, *domain.FishboneData, []domain.Solution, []domain.ControlPlanItem, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return domain.Project{}, nil, nil, nil, nil, nil, err
	}

	charter, _ := s.GetCharter(projectID)
	sipoc, _ := s.GetSIPOC(projectID)
	fishbone, _ := s.GetFishbone(projectID)
	solutions, _ := s.GetSolutions(projectID)
	controlPlan, _ := s.GetControlPlan(projectID)

	return *project, charter, sipoc, fishbone, solutions, controlPlan, nil
}
