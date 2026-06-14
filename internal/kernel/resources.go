// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"sort"
)

// Assignment binds a task to a named resource at the given units
// (1.0 = full-time). Zero or negative units are treated as 1.0 so a
// bare {"resource":"alice"} assignment behaves sensibly.
type Assignment struct {
	Resource string  `json:"resource"`
	Units    float64 `json:"units,omitempty"`
}

func (a Assignment) effectiveUnits() float64 {
	if a.Units <= 0 {
		return 1
	}
	return a.Units
}

// levelingHorizon caps how far LevelResources will push a task while
// searching for capacity, preventing an infinite walk when demand can
// never fit (e.g. units larger than capacity).
const levelingHorizon = 10000

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
		capacity := 1.0
		if c, ok := capacities[r]; ok && c > 0 {
			capacity = c
		}
		for day, demand := range usage[r] {
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
	if !CalculateCPM(tasks) {
		return false
	}

	order, _ := topoSort(tasks) // CalculateCPM already proved acyclicity

	// Ready-queue serial scheduling: pick the ready task with the
	// smallest (LS, ID).
	levelled := make(map[string]bool, len(tasks))
	booked := make(map[string][]float64)

	pending := make([]string, len(order))
	copy(pending, order)

	capacityFor := func(r string) float64 {
		if c, ok := capacities[r]; ok && c > 0 {
			return c
		}
		return 1.0
	}

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
			capacity := capacityFor(a.Resource)
			for d := start; d < start+days; d++ {
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
			if pick == -1 ||
				t.LS < tasks[pending[pick]].LS ||
				(t.LS == tasks[pending[pick]].LS && t.ID < tasks[pending[pick]].ID) {
				pick = i
			}
		}
		if pick == -1 {
			return false // unreachable on an acyclic graph; defensive
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
			for offset := 0; offset <= levelingHorizon; offset++ {
				if fits(t, start+offset) {
					start += offset
					break
				}
				if offset == levelingHorizon {
					// Demand can never fit (e.g. units > capacity):
					// leave the task at its earliest start; the
					// overallocation stays visible to the caller.
					break
				}
			}
		}

		t.ES = float64(start)
		t.EF = t.ES + t.Duration
		if t.Duration > 0 && len(t.Assignments) > 0 {
			book(t, start)
		}
		levelled[id] = true
	}

	return true
}
