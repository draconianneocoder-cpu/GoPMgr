// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"path/filepath"
	"testing"
	"time"

	"gopmgr/internal/calendar"
	"gopmgr/internal/db"
)

// The fixture for every test here: a US project starting Monday 2026-06-01
// (no US holiday falls in the first two weeks of June). A (4 days, budget
// 400) runs Monday to Thursday; B (4 days, budget 400) follows from Friday.
// Planned value through the end of a day, by hand:
//
//	Wed 06-03: 3 of A's 4 days            = 300
//	Thu 06-04: all of A                   = 400
//	Fri 06-05: A + 1 of B's 4 days        = 500
//	Sat 06-06: no working day added       = 500
const evmStatusChart = `{"nodes":[
	{"id":"A","label":"Design","duration":4,"budgeted_cost":400},
	{"id":"B","label":"Build","duration":4,"budgeted_cost":400}],
	"edges":[{"from":"A","to":"B"}]}`

var monday0608 = time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)

func newEVMStatusApp(t *testing.T) (*App, db.Project, db.Chart) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "evm.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	p, err := d.UpsertProject(db.Project{ID: "project-1", Name: "EVM", StartDate: "2026-06-01", CountryCode: "US"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	c, err := d.SaveChart(db.Chart{ProjectID: p.ID, Kind: "cpm", Title: "Plan", Data: evmStatusChart})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	return &App{db: d}, p, c
}

// An entered status date counts through the end of that day
// (docs/review/EVM-STATUS-DATE-INCLUSIVE-OWNER-DECISION.md), and a weekend
// date adds nothing.
func TestComputeScheduleEVMCountsTheStatusDateThroughItsEnd(t *testing.T) {
	app, _, c := newEVMStatusApp(t)
	for date, wantPV := range map[string]float64{
		"2026-06-03": 300,
		"2026-06-04": 400,
		"2026-06-05": 500,
		"2026-06-06": 500,
	} {
		m, err := app.ComputeScheduleEVM(c.ID, date)
		if err != nil {
			t.Fatalf("ComputeScheduleEVM(%s): %v", date, err)
		}
		if m.PV != wantPV || m.StatusDate != date {
			t.Fatalf("status %s: PV %v, status date %q; want PV %v through the end of %s", date, m.PV, m.StatusDate, wantPV, date)
		}
	}
	if _, err := app.ComputeScheduleEVM(c.ID, "06/05/2026"); err == nil {
		t.Fatal("a status date that is not YYYY-MM-DD must be refused")
	}
}

// With no entered date, the status date is the previous working day, and
// the result names it. On Monday 06-08 that is Friday 06-05.
func TestEVMStatusDefaultsToThePreviousWorkingDay(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	day, date, err := evmStatus(start, "", monday0608, calendar.For("US").IsWorkday)
	if err != nil || date != "2026-06-05" || day != 5 {
		t.Fatalf("evmStatus(default on Monday 06-08) = (%v, %q, %v), want (5, 2026-06-05, nil)", day, date, err)
	}
	day, date, err = evmStatus(start, "2026-06-06", monday0608, calendar.For("US").IsWorkday)
	if err != nil || date != "2026-06-06" || day != 5 {
		t.Fatalf("evmStatus(entered Saturday) = (%v, %q, %v), want (5, 2026-06-06, nil)", day, date, err)
	}
}

// A schedule report's summary uses the default status date and names it.
func TestReportEVMNamesItsStatusDate(t *testing.T) {
	_, p, _ := newEVMStatusApp(t)
	tasks, err := cpmChartDataToKernelTasks(evmStatusChart)
	if err != nil {
		t.Fatalf("cpmChartDataToKernelTasks: %v", err)
	}
	if err := scheduleProjectTasks(p, tasks); err != nil {
		t.Fatalf("scheduleProjectTasks: %v", err)
	}
	m, err := reportEVM(p, tasks, monday0608)
	if err != nil || m == nil {
		t.Fatalf("reportEVM = %v, %v", m, err)
	}
	if m.StatusDate != "2026-06-05" || m.PV != 500 {
		t.Fatalf("report EVM: status date %q, PV %v; want 2026-06-05 and 500", m.StatusDate, m.PV)
	}
	p.StartDate = ""
	if m, err := reportEVM(p, tasks, monday0608); m != nil || err != nil {
		t.Fatalf("reportEVM without a project start = %v, %v; want no section", m, err)
	}
}

// The portfolio's status date is the previous UTC calendar day, whatever
// zone the clock reads in.
func TestPortfolioStatusDateIsThePreviousUTCDay(t *testing.T) {
	for now, want := range map[time.Time]string{
		time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC):                         "2026-06-07",
		time.Date(2026, 6, 8, 0, 30, 0, 0, time.FixedZone("+02:00", 2*3600)): "2026-06-06",
		time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC):                         "2026-02-28",
	} {
		if got := portfolioStatusDate(now).Format("2006-01-02"); got != want {
			t.Fatalf("portfolioStatusDate(%s) = %s, want %s", now.Format(time.RFC3339), got, want)
		}
	}
}
