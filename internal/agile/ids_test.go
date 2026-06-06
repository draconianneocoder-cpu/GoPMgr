// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"crypto/rand"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/db"
)

type failingAgileIDReader struct{}

func (failingAgileIDReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestAgileGeneratedIDsFailWhenEntropyUnavailable(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "agile-ids.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	project, err := d.UpsertProject(db.Project{ID: "project-fixed", Name: "Agile entropy test"})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := d.Conn.Exec(
		`INSERT INTO agile_boards (id, project_id, name, is_default) VALUES (?, ?, ?, 0)`,
		"board-fixed", project.ID, "Manual board",
	); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	store := NewStore(d.Conn, project.ID)

	tests := []struct {
		name string
		want string
		save func() error
	}{
		{
			name: "board",
			want: "generate board id",
			save: func() error {
				_, err := store.EnsureDefaultBoard()
				return err
			},
		},
		{
			name: "column",
			want: "generate column id",
			save: func() error {
				return store.SaveColumn(Column{BoardID: "board-fixed", Name: "Blocked"})
			},
		},
		{
			name: "work item",
			want: "generate work item id",
			save: func() error {
				_, err := store.SaveWorkItem(WorkItem{Title: "Story"})
				return err
			},
		},
		{
			name: "sprint",
			want: "generate sprint id",
			save: func() error {
				_, err := store.SaveSprint(Sprint{Name: "Sprint 1"})
				return err
			},
		},
		{
			name: "deployment",
			want: "generate deployment id",
			save: func() error {
				_, err := store.SaveDeployment(Deployment{Version: "1.0.0", Successful: true})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreRand := replaceAgileRandReader(t, failingAgileIDReader{})
			defer restoreRand()

			err := tt.save()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("save error = %v, want %q", err, tt.want)
			}
		})
	}
}

func replaceAgileRandReader(t *testing.T, r io.Reader) func() {
	t.Helper()
	original := rand.Reader
	rand.Reader = r
	return func() {
		rand.Reader = original
	}
}
