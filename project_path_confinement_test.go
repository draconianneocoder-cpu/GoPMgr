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

// TestProjectPathForAcceptsCurrentAndLegacyExtension pins the extension
// check in projectPathFor to accept BOTH ".gopmgr" (written by this build)
// and ".pmforge" (written before the 2026-08-04 PMForge -> GoPMgr rename,
// and never rewritten — nothing migrates existing project files). The
// extension is a persistence boundary, not a validation nicety: every
// project file already on a user's disk ends in one of these two, so if
// support for either were ever dropped, DeleteProject/CloneProject/
// OpenProject would reject real existing projects outright.
//
// This test replaces TestProjectPathForRejectsWrongExtension, which
// asserted ".gopmgr" was *rejected* — correct before this rename, inverted
// deliberately here now that ".gopmgr" is the current extension. See
// DEVELOPER_HANDBOOK.md §9 for the rename this inversion is part of.
//
// This calls projectPathFor directly rather than going through DeleteProject:
// DeleteProject also opens the target as an encrypted SQLite database for
// its audit-log entry, so a fake (non-database) file fails there regardless
// of extension, which would make the assertion pass for the wrong reason
// even if the extension check itself were broken or renamed.
func TestProjectPathForAcceptsCurrentAndLegacyExtension(t *testing.T) {
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
	// the directory-confinement check covered above. ".pmforg"/".gopmg" are
	// deliberate one-character-off typos of the two valid extensions, not
	// an arbitrary wrong extension like ".txt": they catch a check that was
	// narrowed or corrupted by one character, which a generic case would
	// miss.
	projectsDir := filepath.Join(user.DataDir, "projects")
	for _, ext := range []string{".txt", ".pmforg", ".gopmg"} {
		wrongExt := filepath.Join(projectsDir, "not-a-project"+ext)
		if _, _, err := app.projectPathFor(wrongExt); err == nil {
			t.Fatalf("projectPathFor accepted a path with extension %q, want only .gopmgr/.pmforge accepted", ext)
		}
	}

	// Positive controls: both the current and legacy extension, in the same
	// directory, must be accepted. Without these, a check that rejected
	// every path (not just wrong extensions) would also pass the loop above.
	for _, ext := range []string{".gopmgr", ".pmforge"} {
		rightExt := filepath.Join(projectsDir, "real-project"+ext)
		if _, _, err := app.projectPathFor(rightExt); err != nil {
			t.Fatalf("projectPathFor rejected a %s path: %v", ext, err)
		}
	}
}
