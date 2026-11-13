// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package schedaudit

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/kernel"
)

// The expected findings in this file are worked out by hand from the PS-02
// specification and the formulas in kernel.CalculateCPM's comment. No test
// calls the audit or the scheduler to produce its own expectation.
//
// Calendar used throughout: Monday-to-Friday, project start Monday
// 2026-01-05, so working-day index 0 is 5 Jan, 4 is Fri 9 Jan, 5 is Mon 12
// Jan, 7 is Wed 14 Jan, 10 is Mon 19 Jan.

const projectStart = "2026-01-05"

func weekdays(d time.Time) bool {
	return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday
}

func tk(id string, dur float64, preds ...string) kernel.Task {
	return kernel.Task{ID: id, Title: "Task " + id, Duration: dur, Precedents: preds,
		Assignments: []kernel.Assignment{{Resource: "res-" + id}}}
}

func milestone(id string, preds ...string) kernel.Task {
	return kernel.Task{ID: id, Title: "Milestone " + id, Milestone: true, Precedents: preds}
}

// base is a fully dated input: a project start, a reporting date and a
// baseline, so that only the fixture's own defects produce findings.
func base(tasks ...kernel.Task) Input {
	return Input{
		Tasks:             tasks,
		ProjectStart:      projectStart,
		IsWorkday:         weekdays,
		ReportingDate:     "2026-01-14",
		BaselineAvailable: true,
		Source:            Source{ChartID: "chart-1", Revision: "rev-7"},
		Profile:           DefaultProfile(),
	}
}

func run(t *testing.T, in Input) Result {
	t.Helper()
	r, err := Audit(context.Background(), in)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	return r
}

func ofRule(r Result, rule RuleID) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

// subjects names each finding's task, or its link as "pred->succ".
func subjects(fs []Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		if f.Link != nil {
			out = append(out, f.Link.Pred+"->"+f.Link.Succ)
			continue
		}
		out = append(out, f.TaskID)
	}
	return out
}

func wantSubjects(t *testing.T, r Result, rule RuleID, want ...string) {
	t.Helper()
	got := subjects(ofRule(r, rule))
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s findings = %q, want %q", rule, got, want)
	}
}

func status(t *testing.T, r Result, rule RuleID) RuleStatus {
	t.Helper()
	for _, s := range r.Rules {
		if s.Rule == rule {
			return s
		}
	}
	t.Fatalf("no status for %s", rule)
	return RuleStatus{}
}

