// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"testing"

	"gopmgr/internal/charts"
	"gopmgr/internal/db"
)

func TestChartAppMethodsRejectForeignProjectRowsInSameFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")

	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	openProject, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE project SET start_date = '2026-01-05' WHERE id = ?`, openProject.ID); err != nil {
		t.Fatalf("set open-project start date: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}

	seedForeignChart := func(t *testing.T, suffix string) db.Chart {
		t.Helper()
		chart, err := d.SaveChart(db.Chart{
			ID:        "chart-foreign-" + suffix,
			ProjectID: "prj-foreign",
			Kind:      string(charts.KindCPM),
			Title:     "Foreign Chart",
			Data:      `{}`,
			Config:    `{}`,
		})
		if err != nil {
			t.Fatalf("seed foreign chart: %v", err)
		}
		return chart
	}

	cases := []struct {
		name string
		call func(db.Chart) error
	}{
		{"GetChart", func(c db.Chart) error { _, err := app.GetChart(c.ID); return err }},
		{"SaveChart", func(c db.Chart) error { _, err := app.SaveChart(c); return err }},
		{"LayoutChart", func(c db.Chart) error { _, err := app.LayoutChart(c.ID); return err }},
		{"SetScheduleBaseline", func(c db.Chart) error { _, err := app.SetScheduleBaseline(c.ID, "x"); return err }},
		{"ListScheduleBaselines", func(c db.Chart) error { _, err := app.ListScheduleBaselines(c.ID); return err }},
		{"CompareScheduleBaseline", func(c db.Chart) error { _, err := app.CompareScheduleBaseline(c.ID, ""); return err }},
		{"ComputeScheduleEVM", func(c db.Chart) error { _, err := app.ComputeScheduleEVM(c.ID, ""); return err }},
		{"RunChartMonteCarlo", func(c db.Chart) error { _, err := app.RunChartMonteCarlo(c.ID, 10, 1); return err }},
		{"ExportChartMonteCarloRiskReport", func(c db.Chart) error {
			_, err := app.ExportChartMonteCarloRiskReport(c.ID, 10, 1)
			return err
		}},
		{"LevelChartResources", func(c db.Chart) error { _, err := app.LevelChartResources(c.ID, "", false, false); return err }},
		{"PreviewSplitLeveling", func(c db.Chart) error { _, err := app.PreviewSplitLeveling(c.ID); return err }},
		{"GenerateResourceHistogram", func(c db.Chart) error { _, err := app.GenerateResourceHistogram(c.ID); return err }},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			foreign := seedForeignChart(t, fmt.Sprintf("%02d", i))
			if err := tc.call(foreign); !errors.Is(err, db.ErrNoChart) {
				t.Fatalf("%s(foreign chart) error = %v, want db.ErrNoChart", tc.name, err)
			}
			if _, err := d.GetChart(foreign.ID); err != nil {
				t.Fatalf("%s mutated or deleted foreign chart: %v", tc.name, err)
			}
		})
	}

	foreign := seedForeignChart(t, "delete")
	if err := app.DeleteChart(foreign.ID); err != nil {
		t.Fatalf("DeleteChart(foreign chart) error = %v, want nil no-op", err)
	}
	if _, err := d.GetChart(foreign.ID); err != nil {
		t.Fatalf("DeleteChart mutated or deleted foreign chart: %v", err)
	}
	if err := app.DeleteChart("chart-missing"); err != nil {
		t.Fatalf("DeleteChart(missing chart) error = %v, want nil no-op", err)
	}
}

func TestSaveChartBindsNewChartToOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")

	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	openProject, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}

	saved, err := app.SaveChart(db.Chart{
		ProjectID: "prj-foreign",
		Kind:      string(charts.KindLine),
		Title:     "Owned Chart",
		Data:      `{}`,
		Config:    `{}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if saved.ProjectID != openProject.ID {
		t.Fatalf("SaveChart ProjectID = %q, want open project %q", saved.ProjectID, openProject.ID)
	}
}

func TestScheduleBaselineAppMethodsRejectForeignOrWrongChartBaseline(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")

	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	openProject, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}

	chartA, err := d.SaveChart(db.Chart{ProjectID: openProject.ID, Kind: string(charts.KindCPM), Data: `{}`, Config: `{}`})
	if err != nil {
		t.Fatalf("seed chart A: %v", err)
	}
	chartB, err := d.SaveChart(db.Chart{ProjectID: openProject.ID, Kind: string(charts.KindCPM), Data: `{}`, Config: `{}`})
	if err != nil {
		t.Fatalf("seed chart B: %v", err)
	}
	foreign, err := d.SaveBaseline(db.Baseline{ProjectID: "prj-foreign", ChartID: chartA.ID, Data: `{}`})
	if err != nil {
		t.Fatalf("seed foreign baseline: %v", err)
	}
	wrongChart, err := d.SaveBaseline(db.Baseline{ProjectID: openProject.ID, ChartID: chartB.ID, Data: `{}`})
	if err != nil {
		t.Fatalf("seed wrong-chart baseline: %v", err)
	}

	if err := app.DeleteScheduleBaseline(foreign.ID); err != nil {
		t.Fatalf("DeleteScheduleBaseline(foreign) error = %v, want nil no-op", err)
	}
	if listed, err := app.ListScheduleBaselines(chartA.ID); err != nil {
		t.Fatalf("ListScheduleBaselines: %v", err)
	} else if len(listed) != 0 {
		t.Fatalf("ListScheduleBaselines exposed foreign baseline: %+v", listed)
	}
	if compared, err := app.CompareScheduleBaseline(chartA.ID, ""); err != nil {
		t.Fatalf("CompareScheduleBaseline(newest): %v", err)
	} else if len(compared) != 0 {
		t.Fatalf("CompareScheduleBaseline(newest) used foreign baseline: %+v", compared)
	}
	if _, err := app.CompareScheduleBaseline(chartA.ID, foreign.ID); !errors.Is(err, db.ErrNoBaseline) {
		t.Fatalf("CompareScheduleBaseline(foreign) error = %v, want db.ErrNoBaseline", err)
	}
	if _, err := app.CompareScheduleBaseline(chartA.ID, wrongChart.ID); !errors.Is(err, db.ErrNoBaseline) {
		t.Fatalf("CompareScheduleBaseline(wrong chart) error = %v, want db.ErrNoBaseline", err)
	}
	if _, err := d.GetBaseline(foreign.ID); err != nil {
		t.Fatalf("foreign baseline was mutated or deleted: %v", err)
	}
	if err := app.DeleteScheduleBaseline("baseline-missing"); err != nil {
		t.Fatalf("DeleteScheduleBaseline(missing) error = %v, want nil no-op", err)
	}
}
