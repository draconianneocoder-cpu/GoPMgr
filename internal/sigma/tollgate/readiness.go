// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package tollgate

import (
	"strings"

	"pmforge/internal/sigma/domain"
)

// Check represents a single tollgate requirement.
type Check struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// Result holds the overall readiness score and individual checks.
type Result struct {
	Score       float64 `json:"score"`
	CanAdvance  bool    `json:"can_advance"`
	Checks      []Check `json:"checks"`
	MissingList string  `json:"missing_list"`
}

// CheckDefineReadiness evaluates the Define phase charter completeness.
func CheckDefineReadiness(c domain.Charter, sipoc *domain.SIPOCData, voc *domain.VoCData) Result {
	hasCTQs := false
	if voc != nil && len(voc.Entries) > 0 {
		for _, entry := range voc.Entries {
			if len(strings.TrimSpace(entry.CTQ)) > 0 && (entry.LowerSpec != 0 || entry.UpperSpec != 0) {
				hasCTQs = true
				break
			}
		}
	}

	checks := []Check{
		{
			Name:    "Problem Statement",
			Passed:  len(strings.TrimSpace(c.ProblemStatement)) >= 20,
			Message: "Must clearly describe what, where, when, and magnitude (min 20 chars).",
		},
		{
			Name:    "Business Case",
			Passed:  len(strings.TrimSpace(c.BusinessCase)) >= 10,
			Message: "Explain why this matters to the organization.",
		},
		{
			Name:    "Goal Statement",
			Passed:  len(strings.TrimSpace(c.GoalStatement)) >= 10,
			Message: "Define target improvement with measurable outcome.",
		},
		{
			Name:    "Sponsor Assigned",
			Passed:  len(strings.TrimSpace(c.Sponsor)) >= 3,
			Message: "A sponsor must be identified.",
		},
		{
			Name:    "Scope Defined",
			Passed:  len(c.ScopeIn) > 0,
			Message: "At least one 'In Scope' item required.",
		},
		{
			Name:    "SIPOC Diagram",
			Passed:  sipoc != nil && len(sipoc.Elements) >= 5,
			Message: "Add at least one element to each SIPOC category (5 minimum).",
		},
		{
			Name:    "CTQs Defined",
			Passed:  hasCTQs,
			Message: "Define at least one CTQ with spec limits from Voice of Customer.",
		},
	}

	passed := 0
	var missing []string
	for i := range checks {
		if checks[i].Passed {
			passed++
		} else {
			missing = append(missing, checks[i].Name)
		}
	}

	score := float64(passed) / float64(len(checks)) * 100
	return Result{
		Score:       score,
		CanAdvance:  score >= 80,
		Checks:      checks,
		MissingList: strings.Join(missing, ", "),
	}
}

// CheckPhase routes to the correct phase checker.
func CheckPhase(phase domain.Phase, charter domain.Charter, sipoc *domain.SIPOCData, voc *domain.VoCData, fishbone *domain.FishboneData, solutions []domain.Solution, controlPlan []domain.ControlPlanItem) Result {
	switch phase {
	case domain.PhaseDefine:
		return CheckDefineReadiness(charter, sipoc, voc)
	case domain.PhaseAnalyze:
		return CheckAnalyzeReadiness(fishbone)
	case domain.PhaseImprove:
		return CheckImproveReadiness(solutions)
	case domain.PhaseControl:
		return CheckControlReadiness(controlPlan)
	default:
		return Result{Score: 100, CanAdvance: true, Checks: []Check{{Name: "Phase " + string(phase), Passed: true, Message: "Auto-approved in MVP 1"}}}
	}
}

