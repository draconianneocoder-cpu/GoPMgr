// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !duckdb

// Tests for the default (DuckDB-absent) build. The real engine's tests
// live under `//go:build duckdb` (Phase B).
package analytics

import (
	"context"
	"errors"
	"testing"
)

func TestStubReportsUnavailable(t *testing.T) {
	eng := New()
	t.Cleanup(func() {
		if err := eng.Close(); err != nil {
			t.Fatalf("Close on stub returned error: %v", err)
		}
	})

	if eng.Available() {
		t.Fatal("default build: Available() should be false")
	}

	if _, err := eng.PortfolioRollup(context.Background(), nil); !errors.Is(err, ErrAnalyticsUnavailable) {
		t.Fatalf("PortfolioRollup: want ErrAnalyticsUnavailable, got %v", err)
	}

	if _, err := eng.ImportTabular(context.Background(), "data.csv"); !errors.Is(err, ErrAnalyticsUnavailable) {
		t.Fatalf("ImportTabular: want ErrAnalyticsUnavailable, got %v", err)
	}
}
