// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopmgr/internal/agile"
	"gopmgr/internal/db"
)

// newSeederTestDB returns an initialised database plus the ID of a real
// project row (Seeder's FK-constrained inserts need one to attach to).
func newSeederTestDB(t *testing.T) (*db.Database, string) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "seed-test.gopmgr"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	proj, err := d.UpsertProject(db.Project{Name: "Seed Test Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	return d, proj.ID
}

// chartSeeds and documentSeeds mirror applyOne's dispatch table so tests can
// drive every case without duplicating the seed-string literals by hand.
var chartSeeds = []string{"wbs", "cpm", "fishbone", "control", "pareto", "cumulative_flow", "swot"}
var documentSeeds = []string{
	"charter", "plan_word", "statement_of_work", "scope_statement",
	"risk_register", "communication_plan", "status_report", "stakeholder_analysis_doc",
}

func TestSeeder_ChartSeeds(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	for _, seed := range chartSeeds {
		t.Run(seed, func(t *testing.T) {
			receipt, err := s.applyOne(seed)
			if err != nil {
				t.Fatalf("applyOne(%q): %v", seed, err)
			}
			if receipt == nil {
				t.Fatalf("applyOne(%q) returned a nil receipt", seed)
				return
			}
			if receipt.Kind != "chart" {
				t.Errorf("receipt.Kind = %q, want %q", receipt.Kind, "chart")
			}
			if receipt.Seed != seed {
				t.Errorf("receipt.Seed = %q, want %q", receipt.Seed, seed)
			}
			if receipt.ID == "" {
				t.Error("receipt.ID is empty")
			}
			got, err := d.GetChart(receipt.ID)
			if err != nil {
				t.Fatalf("GetChart(%q): %v", receipt.ID, err)
			}
			if got.Kind != seed {
				t.Errorf("saved chart Kind = %q, want %q", got.Kind, seed)
			}
		})
	}
}

func TestSeeder_DocumentSeeds(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	for _, seed := range documentSeeds {
		t.Run(seed, func(t *testing.T) {
			receipt, err := s.applyOne(seed)
			if err != nil {
				t.Fatalf("applyOne(%q): %v", seed, err)
			}
			if receipt == nil {
				t.Fatalf("applyOne(%q) returned a nil receipt", seed)
				return
			}
			if receipt.Kind != "document" {
				t.Errorf("receipt.Kind = %q, want %q", receipt.Kind, "document")
			}
			if receipt.Name == "" {
				t.Error("receipt.Name is empty")
			}
			if receipt.ID == "" {
				t.Error("receipt.ID is empty")
			}
		})
	}
}

func TestSeeder_Kanban(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	receipt, err := s.applyOne("kanban")
	if err != nil {
		t.Fatalf("applyOne(kanban): %v", err)
	}
	if receipt == nil || receipt.Kind != "board" || receipt.ID == "" {
		t.Fatalf("kanban receipt = %+v, want a non-empty board receipt", receipt)
	}

	// seedKanban's doc comment says calling it twice is fine (idempotent
	// via EnsureDefaultBoard) -- pin that explicitly rather than trusting
	// the comment.
	second, err := s.applyOne("kanban")
	if err != nil {
		t.Fatalf("applyOne(kanban) second call: %v", err)
	}
	if second.ID != receipt.ID {
		t.Errorf("second kanban seed returned board ID %q, want the same board %q (not idempotent)", second.ID, receipt.ID)
	}
}

func TestSeeder_Backlog(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	receipt, err := s.applyOne("backlog")
	if err != nil {
		t.Fatalf("applyOne(backlog): %v", err)
	}
	if receipt == nil || receipt.Kind != "board" {
		t.Fatalf("backlog receipt = %+v, want a board receipt", receipt)
	}

	store := agile.NewStore(d.Conn, projID)
	items, err := store.ListWorkItems("", "", "")
	if err != nil {
		t.Fatalf("ListWorkItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 seeded backlog items, got %d", len(items))
	}
	for _, wi := range items {
		if wi.State != "backlog" {
			t.Errorf("seeded item %q has State %q, want %q", wi.Title, wi.State, "backlog")
		}
	}
}

func TestSeeder_FirstSprint(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	receipt, err := s.applyOne("sprint1")
	if err != nil {
		t.Fatalf("applyOne(sprint1): %v", err)
	}
	if receipt == nil || receipt.Kind != "sprint" || receipt.Name != "Sprint 1" {
		t.Fatalf("sprint1 receipt = %+v, want Sprint 1", receipt)
	}
}

func TestSeeder_UnknownSeedIsSilentlySkipped(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	receipt, err := s.applyOne("not-a-real-seed")
	if err != nil {
		t.Fatalf("applyOne(unknown): %v", err)
	}
	if receipt != nil {
		t.Fatalf("applyOne(unknown) receipt = %+v, want nil", receipt)
	}
}

func TestSeeder_ApplyRunsSeedsInOrderAndCollectsReceipts(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	receipts, err := s.Apply([]string{"kanban", "wbs", "not-a-real-seed", "backlog"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// The unknown seed contributes no receipt; the other three do.
	if len(receipts) != 3 {
		t.Fatalf("Apply returned %d receipts, want 3: %+v", len(receipts), receipts)
	}
	wantSeeds := []string{"kanban", "wbs", "backlog"}
	for i, r := range receipts {
		if r.Seed != wantSeeds[i] {
			t.Errorf("receipts[%d].Seed = %q, want %q (order not preserved)", i, r.Seed, wantSeeds[i])
		}
	}
}

// TestSeeder_ApplyReturnsPartialReceiptsOnFailure pins Apply's documented
// contract: receipts for seeds that succeeded BEFORE a failure are
// returned alongside the error, not discarded. A healthy DB can't exercise
// this -- every seed would just succeed -- so a SQLite trigger blocks the
// second seed's insert (mirrors the fault-injection pattern in
// internal/users/store_test.go's TestAuthenticateReturnsLastLoginUpdateError).
func TestSeeder_ApplyReturnsPartialReceiptsOnFailure(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	if _, err := d.Conn.Exec(`
		CREATE TRIGGER block_wbs_chart
		BEFORE INSERT ON charts
		WHEN NEW.kind = 'wbs'
		BEGIN
			SELECT RAISE(ABORT, 'wbs chart insert blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	receipts, err := s.Apply([]string{"kanban", "wbs", "backlog"})
	if err == nil {
		t.Fatal("expected Apply to fail on the blocked wbs insert")
	}
	if !strings.Contains(err.Error(), "seed wbs:") {
		t.Errorf("error %q does not name the failing seed", err.Error())
	}
	// kanban (before the failure) succeeded; backlog (after) never ran.
	if len(receipts) != 1 || receipts[0].Seed != "kanban" {
		t.Fatalf("receipts = %+v, want exactly the kanban receipt from before the failure", receipts)
	}
}

// TestSeeder_SeedDocumentUnknownKind calls seedDocument directly (an
// unexported helper, same-package test) with a kind string that isn't in
// applyOne's dispatch table. All 8 kinds applyOne actually dispatches are
// confirmed registered in internal/documents, so this branch is
// unreachable through the public Apply/applyOne path today -- but
// seedDocument's kind parameter is a plain string, not constrained by the
// type system, so the defensive check itself is real code worth pinning
// directly.
func TestSeeder_SeedDocumentUnknownKind(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)

	_, err := s.seedDocument("not-a-real-kind", "Title")
	if err == nil || !strings.Contains(err.Error(), "unknown document kind") {
		t.Fatalf("seedDocument(unknown kind) error = %v, want an unknown document kind error", err)
	}
}

// The remaining four tests fault-inject each seed handler's DB write via a
// SQLite trigger (same technique as TestSeeder_ApplyReturnsPartialReceiptsOnFailure)
// to cover the error-propagation branch a healthy DB can't reach.

func TestSeeder_SeedDocumentPropagatesSaveError(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)
	mustBlockInsert(t, d, "documents", "block_documents")

	if _, err := s.applyOne("charter"); err == nil {
		t.Fatal("expected applyOne(charter) to propagate the blocked SaveDocument error")
	}
}

func TestSeeder_SeedKanbanPropagatesBoardError(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)
	mustBlockInsert(t, d, "agile_boards", "block_agile_boards")

	if _, err := s.applyOne("kanban"); err == nil {
		t.Fatal("expected applyOne(kanban) to propagate the blocked EnsureDefaultBoard error")
	}
}

func TestSeeder_SeedBacklogPropagatesWorkItemError(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)
	mustBlockInsert(t, d, "agile_work_items", "block_agile_work_items")

	if _, err := s.applyOne("backlog"); err == nil {
		t.Fatal("expected applyOne(backlog) to propagate the blocked SaveWorkItem error")
	}
}

func TestSeeder_SeedFirstSprintPropagatesSprintError(t *testing.T) {
	d, projID := newSeederTestDB(t)
	s := NewSeeder(d, projID)
	mustBlockInsert(t, d, "agile_sprints", "block_agile_sprints")

	if _, err := s.applyOne("sprint1"); err == nil {
		t.Fatal("expected applyOne(sprint1) to propagate the blocked SaveSprint error")
	}
}

var identifierRE = regexp.MustCompile(`^[a-z_]+$`)

// mustBlockInsert installs a SQLite trigger that aborts every INSERT into
// table, so the next write against it returns an error a healthy DB
// wouldn't produce. SQLite can't parameterize identifiers in DDL, so table
// and triggerName are validated against a strict whitelist rather than
// interpolated as-is -- this is test-only, call-site-constant input today,
// but the check keeps the pattern from being copyable into anything else
// without carrying its own guard.
func mustBlockInsert(t *testing.T, d *db.Database, table, triggerName string) {
	t.Helper()
	if !identifierRE.MatchString(table) || !identifierRE.MatchString(triggerName) {
		t.Fatalf("mustBlockInsert: table %q / triggerName %q must match %s", table, triggerName, identifierRE)
	}
	if _, err := d.Conn.Exec(`
		CREATE TRIGGER ` + triggerName + `
		BEFORE INSERT ON ` + table + `
		BEGIN
			SELECT RAISE(ABORT, '` + triggerName + `');
		END;
	`); err != nil {
		t.Fatalf("create trigger on %s: %v", table, err)
	}
}

func TestCoalesce(t *testing.T) {
	if got := coalesce("", "fallback"); got != "fallback" {
		t.Errorf("coalesce(empty) = %q, want %q", got, "fallback")
	}
	if got := coalesce("value", "fallback"); got != "value" {
		t.Errorf("coalesce(non-empty) = %q, want %q", got, "value")
	}
}
