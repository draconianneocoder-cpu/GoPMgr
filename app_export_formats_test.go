// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

// TestExportScheduleReportAllFormats drives every ExportScheduleReport*
// Wails method — each a one-line delegation to the shared
// exportScheduleReportAs — through a real, successful export. No CPM
// chart is needed: loadCurrentProjectSchedule's failure on a project with
// no schedule data is documented as non-fatal (falls back to an empty
// kernelTasks map), so this exercises the full guard-to-file-write path
// on the cheapest fixture that still proves each format actually renders
// and reaches disk, not just that no error was returned.
func TestExportScheduleReportAllFormats(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	cases := []struct {
		name string
		call func() (string, error)
		ext  string
	}{
		{"DOCX", app.ExportScheduleReportDOCX, ".docx"},
		{"ODT", app.ExportScheduleReportODT, ".odt"},
		{"PDF", app.ExportScheduleReportPDF, ".pdf"},
		{"CSV", app.ExportScheduleReportCSV, ".csv"},
		{"HTML", app.ExportScheduleReportHTML, ".html"},
		{"MSPDI", app.ExportScheduleReportMSPDI, ".xml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := tc.call()
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if filepath.Ext(path) != tc.ext {
				t.Fatalf("%s: want extension %q, got path %q", tc.name, tc.ext, path)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("%s: exported file not found: %v", tc.name, err)
			}
			if info.Size() == 0 {
				t.Fatalf("%s: exported file is empty", tc.name)
			}
		})
	}
}

// TestExportScheduleReportPrintsEarnedValueInTheProjectCurrency: the
// schedule report's earned value reads in the open project's reporting
// currency, so a yen project prints whole yen with thousands separators.
// The schedule export used to pass no currency at all.
func TestExportScheduleReportPrintsEarnedValueInTheProjectCurrency(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Yen Line")
	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatal(err)
	}
	// Earned value needs a start date in the past to have a status day.
	meta.CurrencyCode, meta.StartDate, meta.CountryCode = "JPY", "2026-06-01", "US"
	if _, err := app.UpdateProjectMeta(meta); err != nil {
		t.Fatalf("switch the empty project to JPY: %v", err)
	}
	if _, err := app.SaveChart(db.Chart{Kind: "cpm", Title: "Schedule", Data: `{"nodes":[
		{"id":"A","label":"Design","duration":4,"budgeted_cost":642250,"actual_cost":700000,"percent_complete":100},
		{"id":"B","label":"Build","duration":4,"budgeted_cost":642250}
	],"edges":[{"from":"A","to":"B"}]}`}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	path, err := app.ExportScheduleReportHTML()
	if err != nil {
		t.Fatalf("ExportScheduleReportHTML: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the export: %v", err)
	}
	html := string(raw)
	for _, want := range []string{"Budget at completion (BAC): 1,284,500<", "Actual cost (AC): 700,000<"} {
		if !strings.Contains(html, want) {
			t.Errorf("schedule report is missing %q", strings.TrimSuffix(want, "<"))
		}
	}
}

// TestExportDocumentAllFormats drives ExportDocumentDOCX/ODT — the two
// callers of the shared, previously wholly-uncovered exportDocumentAs —
// through a real document render.
func TestExportDocumentAllFormats(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Proj")

	doc, err := app.NewDocument("brief", "")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	cases := []struct {
		name string
		call func(string) (string, error)
		ext  string
	}{
		{"DOCX", app.ExportDocumentDOCX, ".docx"},
		{"ODT", app.ExportDocumentODT, ".odt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := tc.call(doc.ID)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if filepath.Ext(path) != tc.ext {
				t.Fatalf("%s: want extension %q, got path %q", tc.name, tc.ext, path)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("%s: exported file not found: %v", tc.name, err)
			}
			if info.Size() == 0 {
				t.Fatalf("%s: exported file is empty", tc.name)
			}
			if !strings.HasPrefix(filepath.Base(path), doc.Title+"-") {
				t.Fatalf("%s: want output named from the document title %q, got %q", tc.name, doc.Title, filepath.Base(path))
			}
		})
	}

	if _, err := app.ExportDocumentDOCX("does-not-exist"); err == nil {
		t.Fatal("ExportDocumentDOCX: want error for unknown document id, got nil")
	}
}
