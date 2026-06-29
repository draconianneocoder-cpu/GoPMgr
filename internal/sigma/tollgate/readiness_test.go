// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package tollgate

import (
	"strings"
	"testing"

	"pmforge/internal/sigma/domain"
)

// ----- Define phase -----

func TestCheckDefineReadiness_EmptyCharter(t *testing.T) {
	res := CheckDefineReadiness(domain.Charter{}, nil, nil)
	if res.CanAdvance {
		t.Error("empty charter should not be able to advance")
	}
	if res.Score != 0 {
		t.Errorf("empty charter: expected score 0, got %.1f", res.Score)
	}
	if len(res.Checks) == 0 {
		t.Error("expected non-empty checks list")
	}
	// All checks must be failing.
	for _, c := range res.Checks {
		if c.Passed {
			t.Errorf("check %q should fail with empty input", c.Name)
		}
	}
}

func TestCheckDefineReadiness_ShortTexts(t *testing.T) {
	// Texts that are too short should fail even when non-empty.
	c := domain.Charter{
		ProblemStatement: "Too short.", // 10 chars < 20 minimum
		BusinessCase:     "Short.",     // 6 chars < 10 minimum
		GoalStatement:    "Short.",     // 6 chars < 10 minimum
		Sponsor:          "AB",         // 2 chars < 3 minimum
		ScopeIn:          []string{},   // empty
	}
	res := CheckDefineReadiness(c, nil, nil)
	for _, check := range res.Checks {
		if check.Passed {
			t.Errorf("check %q: expected fail with short/empty values", check.Name)
		}
	}
}

func TestCheckDefineReadiness_AllPassing(t *testing.T) {
	charter := domain.Charter{
		ProblemStatement: "Assembly line produces 15% defective units in the welding step.",
		BusinessCase:     "Reducing defects by 10% saves $200k annually.",
		GoalStatement:    "Reduce weld defect rate from 15% to under 5% in 6 months.",
		Sponsor:          "Jane Smith",
		ScopeIn:          []string{"Welding station A"},
	}
	sipoc := &domain.SIPOCData{
		Elements: []domain.SIPOCElement{
			{Category: "supplier"}, {Category: "input"}, {Category: "process"},
			{Category: "output"}, {Category: "customer"},
		},
	}
	voc := &domain.VoCData{
		Entries: []domain.VoCEntry{
			{CTQ: "Weld strength", LowerSpec: 10, UpperSpec: 20},
		},
	}
	res := CheckDefineReadiness(charter, sipoc, voc)
	if !res.CanAdvance {
		t.Errorf("all passing: CanAdvance=false, score=%.1f, missing=%q", res.Score, res.MissingList)
	}
	if res.Score < 80 {
		t.Errorf("expected score >= 80, got %.1f", res.Score)
	}
}

func TestCheckDefineReadiness_CanAdvanceThresholdIs80Pct(t *testing.T) {
	// 7 checks total. 5/7 = 71.4% → cannot advance. 6/7 = 85.7% → can advance.

	// Pass 5 of 7: ProblemStatement, BusinessCase, GoalStatement, Sponsor, ScopeIn.
	// Fail: SIPOC (nil), CTQs (nil VoC).
	charter := domain.Charter{
		ProblemStatement: "Assembly line produces 15% defective units in the welding step.",
		BusinessCase:     "Reducing defects by 10% saves $200k annually.",
		GoalStatement:    "Reduce weld defect rate from 15% to under 5% in 6 months.",
		Sponsor:          "Jane Smith",
		ScopeIn:          []string{"Welding station A"},
	}
	res5 := CheckDefineReadiness(charter, nil, nil)
	if res5.CanAdvance {
		t.Errorf("5/7 checks (score=%.1f) should NOT allow advance", res5.Score)
	}

	// Pass 6 of 7: add SIPOC. Still no CTQs.
	sipoc := &domain.SIPOCData{
		Elements: []domain.SIPOCElement{
			{}, {}, {}, {}, {},
		},
	}
	res6 := CheckDefineReadiness(charter, sipoc, nil)
	if !res6.CanAdvance {
		t.Errorf("6/7 checks (score=%.1f) SHOULD allow advance", res6.Score)
	}
}

