// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gopmgr/internal/db"
)

func newAgileTestStore(t *testing.T) (*db.Database, *Store, db.Project) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "agile-store.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	project, err := d.UpsertProject(db.Project{ID: "project-fixed", Name: "Agile store test"})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return d, NewStore(d.Conn, project.ID), project
}

func TestEnsureDefaultBoardRepairsMissingDefaultColumns(t *testing.T) {
	d, store, project := newAgileTestStore(t)
	if _, err := d.Conn.Exec(
		`INSERT INTO agile_boards (id, project_id, name, is_default) VALUES (?, ?, ?, 1)`,
		"board-repair", project.ID, "Main board",
	); err != nil {
		t.Fatalf("seed incomplete default board: %v", err)
	}
	if _, err := d.Conn.Exec(
		`INSERT INTO agile_columns (id, board_id, name, order_idx, wip_limit) VALUES (?, ?, ?, ?, ?)`,
		"doing", "board-repair", "Doing Custom", 9, 7,
	); err != nil {
		t.Fatalf("seed customized column: %v", err)
	}

	board, err := store.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}
	if board.ID != "board-repair" {
		t.Fatalf("board ID = %q, want existing default board", board.ID)
	}

	columns, err := store.ListColumns(board.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	if len(columns) != 4 {
		t.Fatalf("column count = %d, want 4: %#v", len(columns), columns)
	}

	byID := make(map[string]Column, len(columns))
	for _, c := range columns {
		byID[c.ID] = c
	}
	for _, id := range []string{"todo", "doing", "review", "done"} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("missing default column %q after repair: %#v", id, columns)
		}
	}
	if got := byID["doing"]; got.Name != "Doing Custom" || got.OrderIdx != 9 || got.WIPLimit != 7 {
		t.Fatalf("existing customized column overwritten: %#v", got)
	}
}

// ----- Columns -----

