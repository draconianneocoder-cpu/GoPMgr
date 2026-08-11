// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"testing"
)

func countScenarioCharts(t *testing.T, d *Database) int {
	t.Helper()
	var n int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM scenario_charts`).Scan(&n); err != nil {
		t.Fatalf("count scenario_charts: %v", err)
	}
	return n
}

func countBaselines(t *testing.T, d *Database) int {
	t.Helper()
	var n int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM baselines`).Scan(&n); err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	return n
}

func countScenarios(t *testing.T, d *Database) int {
	t.Helper()
	var n int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&n); err != nil {
		t.Fatalf("count scenarios: %v", err)
	}
	return n
}

// TestDeleteScenario_MissingIDIsANoOpNotAnError mirrors
// TestDeleteDocument_MissingIDIsANoOpNotAnError's pattern: DeleteScenario
// is irreversible, so a caller retrying a delete of an ID that already
// doesn't exist (e.g. after an ambiguous first-attempt response) must get
// a clean nil, not an error -- and, more importantly, must not leave a
// phantom scenario.delete audit event that would misrepresent the audit
// trail as having deleted something that was never there.
func TestDeleteScenario_MissingIDIsANoOpNotAnError(t *testing.T) {
	d := newBackupTestDB(t)
	before := countAuditEvents(t, d)

	if err := d.DeleteScenario("does-not-exist"); err != nil {
		t.Errorf("DeleteScenario(missing) = %v, want nil (no-op)", err)
	}

	after := countAuditEvents(t, d)
	if after != before {
		t.Errorf("audit_events count went from %d to %d after a no-op delete, want unchanged (no phantom scenario.delete event)", before, after)
	}
}

