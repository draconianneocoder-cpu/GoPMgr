// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"strings"
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

// TestUpdateProjectMetaReclassifiesProject covers the Project Settings save
// path for classification and country: the fields persist, an empty time
// zone takes the country's default, and a time zone the new country's
// business-calendar policy does not support is rejected without changing
// the stored project.
func TestUpdateProjectMetaReclassifiesProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	meta.Industry, meta.SubCategory, meta.Methodology = "construction", "civil", "waterfall"
	meta.CountryCode, meta.TimeZone = "US", ""
	updated, err := app.UpdateProjectMeta(meta)
	if err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
	}
	if updated.Industry != "construction" || updated.SubCategory != "civil" || updated.Methodology != "waterfall" {
		t.Fatalf("classification not saved: %+v", updated)
	}
	if want := calendar.DefaultTimeZone("US"); updated.TimeZone != want {
		t.Fatalf("empty time zone = %q after save, want the US default %q", updated.TimeZone, want)
	}

	moved := updated
	moved.CountryCode, moved.TimeZone = "GB", "America/New_York"
	if _, err := app.UpdateProjectMeta(moved); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("UpdateProjectMeta(GB with a US time zone) error = %v, want an unsupported time zone", err)
	}
	stored, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	if stored.CountryCode != "US" || stored.TimeZone != updated.TimeZone {
		t.Fatalf("rejected save changed the project: country %q, time zone %q", stored.CountryCode, stored.TimeZone)
	}
}

func TestListHolidays(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	meta.CountryCode, meta.TimeZone = "US", ""
	if _, err := app.UpdateProjectMeta(meta); err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
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
	if summary.Committed != "500.00" {
		t.Fatalf("want committed cost 500.00, got %q", summary.Committed)
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
