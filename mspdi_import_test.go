// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"pmforge/internal/db"
	"pmforge/internal/export"
)

const sampleImportXML = `<?xml version="1.0" encoding="UTF-8"?>
<Project xmlns="http://schemas.microsoft.com/project">
  <Title>Imported Plan</Title>
  <StartDate>2026-07-06T08:00:00</StartDate>
  <Tasks>
    <Task><UID>1</UID><Name>Pack</Name><Duration>PT16H0M0S</Duration></Task>
    <Task>
      <UID>2</UID><Name>Move</Name><Duration>PT8H0M0S</Duration>
      <PredecessorLink><PredecessorUID>1</PredecessorUID><Type>3</Type><LinkLag>4800</LinkLag></PredecessorLink>
    </Task>
  </Tasks>
</Project>`

func TestImportMSPDIFromBytes(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-import.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	// Project WITHOUT a start date: the import should adopt the file's.
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Import Target"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}

	c, err := app.importMSPDIFromBytes([]byte(sampleImportXML))
	if err != nil {
		t.Fatalf("importMSPDIFromBytes: %v", err)
	}
	if c.Kind != "cpm" || c.Title != "Imported Plan" {
		t.Errorf("chart = %s %q", c.Kind, c.Title)
	}

	var doc struct {
		Nodes []struct {
			ID       string  `json:"id"`
			Label    string  `json:"label"`
			Duration float64 `json:"duration"`
		} `json:"nodes"`
		Edges []struct {
			From, To, Label string
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(c.Data), &doc); err != nil {
		t.Fatalf("unmarshal chart data: %v", err)
	}
	if len(doc.Nodes) != 2 || len(doc.Edges) != 1 {
		t.Fatalf("doc shape: %d nodes %d edges", len(doc.Nodes), len(doc.Edges))
	}
	if doc.Edges[0].Label != "SS+1" {
		t.Errorf("edge label = %q, want SS+1", doc.Edges[0].Label)
	}

	proj, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.StartDate != "2026-07-06" {
		t.Errorf("project start date = %q, want adopted 2026-07-06", proj.StartDate)
	}

	// The imported chart must schedule end-to-end.
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("loader: %v (%d tasks)", err, len(tasks))
	}
	scheduleProjectTasks(proj, tasks)
	if tasks["2"].ES != 1 { // SS+1 off task 1's start (day 0)
		t.Errorf("Move ES = %v, want 1", tasks["2"].ES)
	}
	if tasks["2"].StartDate != "2026-07-07" {
		t.Errorf("Move StartDate = %q, want 2026-07-07", tasks["2"].StartDate)
	}
}

func TestImportMSPDIWithOptionsPersistsMappingReceipt(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-import-options.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Import Target"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}
	c, err := app.importMSPDIFromBytesWithOptions([]byte(sampleImportXML), export.MSPDIImportOptions{
		IncludeDependencies: false, IncludeProgress: false, IncludeAssignments: false,
	})
	if err != nil {
		t.Fatalf("importMSPDIFromBytesWithOptions: %v", err)
	}
	var config struct {
		Receipt export.MSPDIImportReceipt `json:"mspdi_import_receipt"`
	}
	if err := json.Unmarshal([]byte(c.Config), &config); err != nil {
		t.Fatalf("decode import receipt: %v", err)
	}
	if len(config.Receipt.ExcludedFields) != 3 {
		t.Fatalf("receipt exclusions = %#v, want three", config.Receipt.ExcludedFields)
	}
}

func TestImportMSPDIKeepsExistingStartDate(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-import2.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Has Date", StartDate: "2026-01-01"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}

	if _, err := app.importMSPDIFromBytes([]byte(sampleImportXML)); err != nil {
		t.Fatalf("importMSPDIFromBytes: %v", err)
	}
	proj, _ := d.GetProject()
	if proj.StartDate != "2026-01-01" {
		t.Errorf("existing start date must be preserved, got %q", proj.StartDate)
	}
}
