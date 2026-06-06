// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"path/filepath"
	"testing"

	"pmforge/internal/db"
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
