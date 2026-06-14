// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "sort"

// TaskEV is one task's earned-value breakdown at the status date.
type TaskEV struct {
	TaskID string  `json:"task_id"`
	Title  string  `json:"title"`
	BAC    float64 `json:"bac"`
	PV     float64 `json:"pv"`
	EV     float64 `json:"ev"`
	AC     float64 `json:"ac"`
}

// EVMetrics is the project-level earned-value picture at a status
// date, with the standard derived indicators.
//
// Conventions: SV/CV positive is good (ahead of plan / under cost).
// SPI/CPI are 0 when their denominator is 0 (no work planned yet /
// no cost incurred) — display layers should treat 0 as "n/a".
// EAC uses the classic BAC/CPI formula, falling back to BAC when CPI
// is unavailable.
type EVMetrics struct {
	AsOfDay float64 `json:"as_of_day"`

	BAC float64 `json:"bac"` // budget at completion (Σ task budgets)
	PV  float64 `json:"pv"`  // planned value (BCWS)
	EV  float64 `json:"ev"`  // earned value (BCWP)
	AC  float64 `json:"ac"`  // actual cost (ACWP)

	SV  float64 `json:"sv"`  // schedule variance: EV − PV
	CV  float64 `json:"cv"`  // cost variance:     EV − AC
	SPI float64 `json:"spi"` // schedule performance index: EV / PV
	CPI float64 `json:"cpi"` // cost performance index:     EV / AC

	EAC float64 `json:"eac"` // estimate at completion: BAC / CPI
	ETC float64 `json:"etc"` // estimate to complete:   EAC − AC
	VAC float64 `json:"vac"` // variance at completion: BAC − EAC

	Tasks []TaskEV `json:"tasks"`
}

// ComputeEVM derives earned-value metrics from a scheduled task map
// at a status date expressed as a working-day offset (same indexing
// as ES/EF; convert a calendar date with DayOffset).
//
// Per task: PV = BudgetedCost × planned-fraction-complete at asOfDay
// (linear across the task's ES..EF window; a zero-duration milestone
// is fully planned once asOfDay reaches its ES). EV = BudgetedCost ×
// PercentComplete/100. AC = ActualCost. CalculateCPM must have run
// first so ES/EF are populated; PercentComplete is assumed clamped.
func ComputeEVM(tasks map[string]*Task, asOfDay float64) EVMetrics {
	m := EVMetrics{AsOfDay: asOfDay}

	for _, t := range tasks {
		pv := t.BudgetedCost * plannedFraction(t, asOfDay)
		ev := t.BudgetedCost * t.PercentComplete / 100

		m.BAC += t.BudgetedCost
		m.PV += pv
		m.EV += ev
		m.AC += t.ActualCost

		m.Tasks = append(m.Tasks, TaskEV{
			TaskID: t.ID,
			Title:  t.Title,
			BAC:    t.BudgetedCost,
			PV:     pv,
			EV:     ev,
			AC:     t.ActualCost,
		})
	}
	sort.Slice(m.Tasks, func(i, j int) bool { return m.Tasks[i].TaskID < m.Tasks[j].TaskID })

	m.SV = m.EV - m.PV
	m.CV = m.EV - m.AC
	if m.PV > 0 {
		m.SPI = m.EV / m.PV
	}
	if m.AC > 0 {
		m.CPI = m.EV / m.AC
	}

	if m.CPI > 0 {
		m.EAC = m.BAC / m.CPI
	} else {
		m.EAC = m.BAC
	}
	m.ETC = m.EAC - m.AC
	m.VAC = m.BAC - m.EAC

	return m
}

// plannedFraction is the share of a task's budget planned to be
// complete at the status day, linear across its ES..EF window.
func plannedFraction(t *Task, asOfDay float64) float64 {
	if t.Duration <= 0 {
		if asOfDay >= t.ES {
			return 1
		}
		return 0
	}
	f := (asOfDay - t.ES) / t.Duration
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
