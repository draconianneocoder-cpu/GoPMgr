// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"testing"

	"gopmgr/internal/charts"
	"gopmgr/internal/db"
	"gopmgr/internal/documents"
)

func TestDocumentAppMethodsRejectForeignProjectRowsInSameFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")

	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}

	seedForeignDocument := func(t *testing.T, suffix string) db.Document {
		t.Helper()
		document, err := d.SaveDocument(db.Document{
			ID:        "doc-foreign-" + suffix,
			ProjectID: "prj-foreign",
			Kind:      string(documents.KindProjectCharterWord),
			Title:     "Foreign Document",
			Content:   `{}`,
			Status:    "draft",
		})
		if err != nil {
			t.Fatalf("seed foreign document: %v", err)
		}
		return document
	}

	cases := []struct {
		name string
		call func(db.Document) error
	}{
		{"GetDocument", func(doc db.Document) error { _, err := app.GetDocument(doc.ID); return err }},
		{"SaveDocument", func(doc db.Document) error { _, err := app.SaveDocument(doc); return err }},
		{"SyncRiskRegisterToMatrix", func(doc db.Document) error { _, err := app.SyncRiskRegisterToMatrix(doc.ID); return err }},
		{"exportDocumentAs", func(doc db.Document) error {
			_, err := app.exportDocumentAs(doc.ID, ".txt", func(documents.Kind, string, string) ([]byte, error) {
				return []byte("unexpected"), nil
			})
			return err
		}},
		{"ExportDocumentPDF", func(doc db.Document) error { _, err := app.ExportDocumentPDF(doc.ID); return err }},
		{"ExportDocumentPDFGnuPG", func(doc db.Document) error { _, err := app.ExportDocumentPDFGnuPG(doc.ID, "key"); return err }},
		{"exportDocumentPDFSignedWithRuntime", func(doc db.Document) error {
			_, err := app.exportDocumentPDFSignedWithRuntime(doc.ID, "", "", padesExportRuntime{})
			return err
		}},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			foreign := seedForeignDocument(t, fmt.Sprintf("%02d", i))
			if err := tc.call(foreign); !errors.Is(err, db.ErrNoDocument) {
				t.Fatalf("%s(foreign document) error = %v, want db.ErrNoDocument", tc.name, err)
			}
			if _, err := d.GetDocument(foreign.ID); err != nil {
				t.Fatalf("%s mutated or deleted foreign document: %v", tc.name, err)
			}
		})
	}

	foreign := seedForeignDocument(t, "delete")
	if err := app.DeleteDocument(foreign.ID); err != nil {
		t.Fatalf("DeleteDocument(foreign document) error = %v, want nil no-op", err)
	}
	if _, err := d.GetDocument(foreign.ID); err != nil {
		t.Fatalf("DeleteDocument mutated or deleted foreign document: %v", err)
	}
	if err := app.DeleteDocument("doc-missing"); err != nil {
		t.Fatalf("DeleteDocument(missing document) error = %v, want nil no-op", err)
	}
}

func TestSaveDocumentBindsNewDocumentToOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")
	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	openProject, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}

	saved, err := app.SaveDocument(db.Document{
		ProjectID: "prj-foreign",
		Kind:      string(documents.KindProjectCharterWord),
		Content:   `{}`,
		Status:    "draft",
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	if saved.ProjectID != openProject.ID {
		t.Fatalf("SaveDocument ProjectID = %q, want open project %q", saved.ProjectID, openProject.ID)
	}
}

func TestSyncRiskRegisterRejectsForeignLinkedChart(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")
	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	openProject, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}
	foreignChart, err := d.SaveChart(db.Chart{ProjectID: "prj-foreign", Kind: string(charts.KindRiskMatrix), Data: `{"items":[]}`, Config: `{}`})
	if err != nil {
		t.Fatalf("seed foreign chart: %v", err)
	}
	document, err := d.SaveDocument(db.Document{
		ProjectID: openProject.ID,
		Kind:      string(documents.KindRiskRegister),
		Content:   fmt.Sprintf(`{"risk_matrix_ref":%q,"risks":[]}`, foreignChart.ID),
		Status:    "draft",
	})
	if err != nil {
		t.Fatalf("seed risk register: %v", err)
	}

	if _, err := app.SyncRiskRegisterToMatrix(document.ID); !errors.Is(err, db.ErrNoChart) {
		t.Fatalf("SyncRiskRegisterToMatrix error = %v, want db.ErrNoChart", err)
	}
}
