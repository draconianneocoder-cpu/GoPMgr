// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
	"time"

	"gopmgr/internal/agile"
)

// withAgilePackEnabled saves and restores the package-level agile.PackEnabled
// atomic before/after a test. It's shared process-wide state (SetAgileEnabled
// and AgileEnabled's no-project branch both touch it), so any test that reads
// or writes it without resetting would pass or fail depending on what an
// earlier test in the same run left behind — audit_actions_test.go's
// TestDeleteWorkItem_WritesAuditLog already calls SetAgileEnabled(true) and
// never resets it, so this is not a hypothetical.
func withAgilePackEnabled(t *testing.T) {
	t.Helper()
	old := agile.PackEnabled.Load()
	t.Cleanup(func() { agile.PackEnabled.Store(old) })
}

func TestAgileStore_RequiresOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	// No OpenProject call: a.db stays nil.
	if err := app.SaveColumn(agile.Column{BoardID: "b", Name: "x"}); err == nil || !strings.Contains(err.Error(), "agile: no project open") {
		t.Fatalf("SaveColumn with no project open: err = %v, want \"agile: no project open\"", err)
	}
}

// TestDeleteWorkItem_RequiresOpenProject pins DeleteWorkItem's OWN guard
// text, distinct from agileStore()'s "agile: no project open" above:
// DeleteWorkItem calls requireDB() directly (to attribute the audit-log
// entry before deleting) and returns a plain "no project open" if it's nil,
// never reaching agileStore() at all. Asserting the specific string means a
// future unification of the two guards is a deliberate change, not a
// silent one.
func TestDeleteWorkItem_RequiresOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := app.DeleteWorkItem("wi_x"); err == nil || err.Error() != "no project open" {
		t.Fatalf("DeleteWorkItem with no project open: err = %v, want exactly \"no project open\"", err)
	}
}

func TestAgileEnabled_NoProjectOpenReturnsCachedValue(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)

	agile.PackEnabled.Store(true)
	if got, err := app.AgileEnabled(); err != nil || !got {
		t.Fatalf("AgileEnabled (no project, cache=true) = (%v, %v), want (true, nil)", got, err)
	}
	agile.PackEnabled.Store(false)
	if got, err := app.AgileEnabled(); err != nil || got {
		t.Fatalf("AgileEnabled (no project, cache=false) = (%v, %v), want (false, nil)", got, err)
	}
}

// TestAgileEnabled_SetPersistsToProjectSettingsAndReloads proves the
// setting actually round-trips through the project's DB row on disk, not
// just through the in-process agile.PackEnabled atomic both methods happen
// to share. Two things would let a broken AgileEnabled slip past a naive
// SetAgileEnabled(true)-then-AgileEnabled() assertion:
//  1. AgileEnabled silently returning the stale atomic without touching the
//     DB at all -- closed by desyncing the atomic from the expected value
//     immediately before each AgileEnabled() call, so only a real settings
//     read can recover the correct answer.
//  2. SetAgileEnabled writing the atomic but never actually persisting to
//     the project's settings row -- closed by reopening the project (a real
//     Close+InitEncryptedDB round trip through OpenProject, not merely
//     re-reading the same live *db.Database) between the write and the
//     read, so the assertion can only pass if the value survived on disk.
//
// OpenProject does not touch agile.PackEnabled itself, so the atomic-desync
// step after each reopen still does real work.
func TestAgileEnabled_SetPersistsToProjectSettingsAndReloads(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	path := mustOpenProject(t, app, "Agile Bridge Plan")

	if err := app.SetAgileEnabled(true); err != nil {
		t.Fatalf("SetAgileEnabled(true): %v", err)
	}
	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject (reload after SetAgileEnabled(true)): %v", err)
	}
	agile.PackEnabled.Store(false) // desync: only a real DB read should recover true
	if got, err := app.AgileEnabled(); err != nil || !got {
		t.Fatalf("AgileEnabled after SetAgileEnabled(true)+reload = (%v, %v), want (true, nil)", got, err)
	}

	if err := app.SetAgileEnabled(false); err != nil {
		t.Fatalf("SetAgileEnabled(false): %v", err)
	}
	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject (reload after SetAgileEnabled(false)): %v", err)
	}
	agile.PackEnabled.Store(true) // desync: only a real DB read should recover false
	if got, err := app.AgileEnabled(); err != nil || got {
		t.Fatalf("AgileEnabled after SetAgileEnabled(false)+reload = (%v, %v), want (false, nil)", got, err)
	}
}

