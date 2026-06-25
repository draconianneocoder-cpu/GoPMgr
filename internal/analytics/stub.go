// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !duckdb

// This file is compiled when the `duckdb` tag is absent. Production/package
// builds set that tag and use duckdb.go; this stub remains for explicit
// no-DuckDB developer builds and tests.
package analytics

import "context"

// New returns the no-op analytics engine used in untagged builds.
func New() Engine { return stubEngine{} }

// stubEngine satisfies Engine but reports unavailability for every
// capability, so callers can degrade gracefully.
type stubEngine struct{}

func (stubEngine) PortfolioRollup(context.Context, []ProjectMetrics) (PortfolioSummary, error) {
	return PortfolioSummary{}, ErrAnalyticsUnavailable
}

func (stubEngine) ImportTabular(context.Context, string) (Dataset, error) {
	return Dataset{}, ErrAnalyticsUnavailable
}

func (stubEngine) Available() bool { return false }

func (stubEngine) Close() error { return nil }
