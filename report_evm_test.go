// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"pmforge/internal/db"
	"pmforge/internal/documents"
)

func TestResolvedEVMForChartsComputesCPMReferences(t *testing.T) {
	charts := map[string]documents.ResolvedChart{
		"chart-1": {
			Kind: "cpm",
			Data: `{"nodes":[
				{"id":"A","label":"Design","duration":4,"budgeted_cost":400,"actual_cost":500,"percent_complete":75},
				{"id":"B","label":"Build","duration":4,"budgeted_cost":400}
			],"edges":[{"from":"A","to":"B"}]}`,
		},
	}
	proj := db.Project{StartDate: "2026-06-01", CountryCode: "US"}
	asOf := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	resolved := resolvedEVMForCharts(proj, charts, asOf)

	metrics := resolved["chart-1"]
	if metrics == nil {
		t.Fatal("expected EVM metrics for CPM chart reference")
	}
	if metrics.BAC != 800 || metrics.PV != 500 || metrics.EV != 300 || metrics.AC != 500 {
		t.Fatalf("metrics = %+v, want BAC=800 PV=500 EV=300 AC=500", metrics)
	}
}
