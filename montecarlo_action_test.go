// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/db"
	"pmforge/internal/users"
)

func nearMonteCarlo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func newMonteCarloTestApp(t *testing.T) (*App, *db.Database, db.Chart) {
	t.Helper()

	d, err := db.InitDB(filepath.Join(t.TempDir(), "montecarlo.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	proj, err := d.UpsertProject(db.Project{
		ID:        "project-1",
		Name:      "Monte Carlo Test",
		StartDate: "2026-06-01",
	})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	chart, err := d.SaveChart(db.Chart{
		ProjectID: proj.ID,
		Kind:      "cpm",
		Title:     "Risk Schedule",
		Data: `{
			"nodes": [
				{
					"id": "A",
					"label": "Design",
					"duration": 2,
					"duration_estimate": {
						"optimistic": 1,
						"most_likely": 2,
						"pessimistic": 4,
						"distribution": "triangular"
					}
				},
				{
					"id": "B",
					"label": "Build",
					"duration": 3,
					"duration_estimate": {
						"optimistic": 2,
						"most_likely": 3,
						"pessimistic": 6,
						"distribution": "beta-pert"
					}
				}
			],
			"edges": [{"from":"A","to":"B"}]
		}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	dataDir := t.TempDir()
	return &App{
		db:   d,
		user: &users.Account{Username: "alice", DataDir: dataDir},
	}, d, chart
}

func TestRunChartMonteCarloReturnsRiskMetrics(t *testing.T) {
	app, _, chart := newMonteCarloTestApp(t)

	result, err := app.RunChartMonteCarlo(chart.ID, 200, 4)
	if err != nil {
		t.Fatalf("RunChartMonteCarlo: %v", err)
	}
	if !result.Valid {
		t.Fatalf("RunChartMonteCarlo invalid: %s", result.Error)
	}
	if result.Iterations != 200 {
		t.Fatalf("Iterations = %d, want 200", result.Iterations)
	}
	if result.P50 <= 0 || result.P80 < result.P50 || result.P90 < result.P80 {
		t.Fatalf("invalid percentile ordering: P50=%v P80=%v P90=%v", result.P50, result.P80, result.P90)
	}
	if result.P90 <= 5 {
		t.Fatalf("P90 = %v, want estimate-driven finish above deterministic 5d", result.P90)
	}
	if len(result.FinishCDF) != 21 {
		t.Fatalf("FinishCDF points = %d, want 21", len(result.FinishCDF))
	}
	if result.FinishCDF[0].Probability != 0 || result.FinishCDF[len(result.FinishCDF)-1].Probability != 1 {
		t.Fatalf("FinishCDF endpoints = %+v / %+v, want probability 0 / 1", result.FinishCDF[0], result.FinishCDF[len(result.FinishCDF)-1])
	}
	if got := result.CriticalPathFrequency["A"]; got != 1 {
		t.Fatalf("A critical frequency = %v, want 1", got)
	}
	if got := result.CriticalPathFrequency["B"]; got != 1 {
		t.Fatalf("B critical frequency = %v, want 1", got)
	}
	if got := result.DurationPercentiles["A"]; got[0] <= 0 || got[1] < got[0] || got[2] < got[1] {
		t.Fatalf("A duration percentiles not ordered: %v", got)
	} else if got[2] <= 2 {
		t.Fatalf("A P90 duration = %v, want estimate-driven value above deterministic 2d", got[2])
	}
}

func TestRunChartMonteCarloIsWorkerStable(t *testing.T) {
	app, _, chart := newMonteCarloTestApp(t)

	serial, err := app.RunChartMonteCarlo(chart.ID, 150, 1)
	if err != nil {
		t.Fatalf("RunChartMonteCarlo serial: %v", err)
	}
	parallel, err := app.RunChartMonteCarlo(chart.ID, 150, 6)
	if err != nil {
		t.Fatalf("RunChartMonteCarlo parallel: %v", err)
	}

	nearMonteCarlo(t, "P50", parallel.P50, serial.P50)
	nearMonteCarlo(t, "P80", parallel.P80, serial.P80)
	nearMonteCarlo(t, "P90", parallel.P90, serial.P90)
}

func TestRunChartMonteCarloRejectsNonCPMChart(t *testing.T) {
	app, d, _ := newMonteCarloTestApp(t)
	chart, err := d.SaveChart(db.Chart{
		ProjectID: "project-1",
		Kind:      "wbs",
		Title:     "Wrong Kind",
		Data:      `{"nodes":[],"edges":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	if _, err := app.RunChartMonteCarlo(chart.ID, 50, 1); err == nil {
		t.Fatal("RunChartMonteCarlo accepted a non-CPM chart")
	}
}

func TestExportChartMonteCarloRiskReportWritesPrivatePDF(t *testing.T) {
	app, _, chart := newMonteCarloTestApp(t)

	outPath, err := app.ExportChartMonteCarloRiskReport(chart.ID, 200, 3)
	if err != nil {
		t.Fatalf("ExportChartMonteCarloRiskReport: %v", err)
	}
	if !strings.HasSuffix(outPath, ".pdf") {
		t.Fatalf("report path = %q, want .pdf", outPath)
	}
	if filepath.Base(outPath) == ".pdf" {
		t.Fatalf("report path has no filename: %q", outPath)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat report: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("report mode = %v, want 0600", got)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-")) {
		t.Fatalf("exported report is not PDF")
	}
	if !bytes.Contains(raw, []byte("<pdfaid:part>3</pdfaid:part>")) {
		t.Fatalf("exported report missing PDF/A metadata")
	}
}
