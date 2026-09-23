// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package deletionlog

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testDEK(fill byte) []byte {
	return bytes.Repeat([]byte{fill}, 32)
}

func openTestLog(t *testing.T, path string, dek []byte) *Log {
	t.Helper()
	l, err := Open(path, dek)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestDeletionLogRecordsRequestAndOutcomeAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deletions.gopmgr")
	l := openTestLog(t, path, testDEK(0x42))
	req := Request{Actor: "alice", ProjectID: "project_1", ProjectName: "Bridge", Location: "20260922-150405-Bridge/project.gopmgr", AuditEvents: 7, AuditValid: true, AuditTerminalHash: "abc123"}
	first, err := l.Requested(req)
	if err != nil {
		t.Fatalf("Requested: %v", err)
	}
	if err := l.Deleted(first); err != nil {
		t.Fatalf("Deleted: %v", err)
	}
	second, err := l.Requested(Request{Actor: "alice", ProjectName: "Empty", Location: "empty.pmforge"})
	if err != nil {
		t.Fatalf("Requested (uninitialised): %v", err)
	}
	if err := l.Failed(second, errors.New("permission denied")); err != nil {
		t.Fatalf("Failed: %v", err)
	}
	third, err := l.Requested(Request{Actor: "alice", ProjectName: "Interrupted"})
	if err != nil {
		t.Fatalf("Requested (no outcome): %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := openTestLog(t, path, testDEK(0x42)).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 || got[0].ID != third || got[1].ID != second || got[2].ID != first {
		t.Fatalf("List = %+v, want three deletions newest first", got)
	}
	d := got[2]
	if d.Actor != "alice" || d.ProjectID != "project_1" || d.ProjectName != "Bridge" || d.Location != req.Location ||
		d.AuditEvents != 7 || !d.AuditValid || d.AuditTerminalHash != "abc123" || d.Outcome != OutcomeDeleted ||
		d.RequestedAt == "" || d.OutcomeAt == "" {
		t.Fatalf("deleted entry = %+v", d)
	}
	if got[1].Outcome != OutcomeFailed || got[1].Detail != "permission denied" || got[1].ProjectID != "" {
		t.Fatalf("failed entry = %+v", got[1])
	}
	if got[0].Outcome != "" || got[0].OutcomeAt != "" {
		t.Fatalf("interrupted entry = %+v, want no outcome", got[0])
	}
}

func TestDeletionLogAcceptsOneOutcomeOnlyForAKnownRequest(t *testing.T) {
	l := openTestLog(t, filepath.Join(t.TempDir(), "deletions.gopmgr"), testDEK(0x42))
	if err := l.Deleted("deletion_unknown"); err == nil {
		t.Fatal("Deleted accepted an ID that was never requested")
	}
	id, err := l.Requested(Request{Actor: "alice", ProjectName: "Bridge"})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Deleted(id); err != nil {
		t.Fatal(err)
	}
	if err := l.Failed(id, errors.New("late")); err == nil {
		t.Fatal("a second outcome was accepted for the same deletion")
	}
}

func TestDeletionLogIsAppendOnly(t *testing.T) {
	l := openTestLog(t, filepath.Join(t.TempDir(), "deletions.gopmgr"), testDEK(0x42))
	if _, err := l.Requested(Request{Actor: "alice", ProjectName: "Bridge"}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`UPDATE deletion_events SET project_name = 'rewritten'`,
		`DELETE FROM deletion_events`,
	} {
		if _, err := l.db.Exec(stmt); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("%s: err = %v, want the append-only trigger to refuse it", stmt, err)
		}
	}
}

// The "requested" entry must be on disk before a project is removed, so
// commits must be fully synced, not WAL's default NORMAL.
func TestDeletionLogSyncsEveryCommit(t *testing.T) {
	l := openTestLog(t, filepath.Join(t.TempDir(), "deletions.gopmgr"), testDEK(0x42))
	var mode int
	if err := l.db.QueryRow(`PRAGMA synchronous`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != 2 { // 2 = FULL
		t.Fatalf("PRAGMA synchronous = %d, want 2 (FULL)", mode)
	}
}

// The path is concatenated into the SQLCipher DSN, so "?" or "#" could
// smuggle in connection parameters such as a different key.
func TestDeletionLogRejectsDSNCharactersInPath(t *testing.T) {
	for _, name := range []string{"deletions.gopmgr?_pragma_key=x'00'", "deletions.gopmgr#fragment"} {
		if l, err := Open(filepath.Join(t.TempDir(), name), testDEK(0x42)); err == nil {
			_ = l.Close()
			t.Fatalf("Open(%q) accepted a DSN character in the path", name)
		}
	}
}

// Writes on a closed log must fail loudly: DeleteProject relies on an error
// from Requested to keep the project.
func TestDeletionLogWritesFailAfterClose(t *testing.T) {
	l := openTestLog(t, filepath.Join(t.TempDir(), "deletions.gopmgr"), testDEK(0x42))
	id, err := l.Requested(Request{Actor: "alice", ProjectName: "Bridge"})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Requested(Request{Actor: "alice"}); err == nil {
		t.Fatal("Requested succeeded on a closed log")
	}
	if err := l.Deleted(id); err == nil {
		t.Fatal("Deleted succeeded on a closed log")
	}
	if _, err := l.List(); err == nil {
		t.Fatal("List succeeded on a closed log")
	}
	if err := (*Log)(nil).Close(); err != nil {
		t.Fatalf("Close on a nil log: %v", err)
	}
}

func TestDeletionLogIsEncryptedAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deletions.gopmgr")
	l := openTestLog(t, path, testDEK(0x42))
	if _, err := l.Requested(Request{Actor: "alice", ProjectName: "Secret Project Name"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(raw, []byte("SQLite format 3")) || bytes.Contains(raw, []byte("Secret Project Name")) {
		t.Fatal("deletion log is readable without the key")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("deletion log mode = %v, want 0600", info.Mode().Perm())
	}
	wrong, err := Open(path, testDEK(0x43))
	if err == nil {
		_, err = wrong.List()
		_ = wrong.Close()
	}
	if err == nil {
		t.Fatal("deletion log opened and listed with the wrong key")
	}
}
