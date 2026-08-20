// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"strings"
	"testing"
)

func TestGetDocument_FetchesByID(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	saved, err := d.SaveDocument(Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Original charter",
		Content:   `{"summary":"x"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	got, err := d.GetDocument(saved.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.ID != saved.ID || got.Title != "Original charter" || got.Kind != "charter" {
		t.Errorf("GetDocument = %+v, want ID=%q Title=%q Kind=%q", got, saved.ID, "Original charter", "charter")
	}
}

func TestGetDocument_ReturnsErrNoDocumentForMissingID(t *testing.T) {
	d := newBackupTestDB(t)
	if _, err := d.GetDocument("does-not-exist"); err != ErrNoDocument {
		t.Errorf("GetDocument(missing) error = %v, want ErrNoDocument", err)
	}
}

func TestListDocuments_FiltersByKind(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "C1"}); err != nil {
		t.Fatalf("SaveDocument charter: %v", err)
	}
	if _, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "risk_register", Title: "R1"}); err != nil {
		t.Fatalf("SaveDocument risk_register: %v", err)
	}
	if _, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "risk_register", Title: "R2"}); err != nil {
		t.Fatalf("SaveDocument risk_register 2: %v", err)
	}

	all, err := d.ListDocuments(project.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments(all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListDocuments(kind=\"\") returned %d documents, want 3", len(all))
	}

	risks, err := d.ListDocuments(project.ID, "risk_register")
	if err != nil {
		t.Fatalf("ListDocuments(risk_register): %v", err)
	}
	if len(risks) != 2 {
		t.Fatalf("ListDocuments(risk_register) returned %d documents, want 2", len(risks))
	}
	for _, doc := range risks {
		if doc.Kind != "risk_register" {
			t.Errorf("ListDocuments(risk_register) returned a %q document", doc.Kind)
		}
	}
}

// setDocumentUpdatedAt directly overwrites a document's updated_at column,
// bypassing SaveDocument's time.Now()-derived value. Deterministic control
// over timestamps is necessary here: two SaveDocument calls in a tight test
// loop usually get distinct RFC3339Nano values from nanosecond-resolution
// wall-clock reads, but that isn't guaranteed, and a tie makes SQLite's
// ORDER BY between the tied rows unspecified -- the same "don't rely on
// wall-clock jitter" discipline used elsewhere in this repo's test suite.
func setDocumentUpdatedAt(t *testing.T, d *Database, id, updatedAt string) {
	t.Helper()
	if _, err := d.Conn.Exec(`UPDATE documents SET updated_at = ? WHERE id = ?`, updatedAt, id); err != nil {
		t.Fatalf("set updated_at for %s: %v", id, err)
	}
}

func TestListDocuments_OrdersByMostRecentlyUpdatedDescending(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	oldest, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "Oldest"})
	if err != nil {
		t.Fatalf("SaveDocument oldest: %v", err)
	}
	middle, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "Middle"})
	if err != nil {
		t.Fatalf("SaveDocument middle: %v", err)
	}
	newest, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "Newest"})
	if err != nil {
		t.Fatalf("SaveDocument newest: %v", err)
	}
	// Clearly-ordered fractional timestamps, all within the same second,
	// deliberately avoiding the whole-second-vs-fractional edge case pinned
	// separately below.
	setDocumentUpdatedAt(t, d, oldest.ID, "2026-01-01T10:00:00.1Z")
	setDocumentUpdatedAt(t, d, middle.ID, "2026-01-01T10:00:00.5Z")
	setDocumentUpdatedAt(t, d, newest.ID, "2026-01-01T10:00:00.9Z")

	got, err := d.ListDocuments(project.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListDocuments returned %d documents, want 3", len(got))
	}
	wantOrder := []string{"Newest", "Middle", "Oldest"}
	for i, want := range wantOrder {
		if got[i].Title != want {
			t.Errorf("ListDocuments order[%d] = %q, want %q (full order: %v)", i, got[i].Title, want,
				[]string{got[0].Title, got[1].Title, got[2].Title})
		}
	}
}

// TestListDocuments_WholeSecondTimestampDoesNotPanicOrDropRows.
//
// Historical note: ListDocuments originally ordered by the RFC3339Nano
// TEXT `updated_at` column, which has a real string-ordering hazard (a
// whole-second save formats without a fractional component and, being
// lexicographically smaller, can sort after a later same-second
// fractional save). That hazard was closed by internal/db/timestamps.go's
// `updated_at_unixnano` retrofit -- ListDocuments now orders by that
// INTEGER column instead, which has no such ambiguity. charts_test.go has
// the equivalent test and note for ListCharts, which got the same fix.
//
// This test's setDocumentUpdatedAt helper only overwrites the TEXT
// `updated_at` column, not `updated_at_unixnano`, so it can no longer
// reproduce the original hazard against the live query. It still pins the
// one thing it asserts: both rows must come back present, in either order,
// with no panic and no row silently dropped.
func TestListDocuments_WholeSecondTimestampDoesNotPanicOrDropRows(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	wholeSecond, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "ExactSecond"})
	if err != nil {
		t.Fatalf("SaveDocument wholeSecond: %v", err)
	}
	halfSecondLater, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "HalfSecondLater"})
	if err != nil {
		t.Fatalf("SaveDocument halfSecondLater: %v", err)
	}
	setDocumentUpdatedAt(t, d, wholeSecond.ID, "2026-01-01T10:00:00Z")
	setDocumentUpdatedAt(t, d, halfSecondLater.ID, "2026-01-01T10:00:00.5Z")

	got, err := d.ListDocuments(project.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListDocuments returned %d documents, want 2 (both rows must survive regardless of ordering)", len(got))
	}
	gotTitles := map[string]bool{got[0].Title: true, got[1].Title: true}
	if !gotTitles["ExactSecond"] || !gotTitles["HalfSecondLater"] {
		t.Errorf("ListDocuments returned %v, want both ExactSecond and HalfSecondLater present (in either order)", []string{got[0].Title, got[1].Title})
	}
}

func TestListDocuments_WhereClauseFiltersOnProjectID(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Real Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "Mine"}); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	// A second project_id value, backed by a row inserted directly
	// (not a second UpsertProject call, since this application's data
	// model is one-project-per-file -- see project.go's GetProject doc
	// comment -- and a second project row created through the normal
	// path would misrepresent a supported configuration). The `project`
	// row itself is required by documents' FOREIGN KEY constraint; this
	// remains purely a SQL-predicate-correctness check on
	// `WHERE project_id = ?`, not a claim that multi-project files are
	// supported.
	const otherProjectID = "other-project-id"
	if _, err := d.Conn.Exec(
		`INSERT INTO project (id, name, created_at, updated_at) VALUES (?, 'Other', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		otherProjectID,
	); err != nil {
		t.Fatalf("seed other project row: %v", err)
	}
	if _, err := d.Conn.Exec(
		`INSERT INTO documents (id, project_id, kind, title, content, template_id, version, status, created_at, updated_at)
		 VALUES ('doc-other-project', ?, 'charter', 'Not Mine', '{}', '', 1, 'draft', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		otherProjectID,
	); err != nil {
		t.Fatalf("seed other-project document: %v", err)
	}

	got, err := d.ListDocuments(project.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Mine" {
		t.Errorf("ListDocuments(project.ID) = %v, want exactly [Mine] -- WHERE project_id clause did not filter out the other project's document", got)
	}

	// ListDocuments carries the project_id predicate in two separate
	// query strings -- one for the kind-empty branch (exercised above)
	// and a distinct one for the kind-filtered branch. Both "Mine" and
	// "Not Mine" are kind="charter", so this call exercises the second
	// copy of the predicate specifically.
	gotKind, err := d.ListDocuments(project.ID, "charter")
	if err != nil {
		t.Fatalf("ListDocuments(kind=charter): %v", err)
	}
	if len(gotKind) != 1 || gotKind[0].Title != "Mine" {
		t.Errorf("ListDocuments(project.ID, \"charter\") = %v, want exactly [Mine] -- kind-filtered branch's WHERE project_id clause did not filter out the other project's document", gotKind)
	}
}

func TestSaveDocument_VersionSequenceAcrossThreeSaves(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "v1"})
	if err != nil {
		t.Fatalf("SaveDocument create: %v", err)
	}
	if doc.Version != 1 {
		t.Fatalf("Version after create = %d, want 1", doc.Version)
	}

	doc.Title = "v2"
	doc, err = d.SaveDocument(doc)
	if err != nil {
		t.Fatalf("SaveDocument update 1: %v", err)
	}
	if doc.Version != 2 {
		t.Fatalf("Version after 1st update = %d, want 2", doc.Version)
	}

	doc.Title = "v3"
	doc, err = d.SaveDocument(doc)
	if err != nil {
		t.Fatalf("SaveDocument update 2: %v", err)
	}
	if doc.Version != 3 {
		t.Fatalf("Version after 2nd update = %d, want 3", doc.Version)
	}

	fetched, err := d.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if fetched.Version != 3 {
		t.Errorf("GetDocument version = %d, want 3 (returned struct and stored row must agree)", fetched.Version)
	}
}

// TestDeleteDocument_ActuallyRemovesTheRow is the direct behavioral
// contract for an irreversible operation: DeleteDocument's only existing
// exercise (audit_events_test.go's
// TestSaveDocumentAppendsCreateUpdateAndDeleteAuditEvents) checks the
// resulting audit-event type, never that the document itself is gone.
func TestDeleteDocument_ActuallyRemovesTheRow(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "Doomed"})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	if err := d.DeleteDocument(doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	if _, err := d.GetDocument(doc.ID); err != ErrNoDocument {
		t.Errorf("GetDocument after delete: err = %v, want ErrNoDocument", err)
	}
	all, err := d.ListDocuments(project.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("ListDocuments after delete = %v, want empty", all)
	}
}

// TestDeleteDocument_MissingIDIsANoOpNotAnError pins the early-return
// branch (documents.go: `if err == ErrNoDocument { err = nil; return
// tx.Commit() }`), which is otherwise completely unexercised. This
// matters specifically because DeleteDocument is irreversible: a caller
// that retries a delete (e.g. after a timeout on the first attempt's
// response, unsure whether it landed) must not receive an error on the
// harmless second call. Beyond "no error," this asserts the concrete
// prohibited effect: no document.delete audit event may be appended for
// an ID that never existed -- a phantom delete event would corrupt the
// hash chain's meaning as forensic evidence (it would record deleting
// something that was never there).
func TestDeleteDocument_MissingIDIsANoOpNotAnError(t *testing.T) {
	d := newBackupTestDB(t)
	before := countAuditEvents(t, d)

	if err := d.DeleteDocument("does-not-exist"); err != nil {
		t.Errorf("DeleteDocument(missing) = %v, want nil (no-op)", err)
	}

	after := countAuditEvents(t, d)
	if after != before {
		t.Errorf("audit_events count went from %d to %d after a no-op delete, want unchanged (no phantom document.delete event)", before, after)
	}
}

func countAuditEvents(t *testing.T, d *Database) int {
	t.Helper()
	var n int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&n); err != nil {
		t.Fatalf("count audit_events: %v", err)
	}
	return n
}

// TestDeleteDocument_AuditEventCapturesDeletedContent proves the
// forensic value of the delete's audit trail: for an irreversible
// operation, the audit event's BeforeJSON must reflect the document's
// actual content at time of deletion, not merely record that *a* delete
// happened.
func TestDeleteDocument_AuditEventCapturesDeletedContent(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Docs"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Forensic Target",
		Content:   `{"summary":"must survive in the audit trail"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	if err := d.DeleteDocument(doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	events, err := d.ListAuditEvents(project.ID)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	var deleteEvent *AuditEvent
	for i := range events {
		if events[i].EventType == "document.delete" {
			deleteEvent = &events[i]
		}
	}
	if deleteEvent == nil {
		t.Fatalf("no document.delete event found among %d events", len(events))
	}
	if !stringContainsAll(deleteEvent.BeforeCanonicalJSON, "Forensic Target", "must survive in the audit trail") {
		t.Errorf("document.delete BeforeCanonicalJSON = %q, want it to contain the deleted document's title and content", deleteEvent.BeforeCanonicalJSON)
	}
	if deleteEvent.AfterCanonicalJSON != "null" {
		t.Errorf("document.delete AfterCanonicalJSON = %q, want %q (nothing exists after a delete)", deleteEvent.AfterCanonicalJSON, "null")
	}
}

func stringContainsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
