// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopmgr/internal/users"
)

// TestPreRenameInstallIsUsableAfterMigration builds a real, byte-accurate
// pre-2026-08-04 install (current schema, old "PMForge" leaf name) using
// the App/users public API — the same one the shipped pre-rename binary
// used, since DefaultRootDir's leaf name was the only thing that changed —
// then proves the migrated install is actually USABLE: login succeeds and
// the account's project is found, from a fresh App/Store that never saw
// the old root, with the old root deleted first (simulating a user who
// took MigrateLegacyRoot's doc comment's invitation to "delete the old
// copy at leisure").
//
// The existing MigrateLegacyRoot* tests only assert that files get copied.
// They can't see the bug this test catches: users.Account.DataDir used to
// be trusted verbatim from the data_dir column, which CreateAccount writes
// once as an absolute path under whatever root was active at creation
// time. After a PMForge -> GoPMgr migration, that column still pointed at
// the deleted old PMForge tree, so every migrated account silently kept
// reading and writing there instead of the copied data. Store.Authenticate
// and Store.List now recompute DataDir from the current root on every
// read instead of trusting the column (see internal/users/store.go).
//
// Scope note: this fixture is current-schema data in an old-layout
// location. internal/users' migrate() is additive-only (DEVELOPER_HANDBOOK.md),
// so old-schema data should migrate the same way, but this test does not
// cover that axis.
func TestPreRenameInstallIsUsableAfterMigration(t *testing.T) {
	// XDG_DATA_HOME pins legacyRootCandidates() and DefaultRootDir() to
	// this temp dir so the test never touches the real machine's home
	// directory, on any OS.
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	oldRoot := filepath.Join(base, "PMForge") // pre-rename DefaultRootDir's leaf name

	oldStore, err := users.Open(oldRoot)
	if err != nil {
		t.Fatalf("users.Open(oldRoot): %v", err)
	}
	oldApp := &App{store: oldStore}
	if _, err := oldApp.CreateAccount("alice", "Alice", "correct horse battery staple", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := oldApp.Login("alice", "correct horse battery staple"); err != nil {
		t.Fatalf("Login (pre-migration): %v", err)
	}
	project, err := oldApp.CreateProject("Pre-Rename Plan", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	oldApp.shutdown(context.Background())

	if _, err := os.Stat(project.Path); err != nil {
		t.Fatalf("fixture project missing before migration: %v", err)
	}

	// Resolve the new root and migrate through the exact same public
	// entry points main.go's NewApp calls at real startup.
	newRoot, err := users.DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	if newRoot != filepath.Join(base, "GoPMgr") {
		t.Fatalf("DefaultRootDir = %q, want %q", newRoot, filepath.Join(base, "GoPMgr"))
	}
	migrated, err := users.MigrateLegacyRoot(newRoot)
	if err != nil {
		t.Fatalf("MigrateLegacyRoot: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to run")
	}

	// Simulate the user acting on MigrateLegacyRoot's invitation to delete
	// the old copy: if anything in the migrated data still points back at
	// oldRoot, the assertions below fail instead of silently passing
	// because the stale path still happens to resolve.
	if err := os.RemoveAll(oldRoot); err != nil {
		t.Fatalf("RemoveAll(oldRoot): %v", err)
	}

	newStore, err := users.Open(newRoot)
	if err != nil {
		t.Fatalf("users.Open(newRoot): %v", err)
	}
	newApp := &App{store: newStore}
	t.Cleanup(func() { newApp.shutdown(context.Background()) })

	acc, err := newApp.Login("alice", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login (post-migration): %v", err)
	}
	if wantDataDir := filepath.Join(newRoot, "alice"); acc.DataDir != wantDataDir {
		t.Fatalf("DataDir after migration = %q, want %q (still pointing at the deleted old root is exactly the bug this test catches)", acc.DataDir, wantDataDir)
	}

	list, err := newApp.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects (post-migration): %v", err)
	}
	if len(list) != 1 || list[0].Name != "Pre-Rename Plan" {
		t.Fatalf("ListProjects after migration = %#v, want exactly [Pre-Rename Plan]", list)
	}
}