func TestSaveColumn_InsertsThenUpdatesViaUpsert(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	board, err := store.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}

	c := Column{BoardID: board.ID, Name: "Blocked", OrderIdx: 9, WIPLimit: 2}
	if err := store.SaveColumn(c); err != nil {
		t.Fatalf("SaveColumn insert: %v", err)
	}
	cols, err := store.ListColumns(board.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	var inserted Column
	found := false
	for _, got := range cols {
		if got.Name == "Blocked" {
			inserted, found = got, true
			break
		}
	}
	if !found {
		t.Fatalf("inserted column not found: %#v", cols)
	}
	if inserted.ID == "" {
		t.Fatal("SaveColumn did not assign a generated ID")
	}

	// Re-save with the same ID: must update in place, not duplicate.
	inserted.Name = "Blocked (renamed)"
	inserted.OrderIdx = 10
	inserted.WIPLimit = 5
	if err := store.SaveColumn(inserted); err != nil {
		t.Fatalf("SaveColumn update: %v", err)
	}
	cols, err = store.ListColumns(board.ID)
	if err != nil {
		t.Fatalf("ListColumns after update: %v", err)
	}
	matches := 0
	var updated Column
	for _, got := range cols {
		if got.ID == inserted.ID {
			matches++
			updated = got
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly one row for column %q after upsert, found %d", inserted.ID, matches)
	}
	if updated.Name != "Blocked (renamed)" || updated.OrderIdx != 10 || updated.WIPLimit != 5 {
		t.Fatalf("upsert did not apply changes: %#v", updated)
	}
}

func TestDeleteColumn_RemovesRow(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	board, err := store.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}
	if err := store.SaveColumn(Column{BoardID: board.ID, Name: "Blocked"}); err != nil {
		t.Fatalf("SaveColumn: %v", err)
	}
	cols, err := store.ListColumns(board.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	var id string
	for _, c := range cols {
		if c.Name == "Blocked" {
			id = c.ID
		}
	}
	if id == "" {
		t.Fatal("seeded column not found")
	}

	if err := store.DeleteColumn(id); err != nil {
		t.Fatalf("DeleteColumn: %v", err)
	}
	cols, err = store.ListColumns(board.ID)
	if err != nil {
		t.Fatalf("ListColumns after delete: %v", err)
	}
	for _, c := range cols {
		if c.ID == id {
			t.Fatalf("column %q still present after DeleteColumn", id)
		}
	}
}

// ----- Work items -----

func TestSaveWorkItem_AppliesDefaults(t *testing.T) {
	_, store, project := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "Untyped story"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if saved.Type != WorkItemStory {
		t.Errorf("Type = %q, want %q", saved.Type, WorkItemStory)
	}
	if saved.Priority != PrioMedium {
		t.Errorf("Priority = %q, want %q", saved.Priority, PrioMedium)
	}
	if saved.State != "backlog" {
		t.Errorf("State = %q, want backlog", saved.State)
	}
	if saved.ProjectID != project.ID {
		t.Errorf("ProjectID = %q, want %q (store's default, since none was supplied)", saved.ProjectID, project.ID)
	}
}

func TestSaveWorkItem_ClosedAtAutoStampedWhenDoneAndZero(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	before := time.Now().UTC()
	saved, err := store.SaveWorkItem(WorkItem{Title: "Finished", State: "done"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if saved.ClosedAt.IsZero() {
		t.Fatal("expected ClosedAt to be auto-stamped for a done item with no ClosedAt supplied")
	}
	if saved.ClosedAt.Before(before) {
		t.Errorf("ClosedAt = %v, want at or after %v", saved.ClosedAt, before)
	}
}

func TestSaveWorkItem_ClosedAtPreservedWhenExplicit(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	explicit := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	saved, err := store.SaveWorkItem(WorkItem{Title: "Backfilled", State: "done", ClosedAt: explicit})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if !saved.ClosedAt.Equal(explicit) {
		t.Fatalf("ClosedAt = %v, want the explicitly supplied %v unmodified", saved.ClosedAt, explicit)
	}
}

func TestSaveWorkItem_NotDoneStoresEmptyClosedAt(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "In flight", State: "doing"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if !saved.ClosedAt.IsZero() {
		t.Fatalf("ClosedAt = %v, want zero for a non-done item", saved.ClosedAt)
	}
}

func TestSaveWorkItem_UpsertUpdatesFieldsAndPreservesCreatedAt(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	first, err := store.SaveWorkItem(WorkItem{Title: "Original title"})
	if err != nil {
		t.Fatalf("SaveWorkItem insert: %v", err)
	}
	if first.CreatedAt.IsZero() {
		t.Fatal("CreatedAt was not stamped on insert")
	}

	first.Title = "Renamed title"
	second, err := store.SaveWorkItem(first)
	if err != nil {
		t.Fatalf("SaveWorkItem update: %v", err)
	}
	if second.Title != "Renamed title" {
		t.Errorf("Title = %q, want Renamed title", second.Title)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("CreatedAt changed across an update: got %v, want unchanged %v", second.CreatedAt, first.CreatedAt)
	}
}

func TestSaveWorkItem_DefaultsProjectIDToStoreProjectID(t *testing.T) {
	_, store, project := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "No explicit project"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if saved.ProjectID != project.ID {
		t.Fatalf("ProjectID = %q, want the store's own %q", saved.ProjectID, project.ID)
	}
}

func TestGetWorkItem_NotFoundReturnsErrNoWorkItem(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	if _, err := store.GetWorkItem("does-not-exist"); !errors.Is(err, ErrNoWorkItem) {
		t.Fatalf("GetWorkItem error = %v, want ErrNoWorkItem", err)
	}
}

func TestListWorkItems_ScopesToProjectAndOrdersByOrderIdx(t *testing.T) {
	d, store, project := newAgileTestStore(t)
	if _, err := store.SaveWorkItem(WorkItem{Title: "Second", OrderIdx: 2}); err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if _, err := store.SaveWorkItem(WorkItem{Title: "First", OrderIdx: 1}); err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}

	// Seed a work item for a second project directly via SQL. GoPMgr's
	// .pmforge files hold exactly one project each today (see the schema
	// comment on the `project` table in internal/db/sqlite.go), so this
	// two-project state is not reachable through the application — it
	// exists here only to prove ListWorkItems' WHERE project_id = ?
	// predicate actually filters, at the SQL layer, rather than assuming it.
	otherProject, err := d.UpsertProject(db.Project{ID: "other-project", Name: "Other"})
	if err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	otherStore := NewStore(d.Conn, otherProject.ID)
	if _, err := otherStore.SaveWorkItem(WorkItem{Title: "Belongs to other project"}); err != nil {
		t.Fatalf("SaveWorkItem (other project): %v", err)
	}

	items, err := store.ListWorkItems("", "", "")
	if err != nil {
		t.Fatalf("ListWorkItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2 (scoped to %q): %#v", len(items), project.ID, items)
	}
	if items[0].Title != "First" || items[1].Title != "Second" {
		t.Fatalf("order = [%q, %q], want [First, Second] (order_idx ASC)", items[0].Title, items[1].Title)
	}
}

func TestListWorkItems_FiltersBySprintStateAndAssignee(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	sprint, err := store.SaveSprint(Sprint{Name: "Sprint 1"})
	if err != nil {
		t.Fatalf("SaveSprint: %v", err)
	}

	mk := func(title, state, assignee, sprintID string) {
		t.Helper()
		if _, err := store.SaveWorkItem(WorkItem{
			Title: title, State: state, Assignee: assignee, SprintID: sprintID,
		}); err != nil {
			t.Fatalf("SaveWorkItem %q: %v", title, err)
		}
	}
	mk("A", "todo", "alice", sprint.ID)
	mk("B", "todo", "bob", "")
	mk("C", "done", "alice", sprint.ID)

	byState, err := store.ListWorkItems("", "todo", "")
	if err != nil {
		t.Fatalf("ListWorkItems(state=todo): %v", err)
	}
	if len(byState) != 2 {
		t.Fatalf("state filter: len = %d, want 2: %#v", len(byState), byState)
	}

	bySprint, err := store.ListWorkItems(sprint.ID, "", "")
	if err != nil {
		t.Fatalf("ListWorkItems(sprint): %v", err)
	}
	if len(bySprint) != 2 {
		t.Fatalf("sprint filter: len = %d, want 2: %#v", len(bySprint), bySprint)
	}

	byAssignee, err := store.ListWorkItems("", "", "alice")
	if err != nil {
		t.Fatalf("ListWorkItems(assignee=alice): %v", err)
	}
	if len(byAssignee) != 2 {
		t.Fatalf("assignee filter: len = %d, want 2: %#v", len(byAssignee), byAssignee)
	}

	combined, err := store.ListWorkItems(sprint.ID, "todo", "alice")
	if err != nil {
		t.Fatalf("ListWorkItems(combined): %v", err)
	}
	if len(combined) != 1 || combined[0].Title != "A" {
		t.Fatalf("combined filter = %#v, want exactly [A]", combined)
	}
}

func TestDeleteWorkItem_RemovesRow(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "Temporary"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if err := store.DeleteWorkItem(saved.ID); err != nil {
		t.Fatalf("DeleteWorkItem: %v", err)
	}
	if _, err := store.GetWorkItem(saved.ID); !errors.Is(err, ErrNoWorkItem) {
		t.Fatalf("GetWorkItem after delete: err = %v, want ErrNoWorkItem", err)
	}
}

func TestMoveWorkItem_ToDoneStampsClosedAt(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "Moving", State: "todo"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if !saved.ClosedAt.IsZero() {
		t.Fatalf("precondition: ClosedAt should start zero, got %v", saved.ClosedAt)
	}

	if err := store.MoveWorkItem(saved.ID, "done", 3); err != nil {
		t.Fatalf("MoveWorkItem: %v", err)
	}
	moved, err := store.GetWorkItem(saved.ID)
	if err != nil {
		t.Fatalf("GetWorkItem: %v", err)
	}
	if moved.State != "done" || moved.OrderIdx != 3 {
		t.Fatalf("moved = %#v, want state=done order_idx=3", moved)
	}
	if moved.ClosedAt.IsZero() {
		t.Fatal("expected ClosedAt to be stamped when moving into the done column")
	}
}

// TestMoveWorkItem_AwayFromDonePreservesClosedAt locks in a specific and
// easily-missed asymmetry in MoveWorkItem's SQL: the CASE expression only
// sets closed_at when the destination is "done" (`ELSE closed_at`) — moving
// an already-closed item to any other column does NOT clear closed_at. A
// mutation that re-stamped closed_at with time.Now() on every move, or one
// that cleared it when leaving "done", would both still look plausible
// without a test pinning the exact preserved value.
func TestMoveWorkItem_AwayFromDonePreservesClosedAt(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "Reopened", State: "done"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if saved.ClosedAt.IsZero() {
		t.Fatal("precondition: saving as done should have stamped ClosedAt")
	}
	originalClosedAt := saved.ClosedAt

	if err := store.MoveWorkItem(saved.ID, "doing", 0); err != nil {
		t.Fatalf("MoveWorkItem: %v", err)
	}
	moved, err := store.GetWorkItem(saved.ID)
	if err != nil {
		t.Fatalf("GetWorkItem: %v", err)
	}
	if moved.State != "doing" {
		t.Fatalf("State = %q, want doing", moved.State)
	}
	if !moved.ClosedAt.Equal(originalClosedAt) {
		t.Fatalf("ClosedAt = %v, want it preserved unchanged at %v after leaving the done column", moved.ClosedAt, originalClosedAt)
	}
}

// TestWIPCountByColumn_ScopesToProject seeds a second project directly via
// SQL, the same not-reachable-through-the-app technique and rationale as
// TestListWorkItems_ScopesToProjectAndOrdersByOrderIdx above.
func TestWIPCountByColumn_ScopesToProject(t *testing.T) {
	d, store, _ := newAgileTestStore(t)
	mk := func(s *Store, title, state string) {
		t.Helper()
		if _, err := s.SaveWorkItem(WorkItem{Title: title, State: state}); err != nil {
			t.Fatalf("SaveWorkItem %q: %v", title, err)
		}
	}
	mk(store, "A", "todo")
	mk(store, "B", "todo")
	mk(store, "C", "done")

	otherProject, err := d.UpsertProject(db.Project{ID: "other-project", Name: "Other"})
	if err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	otherStore := NewStore(d.Conn, otherProject.ID)
	mk(otherStore, "D", "todo")

	counts, err := store.WIPCountByColumn()
	if err != nil {
		t.Fatalf("WIPCountByColumn: %v", err)
	}
	if counts["todo"] != 2 {
		t.Errorf("todo count = %d, want 2 (scoped to this store's project)", counts["todo"])
	}
	if counts["done"] != 1 {
		t.Errorf("done count = %d, want 1", counts["done"])
	}
}

// ----- Sprints -----

func TestSaveSprint_AppliesDefaultStatusAndProjectID(t *testing.T) {
	_, store, project := newAgileTestStore(t)
	saved, err := store.SaveSprint(Sprint{Name: "Sprint 1"})
	if err != nil {
		t.Fatalf("SaveSprint: %v", err)
	}
	if saved.Status != SprintPlanning {
		t.Errorf("Status = %q, want %q", saved.Status, SprintPlanning)
	}
	if saved.ProjectID != project.ID {
		t.Errorf("ProjectID = %q, want %q", saved.ProjectID, project.ID)
	}
}

func TestGetSprint_NotFoundReturnsErrNoSprint(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	if _, err := store.GetSprint("does-not-exist"); !errors.Is(err, ErrNoSprint) {
		t.Fatalf("GetSprint error = %v, want ErrNoSprint", err)
	}
}

// TestListSprints_OrdersNewestFirst seeds created_at directly via SQL
// rather than through two back-to-back SaveSprint calls: created_at is a
// DB-side default (`strftime(..., 'now')`, millisecond precision, not
// settable through SaveSprint's own INSERT column list), so two inserts in
// the same test could land in the same millisecond and make the DESC
// ordering assertion flaky. Explicit, clearly-separated timestamps make
// this deterministic.
func TestListSprints_OrdersNewestFirst(t *testing.T) {
	d, store, project := newAgileTestStore(t)
	seed := func(id, name, createdAt string) {
		t.Helper()
		if _, err := d.Conn.Exec(
			`INSERT INTO agile_sprints (id, project_id, name, created_at) VALUES (?, ?, ?, ?)`,
			id, project.ID, name, createdAt,
		); err != nil {
			t.Fatalf("seed sprint %q: %v", name, err)
		}
	}
	seed("sprint-old", "Older", "2024-01-01T00:00:00.000Z")
	seed("sprint-new", "Newer", "2025-01-01T00:00:00.000Z")

	sprints, err := store.ListSprints()
	if err != nil {
		t.Fatalf("ListSprints: %v", err)
	}
	if len(sprints) != 2 || sprints[0].Name != "Newer" || sprints[1].Name != "Older" {
		t.Fatalf("sprints = %#v, want [Newer, Older]", sprints)
	}
}

func TestDeleteSprint_RemovesSprintAndClearsWorkItemReferences(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	sprint, err := store.SaveSprint(Sprint{Name: "Sprint 1"})
	if err != nil {
		t.Fatalf("SaveSprint: %v", err)
	}
	item, err := store.SaveWorkItem(WorkItem{Title: "In the sprint", SprintID: sprint.ID})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}

	if err := store.DeleteSprint(sprint.ID); err != nil {
		t.Fatalf("DeleteSprint: %v", err)
	}
	if _, err := store.GetSprint(sprint.ID); !errors.Is(err, ErrNoSprint) {
		t.Fatalf("GetSprint after delete: err = %v, want ErrNoSprint", err)
	}
	reread, err := store.GetWorkItem(item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem after sprint delete: %v", err)
	}
	if reread.SprintID != "" {
		t.Fatalf("SprintID = %q, want cleared to empty after the sprint it referenced was deleted", reread.SprintID)
	}
}

// ----- Deployments -----

func TestSaveDeployment_StampsTSWhenZeroAndPreservesWhenSet(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	before := time.Now().UTC()
	autoStamped, err := store.SaveDeployment(Deployment{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}
	if autoStamped.TS.Before(before) {
		t.Errorf("TS = %v, want at or after %v", autoStamped.TS, before)
	}

	explicit := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	preserved, err := store.SaveDeployment(Deployment{Version: "2.0.0", TS: explicit})
	if err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}
	if !preserved.TS.Equal(explicit) {
		t.Fatalf("TS = %v, want the explicitly supplied %v unmodified", preserved.TS, explicit)
	}
}

func TestSaveDeployment_SuccessfulRoundTrips(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	for _, want := range []bool{true, false} {
		saved, err := store.SaveDeployment(Deployment{
			Version: "1.0.0", Successful: want, TS: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("SaveDeployment(Successful=%v): %v", want, err)
		}
		if saved.Successful != want {
			t.Errorf("Successful = %v, want %v", saved.Successful, want)
		}

		reread, err := store.ListDeployments(time.Time{})
		if err != nil {
			t.Fatalf("ListDeployments: %v", err)
		}
		found := false
		for _, d := range reread {
			if d.ID == saved.ID {
				found = true
				if d.Successful != want {
					t.Errorf("re-read Successful = %v, want %v", d.Successful, want)
				}
			}
		}
		if !found {
			t.Fatalf("saved deployment %q not found in ListDeployments", saved.ID)
		}
	}
}

func TestListDeployments_AllVsWindowed(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := store.SaveDeployment(Deployment{Version: "old", TS: older}); err != nil {
		t.Fatalf("SaveDeployment(old): %v", err)
	}
	if _, err := store.SaveDeployment(Deployment{Version: "new", TS: newer}); err != nil {
		t.Fatalf("SaveDeployment(new): %v", err)
	}

	all, err := store.ListDeployments(time.Time{})
	if err != nil {
		t.Fatalf("ListDeployments(all): %v", err)
	}
	if len(all) != 2 || all[0].Version != "new" || all[1].Version != "old" {
		t.Fatalf("all = %#v, want [new, old] (newest first)", all)
	}

	windowed, err := store.ListDeployments(newer.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListDeployments(windowed): %v", err)
	}
	if len(windowed) != 1 || windowed[0].Version != "new" {
		t.Fatalf("windowed = %#v, want only [new]", windowed)
	}
}

func TestDeleteDeployment_RemovesRow(t *testing.T) {
	_, store, _ := newAgileTestStore(t)
	saved, err := store.SaveDeployment(Deployment{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}
	if err := store.DeleteDeployment(saved.ID); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}
	remaining, err := store.ListDeployments(time.Time{})
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	for _, d := range remaining {
		if d.ID == saved.ID {
			t.Fatalf("deployment %q still present after DeleteDeployment", saved.ID)
		}
	}
}

// ----- Error propagation (closed connection, corrupted columns) -----

// TestStoreMethods_PropagateClosedConnectionErrors closes the underlying
// *sql.DB and confirms every read/write entry point returns an error
// instead of a zero value with a nil error, rather than assuming
// closed-connection failures propagate correctly. Note this proves
// end-to-end propagation, not that it's specifically the FIRST db call's
// own `if err != nil` guard doing the work: for methods with more than one
// DB call in sequence (e.g. SaveWorkItem's Exec then its own GetWorkItem
// call), closing the connection fails every one of them, so a mutation
// deleting the first guard is masked by the second call failing on its own
// — confirmed directly by fault-seeding SaveWorkItem's guard specifically,
// the same cascading-fallible-path shape documented throughout
// internal/users' store_test.go. Still a real, worthwhile invariant: no
// method here silently returns a zero value on a dead connection.
func TestStoreMethods_PropagateClosedConnectionErrors(t *testing.T) {
	d, store, _ := newAgileTestStore(t)
	if err := d.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"EnsureDefaultBoard", func() error { _, err := store.EnsureDefaultBoard(); return err }},
		{"ListColumns", func() error { _, err := store.ListColumns("any-board"); return err }},
		{"SaveWorkItem", func() error { _, err := store.SaveWorkItem(WorkItem{Title: "x"}); return err }},
		{"ListWorkItems", func() error { _, err := store.ListWorkItems("", "", ""); return err }},
		{"WIPCountByColumn", func() error { _, err := store.WIPCountByColumn(); return err }},
		{"SaveSprint", func() error { _, err := store.SaveSprint(Sprint{Name: "x"}); return err }},
		{"ListSprints", func() error { _, err := store.ListSprints(); return err }},
		{"DeleteSprint", func() error { return store.DeleteSprint("any-sprint") }},
		{"SaveDeployment", func() error { _, err := store.SaveDeployment(Deployment{Version: "x"}); return err }},
		{"ListDeployments", func() error { _, err := store.ListDeployments(time.Time{}); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatalf("%s on a closed connection = nil error, want a propagated failure", tc.name)
			}
		})
	}
}

