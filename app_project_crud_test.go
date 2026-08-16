// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gopmgr/internal/calendar"
	"gopmgr/internal/db"
)

// TestAppMethodsRequireOpenProject covers the shared "no project open"
// guard on every thin App wrapper in this file's slice, without a
// per-method test each duplicating the same assertion.
func TestAppMethodsRequireOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"ListDocuments", func() error { _, err := app.ListDocuments("charter"); return err }},
		{"GetDocument", func() error { _, err := app.GetDocument("missing"); return err }},
		{"ListStakeholders", func() error { _, err := app.ListStakeholders(""); return err }},
		{"SaveStakeholder", func() error { _, err := app.SaveStakeholder(db.Stakeholder{}); return err }},
		{"DeleteStakeholder", func() error { return app.DeleteStakeholder("x") }},
		{"ListResourceCalendars", func() error { _, err := app.ListResourceCalendars(); return err }},
		{"SaveResourceCalendar", func() error { _, err := app.SaveResourceCalendar(db.ResourceCalendar{}); return err }},
		{"DeleteResourceCalendar", func() error { return app.DeleteResourceCalendar("x") }},
		{"UpdateProjectIndustry", func() error { _, err := app.UpdateProjectIndustry("", "", "", ""); return err }},
		{"ListHolidays", func() error { _, err := app.ListHolidays("2026-01-01", "2026-12-31"); return err }},
		{"ComputeBudget", func() error { _, err := app.ComputeBudget(); return err }},
		{"BuildTimeline", func() error { _, err := app.BuildTimeline(); return err }},
		{"RepairAndSwap", func() error { _, err := app.RepairAndSwap(); return err }},
		{"PreflightCombinedReport", func() error {
			_, err := app.PreflightCombinedReport(nil, CombinedReportOptions{})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil || err.Error() != "no project open" {
				t.Fatalf("%s: want \"no project open\", got %v", tc.name, err)
			}
		})
	}
}

// TestLaunchpadEvaluateFallsBackWithoutTemplatesEngine covers the only
// branch of LaunchpadEvaluate reachable in this codebase today: a.templates
// is never assigned anywhere outside this nil check, so the a.templates.
// Evaluate call and its error path are dead code under the current wiring,
// not merely untested by this slice.
func TestLaunchpadEvaluateFallsBackWithoutTemplatesEngine(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	seeds, err := app.LaunchpadEvaluate("construction", "waterfall")
	if err != nil {
		t.Fatalf("LaunchpadEvaluate: %v", err)
	}
	if len(seeds) != 1 || seeds[0] != "charter" {
		t.Fatalf("want fallback [\"charter\"], got %v", seeds)
	}
}

func TestListDocumentsAndGetDocument(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	created, err := app.NewDocument("brief", "")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	docs, err := app.ListDocuments("brief")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != created.ID {
		t.Fatalf("want one charter document matching %q, got %v", created.ID, docs)
	}

	got, err := app.GetDocument(created.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetDocument returned %q, want %q", got.ID, created.ID)
	}

	if _, err := app.GetDocument("does-not-exist"); err == nil {
		t.Fatal("GetDocument: want error for unknown id, got nil")
	}
}

