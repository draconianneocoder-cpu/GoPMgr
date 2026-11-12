// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/kernel"
)

// A schedule with one task of a billion days: before the 50-year limit,
// anchoring it built a 24 GB date table.
const overlongChartData = `{"nodes":[{"id":"A","label":"Pour slab","duration":1000000000,` +
	`"assignments":[{"resource":"crew","units":1}]}],"edges":[]}`

func scheduleLimitApp(t *testing.T) (*App, *db.Database) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "schedule-limit.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Limit", StartDate: "2026-01-05"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	return &App{db: d}, d
}

func assertScheduleTooLong(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, kernel.ErrScheduleTooLong) {
		t.Fatalf("%s = %v, want kernel.ErrScheduleTooLong", what, err)
	}
	msg := err.Error()
	for _, want := range []string{"limit of 18250 working days (at least 50 years)", "shorten the task durations or lags", `task "Pour slab" has a duration of 1e+09 days`} {
		if !strings.Contains(msg, want) {
			t.Errorf("%s message %q does not say %q", what, msg, want)
		}
	}
	if strings.Contains(msg, "kernel:") || strings.Contains(msg, "schema") {
		t.Errorf("%s message %q exposes internal wording", what, msg)
	}
}

func TestSaveChartRejectsAnOverlongSchedule(t *testing.T) {
	app, d := scheduleLimitApp(t)
	for _, kind := range []string{"cpm", "gantt"} {
		_, err := app.SaveChart(db.Chart{Kind: kind, Title: "Too long", Data: overlongChartData})
		assertScheduleTooLong(t, "SaveChart("+kind+")", err)
		if saved, err := d.ListCharts("project-1", kind); err != nil || len(saved) != 0 {
			t.Fatalf("after refused save: %d %s charts stored (err %v), want 0", len(saved), kind, err)
		}
	}
}

func TestImportRefusesAnOverlongScheduleAndStoresNothing(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "schedule-limit-import.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Import Target"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}

	// 8e9 hours is a billion 8-hour days.
	xml := strings.Replace(sampleImportXML, "<Duration>PT16H0M0S</Duration>", "<Duration>PT8000000000H0M0S</Duration>", 1)
	if xml == sampleImportXML {
		t.Fatal("sample XML no longer has the PT16H0M0S duration this test rewrites")
	}
	_, err = app.importMSPDIFromBytesWithOptions([]byte(xml), allMSPDIFields)
	if !errors.Is(err, kernel.ErrScheduleTooLong) || !strings.Contains(err.Error(), "this file was not imported") {
		t.Fatalf("import = %v, want a not-imported kernel.ErrScheduleTooLong", err)
	}
	if saved, err := d.ListCharts("project-1", "cpm"); err != nil || len(saved) != 0 {
		t.Fatalf("after refused import: %d charts stored (err %v), want 0", len(saved), err)
	}
	proj, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.StartDate != "" {
		t.Errorf("refused import still set the project start date to %q", proj.StartDate)
	}
}

// A chart stored before the limit existed, or arriving in a restored
// backup, bypasses SaveChart. Every reader of it must stay bounded.
func TestStoredOverlongChartIsRefusedByEveryReader(t *testing.T) {
	app, d := scheduleLimitApp(t)
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Legacy", Data: overlongChartData})
	if err != nil {
		t.Fatalf("store chart directly: %v", err)
	}

	if got, err := app.GetChart(c.ID); err != nil || got.Data != overlongChartData {
		t.Fatalf("GetChart = %v, want the stored data so the editor can correct it", err)
	}
	_, err = app.LayoutChart(c.ID)
	assertScheduleTooLong(t, "LayoutChart", err)
	_, err = app.LevelChartResources(c.ID, "", false, false)
	assertScheduleTooLong(t, "LevelChartResources", err)
	if _, err := app.RunChartMonteCarlo(c.ID, 10, 1); err == nil || !strings.Contains(err.Error(), "longer than 18250 working days") {
		t.Errorf("RunChartMonteCarlo = %v, want the schedule-length refusal", err)
	}
	_, err = app.GenerateResourceHistogram(c.ID)
	assertScheduleTooLong(t, "GenerateResourceHistogram", err)
	_, err = app.SetScheduleBaseline(c.ID, "b")
	assertScheduleTooLong(t, "SetScheduleBaseline", err)
	_, err = app.ComputeScheduleEVM(c.ID, "2026-01-06")
	assertScheduleTooLong(t, "ComputeScheduleEVM", err)
}

// Each task is within the limit but the chain is not. The scheduler clears
// every date, and earned value computed from cleared dates would count the
// successor as planned from day one. Reports must refuse instead.
func TestReportsRefuseAChainBeyondTheLimit(t *testing.T) {
	app, d := scheduleLimitApp(t)
	const chain = `{"nodes":[{"id":"A","label":"Long lead","duration":18250},` +
		`{"id":"B","label":"Fit out","duration":1,"budgeted_cost":100}],"edges":[{"from":"A","to":"B"}]}`
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Chain", Data: chain})
	if err != nil {
		t.Fatalf("store chart directly: %v", err)
	}
	refusals := map[string]func() error{
		"ComputeScheduleEVM":  func() error { _, err := app.ComputeScheduleEVM(c.ID, "2026-01-06"); return err },
		"SetScheduleBaseline": func() error { _, err := app.SetScheduleBaseline(c.ID, "b"); return err },
		"LayoutChart":         func() error { _, err := app.LayoutChart(c.ID); return err },
		"scheduleProjectTasks": func() error {
			tasks, err := cpmChartDataToKernelTasks(chain)
			if err != nil {
				return err
			}
			proj, err := d.GetProject()
			if err != nil {
				return err
			}
			return scheduleProjectTasks(proj, tasks)
		},
	}
	for name, call := range refusals {
		err := call()
		if !errors.Is(err, kernel.ErrScheduleTooLong) || !strings.Contains(err.Error(), `task "Fit out" would finish on working day 18251`) {
			t.Errorf("%s = %v, want the schedule-length refusal naming the task", name, err)
		}
	}
}

func TestReportsRefuseACyclicSchedule(t *testing.T) {
	app, d := scheduleLimitApp(t)
	const cyclic = `{"nodes":[{"id":"A","label":"A","duration":1,"budgeted_cost":100},{"id":"B","label":"B","duration":1}],` +
		`"edges":[{"from":"A","to":"B"},{"from":"B","to":"A"}]}`
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Cyclic", Data: cyclic})
	if err != nil {
		t.Fatalf("store chart directly: %v", err)
	}
	if _, err := app.ComputeScheduleEVM(c.ID, "2026-01-06"); !errors.Is(err, errScheduleCycle) {
		t.Errorf("ComputeScheduleEVM = %v, want the dependency-cycle refusal", err)
	}
	if _, err := app.SetScheduleBaseline(c.ID, "b"); !errors.Is(err, errScheduleCycle) {
		t.Errorf("SetScheduleBaseline = %v, want the dependency-cycle refusal", err)
	}
}