// TestEnsureDefaultColumns_PropagatesExecutorError unit-tests the free
// function directly against a fake columnExecutor, rather than trying to
// force a live *sql.DB's INSERT to fail mid-transaction (no deterministic,
// portable hook exists for that — the transaction's own Begin already
// succeeded by the time this runs, so closing the connection first would
// only reach Begin's own error branch, not this one).
type failingColumnExecutor struct{ err error }

func (f failingColumnExecutor) Exec(string, ...any) (sql.Result, error) { return nil, f.err }

func TestEnsureDefaultColumns_PropagatesExecutorError(t *testing.T) {
	wantErr := errors.New("insert blocked")
	if err := ensureDefaultColumns(failingColumnExecutor{err: wantErr}, "board-x"); !errors.Is(err, wantErr) {
		t.Fatalf("ensureDefaultColumns error = %v, want %v", err, wantErr)
	}
}

// EnsureDefaultBoard's existing-default-board Scan-error branch (distinct
// from the sql.ErrNoRows seed path) is exercised by
// TestStoreMethods_PropagateClosedConnectionErrors above, not by a
// corrupted-column variant here: is_default is both the only non-string
// column in that SELECT and the column the WHERE clause filters on
// (`is_default = 1`), so corrupting it to a non-1 value makes the row stop
// matching the WHERE clause before Scan ever runs — it just falls through
// to the sql.ErrNoRows seed-a-new-board path instead of erroring. Confirmed
// by running exactly that corrupted-column variant: it failed the "want a
// Scan error" assertion, which is what revealed the WHERE-clause coupling.

