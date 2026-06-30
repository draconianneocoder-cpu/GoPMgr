// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"sort"

	"pmforge/internal/money"
)

// TaskEV is one task's earned-value breakdown at the status date.
type TaskEV struct {
	TaskID        string  `json:"task_id"`
	Title         string  `json:"title"`
	BAC           float64 `json:"bac"`
	PV            float64 `json:"pv"`
	EV            float64 `json:"ev"`
	AC            float64 `json:"ac"`
	BACMinorUnits int64   `json:"bac_minor_units"`
	PVMinorUnits  int64   `json:"pv_minor_units"`
	EVMinorUnits  int64   `json:"ev_minor_units"`
	ACMinorUnits  int64   `json:"ac_minor_units"`
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

	BAC           float64 `json:"bac"` // budget at completion (Σ task budgets)
	PV            float64 `json:"pv"`  // planned value (BCWS)
	EV            float64 `json:"ev"`  // earned value (BCWP)
	AC            float64 `json:"ac"`  // actual cost (ACWP)
	BACMinorUnits int64   `json:"bac_minor_units"`
	PVMinorUnits  int64   `json:"pv_minor_units"`
	EVMinorUnits  int64   `json:"ev_minor_units"`
	ACMinorUnits  int64   `json:"ac_minor_units"`

	SV  float64 `json:"sv"`  // schedule variance: EV − PV
	CV  float64 `json:"cv"`  // cost variance:     EV − AC
	SPI float64 `json:"spi"` // schedule performance index: EV / PV
	CPI float64 `json:"cpi"` // cost performance index:     EV / AC

	EAC           float64 `json:"eac"` // estimate at completion: BAC / CPI
	ETC           float64 `json:"etc"` // estimate to complete:   EAC − AC
	VAC           float64 `json:"vac"` // variance at completion: BAC − EAC
	SVMinorUnits  int64   `json:"sv_minor_units"`
	CVMinorUnits  int64   `json:"cv_minor_units"`
	EACMinorUnits int64   `json:"eac_minor_units"`
	ETCMinorUnits int64   `json:"etc_minor_units"`
	VACMinorUnits int64   `json:"vac_minor_units"`

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
		bac := taskBudgetAmount(t)
		ac := taskActualAmount(t)
		pv := money.RateTimesQuantity(bac, plannedFraction(t, asOfDay))
		ev := money.RateTimesQuantity(bac, t.PercentComplete/100)

		m.BACMinorUnits += bac.MinorUnits
		m.PVMinorUnits += pv.MinorUnits
		m.EVMinorUnits += ev.MinorUnits
		m.ACMinorUnits += ac.MinorUnits

		m.Tasks = append(m.Tasks, TaskEV{
			TaskID:        t.ID,
			Title:         t.Title,
			BAC:           bac.MajorFloat(),
			PV:            pv.MajorFloat(),
			EV:            ev.MajorFloat(),
			AC:            ac.MajorFloat(),
			BACMinorUnits: bac.MinorUnits,
			PVMinorUnits:  pv.MinorUnits,
			EVMinorUnits:  ev.MinorUnits,
			ACMinorUnits:  ac.MinorUnits,
		})
	}
	sort.Slice(m.Tasks, func(i, j int) bool { return m.Tasks[i].TaskID < m.Tasks[j].TaskID })

	m.SVMinorUnits = m.EVMinorUnits - m.PVMinorUnits
	m.CVMinorUnits = m.EVMinorUnits - m.ACMinorUnits
	if m.PVMinorUnits > 0 {
		m.SPI = float64(m.EVMinorUnits) / float64(m.PVMinorUnits)
	}
	if m.ACMinorUnits > 0 {
		m.CPI = float64(m.EVMinorUnits) / float64(m.ACMinorUnits)
	}

	if m.EVMinorUnits > 0 && m.ACMinorUnits > 0 {
		m.EACMinorUnits = money.ScaleByRatio(
			money.Amount{MinorUnits: m.BACMinorUnits},
			m.ACMinorUnits,
			m.EVMinorUnits,
		).MinorUnits
	} else {
		m.EACMinorUnits = m.BACMinorUnits
	}
	m.ETCMinorUnits = m.EACMinorUnits - m.ACMinorUnits
	m.VACMinorUnits = m.BACMinorUnits - m.EACMinorUnits

	m.BAC = money.Amount{MinorUnits: m.BACMinorUnits}.MajorFloat()
	m.PV = money.Amount{MinorUnits: m.PVMinorUnits}.MajorFloat()
	m.EV = money.Amount{MinorUnits: m.EVMinorUnits}.MajorFloat()
	m.AC = money.Amount{MinorUnits: m.ACMinorUnits}.MajorFloat()
	m.SV = money.Amount{MinorUnits: m.SVMinorUnits}.MajorFloat()
	m.CV = money.Amount{MinorUnits: m.CVMinorUnits}.MajorFloat()
	m.EAC = money.Amount{MinorUnits: m.EACMinorUnits}.MajorFloat()
	m.ETC = money.Amount{MinorUnits: m.ETCMinorUnits}.MajorFloat()
	m.VAC = money.Amount{MinorUnits: m.VACMinorUnits}.MajorFloat()

	return m
}

func taskBudgetAmount(t *Task) money.Amount {
	if t.BudgetedCostMinorUnits != 0 || t.BudgetedCost == 0 {
		return money.Amount{MinorUnits: t.BudgetedCostMinorUnits}
	}
	return money.FromMajorFloat(t.BudgetedCost)
}

func taskActualAmount(t *Task) money.Amount {
	if t.ActualCostMinorUnits != 0 || t.ActualCost == 0 {
		return money.Amount{MinorUnits: t.ActualCostMinorUnits}
	}
	return money.FromMajorFloat(t.ActualCost)
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