func metric(t *testing.T, r Result, id string) Metric {
	t.Helper()
	for _, m := range r.Metrics {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no metric %s in %+v", id, r.Metrics)
	return Metric{}
}

// cleanSchedule has no defect under the default profile: a start milestone,
// a three-task chain with consistent progress and resources that never
// overlap, and a finish milestone.
//
//	S (milestone, done 5 Jan) -> A (5d, done 5-9 Jan) -> B (3d, started 12 Jan, 50%)
//	-> C (4d, ES 8 = 15 Jan) -> F (milestone)
//
// With the reporting date 14 Jan (index 7), C is not yet due, so SQ-15 has
// nothing to report. Every float is 0.
func cleanSchedule() Input {
	s := milestone("S")
	s.PercentComplete, s.ActualStart, s.ActualFinish = 100, "2026-01-05", "2026-01-05"
	a := tk("A", 5, "S")
	a.Assignments = []kernel.Assignment{{Resource: "alice"}}
	a.PercentComplete, a.ActualStart, a.ActualFinish = 100, "2026-01-05", "2026-01-09"
	b := tk("B", 3, "A")
	b.Assignments = []kernel.Assignment{{Resource: "bob"}}
	b.PercentComplete, b.ActualStart = 50, "2026-01-12"
	c := tk("C", 4, "B")
	c.Assignments = []kernel.Assignment{{Resource: "alice"}}
	in := base(s, a, b, c, milestone("F", "C"))
	in.StartAnchors, in.FinishAnchors = []string{"S"}, []string{"F"}
	return in
}

// SQA-019
func TestCleanScheduleHasNoFindings(t *testing.T) {
	for _, p := range []Profile{DefaultProfile(), DCMAStyleProfile()} {
		in := cleanSchedule()
		in.Profile = p
		r := run(t, in)
		if len(r.Findings) != 0 {
			t.Fatalf("%s: clean schedule gave findings %+v", p.Name, r.Findings)
		}
		for _, s := range r.Rules {
			if !s.Evaluated {
				t.Fatalf("%s: %s not evaluated on a fully dated schedule: %s", p.Name, s.Rule, s.Reason)
			}
		}
		for _, m := range r.Metrics {
			if m.Pass != nil && !*m.Pass {
				t.Fatalf("%s: metric %s failed on the clean schedule: %+v", p.Name, m.ID, m)
			}
		}
	}
}

// SQA-001, SQA-002, SQA-027: two chains, A->B->C and D->E. CPM gives D an ES
// of 0, the same as A, so an exemption based on computed dates would excuse
// D. Only designated anchors are exempt.
func TestMissingLogicUsesDesignatedAnchorsOnly(t *testing.T) {
	tasks := []kernel.Task{tk("A", 1), tk("B", 1, "A"), tk("C", 1, "B"), tk("D", 1), tk("E", 1, "D")}

	in := base(tasks...)
	in.StartAnchors, in.FinishAnchors = []string{"A"}, []string{"E"}
	r := run(t, in)
	wantSubjects(t, r, MissingPredecessor, "D")
	wantSubjects(t, r, MissingSuccessor, "C")

	in = base(tasks...)
	r = run(t, in)
	wantSubjects(t, r, MissingPredecessor, "A", "D")
	wantSubjects(t, r, MissingSuccessor, "C", "E")
	f := ofRule(r, MissingPredecessor)[0]
	if f.Severity != SeverityWarning || f.TaskTitle != "Task A" || f.Explanation == "" || f.Advice == "" {
		t.Fatalf("SQ-01 finding incomplete: %+v", f)
	}

	// Four of the five tasks lack logic: 80 %, above the DCMA-style 5 %.
	in.Profile = DCMAStyleProfile()
	m := metric(t, run(t, in), LogicMetric)
	if m.Numerator != 4 || m.Denominator != 5 || m.Percent != 80 || m.Pass == nil || *m.Pass {
		t.Fatalf("logic metric = %+v, want 4/5 = 80 %% and a fail", m)
	}
}

// Milestones are exempt from SQ-01 and SQ-02 (SQ-13 covers them), and only
// the Milestone flag makes a milestone: a zero-duration task without it is
// an ordinary task.
func TestMilestoneFlagAloneExemptsFromLogicRules(t *testing.T) {
	r := run(t, base(milestone("M"), tk("Z", 0), tk("A", 2, "M", "Z"), milestone("N", "A")))
	wantSubjects(t, r, MissingPredecessor, "Z")
	wantSubjects(t, r, MissingSuccessor)
}

// SQA-003: a legacy precedent and a typed link naming tasks that do not
// exist. A dangling reference is not logic, so A and B also lack a
// predecessor.
func TestDanglingReferencesInBothLinkLists(t *testing.T) {
	a := tk("A", 1)
	a.Precedents = []string{"X"}
	b := tk("B", 1, "A")
	b.Links = []kernel.Link{{Pred: "Y", Type: kernel.FinishToStart}}
	r := run(t, base(a, b))
	wantSubjects(t, r, DanglingReference, "X->A", "Y->B")
	if f := ofRule(r, DanglingReference)[0]; f.Severity != SeverityError || f.Observed == "" {
		t.Fatalf("SQ-03 finding = %+v", f)
	}
	wantSubjects(t, r, MissingPredecessor, "A")
}

// SQA-004 and SQA-022: A->B->C->A is a cycle and E depends on itself. D
// feeds the cycle but is not in it. The scheduler refuses the network, so
// every rule that needs its results is reported as not evaluated.
func TestCycleMembersAndRulesNotEvaluated(t *testing.T) {
	a := tk("A", 1, "C", "D")
	e := tk("E", 1)
	e.Links = []kernel.Link{{Pred: "E"}}
	r := run(t, base(a, tk("B", 1, "A"), tk("C", 1, "B"), tk("D", 1), e))
	wantSubjects(t, r, DependencyCycle, "A", "B", "C", "E")
	if f := ofRule(r, DependencyCycle)[0]; f.Severity != SeverityError || !strings.Contains(f.Observed, "A, B, C") {
		t.Fatalf("SQ-04 finding should name the cycle's members: %+v", f)
	}
	for _, rule := range []RuleID{ConstraintViolated, HighFloat, NegativeFloat, MissingProgress, Overallocation} {
		s := status(t, r, rule)
		if s.Evaluated || !strings.Contains(s.Reason, "cycle") {
			t.Fatalf("%s status = %+v, want not evaluated because of the cycle", rule, s)
		}
		if len(ofRule(r, rule)) != 0 {
			t.Fatalf("%s reported findings although it was not evaluated", rule)
		}
	}
	if !status(t, r, DependencyCycle).Evaluated {
		t.Fatal("SQ-04 must still run when the scheduler refuses the network")
	}
}

// SQA-022: a schedule the scheduler refuses as too long. The kernel checks
// limits before it looks for cycles, so the cycle here is still found by
// the audit's own search, and the CPM rules give the refusal's reason.
func TestScheduleTooLongMarksRulesNotEvaluated(t *testing.T) {
	long := tk("L", kernel.MaxScheduleDays+1)
	r := run(t, base(long, tk("A", 1, "B"), tk("B", 1, "A")))
	wantSubjects(t, r, DependencyCycle, "A", "B")
	s := status(t, r, NegativeFloat)
	if s.Evaluated || !strings.Contains(s.Reason, "longer than") {
		t.Fatalf("SQ-11 status = %+v, want not evaluated because the schedule is too long", s)
	}
}

// SQA-005, SQA-006 and part of SQA-023.
func TestLeadsAndLongLags(t *testing.T) {
	b := tk("B", 1)
	b.Links = []kernel.Link{{Pred: "A", Lag: -2}}
	c := tk("C", 1)
	c.Links = []kernel.Link{{Pred: "B", Lag: 12}}
	d := tk("D", 1)
	d.Links = []kernel.Link{{Pred: "C", Lag: 10}}
	in := base(tk("A", 1), b, c, d)

	r := run(t, in)
	wantSubjects(t, r, Lead, "A->B")
	if f := ofRule(r, Lead)[0]; f.Link.Lag != -2 || f.Severity != SeverityWarning {
		t.Fatalf("SQ-05 finding = %+v", f)
	}
	// The default limit is 10 working days; exactly 10 is not long.
	wantSubjects(t, r, LongLag, "B->C")
	if f := ofRule(r, LongLag)[0]; f.Threshold != "more than 10 working days" {
		t.Fatalf("SQ-06 threshold = %q", f.Threshold)
	}

	// DCMA-style counts every positive lag; the lead is not a lag.
	in.Profile = DCMAStyleProfile()
	r = run(t, in)
	wantSubjects(t, r, LongLag, "B->C", "C->D")
	m := metric(t, r, string(LongLag))
	if m.Numerator != 2 || m.Denominator != 3 || m.Pass == nil || *m.Pass {
		t.Fatalf("lag metric = %+v, want 2 of 3 links and a fail", m)
	}
	if m := metric(t, r, string(Lead)); m.Numerator != 1 || *m.Pass {
		t.Fatalf("lead metric = %+v, want 1 and a fail", m)
	}
}

// SQA-007: the share of finish-to-start links. A link whose type the kernel
// does not recognise is scheduled as FS but is not counted as one, and it is
// reported.
func TestFinishToStartShare(t *testing.T) {
	b := tk("B", 1)
	b.Links = []kernel.Link{{Pred: "A", Type: kernel.StartToStart}}
	c := tk("C", 1)
	c.Links = []kernel.Link{{Pred: "B", Type: kernel.FinishToStart}}
	d := tk("D", 1)
	d.Links = []kernel.Link{{Pred: "C", Type: kernel.FinishToFinish}}
	e := tk("E", 1)
	e.Links = []kernel.Link{{Pred: "D", Type: "XX"}}
	in := base(tk("A", 1), b, c, d, e)

	r := run(t, in)
	wantSubjects(t, r, NonFinishToStart, "D->E")
	m := metric(t, r, string(NonFinishToStart))
	if m.Numerator != 1 || m.Denominator != 4 || m.Percent != 25 || m.Pass != nil {
		t.Fatalf("FS metric = %+v, want 1 of 4 = 25 %% with no limit", m)
	}

	in.Profile = DCMAStyleProfile()
	r = run(t, in)
	if m := metric(t, r, string(NonFinishToStart)); m.Pass == nil || *m.Pass {
		t.Fatalf("FS share 25 %% must fail an at-least-90 %% limit: %+v", m)
	}
	// The unrecognised link, then the failed share, which names no task.
	wantSubjects(t, r, NonFinishToStart, "", "D->E")
}

// SQA-008, SQA-009, SQA-011 and SQA-028: date constraints are armed before
// CPM, so their violations and the float they cause are reported.
//
//	A 5d (index 0-4)
//	B 3d after A: ES 5, EF 8; FNLT Tue 13 Jan = index 6, stored 7.
//	  EF 8 > 7, so it is violated. LF is capped at 7: LS 4, float -1.
//	D 1d after A: ES 5, EF 6; MFO Wed 7 Jan = index 2, stored 3.
//	  EF 6 > 3, violated. LF 3: LS 2, float -3.
//	A's LF is the smaller successor LS, 2: LS -3, float -3.
//	C 2d, SNET Mon 19 Jan = index 10: ES 10, EF 12, float 0. SNET is not
//	  a hard constraint.
func TestArmedConstraintsAndNegativeFloat(t *testing.T) {
	b := tk("B", 3, "A")
	b.Constraint, b.ConstraintDate = kernel.FinishNoLaterThan, "2026-01-13"
	c := tk("C", 2)
	c.Constraint, c.ConstraintDate = kernel.StartNoEarlierThan, "2026-01-19"
	d := tk("D", 1, "A")
	d.Constraint, d.ConstraintDate = kernel.MustFinishOn, "2026-01-07"
	tasks := []kernel.Task{tk("A", 5), b, c, d}

	r := run(t, base(tasks...))
	wantSubjects(t, r, HardConstraint, "B", "D")
	if f := ofRule(r, HardConstraint)[0]; !strings.Contains(f.Observed, "FNLT 2026-01-13") || !strings.Contains(f.Observed, "binds") {
		t.Fatalf("SQ-08 should report the constraint and that it binds: %q", f.Observed)
	}
	wantSubjects(t, r, ConstraintViolated, "B", "D")
	wantSubjects(t, r, NegativeFloat, "A", "B", "D")
	floats := map[string]string{}
	for _, f := range ofRule(r, NegativeFloat) {
		floats[f.TaskID] = f.Observed
	}
	want := map[string]string{"A": "float -3 working days", "B": "float -1 working days", "D": "float -3 working days"}
	if !reflect.DeepEqual(floats, want) {
		t.Fatalf("negative float observed = %v, want %v", floats, want)
	}

	// Without a project start date the constraints cannot be applied.
	in := base(tasks...)
	in.ProjectStart = ""
	r = run(t, in)
	for _, rule := range []RuleID{ConstraintViolated, HighFloat, NegativeFloat, MissingProgress} {
		if s := status(t, r, rule); s.Evaluated || !strings.Contains(s.Reason, "project start date") {
			t.Fatalf("%s status = %+v, want not evaluated for want of a start date", rule, s)
		}
		if len(ofRule(r, rule)) != 0 {
			t.Fatalf("%s reported findings although it was not evaluated", rule)
		}
	}
	wantSubjects(t, r, HardConstraint, "B", "D")
	if f := ofRule(r, HardConstraint)[0]; !strings.Contains(f.Observed, "not evaluated") {
		t.Fatalf("SQ-08 binding must read not evaluated without a start date: %q", f.Observed)
	}
}

// A date constraint whose date cannot be applied is not in force. The
// kernel leaves it dormant without saying so; the audit reports it.
func TestUnappliedConstraintDateIsReported(t *testing.T) {
	b := tk("B", 1, "A")
	b.Constraint, b.ConstraintDate = kernel.MustFinishOn, "2026-02-30"
	r := run(t, base(tk("A", 1), b))
	wantSubjects(t, r, ConstraintViolated, "B")
	if f := ofRule(r, ConstraintViolated)[0]; !strings.Contains(f.Observed, "2026-02-30") || !strings.Contains(f.Explanation, "not applied") {
		t.Fatalf("SQ-09 should explain the constraint was not applied: %+v", f)
	}
}

// SQA-010 and part of SQA-023: B has no logic, so its late finish is the
// project finish, 50: float 50 - 1 - 0 = 49.
func TestHighFloat(t *testing.T) {
	in := base(tk("A", 50), tk("B", 1))
	r := run(t, in)
	wantSubjects(t, r, HighFloat, "B")
	if f := ofRule(r, HighFloat)[0]; f.Observed != "float 49 working days" || f.Threshold != "more than 44 working days" {
		t.Fatalf("SQ-10 finding = %+v", f)
	}
	in.Profile.HighFloatDays = 49
	wantSubjects(t, run(t, in), HighFloat)
}

// SQA-012: A is longer than 44 days, B is exactly 44, and the milestone is
// excluded whatever its duration.
func TestLongDuration(t *testing.T) {
	m := milestone("M")
	m.Duration = 50
	in := base(tk("A", 45), tk("B", 44), m)
	wantSubjects(t, run(t, in), LongDuration, "A")
	in.Profile.LongDurationDays = 45
	wantSubjects(t, run(t, in), LongDuration)
}

// SQA-013
func TestMilestoneWithoutLogic(t *testing.T) {
	r := run(t, base(milestone("M"), tk("A", 1), milestone("N", "A")))
	wantSubjects(t, r, MilestoneWithout, "M")
}

// SQA-014: one defect per task, read from the values the caller stored.
// The scheduler clamps percent complete to 0..100, so the range check must
// read the copy taken before scheduling.
func TestInvalidActuals(t *testing.T) {
	withProgress := func(id string, pct float64, start, finish string) kernel.Task {
		x := tk(id, 1)
		x.PercentComplete, x.ActualStart, x.ActualFinish = pct, start, finish
		return x
	}
	tasks := []kernel.Task{
		withProgress("A", 10, "2026-01-20", ""),           // starts after the reporting date
		withProgress("B", 10, "2026-01-10", "2026-01-08"), // finishes before it starts
		withProgress("C", 0, "", "2026-01-08"),            // finishes without a start
		withProgress("D", 100, "2026-01-05", ""),          // complete without a finish
		withProgress("E", 30, "", ""),                     // progress without a start
		withProgress("F", 150, "2026-01-05", ""),          // percent out of range
		withProgress("G", 10, "2026-13-40", ""),           // unreadable date
		withProgress("H", 50, "2026-01-05", ""),           // valid
	}
	in := base(tasks...)
	r := run(t, in)
	wantSubjects(t, r, InvalidActuals, "A", "B", "C", "D", "E", "F", "G")
	// Each task is flagged for its own defect. F in particular must be
	// reported as out of range, not as "100% complete", which is what the
	// scheduler's clamped copy would say.
	want := map[string]string{
		"A": "actual start 2026-01-20 is after the reporting date 2026-01-14",
		"B": "actual finish 2026-01-08 is before actual start 2026-01-10",
		"C": "actual finish without an actual start",
		"D": "100% complete without an actual finish",
		"E": "30% complete without an actual start",
		"F": "percent complete 150 is outside 0 to 100",
		"G": `actual start "2026-13-40" is not a date`,
	}
	for _, f := range ofRule(r, InvalidActuals) {
		if f.Observed != want[f.TaskID] {
			t.Errorf("SQ-14 on %s observed %q, want %q", f.TaskID, f.Observed, want[f.TaskID])
		}
	}

	in.ReportingDate = ""
	r = run(t, in)
	wantSubjects(t, r, InvalidActuals, "B", "C", "D", "E", "F", "G")
	if s := status(t, r, InvalidActuals); !s.Evaluated || !strings.Contains(s.Reason, "reporting date") {
		t.Fatalf("SQ-14 status = %+v, want evaluated with a note about the reporting date", s)
	}
}

// SQA-015, including a reporting date on a Saturday: the kernel maps it to
// the next working day, Monday 12 Jan (index 5).
func TestMissingProgressEvidence(t *testing.T) {
	snet := func(id, date string) kernel.Task {
		x := tk(id, 1)
		x.Constraint, x.ConstraintDate = kernel.StartNoEarlierThan, date
		return x
	}
	b := tk("B", 2, "A")
	b.PercentComplete, b.ActualStart = 10, "2026-01-07"
	tasks := []kernel.Task{tk("A", 2), b, snet("C", "2026-01-14"), snet("D", "2026-01-13")}

	r := run(t, base(tasks...))
	wantSubjects(t, r, MissingProgress, "A", "D")
	if f := ofRule(r, MissingProgress)[0]; !strings.Contains(f.Observed, "2026-01-14") {
		t.Fatalf("SQ-15 must state the reporting date it used: %q", f.Observed)
	}

	in := base(tk("A", 2), snet("E", "2026-01-09"), snet("F", "2026-01-12"))
	in.ReportingDate = "2026-01-10"
	wantSubjects(t, run(t, in), MissingProgress, "A", "E")

	// A reporting date the kernel cannot place on the calendar (more than
	// 100,000 working days out) is not evaluated rather than guessed.
	in.ReportingDate = "2500-01-01"
	if s := status(t, run(t, in), MissingProgress); s.Evaluated || !strings.Contains(s.Reason, "too far") {
		t.Fatalf("SQ-15 status = %+v, want not evaluated for a date beyond the calendar", s)
	}

	in.ReportingDate = ""
	if s := status(t, run(t, in), MissingProgress); s.Evaluated || !strings.Contains(s.Reason, "reporting date") {
		t.Fatalf("SQ-15 status = %+v, want not evaluated without a reporting date", s)
	}
}

// SQA-016: A and B both use alice on days 0-2. A stored Overallocated flag
// is never trusted: C carries one and uses no one.
func TestOverallocationUsesTheCapacityPlan(t *testing.T) {
	a, b := tk("A", 3), tk("B", 3)
	a.Assignments = []kernel.Assignment{{Resource: "alice"}}
	b.Assignments = []kernel.Assignment{{Resource: "alice"}}
	c := tk("C", 1)
	c.Overallocated = true
	in := base(a, b, c)
	wantSubjects(t, run(t, in), Overallocation, "A", "B")

	in.Capacity = kernel.ResourceCapacityPlan{Capacities: map[string]float64{"alice": 2}}
	wantSubjects(t, run(t, in), Overallocation)
}

// SQA-017
func TestUnresourcedTask(t *testing.T) {
	a := tk("A", 1)
	a.Assignments = nil
	r := run(t, base(a, tk("B", 1, "A"), milestone("M", "B")))
	wantSubjects(t, r, UnresourcedTask, "A")
}

// SQA-018
func TestReportingContext(t *testing.T) {
	in := base(tk("A", 1))
	in.ProjectStart, in.ReportingDate, in.BaselineAvailable = "", "", false
	in.StartAnchors = []string{"ZZ"}
	r := run(t, in)
	fs := ofRule(r, ReportingContext)
	if len(fs) != 4 {
		t.Fatalf("SQ-18 findings = %+v, want four: start, reporting date, baseline, anchor", fs)
	}
	text := ""
	for _, f := range fs {
		if f.Severity != SeverityInfo {
			t.Fatalf("SQ-18 must be information: %+v", f)
		}
		text += f.Observed + "\n"
	}
	for _, want := range []string{"project start date", "reporting date", "baseline", "ZZ"} {
		if !strings.Contains(text, want) {
			t.Fatalf("SQ-18 findings do not mention %q:\n%s", want, text)
		}
	}

	in = base(tk("A", 1))
	in.ProjectStart, in.ReportingDate = "2026-02-30", "14/01/2026"
	fs = ofRule(run(t, in), ReportingContext)
	if len(fs) != 2 || !strings.Contains(fs[0].Observed+fs[1].Observed, "unreadable") {
		t.Fatalf("unreadable dates must be reported as such: %+v", fs)
	}
}

// SQA-023: each threshold moves exactly the findings it governs.
func TestProfilesMoveOnlyTheirFindings(t *testing.T) {
	b := tk("B", 1)
	b.Links = []kernel.Link{{Pred: "A", Lag: 3}}
	in := base(tk("A", 45), b, tk("C", 1))
	before := run(t, in)

	in.Profile.LongLagDays = 2
	after := run(t, in)
	if got := subjects(ofRule(after, LongLag)); !reflect.DeepEqual(got, []string{"A->B"}) {
		t.Fatalf("lowering the lag limit: SQ-06 = %q", got)
	}
	if len(after.Findings) != len(before.Findings)+1 {
		t.Fatalf("lowering the lag limit changed other findings: %d -> %d", len(before.Findings), len(after.Findings))
	}
}

func TestInvalidInputIsRefused(t *testing.T) {
	cases := map[string]Input{
		"empty ID":     base(tk("", 1)),
		"duplicate ID": base(tk("A", 1), tk("A", 2)),
		"no profile":   func() Input { in := base(tk("A", 1)); in.Profile = Profile{}; return in }(),
		"negative lag limit": func() Input {
			in := base(tk("A", 1))
			in.Profile.LongLagDays = -1
			return in
		}(),
	}
	for name, in := range cases {
		if _, err := Audit(context.Background(), in); err == nil {
			t.Errorf("%s: Audit accepted invalid input", name)
		}
	}
	in := base(tk("A", 1))
	in.Profile.Limits = map[string]Limit{LogicMetric: {Percent: 101}}
	if _, err := Audit(context.Background(), in); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("a limit above 100 %% must be refused, got %v", err)
	}
	in = base(tk("A", 1))
	in.Profile.LongLagDays = -1
	if _, err := Audit(context.Background(), in); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("an invalid profile must wrap ErrInvalidProfile, got %v", err)
	}
}

