// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package oracle

import (
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"testing"

	"gopmgr/internal/money"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

func TestLegacyMinor(t *testing.T) {
	tests := []struct {
		name              string
		minor             int64
		major             float64
		want              int64
		fallback, invalid bool
	}{
		{"minor units win", 12_34, 999, 12_34, false, false},
		{"both zero", 0, 0, 0, false, false},
		{"float fallback", 0, 12.34, 12_34, true, false},
		{"half away from zero", 0, 0.005, 1, true, false},
		{"negative half away from zero", 0, -0.005, -1, true, false},
		{"binary 1.005 rounds down", 0, 1.005, 100, true, false},
		// 2.675 is stored as 2.67499999999999982..., but the float64 product
		// 2.675*100 rounds to exactly 267.5, a tie, so the legacy rule gives
		// 268 where exact decimal rounding of the stored value gives 267.
		{"binary 2.675 product ties up", 0, 2.675, 268, true, false},
		{"NaN becomes zero", 0, math.NaN(), 0, true, true},
		{"+Inf becomes zero", 0, math.Inf(1), 0, true, true},
		{"saturates high", 0, 1e300, math.MaxInt64, true, true},
		{"saturates low", 0, -1e300, math.MinInt64, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, fb, inv := legacyMinor(tt.minor, tt.major)
			if got != tt.want || fb != tt.fallback || inv != tt.invalid {
				t.Fatalf("legacyMinor(%d, %v) = %d, %v, %v; want %d, %v, %v", tt.minor, tt.major, got, fb, inv, tt.want, tt.fallback, tt.invalid)
			}
		})
	}
}

func p(id string, budget, committed, ac, ev, pv int64, evm bool) fixtures.Project {
	return fixtures.Project{ProjectID: id, BudgetedCostMinorUnits: budget, CommittedCostMinorUnits: committed,
		ActualCostMinorUnits: ac, EarnedValueMinorUnits: ev, PlannedValueMinorUnits: pv, EVMAvailable: evm}
}

// Hand-computed expectations, so the oracle itself is checked.
func TestRollup(t *testing.T) {
	out, corr := Rollup([]fixtures.Project{
		p("a", 100_00, 10_00, 20_00, 30_00, 40_00, true),
		p("b", 200_00, 20_00, 999_00, 999_00, 999_00, false),
		p("c", 300_00, 30_00, 1_00, 2_00, 3_00, true),
	})
	if out.Err != "" || !corr.Exact() {
		t.Fatalf("unexpected err %q or inexact %+v", out.Err, corr)
	}
	s := out.Summary
	wantTotals := []int64{600_00, 60_00, 21_00, 32_00, 43_00}
	for f, w := range wantTotals {
		if s.Totals[f].Cmp(big.NewInt(w)) != 0 {
			t.Errorf("%s = %s, want %d", FieldNames[f], s.Totals[f], w)
		}
	}
	if s.ProjectCount != 3 || s.EVMProjectCount != 2 || s.EVMUnavailableProjectCount != 1 {
		t.Errorf("counts = %d/%d/%d", s.ProjectCount, s.EVMProjectCount, s.EVMUnavailableProjectCount)
	}
	if s.SPI != 32.0/43.0 || s.CPI != 32.0/21.0 || !s.SPIDefined || !s.CPIDefined {
		t.Errorf("SPI %v CPI %v", s.SPI, s.CPI)
	}
}

func TestRollupEdges(t *testing.T) {
	empty, _ := Rollup(nil)
	if empty.Summary == nil || empty.Summary.ProjectCount != 0 || empty.Summary.SPIDefined || empty.Summary.SPI != 0 {
		t.Fatalf("empty: %+v", empty.Summary)
	}
	over, _ := Rollup([]fixtures.Project{p("a", math.MaxInt64, 0, 0, 0, 0, false), p("b", 1, 0, 0, 0, 0, false)})
	if over.Err != "overflow" {
		t.Fatalf("overflow not detected: %+v", over)
	}
	excluded, _ := Rollup([]fixtures.Project{p("a", 1, 1, math.MaxInt64, 0, 0, false), p("b", 1, 1, math.MaxInt64, 0, 0, false)})
	if excluded.Err != "" {
		t.Fatalf("an EVM-unavailable AC must not count toward overflow: %+v", excluded)
	}
	_, corr := Rollup([]fixtures.Project{{ProjectID: "a", BudgetedCost: 5000, CommittedCost: math.NaN(), ActualCost: 7}})
	if !reflect.DeepEqual(corr.Ambiguous, []string{"0.budgeted_cost"}) || !reflect.DeepEqual(corr.Invalid, []string{"0.committed_cost"}) {
		t.Fatalf("correctness = %+v (actual_cost is excluded: EVM unavailable)", corr)
	}
}

func TestULPDistance(t *testing.T) {
	if ULPDistance(1, 1) != 0 || ULPDistance(1, math.Nextafter(1, 2)) != 1 || ULPDistance(math.NaN(), 1) != math.MaxUint64 {
		t.Fatal("ULPDistance")
	}
	if ULPDistance(-0.0, 0) != 0 {
		t.Fatal("±0 are equal")
	}
}

// The legacy rule is written from its specification, not by calling the
// production code; this differential test proves the two agree.
func TestLegacyMinorMatchesProduction(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	edge := []float64{0.005, -0.005, 1.005, 2.675, 0.015, 1e17, -1e17, 9.2e16, math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64, 92233720368547758.07}
	for i := 0; i < 200_000; i++ {
		var v float64
		switch {
		case i < len(edge):
			v = edge[i]
		case i%3 == 0:
			v = math.Float64frombits(r.Uint64()) // any bit pattern, NaN and Inf included
		case i%3 == 1:
			v = float64(r.Int64N(1_000_000_000_00)) / 100 * float64(1-2*r.IntN(2))
		default:
			v = (r.Float64() - 0.5) * math.Pow(10, float64(r.IntN(25)))
		}
		got, _, _ := legacyMinor(0, v)
		want := money.FromMajorFloat(v).MinorUnits
		if v == 0 {
			want = 0
		}
		if got != want {
			t.Fatalf("legacyMinor(0, %v) = %d, money.FromMajorFloat = %d", v, got, want)
		}
	}
}