func TestColumnAndWorkItem_AppMethodsRoundTrip(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Agile Bridge Plan")

	bwc, err := app.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}
	if len(bwc.Columns) == 0 {
		t.Fatal("EnsureDefaultBoard returned no columns")
	}
	firstColumn := bwc.Columns[0].ID

	if err := app.SaveColumn(agile.Column{BoardID: bwc.Board.ID, Name: "Blocked", OrderIdx: 9}); err != nil {
		t.Fatalf("SaveColumn: %v", err)
	}
	after, err := app.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard after SaveColumn: %v", err)
	}
	var blockedID string
	for _, c := range after.Columns {
		if c.Name == "Blocked" {
			blockedID = c.ID
		}
	}
	if blockedID == "" {
		t.Fatalf("new column not present after SaveColumn: %#v", after.Columns)
	}
	if err := app.DeleteColumn(blockedID); err != nil {
		t.Fatalf("DeleteColumn: %v", err)
	}
	after, err = app.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard after DeleteColumn: %v", err)
	}
	for _, c := range after.Columns {
		if c.ID == blockedID {
			t.Fatalf("column %q still present after DeleteColumn", blockedID)
		}
	}

	saved, err := app.SaveWorkItem(agile.WorkItem{Title: "Bridge Story", State: firstColumn})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	got, err := app.GetWorkItem(saved.ID)
	if err != nil {
		t.Fatalf("GetWorkItem: %v", err)
	}
	if got.Title != "Bridge Story" {
		t.Fatalf("GetWorkItem.Title = %q, want Bridge Story", got.Title)
	}

	list, err := app.ListWorkItems("", firstColumn, "")
	if err != nil {
		t.Fatalf("ListWorkItems: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("ListWorkItems(state=%q) = %#v, want exactly [%s]", firstColumn, list, saved.ID)
	}

	doneColumn := bwc.Columns[len(bwc.Columns)-1].ID
	if err := app.MoveWorkItem(saved.ID, doneColumn, 5); err != nil {
		t.Fatalf("MoveWorkItem: %v", err)
	}
	moved, err := app.GetWorkItem(saved.ID)
	if err != nil {
		t.Fatalf("GetWorkItem after move: %v", err)
	}
	if moved.State != doneColumn || moved.OrderIdx != 5 {
		t.Fatalf("moved work item = %#v, want state=%q order_idx=5", moved, doneColumn)
	}

	counts, err := app.WIPCounts()
	if err != nil {
		t.Fatalf("WIPCounts: %v", err)
	}
	if counts[doneColumn] != 1 {
		t.Fatalf("WIPCounts[%q] = %d, want 1: %#v", doneColumn, counts[doneColumn], counts)
	}
}

