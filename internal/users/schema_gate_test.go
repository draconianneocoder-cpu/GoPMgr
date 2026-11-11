// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"gopmgr/internal/sqlitedriver"
)

// rawSystemDB runs fn against root/system.db on a plain connection, outside
// the Store, and closes it again. The file is left in WAL mode, as every
// GoPMgr build leaves it.
func rawSystemDB(t *testing.T, root string, fn func(conn *sql.DB)) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	conn, err := sql.Open(sqlitedriver.Name, filepath.Join(root, "system.db")+"?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open raw system.db: %v", err)
	}
	conn.SetMaxOpenConns(1)
	fn(conn)
	if err := conn.Close(); err != nil {
		t.Fatalf("close raw system.db: %v", err)
	}
}

func rawExec(t *testing.T, conn *sql.DB, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func systemSchemaStamp(t *testing.T, root string) int {
	t.Helper()
	var stamp int
	rawSystemDB(t, root, func(conn *sql.DB) {
		if err := conn.QueryRow(`PRAGMA user_version`).Scan(&stamp); err != nil {
			t.Fatalf("read user_version: %v", err)
		}
	})
	return stamp
}

func schemaObjectNames(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	rawSystemDB(t, root, func(conn *sql.DB) {
		rows, err := conn.Query(`SELECT name FROM sqlite_master ORDER BY name`)
		if err != nil {
			t.Fatalf("read sqlite_master: %v", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("scan sqlite_master: %v", err)
			}
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate sqlite_master: %v", err)
		}
	})
	return names
}

func readSystemDBFile(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "system.db"))
	if err != nil {
		t.Fatalf("read system.db: %v", err)
	}
	return data
}

func TestSystemSchemaReadable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		stamp     int
		wantErr   bool
		wantNewer bool
	}{
		{"negative stamp is invalid, not newer", -1, true, false},
		{"file from before the gate", 0, false, false},
		{"this build's level", systemSchemaReader, false, false},
		{"one level newer", systemSchemaReader + 1, true, true},
		{"largest stamp", math.MaxInt32, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := systemSchemaReadable(tc.stamp)
			if (err != nil) != tc.wantErr {
				t.Fatalf("systemSchemaReadable(%d) = %v, want error %v", tc.stamp, err, tc.wantErr)
			}
			if got := errors.Is(err, ErrSystemSchemaNewer); got != tc.wantNewer {
				t.Fatalf("systemSchemaReadable(%d) newer = %v, want %v (err %v)", tc.stamp, got, tc.wantNewer, err)
			}
		})
	}
}

func TestOpenStampsANewSystemDatabase(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := systemSchemaStamp(t, root); got != systemSchemaReader {
		t.Fatalf("stamp = %d, want %d", got, systemSchemaReader)
	}
}

// A file from before the gate carries stamp 0 and real accounts. It must
// still open, keep its accounts, and leave stamped.
func TestOpenStampsAnOlderSystemDatabase(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), templates.status.leaf)
	if err := copyTemplate(templates.status, root); err != nil {
		t.Fatalf("copy template: %v", err)
	}
	rawSystemDB(t, root, func(conn *sql.DB) { rawExec(t, conn, `PRAGMA user_version = 0`) })
	if got := systemSchemaStamp(t, root); got != 0 {
		t.Fatalf("fixture stamp = %d, want 0", got)
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	accounts, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("accounts after upgrade = %d, want 2", len(accounts))
	}
	if got := systemSchemaStamp(t, root); got != systemSchemaReader {
		t.Fatalf("stamp = %d, want %d", got, systemSchemaReader)
	}
}

// Reopening a file already at this build's level writes nothing to it.
func TestReopenLeavesAStampedSystemDatabaseUnchanged(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "GoPMgr")
	for range 2 {
		store, err := Open(root)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
	before := readSystemDBFile(t, root)
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !bytes.Equal(before, readSystemDBFile(t, root)) {
		t.Fatal("reopening a stamped system.db changed the file")
	}
}