// Each finding carries the source chart, the engine version and the
// profile; findings come out ordered by severity, rule, task and link.
func TestFindingsCarrySourceAndAreOrdered(t *testing.T) {
	b := tk("B", 1)
	b.Links = []kernel.Link{{Pred: "A", Lag: -1}}
	a := tk("A", 1)
	a.Assignments = nil
	r := run(t, base(a, b, tk("C", 1, "Q")))
	if len(r.Findings) == 0 {
		t.Fatal("fixture should produce findings")
	}
	for _, f := range r.Findings {
		if f.Source != (Source{ChartID: "chart-1", Revision: "rev-7"}) || f.EngineVersion != EngineVersion || f.Profile != "GoPMgr default" {
			t.Fatalf("finding lacks its source: %+v", f)
		}
	}
	sorted := sort.SliceIsSorted(r.Findings, func(i, j int) bool {
		x, y := r.Findings[i], r.Findings[j]
		if x.Severity.rank() != y.Severity.rank() {
			return x.Severity.rank() < y.Severity.rank()
		}
		return x.Rule < y.Rule
	})
	if !sorted || r.Findings[0].Severity != SeverityError {
		t.Fatalf("findings are not ordered by severity then rule: %+v", r.Findings)
	}
	if len(r.Rules) != len(AllRules()) {
		t.Fatalf("Rules lists %d statuses, want one per rule", len(r.Rules))
	}
}

