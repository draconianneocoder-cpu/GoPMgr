// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/charts/matrix"
	"pmforge/internal/db"
	"pmforge/internal/documents"
)

func newRiskSyncTestApp(t *testing.T) (*App, *db.Database, db.Project) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "risk-sync.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	project, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Risk Sync"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	return &App{db: d}, d, project
}

func TestSyncRiskRegisterToMatrixMapsDocumentRows(t *testing.T) {
	app, _, project := newRiskSyncTestApp(t)
	chart, err := app.SaveChart(db.Chart{
		ProjectID: project.ID,
		Kind:      "risk_matrix",
		Title:     "Project Exposure",
		Data:      `{"items":[]}`,
		Config:    `{"owner":"controls"}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	document, err := app.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      string(documents.KindRiskRegister),
		Title:     "Risk Register",
		Content: `{
			"risk_matrix_ref":"` + chart.ID + `",
			"risks":[{
				"id":"R-1",
				"description":"Supplier delay",
				"kind":"risk",
				"probability":4,
				"impact":5,
				"owner":"Procurement",
				"status":"open",
				"mitigation":"Qualify backup supplier",
				"linked_task":"T-12"
			}]
		}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	updated, err := app.SyncRiskRegisterToMatrix(document.ID)
	if err != nil {
		t.Fatalf("SyncRiskRegisterToMatrix: %v", err)
	}
	if updated.Title != chart.Title || updated.Config != chart.Config {
		t.Fatalf("chart metadata changed: title=%q config=%s", updated.Title, updated.Config)
	}
	var got matrix.RiskMatrixDocument
	if err := json.Unmarshal([]byte(updated.Data), &got); err != nil {
		t.Fatalf("unmarshal synced chart: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(got.Items))
	}
	item := got.Items[0]
	if item.ID != "R-1" || item.Title != "Supplier delay" || item.Probability != 4 ||
		item.Impact != 5 || item.Owner != "Procurement" || item.Status != "open" ||
		item.Mitigation != "Qualify backup supplier" || item.LinkedTask != "T-12" {
		t.Fatalf("synced item = %+v", item)
	}
}

func TestSyncRiskRegisterToMatrixRefusesWrongChartKind(t *testing.T) {
	app, _, project := newRiskSyncTestApp(t)
	chart, err := app.SaveChart(db.Chart{
		ProjectID: project.ID,
		Kind:      "bar",
		Title:     "Not a risk matrix",
		Data:      `{"categories":[],"series":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	document, err := app.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      string(documents.KindRiskRegister),
		Content:   `{"risk_matrix_ref":"` + chart.ID + `","risks":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	_, err = app.SyncRiskRegisterToMatrix(document.ID)
	if err == nil || !strings.Contains(err.Error(), "risk_matrix") {
		t.Fatalf("error = %v, want risk_matrix kind refusal", err)
	}
}

func TestSyncRiskRegisterToMatrixLeavesChartUnchangedOnInvalidRows(t *testing.T) {
	app, d, project := newRiskSyncTestApp(t)
	const original = `{"items":[{"id":"R-old","title":"Keep me","kind":"risk","probability":2,"impact":2}]}`
	chart, err := app.SaveChart(db.Chart{
		ProjectID: project.ID,
		Kind:      "risk_matrix",
		Title:     "Project Exposure",
		Data:      original,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	document, err := app.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      string(documents.KindRiskRegister),
		Content:   `{"risk_matrix_ref":"` + chart.ID + `","risks":[{"id":"R-1","description":"Bad coordinate","probability":0,"impact":9}]}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	if _, err := app.SyncRiskRegisterToMatrix(document.ID); err == nil {
		t.Fatal("SyncRiskRegisterToMatrix accepted invalid rows")
	}
	unchanged, err := d.GetChart(chart.ID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	if unchanged.Data != original {
		t.Fatalf("chart data changed after refused sync: %s", unchanged.Data)
	}
}
