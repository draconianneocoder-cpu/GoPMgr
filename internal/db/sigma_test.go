// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/sigma/domain"
)

func newSigmaTestDB(t *testing.T) *Database {
	t.Helper()
	d, err := InitDB(filepath.Join(t.TempDir(), "sigma.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Conn.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "p1", "Sigma Test"); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO sigma_projects (id, title) VALUES (?, ?)`, "p1", "Sigma Test"); err != nil {
		t.Fatalf("insert sigma project: %v", err)
	}
	return d
}

func requireCorruptSigmaJSONError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected corrupt Sigma JSON to return an error")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode context in error, got %q", err)
	}
}

func TestSigmaGetCharterRejectsMalformedJSON(t *testing.T) {
	d := newSigmaTestDB(t)
	if _, err := d.Conn.Exec(
		`INSERT INTO sigma_charters (id, project_id, scope_in, scope_out, ctqs) VALUES (?, ?, ?, ?, ?)`,
		"charter-p1", "p1", "[", "[]", "[]",
	); err != nil {
		t.Fatalf("insert corrupt charter: %v", err)
	}

	_, err := d.SigmaGetCharter("p1")
	requireCorruptSigmaJSONError(t, err)
}

func TestSigmaGettersRejectMalformedJSON(t *testing.T) {
	tests := []struct {
		name   string
		insert string
		get    func(*Database) error
	}{
		{
			name:   "fishbone",
			insert: `INSERT INTO sigma_fishbones (id, project_id, data_json) VALUES ('fishbone-p1', 'p1', '[')`,
			get: func(d *Database) error {
				_, err := d.SigmaGetFishbone("p1")
				return err
			},
		},
		{
			name:   "solutions",
			insert: `INSERT INTO sigma_solutions (id, project_id, data_json) VALUES ('solutions-p1', 'p1', '[')`,
			get: func(d *Database) error {
				_, err := d.SigmaGetSolutions("p1")
				return err
			},
		},
		{
			name:   "control plan",
			insert: `INSERT INTO sigma_control_plans (id, project_id, data_json) VALUES ('controlplan-p1', 'p1', '[')`,
			get: func(d *Database) error {
				_, err := d.SigmaGetControlPlan("p1")
				return err
			},
		},
		{
			name:   "sipoc",
			insert: `INSERT INTO sigma_sipocs (id, project_id, data_json) VALUES ('sipoc-p1', 'p1', '[')`,
			get: func(d *Database) error {
				_, err := d.SigmaGetSIPOC("p1")
				return err
			},
		},
		{
			name:   "voc",
			insert: `INSERT INTO sigma_voc (id, project_id, data_json) VALUES ('voc-p1', 'p1', '[')`,
			get: func(d *Database) error {
				_, err := d.SigmaGetVoC("p1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newSigmaTestDB(t)
			if _, err := d.Conn.Exec(tt.insert); err != nil {
				t.Fatalf("insert corrupt %s: %v", tt.name, err)
			}
			requireCorruptSigmaJSONError(t, tt.get(d))
		})
	}
}

func TestSigmaFishboneRoundTripPreservesBranches(t *testing.T) {
	d := newSigmaTestDB(t)
	want := domain.FishboneData{
		ProblemStatement: "late delivery",
		Branches: []domain.FishboneBranch{
			{
				Category: "Method",
				Causes: []domain.Cause{
					{ID: "c1", Description: "handoff gap", IsRootCause: true},
				},
			},
		},
	}

	if err := d.SigmaSaveFishbone(want, "p1"); err != nil {
		t.Fatalf("save fishbone: %v", err)
	}
	got, err := d.SigmaGetFishbone("p1")
	if err != nil {
		t.Fatalf("get fishbone: %v", err)
	}
	if got.ProblemStatement != want.ProblemStatement {
		t.Fatalf("problem statement = %q, want %q", got.ProblemStatement, want.ProblemStatement)
	}
	if len(got.Branches) != 1 || got.Branches[0].Category != "Method" {
		t.Fatalf("branches = %#v, want Method branch", got.Branches)
	}
	if len(got.Branches[0].Causes) != 1 || got.Branches[0].Causes[0].Description != "handoff gap" {
		t.Fatalf("causes = %#v, want saved cause", got.Branches[0].Causes)
	}
}
