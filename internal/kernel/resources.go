// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"math"
	"slices"
	"sort"
)

// Assignment binds a task to a named resource at the given units
// (1.0 = full-time). Zero or negative units are treated as 1.0 so a
// bare {"resource":"alice"} assignment behaves sensibly.
type Assignment struct {
	Resource   string   `json:"resource"`
	Units      float64  `json:"units,omitempty"`
	CalendarID string   `json:"calendar_id,omitempty"`
	SkillTags  []string `json:"skill_tags,omitempty"`
	MaxUnits   float64  `json:"max_units,omitempty"`
}

func (a Assignment) effectiveUnits() float64 {
	units := a.Units
	if a.Units <= 0 {
		units = 1
	}
	if a.MaxUnits > 0 && units > a.MaxUnits {
		return a.MaxUnits
	}
	return units
}

// ResourceCalendar describes one resource's capacity exceptions by
// integer project day. It intentionally stays in working-day offsets
// so the pure kernel remains independent of wall-clock calendars.
type ResourceCalendar struct {
	ID              string            `json:"id,omitempty"`
	Resource        string            `json:"resource,omitempty"`
	DefaultCapacity float64           `json:"default_capacity,omitempty"`
	Overrides       map[int]float64   `json:"overrides,omitempty"`
	WeeklyCapacity  map[int]float64   `json:"weekly_capacity,omitempty"`
	Notes           map[int]string    `json:"notes,omitempty"`
	SkillTags       []string          `json:"skill_tags,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// ResourceCapacityPlan resolves per-resource capacity at a project day.
// Capacities preserves the old API's static resource capacities;
// Calendars adds named per-resource overrides for availability changes.
type ResourceCapacityPlan struct {
	DefaultCapacity float64                     `json:"default_capacity,omitempty"`
	Capacities      map[string]float64          `json:"capacities,omitempty"`
	Calendars       map[string]ResourceCalendar `json:"calendars,omitempty"`
}

func capacityPlanFromMap(capacities map[string]float64) ResourceCapacityPlan {
	return ResourceCapacityPlan{DefaultCapacity: 1, Capacities: capacities}
}

func (p ResourceCapacityPlan) baseCapacity(resource string) float64 {
	defaultCapacity := p.DefaultCapacity
	if defaultCapacity <= 0 {
		defaultCapacity = 1
	}
	capacity := defaultCapacity
	if c, ok := p.Capacities[resource]; ok && c > 0 {
		capacity = c
	}
	return capacity
}

func (p ResourceCapacityPlan) capacityFor(resource string, day int) float64 {
	capacity := p.baseCapacity(resource)
	if cal, ok := p.Calendars[resource]; ok {
		capacity = p.capacityFromCalendar(cal, capacity, day)
	}
	return capacity
}

func (p ResourceCapacityPlan) capacityForAssignment(a Assignment, day int) float64 {
	capacity := p.baseCapacity(a.Resource)
	if a.CalendarID != "" {
		if cal, ok := p.Calendars[a.CalendarID]; ok {
			return p.capacityFromCalendar(cal, capacity, day)
		}
	}
	return p.capacityFor(a.Resource, day)
}

func (p ResourceCapacityPlan) capacityFromCalendar(cal ResourceCalendar, fallback float64, day int) float64 {
	capacity := fallback
	if cal.DefaultCapacity > 0 {
		capacity = cal.DefaultCapacity
	}
	if cal.WeeklyCapacity != nil {
		weekday := day % 7
		if weekday < 0 {
			weekday += 7
		}
		if c, ok := cal.WeeklyCapacity[weekday]; ok && c >= 0 {
			capacity = c
		}
	}
	if cal.Overrides != nil {
		if c, ok := cal.Overrides[day]; ok && c >= 0 {
			capacity = c
		}
	}
	return capacity
}

// DefaultLevelingHorizon caps how far (in whole days) resource leveling
// will push a task while searching for capacity, preventing an infinite
// walk when demand can never fit (e.g. units larger than capacity). It is
// the fallback when LevelingOptions.Horizon is unset; callers can override
// it per schedule via LevelResourcesWithOptions.
const DefaultLevelingHorizon = 10000

// ErrSchedulingCycle is returned by LevelResourcesWithOptions when the task
// graph contains a dependency cycle, so the CPM pass could not run.
var ErrSchedulingCycle = errors.New("kernel: schedule contains a dependency cycle")

// ErrLevelingHorizonExceeded is returned by LevelResourcesWithOptions when
// one or more tasks could not be placed within the configured leveling
// horizon (their demand can never fit the available capacity). It is a
// surfaced warning, not a hard failure: the schedule is still returned
// levelled as far as possible, the unplaceable tasks are left at their
// precedence-earliest start, and LevelingResult.UnplacedTaskIDs names them.
var ErrLevelingHorizonExceeded = errors.New("kernel: leveling horizon exceeded; some tasks could not be placed within capacity")

// LevelingStrategy selects the ready-queue priority rule for serial
// leveling: which of several ready tasks claims a contended slot first.
type LevelingStrategy string

const (
	// LeastTotalFloat schedules the ready task with the least scheduling
	// slack first (ordered by latest start, ties broken by ID). This is the
	// default and matches the classic serial method — near-critical work
	// wins contention so the project finish is protected.
	//
	// NOTE: it orders by latest start (LS), a proxy for total float
	// (LS−ES) that is exact only when ready tasks share the same ES; kept
	// as LS to preserve historical behaviour. A future priority-override
	// slice reasoning about true float should not assume the name is
	// literal.
	LeastTotalFloat LevelingStrategy = "ltf"
	// EarliestDeadline schedules the ready task with the earliest deadline
	// first (ordered by latest finish, ties broken by ID). Useful when the
	// goal is to hit per-task due dates rather than protect total float.
	EarliestDeadline LevelingStrategy = "edf"
)

// LevelingOptions tunes a serial resource-leveling pass.
type LevelingOptions struct {
	// Horizon is the maximum number of whole days a task may be delayed
	// while searching for capacity. Zero or negative means use
	// DefaultLevelingHorizon.
	Horizon int `json:"horizon,omitempty"`
	// Strategy is the ready-queue priority rule. Empty means LeastTotalFloat.
	Strategy LevelingStrategy `json:"strategy,omitempty"`
	// PriorityCritical, when true, makes critical-path tasks win ready-queue
	// contention ahead of any non-critical task, before the base Strategy
	// tie-break is applied. This protects the critical path from being
	// delayed by a floating task — most visible under EarliestDeadline and
	// on strategy ties.
	PriorityCritical bool `json:"priority_critical,omitempty"`
	// AllowSplitting, when true, lets a task that cannot fit its whole
	// duration in contiguous days be worked on non-contiguous days where
	// capacity exists (activity splitting / interruption). Split tasks
	// record their working days in Task.WorkDays and are reported in
	// LevelingResult.SplitTaskIDs. A task whose single-day demand exceeds
	// capacity still cannot be placed and is reported unplaced.
	AllowSplitting bool `json:"allow_splitting,omitempty"`
}

// LevelingResult reports the outcome of a resource-leveling pass.
type LevelingResult struct {
	// UnplacedTaskIDs lists tasks whose demand never fit within the
	// horizon; each was left at its precedence-earliest start and remains
	// flagged for the caller. Sorted for deterministic output. Empty on a
	// fully levelled schedule.
	UnplacedTaskIDs []string `json:"unplaced_task_ids,omitempty"`
	// SplitTaskIDs lists tasks that AllowSplitting placed on non-contiguous
	// working days (sorted). Empty when splitting was off or unnecessary.
	SplitTaskIDs []string `json:"split_task_ids,omitempty"`
}

// occupiedDays returns the working-day offsets a task consumes its
// resources on. For a split task (WorkDays set by leveling) that is the
// explicit non-contiguous list; otherwise it is the contiguous taskSpan.
func occupiedDays(t *Task) []int {
	if len(t.WorkDays) > 0 {
		return t.WorkDays
	}
	first, last, ok := taskSpan(t)
	if !ok {
		return nil
	}
	out := make([]int, 0, last-first+1)
	for d := first; d <= last; d++ {
		out = append(out, d)
	}
	return out
}

// taskSpan is the inclusive integer day range a task occupies for
// resource demand: the days its work falls in, by the rule AnchorSchedule
// uses to show it (first = floor(ES), last = ceil(EF)-1, raised to the
// first; the epsilon absorbs noise from summed fractions). Zero-duration
// tasks occupy no days. Since 2026-11-21 (owner decision, OR-01 contract
// change) a part-day start counts on the day it falls in, not the next
// one. A task whose dates are not finite offsets within
// maxAllocationDays, which CalculateCPM never produces, is reported as
// occupying no days, because callers size slices from the span.
func taskSpan(t *Task) (first, last int, occupies bool) {
	if t.Duration <= 0 || !withinAllocationLimit(t.ES) || !withinAllocationLimit(t.EF) {
		return 0, 0, false
	}
	first, last = spanDays(t.ES, t.EF)
	return first, last, true
}

// spanDays is the inclusive day range [es, ef) of work falls in: floor(es)
// to ceil(ef)-1, raised to the first day. Levelling and detection share it.
func spanDays(es, ef float64) (first, last int) {
	first = int(math.Floor(es + 1e-9))
	last = int(math.Ceil(ef-1e-9)) - 1
	if last < first {
		last = first
	}
	return first, last
}

// dayShare is the fraction of working day d that [es, ef) covers, never
// negative: 1 for a day the work spans whole, less on a day it starts or
// ends part-way through. Resource demand on day d is units times the share
// (OR-01 contract change, 2026-11-21).
func dayShare(es, ef float64, d int) float64 {
	return max(0, min(ef, float64(d+1))-max(es, float64(d)))
}

// taskDayShare is a task's share of day d for resource demand. A split
// task (WorkDays) takes each listed day whole, as levelling books it.
func taskDayShare(t *Task, d int) float64 {
	if len(t.WorkDays) > 0 {
		return 1
	}
	return dayShare(t.ES, t.EF, d)
}

// ResourceUsage builds each resource's per-day demand profile from a
// scheduled task map (CalculateCPM must have run). The slice index is
// the working-day offset; the value is the sum, over every task occupying
// that day, of its assignment units times the share of the day it uses
// (taskDayShare). All profiles share the same length (the last day of any
// task's span + 1; split days never extend it).
func ResourceUsage(tasks map[string]*Task) map[string][]float64 {
	horizon := 0
	for _, t := range tasks {
		if _, last, ok := taskSpan(t); ok && last+1 > horizon {
			horizon = last + 1
		}
	}

	usage := make(map[string][]float64)
	for _, t := range tasks {
		od := occupiedDays(t)
		if len(od) == 0 {
			continue
		}
		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			profile, exists := usage[a.Resource]
			if !exists {
				profile = make([]float64, horizon)
				usage[a.Resource] = profile
			}
			for _, d := range od {
				if d >= 0 && d < len(profile) {
					profile[d] += a.effectiveUnits() * taskDayShare(t, d)
				}
			}
		}
	}
	return usage
}

// peakUnits is, per resource and day of its usage profile, the largest
// effective units any one contiguous task uses it at on a day it works
// part of. Units are a rate: a task needing more of a resource at once
// than the resource has can never run, however little of the day it uses
// (owner decision 2026-11-23), so detection flags such a day even when the
// day's total demand fits. Whole days and split days already add the full
// units to the day's demand, so only part days need this.
func peakUnits(tasks map[string]*Task, usage map[string][]float64) map[string][]float64 {
	peak := make(map[string][]float64, len(usage))
	for _, t := range tasks {
		if len(t.WorkDays) > 0 {
			continue
		}
		first, last, ok := taskSpan(t)
		if !ok {
			continue
		}
		for _, a := range t.Assignments {
			profile, exists := usage[a.Resource]
			if a.Resource == "" || !exists {
				continue
			}
			if peak[a.Resource] == nil {
				peak[a.Resource] = make([]float64, len(profile))
			}
			for d := max(first, 0); d <= last && d < len(profile); d++ {
				if dayShare(t.ES, t.EF, d) > 0 {
					peak[a.Resource][d] = max(peak[a.Resource][d], a.effectiveUnits())
				}
			}
		}
	}
	return peak
}

// peakAt reads peakUnits, 0 where no part-day task works.
func peakAt(peak map[string][]float64, resource string, day int) float64 {
	if p := peak[resource]; day < len(p) {
		return p[day]
	}
	return 0
}

// ResourceCapacityProfiles builds calendar-aware per-day capacity
// profiles for the requested resources. Every returned slice has
// length horizon, with index d matching project working-day offset d.
func ResourceCapacityProfiles(plan ResourceCapacityPlan, resources []string, horizon int) map[string][]float64 {
	if horizon <= 0 || len(resources) == 0 {
		return map[string][]float64{}
	}

	seen := make(map[string]bool, len(resources))
	ordered := make([]string, 0, len(resources))
	for _, r := range resources {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		ordered = append(ordered, r)
	}
	sort.Strings(ordered)

	profiles := make(map[string][]float64, len(ordered))
	for _, r := range ordered {
		values := make([]float64, horizon)
		for day := range values {
			values[day] = plan.capacityFor(r, day)
		}
		profiles[r] = values
	}
	return profiles
}

// Overallocation reports one resource exceeding capacity on one day.
type Overallocation struct {
	Resource string   `json:"resource"`
	Day      int      `json:"day"`
	Demand   float64  `json:"demand"`
	Capacity float64  `json:"capacity"`
	TaskIDs  []string `json:"task_ids"`
}

// DetectOverallocations compares each resource's usage profile to its
// capacity (capacities[resource]; a missing entry means 1.0) and
// returns every (resource, day) breach sorted by resource then day.
// It also sets Task.Overallocated on each task that occupies a
// breached day with the breached resource (clearing the flag on all
// other tasks first), so editors can mark the offenders directly.
func DetectOverallocations(tasks map[string]*Task, capacities map[string]float64) []Overallocation {
	return DetectOverallocationsWithPlan(tasks, capacityPlanFromMap(capacities))
}

// DetectOverallocationsWithPlan compares each resource's usage profile
// to calendar-aware capacity and returns every breach.
func DetectOverallocationsWithPlan(tasks map[string]*Task, plan ResourceCapacityPlan) []Overallocation {
	for _, t := range tasks {
		t.Overallocated = false
	}

	usage := ResourceUsage(tasks)
	peak := peakUnits(tasks, usage)
	entries := overallocationEntries(tasks)
	runs := overallocationMembershipRuns(entries, usage)

	resources := make([]string, 0, len(usage))
	for r := range usage {
		resources = append(resources, r)
	}
	sort.Strings(resources)

	var out []Overallocation
	members := newOverallocationMembers(len(entries))
	for _, r := range resources {
		members.start(runs[r])
		for day, demand := range usage[r] {
			members.advanceTo(day)
			capacity := plan.capacityFor(r, day)
			if demand <= capacity+1e-9 && peakAt(peak, r, day) <= capacity+1e-9 {
				continue
			}
			breach := Overallocation{
				Resource: r,
				Day:      day,
				Demand:   demand,
				Capacity: capacity,
			}
			for _, i := range members.sorted() {
				breach.TaskIDs = append(breach.TaskIDs, entries[i].task.ID)
				entries[i].task.Overallocated = true
			}
			out = append(out, breach)
		}
	}
	return out
}

// The participant index below replaces a scan of every task on every
// overallocated day (ADR-010 S0 finding F3). It reports exactly the
// participants that scan did; docs/testing/resource-overallocation-contract.md
// records that contract and the tests that hold it.

// overallocationEntry is one input map entry. Participants are tracked per
// entry, not per *Task or printed ID: a task referenced by two map keys is
// listed twice, as it always has been.
type overallocationEntry struct {
	key  string
	task *Task
}

// overallocationEntries returns the map's entries ordered by printed ID and
// then key, so a breach lists its participants by sorting entry indexes.
func overallocationEntries(tasks map[string]*Task) []overallocationEntry {
	entries := make([]overallocationEntry, 0, len(tasks))
	for key, t := range tasks {
		entries = append(entries, overallocationEntry{key: key, task: t})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].task.ID != entries[j].task.ID {
			return entries[i].task.ID < entries[j].task.ID
		}
		return entries[i].key < entries[j].key
	})
	return entries
}

// overallocationRun is a half-open range of days [start, end) on which one
// entry occupies a resource.
type overallocationRun struct {
	start, end, entry int
}

// overallocationMembershipRuns returns, per resource with a usage profile,
// the runs of days each entry occupies it. An entry takes part when it has
// an assignment to the resource and occupies the day: a non-empty WorkDays
// list names the exact days, otherwise the task's span does. Runs are
// clipped to the resource's profile, the only days detection examines.
func overallocationMembershipRuns(entries []overallocationEntry, usage map[string][]float64) map[string][]overallocationRun {
	runs := make(map[string][]overallocationRun, len(usage))
	for i, e := range entries {
		var resources []string
		for _, a := range e.task.Assignments {
			if _, ok := usage[a.Resource]; ok && !slices.Contains(resources, a.Resource) {
				resources = append(resources, a.Resource)
			}
		}
		for _, r := range resources {
			for _, run := range occupiedRuns(e.task, len(usage[r])) {
				runs[r] = append(runs[r], overallocationRun{start: run[0], end: run[1], entry: i})
			}
		}
	}
	return runs
}

// occupiedRuns returns the days in [0, limit) a task occupies as sorted,
// disjoint, non-adjacent half-open runs. A contiguous span is one run however
// long it is. Split days are deduplicated and adjacent days merged, so an
// entry never leaves and re-enters on the same day.
func occupiedRuns(t *Task, limit int) [][2]int {
	if len(t.WorkDays) == 0 {
		first, last, ok := taskSpan(t)
		if !ok {
			return nil
		}
		start, end := max(first, 0), min(last+1, limit)
		if start >= end {
			return nil
		}
		return [][2]int{{start, end}}
	}
	days := make([]int, 0, len(t.WorkDays))
	for _, d := range t.WorkDays {
		if d >= 0 && d < limit {
			days = append(days, d)
		}
	}
	slices.Sort(days)
	var runs [][2]int
	for _, d := range days {
		if n := len(runs); n > 0 && d <= runs[n-1][1] {
			runs[n-1][1] = max(runs[n-1][1], d+1)
			continue
		}
		runs = append(runs, [2]int{d, d + 1})
	}
	return runs
}

// overallocationMembers tracks which entries occupy one resource on the day
// being examined, moving forward one day at a time. Its buffers are reused
// across resources.
type overallocationMembers struct {
	events []overallocationEvent
	next   int
	active []int // entry indexes
	pos    []int // pos[entry] is the entry's index in active, or -1
}

type overallocationEvent struct {
	day, entry int
	enter      bool
}

func newOverallocationMembers(entries int) *overallocationMembers {
	pos := make([]int, entries)
	for i := range pos {
		pos[i] = -1
	}
	return &overallocationMembers{pos: pos}
}

// start resets the tracker for a resource whose membership runs are given.
func (m *overallocationMembers) start(runs []overallocationRun) {
	for _, e := range m.active {
		m.pos[e] = -1
	}
	m.active = m.active[:0]
	m.events = m.events[:0]
	for _, r := range runs {
		m.events = append(m.events,
			overallocationEvent{day: r.start, entry: r.entry, enter: true},
			overallocationEvent{day: r.end, entry: r.entry})
	}
	// Order within a day does not matter: one entry's runs never share a
	// boundary, and different entries are independent.
	slices.SortFunc(m.events, func(a, b overallocationEvent) int { return a.day - b.day })
	m.next = 0
}

// advanceTo applies every membership change up to and including day.
func (m *overallocationMembers) advanceTo(day int) {
	for ; m.next < len(m.events) && m.events[m.next].day <= day; m.next++ {
		ev := m.events[m.next]
		if ev.enter {
			m.pos[ev.entry] = len(m.active)
			m.active = append(m.active, ev.entry)
			continue
		}
		i := m.pos[ev.entry]
		last := m.active[len(m.active)-1]
		m.active[i] = last
		m.pos[last] = i
		m.active = m.active[:len(m.active)-1]
		m.pos[ev.entry] = -1
	}
}

// sorted returns the current members in entry order.
func (m *overallocationMembers) sorted() []int {
	out := slices.Clone(m.active)
	slices.Sort(out)
	return out
}

// LevelResourcesWithOptions reschedules ES/EF so no resource exceeds capacity,
// using the serial method: tasks become ready when all predecessors
// are levelled, the ready task with the smallest (LS, ID) — i.e. the
// least float, under the default LeastTotalFloat strategy — goes first,
// and each task is placed at the first of these starts where every
// assigned resource has capacity across its whole span: its earliest start
// (the precedence links and its constrained ES), which may fall part-way
// through a day, then each following whole day. Demand counts the share of
// each day used, as DetectOverallocations does, so part-day work can share
// a day (part-day levelling, owner decision 2026-11-21). Splitting, when
// allowed, still books whole days.
//
// Semantics and limits (documented simplifications for this first
// leveling pass):
//
//   - ScheduleCPM is run internally first; its error (a cycle, or a
//     schedule beyond MaxScheduleDays) is returned unchanged.
//   - After leveling, ES/EF are the resource-feasible dates. LS, LF
//     and Float still describe the precedence-only schedule — float
//     analysis of a levelled plan is a later refinement.
//   - Capacities follow DetectOverallocations' convention (missing =
//     1.0). A task whose own demand exceeds capacity on day one of
//     the search is placed at its precedence-earliest start and left
//     flagged rather than pushed past the levelling horizon.
//   - Date constraints: the forward effects of SNET, FNET, MSO and MFO
//     are preserved via the initial CalculateCPM pass (the levelled
//     start never moves earlier than the constrained ES). Levelling does
//     not avoid latest limits while it places tasks, but afterwards it
//     sets ConstraintViolated on any SNLT, FNLT, MSO or MFO task its
//     dates now break. An earlier run's LevelStart is disarmed first.
//     The desktop persists each levelled delay as the task's LevelStart,
//     never as a constraint.
//
// Beyond the capacity plan it accepts LevelingOptions carrying a
// per-schedule Horizon (zero uses DefaultLevelingHorizon) and Strategy
// (empty uses LeastTotalFloat).
//
// Return values:
//   - (LevelingResult{}, ErrSchedulingCycle) if the graph has a cycle.
//   - (LevelingResult{}, ErrScheduleTooLong) if the schedule is longer
//     than MaxScheduleDays. A Horizon above MaxScheduleDays is lowered to it.
//   - (result, ErrLevelingHorizonExceeded) if one or more tasks could not
//     be placed within the horizon. The schedule is still levelled as far as
//     possible and result.UnplacedTaskIDs names the tasks left at their
//     precedence-earliest start. This is a surfaced warning, not a hard
//     failure — the returned schedule is usable; check with errors.Is.
//   - (LevelingResult{}, nil) on a fully levelled schedule.
func LevelResourcesWithOptions(tasks map[string]*Task, plan ResourceCapacityPlan, opts LevelingOptions) (LevelingResult, error) {
	horizon := opts.Horizon
	if horizon <= 0 {
		horizon = DefaultLevelingHorizon
	}
	if horizon > MaxScheduleDays {
		horizon = MaxScheduleDays
	}
	strategy := opts.Strategy
	if strategy == "" {
		strategy = LeastTotalFloat
	}
	// Levelling recomputes the levelled starts from the logic and the
	// user's constraints, so an earlier run's must not hold tasks back.
	for _, t := range tasks {
		t.LevelStartArmed, t.LevelStartDay = false, 0
	}
	if err := ScheduleCPM(tasks); err != nil {
		return LevelingResult{}, err
	}

	// higherPriority reports whether task a should claim a contended slot
	// ahead of task b under the active strategy (ties broken by ID for
	// determinism).
	higherPriority := func(a, b *Task) bool {
		// Priority-override: a critical-path task always outranks a
		// non-critical one, regardless of the base strategy, so leveling
		// never delays the critical path in favour of a floating task.
		if opts.PriorityCritical && a.IsCritical != b.IsCritical {
			return a.IsCritical
		}
		switch strategy {
		case EarliestDeadline:
			if a.LF != b.LF {
				return a.LF < b.LF
			}
			return a.ID < b.ID
		default: // LeastTotalFloat
			if a.LS != b.LS {
				return a.LS < b.LS
			}
			return a.ID < b.ID
		}
	}

	order, _ := topoSort(tasks) // CalculateCPM already proved acyclicity

	// Ready-queue serial scheduling: pick the ready task with the
	// smallest (LS, ID).
	levelled := make(map[string]bool, len(tasks))
	booked := make(map[string][]float64)
	var unplaced []string
	var splitIDs []string

	pending := make([]string, len(order))
	copy(pending, order)

	demand := func(profile []float64, day int) float64 {
		if day < len(profile) {
			return profile[day]
		}
		return 0
	}

	// dayHasCapacity reports whether every assignment of t has room on a
	// single day d.
	dayHasCapacity := func(t *Task, d int) bool {
		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			if demand(booked[a.Resource], d)+a.effectiveUnits() > plan.capacityForAssignment(a, d)+1e-9 {
				return false
			}
		}
		return true
	}

	// fits reports whether t can run contiguously from start (which may
	// fall part-way through a day): on every day its work touches, each
	// assignment's units times the share of the day used must fit, the
	// rule DetectOverallocations applies (spanDays, dayShare).
	fits := func(t *Task, start float64) bool {
		finish := start + t.Duration
		first, last := spanDays(start, finish)
		for d := first; d <= last; d++ {
			share := dayShare(start, finish, d)
			if share <= 0 {
				continue
			}
			for _, a := range t.Assignments {
				if a.Resource == "" {
					continue
				}
				capacity := plan.capacityForAssignment(a, d) + 1e-9
				// Units are a rate: more at once than the capacity never
				// fits, however little of the day is used (peakUnits).
				if a.effectiveUnits() > capacity || demand(booked[a.Resource], d)+a.effectiveUnits()*share > capacity {
					return false
				}
			}
		}
		return true
	}

	// splitDays scans forward from `from`, collecting `need` non-contiguous
	// days (within the horizon) where every assignment has capacity. Returns
	// (days, true) only if it found enough.
	splitDays := func(t *Task, from, need int) ([]int, bool) {
		out := make([]int, 0, need)
		for d := from; d <= from+horizon && len(out) < need; d++ {
			if dayHasCapacity(t, d) {
				out = append(out, d)
			}
		}
		if len(out) < need {
			return nil, false
		}
		return out, true
	}

	// bookProfileDays adds t's demand on each of the given working days.
	bookProfileDays := func(t *Task, workDays []int) {
		if len(workDays) == 0 {
			return
		}
		maxDay := workDays[len(workDays)-1] + 1
		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			profile := booked[a.Resource]
			if len(profile) < maxDay {
				grown := make([]float64, maxDay)
				copy(grown, profile)
				profile = grown
			}
			for _, d := range workDays {
				profile[d] += a.effectiveUnits()
			}
			booked[a.Resource] = profile
		}
	}

	// bookShares adds t's demand for a contiguous run from start: units
	// times the share of each day used.
	bookShares := func(t *Task, start float64) {
		finish := start + t.Duration
		first, last := spanDays(start, finish)
		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			profile := booked[a.Resource]
			if len(profile) < last+1 {
				grown := make([]float64, last+1)
				copy(grown, profile)
				profile = grown
			}
			for d := max(first, 0); d <= last; d++ {
				profile[d] += a.effectiveUnits() * dayShare(start, finish, d)
			}
			booked[a.Resource] = profile
		}
	}

	for len(pending) > 0 {
		// Pick the ready task with the smallest (LS, ID).
		pick := -1
		for i, id := range pending {
			t := tasks[id]
			ready := true
			for _, l := range effectiveLinks(t) {
				if _, exists := tasks[l.Pred]; exists && !levelled[l.Pred] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			if pick == -1 || higherPriority(t, tasks[pending[pick]]) {
				pick = i
			}
		}
		if pick == -1 {
			// Unreachable on an acyclic graph (CalculateCPM already
			// proved acyclicity); defensive.
			return LevelingResult{}, ErrSchedulingCycle
		}
		id := pending[pick]
		pending = append(pending[:pick], pending[pick+1:]...)
		t := tasks[id]

		// Precedence-earliest start against the LEVELLED predecessors,
		// never earlier than the constrained ES from CalculateCPM.
		earliest := t.ES
		for _, l := range effectiveLinks(t) {
			p, exists := tasks[l.Pred]
			if !exists {
				continue
			}
			var candidate float64
			switch l.Type {
			case StartToStart:
				candidate = p.ES + l.Lag
			case FinishToFinish:
				candidate = p.EF + l.Lag - t.Duration
			case StartToFinish:
				candidate = p.ES + l.Lag - t.Duration
			default: // FinishToStart
				candidate = p.EF + l.Lag
			}
			if candidate > earliest {
				earliest = candidate
			}
		}
		if earliest < 0 {
			earliest = 0
		}

		// Part-day levelling (owner decision 2026-11-21): a task is tried
		// at its earliest start, which may fall part-way through a day, and
		// then at each following whole day. Whole-day schedules see the
		// same candidates as before.
		startFloor := int(math.Ceil(earliest - 1e-9))
		start := earliest
		t.WorkDays = nil
		if t.Duration > 0 && len(t.Assignments) > 0 {
			days := int(math.Ceil(t.Duration))

			// Earliest contiguous placement (if any within the horizon).
			contigStart, contigOK := 0.0, false
			if fits(t, earliest) {
				contigStart, contigOK = earliest, true
			}
			for offset := 0; !contigOK && offset <= horizon; offset++ {
				if fits(t, float64(startFloor+offset)) {
					contigStart, contigOK = float64(startFloor+offset), true
				}
			}
			// Earliest split placement (greedy earliest capacity days). Its
			// finish is always <= any contiguous finish, so it only wins when
			// it is genuinely non-contiguous and therefore finishes sooner.
			var splitWD []int
			splitOK := false
			if opts.AllowSplitting {
				splitWD, splitOK = splitDays(t, startFloor, days)
			}

			contigFinish := math.Inf(1)
			if contigOK {
				contigFinish = contigStart + t.Duration
			}
			splitFinish := math.Inf(1)
			if splitOK {
				splitFinish = float64(splitWD[len(splitWD)-1] + 1)
			}

			switch {
			case splitOK && splitFinish < contigFinish:
				// Splitting finishes strictly sooner => the days are
				// non-contiguous; interrupt the task around the busy days.
				start = float64(splitWD[0])
				bookProfileDays(t, splitWD)
				t.WorkDays = append([]int(nil), splitWD...)
				t.EF = float64(splitWD[len(splitWD)-1] + 1)
				splitIDs = append(splitIDs, id)
			case contigOK:
				start = contigStart
				bookShares(t, start)
				t.EF = start + t.Duration
			default:
				// Nothing fit within the horizon (e.g. single-day demand >
				// capacity): leave the task at its earliest start, booked so
				// the overallocation stays visible to DetectOverallocations,
				// and report it unplaced.
				bookShares(t, start)
				t.EF = start + t.Duration
				unplaced = append(unplaced, id)
			}
			t.ES = start
		} else {
			// No resource or no work: nothing to level, so the task keeps
			// its earliest start, part days included.
			t.ES = start
			t.EF = t.ES + t.Duration
		}
		levelled[id] = true
	}

	// Re-check the latest limits against the levelled dates: a delay can
	// push a task past an SNLT, FNLT, MSO or MFO date. Flags set by the
	// precedence pass are kept.
	for _, t := range tasks {
		if !t.ConstraintArmed {
			continue
		}
		late := lateConstraintDay(t)
		switch t.Constraint {
		case StartNoLaterThan, MustStartOn:
			if t.ES > late+1e-9 {
				t.ConstraintViolated = true
			}
		case FinishNoLaterThan, MustFinishOn:
			if t.EF > late+1e-9 {
				t.ConstraintViolated = true
			}
		}
	}

	res := LevelingResult{}
	if len(splitIDs) > 0 {
		sort.Strings(splitIDs)
		res.SplitTaskIDs = splitIDs
	}
	if len(unplaced) > 0 {
		sort.Strings(unplaced)
		res.UnplacedTaskIDs = unplaced
		return res, ErrLevelingHorizonExceeded
	}
	return res, nil
}
