// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunPortfolioAnalyticsRequiresOneReportingCurrency(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	app.ctx = context.Background()
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := app.CreateProject("USD project", ""); err != nil {
		t.Fatalf("CreateProject(USD): %v", err)
	}
	eur, err := app.CreateProject("EUR project", "")
	if err != nil {
		t.Fatalf("CreateProject(EUR): %v", err)
	}
	setPortfolioProjectCurrency(t, app, eur.Path, "EUR")

	_, err = app.RunPortfolioAnalytics()
	if err == nil || !strings.Contains(err.Error(), "EUR and USD") || !strings.Contains(err.Error(), "foreign exchange conversion is not implemented") {
		t.Fatalf("RunPortfolioAnalytics mixed currencies error = %v", err)
	}

	setPortfolioProjectCurrency(t, app, eur.Path, "USD")
	got, err := app.RunPortfolioAnalytics()
	if err != nil {
		t.Fatalf("RunPortfolioAnalytics USD-only: %v", err)
	}
	if got.ProjectCount != 2 || got.CurrencyCode != "USD" {
		t.Fatalf("USD-only rollup = %+v, want two USD projects", got)
	}
	if got.TotalBudgetedCost != "0.00" || got.TotalCommittedCost != "0.00" {
		t.Fatalf("USD-only money = budget %q committed %q, want 0.00/0.00", got.TotalBudgetedCost, got.TotalCommittedCost)
	}
}

func setPortfolioProjectCurrency(t *testing.T, app *App, path, currency string) {
	t.Helper()
	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject(%s): %v", path, err)
	}
	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	meta.CurrencyCode = currency
	if _, err := app.UpdateProjectMeta(meta); err != nil {
		t.Fatalf("UpdateProjectMeta(%s): %v", currency, err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
}
