// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"sort"
)

// Frozen legacy overallocation detector (design PS04-RES-002, oracle OR-02).
//
// This file is a behavioural copy of the detection path in resources.go at
// git blob 0826dfc62755a0b7eb50c06680c59650809edad2 (main 28a758d). It is the
// compatibility reference a faster detector must match, quirks included; it
// is not a statement that every legacy behaviour is correct.
//
// Rules for this file:
//   - Do not edit it to follow production. If production must change what it
//     returns, that is a contract decision, recorded with its own tests.
//   - It must not call any function or method declared in non-test kernel
//     files; TestLegacyReferenceIsSeparateFromProduction enforces that.
//     Production methods (effectiveUnits, capacityFor, ...) are copied here
//     as free functions for that reason.

func legacyEffectiveUnits(a Assignment) float64 {
	units := a.Units
	if a.Units <= 0 {
		units = 1
	}
	if a.MaxUnits > 0 && units > a.MaxUnits {
		return a.MaxUnits
	}
	return units
}

func legacyBaseCapacity(p ResourceCapacityPlan, resource string) float64 {
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

func legacyCapacityFor(p ResourceCapacityPlan, resource string, day int) float64 {
	capacity := legacyBaseCapacity(p, resource)
	if cal, ok := p.Calendars[resource]; ok {
		capacity = legacyCapacityFromCalendar(cal, capacity, day)
	}
	return capacity
}

func legacyCapacityFromCalendar(cal ResourceCalendar, fallback float64, day int) float64 {
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

func legacyOccupiedDays(t *Task) []int {
	if len(t.WorkDays) > 0 {
		return t.WorkDays
	}
	first, last, ok := legacyTaskSpan(t)
	if !ok {
		return nil
	}
	out := make([]int, 0, last-first+1)
	for d := first; d <= last; d++ {
		out = append(out, d)
	}
	return out
}

func legacyOccupiesDay(t *Task, day int) bool {
	if len(t.WorkDays) > 0 {
		for _, d := range t.WorkDays {
			if d == day {
				return true
			}
		}
		return false
	}
	first, last, ok := legacyTaskSpan(t)
	return ok && day >= first && day <= last
}

func legacyTaskSpan(t *Task) (first, last int, occupies bool) {
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

func legacyResourceUsage(tasks map[string]*Task) map[string][]float64 {
	horizon := 0
	for _, t := range tasks {
		if _, last, ok := legacyTaskSpan(t); ok && last+1 > horizon {
			horizon = last + 1
		}
	}

	usage := make(map[string][]float64)
	for _, t := range tasks {
		od := legacyOccupiedDays(t)
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
					profile[d] += legacyEffectiveUnits(a)
				}
			}
		}
	}
	return usage
}

func legacyDetectOverallocationsWithPlan(tasks map[string]*Task, plan ResourceCapacityPlan) []Overallocation {
	for _, t := range tasks {
		t.Overallocated = false
	}

	usage := legacyResourceUsage(tasks)

	resources := make([]string, 0, len(usage))
	for r := range usage {
		resources = append(resources, r)
	}
	sort.Strings(resources)

	var out []Overallocation
	for _, r := range resources {
		for day, demand := range usage[r] {
			capacity := legacyCapacityFor(plan, r, day)
			if demand <= capacity+1e-9 {
				continue
			}
			breach := Overallocation{
				Resource: r,
				Day:      day,
				Demand:   demand,
				Capacity: capacity,
			}
			for _, t := range legacyTasksOnDay(tasks, r, day) {
				breach.TaskIDs = append(breach.TaskIDs, t.ID)
				t.Overallocated = true
			}
			out = append(out, breach)
		}
	}
	return out
}

func legacyTasksOnDay(tasks map[string]*Task, r string, day int) []*Task {
	var out []*Task
	for _, t := range tasks {
		if !legacyOccupiesDay(t, day) {
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
