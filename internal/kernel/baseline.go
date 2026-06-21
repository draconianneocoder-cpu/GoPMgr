// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

// ScheduleVariance is the planned-vs-baseline delta for one task.
// Variances are in working days; positive means the current schedule
// is LATER than the baseline (a slip), negative means earlier.
type ScheduleVariance struct {
	TaskID         string  `json:"task_id"`
	BaselineStart  string  `json:"baseline_start,omitempty"`
	BaselineFinish string  `json:"baseline_finish,omitempty"`
	StartVarDays   float64 `json:"start_var_days"`
	FinishVarDays  float64 `json:"finish_var_days"`
}

// CompareSchedules diffs a current schedule against a baseline
// snapshot, keyed by task ID. Both maps should already be scheduled
// (CalculateCPM run; AnchorSchedule too if dates are wanted in the
// result). Tasks present in only one of the two maps are skipped —
// added or removed tasks have no meaningful variance.
func CompareSchedules(current, baseline map[string]*Task) map[string]ScheduleVariance {
	out := make(map[string]ScheduleVariance)
	for id, cur := range current {
		base, ok := baseline[id]
		if !ok {
			continue
		}
		out[id] = ScheduleVariance{
			TaskID:         id,
			BaselineStart:  base.StartDate,
			BaselineFinish: base.FinishDate,
			StartVarDays:   cur.ES - base.ES,
			FinishVarDays:  cur.EF - base.EF,
		}
	}
	return out
}
