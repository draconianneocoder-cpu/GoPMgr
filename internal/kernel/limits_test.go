// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestScheduleCPMRefusesValuesBeyondTheLimit(t *testing.T) {
	t.Parallel()
	const over = MaxScheduleDays + 1
	cases := []struct {
		name   string
		tasks  map[string]*Task
		detail string
	}{
		{"duration", map[string]*Task{"A": {ID: "A", Title: "Pour slab", Duration: over}}, `task "Pour slab" has a duration`},
		{"duration of a billion days", map[string]*Task{"A": {ID: "A", Duration: 1e9}}, `task "A" has a duration`},
		{"negative duration", map[string]*Task{"A": {ID: "A", Duration: -over}}, "duration"},
		{"NaN duration", map[string]*Task{"A": {ID: "A", Duration: math.NaN()}}, "duration"},
		{"infinite duration", map[string]*Task{"A": {ID: "A", Duration: math.Inf(1)}}, "duration"},
		{"lag", map[string]*Task{
			"A": {ID: "A", Duration: 1},
			"B": {ID: "B", Duration: 1, Links: []Link{{Pred: "A", Lag: over}}},
		}, `task "B" has a lag`},
		{"lead", map[string]*Task{
			"A": {ID: "A", Duration: 1},
			"B": {ID: "B", Duration: 1, Links: []Link{{Pred: "A", Lag: -over}}},
		}, "lag"},
		{"pessimistic estimate", map[string]*Task{
			"A": {ID: "A", Duration: 1, DurationEstimate: DurationEstimate{Optimistic: 1, MostLikely: 2, Pessimistic: over}},
		}, "duration estimate"},
		{"chain whose parts are each within the limit", map[string]*Task{
			"A": {ID: "A", Duration: MaxScheduleDays},
			"B": {ID: "B", Duration: 1, Precedents: []string{"A"}},
		}, `task "B" would finish on working day 18251`},
		{"constraint date beyond the limit", map[string]*Task{
			"A": {ID: "A", Duration: 1, Constraint: StartNoEarlierThan, ConstraintArmed: true, ConstraintDay: MaxScheduleDays},
		}, "would finish"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := ScheduleCPM(c.tasks)
			if !errors.Is(err, ErrScheduleTooLong) {
				t.Fatalf("ScheduleCPM = %v, want ErrScheduleTooLong", err)
			}
			if !strings.Contains(err.Error(), c.detail) {
				t.Errorf("error %q does not say %q", err, c.detail)
			}
			for id, task := range c.tasks {
				if task.ES != 0 || task.EF != 0 || task.LS != 0 || task.LF != 0 || task.Float != 0 || task.IsCritical {
					t.Errorf("task %s keeps CPM results after a refusal: %+v", id, *task)
				}
			}
			if CalculateCPM(c.tasks) {
				t.Error("CalculateCPM = true, want false")
			}
		})
	}
}

func TestScheduleCPMAcceptsAScheduleExactlyAtTheLimit(t *testing.T) {
	t.Parallel()
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: MaxScheduleDays - 1},
		"B": {ID: "B", Duration: 1, Links: []Link{{Pred: "A", Lag: -MaxScheduleDays}, {Pred: "A"}}},
	}
	if err := ScheduleCPM(tasks); err != nil {
		t.Fatalf("ScheduleCPM = %v, want nil", err)
	}
	if got := tasks["B"].EF; got != MaxScheduleDays {
		t.Errorf("B.EF = %v, want %d", got, MaxScheduleDays)
	}
	AnchorSchedule(tasks, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	if got, want := tasks["B"].FinishDate, "2075-12-19"; got != want {
		t.Errorf("B.FinishDate = %q, want %q", got, want)
	}
}

func TestScheduleCPMStillReportsACycle(t *testing.T) {
	t.Parallel()
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1, Precedents: []string{"B"}},
		"B": {ID: "B", Duration: 1, Precedents: []string{"A"}},
	}
	if err := ScheduleCPM(tasks); !errors.Is(err, ErrSchedulingCycle) {
		t.Fatalf("ScheduleCPM = %v, want ErrSchedulingCycle", err)
	}
}

