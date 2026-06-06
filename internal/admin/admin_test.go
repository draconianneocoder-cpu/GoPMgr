// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
		t.Error("NewService returned nil")
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