func TestSprint_AppMethodsRoundTrip(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Agile Bridge Plan")

	saved, err := app.SaveSprint(agile.Sprint{Name: "Sprint 1"})
	if err != nil {
		t.Fatalf("SaveSprint: %v", err)
	}
	list, err := app.ListSprints()
	if err != nil {
		t.Fatalf("ListSprints: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("ListSprints = %#v, want exactly [%s]", list, saved.ID)
	}

	if err := app.DeleteSprint(saved.ID); err != nil {
		t.Fatalf("DeleteSprint: %v", err)
	}
	list, err = app.ListSprints()
	if err != nil {
		t.Fatalf("ListSprints after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListSprints after delete = %#v, want empty", list)
	}
}

func TestDeployment_AppMethodsRoundTrip(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Agile Bridge Plan")

	saved, err := app.SaveDeployment(agile.Deployment{Version: "1.0.0", Successful: true})
	if err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}
	list, err := app.ListDeployments("")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("ListDeployments = %#v, want exactly [%s]", list, saved.ID)
	}

	if err := app.DeleteDeployment(saved.ID); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}
	list, err = app.ListDeployments("")
	if err != nil {
		t.Fatalf("ListDeployments after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListDeployments after delete = %#v, want empty", list)
	}
}

// TestListDeployments_SinceISOParsing pins ListDeployments' current,
// deliberately-not-changed behavior: an unparseable sinceISO string is
// treated the same as an empty one (no filter, all deployments returned)
// rather than surfaced as an error. Confirmed via
// `grep -rn ListDeployments frontend/src` that the frontend only ever
// calls this with "" — never a real date, valid or otherwise — so there is
// no live evidence either way for what a non-empty caller should expect,
// and no user-visible behavior this test's leniency assertion could be
// silently changing. Pinning it here means a future switch to a strict
// parse error is a deliberate, tested change, not an incidental one.
func TestListDeployments_SinceISOParsing(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Agile Bridge Plan")

	older := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := app.SaveDeployment(agile.Deployment{Version: "old", TS: older}); err != nil {
		t.Fatalf("SaveDeployment(old): %v", err)
	}
	if _, err := app.SaveDeployment(agile.Deployment{Version: "new", TS: newer}); err != nil {
		t.Fatalf("SaveDeployment(new): %v", err)
	}

	windowed, err := app.ListDeployments(newer.Add(-time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatalf("ListDeployments(valid since): %v", err)
	}
	if len(windowed) != 1 || windowed[0].Version != "new" {
		t.Fatalf("ListDeployments(valid since) = %#v, want only [new]", windowed)
	}

	unparseable, err := app.ListDeployments("not-a-timestamp")
	if err != nil {
		t.Fatalf("ListDeployments(unparseable since) returned an error: %v — behavior today is lenient fallback, not rejection", err)
	}
	if len(unparseable) != 2 {
		t.Fatalf("ListDeployments(unparseable since) = %#v, want both deployments (current lenient behavior)", unparseable)
	}
}

// TestComputeDORA_FetchWindowMatchesClassificationWindow is a regression
// test for a fetch/classification window mismatch fixed alongside this
// test: ComputeDORA used to compute `since` from the raw windowDays
// argument (so windowDays=0 fetched deployments since "now", i.e. almost
// none) while passing that same raw windowDays through to
// agile.ComputeDORA, which defaults windowDays<=0 to a 30-day
// classification window internally. A deployment 20 days old — well
// within the intended 30-day default — would silently vanish from the
// result: not filtered out by classification, but never fetched from the
// database in the first place.
func TestComputeDORA_FetchWindowMatchesClassificationWindow(t *testing.T) {
	withAgilePackEnabled(t)
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Agile Bridge Plan")

	twentyDaysAgo := time.Now().UTC().AddDate(0, 0, -20)
	if _, err := app.SaveDeployment(agile.Deployment{Version: "recent-ish", Successful: true, TS: twentyDaysAgo}); err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}

	result, err := app.ComputeDORA(0) // 0 means "use the documented 30-day default"
	if err != nil {
		t.Fatalf("ComputeDORA(0): %v", err)
	}
	if result.WindowDays != 30 {
		t.Fatalf("WindowDays = %d, want 30 (the documented default)", result.WindowDays)
	}
	if result.TotalDeploys != 1 {
		t.Fatalf("TotalDeploys = %d, want 1 — a deployment 20 days old must be included in a 30-day-default window", result.TotalDeploys)
	}
}
