// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/deletionlog"
)

func newDeletionTestApp(t *testing.T) *App {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return app
}

func onlyDeletion(t *testing.T, app *App) ProjectDeletionWire {
	t.Helper()
	got, err := app.ListProjectDeletions()
	if err != nil {
		t.Fatalf("ListProjectDeletions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListProjectDeletions = %+v, want exactly one entry", got)
	}
	return got[0]
}

// The project's own audit chain is removed with the project, so the record
// must outlive it and pin the chain's final state.
func TestDeleteProjectLeavesDurableRecord(t *testing.T) {
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Bridge Retrofit", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := app.OpenProject(file.Path)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	chain, err := app.requireDB().VerifyAuditChain(proj.ID)
	if err != nil || !chain.Valid || chain.CheckedEvents == 0 || chain.TerminalEventHash == "" {
		t.Fatalf("audit chain before delete = %+v, %v; want a valid, non-empty chain", chain, err)
	}

	if err := app.DeleteProject(file.Path); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(file.Path)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("project folder still present (err %v)", err)
	}
	d := onlyDeletion(t, app)
	wantLocation := filepath.Join(filepath.Base(filepath.Dir(file.Path)), "project.gopmgr")
	if d.ProjectID != proj.ID || d.ProjectName != "Bridge Retrofit" || d.DeletedBy != "alice" ||
		d.Location != wantLocation || d.Outcome != deletionlog.OutcomeDeleted || d.RequestedAt == "" || d.OutcomeAt == "" ||
		d.AuditEvents != chain.CheckedEvents || !d.AuditValid || d.AuditTerminalHash != chain.TerminalEventHash {
		t.Fatalf("deletion record = %+v, want it to match the deleted project and its final chain %+v", d, chain)
	}
}

// An empty database with no project row (left by a stale path before
// projectPathFor refused missing files) could not be deleted, because the
// old delete step needed the project row. It holds no project data.
func TestDeleteProjectRemovesUninitialisedFile(t *testing.T) {
	app := newDeletionTestApp(t)
	projects := filepath.Join(app.requireUser().DataDir, "projects")
	if err := os.MkdirAll(projects, 0o700); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(projects, "stray.pmforge")
	app.mu.RLock()
	dek, err := app.requireDEKLocked()
	app.mu.RUnlock()
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.InitEncryptedDB(stray, dek)
	if err != nil {
		t.Fatalf("create uninitialised file: %v", err)
	}
	if _, err := d.GetProject(); !errors.Is(err, db.ErrNoProject) {
		t.Fatalf("fixture has a project row (err %v)", err)
	}
	_ = d.Close()

	if err := app.DeleteProject(stray); err != nil {
		t.Fatalf("DeleteProject(uninitialised): %v", err)
	}
	if _, err := os.Stat(stray); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("uninitialised file still present (err %v)", err)
	}
	rec := onlyDeletion(t, app)
	if rec.ProjectID != "" || rec.ProjectName != "stray" || rec.Outcome != deletionlog.OutcomeDeleted || rec.AuditEvents != 0 {
		t.Fatalf("deletion record = %+v, want an uninitialised 'stray' entry", rec)
	}
}

