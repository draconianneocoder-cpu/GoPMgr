// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/documents"
)

// DOCX and ODT exports render with the open project's reporting currency.
func TestExportDocumentAs_RendersInTheProjectCurrency(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Euro Project")

	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	project, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	project.CurrencyCode = "EUR"
	if _, err := d.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject(EUR): %v", err)
	}
	doc, err := d.SaveDocument(db.Document{
		ID:        "doc-charter",
		ProjectID: project.ID,
		Kind:      string(documents.KindProjectCharterWord),
		Title:     "Charter",
		Content:   `{"high_level_budget":1000}`,
		Status:    "draft",
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	errStop := errors.New("stop before the save dialog")
	var got string
	_, err = app.exportDocumentAs(doc.ID, ".docx", func(_ documents.Kind, _, _, currency string) ([]byte, error) {
		got = currency
		return nil, errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatalf("exportDocumentAs error = %v, want the renderer's", err)
	}
	if got != "EUR" {
		t.Errorf("renderer currency = %q, want the project's EUR", got)
	}
}