func TestCheckDefineReadiness_CTQsRequireSpecLimits(t *testing.T) {
	// CTQ entry with a trimmed non-empty CTQ but zero both spec limits fails the check.
	charter := domain.Charter{
		ProblemStatement: "Assembly line produces 15% defective units in the welding step.",
		BusinessCase:     "Reducing defects by 10% saves $200k annually.",
		GoalStatement:    "Reduce weld defect rate from 15% to under 5% in 6 months.",
		Sponsor:          "Jane Smith",
		ScopeIn:          []string{"Step A"},
	}
	vocNoLimits := &domain.VoCData{
		Entries: []domain.VoCEntry{
			{CTQ: "Weld strength", LowerSpec: 0, UpperSpec: 0},
		},
	}
	res := CheckDefineReadiness(charter, nil, vocNoLimits)

	for _, c := range res.Checks {
		if c.Name == "CTQs Defined" && c.Passed {
			t.Error("CTQ with zero spec limits should fail the CTQs Defined check")
		}
	}
}

func TestCheckDefineReadiness_MissingListNamesFailingChecks(t *testing.T) {
	res := CheckDefineReadiness(domain.Charter{}, nil, nil)
	if res.MissingList == "" {
		t.Error("expected non-empty MissingList for all-fail result")
	}
	// Every failing check name should appear in the missing list.
	for _, c := range res.Checks {
		if !c.Passed && !strings.Contains(res.MissingList, c.Name) {
			t.Errorf("check %q is failing but not in MissingList %q", c.Name, res.MissingList)
		}
	}
}

// ----- Analyze phase -----

func TestCheckAnalyzeReadiness_NilFishbone(t *testing.T) {
	res := CheckAnalyzeReadiness(nil)
	if res.CanAdvance {
		t.Error("nil fishbone should not advance")
	}
	if res.Score != 0 {
		t.Errorf("expected score 0, got %.1f", res.Score)
	}
}

func TestCheckAnalyzeReadiness_CausesButNoFiveWhys(t *testing.T) {
	fb := &domain.FishboneData{
		Branches: []domain.FishboneBranch{
			{Causes: []domain.Cause{{Description: "Root cause A"}}},
		},
	}
	res := CheckAnalyzeReadiness(fb)
	if res.CanAdvance {
		t.Error("causes without 5-Whys should not advance (requires 100%)")
	}
	// Fishbone Diagram check must pass; 5 Whys must fail.
	for _, c := range res.Checks {
		switch c.Name {
		case "Fishbone Diagram":
			if !c.Passed {
				t.Error("Fishbone Diagram check should pass when causes exist")
			}
		case "5 Whys Drill-Down":
			if c.Passed {
				t.Error("5 Whys check should fail when no cause has >= 3 levels")
			}
		}
	}
}

func TestCheckAnalyzeReadiness_AllPassing(t *testing.T) {
	fb := &domain.FishboneData{
		Branches: []domain.FishboneBranch{
			{Causes: []domain.Cause{
				{
					Description: "Over-pressure",
					FiveWhys:    []string{"Why 1", "Why 2", "Why 3"},
				},
			}},
		},
	}
	res := CheckAnalyzeReadiness(fb)
	if !res.CanAdvance {
		t.Errorf("analyze: CanAdvance=false, score=%.1f, missing=%q", res.Score, res.MissingList)
	}
	if res.Score != 100 {
		t.Errorf("expected 100%%, got %.1f", res.Score)
	}
}

func TestCheckAnalyzeReadiness_RequiresThreeWhys(t *testing.T) {
	// Only 2 whys — falls short of the >= 3 minimum.
	fb := &domain.FishboneData{
		Branches: []domain.FishboneBranch{
			{Causes: []domain.Cause{
				{Description: "Root cause", FiveWhys: []string{"Why 1", "Why 2"}},
			}},
		},
	}
	res := CheckAnalyzeReadiness(fb)
	for _, c := range res.Checks {
		if c.Name == "5 Whys Drill-Down" && c.Passed {
			t.Error("2 whys should not satisfy the 3-level minimum")
		}
	}
}

// ----- Improve phase -----

func TestCheckImproveReadiness_EmptySolutions(t *testing.T) {
	res := CheckImproveReadiness(nil)
	if res.CanAdvance {
		t.Error("no solutions should not advance")
	}
}

func TestCheckImproveReadiness_OneSolutionFails(t *testing.T) {
	sols := []domain.Solution{{ID: "s1", Impact: 8, Effort: 3, Selected: true}}
	res := CheckImproveReadiness(sols)
	if res.CanAdvance {
		t.Error("only 1 solution should not advance (need >= 2)")
	}
}

