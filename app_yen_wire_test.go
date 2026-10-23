// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gopmgr/internal/analytics"
	"gopmgr/internal/budget"
	"gopmgr/internal/db"
)

// Every screen that shows a project's amounts reads yen as whole numbers and
// other currencies with two decimals, from the same stored hundredths
// (ADR-009).
func TestWiresShowYenAsWholeNumbers(t *testing.T) {
	for _, c := range []struct {
		currency, whole, fraction string
	}{
		{"JPY", "1500000", "1234.56"},
		{"USD", "1500000.00", "1234.56"},
	} {
		t.Run(c.currency, func(t *testing.T) {
			meta := projectMetaWire(db.Project{CurrencyCode: c.currency, BudgetMinorUnits: 150_000_000})
			if meta.Budget != c.whole {
				t.Errorf("project budget = %q, want %q", meta.Budget, c.whole)
			}

			rollup := budgetSummaryWire(budget.Summary{
				BudgetMinorUnits:     150_000_000,
				CommittedMinorUnits:  123_456,
				ByCategoryMinorUnits: map[string]int64{"vendor": 123_456},
			}, c.currency)
			if rollup.Budget != c.whole || rollup.Committed != c.fraction || rollup.ByCategory["vendor"] != c.fraction {
				t.Errorf("budget rollup = %+v, want budget %q and committed %q", rollup, c.whole, c.fraction)
			}

			portfolio, err := portfolioSummaryWire(analytics.PortfolioSummary{
				TotalBudgetedCostMinorUnits:  150_000_000,
				TotalCommittedCostMinorUnits: 0,
			}, c.currency)
			if err != nil {
				t.Fatal(err)
			}
			if portfolio.TotalBudgetedCost != c.whole || portfolio.Remaining != c.whole {
				t.Errorf("portfolio = %+v, want %q", portfolio, c.whole)
			}
		})
	}
}
