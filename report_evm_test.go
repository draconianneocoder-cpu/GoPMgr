// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/money"
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

	resolved, err := resolvedEVMForCharts(proj, charts, asOf)
	if err != nil {
		t.Fatal(err)
	}

	metrics := resolved["chart-1"]
	if metrics == nil {
		t.Fatal("expected EVM metrics for CPM chart reference")
		return
	}
	if metrics.BAC != 800 || metrics.PV != 500 || metrics.EV != 300 || metrics.AC != 500 {
		t.Fatalf("metrics = %+v, want BAC=800 PV=500 EV=300 AC=500", metrics)
	}
}

func TestResolvedEVMForChartsReportsOverflow(t *testing.T) {
	charts := map[string]documents.ResolvedChart{
		"z-last": {
			Kind: "cpm",
			Data: `{"nodes":[
				{"id":"A","duration":1,"budgeted_cost_minor_units":9223372036854775807},
				{"id":"B","duration":1,"budgeted_cost_minor_units":1}
			],"edges":[]}`,
		},
		"a-first": {
			Kind: "cpm",
			Data: `{"nodes":[
				{"id":"C","duration":1,"budgeted_cost_minor_units":9223372036854775807},
				{"id":"D","duration":1,"budgeted_cost_minor_units":1}
			],"edges":[]}`,
		},
	}
	proj := db.Project{StartDate: "2026-06-01", CountryCode: "US"}

	_, err := resolvedEVMForCharts(proj, charts, time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("resolvedEVMForCharts error = %v, want ErrOverflow", err)
	}
	if !strings.Contains(err.Error(), `"a-first"`) {
		t.Fatalf("resolvedEVMForCharts error = %v, want lexical first chart", err)
	}
}