// A file marked by a newer build is refused before this build creates or
// changes anything in it.
func TestOpenRefusesANewerSystemDatabase(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "GoPMgr")
	newer := systemSchemaReader + 1
	rawSystemDB(t, root, func(conn *sql.DB) {
		rawExec(t, conn,
			`CREATE TABLE from_a_newer_build(id INTEGER PRIMARY KEY)`,
			fmt.Sprintf(`PRAGMA user_version = %d`, newer),
		)
	})
	before := readSystemDBFile(t, root)

	store, err := Open(root)
	if err == nil {
		_ = store.Close()
		t.Fatal("Open accepted a system.db that needs a newer reader")
	}
	if !errors.Is(err, ErrSystemSchemaNewer) {
		t.Fatalf("Open error = %v, want ErrSystemSchemaNewer", err)
	}
	// The headless command prints this error as its own text.
	if !strings.Contains(err.Error(), "last opened by a newer GoPMgr; install that version or a later one") {
		t.Fatalf("Open error does not tell the user what to do: %v", err)
	}

	if !bytes.Equal(before, readSystemDBFile(t, root)) {
		t.Fatal("a refused Open changed system.db")
	}
	if got, want := schemaObjectNames(t, root), []string{"from_a_newer_build"}; !slices.Equal(got, want) {
		t.Fatalf("schema after refusal = %v, want %v", got, want)
	}
	if got := systemSchemaStamp(t, root); got != newer {
		t.Fatalf("stamp after refusal = %d, want %d", got, newer)
	}
}

func TestOpenRefusesAnInvalidSchemaStamp(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "GoPMgr")
	rawSystemDB(t, root, func(conn *sql.DB) { rawExec(t, conn, `PRAGMA user_version = -1`) })

	store, err := Open(root)
	if err == nil {
		_ = store.Close()
		t.Fatal("Open accepted a negative schema stamp")
	}
	if errors.Is(err, ErrSystemSchemaNewer) {
		t.Fatalf("a negative stamp was reported as a newer build: %v", err)
	}
	if got := schemaObjectNames(t, root); len(got) != 0 {
		t.Fatalf("schema after refusal = %v, want none", got)
	}
}

// A store opened before a newer build marks the file must stop writing to
// it. raiseSystemSchemaStamp stands in for that newer build, and also shows
// that a stamp below the wanted level is raised, not only a stamp of 0.
func TestOpenStoreStopsWritingOnceANewerBuildMarksTheFile(t *testing.T) {
	t.Parallel()
	store := newStatusStore(t)
	newer := systemSchemaReader + 1
	ctx := context.Background()

	if err := store.inWriteTx("mark as newer", func(ctx context.Context, q accountWriter) error {
		return raiseSystemSchemaStamp(ctx, q, newer)
	}); err != nil {
		t.Fatalf("raise stamp: %v", err)
	}
	var stamp int
	if err := store.conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&stamp); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if stamp != newer {
		t.Fatalf("stamp = %d, want %d", stamp, newer)
	}

	ran := false
	err := store.inWriteTx("write after mark", func(context.Context, accountWriter) error {
		ran = true
		return nil
	})
	if !errors.Is(err, ErrSystemSchemaNewer) {
		t.Fatalf("write after mark = %v, want ErrSystemSchemaNewer", err)
	}
	if ran {
		t.Fatal("the write ran against a file marked for a newer build")
	}
	if err := store.SetDisabled("alice", "bob", true); !errors.Is(err, ErrSystemSchemaNewer) {
		t.Fatalf("SetDisabled after mark = %v, want ErrSystemSchemaNewer", err)
	}
}

