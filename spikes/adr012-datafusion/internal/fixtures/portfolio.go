// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package fixtures generates the DF0 corpus: synthetic portfolio inputs and
// tabular files, each with the logical content the generator wrote, so that
// no engine under test is its own oracle. Every case is deterministic: the
// same code writes the same bytes, and the manifest pins their SHA-256.
package fixtures

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
)

// Class labels a fixture's purpose (DF-A11).
type Class string

const (
	LegacyParity        Class = "legacy_parity"
	PlausibleProduction Class = "plausible_production"
	FutureS7A           Class = "future_s7a"
	AdversarialStress   Class = "adversarial_stress"
)

// Project is one portfolio input row, in engine-neutral form. Floats are kept
// as Go float64 here and written as strconv 'g' strings so NaN and ±Inf
// survive a JSON round trip exactly.
type Project struct {
	ProjectID               string
	Name                    string
	BudgetedCost            float64
	CommittedCost           float64
	ActualCost              float64
	EarnedValue             float64
	PlannedValue            float64
	BudgetedCostMinorUnits  int64
	CommittedCostMinorUnits int64
	ActualCostMinorUnits    int64
	EarnedValueMinorUnits   int64
	PlannedValueMinorUnits  int64
	PercentComplete         float64
	EVMAvailable            bool
}

// PortfolioCase is one PortfolioRollup input.
type PortfolioCase struct {
	ID       string
	Class    Class
	Note     string
	Projects []Project
}

func minor(id string, budget, committed, ac, ev, pv int64, evm bool) Project {
	return Project{
		ProjectID: id, Name: "Project " + id,
		BudgetedCostMinorUnits: budget, CommittedCostMinorUnits: committed,
		ActualCostMinorUnits: ac, EarnedValueMinorUnits: ev, PlannedValueMinorUnits: pv,
		EVMAvailable: evm,
	}
}

// scaled returns n plausible projects from a seeded generator: budgets up to
// 50 million major units, committed and EVM values below budget, about one
// in five without EVM, as RunPortfolioAnalytics produces for projects
// without a start date or a valid schedule.
func scaled(n int, seed uint64) []Project {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	out := make([]Project, n)
	for i := range out {
		budget := r.Int64N(5_000_000_000) + 1
		p := minor(fmt.Sprintf("p%06d", i), budget, r.Int64N(budget+1), 0, 0, 0, r.IntN(5) != 0)
		if p.EVMAvailable {
			p.PlannedValueMinorUnits = r.Int64N(budget + 1)
			p.EarnedValueMinorUnits = r.Int64N(budget + 1)
			p.ActualCostMinorUnits = r.Int64N(budget*2 + 1)
			p.PercentComplete = float64(p.EarnedValueMinorUnits) / float64(budget) * 100
		}
		out[i] = p
	}
	return out
}

