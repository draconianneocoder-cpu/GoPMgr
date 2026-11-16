// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// migratedProject encrypts a fresh plaintext project with dek and returns
// its path; the unencrypted copy is beside it.
func migratedProject(t *testing.T, dek []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.gopmgr")
	plain, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.UpsertProject(Project{Name: "Copy deletion", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := plain.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := MigratePlaintextToEncrypted(path, dek); err != nil {
		t.Fatal(err)
	}
	return path
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func TestRemovePlaintextBackup(t *testing.T) {
	key := testDEK(t, 0x44)
	cases := []struct {
		name string
		// arrange changes the files after migration; dek is the key used to
		// delete.
		arrange func(t *testing.T, project string)
		dek     []byte
		wantErr string // "" for success; ErrNoPlaintextBackup by its text
		// kept lists files (relative to the copy) that must still exist after
		// a refusal.
		kept []string
	}{
		{
			name: "the right key removes the copy and a leftover WAL",
			arrange: func(t *testing.T, project string) {
				writeTestFile(t, PlaintextBackupPath(project)+"-wal", "plaintext pages")
			},
			dek: key,
		},
		{
			name:    "a wrong key keeps everything",
			dek:     testDEK(t, 0x55),
			wantErr: "did not open",
			kept:    []string{""},
		},
		{
			name: "a damaged encrypted project keeps everything",
			arrange: func(t *testing.T, project string) {
				damagePage(t, project)
			},
			dek:     key,
			wantErr: "kept",
			kept:    []string{""},
		},
		{
			name: "nothing left to delete",
			arrange: func(t *testing.T, project string) {
				if err := os.Remove(PlaintextBackupPath(project)); err != nil {
					t.Fatal(err)
				}
			},
			dek:     key,
			wantErr: ErrNoPlaintextBackup.Error(),
		},
		{
			name: "a file that is not SQLite is left alone",
			arrange: func(t *testing.T, project string) {
				writeTestFile(t, PlaintextBackupPath(project), "someone's notes, not a database")
			},
			dek:     key,
			wantErr: "left alone",
			kept:    []string{""},
		},
		{
			name: "a directory at the copy's name is left alone",
			arrange: func(t *testing.T, project string) {
				backup := PlaintextBackupPath(project)
				if err := os.Remove(backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(backup, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			dek:     key,
			wantErr: "not a regular file",
			kept:    []string{""},
		},
		{
			name: "a directory at a sidecar's name is left alone, and so is the copy",
			arrange: func(t *testing.T, project string) {
				if err := os.Mkdir(PlaintextBackupPath(project)+"-wal", 0o700); err != nil {
					t.Fatal(err)
				}
			},
			dek:     key,
			wantErr: "not a regular file",
			kept:    []string{"", "-wal"},
		},
		{
			name: "a sidecar left on its own is still deleted",
			arrange: func(t *testing.T, project string) {
				backup := PlaintextBackupPath(project)
				if err := os.Remove(backup); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, backup+"-wal", "plaintext pages")
			},
			dek: key,
		},
	}
	if runtime.GOOS != "windows" {
		cases = append(cases, struct {
			name    string
			arrange func(t *testing.T, project string)
			dek     []byte
			wantErr string
			kept    []string
		}{
			name: "a symlink at the copy's name is left alone, and so is its target",
			arrange: func(t *testing.T, project string) {
				backup := PlaintextBackupPath(project)
				target := backup + ".moved"
				if err := os.Rename(backup, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, backup); err != nil {
					t.Fatal(err)
				}
			},
			dek:     key,
			wantErr: "not a regular file",
			kept:    []string{"", ".moved"},
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := migratedProject(t, key)
			backup := PlaintextBackupPath(project)
			if tc.arrange != nil {
				tc.arrange(t, project)
			}
			removed, err := RemovePlaintextBackup(project, tc.dek)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("RemovePlaintextBackup = %v, want an error containing %q", err, tc.wantErr)
				}
				for _, suffix := range tc.kept {
					if !exists(t, backup+suffix) {
						t.Fatalf("%s was removed after a refusal", backup+suffix)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("RemovePlaintextBackup: %v", err)
			}
			if removed != backup {
				t.Fatalf("removed %q, want %q", removed, backup)
			}
			for _, f := range plaintextBackupFiles(project) {
				if exists(t, f) {
					t.Fatalf("%s is still there", f)
				}
			}
			if has, err := HasPlaintextBackup(project); err != nil || has {
				t.Fatalf("HasPlaintextBackup = %v, %v after deleting", has, err)
			}
			d, err := OpenEncryptedReadOnly(project, key)
			if err != nil {
				t.Fatalf("the encrypted project no longer opens: %v", err)
			}
			_ = d.Close()
		})
	}
}

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// damagePage overwrites part of the encrypted project's second page, which
// SQLCipher's page authentication detects.
func damagePage(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteAt(make([]byte, 64), 4096+512); err != nil {
		t.Fatal(err)
	}
}