// SQA-020: the caller's tasks are unchanged. The snapshot is JSON, which is
// independent of cloneTask.
func TestAuditDoesNotChangeItsInput(t *testing.T) {
	in := cleanSchedule()
	in.Tasks[2].Overallocated = true
	in.Tasks[2].Links = []kernel.Link{{Pred: "A", Type: "XX", Lag: 2}}
	in.Tasks[3].PercentComplete = 150
	before, err := json.Marshal(in.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	run(t, in)
	after, _ := json.Marshal(in.Tasks)
	if string(before) != string(after) {
		t.Fatalf("Audit changed its input:\nbefore %s\nafter  %s", before, after)
	}
}

// cloneTask must share no memory with its source. Every slice in the copy
// is overwritten and the source must be untouched.
func TestCloneTaskSharesNothing(t *testing.T) {
	src := kernel.Task{
		ID: "A", Precedents: []string{"P"}, Links: []kernel.Link{{Pred: "Q"}},
		Assignments: []kernel.Assignment{{Resource: "r", SkillTags: []string{"s"}}},
		WorkDays:    []int{1}, PlannedWorkSegments: []kernel.WorkSegment{{Start: 0, End: 1}},
	}
	keep, _ := json.Marshal(src)
	c := cloneTask(src)
	if !reflect.DeepEqual(c, src) {
		t.Fatal("clone differs from its source")
	}
	c.Precedents[0], c.Links[0].Pred, c.WorkDays[0] = "x", "x", 9
	c.Assignments[0].Resource, c.Assignments[0].SkillTags[0] = "x", "x"
	c.PlannedWorkSegments[0].End = 9
	if got, _ := json.Marshal(src); string(got) != string(keep) {
		t.Fatalf("writing to the clone changed the source: %s", got)
	}
	empty := kernel.Task{Precedents: []string{}}
	if c := cloneTask(empty); c.Precedents == nil || c.Links != nil {
		t.Fatal("clone must keep empty slices empty and nil slices nil")
	}
}

// If kernel.Task or a type inside it gains a slice, map or pointer, this
// test fails until cloneTask copies it and the list below names it.
func TestCloneTaskCoversEveryReferenceField(t *testing.T) {
	known := map[string]bool{
		"Task.Precedents": true, "Task.Links": true, "Task.Assignments": true,
		"Task.WorkDays": true, "Task.PlannedWorkSegments": true, "Assignment.SkillTags": true,
	}
	var walk func(reflect.Type)
	seen := map[reflect.Type]bool{}
	walk = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			ft := f.Type
			switch ft.Kind() {
			case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan:
				if !known[typ.Name()+"."+f.Name] {
					t.Errorf("%s.%s is a reference field that cloneTask does not copy", typ.Name(), f.Name)
				}
				if ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct {
					walk(ft.Elem())
				}
			case reflect.Struct:
				walk(ft)
			}
		}
	}
	walk(reflect.TypeOf(kernel.Task{}))
}

