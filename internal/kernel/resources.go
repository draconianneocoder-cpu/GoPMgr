// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"math"
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
}

// LevelingResult reports the outcome of a resource-leveling pass.
type LevelingResult struct {
	// UnplacedTaskIDs lists tasks whose demand never fit within the
	// horizon; each was left at its precedence-earliest start and remains
	// flagged for the caller. Sorted for deterministic output. Empty on a
	// fully levelled schedule.
	UnplacedTaskIDs []string `json:"unplaced_task_ids,omitempty"`
}

// taskSpan is the inclusive integer day range a task occupies, using
// the same convention as AnchorSchedule (start = round(ES), last day
// = ceil(EF)-1; zero-duration tasks occupy no days).
func taskSpan(t *Task) (first, last int, occupies bool) {
	if t.Duration <= 0 {
		return 0, 0, false
	}
	first = int(math.Round(t.ES))
	last = int(math.Ceil(t.EF)) - 1
	if last < first {
		last = first
	}
	return first, last, true
}

// ResourceUsage builds each resource's per-day demand profile from a
// scheduled task map (CalculateCPM must have run). The slice index is
// the working-day offset; the value is the summed assignment units of
// every task occupying that day. All profiles share the same length
// (the project's last occupied day + 1).
func ResourceUsage(tasks map[string]*Task) map[string][]float64 {
	horizon := 0
	for _, t := range tasks {
		if _, last, ok := taskSpan(t); ok && last+1 > horizon {
			horizon = last + 1
		}
	}

	usage := make(map[string][]float64)
	for _, t := range tasks {
		first, last, ok := taskSpan(t)
		if !ok {
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
			for d := first; d <= last && d < len(profile); d++ {
				profile[d] += a.effectiveUnits()
			}
		}
	}
	return usage
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

	resources := make([]string, 0, len(usage))
	for r := range usage {
		resources = append(resources, r)
	}
	sort.Strings(resources)

	var out []Overallocation
	for _, r := range resources {
		for day, demand := range usage[r] {
			capacity := plan.capacityFor(r, day)
			if demand <= capacity+1e-9 {
				continue
			}
			breach := Overallocation{
				Resource: r,
				Day:      day,
				Demand:   demand,
				Capacity: capacity,
			}
			for _, t := range tasksOnDay(tasks, r, day) {
				breach.TaskIDs = append(breach.TaskIDs, t.ID)
				t.Overallocated = true
			}
			out = append(out, breach)
		}
	}
	return out
}

// tasksOnDay returns the tasks assigned to resource r that occupy the
// given day, sorted by ID for determinism.
func tasksOnDay(tasks map[string]*Task, r string, day int) []*Task {
	var out []*Task
	for _, t := range tasks {
		first, last, ok := taskSpan(t)
		if !ok || day < first || day > last {
			continue
		}
		for _, a := range t.Assignments {
			if a.Resource == r {
				out = append(out, t)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LevelResources reschedules ES/EF so no resource exceeds capacity,
// using the serial method: tasks become ready when all predecessors
// are levelled, the ready task with the smallest (LS, ID) — i.e. the
// least float — goes first, and each task is delayed to the earliest
// integer start where its precedence links are satisfied and every
// assigned resource has capacity across its whole span.
//
// Semantics and limits (documented simplifications for this first
// leveling pass):
//
//   - CalculateCPM is run internally first; it returns false on a
//     cycle and LevelResources propagates that.
//   - After leveling, ES/EF are the resource-feasible dates. LS, LF
//     and Float still describe the precedence-only schedule — float
//     analysis of a levelled plan is a later refinement.
//   - Capacities follow DetectOverallocations' convention (missing =
//     1.0). A task whose own demand exceeds capacity on day one of
//     the search is placed at its precedence-earliest start and left
//     flagged rather than pushed past the levelling horizon.
//   - Date constraints: SNET/MFO forward effects are preserved via
//     the initial CalculateCPM pass (the levelled start never moves
//     earlier than the constrained ES).
func LevelResources(tasks map[string]*Task, capacities map[string]float64) bool {
	return LevelResourcesWithPlan(tasks, capacityPlanFromMap(capacities))
}

// LevelResourcesWithPlan reschedules ES/EF using calendar-aware resource
// capacities and the default leveling horizon. It returns false only on a
// dependency cycle; a horizon overflow (some tasks unplaceable) still
// returns true, with those tasks left at their earliest start and visible
// to DetectOverallocations — preserving the original silent-cap behaviour
// for existing callers. Callers that need the horizon outcome or a custom
// per-schedule horizon should use LevelResourcesWithOptions.
func LevelResourcesWithPlan(tasks map[string]*Task, plan ResourceCapacityPlan) bool {
	_, err := LevelResourcesWithOptions(tasks, plan, LevelingOptions{})
	return !errors.Is(err, ErrSchedulingCycle)
}

// LevelResourcesWithOptions is the full serial resource-leveling entry
// point. See LevelResources for the leveling semantics. Beyond the capacity
// plan it accepts LevelingOptions carrying a per-schedule Horizon (zero uses
// DefaultLevelingHorizon) and Strategy (empty uses LeastTotalFloat).
//
// Return values:
//   - (LevelingResult{}, ErrSchedulingCycle) if the graph has a cycle.
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
	strategy := opts.Strategy
	if strategy == "" {
		strategy = LeastTotalFloat
	}
	if !CalculateCPM(tasks) {
		return LevelingResult{}, ErrSchedulingCycle
	}

	// higherPriority reports whether task a should claim a contended slot
	// ahead of task b under the active strategy (ties broken by ID for
	// determinism).
	higherPriority := func(a, b *Task) bool {
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

	pending := make([]string, len(order))
	copy(pending, order)

	demand := func(profile []float64, day int) float64 {
		if day < len(profile) {
			return profile[day]
		}
		return 0
	}

	fits := func(t *Task, start int) bool {
		days := int(math.Ceil(t.Duration))
		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			profile := booked[a.Resource]
			for d := start; d < start+days; d++ {
				capacity := plan.capacityForAssignment(a, d)
				if demand(profile, d)+a.effectiveUnits() > capacity+1e-9 {
					return false
				}
			}
		}
		return true
	}

	book := func(t *Task, start int) {
		days := int(math.Ceil(t.Duration))
		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			profile := booked[a.Resource]
			if len(profile) < start+days {
				grown := make([]float64, start+days)
				copy(grown, profile)
				profile = grown
			}
			for d := start; d < start+days; d++ {
				profile[d] += a.effectiveUnits()
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

		start := int(math.Ceil(earliest - 1e-9))
		if t.Duration > 0 && len(t.Assignments) > 0 {
			placed := false
			for offset := 0; offset <= horizon; offset++ {
				if fits(t, start+offset) {
					start += offset
					placed = true
					break
				}
			}
			if !placed {
				// Demand never fit within the horizon (e.g. units >
				// capacity): leave the task at its earliest start. The
				// overallocation stays visible to DetectOverallocations
				// and the task is reported in UnplacedTaskIDs.
				unplaced = append(unplaced, id)
			}
		}

		t.ES = float64(start)
		t.EF = t.ES + t.Duration
		if t.Duration > 0 && len(t.Assignments) > 0 {
			book(t, start)
		}
		levelled[id] = true
	}

	if len(unplaced) > 0 {
		sort.Strings(unplaced)
		return LevelingResult{UnplacedTaskIDs: unplaced}, ErrLevelingHorizonExceeded
	}
	return LevelingResult{}, nil
}
