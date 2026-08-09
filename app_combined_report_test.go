// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopmgr/internal/documents"
)

func TestExportCombinedReportWithOptionsUsesCurrentUserExports(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Combined Report Plan")
	document, err := app.NewDocument("charter_word", "Project Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	path, err := app.ExportCombinedReportWithOptions(
		"Governance Pack",
		"August",
		[]documents.ReportSection{{DocumentID: document.ID}},
		CombinedReportOptions{ProfileID: "custom"},
	)
	if err != nil {
		t.Fatalf("ExportCombinedReportWithOptions: %v", err)
	}
	user := app.CurrentUser()
	if user == nil {
		t.Fatal("expected signed-in user")
	}
	if wantDir := filepath.Join(user.DataDir, "exports"); filepath.Dir(path) != wantDir {
		t.Fatalf("report directory = %q, want %q", filepath.Dir(path), wantDir)
	}
	if _, err := os.Stat(path + ".manifest.json"); err != nil {
		t.Fatalf("stat provenance manifest: %v", err)
	}
}
