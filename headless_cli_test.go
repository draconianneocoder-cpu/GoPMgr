// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"pmforge/internal/cli"
	"pmforge/internal/crypto"
	"pmforge/internal/db"
	"pmforge/internal/export"
)

// seedHeadlessProject creates a plaintext project DB with one project row
// and returns an open handle.
func seedHeadlessProject(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "project.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(db.Project{Name: "Headless Demo", Status: "planning", Phase: "planning"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestParseHeadlessFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    export.ExportFormat
		wantErr bool
	}{
		{"", export.FormatPDF, false},
		{"pdf", export.FormatPDF, false},
		{"DOCX", export.FormatDOCX, false},
		{"odt", export.FormatODT, false},
		{"xlsx", export.FormatXLSX, false},
		{"csv", export.FormatCSV, false},
		{"html", export.FormatHTML, false},
		{"mspdi", export.FormatMSPDI, false},
		{"xml", export.FormatMSPDI, false},
		{"docx ", export.FormatDOCX, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := parseHeadlessFormat(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseHeadlessFormat(%q) = %q, want error", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("parseHeadlessFormat(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestRunHeadlessExportWritesFile(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.csv")
	cfg := &cli.Config{ExportPath: out, ExportFormat: "csv"}
	if err := runHeadlessExport(cfg, d); err != nil {
		t.Fatalf("runHeadlessExport: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("export file is empty")
	}
}

func TestRunHeadlessExportEncrypts(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.csv.enc")
	const pw = "correct horse battery staple"
	t.Setenv("PMF_TEST_PW", pw)
	cfg := &cli.Config{
		ExportPath:    out,
		ExportFormat:  "csv",
		EncryptExport: true,
		PasswordEnv:   "PMF_TEST_PW",
	}
	if err := runHeadlessExport(cfg, d); err != nil {
		t.Fatalf("runHeadlessExport encrypted: %v", err)
	}
	blob, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read encrypted export: %v", err)
	}
	plain, err := crypto.DecryptBuffer(blob, pw)
	if err != nil {
		t.Fatalf("decrypt export: %v", err)
	}
	if len(plain) == 0 {
		t.Fatal("decrypted export is empty")
	}
}

func TestRunHeadlessExportEncryptRequiresPassword(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.csv.enc")
	cfg := &cli.Config{ExportPath: out, ExportFormat: "csv", EncryptExport: true}
	if err := runHeadlessExport(cfg, d); err == nil {
		t.Fatal("runHeadlessExport with --encrypt but no --password-env should error")
	}
}

func TestPrintHeadlessStatsSucceeds(t *testing.T) {
	d := seedHeadlessProject(t)
	if err := printHeadlessStats(d); err != nil {
		t.Fatalf("printHeadlessStats: %v", err)
	}
}