// PortfolioCases returns the portfolio corpus (DF-002 to DF-010, DF-054,
// DF-061). Currency and the reporting date are host checks in
// RunPortfolioAnalytics and never reach the engine, so they are covered by
// the host-policy cases, not here.
func PortfolioCases() []PortfolioCase {
	maxI, minI := int64(math.MaxInt64), int64(math.MinInt64)
	cases := []PortfolioCase{
		{ID: "portfolio-empty", Class: LegacyParity, Note: "zero projects: neutral totals, indices n/a"},
		{ID: "portfolio-one", Class: LegacyParity, Projects: []Project{
			minor("a", 1_000_000, 250_000, 40_000, 50_000, 60_000, true)}},
		{ID: "portfolio-evm-mixed", Class: LegacyParity, Note: "EVM sums include only evm_available projects; budget and committed include all",
			Projects: []Project{
				minor("a", 100_00, 10_00, 20_00, 30_00, 40_00, true),
				minor("b", 200_00, 20_00, 999_00, 999_00, 999_00, false),
				minor("c", 300_00, 30_00, 1_00, 2_00, 3_00, true),
			}},
		{ID: "portfolio-evm-zero", Class: LegacyParity, Note: "EVM available with zero AC/EV/PV: both indices n/a (0)",
			Projects: []Project{minor("a", 100_00, 0, 0, 0, 0, true)}},
		{ID: "portfolio-spi-undefined", Class: LegacyParity, Note: "PV sum zero, AC nonzero",
			Projects: []Project{minor("a", 100_00, 0, 50_00, 25_00, 0, true)}},
		{ID: "portfolio-cpi-undefined", Class: LegacyParity, Note: "AC sum zero, PV nonzero",
			Projects: []Project{minor("a", 100_00, 0, 0, 25_00, 50_00, true)}},
		{ID: "portfolio-all-evm-unavailable", Class: LegacyParity,
			Projects: []Project{minor("a", 100_00, 1, 7, 8, 9, false), minor("b", 1, 2, 3, 4, 5, false)}},
		{ID: "portfolio-negative", Class: AdversarialStress, Note: "negative budgets and EVM values are summed as given",
			Projects: []Project{minor("a", -500_00, -1, -2_00, -3_00, -4_00, true), minor("b", 200_00, 5, 1_00, 1_00, 1_00, true)}},
		{ID: "portfolio-large-exact", Class: AdversarialStress, Note: "sums that fit int64 exactly but exceed 2^53",
			Projects: []Project{minor("a", maxI/2, 1<<53+1, 1<<53+3, 1<<53+5, 1<<53+7, true), minor("b", maxI/2, 1, 1, 1, 1, true)}},
		{ID: "portfolio-overflow-positive", Class: AdversarialStress, Note: "budget sum exceeds int64 max",
			Projects: []Project{minor("a", maxI, 0, 0, 0, 0, false), minor("b", 1, 0, 0, 0, 0, false)}},
		{ID: "portfolio-overflow-negative", Class: AdversarialStress, Note: "committed sum below int64 min",
			Projects: []Project{minor("a", 0, minI, 0, 0, 0, false), minor("b", 0, -1, 0, 0, 0, false)}},
		{ID: "portfolio-overflow-evm-excluded", Class: AdversarialStress, Note: "an overflowing AC on an EVM-unavailable project is excluded from the AC sum",
			Projects: []Project{minor("a", 1, 1, maxI, 0, 0, false), minor("b", 1, 1, maxI, 0, 0, false)}},
		{ID: "portfolio-float-fallback", Class: LegacyParity, Note: "minor units zero and float nonzero: legacy converts the float",
			Projects: []Project{{ProjectID: "a", Name: "a", BudgetedCost: 1234.56, CommittedCost: 0.01, ActualCost: 10, EarnedValue: 20.5, PlannedValue: 30.25, EVMAvailable: true}}},
		{ID: "portfolio-minor-preferred", Class: LegacyParity, Note: "nonzero minor units win over a disagreeing float",
			Projects: []Project{{ProjectID: "a", Name: "a", BudgetedCost: 999, BudgetedCostMinorUnits: 100_00, CommittedCost: 1, CommittedCostMinorUnits: 2, EVMAvailable: false}}},
		{ID: "portfolio-zero-ambiguity", Class: LegacyParity, Note: "DF-061: a legitimate zero minor-unit amount with a stale nonzero float is replaced by the float in legacy code",
			Projects: []Project{{ProjectID: "a", Name: "a", BudgetedCost: 5000, BudgetedCostMinorUnits: 0, CommittedCost: 12.34, CommittedCostMinorUnits: 0}}},
		{ID: "portfolio-float-rounding", Class: AdversarialStress, Note: "half-cent floats: math.Round(v*100) rounds half away from zero after binary scaling",
			Projects: []Project{
				{ProjectID: "a", Name: "a", BudgetedCost: 0.005},
				{ProjectID: "b", Name: "b", BudgetedCost: 1.005},
				{ProjectID: "c", Name: "c", BudgetedCost: -0.005},
				{ProjectID: "d", Name: "d", BudgetedCost: 2.675},
				{ProjectID: "e", Name: "e", BudgetedCost: 0.015},
			}},
		{ID: "portfolio-float-nonfinite", Class: AdversarialStress, Note: "NaN and ±Inf floats with zero minor units convert to 0 in legacy code",
			Projects: []Project{{ProjectID: "a", Name: "a", BudgetedCost: math.NaN(), CommittedCost: math.Inf(1), ActualCost: math.Inf(-1), EVMAvailable: true}}},
		{ID: "portfolio-float-saturating", Class: AdversarialStress, Note: "huge floats saturate to int64 bounds in legacy code, then the sum overflows",
			Projects: []Project{{ProjectID: "a", Name: "a", BudgetedCost: 1e300}, {ProjectID: "b", Name: "b", BudgetedCost: 1e300}}},
		{ID: "portfolio-names", Class: AdversarialStress, Note: "names and IDs are not aggregated; Unicode, empty, duplicate IDs must not change totals",
			Projects: []Project{
				{ProjectID: "dup", Name: "Ünïcødé 项目 🚀", BudgetedCostMinorUnits: 1},
				{ProjectID: "dup", Name: "", BudgetedCostMinorUnits: 2},
				{ProjectID: "", Name: "'; DROP TABLE portfolio; --", BudgetedCostMinorUnits: 3},
			}},
	}
	for _, n := range []int{10, 100, 1_000, 10_000, 50_000} {
		class := PlausibleProduction
		if n >= 10_000 {
			class = AdversarialStress
		}
		cases = append(cases, PortfolioCase{
			ID:       "portfolio-scale-" + strconv.Itoa(n),
			Class:    class,
			Note:     "seeded synthetic projects",
			Projects: scaled(n, uint64(n)),
		})
	}
	return cases
}