// The stamp is written inside the write transaction: when that transaction
// fails, the stamp goes back with it. This runs on the project's own
// SQLCipher build, which has defensive mode on.
func TestSchemaStampRollsBackWithItsTransaction(t *testing.T) {
	t.Parallel()
	store := newStatusStore(t)
	failed := errors.New("later step failed")

	err := store.inWriteTx("failed raise", func(ctx context.Context, q accountWriter) error {
		if err := raiseSystemSchemaStamp(ctx, q, systemSchemaReader+1); err != nil {
			return err
		}
		inside, err := readSystemSchemaStamp(ctx, q)
		if err != nil {
			return err
		}
		if inside != systemSchemaReader+1 {
			return fmt.Errorf("stamp inside the transaction = %d, want %d", inside, systemSchemaReader+1)
		}
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatalf("inWriteTx = %v, want the injected failure", err)
	}
	var stamp int
	if err := store.conn.QueryRow(`PRAGMA user_version`).Scan(&stamp); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if stamp != systemSchemaReader {
		t.Fatalf("stamp after rollback = %d, want %d", stamp, systemSchemaReader)
	}
}

func TestRaiseSystemSchemaStampNeverLowers(t *testing.T) {
	t.Parallel()
	store := newStatusStore(t)
	if err := store.inWriteTx("lower", func(ctx context.Context, q accountWriter) error {
		return raiseSystemSchemaStamp(ctx, q, 0)
	}); err != nil {
		t.Fatalf("raise to 0: %v", err)
	}
	var stamp int
	if err := store.conn.QueryRow(`PRAGMA user_version`).Scan(&stamp); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if stamp != systemSchemaReader {
		t.Fatalf("stamp = %d, want %d", stamp, systemSchemaReader)
	}
}

// Several processes may upgrade the same older file at once. Each Open must
// succeed and the file must end stamped exactly once.
func TestConcurrentOpensStampAnOlderSystemDatabase(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), templates.status.leaf)
	if err := copyTemplate(templates.status, root); err != nil {
		t.Fatalf("copy template: %v", err)
	}
	rawSystemDB(t, root, func(conn *sql.DB) { rawExec(t, conn, `PRAGMA user_version = 0`) })

	const openers = 6
	errs := make([]error, openers)
	var wg sync.WaitGroup
	for i := range openers {
		wg.Go(func() {
			store, err := Open(root)
			if err != nil {
				errs[i] = err
				return
			}
			errs[i] = store.Close()
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("opener %d: %v", i, err)
		}
	}
	if got := systemSchemaStamp(t, root); got != systemSchemaReader {
		t.Fatalf("stamp = %d, want %d", got, systemSchemaReader)
	}
}

// accountRows is every stored credential and preference the gated writers
// could change.
func accountRows(t *testing.T, store *Store) []string {
	t.Helper()
	var out []string
	for _, query := range []string{
		`SELECT username || '|' || password_hash || '|' || coalesce(wrapped_dek_pw,'') || '|' || coalesce(last_export_directory,'') FROM users ORDER BY username`,
		`SELECT id || '|' || username || '|' || code_hash || '|' || coalesce(wrapped_dek,'') || '|' || used FROM recovery_codes ORDER BY id`,
	} {
		rows, err := store.conn.Query(query)
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				_ = rows.Close()
				t.Fatalf("scan: %v", err)
			}
			out = append(out, row)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			t.Fatalf("iterate: %v", err)
		}
	}
	return out
}

// Password, recovery-code and preference writes do not all share one
// transaction helper. Each must still refuse a file a newer build has
// marked, and leave the stored rows as they were.
func TestCredentialWritesStopOnceANewerBuildMarksTheFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		write func(store *Store, dek []byte, codes []string) error
	}{
		{"change password", func(store *Store, _ []byte, _ []string) error {
			return store.ChangePassword("alice", oldPassword, "a-different-password")
		}},
		{"issue recovery codes", func(store *Store, dek []byte, _ []string) error {
			_, err := store.IssueRecoveryCodes("alice", dek)
			return err
		}},
		{"reset with a recovery code", func(store *Store, _ []byte, codes []string) error {
			return store.ResetWithRecoveryCode("alice", codes[0], "a-different-password")
		}},
		{"remember the export folder", func(store *Store, _ []byte, _ []string) error {
			return store.SetLastExportDirectory("alice", "/somewhere/else")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store, dek, codes := storeFromTemplate(t, templates.rotation)
			if err := store.inWriteTx("mark as newer", func(ctx context.Context, q accountWriter) error {
				return raiseSystemSchemaStamp(ctx, q, systemSchemaReader+1)
			}); err != nil {
				t.Fatalf("raise stamp: %v", err)
			}
			before := accountRows(t, store)
			if len(before) < 2 {
				t.Fatalf("fixture rows = %d, want an account and its recovery codes", len(before))
			}

			if err := tc.write(store, dek, codes); !errors.Is(err, ErrSystemSchemaNewer) {
				t.Fatalf("write = %v, want ErrSystemSchemaNewer", err)
			}
			if after := accountRows(t, store); !slices.Equal(before, after) {
				t.Fatal("a refused write changed the stored rows")
			}
		})
	}
}
