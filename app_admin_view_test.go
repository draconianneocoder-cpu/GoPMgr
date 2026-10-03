// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

// viewerFixtureApp returns an app signed in as alice with access open to
// bob, who has one project with a schedule, a cost entry, and a document,
// and that project's path.
func viewerFixtureApp(t *testing.T) (*App, string) {
	t.Helper()
	app, path := adminAccessApp(t)
	switchUser(t, app, "bob")
	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("bob opens his project: %v", err)
	}
	if _, err := app.db.Conn.Exec(`UPDATE project SET start_date = '2026-10-05'`); err != nil {
		t.Fatalf("set start date: %v", err)
	}
	proj, err := app.db.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	for _, c := range []db.Chart{
		{ProjectID: proj.ID, Kind: "gantt", Title: "Delivery", Data: `{"nodes":[
			{"id":"b","label":"Build","duration":5,"percent_complete":40},
			{"id":"a","label":"Design","duration":3,"percent_complete":100},
			{"id":"m","label":"Handover","duration":0,"milestone":true}],
			"edges":[{"from":"a","to":"b"},{"from":"b","to":"m"}]}`},
		{ProjectID: proj.ID, Kind: "cpm", Title: "Broken", Data: `{"nodes":[],"edges":[]}`},
		{ProjectID: proj.ID, Kind: "wbs", Title: "Not a schedule"},
	} {
		if _, err := app.SaveChart(c); err != nil {
			t.Fatalf("SaveChart(%s): %v", c.Title, err)
		}
	}
	// A schedule damaged outside GoPMgr (SaveChart refuses invalid data).
	if _, err := app.db.Conn.Exec(`UPDATE charts SET data = '{"nodes":"not a list of tasks"}' WHERE title = 'Broken'`); err != nil {
		t.Fatalf("damage a schedule: %v", err)
	}
	types, err := app.ListCostTypes()
	if err != nil || len(types) == 0 {
		t.Fatalf("ListCostTypes = %v, %v", types, err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-10-06", Description: "Rebar", Amount: "500.00", SupplierName: "Acme Steel"}); err != nil {
		t.Fatalf("SaveCostEntry: %v", err)
	}
	doc, err := app.NewDocument("charter_word", "Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	doc.Content = `{"body":"the secret terms"}`
	if _, err := app.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	switchUser(t, app, "alice")
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	if _, err := app.AdminViewUserProject(path); err != nil {
		t.Fatalf("AdminViewUserProject: %v", err)
	}
	return app, path
}

func TestAdminViewShowsTheScheduleInOrderWithDates(t *testing.T) {
	app, _ := viewerFixtureApp(t)
	schedules, err := app.AdminViewSchedule()
	if err != nil {
		t.Fatalf("AdminViewSchedule: %v", err)
	}
	if len(schedules) != 2 {
		t.Fatalf("schedules = %+v; want the Gantt chart and the unreadable CPM chart, not the WBS", schedules)
	}
	byTitle := map[string]AdminViewScheduleWire{}
	for _, s := range schedules {
		byTitle[s.Title] = s
	}
	delivery := byTitle["Delivery"]
	var titles []string
	for _, task := range delivery.Tasks {
		titles = append(titles, task.Title)
	}
	if strings.Join(titles, ",") != "Design,Build,Handover" {
		t.Fatalf("task order = %v; want schedule order Design, Build, Handover", titles)
	}
	design, build, handover := delivery.Tasks[0], delivery.Tasks[1], delivery.Tasks[2]
	if design.StartDate == "" || build.FinishDate == "" || design.PercentComplete != 100 || build.PercentComplete != 40 {
		t.Fatalf("tasks = %+v; want dates from the project start and progress", delivery.Tasks)
	}
	if !handover.Milestone || !design.Critical {
		t.Fatalf("tasks = %+v; want the milestone flagged and the chain critical", delivery.Tasks)
	}
	if broken := byTitle["Broken"]; broken.Note == "" || len(broken.Tasks) != 0 {
		t.Fatalf("unreadable schedule = %+v; want a note and no tasks", broken)
	}
}

func TestAdminViewShowsCostsAndDocumentTitlesOnly(t *testing.T) {
	app, _ := viewerFixtureApp(t)
	costs, err := app.AdminViewCosts()
	if err != nil {
		t.Fatalf("AdminViewCosts: %v", err)
	}
	if len(costs.Entries) != 1 || costs.Entries[0].Description != "Rebar" || costs.Entries[0].SupplierName != "Acme Steel" {
		t.Fatalf("cost entries = %+v; want bob's one entry", costs.Entries)
	}
	docs, err := app.AdminViewDocuments()
	if err != nil {
		t.Fatalf("AdminViewDocuments: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "Charter" || docs[0].Kind != "charter_word" {
		t.Fatalf("documents = %+v; want bob's charter", docs)
	}
	wire, err := json.Marshal(docs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(wire), "secret terms") || strings.Contains(string(wire), "content") {
		t.Fatalf("the document list carries contents: %s", wire)
	}
}

// Every viewer method goes through withAdminView: after access stops, or
// after the caller is demoted, each one is refused. The method list is
// checked against the App's exported AdminView* methods, so a new one must
// be added here.
func TestEveryAdminViewMethodIsRefusedWithoutAccess(t *testing.T) {
	calls := map[string]func(app *App, path string) error{
		"AdminListUserProjects": func(app *App, _ string) error { _, err := app.AdminListUserProjects(); return err },
		"AdminViewUserProject":  func(app *App, path string) error { _, err := app.AdminViewUserProject(path); return err },
		"AdminViewSchedule":     func(app *App, _ string) error { _, err := app.AdminViewSchedule(); return err },
		"AdminViewCosts":        func(app *App, _ string) error { _, err := app.AdminViewCosts(); return err },
		"AdminViewDocuments":    func(app *App, _ string) error { _, err := app.AdminViewDocuments(); return err },
	}
	appType := reflect.TypeOf(&App{})
	for i := range appType.NumMethod() {
		name := appType.Method(i).Name
		// Every method that reads the open account's data.
		if strings.HasPrefix(name, "AdminView") || name == "AdminListUserProjects" {
			if _, ok := calls[name]; !ok {
				t.Errorf("%s is not in this test's list; add it so its access check is tested", name)
			}
		}
	}

	// A fresh fixture per method, so one method's own check (which ends
	// access) cannot make the next method's refusal look like its own.
	for name, call := range calls {
		for _, ending := range []string{"stopped", "demoted"} {
			app, path := viewerFixtureApp(t)
			if ending == "stopped" {
				if err := app.AdminStopUserData(); err != nil {
					t.Fatalf("AdminStopUserData: %v", err)
				}
			} else {
				if _, err := app.CreateAccount("carol", "Carol", escrowTestPassword, true); err != nil {
					t.Fatalf("CreateAccount(carol): %v", err)
				}
				if err := app.store.SetAdmin("carol", "alice", false); err != nil {
					t.Fatalf("demote alice: %v", err)
				}
			}
			if err := call(app, path); err == nil {
				t.Errorf("%s after access %s: succeeded, want refused", name, ending)
			}
		}
	}
}

func TestAdminViewWarnsAboutABrokenAuditTrail(t *testing.T) {
	app, path := adminAccessApp(t)
	switchUser(t, app, "bob")
	project, err := app.OpenProject(path)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	settings, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	settings.ComplianceMode = true
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if _, err := app.db.Conn.Exec(`UPDATE audit_events SET after_canonical_json = ? WHERE project_id = ? AND sequence_number = 1`,
		`{"name":"tampered"}`, project.ID); err != nil {
		t.Fatalf("tamper audit chain: %v", err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	switchUser(t, app, "alice")
	if err := app.AdminOpenUserData("bob", "audit question"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}

	view, err := app.AdminViewUserProject(path)
	if err != nil {
		t.Fatalf("viewing a project with a broken audit trail: %v; want it shown with a warning", err)
	}
	if !strings.Contains(view.AuditWarning, "audit trail fails its tamper check") {
		t.Fatalf("audit warning = %q; want the broken trail named", view.AuditWarning)
	}
}

func TestAdminGrantKeyGivesTheKeyOrSaysWhyNot(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	// bob and carol are administrators without the key: made so in
	// system.db, as an install upgraded from before escrow would have them.
	for _, name := range []string{"bob", "carol"} {
		if _, err := app.CreateAccount(name, name, escrowTestPassword, false); err != nil {
			t.Fatalf("CreateAccount(%s): %v", name, err)
		}
	}
	if _, err := app.store.CreateAccount("dave", "Dave", escrowTestPassword, true); err != nil {
		t.Fatalf("create dave: %v", err)
	}
	conn := systemDB(t, app)
	if _, err := conn.Exec(`UPDATE users SET is_admin = 1 WHERE username IN ('bob', 'carol')`); err != nil {
		t.Fatalf("make administrators: %v", err)
	}

	waiting, err := app.AdminListAdminsWithoutKey()
	if err != nil || strings.Join(waiting, ",") != "bob,carol,dave" {
		t.Fatalf("AdminListAdminsWithoutKey = %v, %v; want bob, carol, dave", waiting, err)
	}
	if err := app.AdminGrantKey("bob"); err != nil {
		t.Fatalf("AdminGrantKey(bob): %v", err)
	}
	want := "dave must sign in once before they can get the administrator key"
	if err := app.AdminGrantKey("dave"); err == nil || err.Error() != want {
		t.Fatalf("AdminGrantKey(dave) = %v, want %q", err, want)
	}
	if waiting, _ := app.AdminListAdminsWithoutKey(); strings.Join(waiting, ",") != "carol,dave" {
		t.Fatalf("after granting bob, waiting = %v; want carol, dave", waiting)
	}

	// An administrator without the key cannot give it, and is told so.
	switchUser(t, app, "carol")
	want = "you don't hold the administrator key yourself, so you can't give it"
	if err := app.AdminGrantKey("dave"); err == nil || err.Error() != want {
		t.Fatalf("AdminGrantKey by an administrator without the key = %v, want %q", err, want)
	}
}
