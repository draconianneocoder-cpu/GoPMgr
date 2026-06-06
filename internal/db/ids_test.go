// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