// TestBranchScenarioChart_RejectsChartOutsideScenarioProject is a
// trust-boundary guard: a scenario branch must only ever reference
// charts from its own project. This guard (scenarios.go ~272) fires
// before BranchScenarioChart even opens a transaction, so nothing could
// have been written regardless -- the row-count assertion pins that
// property as a regression guard against a future refactor that moves
// the check after the INSERT, not as evidence of today's rollback
// behavior.
func TestBranchScenarioChart_RejectsChartOutsideScenarioProject(t *testing.T) {
	d := newBackupTestDB(t)
	projectA, err := d.UpsertProject(Project{Name: "Project A"})
	if err != nil {
		t.Fatalf("UpsertProject A: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: projectA.ID, Name: "Scenario A", IsActive: true})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}

	// A second project, purely as a SQL-predicate-correctness fixture for
	// this cross-project guard -- not a claim that multi-project files
	// are supported (this application's data model is one project per
	// file; see project.go's GetProject doc comment).
	const projectB = "prj-branch-guard-b"
	insertBareProjectRow(t, d, projectB)
	chartB, err := d.SaveChart(Chart{ProjectID: projectB, Kind: "cpm", Title: "Chart in B"})
	if err != nil {
		t.Fatalf("SaveChart in project B: %v", err)
	}

	before := countScenarioCharts(t, d)
	if _, err := d.BranchScenarioChart(scenario.ID, chartB.ID, ""); err == nil {
		t.Fatal("BranchScenarioChart across projects = nil error, want a rejection")
	}
	after := countScenarioCharts(t, d)
	if after != before {
		t.Errorf("scenario_charts count went from %d to %d, want unchanged (no scenario chart branched from a cross-project source)", before, after)
	}
}

// TestBranchScenarioChart_RejectsBaselineOutsideScenarioProject is the
// same class of guard as above, for an explicitly-supplied baselineID
// (scenarios.go ~284) rather than the chart itself. The fixture
// deliberately keeps baseline.ChartID pointed at the real chart being
// branched (via a direct SQL insert -- SaveBaseline itself enforces no
// project/chart consistency, confirmed by reading baselines.go) so that
// only the ProjectID mismatch is exercised; a baseline for a mismatched
// chart in another project would also trip the separate ChartID guard
// (scenarios.go ~287, its own test below) and this test wouldn't isolate
// which guard actually caught it -- confirmed by fault-seeding (see the
// increment's memory note): seeding the ProjectID guard alone with a
// same-project-mismatched-chart baseline left this test passing, because
// the ChartID guard fired instead.
func TestBranchScenarioChart_RejectsBaselineOutsideScenarioProject(t *testing.T) {
	d := newBackupTestDB(t)
	projectA, err := d.UpsertProject(Project{Name: "Project A"})
	if err != nil {
		t.Fatalf("UpsertProject A: %v", err)
	}
	chartA, err := d.SaveChart(Chart{ProjectID: projectA.ID, Kind: "cpm", Title: "Chart in A"})
	if err != nil {
		t.Fatalf("SaveChart in project A: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: projectA.ID, Name: "Scenario A", IsActive: true})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}

	const projectB = "prj-branch-guard-baseline-b"
	insertBareProjectRow(t, d, projectB)
	if _, err := d.Conn.Exec(
		`INSERT INTO baselines (id, project_id, chart_id, name, data, created_at)
		 VALUES ('bl-cross-project', ?, ?, 'Baseline claiming to be in B', '{}', '2026-01-01T00:00:00Z')`,
		projectB, chartA.ID,
	); err != nil {
		t.Fatalf("seed cross-project baseline: %v", err)
	}

	before := countScenarioCharts(t, d)
	if _, err := d.BranchScenarioChart(scenario.ID, chartA.ID, "bl-cross-project"); err == nil {
		t.Fatal("BranchScenarioChart with a cross-project baseline = nil error, want a rejection")
	}
	after := countScenarioCharts(t, d)
	if after != before {
		t.Errorf("scenario_charts count went from %d to %d, want unchanged", before, after)
	}
}

// TestBranchScenarioChart_RejectsBaselineForADifferentChart guards
// against passing a real, same-project baseline that happens to belong
// to a different chart than the one being branched -- data that would
// otherwise be silently mismatched (scenarios.go ~287).
func TestBranchScenarioChart_RejectsBaselineForADifferentChart(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Mismatch Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart1, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Chart 1"})
	if err != nil {
		t.Fatalf("SaveChart 1: %v", err)
	}
	chart2, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Chart 2"})
	if err != nil {
		t.Fatalf("SaveChart 2: %v", err)
	}
	baseline1, err := d.SaveBaseline(Baseline{ProjectID: project.ID, ChartID: chart1.ID, Name: "Baseline for chart 1"})
	if err != nil {
		t.Fatalf("SaveBaseline for chart 1: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "Mismatch scenario", IsActive: true})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}

	before := countScenarioCharts(t, d)
	if _, err := d.BranchScenarioChart(scenario.ID, chart2.ID, baseline1.ID); err == nil {
		t.Fatal("BranchScenarioChart(chart2, baseline-for-chart1) = nil error, want a rejection")
	}
	after := countScenarioCharts(t, d)
	if after != before {
		t.Errorf("scenario_charts count went from %d to %d, want unchanged", before, after)
	}
}

// TestPromoteScenarioChartToBaseline_RejectsCrossProjectSourceChart
// exercises PromoteScenarioChartToBaseline's own project-isolation guard
// (scenarios.go ~471). Because BranchScenarioChart already validates
// project match at branch time, this state can't arise through the
// normal API -- so it's simulated directly via SQL, treating this as a
// defense-in-depth / corrupted-state check on the guard itself, not a
// claim that this state is reachable in ordinary use. The guard runs
// inside an open transaction, before either write (saveBaselineTx, the
// approval checkpoint) -- the assertions pin that both are skipped, not
// merely that an error is returned.
func TestPromoteScenarioChartToBaseline_RejectsCrossProjectSourceChart(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Promote Guard Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Chart"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "Promote Guard Scenario", IsActive: true})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}
	branched, err := d.BranchScenarioChart(scenario.ID, chart.ID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}

	const otherProjectID = "prj-promote-guard-other"
	insertBareProjectRow(t, d, otherProjectID)
	if _, err := d.Conn.Exec(`UPDATE scenario_charts SET project_id = ? WHERE id = ?`, otherProjectID, branched.ID); err != nil {
		t.Fatalf("corrupt scenario_charts.project_id: %v", err)
	}

	beforeBaselines := countBaselines(t, d)
	beforeEvents := countAuditEvents(t, d)

	if _, err := d.PromoteScenarioChartToBaseline(branched.ID, "Should not be created"); err == nil {
		t.Fatal("PromoteScenarioChartToBaseline with a cross-project source chart = nil error, want a rejection")
	}

	afterBaselines := countBaselines(t, d)
	if afterBaselines != beforeBaselines {
		t.Errorf("baselines count went from %d to %d, want unchanged (the guard must fire before saveBaselineTx)", beforeBaselines, afterBaselines)
	}
	afterEvents := countAuditEvents(t, d)
	if afterEvents != beforeEvents {
		t.Errorf("audit_events count went from %d to %d, want unchanged (no approval checkpoint for a rejected promotion)", beforeEvents, afterEvents)
	}
}

// TestPromoteScenarioChartToBaseline_FailsCleanlyWhenSourceChartWasDeleted
// covers a real lifecycle case: scenario_charts carries no foreign key
// to charts (confirmed by reading sqlite.go's schema -- only scenario_id
// and project_id are FK-constrained), so deleting the source chart after
// branching leaves the scenario chart's source_chart_id dangling by
// design, not by omission. PromoteScenarioChartToBaseline must fail
// cleanly (ErrNoChart) rather than panic or write a baseline with
// corrupted/zero-value provenance.
func TestPromoteScenarioChartToBaseline_FailsCleanlyWhenSourceChartWasDeleted(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Orphan Source Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Doomed Chart"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "Orphan Scenario", IsActive: true})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}
	branched, err := d.BranchScenarioChart(scenario.ID, chart.ID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}
	if err := d.DeleteChart(chart.ID); err != nil {
		t.Fatalf("DeleteChart: %v", err)
	}

	if _, err := d.PromoteScenarioChartToBaseline(branched.ID, "Orphaned promotion"); err != ErrNoChart {
		t.Errorf("PromoteScenarioChartToBaseline after source chart deleted: err = %v, want ErrNoChart", err)
	}
}

// TestSaveScenario_RejectsMissingRequiredFields covers SaveScenario's two
// required-field guards (scenarios.go ~58-63), both of which return
// before opening a transaction -- nothing is written either way, pinned
// as a regression guard. Fault-seeding both guards together (disabling
// both at once, since that's the seed that was actually run) found they
// are not equally load-bearing: with both disabled, the "missing name"
// subtest failed (SaveScenario let a blank name through -- name has no
// FK and NOT NULL accepts ""), but the "missing project_id" subtest
// still passed even with its own guard disabled, because the scenarios
// table's `FOREIGN KEY (project_id) REFERENCES project(id)` constraint
// rejects an empty project_id INSTEAD -- an empty string matches no real
// project row, so the INSERT fails regardless of the app-level check.
// This test therefore has no observed sensitivity to the project_id
// guard specifically (its subtest would pass even if that guard were
// deleted); the guard is still worth keeping for a clearer error message
// than a raw SQLite constraint violation, but that's a code-quality
// argument, not one this test provides evidence for.
func TestSaveScenario_RejectsMissingRequiredFields(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Validation Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	t.Run("missing project_id", func(t *testing.T) {
		before := countScenarios(t, d)
		if _, err := d.SaveScenario(Scenario{Name: "No project"}); err == nil {
			t.Fatal("SaveScenario without project_id = nil error, want a rejection")
		}
		if after := countScenarios(t, d); after != before {
			t.Errorf("scenarios count went from %d to %d, want unchanged", before, after)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		before := countScenarios(t, d)
		if _, err := d.SaveScenario(Scenario{ProjectID: project.ID}); err == nil {
			t.Fatal("SaveScenario without name = nil error, want a rejection")
		}
		if after := countScenarios(t, d); after != before {
			t.Errorf("scenarios count went from %d to %d, want unchanged", before, after)
		}
	})
}

// TestSaveScenarioChart_RequiresID covers the c.ID == "" guard
// (scenarios.go ~380-382) and the nonexistent-ID error path
// (getScenarioChartTx failing inside the transaction) -- unlike
// BranchScenarioChart's create flow, SaveScenarioChart never generates
// an ID; it only ever updates a chart branched by BranchScenarioChart.
//
// Fault-seeding found the c.ID == "" guard is an equivalent mutation:
// disabling it and calling SaveScenarioChart with a blank ID still fails,
// because getScenarioChartTx(tx, "") naturally returns ErrNoScenarioChart
// for an ID that matches no row (IDs are always non-empty, generated via
// newID()) -- there is no reachable input for which this guard's absence
// changes the outcome. The guard is still worth keeping (a distinct
// "id is required" error is more useful to a caller than "scenario
// chart not found"), but this test cannot and does not claim to prove
// it's load-bearing; only the second assertion (a real, non-empty,
// nonexistent ID) is a live sensitivity check.
func TestSaveScenarioChart_RequiresID(t *testing.T) {
	d := newBackupTestDB(t)

	if _, err := d.SaveScenarioChart(ScenarioChart{Title: "No ID"}); err == nil {
		t.Fatal("SaveScenarioChart without an ID = nil error, want a rejection")
	}
	if _, err := d.SaveScenarioChart(ScenarioChart{ID: "does-not-exist"}); err != ErrNoScenarioChart {
		t.Errorf("SaveScenarioChart(nonexistent ID) err = %v, want ErrNoScenarioChart", err)
	}
}

// TestSaveScenarioChart_BlankFieldBehaviorDiffersByField pins a real,
// verified (not assumed) asymmetry in how SaveScenarioChart treats blank
// input across its three editable fields: a blank Title falls back to
// the *existing* stored title (preserved), while blank Data and Config
// both fall back to the literal "{}" (reset), not to their existing
// values. The doc comment ("updates the editable fields... remain
// immutable" -- referring to project/source/kind fields) does not
// mention this difference. This is disclosed as an undocumented
// behavior worth confirming intent on, not asserted to be a defect: a
// caller submitting a partial edit with Title omitted keeps the old
// title, but the same caller omitting Data silently wipes it to an
// empty object rather than leaving it alone.
func TestSaveScenarioChart_BlankFieldBehaviorDiffersByField(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Blank Field Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{
		ProjectID: project.ID, Kind: "cpm", Title: "Live Chart",
		Data: `{"a":1}`, Config: `{"b":2}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "Blank Field Scenario", IsActive: true})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}
	branched, err := d.BranchScenarioChart(scenario.ID, chart.ID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}
	if branched.Title == "" || branched.Data == "{}" || branched.Config == "{}" {
		t.Fatalf("fixture precondition failed, expected non-blank branched fields: %+v", branched)
	}

	saved, err := d.SaveScenarioChart(ScenarioChart{ID: branched.ID}) // every editable field left blank
	if err != nil {
		t.Fatalf("SaveScenarioChart with all fields blank: %v", err)
	}
	if saved.Title != branched.Title {
		t.Errorf("Title = %q after a blank-Title save, want it preserved as %q", saved.Title, branched.Title)
	}
	if saved.Data != "{}" {
		t.Errorf("Data = %q after a blank-Data save, want it reset to \"{}\"", saved.Data)
	}
	if saved.Config != "{}" {
		t.Errorf("Config = %q after a blank-Config save, want it reset to \"{}\"", saved.Config)
	}
}