func TestStakeholderCRUDFiltersByCategory(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	team, err := app.SaveStakeholder(db.Stakeholder{Name: "Dana", Category: db.StakeholderTeam})
	if err != nil {
		t.Fatalf("SaveStakeholder team: %v", err)
	}
	if team.ProjectID == "" {
		t.Fatal("SaveStakeholder: want ProjectID filled in from the open project, got empty")
	}
	if _, err := app.SaveStakeholder(db.Stakeholder{Name: "Vendor Co", Category: db.StakeholderVendor}); err != nil {
		t.Fatalf("SaveStakeholder vendor: %v", err)
	}

	all, err := app.ListStakeholders("")
	if err != nil {
		t.Fatalf("ListStakeholders(all): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 stakeholders, got %d", len(all))
	}

	teamOnly, err := app.ListStakeholders(string(db.StakeholderTeam))
	if err != nil {
		t.Fatalf("ListStakeholders(team): %v", err)
	}
	if len(teamOnly) != 1 || teamOnly[0].ID != team.ID {
		t.Fatalf("want only %q filtered by category, got %v", team.ID, teamOnly)
	}

	if err := app.DeleteStakeholder(team.ID); err != nil {
		t.Fatalf("DeleteStakeholder: %v", err)
	}
	remaining, err := app.ListStakeholders("")
	if err != nil {
		t.Fatalf("ListStakeholders(after delete): %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("want 1 stakeholder after delete, got %d", len(remaining))
	}
}

func TestResourceCalendarCRUD(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	saved, err := app.SaveResourceCalendar(db.ResourceCalendar{Resource: "dana", Name: "Dana", DefaultCapacity: 1})
	if err != nil {
		t.Fatalf("SaveResourceCalendar: %v", err)
	}
	if saved.ProjectID == "" {
		t.Fatal("SaveResourceCalendar: want ProjectID filled in from the open project, got empty")
	}

	list, err := app.ListResourceCalendars()
	if err != nil {
		t.Fatalf("ListResourceCalendars: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("want one resource calendar matching %q, got %v", saved.ID, list)
	}

	if err := app.DeleteResourceCalendar(saved.ID); err != nil {
		t.Fatalf("DeleteResourceCalendar: %v", err)
	}
	after, err := app.ListResourceCalendars()
	if err != nil {
		t.Fatalf("ListResourceCalendars(after delete): %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("want no resource calendars after delete, got %d", len(after))
	}
}

// TestUpdateProjectIndustry covers both sides of the invalid-timezone
// replacement guard. A fresh project's empty TimeZone is always valid
// (ValidTimeZone treats "" as valid), so the happy-path case only exercises
// the "no replacement needed" side. To reach the calendar.DefaultTimeZone
// replacement branch, a timezone that's valid for one country but not
// another is set via UpdateProjectMeta (which independently rejects an
// outright-invalid timezone, so a bare garbage string can't be used here),
// then UpdateProjectIndustry is called with the other country — leaving a
// now-mismatched TimeZone for UpdateProjectIndustry's own check to catch.
func TestUpdateProjectIndustry(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	updated, err := app.UpdateProjectIndustry("construction", "civil", "waterfall", "US")
	if err != nil {
		t.Fatalf("UpdateProjectIndustry: %v", err)
	}
	if updated.Industry != "construction" || updated.SubCategory != "civil" || updated.Methodology != "waterfall" {
		t.Fatalf("fields not set as requested: %+v", updated)
	}
	if updated.TimeZone == "" {
		t.Fatal("UpdateProjectIndustry: want a non-empty TimeZone on a US project, got empty")
	}

	proj, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	proj.CountryCode = "US"
	proj.TimeZone = "America/New_York"
	if _, err := app.UpdateProjectMeta(proj); err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
	}

	replaced, err := app.UpdateProjectIndustry("construction", "civil", "waterfall", "GB")
	if err != nil {
		t.Fatalf("UpdateProjectIndustry (mismatched tz): %v", err)
	}
	want := calendar.DefaultTimeZone("GB")
	if replaced.TimeZone != want {
		t.Fatalf("want US timezone replaced with GB default %q on country change, got %q", want, replaced.TimeZone)
	}
}

func TestListHolidays(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	if _, err := app.UpdateProjectIndustry("", "", "", "US"); err != nil {
		t.Fatalf("UpdateProjectIndustry: %v", err)
	}

	events, err := app.ListHolidays("2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatalf("ListHolidays: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("want at least one US holiday in 2026, got none")
	}

	if _, err := app.ListHolidays("not-a-date", "2026-12-31"); err == nil {
		t.Fatal("ListHolidays: want error for malformed from date, got nil")
	}
	if _, err := app.ListHolidays("2026-01-01", "not-a-date"); err == nil {
		t.Fatal("ListHolidays: want error for malformed to date, got nil")
	}
}

func TestComputeBudgetIncludesStakeholderContractValue(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	if _, err := app.SaveStakeholder(db.Stakeholder{
		Name:                    "Vendor Co",
		Category:                db.StakeholderVendor,
		ContractValueMinorUnits: 500_00,
	}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}

	summary, err := app.ComputeBudget()
	if err != nil {
		t.Fatalf("ComputeBudget: %v", err)
	}
	if summary.CommittedMinorUnits != 500_00 {
		t.Fatalf("want committed cost 50000 minor units, got %d", summary.CommittedMinorUnits)
	}
}

// TestPreflightCombinedReport covers only the App-layer wrapper's own four
// statements (requireDB, nil guard, delegation, return) — reporting.
// Service.Preflight's actual preflight logic (missing documents/charts,
// profile selection) is already covered at the reporting package level
// (internal/reporting/reporting_test.go), so this doesn't re-test that.
func TestPreflightCombinedReport(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	preflight, err := app.PreflightCombinedReport(nil, CombinedReportOptions{})
	if err != nil {
		t.Fatalf("PreflightCombinedReport: %v", err)
	}
	if preflight.Profile.ID == "" {
		t.Fatal("PreflightCombinedReport: want a resolved profile, got empty ID")
	}
}

func TestBuildTimelineOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	if _, err := app.BuildTimeline(); err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
}
