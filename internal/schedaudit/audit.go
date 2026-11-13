// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package schedaudit

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gopmgr/internal/kernel"
)

// ErrInvalidInput reports tasks the audit cannot check.
var ErrInvalidInput = errors.New("schedaudit: invalid input")

// edge is one effective precedence link, after the kernel's merge of typed
// links and legacy precedents.
type edge struct {
	pred, succ string
	typ        kernel.LinkType
	lag        float64
}

func (e edge) ref() *LinkRef {
	return &LinkRef{Pred: e.pred, Succ: e.succ, Type: e.typ, Lag: e.lag}
}

// auditor holds one audit's working state.
type auditor struct {
	ctx  context.Context
	in   Input
	prof Profile

	ids   []string                // sorted task IDs
	raw   map[string]*kernel.Task // copy taken before scheduling
	links []edge                  // links whose predecessor exists
	stray []edge                  // links naming a task that does not exist
	// unknownType lists links whose type the kernel does not recognise and
	// schedules as FS.
	unknownType []edge
	indeg       map[string]int
	outdeg      map[string]int

	start    time.Time
	startOK  bool
	report   time.Time
	reportOK bool

	cpm      map[string]*kernel.Task // scheduled copy
	cpmErr   error                   // the scheduler's refusal, if any
	breaches []kernel.Overallocation

	findings []Finding
	metrics  []Metric
	status   map[RuleID]RuleStatus
	count    int // tasks handled since the last context check
}

// Audit checks in.Tasks against every rule and returns the findings. It
// never changes in.Tasks. It returns ErrInvalidInput for an empty or
// duplicate task ID, ErrInvalidProfile for an unusable profile, and
// ctx.Err() when ctx is cancelled.
func Audit(ctx context.Context, in Input) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := in.Profile.validate(); err != nil {
		return Result{}, err
	}
	a := &auditor{ctx: ctx, in: in, prof: in.Profile, status: map[RuleID]RuleStatus{}}
	if err := a.load(); err != nil {
		return Result{}, err
	}
	steps := []func() error{
		a.schedule,
		a.checkLogic, a.checkStray, a.checkCycles, a.checkLags, a.checkLinkTypes,
		a.checkConstraints, a.checkFloat, a.checkDurations, a.checkMilestones,
		a.checkActuals, a.checkProgress, a.checkOverallocation, a.checkResources,
		a.checkContext,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return Result{}, err
		}
	}
	return a.result(), nil
}

// tick checks the context every cancellationInterval calls.
func (a *auditor) tick() error {
	a.count++
	if a.count%cancellationInterval != 0 {
		return nil
	}
	return a.ctx.Err()
}

// load copies the tasks, checks their IDs and builds the link indexes.
func (a *auditor) load() error {
	a.raw = make(map[string]*kernel.Task, len(a.in.Tasks))
	a.cpm = make(map[string]*kernel.Task, len(a.in.Tasks))
	for i := range a.in.Tasks {
		t := &a.in.Tasks[i]
		if t.ID == "" {
			return fmt.Errorf("%w: task %d has no ID", ErrInvalidInput, i+1)
		}
		if _, dup := a.raw[t.ID]; dup {
			return fmt.Errorf("%w: task ID %q appears more than once", ErrInvalidInput, t.ID)
		}
		raw, cpm := cloneTask(*t), cloneTask(*t)
		a.raw[t.ID], a.cpm[t.ID] = &raw, &cpm
		a.ids = append(a.ids, t.ID)
	}
	sort.Strings(a.ids)

	a.indeg, a.outdeg = map[string]int{}, map[string]int{}
	for _, id := range a.ids {
		if err := a.tick(); err != nil {
			return err
		}
		t := a.raw[id]
		for _, l := range kernel.EffectiveLinks(t) {
			e := edge{pred: l.Pred, succ: id, typ: l.Type, lag: l.Lag}
			if _, ok := a.raw[l.Pred]; !ok {
				a.stray = append(a.stray, e)
				continue
			}
			a.links = append(a.links, e)
			a.indeg[id]++
			a.outdeg[l.Pred]++
		}
		// Each typed link yields exactly one effective link, which the
		// kernel turns into FS when it does not recognise the type.
		for _, l := range t.Links {
			if _, ok := a.raw[l.Pred]; !ok || knownLinkType(l.Type) {
				continue
			}
			a.unknownType = append(a.unknownType, edge{pred: l.Pred, succ: id, typ: l.Type, lag: l.Lag})
		}
	}

	a.start, a.startOK = parseDate(a.in.ProjectStart)
	a.report, a.reportOK = parseDate(a.in.ReportingDate)
	return nil
}

func knownLinkType(t kernel.LinkType) bool {
	switch t {
	case "", kernel.FinishToStart, kernel.StartToStart, kernel.FinishToFinish, kernel.StartToFinish:
		return true
	}
	return false
}

