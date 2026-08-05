// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package admin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/db"
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
		return
	}
	if s.DB != d {
		t.Error("NewService did not store the provided Database")
	}
}

// The absence-check in TestSecureArchiveRemovesArchiveWhenCreatedAuditLogFails
// below globs for "GoPMgr_Archive_*.pmba" but only ever asserts zero
// matches, on a path where SecureArchive is expected to fail and clean up
// after itself. It never exercises the success path, so nothing today
// positively pins the "GoPMgr_Archive_" prefix the way root_dir_test.go,
// project_path_confinement_test.go, and backup_test.go pin the other
// persistence-boundary literals. This test closes that gap by asserting the
// literal against a real archive SecureArchive wrote.
//
// Renamed 2026-08-04 from "PMForge_Archive_" to "GoPMgr_Archive_" alongside
// workflow.go's SecureArchive: this prefix is generated fresh on every call
// and nothing reads it back, so — unlike the other frozen literals — it was
// a pure rename with no dual-read compatibility to preserve.
func TestSecureArchiveUsesGoPMgrArchivePrefix(t *testing.T) {
	d := newAdminTestDB(t)
	workDir := t.TempDir()
	t.Chdir(workDir)

	s := NewService(d)
	backupPath, err := s.SecureArchive(d.Path)
	if err != nil {
		t.Fatalf("SecureArchive: %v", err)
	}

	if got := filepath.Base(backupPath); !strings.HasPrefix(got, "GoPMgr_Archive_") {
		t.Fatalf("archive filename = %q, want prefix %q", got, "GoPMgr_Archive_")
	}
}

// TestSecureArchive_PropagatesSettingsLoadError forces GetSettings to
// fail by closing the DB first. db.Database.Close is safe to call
// twice (confirmed directly: sql.DB.Close is idempotent), so this
// doesn't conflict with newAdminTestDB's own t.Cleanup(d.Close).
func TestSecureArchive_PropagatesSettingsLoadError(t *testing.T) {
	d := newAdminTestDB(t)
	if err := d.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	s := NewService(d)
	if _, err := s.SecureArchive(d.Path); err == nil || !strings.Contains(err.Error(), "ARCHIVE_SETTINGS_LOAD_FAILED") {
		t.Fatalf("SecureArchive() error = %v, want ARCHIVE_SETTINGS_LOAD_FAILED", err)
	}
}

// TestSecureArchive_PropagatesCertBundlingError covers two branches in
// one test: settings.CertPath != "" (a saved cert path is included in
// the archive) and CreateArchivalBundle's own error path, forced the
// same way internal/db/backup_test.go's
// TestCreateArchivalBundleDoesNotPublishPartialArchiveOnBundleFailure
// does -- point CertPath at a directory, which os.ReadFile (used to
// read the cert bytes) rejects.
func TestSecureArchive_PropagatesCertBundlingError(t *testing.T) {
	d := newAdminTestDB(t)
	workDir := t.TempDir()
	t.Chdir(workDir)

	certDir := filepath.Join(workDir, "not-a-cert-file")
	if err := os.Mkdir(certDir, 0o700); err != nil {
		t.Fatalf("mkdir cert path: %v", err)
	}
	if err := d.SaveSettings(db.UserSettings{CertPath: certDir}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	s := NewService(d)
	if _, err := s.SecureArchive(d.Path); err == nil || !strings.Contains(err.Error(), "security backup failed") {
		t.Fatalf("SecureArchive() error = %v, want cert bundling failure", err)
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

	matches, err := filepath.Glob(filepath.Join(workDir, "GoPMgr_Archive_*.pmba"))
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

// TestLogDocumentSignatureOutcome_DefaultsEmptySignatureStatusAndDetails
// covers the two zero-value defaulting branches: an empty
// signatureStatus becomes "unsigned" and empty details becomes the
// stock "Document signature outcome recorded." message.
//
// This asserts against audit_log.details (LogAction's plain-text
// audit trail), not audit_events.signature_status: AppendAuditEvent
// (internal/db/audit.go's appendAuditEventTx) applies its own
// SignatureStatus=="" -> "unsigned" default downstream, which would
// silently absorb a deleted default in this package and let a broken
// mutation pass. audit_log.details is written directly from this
// package's local signatureStatus/details variables with no
// downstream re-defaulting, so it is the only observable proof this
// package's own defaulting ran. (Caught by break-verification: an
// earlier draft of this test asserted against audit_events and did
// not go red when the defaulting branch was deleted.)
func TestLogDocumentSignatureOutcome_DefaultsEmptySignatureStatusAndDetails(t *testing.T) {
	d := newAdminTestDB(t)
	project, err := d.UpsertProject(db.Project{Name: "Default Signature Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Unsigned Charter",
		Content:   `{"summary":"pending"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	NewService(d).LogDocumentSignatureOutcome(doc.ID, "", "", "")

	var details string
	if err := d.Conn.QueryRow(
		`SELECT details FROM audit_log
		 WHERE action = 'SIGNATURE_EVENT' AND target_id = ?
		 ORDER BY id DESC LIMIT 1`,
		doc.ID,
	).Scan(&details); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if want := "[unsigned] Document signature outcome recorded."; details != want {
		t.Fatalf("audit_log.details = %q, want %q", details, want)
	}
}

// TestLogSignatureCheckpoint_AppendAuditEventFailureLeavesNoRow forces
// AppendAuditEvent to fail via a SQLite trigger (matching the
// audit_log trigger technique used above for SecureArchive), driving
// execution into the true branch of logSignatureCheckpointWithStatus's
// `if err != nil { debug.Wrap(...) }` guard -- the coverage this test
// closes. The row-count-0 assertion confirms AppendAuditEvent was
// genuinely reached and failed (not reached-and-succeeded), by
// checking a value the trigger controls directly rather than relying
// on "no panic".
//
// It does NOT distinguish the guard's presence from its absence: the
// guard is log-only with no propagation to the caller, so deleting it
// entirely (`_, _ = s.DB.AppendAuditEvent(...)`) would still hit the
// same blocked insert and leave the same zero rows -- structurally
// identical to applog's TestPruneOldLogs_UnreadableDirIsANoOp
// disclosure (see DEVELOPER_HANDBOOK.md): the branch's own contract
// admits no assertion that can tell "the guard ran" from "the guard
// is gone".
func TestLogSignatureCheckpoint_AppendAuditEventFailureLeavesNoRow(t *testing.T) {
	d := newAdminTestDB(t)
	project, err := d.UpsertProject(db.Project{Name: "Blocked Signature Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Blocked Charter",
		Content:   `{"summary":"blocked"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	_, err = d.Conn.Exec(`
		CREATE TRIGGER block_signature_audit_event
		BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'document.signature'
		BEGIN
			SELECT RAISE(ABORT, 'signature audit event unavailable');
		END;
	`)
	if err != nil {
		t.Fatalf("create audit_events trigger: %v", err)
	}

	NewService(d).LogSignatureEvent(doc.ID, true, nil)

	var count int
	if err := d.Conn.QueryRow(
		`SELECT COUNT(*) FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.signature'`,
		project.ID,
		doc.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count signature audit events: %v", err)
	}
	if count != 0 {
		t.Fatalf("signature audit event count = %d, want 0 (blocked insert should leave no row)", count)
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