// CheckAnalyzeReadiness ensures Pareto and Fishbone/5 Whys are used.
func CheckAnalyzeReadiness(fb *domain.FishboneData) Result {
	if fb == nil {
		return Result{Score: 0, CanAdvance: false, Checks: []Check{{Name: "Fishbone Diagram", Passed: false, Message: "Create a Fishbone diagram."}}}
	}

	hasCauses := false
	hasFiveWhys := false
	for _, b := range fb.Branches {
		if len(b.Causes) > 0 {
			hasCauses = true
			for _, c := range b.Causes {
				if len(c.FiveWhys) >= 3 {
					hasFiveWhys = true
				}
			}
		}
	}

	checks := []Check{
		{Name: "Fishbone Diagram", Passed: hasCauses, Message: "Add at least one cause to the diagram."},
		{Name: "5 Whys Drill-Down", Passed: hasFiveWhys, Message: "Complete at least one 5 Whys drill-down (3 levels)."},
	}

	passed := 0
	var missing []string
	for i := range checks {
		if checks[i].Passed {
			passed++
		} else {
			missing = append(missing, checks[i].Name)
		}
	}

	score := float64(passed) / float64(len(checks)) * 100
	return Result{
		Score:       score,
		CanAdvance:  score >= 100,
		Checks:      checks,
		MissingList: strings.Join(missing, ", "),
	}
}

// CheckImproveReadiness ensures solutions are evaluated and at least one is selected.
func CheckImproveReadiness(solutions []domain.Solution) Result {
	if len(solutions) == 0 {
		return Result{Score: 0, CanAdvance: false, Checks: []Check{{Name: "Solution Matrix", Passed: false, Message: "Add potential solutions to the matrix."}}}
	}

	hasSelected := false
	hasImpactEffort := false
	for _, s := range solutions {
		if s.Selected {
			hasSelected = true
		}
		if s.Impact > 0 && s.Effort > 0 {
			hasImpactEffort = true
		}
	}

	checks := []Check{
		{Name: "Solutions Added", Passed: len(solutions) >= 2, Message: "Add at least 2 potential solutions."},
		{Name: "Impact/Effort Scored", Passed: hasImpactEffort, Message: "Score solutions on impact and effort (1-10)."},
		{Name: "Solution Selected", Passed: hasSelected, Message: "Select at least one solution for implementation."},
	}

	passed := 0
	var missing []string
	for i := range checks {
		if checks[i].Passed {
			passed++
		} else {
			missing = append(missing, checks[i].Name)
		}
	}

	score := float64(passed) / float64(len(checks)) * 100
	return Result{
		Score:       score,
		CanAdvance:  score >= 100,
		Checks:      checks,
		MissingList: strings.Join(missing, ", "),
	}
}

// CheckControlReadiness ensures control plan items have owners and response plans.
func CheckControlReadiness(items []domain.ControlPlanItem) Result {
	if len(items) == 0 {
		return Result{Score: 0, CanAdvance: false, Checks: []Check{{Name: "Control Plan", Passed: false, Message: "Add control plan items."}}}
	}

	hasOwner := false
	hasResponsePlan := false
	for _, item := range items {
		if len(strings.TrimSpace(item.Owner)) > 0 {
			hasOwner = true
		}
		if len(strings.TrimSpace(item.ResponsePlan)) > 0 {
			hasResponsePlan = true
		}
	}

	checks := []Check{
		{Name: "Control Items Added", Passed: len(items) >= 1, Message: "Add at least 1 control plan item."},
		{Name: "Owners Assigned", Passed: hasOwner, Message: "Assign an owner to each control item."},
		{Name: "Response Plans Defined", Passed: hasResponsePlan, Message: "Define a response plan for out-of-control conditions."},
	}

	passed := 0
	var missing []string
	for i := range checks {
		if checks[i].Passed {
			passed++
		} else {
			missing = append(missing, checks[i].Name)
		}
	}

	score := float64(passed) / float64(len(checks)) * 100
	return Result{
		Score:       score,
		CanAdvance:  score >= 100,
		Checks:      checks,
		MissingList: strings.Join(missing, ", "),
	}
}
