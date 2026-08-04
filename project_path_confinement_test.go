// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"

	"gopmgr/internal/db"
)

// TestPathTakingIPCMethodsConfineToOwnProjectsDir locks in F-1 from the
// 2026-06-29 security review: every IPC method that opens, mutates, or
// archives a project by a frontend-supplied path must reject paths outside
// the signed-in user's own projects folder, exactly as DeleteProject and
// CloneProject already do via projectPathFor. A regression here would hand a
// logged-in GoPMgr user a filesystem primitive over another user's files
// within the same OS account.
func TestPathTakingIPCMethodsConfineToOwnProjectsDir(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// A plausible target outside the user's projects sandbox: a sibling path
	// under the data root that a path-traversal attempt would aim for.
	outside := filepath.Join(t.TempDir(), "victim.pmforge")
	if d, err := db.InitDB(outside); err != nil {
		t.Fatalf("seed outside file: %v", err)
	} else if err := d.Close(); err != nil {
		t.Fatalf("close outside file: %v", err)
	}

	t.Run("OpenProject", func(t *testing.T) {
		if _, err := app.OpenProject(outside); err == nil {
			t.Fatal("OpenProject accepted a path outside the user's projects dir")
		}
	})
	t.Run("IsProjectEncrypted", func(t *testing.T) {
		if _, err := app.IsProjectEncrypted(outside); err == nil {
			t.Fatal("IsProjectEncrypted accepted a path outside the user's projects dir")
		}
	})
	t.Run("EncryptProjectAtRest", func(t *testing.T) {
		if _, err := app.EncryptProjectAtRest(outside); err == nil {
			t.Fatal("EncryptProjectAtRest accepted a path outside the user's projects dir")
		}
	})
	t.Run("SecureArchive", func(t *testing.T) {
		if _, err := app.SecureArchive(outside); err == nil {
			t.Fatal("SecureArchive accepted a path outside the user's projects dir")
		}
	})
}

// TestEncryptedDSNRejectsAmbiguousPath locks in F-2: a project path containing
// a DSN-significant character ('?' or '#') must be refused rather than folded
// into the SQLCipher DSN, where it could inject or override _pragma_* options
// (including the key).
func TestEncryptedDSNRejectsAmbiguousPath(t *testing.T) {
	dek := make([]byte, 32)
	for _, p := range []string{
		filepath.Join(t.TempDir(), "weird?_pragma_key=x'00'.pmforge"),
		filepath.Join(t.TempDir(), "frag#.pmforge"),
	} {
		if _, err := db.InitEncryptedDB(p, dek); err == nil {
			t.Fatalf("InitEncryptedDB accepted DSN-ambiguous path %q", p)
		}
	}
}

// TestProjectPathForRejectsWrongExtension pins the ".pmforge" check in
// projectPathFor. The extension is a persistence boundary, not a validation
// nicety: every project file already on a user's disk ends in ".pmforge", so
// if this check's literal were ever renamed alongside the rest of the
// PMForge -> GoPMgr rebrand, DeleteProject/CloneProject/OpenProject would
// either reject every existing project outright or (worse) silently accept
// unrelated files that happen to sit beside one. A regression here fails
// this test instead of surfacing as "my projects disappeared" after upgrade.
//
// This calls projectPathFor directly rather than going through DeleteProject:
// DeleteProject also opens the target as an encrypted SQLite database for
// its audit-log entry, so a fake (non-database) file fails there regardless
// of extension, which would make the assertion pass for the wrong reason
// even if the extension check itself were broken or renamed.
func TestProjectPathForRejectsWrongExtension(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	user := app.CurrentUser()
	if user == nil {
		t.Fatal("CurrentUser is nil after CreateAccount")
	}

	// projectPathFor is pure path validation (no filesystem access), so
	// these paths don't need to exist on disk. They live inside the user's
	// own projects directory so the test isolates the extension check from
	// the directory-confinement check covered above. ".gopmgr" specifically
	// covers the adjacent-rename risk: if a future PMForge -> GoPMgr-style
	// rebrand changed this check's literal to match the new product name,
	// a generic wrong-extension case like ".txt" would still pass while
	// missing that exact regression.
	projectsDir := filepath.Join(user.DataDir, "projects")
	for _, ext := range []string{".txt", ".gopmgr"} {
		wrongExt := filepath.Join(projectsDir, "not-a-project"+ext)
		if _, _, err := app.projectPathFor(wrongExt); err == nil {
			t.Fatalf("projectPathFor accepted a path with extension %q, want only .pmforge accepted", ext)
		}
	}

	// Positive control: the same directory, the correct extension, must be
	// accepted. Without this, a check that rejected every path (not just
	// wrong extensions) would also pass the loop above.
	rightExt := filepath.Join(projectsDir, "real-project.pmforge")
	if _, _, err := app.projectPathFor(rightExt); err != nil {
		t.Fatalf("projectPathFor rejected a .pmforge path: %v", err)
	}
}