// parseDate reads a calendar date. time.Parse gives UTC midnight, so a
// comparison between two dates never depends on the local time zone.
func parseDate(s string) (time.Time, bool) {
	d, err := time.Parse(kernel.DateLayout, s)
	return d, err == nil
}

// schedule arms date constraints on the scheduling copy, when there is a
// project start date, and runs the kernel's scheduler and overallocation
// detector on it.
func (a *auditor) schedule() error {
	if a.startOK {
		kernel.ApplyConstraintDates(a.cpm, a.start, a.in.IsWorkday)
	}
	a.cpmErr = kernel.ScheduleCPM(a.cpm)
	if a.cpmErr == nil {
		a.breaches = kernel.DetectOverallocationsWithPlan(a.cpm, a.in.Capacity)
	}
	return a.ctx.Err()
}

// cpmReason is why a rule that needs the scheduler's results did not run,
// or "" when it can.
func (a *auditor) cpmReason() string {
	switch {
	case a.cpmErr == nil:
		return ""
	case errors.Is(a.cpmErr, kernel.ErrSchedulingCycle):
		return "the schedule has a dependency cycle (see SQ-04), so the scheduler computed no dates"
	default:
		return "the scheduler refused the schedule: " + a.cpmErr.Error()
	}
}

const noStart = "there is no readable project start date, so date constraints could not be applied"

// datedReason is why a rule that needs armed constraints and the
// scheduler's results did not run, or "".
func (a *auditor) datedReason() string {
	if !a.startOK {
		return noStart
	}
	return a.cpmReason()
}

func (a *auditor) ran(rule RuleID, note string) {
	a.status[rule] = RuleStatus{Rule: rule, Evaluated: true, Reason: note}
}

func (a *auditor) skipped(rule RuleID, reason string) {
	a.status[rule] = RuleStatus{Rule: rule, Reason: reason}
}

// add records one finding for a task (link nil) or a link (task "").
func (a *auditor) add(rule RuleID, sev Severity, taskID string, link *LinkRef, observed, threshold, explanation, advice string) {
	f := Finding{
		Rule: rule, Profile: a.prof.Name, Severity: sev, Link: link,
		Observed: observed, Threshold: threshold, Explanation: explanation, Advice: advice,
		Source: a.in.Source, EngineVersion: EngineVersion,
	}
	if taskID != "" {
		f.TaskID = taskID
		f.TaskTitle = a.raw[taskID].Title
	}
	a.findings = append(a.findings, f)
}

// measure records a metric and applies the profile's limit to it, if any.
func (a *auditor) measure(id string, num, den int) Metric {
	m := Metric{ID: id, Numerator: num, Denominator: den}
	if den > 0 {
		m.Percent = float64(num) * 100 / float64(den)
	}
	if l, ok := a.prof.Limits[id]; ok {
		l := l
		pass := den == 0 || (l.AtLeast && m.Percent >= l.Percent) || (!l.AtLeast && m.Percent <= l.Percent)
		m.Limit, m.Pass = &l, &pass
	}
	a.metrics = append(a.metrics, m)
	return m
}

func days(v float64) string { return fmt.Sprintf("%g working days", v) }

func moreThan(v float64) string { return "more than " + days(v) }

func (a *auditor) nonMilestones() int {
	n := 0
	for _, id := range a.ids {
		if !a.raw[id].Milestone {
			n++
		}
	}
	return n
}

func (a *auditor) result() Result {
	sort.SliceStable(a.findings, func(i, j int) bool {
		x, y := a.findings[i], a.findings[j]
		if x.Severity.rank() != y.Severity.rank() {
			return x.Severity.rank() < y.Severity.rank()
		}
		if x.Rule != y.Rule {
			return x.Rule < y.Rule
		}
		if x.TaskID != y.TaskID {
			return x.TaskID < y.TaskID
		}
		xp, xs := linkKey(x.Link)
		yp, ys := linkKey(y.Link)
		if xp != yp {
			return xp < yp
		}
		if xs != ys {
			return xs < ys
		}
		return x.Observed < y.Observed
	})
	sort.SliceStable(a.metrics, func(i, j int) bool { return a.metrics[i].ID < a.metrics[j].ID })
	r := Result{
		Profile: a.prof.Name, Source: a.in.Source, EngineVersion: EngineVersion,
		Findings: a.findings, Metrics: a.metrics,
	}
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if r.Metrics == nil {
		r.Metrics = []Metric{}
	}
	if a.reportOK {
		r.ReportingDate = a.in.ReportingDate
	}
	for _, rule := range AllRules() {
		s, ok := a.status[rule]
		if !ok {
			s = RuleStatus{Rule: rule, Reason: "not run"}
		}
		r.Rules = append(r.Rules, s)
	}
	return r
}

func linkKey(l *LinkRef) (string, string) {
	if l == nil {
		return "", ""
	}
	return l.Pred, l.Succ
}