func TestCheckImproveReadiness_TwoSolutionsNotSelected(t *testing.T) {
	sols := []domain.Solution{
		{ID: "s1", Impact: 8, Effort: 3},
		{ID: "s2", Impact: 5, Effort: 5},
	}
	res := CheckImproveReadiness(sols)
	if res.CanAdvance {
		t.Error("two solutions but none selected should not advance")
	}
	for _, c := range res.Checks {
		if c.Name == "Solution Selected" && c.Passed {
			t.Error("Solution Selected check should fail when none are selected")
		}
	}
}

func TestCheckImproveReadiness_AllPassing(t *testing.T) {
	sols := []domain.Solution{
		{ID: "s1", Impact: 8, Effort: 3, Selected: true},
		{ID: "s2", Impact: 5, Effort: 5, Selected: false},
	}
	res := CheckImproveReadiness(sols)
	if !res.CanAdvance {
		t.Errorf("improve: CanAdvance=false, score=%.1f, missing=%q", res.Score, res.MissingList)
	}
	if res.Score != 100 {
		t.Errorf("expected 100%%, got %.1f", res.Score)
	}
}

func TestCheckImproveReadiness_MissingImpactEffortFails(t *testing.T) {
	// Two solutions, one selected, but neither has impact/effort scored.
	sols := []domain.Solution{
		{ID: "s1", Selected: true},
		{ID: "s2"},
	}
	res := CheckImproveReadiness(sols)
	if res.CanAdvance {
		t.Error("missing impact/effort scores should not advance")
	}
}

// ----- Control phase -----

func TestCheckControlReadiness_EmptyItems(t *testing.T) {
	res := CheckControlReadiness(nil)
	if res.CanAdvance {
		t.Error("no control items should not advance")
	}
}

func TestCheckControlReadiness_ItemNoOwner(t *testing.T) {
	items := []domain.ControlPlanItem{
		{ID: "c1", ResponsePlan: "Halt production"},
	}
	res := CheckControlReadiness(items)
	if res.CanAdvance {
		t.Error("item without owner should not advance")
	}
}

func TestCheckControlReadiness_ItemNoResponsePlan(t *testing.T) {
	items := []domain.ControlPlanItem{
		{ID: "c1", Owner: "Process engineer"},
	}
	res := CheckControlReadiness(items)
	if res.CanAdvance {
		t.Error("item without response plan should not advance")
	}
}

func TestCheckControlReadiness_AllPassing(t *testing.T) {
	items := []domain.ControlPlanItem{
		{ID: "c1", Owner: "Process engineer", ResponsePlan: "Halt production and notify QA"},
	}
	res := CheckControlReadiness(items)
	if !res.CanAdvance {
		t.Errorf("control: CanAdvance=false, score=%.1f, missing=%q", res.Score, res.MissingList)
	}
	if res.Score != 100 {
		t.Errorf("expected 100%%, got %.1f", res.Score)
	}
}

// ----- CheckPhase router -----

func TestCheckPhase_RoutesDefine(t *testing.T) {
	res := CheckPhase(domain.PhaseDefine, domain.Charter{}, nil, nil, nil, nil, nil)
	// Empty charter fails all Define checks — score should be 0.
	if res.Score != 0 {
		t.Errorf("expected 0 from empty define, got %.1f", res.Score)
	}
}

func TestCheckPhase_RoutesAnalyze(t *testing.T) {
	res := CheckPhase(domain.PhaseAnalyze, domain.Charter{}, nil, nil, nil, nil, nil)
	if res.CanAdvance {
		t.Error("nil fishbone routed through CheckPhase should not advance")
	}
}

func TestCheckPhase_RoutesImprove(t *testing.T) {
	res := CheckPhase(domain.PhaseImprove, domain.Charter{}, nil, nil, nil, nil, nil)
	if res.CanAdvance {
		t.Error("nil solutions routed through CheckPhase should not advance")
	}
}

func TestCheckPhase_RoutesControl(t *testing.T) {
	res := CheckPhase(domain.PhaseControl, domain.Charter{}, nil, nil, nil, nil, nil)
	if res.CanAdvance {
		t.Error("nil control items routed through CheckPhase should not advance")
	}
}

func TestCheckPhase_MeasureAutoApproved(t *testing.T) {
	// PhaseMe asure falls through to the default arm, which auto-approves.
	res := CheckPhase(domain.PhaseMeasure, domain.Charter{}, nil, nil, nil, nil, nil)
	if !res.CanAdvance {
		t.Error("Measure phase default should auto-approve")
	}
	if res.Score != 100 {
		t.Errorf("Measure auto-approve: expected score 100, got %.1f", res.Score)
	}
}
