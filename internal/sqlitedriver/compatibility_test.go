// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

const compatibilityKey = "x'4242424242424242424242424242424242424242424242424242424242424242'"

// The fixtures were written on 2026-11-10 by the engine GoPMgr shipped until
// then (go-sqlcipher v4.4.2: SQLCipher 4.4.2 on SQLite 3.33.0), in WAL mode
// with the key above. Their hashes are pinned so a later engine cannot
// quietly regenerate them. docs/testing/sqlcipher-4.20-upgrade.md records
// how they were made and the one-off check in the other direction.
var legacyEngineFixtures = []struct {
	file, sha256 string
	keyed        bool
}{
	{"sqlcipher-4.4.2-keyed.db", "d1bd456f7a1a768e4810709c1f965c890dd475a5660bc39d816596fc90aa7381", true},
	{"sqlcipher-4.4.2-plain.db", "24aff422cddef7c8a612d06ca5d1d2a4b2272e22b1fb65c7fc92a043a8ba7d37", false},
}

// TestFilesFromSQLCipher442OpenUnderTheCurrentEngine is the upgrade path
// every existing user takes: a file the old engine wrote must read back
// exactly, pass both integrity checks, accept writes, and stay encrypted.
func TestFilesFromSQLCipher442OpenUnderTheCurrentEngine(t *testing.T) {
	for _, fixture := range legacyEngineFixtures {
		t.Run(fixture.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", fixture.file))
			if err != nil {
				t.Fatal(err)
			}
			assertSHA256(t, fixture.file, data, fixture.sha256)
			path := filepath.Join(t.TempDir(), fixture.file)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			dsn := func(key string) string {
				if !fixture.keyed {
					return path + "?_journal_mode=WAL&_foreign_keys=on"
				}
				return compatibilityDSN(path, key) + "&_foreign_keys=on"
			}
			conn, err := sql.Open(Name, dsn(compatibilityKey))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			checkCompatibilityValue(t, conn, 1, "old café")
			checkCompatibilityValue(t, conn, 2, "東京")
			var note string
			if err := conn.QueryRow("SELECT note FROM child WHERE parent_id=2").Scan(&note); err != nil || note != "second" {
				t.Fatalf("indexed child row = %q, %v; want \"second\"", note, err)
			}
			var integrity string
			if err := conn.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("integrity_check = %q, %v", integrity, err)
			}
			if fixture.keyed {
				rows, err := conn.Query("PRAGMA cipher_integrity_check")
				if err != nil {
					t.Fatal(err)
				}
				failed := rows.Next()
				// Without Err, a check that could not run reads as one
				// that found nothing.
				if err := errors.Join(rows.Err(), rows.Close()); err != nil {
					t.Fatal(err)
				}
				if failed {
					t.Fatal("cipher_integrity_check reported a failure")
				}
			}
			if _, err := conn.Exec("INSERT INTO parent VALUES(3,'written by the current engine')"); err != nil {
				t.Fatalf("write to a file from the old engine: %v", err)
			}
			if _, err := conn.Exec("DELETE FROM parent WHERE id=1"); err != nil {
				t.Fatal(err)
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}

			reopened, err := sql.Open(Name, dsn(compatibilityKey))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			checkCompatibilityValue(t, reopened, 3, "written by the current engine")
			var children int
			if err := reopened.QueryRow("SELECT count(*) FROM child").Scan(&children); err != nil || children != 1 {
				t.Fatalf("children after the cascading delete = %d, %v; want 1", children, err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
			if !fixture.keyed {
				return
			}
			wrong, err := sql.Open(Name, dsn("wrong-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = wrong.Close() })
			if err := wrong.QueryRow("SELECT count(*) FROM parent").Scan(&children); err == nil {
				t.Fatal("the current engine opened the file with the wrong key")
			}
			header, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(header) < 16 || string(header[:16]) == "SQLite format 3\x00" {
				t.Fatal("the file is no longer encrypted")
			}
		})
	}
}

func compatibilityDSN(path, key string) string {
	return path + "?_pragma_key=" + url.QueryEscape(key) + "&_journal_mode=WAL&_pragma_cipher_page_size=4096"
}

func checkCompatibilityValue(t *testing.T, conn *sql.DB, id int, want string) {
	t.Helper()
	var got string
	if err := conn.QueryRow("SELECT v FROM parent WHERE id=?", id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("row %d=%q, want %q", id, got, want)
	}
}