func TestListColumns_PropagatesScanErrorOnCorruptedColumn(t *testing.T) {
	d, store, _ := newAgileTestStore(t)
	board, err := store.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE agile_columns SET order_idx = 'not-an-int' WHERE board_id = ?`, board.ID); err != nil {
		t.Fatalf("corrupt order_idx: %v", err)
	}
	if _, err := store.ListColumns(board.ID); err == nil {
		t.Fatal("ListColumns with corrupted order_idx column = nil, want a Scan error")
	}
}

func TestScanWorkItem_PropagatesScanErrorOnCorruptedColumn(t *testing.T) {
	d, store, _ := newAgileTestStore(t)
	saved, err := store.SaveWorkItem(WorkItem{Title: "x"})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE agile_work_items SET points = 'not-a-float' WHERE id = ?`, saved.ID); err != nil {
		t.Fatalf("corrupt points: %v", err)
	}
	if _, err := store.GetWorkItem(saved.ID); err == nil {
		t.Fatal("GetWorkItem with corrupted points column = nil, want a Scan error")
	}
	if _, err := store.ListWorkItems("", "", ""); err == nil {
		t.Fatal("ListWorkItems with corrupted points column = nil, want a Scan error")
	}
}

func TestScanSprint_PropagatesScanErrorOnCorruptedColumn(t *testing.T) {
	d, store, _ := newAgileTestStore(t)
	saved, err := store.SaveSprint(Sprint{Name: "x"})
	if err != nil {
		t.Fatalf("SaveSprint: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE agile_sprints SET capacity = 'not-a-float' WHERE id = ?`, saved.ID); err != nil {
		t.Fatalf("corrupt capacity: %v", err)
	}
	if _, err := store.GetSprint(saved.ID); err == nil {
		t.Fatal("GetSprint with corrupted capacity column = nil, want a Scan error")
	}
	if _, err := store.ListSprints(); err == nil {
		t.Fatal("ListSprints with corrupted capacity column = nil, want a Scan error")
	}
}

func TestListDeployments_PropagatesScanErrorOnCorruptedColumn(t *testing.T) {
	d, store, _ := newAgileTestStore(t)
	saved, err := store.SaveDeployment(Deployment{Version: "x"})
	if err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE agile_deployments SET successful = 'not-an-int' WHERE id = ?`, saved.ID); err != nil {
		t.Fatalf("corrupt successful: %v", err)
	}
	if _, err := store.ListDeployments(time.Time{}); err == nil {
		t.Fatal("ListDeployments with corrupted successful column = nil, want a Scan error")
	}
}
