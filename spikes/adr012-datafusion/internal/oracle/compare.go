// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package oracle

import (
	"fmt"
	"math/big"

	"gopmgr/internal/analytics"
)

// CompareSummary checks every field exactly, except the two ratios, which
// the legacy code computes from float64 major units: they must be within
// 2 ULP of the exact ratio's nearest float64 (the documented tolerance).
func CompareSummary(got analytics.PortfolioSummary, want *Summary) []string {
	var out []string
	eqInt := func(name string, g int, w int) {
		if g != w {
			out = append(out, fmt.Sprintf("%s: got %d, want %d", name, g, w))
		}
	}
	eqInt("project_count", got.ProjectCount, want.ProjectCount)
	eqInt("evm_project_count", got.EVMProjectCount, want.EVMProjectCount)
	eqInt("evm_unavailable_project_count", got.EVMUnavailableProjectCount, want.EVMUnavailableProjectCount)
	gm := [5]int64{got.TotalBudgetedCostMinorUnits, got.TotalCommittedCostMinorUnits, got.TotalActualCostMinorUnits, got.TotalEarnedValueMinorUnits, got.TotalPlannedValueMinorUnits}
	gf := [5]float64{got.TotalBudgetedCost, got.TotalCommittedCost, got.TotalActualCost, got.TotalEarnedValue, got.TotalPlannedValue}
	for f, name := range FieldNames {
		if big.NewInt(gm[f]).Cmp(want.Totals[f]) != 0 {
			out = append(out, fmt.Sprintf("%s_minor_units: got %d, want %s", name, gm[f], want.Totals[f]))
		}
		// The display float is minor/100 in float64.
		if wf := float64(want.Totals[f].Int64()) / 100; gf[f] != wf {
			out = append(out, fmt.Sprintf("%s: got %v, want %v", name, gf[f], wf))
		}
	}
	ratio := func(name string, g, w float64, defined bool) {
		if !defined {
			if g != 0 {
				out = append(out, fmt.Sprintf("%s: got %v, want 0 (n/a)", name, g))
			}
			return
		}
		if d := ULPDistance(g, w); d > 2 {
			out = append(out, fmt.Sprintf("%s: got %v, want %v (%d ULP)", name, g, w, d))
		}
	}
	ratio("spi", got.SchedulePerformanceIndex, want.SPI, want.SPIDefined)
	ratio("cpi", got.CostPerformanceIndex, want.CPI, want.CPIDefined)
	return out
}
