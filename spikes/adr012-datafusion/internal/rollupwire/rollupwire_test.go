// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package rollupwire

import (
	"testing"

	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

func TestNormaliseUsesTheLegacyFallbackOnlyForZeroMinor(t *testing.T) {
	tests := []struct {
		name  string
		minor int64
		major float64
		want  string
	}{
		{"minor wins", 12345, 999.99, "12345"},
		{"zero minor, float fallback rounds half away", 0, 2.675, "268"},
		{"both zero", 0, 0, "0"},
		{"negative fallback", 0, -1.005, "-100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalise(fixtures.Project{BudgetedCostMinorUnits: tt.minor, BudgetedCost: tt.major})
			if got["budgeted_cost_minor_units"] != tt.want {
				t.Fatalf("got %v, want %s", got["budgeted_cost_minor_units"], tt.want)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name    string
		in      Result
		wantErr bool
	}{
		{"exact totals", Result{ProjectCount: 2, Totals: []string{"100", "0", "-5", "9223372036854775807", "1"}, SPI: 1.5}, false},
		{"four totals", Result{Totals: []string{"1", "2", "3", "4"}}, true},
		{"not an integer", Result{Totals: []string{"1", "2", "3", "4", "5.0"}}, true},
		{"over int64", Result{Totals: []string{"1", "2", "3", "4", "9223372036854775808"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Summary(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.TotalBudgetedCostMinorUnits != 100 || got.TotalBudgetedCost != 1 || got.TotalActualCost != -0.05 ||
				got.TotalEarnedValueMinorUnits != 9223372036854775807 || got.SchedulePerformanceIndex != 1.5 || got.ProjectCount != 2 {
				t.Fatalf("summary %+v", got)
			}
		})
	}
}
