// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package admin

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/db"
)

func newAdminTestDB(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "admin.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	return d
}

func TestNewService_ReturnsNonNil(t *testing.T) {
	d := newAdminTestDB(t)
	s := NewService(d)
	if s == nil {
		t.Fatal("NewService returned nil")
	}
	if s.DB != d {
		t.Error("NewService did not store the provided Database")
	}
}

func TestSecureArchiveRemovesArchiveWhenCreatedAuditLogFails(t *testing.T) {
	d := newAdminTestDB(t)
	workDir := t.TempDir()
	t.Chdir(workDir)

	_, err := d.Conn.Exec(`
		CREATE TRIGGER block_archive_created_audit
		BEFORE INSERT ON audit_log
		WHEN NEW.action = 'ARCHIVE_CREATED'
		BEGIN
			SELECT RAISE(ABORT, 'archive audit unavailable');
		END;
	`)
	if err != nil {
		t.Fatalf("create audit trigger: %v", err)
	}

	s := NewService(d)
	if _, err := s.SecureArchive(d.Path); err == nil || !strings.Contains(err.Error(), "AUDIT_LOG_WRITE_FAILED") {
		t.Fatalf("SecureArchive error = %v, want audit write failure", err)
	}

	matches, err := filepath.Glob(filepath.Join(workDir, "PMForge_Archive_*.pmba"))
	if err != nil {
		t.Fatalf("glob archive output: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("unaudited archive was left behind: %v", matches)
	}
}

func TestLogSignatureEvent_Success_NoPanic(t *testing.T) {
	d := newAdminTestDB(t)
	s := NewService(d)
	// success=true: must not panic and must write an audit row
	s.LogSignatureEvent("doc-abc", true, nil)
}

func TestLogSignatureEvent_Failure_NoPanic(t *testing.T) {
	d := newAdminTestDB(t)
	s := NewService(d)
	// success=false with a real error: must not panic
	s.LogSignatureEvent("doc-abc", false, errors.New("signing key not found"))
}

func TestLogSignatureEvent_NilError_WithSuccessFalse_NoPanic(t *testing.T) {
	d := newAdminTestDB(t)
	s := NewService(d)
	// success=false, err=nil edge case (avoids a format-string nil panic)
	s.LogSignatureEvent("doc-xyz", false, nil)
}

func TestLogSignatureEvent_WritesTamperEvidentCheckpoint(t *testing.T) {
	d := newAdminTestDB(t)
	project, err := d.UpsertProject(db.Project{Name: "Signature Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Signed Charter",
		Content:   `{"summary":"ready"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	NewService(d).LogSignatureEvent(doc.ID, true, nil)

	var eventType, entityType, entityID, signatureStatus string
	if err := d.Conn.QueryRow(
		`SELECT event_type, entity_type, entity_id, signature_status
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.signature'
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		project.ID,
		doc.ID,
	).Scan(&eventType, &entityType, &entityID, &signatureStatus); err != nil {
		t.Fatalf("query signature audit event: %v", err)
	}
	if eventType != "document.signature" || entityType != "document" || entityID != doc.ID || signatureStatus != "signed" {
		t.Fatalf("signature audit event = type:%q entity:%q id:%q status:%q",
			eventType, entityType, entityID, signatureStatus)
	}
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid {
		t.Fatalf("verification = %+v, want valid", report)
	}
}

func TestLogSignatureEvent_FailureCheckpointUsesFailedStatus(t *testing.T) {
	d := newAdminTestDB(t)
	project, err := d.UpsertProject(db.Project{Name: "Signature Failure Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Unsigned Charter",
		Content:   `{"summary":"not ready"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	NewService(d).LogSignatureEvent(doc.ID, false, errors.New("certificate rejected"))

	var signatureStatus string
	if err := d.Conn.QueryRow(
		`SELECT signature_status
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.signature'
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		project.ID,
		doc.ID,
	).Scan(&signatureStatus); err != nil {
		t.Fatalf("query signature audit event: %v", err)
	}
	if signatureStatus != "failed" {
		t.Fatalf("signature_status = %q, want failed", signatureStatus)
	}
}

func TestLogDocumentSignatureOutcomeRecordsGnuPGStatus(t *testing.T) {
	d := newAdminTestDB(t)
	project, err := d.UpsertProject(db.Project{Name: "GnuPG Signature Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Detached Signature Charter",
		Content:   `{"summary":"ready"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	NewService(d).LogDocumentSignatureOutcome(doc.ID, "gpg_signed", "Detached GnuPG signature written.", "signature.asc")

	var signatureStatus, signatureBlob string
	if err := d.Conn.QueryRow(
		`SELECT signature_status, signature_blob_optional
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.signature'
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		project.ID,
		doc.ID,
	).Scan(&signatureStatus, &signatureBlob); err != nil {
		t.Fatalf("query signature audit event: %v", err)
	}
	if signatureStatus != "gpg_signed" {
		t.Fatalf("signature_status = %q, want gpg_signed", signatureStatus)
	}
	if signatureBlob != "signature.asc" {
		t.Fatalf("signature_blob_optional = %q, want signature.asc", signatureBlob)
	}
}
