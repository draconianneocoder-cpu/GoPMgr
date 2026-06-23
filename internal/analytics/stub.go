// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !duckdb

// This file is compiled into the DEFAULT build (no `duckdb` tag). It
// links zero DuckDB code, so the standard desktop download carries no
// extra binary weight. The DuckDB-backed New() lives in duckdb.go under
// `//go:build duckdb` (Phase B) and replaces this one when the tag is set.
package analytics

import "context"

// New returns the no-op analytics engine used in default builds.
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
