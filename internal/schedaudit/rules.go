// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package schedaudit

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"gopmgr/internal/kernel"
)

// epsilon absorbs floating-point noise in the scheduler's computed values
// (float and early start). Durations and lags the user entered are compared
// exactly.
const epsilon = 1e-9

// maxNamed is how many cycle members a finding names before "and N more".
const maxNamed = 10

func percent(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + "%"
}

// number shows a value for reading, rounded to two decimals, so that the
// scheduler's floating-point noise (50.199999999999996) is not shown.
// Comparisons use the unrounded value.
func number(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

func set(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// SQ-01 and SQ-02. Only links to tasks that exist count as logic; a
// dangling one is reported by SQ-03.
func (a *auditor) checkLogic() error {
	start, finish := set(a.in.StartAnchors), set(a.in.FinishAnchors)
	flagged := map[string]bool{}
	for _, id := range a.ids {
		if err := a.tick(); err != nil {
			return err
		}
		if a.raw[id].Milestone {
			continue
		}
		if a.indeg[id] == 0 && !start[id] {
			flagged[id] = true
			a.add(MissingPredecessor, SeverityWarning, id, nil, "no predecessor", "",
				"Nothing drives this task's start, so a delay elsewhere cannot move it and its dates can look better than they are.",
				"Link it to the work it depends on, or designate it as the schedule's start.")
		}
		if a.outdeg[id] == 0 && !finish[id] {
			flagged[id] = true
			a.add(MissingSuccessor, SeverityWarning, id, nil, "no successor", "",
				"Nothing depends on this task, so a delay to it does not move the finish and it shows float it does not have.",
				"Link it to the work that needs it, or designate it as the schedule's finish.")
		}
	}
	a.ran(MissingPredecessor, "")
	a.ran(MissingSuccessor, "")
	a.measure(LogicMetric, len(flagged), a.nonMilestones())
	return nil
}

// SQ-03
func (a *auditor) checkStray() error {
	for _, e := range a.stray {
		a.add(DanglingReference, SeverityError, "", e.ref(),
			fmt.Sprintf("links to %q, which is not in this schedule", e.pred), "",
			"The scheduler skips a link to a task that does not exist, so this dependency has no effect.",
			"Remove the link, or link to the task that was meant.")
	}
	a.ran(DanglingReference, "")
	a.measure(string(DanglingReference), len(a.stray), len(a.links)+len(a.stray))
	return nil
}

// SQ-04
func (a *auditor) checkCycles() error {
	comps, err := a.cycles()
	if err != nil {
		return err
	}
	members := 0
	for _, comp := range comps {
		observed := "in a cycle with " + nameList(comp)
		if len(comp) == 1 {
			observed = "depends on itself"
		}
		for _, id := range comp {
			a.add(DependencyCycle, SeverityError, id, nil, observed, "",
				"Tasks that depend on each other in a loop cannot be scheduled. While the loop exists the scheduler computes no dates for any task.",
				"Remove or reverse one of the links in the loop.")
		}
		members += len(comp)
	}
	a.ran(DependencyCycle, "")
	a.measure(string(DependencyCycle), members, len(a.ids))
	return nil
}

func nameList(ids []string) string {
	if len(ids) <= maxNamed {
		return strings.Join(ids, ", ")
	}
	return strings.Join(ids[:maxNamed], ", ") + fmt.Sprintf(" and %d more", len(ids)-maxNamed)
}

// SQ-05 and SQ-06
func (a *auditor) checkLags() error {
	leads, long := 0, 0
	for _, e := range a.links {
		if e.lag < 0 {
			leads++
			a.add(Lead, SeverityWarning, "", e.ref(), "lag "+days(e.lag), "",
				"A lead lets the successor start before its predecessor's logic allows, which hides how the work really overlaps.",
				"State the real overlap instead, for example with a start-to-start link, or split the predecessor.")
		}
		if e.lag > a.prof.LongLagDays {
			long++
			a.add(LongLag, SeverityInfo, "", e.ref(), "lag "+days(e.lag), moreThan(a.prof.LongLagDays),
				"A lag is waiting time that is not shown as work, so it cannot be tracked or given progress.",
				"Replace the lag with a task that describes the wait, such as curing or an approval.")
		}
	}
	a.ran(Lead, "")
	a.ran(LongLag, "")
	a.measure(string(Lead), leads, len(a.links))
	a.measure(string(LongLag), long, len(a.links))
	return nil
}

// SQ-07. The kernel schedules a link of an unrecognised type as FS, so it
// appears as FS among the effective links; it is not counted as one here.
func (a *auditor) checkLinkTypes() error {
	fs := 0
	for _, e := range a.links {
		if e.typ == kernel.FinishToStart {
			fs++
		}
	}
	fs -= len(a.unknownType)
	for _, e := range a.unknownType {
		a.add(NonFinishToStart, SeverityInfo, "", e.ref(),
			fmt.Sprintf("link type %q is not recognised; it is scheduled as finish-to-start", e.typ), "",
			"The scheduler treats a link type it does not recognise as finish-to-start, which may not be what was meant.",
			"Set the link type to FS, SS, FF or SF.")
	}
	m := a.measure(string(NonFinishToStart), fs, len(a.links))
	if m.Pass != nil && !*m.Pass {
		a.add(NonFinishToStart, SeverityInfo, "", nil,
			fmt.Sprintf("%s of %d links are finish-to-start", percent(m.Percent), m.Denominator),
			"at least "+percent(m.Limit.Percent),
			"Other link types are harder to read and to give progress, and they can hide the true critical path.",
			"Use finish-to-start links unless the work genuinely overlaps.")
	}
	a.ran(NonFinishToStart, "")
	return nil
}

func dated(c kernel.ConstraintType) bool {
	return c == kernel.StartNoEarlierThan || c == kernel.FinishNoLaterThan || c == kernel.MustFinishOn
}

// supported reports whether the kernel acts on a constraint type: it arms
// the dated ones and honours ALAP. Anything else is scheduled as ASAP.
func supported(c kernel.ConstraintType) bool {
	return c == "" || c == kernel.AsSoonAsPossible || c == kernel.AsLateAsPossible || dated(c)
}

// SQ-08 and SQ-09. A date constraint is in force only once
// kernel.ApplyConstraintDates has converted its date; one it could not
// convert is ignored by the scheduler without notice, so SQ-09 reports it.
// So is a constraint type the scheduler does not support (KRN-01).
func (a *auditor) checkConstraints() error {
	reason := a.datedReason()
	hard, broken := 0, 0
	for _, id := range a.ids {
		if err := a.tick(); err != nil {
			return err
		}
		t, c := a.raw[id], a.cpm[id]
		if !supported(t.Constraint) {
			if reason == "" {
				broken++
				a.add(ConstraintViolated, SeverityError, id, nil,
					fmt.Sprintf("constraint type %s is not supported", t.Constraint), "",
					"The scheduler supports ASAP, ALAP, SNET, FNLT and MFO only, so it ignores this constraint and schedules the task as soon as possible.",
					"Replace it with a supported constraint, or remove it.")
			}
			continue
		}
		if !dated(t.Constraint) {
			continue
		}
		label := fmt.Sprintf("%s %s", t.Constraint, t.ConstraintDate)
		unapplied := a.startOK && !c.ConstraintArmed
		if t.Constraint == kernel.MustFinishOn || t.Constraint == kernel.FinishNoLaterThan {
			hard++
			var binding string
			switch {
			case !a.startOK:
				binding = "binding not evaluated: " + noStart
			case unapplied:
				binding = "could not be applied, so the scheduler ignores it"
			case a.cpmErr != nil:
				binding = "binding not evaluated: " + a.cpmReason()
			case c.ConstraintViolated:
				binding = "binds: the logic pushes the task past it"
			default:
				binding = "met by the current logic"
			}
			a.add(HardConstraint, SeverityWarning, id, nil, label+"; "+binding, "",
				"A hard constraint fixes a date whatever the logic says, so the schedule cannot show what a delay does to it.",
				"Prefer start-no-earlier-than, or track the date as a deadline instead of constraining the task.")
		}
		if reason != "" {
			continue
		}
		switch {
		case unapplied:
			broken++
			a.add(ConstraintViolated, SeverityError, id, nil, label+" could not be applied", "",
				"The constraint date is not a valid date or lies too far from the project start, so it was not applied and the scheduler ignored the constraint.",
				"Correct the constraint date.")
		case c.ConstraintViolated:
			broken++
			a.add(ConstraintViolated, SeverityError, id, nil, label+" cannot be met", "",
				"The logic places this task past its constraint date, so the date cannot be met.",
				"Shorten or re-sequence the work before it, or move the date.")
		}
	}
	a.ran(HardConstraint, "")
	a.measure(string(HardConstraint), hard, len(a.ids))
	if reason != "" {
		a.skipped(ConstraintViolated, reason)
		return nil
	}
	a.ran(ConstraintViolated, "")
	a.measure(string(ConstraintViolated), broken, len(a.ids))
	return nil
}

// SQ-10 and SQ-11. Float depends on the armed constraints, so both need a
// project start date as well as the scheduler's results.
func (a *auditor) checkFloat() error {
	if reason := a.datedReason(); reason != "" {
		a.skipped(HighFloat, reason)
		a.skipped(NegativeFloat, reason)
		return nil
	}
	high, negative := 0, 0
	for _, id := range a.ids {
		if err := a.tick(); err != nil {
			return err
		}
		f := a.cpm[id].Float
		observed := "float " + days(f)
		if f > a.prof.HighFloatDays+epsilon {
			high++
			a.add(HighFloat, SeverityInfo, id, nil, observed, moreThan(a.prof.HighFloatDays),
				"Float this large usually means logic is missing, so the task is not really tied to the work around it.",
				"Check the task's successors and the dates it should drive.")
		}
		if f < -epsilon {
			negative++
			a.add(NegativeFloat, SeverityError, id, nil, observed, "",
				"Negative float means the current logic and durations cannot meet a constraint date.",
				"Recover the time on the critical path, or revisit the constraint.")
		}
	}
	a.ran(HighFloat, "")
	a.ran(NegativeFloat, "")
	a.measure(string(HighFloat), high, len(a.ids))
	a.measure(string(NegativeFloat), negative, len(a.ids))
	return nil
}

// SQ-12. Milestones are excluded whatever their duration.
func (a *auditor) checkDurations() error {
	long := 0
	for _, id := range a.ids {
		t := a.raw[id]
		if t.Milestone || !(t.Duration > a.prof.LongDurationDays) {
			continue
		}
		long++
		a.add(LongDuration, SeverityInfo, id, nil, "duration "+days(t.Duration), moreThan(a.prof.LongDurationDays),
			"A long task is hard to give progress to and hides the detail needed to manage it.",
			"Break it into shorter pieces of work, each with its own result.")
	}
	a.ran(LongDuration, "")
	a.measure(string(LongDuration), long, a.nonMilestones())
	return nil
}

// SQ-13
func (a *auditor) checkMilestones() error {
	for _, id := range a.ids {
		if !a.raw[id].Milestone || a.indeg[id] > 0 || a.outdeg[id] > 0 {
			continue
		}
		a.add(MilestoneWithout, SeverityWarning, id, nil, "no predecessor and no successor", "",
			"A milestone without links is a date in name only: nothing moves it and it drives nothing.",
			"Link the milestone to the work that achieves it and to the work that follows.")
	}
	a.ran(MilestoneWithout, "")
	return nil
}

// SQ-14 reads the copy taken before scheduling: the scheduler clamps
// percent complete to 0..100, which would hide an out-of-range value.
func (a *auditor) checkActuals() error {
	invalid := 0
	for _, id := range a.ids {
		if err := a.tick(); err != nil {
			return err
		}
		problems := a.actualsProblems(a.raw[id])
		if len(problems) > 0 {
			invalid++
		}
		for _, p := range problems {
			a.add(InvalidActuals, SeverityError, id, nil, p, "",
				"Progress that contradicts itself makes earned value and forecasts unreliable.",
				"Correct the actual dates or the percent complete so that they agree.")
		}
	}
	note := ""
	if !a.reportOK {
		note = "actual dates after the reporting date were not checked: there is no readable reporting date"
	}
	a.ran(InvalidActuals, note)
	a.measure(string(InvalidActuals), invalid, len(a.ids))
	return nil
}

func (a *auditor) actualsProblems(t *kernel.Task) []string {
	var out []string
	pct := t.PercentComplete
	startSet, finishSet := t.ActualStart != "", t.ActualFinish != ""
	start, startOK := parseDate(t.ActualStart)
	finish, finishOK := parseDate(t.ActualFinish)

	switch {
	case math.IsNaN(pct) || pct < 0 || pct > 100:
		out = append(out, "percent complete "+number(pct)+" is outside 0 to 100")
	case pct == 100 && !finishSet:
		out = append(out, "100% complete without an actual finish")
	case pct > 0 && !startSet:
		out = append(out, number(pct)+"% complete without an actual start")
	}
	if startSet && !startOK {
		out = append(out, fmt.Sprintf("actual start %q is not a date", t.ActualStart))
	}
	if finishSet && !finishOK {
		out = append(out, fmt.Sprintf("actual finish %q is not a date", t.ActualFinish))
	}
	if finishSet && !startSet {
		out = append(out, "actual finish without an actual start")
	}
	if startOK && finishOK && finish.Before(start) {
		out = append(out, fmt.Sprintf("actual finish %s is before actual start %s", t.ActualFinish, t.ActualStart))
	}
	if a.reportOK {
		if startOK && start.After(a.report) {
			out = append(out, fmt.Sprintf("actual start %s is after the reporting date %s", t.ActualStart, a.in.ReportingDate))
		}
		if finishOK && finish.After(a.report) {
			out = append(out, fmt.Sprintf("actual finish %s is after the reporting date %s", t.ActualFinish, a.in.ReportingDate))
		}
	}
	return out
}

// SQ-15. The reporting date becomes a working-day index with the kernel's
// own mapping: a date on a non-working day maps to the next working day.
func (a *auditor) checkProgress() error {
	reason := ""
	switch {
	case !a.startOK:
		reason = noStart
	case !a.reportOK:
		reason = "there is no readable reporting date"
	default:
		reason = a.cpmReason()
	}
	if reason != "" {
		a.skipped(MissingProgress, reason)
		return nil
	}
	due, ok := kernel.DayOffset(a.start, a.report, a.in.IsWorkday)
	if !ok {
		a.skipped(MissingProgress, "the reporting date is too far from the project start to place on the calendar")
		return nil
	}
	for _, id := range a.ids {
		if err := a.tick(); err != nil {
			return err
		}
		t := a.raw[id]
		if !(a.cpm[id].ES < due-epsilon) || t.PercentComplete > 0 || t.ActualStart != "" {
			continue
		}
		a.add(MissingProgress, SeverityWarning, id, nil,
			"planned to start before the reporting date "+a.in.ReportingDate+", with no progress recorded", "",
			"The plan says this work should have started, but it has no actual start or progress.",
			"Record the actual start and progress, or reschedule the task.")
	}
	a.ran(MissingProgress, "")
	return nil
}

// SQ-16 uses the overallocations found on the scheduled copy with the
// capacity plan the caller passed; a stored flag is never read.
func (a *auditor) checkOverallocation() error {
	if reason := a.cpmReason(); reason != "" {
		a.skipped(Overallocation, reason)
		return nil
	}
	byTask := map[string]map[string]bool{}
	for _, b := range a.breaches {
		for _, id := range b.TaskIDs {
			if byTask[id] == nil {
				byTask[id] = map[string]bool{}
			}
			byTask[id][b.Resource] = true
		}
	}
	for _, id := range a.ids {
		if !a.cpm[id].Overallocated {
			continue
		}
		var res []string
		for r := range byTask[id] {
			res = append(res, r)
		}
		sort.Strings(res)
		a.add(Overallocation, SeverityWarning, id, nil, "over capacity: "+strings.Join(res, ", "), "",
			"On at least one day the work assigned to a resource is more than its capacity, so the dates cannot all be met as planned.",
			"Level the resource, change the assignment, or add capacity.")
	}
	a.ran(Overallocation, "")
	return nil
}

// SQ-17
func (a *auditor) checkResources() error {
	none := 0
	for _, id := range a.ids {
		t := a.raw[id]
		if t.Milestone || len(t.Assignments) > 0 {
			continue
		}
		none++
		a.add(UnresourcedTask, SeverityInfo, id, nil, "no resource assigned", "",
			"Work with no resource cannot be checked against capacity or costed by resource.",
			"Assign the people or equipment who will do it.")
	}
	a.ran(UnresourcedTask, "")
	a.measure(string(UnresourcedTask), none, a.nonMilestones())
	return nil
}

// SQ-18
func (a *auditor) checkContext() error {
	note := func(observed, explanation, advice string) {
		a.add(ReportingContext, SeverityInfo, "", nil, observed, "", explanation, advice)
	}
	switch {
	case a.in.ProjectStart == "":
		note("there is no project start date",
			"Without a project start date, date constraints cannot be applied, so several checks were not run.",
			"Set the project start date in Project Settings.")
	case !a.startOK:
		note(fmt.Sprintf("the project start date %q is unreadable", a.in.ProjectStart),
			"Without a readable project start date, date constraints cannot be applied, so several checks were not run.",
			"Set the project start date in Project Settings.")
	}
	switch {
	case a.in.ReportingDate == "":
		note("no reporting date was given",
			"Without a reporting date, progress cannot be compared with the plan.",
			"Choose the status date for this audit.")
	case !a.reportOK:
		note(fmt.Sprintf("the reporting date %q is unreadable", a.in.ReportingDate),
			"Without a readable reporting date, progress cannot be compared with the plan.",
			"Choose the status date for this audit.")
	}
	if !a.in.BaselineAvailable {
		note("there is no baseline",
			"Without a baseline, the schedule cannot be compared with the plan it was approved against.",
			"Save a baseline once the plan is agreed.")
	}
	for _, anchor := range []struct {
		kind string
		ids  []string
	}{{"start", a.in.StartAnchors}, {"finish", a.in.FinishAnchors}} {
		for id := range set(anchor.ids) {
			if _, ok := a.raw[id]; ok {
				continue
			}
			note(fmt.Sprintf("the designated %s anchor %q is not in this schedule", anchor.kind, id),
				"An anchor that is not a task exempts nothing, so the start or end of every chain is reported.",
				"Choose an existing task as the anchor.")
		}
	}
	a.ran(ReportingContext, "")
	return nil
}
