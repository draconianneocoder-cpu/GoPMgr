// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"
)

func TestFailedCommitCleanup(t *testing.T) {
	for _, keyed := range []bool{false, true} {
		name := "plaintext"
		if keyed {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			for _, mode := range []string{"deferred_constraint", "commit_hook", "denied_rollback", "denied_rollback_pool"} {
				t.Run(mode, func(t *testing.T) { testFailedCommitCleanup(t, keyed, mode) })
			}
		})
	}
}

func testFailedCommitCleanup(t *testing.T, keyed bool, mode string) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "commit.db")
	dsn := path + "?_foreign_keys=on&_journal_mode=WAL"
	if keyed {
		dsn = compatibilityDSN(path, compatibilityKey) + "&_foreign_keys=on"
	}
	db, err := sql.Open(Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`CREATE TABLE parent(id INTEGER PRIMARY KEY);
CREATE TABLE child(id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED);
INSERT INTO parent VALUES(1)`); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var original *sqlite3.SQLiteConn
	var rawStmt driver.Stmt
	if err := conn.Raw(func(raw any) error { original = raw.(*sqlite3.SQLiteConn); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := conn.Raw(func(raw any) error {
		var err error
		rawStmt, err = raw.(*sqlite3.SQLiteConn).Prepare("SELECT count(*) FROM child")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rawStmt.Close() })
	stmt, err := conn.PrepareContext(ctx, "SELECT count(*) FROM child")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stmt.Close() })
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	parent := 2
	want := sqlite3.ErrConstraintForeignKey
	if mode == "commit_hook" {
		parent = 1
		want = sqlite3.ErrConstraintCommitHook
	}
	if _, err := tx.Exec("INSERT INTO child VALUES(10, ?)", parent); err != nil {
		t.Fatal(err)
	}
	var rows *sql.Rows
	if mode == "denied_rollback" {
		rows, err = conn.QueryContext(ctx, "SELECT id FROM child")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = rows.Close() })
	}
	if err := conn.Raw(func(raw any) error {
		c := raw.(*sqlite3.SQLiteConn)
		if mode == "commit_hook" {
			c.RegisterCommitHook(func() int { return 1 })
		}
		if mode == "denied_rollback" || mode == "denied_rollback_pool" {
			c.RegisterAuthorizer(func(op int, arg, _, _ string) int {
				if op == sqlite3.SQLITE_TRANSACTION && arg == "ROLLBACK" {
					return sqlite3.SQLITE_DENY
				}
				return sqlite3.SQLITE_OK
			})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	commitErr := tx.Commit()
	var sqlErr sqlite3.Error
	if !errors.As(commitErr, &sqlErr) || sqlErr.ExtendedCode != want {
		t.Fatalf("commit error = %v, want extended code %v", commitErr, want)
	}
	if err := tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("finished Tx rollback = %v", err)
	}
	switch mode {
	case "denied_rollback":
		// Check the still-pinned physical connection before ErrBadConn closes it.
		if err := conn.Raw(func(raw any) error {
			c := raw.(*sqlite3.SQLiteConn)
			if _, err := c.Prepare("SELECT 1"); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("poisoned prepare: %v", err)
			}
			if _, err := c.Exec("INSERT INTO parent VALUES(99)", nil); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("poisoned exec: %v", err)
			}
			if _, err := c.Query("SELECT * FROM child", nil); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("poisoned query: %v", err)
			}
			if _, err := rawStmt.(driver.StmtExecContext).ExecContext(ctx, nil); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("poisoned prepared exec: %v", err)
			}
			if _, err := rawStmt.(driver.StmtQueryContext).QueryContext(ctx, nil); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("poisoned prepared query: %v", err)
			}
			return rawStmt.Close()
		}); err != nil {
			t.Fatal(err)
		}
		if rows.Next() || !errors.Is(rows.Err(), driver.ErrBadConn) {
			t.Fatalf("poisoned cursor: %v", rows.Err())
		}
		// Prepared statements bypass fresh connection preparation.
		if _, err := stmt.ExecContext(ctx); !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
			t.Fatalf("poisoned statement exec: %v", err)
		}
		if _, err := stmt.QueryContext(ctx); !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
			t.Fatalf("poisoned statement query: %v", err)
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO parent VALUES(99)"); !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
			t.Fatalf("poisoned connection exec: %v", err)
		}
	case "denied_rollback_pool":
		if err := rawStmt.Close(); err != nil {
			t.Fatal(err)
		}
	default:
		if err := rawStmt.Close(); err != nil {
			t.Fatal(err)
		}
		if err := conn.Raw(func(raw any) error {
			c := raw.(*sqlite3.SQLiteConn)
			if c != original || !c.AutoCommit() {
				t.Error("failed commit left transaction active or replaced healthy connection")
			}
			c.RegisterCommitHook(nil)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := stmt.QueryRowContext(ctx).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failed rows = %d, error = %v", count, err)
		}
	}
	_ = stmt.Close()
	if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
		t.Fatal(err)
	}
	next, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = next.Close() })
	if err := next.Raw(func(raw any) error {
		c := raw.(*sqlite3.SQLiteConn)
		if (c == original) == (mode == "denied_rollback" || mode == "denied_rollback_pool") {
			t.Error("pool did not retain healthy or discard poisoned connection")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := next.QueryRowContext(ctx, "SELECT count(*) FROM child").Scan(&count); err != nil || count != 0 {
		t.Fatalf("pool leaked rows: %d, %v", count, err)
	}
	valid, err := next.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = valid.Rollback() })
	if _, err := valid.Exec("INSERT INTO child VALUES(20,1)"); err != nil {
		t.Fatal(err)
	}
	if err := valid.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open(Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var id int
	if err := reopened.QueryRow("SELECT id FROM child").Scan(&id); err != nil || id != 20 {
		t.Fatalf("durable child = %d, %v", id, err)
	}
	if err := reopened.QueryRow("SELECT count(*) FROM child").Scan(&count); err != nil || count != 1 {
		t.Fatalf("durable count = %d, %v", count, err)
	}
}

func TestManualTransactionRemainsUsable(t *testing.T) {
	db, err := sql.Open(Name, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	for _, query := range []string{"CREATE TABLE sample(id)", "BEGIN", "INSERT INTO sample VALUES(1)", "COMMIT"} {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sample").Scan(&count); err != nil || count != 1 {
		t.Fatalf("manual transaction: %d, %v", count, err)
	}
}

func TestBusyCommitRollsBack(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "busy.db") + "?_journal_mode=DELETE&_busy_timeout=0"
	writer, err := sql.Open(Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := sql.Open(Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	if _, err := writer.Exec("CREATE TABLE sample(id); INSERT INTO sample VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	rtx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rtx.Rollback() })
	var count int
	if err := rtx.QueryRow("SELECT count(*) FROM sample").Scan(&count); err != nil {
		t.Fatal(err)
	}
	// The held reader snapshot prevents the writer from obtaining the exclusive
	// DELETE-journal commit lock. No sleeps or scheduling assumptions are needed.
	wtx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wtx.Rollback() })
	if _, err := wtx.Exec("INSERT INTO sample VALUES(2)"); err != nil {
		t.Fatal(err)
	}
	var sqlErr sqlite3.Error
	if err := wtx.Commit(); !errors.As(err, &sqlErr) || sqlErr.Code != sqlite3.ErrBusy {
		t.Fatalf("commit = %v, want busy", err)
	}
	if err := rtx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := writer.QueryRow("SELECT count(*) FROM sample").Scan(&count); err != nil || count != 1 {
		t.Fatalf("busy commit leaked rows: %d, %v", count, err)
	}
	if _, err := writer.Exec("INSERT INTO sample VALUES(3)"); err != nil {
		t.Fatal(err)
	}
}

func TestDiscardPreservesRollbackHookUntilFinalStatement(t *testing.T) {
	for _, keyed := range []bool{false, true} {
		for _, retained := range []int{0, 2} {
			t.Run(fmt.Sprintf("keyed=%v/statements=%d", keyed, retained), func(t *testing.T) {
				dsn := filepath.Join(t.TempDir(), "hook.db") + "?_foreign_keys=on"
				if keyed {
					dsn = compatibilityDSN(filepath.Join(t.TempDir(), "hook.db"), compatibilityKey) + "&_foreign_keys=on"
				}
				db, err := sql.Open(Name, dsn)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				db.SetMaxOpenConns(1)
				if _, err := db.Exec(`CREATE TABLE p(id PRIMARY KEY); CREATE TABLE c(id REFERENCES p(id) DEFERRABLE INITIALLY DEFERRED)`); err != nil {
					t.Fatal(err)
				}
				conn, err := db.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = conn.Close() })
				calls := 0
				err = conn.Raw(func(raw any) error {
					c := raw.(*sqlite3.SQLiteConn)
					c.RegisterRollbackHook(func() { calls++ })
					stmts := make([]driver.Stmt, 0, retained)
					for i := 0; i < retained; i++ {
						s, err := c.Prepare("SELECT 1")
						if err != nil {
							return err
						}
						stmts = append(stmts, s)
					}
					defer func() {
						for _, s := range stmts {
							_ = s.Close()
						}
					}()
					tx, err := c.Begin()
					if err != nil {
						return err
					}
					if _, err := c.Exec("INSERT INTO c VALUES(7)", nil); err != nil {
						return err
					}
					c.RegisterAuthorizer(func(op int, arg, _, _ string) int {
						if op == sqlite3.SQLITE_TRANSACTION && arg == "ROLLBACK" {
							return sqlite3.SQLITE_DENY
						}
						return sqlite3.SQLITE_OK
					})
					var sqlErr sqlite3.Error
					if err := tx.Commit(); !errors.As(err, &sqlErr) || sqlErr.ExtendedCode != sqlite3.ErrConstraintForeignKey {
						t.Errorf("commit: %v", err)
					}
					if err := c.Close(); err != nil {
						return err
					}
					if retained == 2 {
						if calls != 0 {
							t.Errorf("rollback hook before native destruction: %d", calls)
						}
						if err := stmts[0].Close(); err != nil {
							return err
						}
						if calls != 0 {
							t.Errorf("rollback hook before last statement: %d", calls)
						}
						if err := stmts[1].Close(); err != nil {
							return err
						}
						if err := stmts[1].Close(); err != nil {
							return err
						}
					}
					if calls != 1 {
						t.Errorf("rollback hook calls=%d, want1", calls)
					}
					return driver.ErrBadConn
				})
				if !errors.Is(err, driver.ErrBadConn) {
					t.Fatal(err)
				}
				if _, err := db.Exec("INSERT INTO p VALUES(1)"); err != nil {
					t.Fatal(err)
				}
				var count int
				if err := db.QueryRow("SELECT count(*) FROM c").Scan(&count); err != nil || count != 0 {
					t.Fatalf("discarded rows=%d, %v", count, err)
				}
				if calls != 1 {
					t.Fatalf("double close repeated rollback hook: %d", calls)
				}
			})
		}
	}
}

func TestFinalizeAfterConnectionCloseReleasesStatement(t *testing.T) {
	raw, err := (&sqlite3.SQLiteDriver{}).Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	c := raw.(*sqlite3.SQLiteConn)
	t.Cleanup(func() { _ = c.Close() })
	s, err := c.Prepare("SELECT abs(-9223372036854775808)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	rows, err := s.(driver.StmtQueryContext).QueryContext(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var sqlErr sqlite3.Error
	if err := rows.Next(make([]driver.Value, 1)); !errors.As(err, &sqlErr) || sqlErr.Code != sqlite3.ErrError {
		t.Fatalf("overflow: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	// Rows.Next resets an errored statement; finalization must still release it
	// after close_v2, rather than refusing cleanup because the Go handle is nil.
	if err := s.Close(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
