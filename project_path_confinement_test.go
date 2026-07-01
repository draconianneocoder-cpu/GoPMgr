// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"

	"pmforge/internal/db"
)

// TestPathTakingIPCMethodsConfineToOwnProjectsDir locks in F-1 from the
// 2026-06-29 security review: every IPC method that opens, mutates, or
// archives a project by a frontend-supplied path must reject paths outside
// the signed-in user's own projects folder, exactly as DeleteProject and
// CloneProject already do via projectPathFor. A regression here would hand a
// logged-in PMForge user a filesystem primitive over another user's files
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