// A file the user's key cannot open may still be recoverable, so it is not
// deleted and no deletion is recorded.
func TestDeleteProjectRefusesUnreadableFileWithoutRecording(t *testing.T) {
	app := newDeletionTestApp(t)
	projects := filepath.Join(app.requireUser().DataDir, "projects")
	if err := os.MkdirAll(projects, 0o700); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(projects, "unreadable.pmforge")
	if err := os.WriteFile(unreadable, []byte("not a database under this key, padded well past any header"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteProject(unreadable); err == nil {
		t.Fatal("DeleteProject removed a file it could not open")
	}
	if _, err := os.Stat(unreadable); err != nil {
		t.Fatalf("unreadable file was touched: %v", err)
	}
	got, err := app.ListProjectDeletions()
	if err != nil || len(got) != 0 {
		t.Fatalf("ListProjectDeletions = %+v, %v; want no entry for a refused deletion", got, err)
	}
}

// A tampered chain must not stop the user deleting their project; the record
// shows the chain did not verify.
func TestDeleteProjectRecordsTamperedAuditChain(t *testing.T) {
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Tampered", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := app.OpenProject(file.Path)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if _, err := app.requireDB().Conn.Exec(
		`UPDATE audit_events SET after_canonical_json = ? WHERE project_id = ? AND sequence_number = 1`,
		`{"name":"tampered"}`, proj.ID,
	); err != nil {
		t.Fatalf("tamper audit chain: %v", err)
	}
	if err := app.DeleteProject(file.Path); err != nil {
		t.Fatalf("DeleteProject(tampered): %v", err)
	}
	// VerifyAuditChain stops at the first bad event: the count includes it and
	// the hash is the last good one (none, since event 1 is the bad one).
	if rec := onlyDeletion(t, app); rec.AuditValid || rec.Outcome != deletionlog.OutcomeDeleted ||
		rec.AuditEvents != 1 || rec.AuditTerminalHash != "" {
		t.Fatalf("deletion record = %+v, want deleted with an invalid chain stopped at event 1", rec)
	}
}

// No record, no deletion: if the log cannot be written, the project stays.
func TestDeleteProjectKeepsProjectWhenLogUnavailable(t *testing.T) {
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Kept", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	// A directory where the log file belongs makes the log unopenable.
	if err := os.Mkdir(filepath.Join(app.requireUser().DataDir, "deletions.gopmgr"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteProject(file.Path); err == nil {
		t.Fatal("DeleteProject succeeded without a deletion log")
	}
	if _, err := os.Stat(file.Path); err != nil {
		t.Fatalf("project removed without a record: %v", err)
	}
}

// Opening the log is not enough: if the "requested" entry cannot be written,
// nothing is removed.
func TestDeleteProjectKeepsProjectWhenRequestCannotBeRecorded(t *testing.T) {
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Kept too", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	orig := recordProjectDeletionRequested
	recordProjectDeletionRequested = func(*deletionlog.Log, deletionlog.Request) (string, error) {
		return "", errors.New("disk full")
	}
	t.Cleanup(func() { recordProjectDeletionRequested = orig })

	if err := app.DeleteProject(file.Path); err == nil {
		t.Fatal("DeleteProject succeeded without recording the request")
	}
	if _, err := os.Stat(file.Path); err != nil {
		t.Fatalf("project removed without a record: %v", err)
	}
}

// Two deletes of the same legacy project at once: one removes it, the other
// finds it gone. Without serialisation the second could recreate an empty
// file through the pre-delete read and log a deletion that never mattered.
func TestConcurrentDeletesOfOneProjectRecordOnce(t *testing.T) {
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Raced", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	legacy := filepath.Join(app.requireUser().DataDir, "projects", "raced.pmforge")
	if err := os.Rename(file.Path, legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(file.Path)); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			errs <- app.DeleteProject(legacy)
		}()
	}
	start.Done()
	first, second := <-errs, <-errs
	if (first == nil) == (second == nil) {
		t.Fatalf("errors = %v, %v; want exactly one success", first, second)
	}
	if other := errors.Join(first, second); !errors.Is(other, ErrProjectNotFound) {
		t.Fatalf("losing delete error = %v, want ErrProjectNotFound", other)
	}
	if _, err := os.Stat(legacy); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("legacy file present after deletes (err %v)", err)
	}
	if rec := onlyDeletion(t, app); rec.ProjectName != "Raced" || rec.ProjectID == "" {
		t.Fatalf("deletion record = %+v, want the one real deletion", rec)
	}
}

// When removal fails the log records the failure and its cause. Removal is
// not atomic, so "failed" can mean some files were already removed.
func TestDeleteProjectRecordsFailedRemoval(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Locked", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	// A read-only subfolder cannot be emptied, so removing the project folder
	// fails, while the project itself still opens (the folder stays writable
	// for SQLite's WAL files, which the pre-delete read needs).
	locked := filepath.Join(filepath.Dir(file.Path), "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	if err := app.DeleteProject(file.Path); err == nil {
		t.Fatal("DeleteProject reported success for a folder it could not remove")
	}
	if _, err := os.Stat(locked); err != nil {
		t.Fatalf("locked subfolder unexpectedly removed: %v", err)
	}
	rec := onlyDeletion(t, app)
	if rec.Outcome != deletionlog.OutcomeFailed || rec.Detail == "" {
		t.Fatalf("deletion record = %+v, want a failed outcome with its cause", rec)
	}
}

// If the project is gone but the final log write fails, the deletion still
// succeeded; the log shows the request with no outcome rather than a lie.
func TestDeleteProjectSucceedsWhenOutcomeCannotBeRecorded(t *testing.T) {
	app := newDeletionTestApp(t)
	file, err := app.CreateProject("Outcome lost", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	orig := recordProjectDeleted
	recordProjectDeleted = func(*deletionlog.Log, string) error { return errors.New("disk full") }
	t.Cleanup(func() { recordProjectDeleted = orig })

	if err := app.DeleteProject(file.Path); err != nil {
		t.Fatalf("DeleteProject: %v, want success once the project is removed", err)
	}
	if _, err := os.Stat(filepath.Dir(file.Path)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("project folder still present (err %v)", err)
	}
	if rec := onlyDeletion(t, app); rec.Outcome != "" || rec.ProjectName != "Outcome lost" {
		t.Fatalf("deletion record = %+v, want the request with no outcome", rec)
	}
}
