// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
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

// TestListDocuments_WholeSecondTimestampSortsWrongRelativeToFractional
// pins a real, verified (not assumed) latent defect in ListDocuments'
// `ORDER BY updated_at DESC`: RFC3339Nano trims trailing zeros from the
// fractional-seconds component, and OMITS the fractional component
// entirely when it's exactly zero (formats as "...T10:00:00Z", not
// "...T10:00:00.000000000Z"). Comparing that omitted-fraction string
// against a same-second fractional string is a plain byte/lexicographic
// comparison (both in Go's string "<" and in SQLite's default TEXT
// ordering) -- and '.' (0x2E) sorts BEFORE 'Z' (0x5A) in ASCII, so
// "10:00:00.5Z" < "10:00:00Z" lexicographically, even though
// 10:00:00.000 is chronologically EARLIER than 10:00:00.5.
//
// Confirmed directly with Go's time.Format before writing this test,
// not assumed from reading the RFC3339Nano format documentation. In
// practice this requires a save landing at time.Now() with exactly
// zero nanoseconds -- vanishingly unlikely under real wall-clock
// jitter -- so it is not expected to occur in normal use, and is not
// fixed here (out of this increment's authorized scope: it would mean
// changing the stored timestamp format or the query, a production
// change, not a test). Pinned so the behavior is deliberate knowledge,
// not a surprise the next person to touch this ordering rediscovers
// from a bug report.
func TestListDocuments_WholeSecondTimestampSortsWrongRelativeToFractional(t *testing.T) {
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
	// wholeSecond is chronologically EARLIER (exactly on the second);
	// halfSecondLater is 500ms later in the same second.
	setDocumentUpdatedAt(t, d, wholeSecond.ID, "2026-01-01T10:00:00Z")
	setDocumentUpdatedAt(t, d, halfSecondLater.ID, "2026-01-01T10:00:00.5Z")

	got, err := d.ListDocuments(project.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListDocuments returned %d documents, want 2", len(got))
	}
	// Chronologically-correct DESC order would be [HalfSecondLater,
	// ExactSecond]. The lexicographic ORDER BY this function actually
	// uses produces the reverse -- this assertion documents the
	// observed (wrong, but real) behavior, not the desired one.
	if got[0].Title != "ExactSecond" || got[1].Title != "HalfSecondLater" {
		t.Errorf("ListDocuments order = [%q, %q]; expected the documented lexicographic-ordering defect to produce [ExactSecond, HalfSecondLater] "+
			"(chronologically-correct order would be the reverse) -- if this now returns the chronologically-correct order, "+
			"the defect this test pins has been fixed and this test should be deleted, not adjusted",
			got[0].Title, got[1].Title)
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
