// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/rand"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

type failingIDReader struct{}

func (failingIDReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestProjectIDRequiresFull128BitEntropy(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "full entropy"
		if partial {
			name = "partial entropy leaves no project or audit event"
		}
		t.Run(name, func(t *testing.T) {
			d := newBackupTestDB(t)
			// The second block supplies the independent audit-event ID.
			entropy := "0123456789abcdef0123456789abcdef"
			if partial {
				entropy = "0123456789abcde"
			}
			t.Cleanup(replaceRandReader(t, strings.NewReader(entropy)))
			project, err := d.UpsertProject(Project{Name: "ID contract"})
			if partial {
				if !errors.Is(err, io.ErrUnexpectedEOF) || project.ID != "" {
					t.Fatalf("UpsertProject = %+v, %v; want no ID and incomplete entropy error", project, err)
				}
				for _, query := range []string{"SELECT COUNT(*) FROM project", "SELECT COUNT(*) FROM audit_events"} {
					var count int
					if err := d.Conn.QueryRow(query).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						t.Fatalf("%s = %d, want 0", query, count)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("UpsertProject: %v", err)
			}
			const want = "prj_30313233343536373839616263646566"
			if project.ID != want {
				t.Fatalf("project ID = %q, want %q", project.ID, want)
			}
			persisted, err := d.GetProject()
			if err != nil || persisted.ID != want {
				t.Fatalf("GetProject = %+v, %v; want ID %q", persisted, err, want)
			}
		})
	}
}

func TestGeneratedProjectIDFailsWhenEntropyUnavailable(t *testing.T) {
	d, err := InitDB(filepath.Join(t.TempDir(), "ids.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	restoreRand := replaceRandReader(t, failingIDReader{})
	defer restoreRand()

	_, err = d.UpsertProject(Project{Name: "Entropy test"})
	if err == nil || !strings.Contains(err.Error(), "generate project id") {
		t.Fatalf("UpsertProject error = %v, want generate project id error", err)
	}
}

func TestGeneratedEntityIDsFailWhenEntropyUnavailable(t *testing.T) {
	d, err := InitDB(filepath.Join(t.TempDir(), "entities.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	project, err := d.UpsertProject(Project{ID: "project-fixed", Name: "Entropy test"})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}

	tests := []struct {
		name string
		want string
		save func() error
	}{
		{
			name: "chart",
			want: "generate chart id",
			save: func() error {
				_, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "line", Title: "Chart"})
				return err
			},
		},
		{
			name: "document",
			want: "generate document id",
			save: func() error {
				_, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "Document"})
				return err
			},
		},
		{
			name: "stakeholder",
			want: "generate stakeholder id",
			save: func() error {
				_, err := d.SaveStakeholder(Stakeholder{ProjectID: project.ID, Name: "Stakeholder"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreRand := replaceRandReader(t, failingIDReader{})
			defer restoreRand()

			err := tt.save()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("save error = %v, want %q", err, tt.want)
			}
		})
	}
}

func replaceRandReader(t *testing.T, r io.Reader) func() {
	t.Helper()
	original := rand.Reader
	rand.Reader = r
	return func() {
		rand.Reader = original
	}
}
