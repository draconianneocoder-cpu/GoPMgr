// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/sigma/domain"
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

// TestSigmaSaveFishbone_SecondSaveOverwritesFirst exercises the
// ON CONFLICT(project_id) DO UPDATE path -- the round trip test above
// only ever saves once (a create), never a second time against an
// existing row, so the update branch had no coverage at all.
func TestSigmaSaveFishbone_SecondSaveOverwritesFirst(t *testing.T) {
	d := newSigmaTestDB(t)
	first := domain.FishboneData{
		ProblemStatement: "late delivery",
		Branches:         []domain.FishboneBranch{{Category: "Method"}},
	}
	if err := d.SigmaSaveFishbone(first, "p1"); err != nil {
		t.Fatalf("save first: %v", err)
	}

	second := domain.FishboneData{
		ProblemStatement: "quality defects",
		Branches:         []domain.FishboneBranch{{Category: "Machine"}, {Category: "Material"}},
	}
	if err := d.SigmaSaveFishbone(second, "p1"); err != nil {
		t.Fatalf("save second: %v", err)
	}

	got, err := d.SigmaGetFishbone("p1")
	if err != nil {
		t.Fatalf("get fishbone: %v", err)
	}
	if got.ProblemStatement != second.ProblemStatement {
		t.Fatalf("problem statement = %q, want %q (second save must overwrite, not append)", got.ProblemStatement, second.ProblemStatement)
	}
	if len(got.Branches) != 2 || got.Branches[0].Category != "Machine" || got.Branches[1].Category != "Material" {
		t.Fatalf("branches = %#v, want [Machine Material] (second save's branches, not merged with the first)", got.Branches)
	}
}

// TestSigmaGetFishbone_ReturnsSixStandardCategoriesWhenNoRowExists covers
// the sql.ErrNoRows default path, previously untested -- every existing
// test either saved a fishbone first or inserted corrupt JSON directly,
// so the "no fishbone yet" happy path (what a brand-new Sigma project's
// Fishbone tab actually renders on first open) had no test at all.
func TestSigmaGetFishbone_ReturnsSixStandardCategoriesWhenNoRowExists(t *testing.T) {
	d := newSigmaTestDB(t)
	got, err := d.SigmaGetFishbone("p1")
	if err != nil {
		t.Fatalf("get fishbone (no row): %v", err)
	}
	wantCategories := []string{"Man", "Machine", "Method", "Material", "Measurement", "Environment"}
	if len(got.Branches) != len(wantCategories) {
		t.Fatalf("branches = %#v, want %d standard categories", got.Branches, len(wantCategories))
	}
	for i, want := range wantCategories {
		if got.Branches[i].Category != want {
			t.Errorf("branches[%d].Category = %q, want %q", i, got.Branches[i].Category, want)
		}
	}
}

// TestSigmaGetFishbone_ParsesLegacyArrayOnlyFormatAndFallsBackToColumnProblemStatement
// covers two real, previously-untested branches together: the legacy
// decode fallback (data_json as a bare JSON array of branches, the format
// before FishboneData wrapped Branches in a {problem_statement, branches}
// object) and the ProblemStatement column fallback (fb.ProblemStatement
// == "" after decode, since the legacy array format has no embedded
// problem statement at all). A legacy row surviving from before this
// wrapping change must still render correctly, not fail to decode or
// silently lose its problem statement.
func TestSigmaGetFishbone_ParsesLegacyArrayOnlyFormatAndFallsBackToColumnProblemStatement(t *testing.T) {
	d := newSigmaTestDB(t)
	legacyBranchesJSON := `[{"category":"Man","causes":[{"id":"c1","description":"training gap","is_root_cause":true}]}]`
	if _, err := d.Conn.Exec(
		`INSERT INTO sigma_fishbones (id, project_id, problem_statement, data_json) VALUES (?, ?, ?, ?)`,
		"fishbone-p1", "p1", "legacy problem statement", legacyBranchesJSON,
	); err != nil {
		t.Fatalf("insert legacy fishbone row: %v", err)
	}

	got, err := d.SigmaGetFishbone("p1")
	if err != nil {
		t.Fatalf("get legacy fishbone: %v", err)
	}
	if len(got.Branches) != 1 || got.Branches[0].Category != "Man" {
		t.Fatalf("branches = %#v, want the legacy array's single Man branch", got.Branches)
	}
	if len(got.Branches[0].Causes) != 1 || got.Branches[0].Causes[0].Description != "training gap" {
		t.Fatalf("causes = %#v, want the legacy array's cause", got.Branches[0].Causes)
	}
	if got.ProblemStatement != "legacy problem statement" {
		t.Fatalf("ProblemStatement = %q, want the problem_statement column's value (the legacy JSON format has no embedded problem statement to prefer)", got.ProblemStatement)
	}
}

// TestSigmaGetFishbone_ModernFormatWithEmptyEmbeddedStatementFallsBackToColumn
// covers the column fallback on the MODERN decode path, not just the
// legacy one above. SigmaSaveFishbone always writes the same problem
// statement to both the column and the embedded JSON, so the two can't
// diverge through the normal save/get API -- but a row written directly
// by SQL, or migrated from an older schema into the modern
// {"problem_statement":...,"branches":[...]} shape without carrying the
// statement forward, can still decode successfully with an empty
// ProblemStatement field. That must still fall back to the column, the
// same as the legacy no-embedded-statement case, or a real row like this
// would silently render a blank problem statement.
func TestSigmaGetFishbone_ModernFormatWithEmptyEmbeddedStatementFallsBackToColumn(t *testing.T) {
	d := newSigmaTestDB(t)
	modernJSONNoStatement := `{"problem_statement":"","branches":[{"category":"Machine"}]}`
	if _, err := d.Conn.Exec(
		`INSERT INTO sigma_fishbones (id, project_id, problem_statement, data_json) VALUES (?, ?, ?, ?)`,
		"fishbone-p1", "p1", "column problem statement", modernJSONNoStatement,
	); err != nil {
		t.Fatalf("insert modern fishbone row with empty embedded statement: %v", err)
	}

	got, err := d.SigmaGetFishbone("p1")
	if err != nil {
		t.Fatalf("get fishbone: %v", err)
	}
	if got.ProblemStatement != "column problem statement" {
		t.Fatalf("ProblemStatement = %q, want the problem_statement column's value (modern decode succeeded but left ProblemStatement empty)", got.ProblemStatement)
	}
	if len(got.Branches) != 1 || got.Branches[0].Category != "Machine" {
		t.Fatalf("branches = %#v, want the modern JSON's Machine branch", got.Branches)
	}
}