// SQA-021: the same input gives byte-identical output, whatever the task
// order, including from concurrent calls under -race.
func TestDeterministic(t *testing.T) {
	in := base(tk("A", 50), tk("B", 1), tk("C", 1, "Q"), tk("D", 1, "D"))
	want, _ := json.Marshal(run(t, in))

	rev := base()
	for i := len(in.Tasks) - 1; i >= 0; i-- {
		rev.Tasks = append(rev.Tasks, in.Tasks[i])
	}
	if got, _ := json.Marshal(run(t, rev)); string(got) != string(want) {
		t.Fatalf("task order changed the output:\n%s\n%s", got, want)
	}

	errs := make(chan string, 8)
	for i := 0; i < 8; i++ {
		go func() {
			r, err := Audit(context.Background(), in)
			got, _ := json.Marshal(r)
			if err != nil || string(got) != string(want) {
				errs <- string(got)
				return
			}
			errs <- ""
		}()
	}
	for i := 0; i < 8; i++ {
		if got := <-errs; got != "" {
			t.Fatalf("concurrent audit differed: %s", got)
		}
	}
}

// SQA-025
func TestCancelledAuditStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Audit(ctx, base(tk("A", 1))); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled audit returned %v, want context.Canceled", err)
	}
}

// SQA-026: no output under either profile claims certification or
// conformance.
func TestNoCertificationClaim(t *testing.T) {
	inputs := []Input{
		cleanSchedule(),
		base(tk("A", 50), tk("B", 1), tk("C", 1, "Q"), tk("D", 1, "D")),
		func() Input { in := base(tk("A", 1)); in.ProjectStart, in.ReportingDate = "", ""; return in }(),
	}
	for _, p := range []Profile{DefaultProfile(), DCMAStyleProfile()} {
		for _, in := range inputs {
			in.Profile = p
			out, _ := json.Marshal(run(t, in))
			low := strings.ToLower(string(out))
			for _, banned := range []string{"certif", "complian", "conform"} {
				if strings.Contains(low, banned) {
					t.Fatalf("%s output contains %q: %s", p.Name, banned, out)
				}
			}
		}
	}
}

// A 50,000-task chain closed into one cycle: the cycle search must handle
// it without deep recursion.
func TestLargeCycle(t *testing.T) {
	const n = 50000
	tasks := make([]kernel.Task, n)
	for i := range tasks {
		tasks[i] = kernel.Task{ID: taskID(i), Duration: 1, Assignments: []kernel.Assignment{{Resource: "r"}}}
		if i > 0 {
			tasks[i].Precedents = []string{taskID(i - 1)}
		}
	}
	tasks[0].Precedents = []string{taskID(n - 1)}
	r := run(t, base(tasks...))
	if got := len(ofRule(r, DependencyCycle)); got != n {
		t.Fatalf("SQ-04 found %d members, want %d", got, n)
	}
}

func taskID(i int) string {
	const digits = "0123456789"
	b := []byte("T00000")
	for p := len(b) - 1; p > 0; p-- {
		b[p] = digits[i%10]
		i /= 10
	}
	return string(b)
}