// The functions that size slices from a schedule must not trust ES and EF:
// baselines and tests hand them tasks that never went through CalculateCPM.
// Each check is fatal so that a lost guard fails on the smallest case
// instead of going on to allocate for the largest.
func TestDaySizedAllocationsRefuseOutOfRangeDates(t *testing.T) {
	t.Parallel()
	for _, ef := range []float64{maxAllocationDays + 1, 1e9, 1e300, math.Inf(1), math.NaN()} {
		tasks := map[string]*Task{
			"near": {ID: "near", Duration: 1, ES: 0, EF: 1, StartDate: "stale", FinishDate: "stale", Assignments: []Assignment{{Resource: "r"}}},
			"far":  {ID: "far", Duration: 1, ES: 0, EF: ef, StartDate: "stale", FinishDate: "stale", Assignments: []Assignment{{Resource: "r"}}},
		}
		AnchorSchedule(tasks, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil)
		for id, task := range tasks {
			if task.StartDate != "" || task.FinishDate != "" {
				t.Fatalf("EF %v: task %s has dates %q..%q, want none", ef, id, task.StartDate, task.FinishDate)
			}
		}
		if got := len(ResourceUsage(tasks)["r"]); got != 1 {
			t.Fatalf("EF %v: usage profile has %d days, want 1 (the in-range task only)", ef, got)
		}
		if got := occupiedDays(tasks["far"]); got != nil {
			t.Fatalf("EF %v: occupiedDays returned %d days, want none", ef, len(got))
		}
	}
}

func TestLevelingRefusesAnOverlongScheduleAndCapsItsHorizon(t *testing.T) {
	t.Parallel()
	long := map[string]*Task{"A": {ID: "A", Duration: 1e9, Assignments: []Assignment{{Resource: "r"}}}}
	if _, err := LevelResourcesWithOptions(long, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{}); !errors.Is(err, ErrScheduleTooLong) {
		t.Fatalf("leveling an over-long schedule = %v, want ErrScheduleTooLong", err)
	}

	// The resource is free on one day only, further out than the capped
	// horizon reaches. Honouring the caller's larger horizon would place
	// the task there, past what AnchorSchedule will build dates for.
	const freeDay = MaxScheduleDays + 500
	idle := map[int]float64{0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0}
	plan := ResourceCapacityPlan{DefaultCapacity: 1, Calendars: map[string]ResourceCalendar{
		"r": {WeeklyCapacity: idle, Overrides: map[int]float64{freeDay: 1}},
	}}
	far := map[string]*Task{"A": {ID: "A", Duration: 1, Assignments: []Assignment{{Resource: "r"}}}}
	res, err := LevelResourcesWithOptions(far, plan, LevelingOptions{Horizon: 2 * MaxScheduleDays})
	if !errors.Is(err, ErrLevelingHorizonExceeded) || len(res.UnplacedTaskIDs) != 1 || far["A"].ES != 0 {
		t.Fatalf("leveling = %+v, %v with A.ES = %v; want A unplaced at day 0 and ErrLevelingHorizonExceeded", res, err, far["A"].ES)
	}
}

func TestMonteCarloRefusesAnOverlongEstimate(t *testing.T) {
	t.Parallel()
	tasks := map[string]*Task{"A": {ID: "A", Duration: 1, DurationEstimate: DurationEstimate{
		Optimistic: 1, MostLikely: 2, Pessimistic: 1e9, Distribution: MonteCarloTriangular}}}
	res := RunMonteCarlo(tasks, 10, 1)
	if res.Valid || !strings.Contains(res.Error, ErrScheduleTooLong.Error()) {
		t.Fatalf("RunMonteCarlo = valid %v, error %q; want the schedule-length refusal", res.Valid, res.Error)
	}
}
