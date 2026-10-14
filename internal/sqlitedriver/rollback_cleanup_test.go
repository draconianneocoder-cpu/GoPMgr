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
	"reflect"
	"testing"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"
)

func TestRollbackCleanup(t *testing.T) {
	for _, keyed := range []bool{false, true} {
		for _, mode := range []string{"denied_pool", "denied_pinned", "success", "already_rolled_back"} {
			t.Run(fmt.Sprintf("keyed=%v/%s", keyed, mode), func(t *testing.T) {
				testRollbackCleanup(t, keyed, mode)
			})
		}
	}
}

func testRollbackCleanup(t *testing.T, keyed bool, mode string) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rollback.db")
	dsn := path
	if keyed {
		dsn = compatibilityDSN(path, compatibilityKey)
	}
	db, err := sql.Open(Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("CREATE TABLE sample(id INTEGER PRIMARY KEY); INSERT INTO sample VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if mode == "already_rolled_back" {
		if _, err := db.Exec(`CREATE TRIGGER reject BEFORE INSERT ON sample WHEN NEW.id=2 BEGIN SELECT RAISE(ROLLBACK,'reject'); END`); err != nil {
			t.Fatal(err)
		}
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var original *sqlite3.SQLiteConn
	var stmt driver.Stmt
	var cursor driver.Rows
	if err := conn.Raw(func(raw any) error {
		original = raw.(*sqlite3.SQLiteConn)
		if mode == "denied_pinned" {
			var err error
			stmt, err = original.Prepare("SELECT id FROM sample")
			if err != nil {
				return err
			}
			cursor, err = stmt.(driver.StmtQueryContext).QueryContext(ctx, nil)
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if stmt != nil {
		t.Cleanup(func() { _ = stmt.Close() })
	}
	if cursor != nil {
		t.Cleanup(func() { _ = cursor.Close() })
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	_, insertErr := tx.Exec("INSERT INTO sample VALUES(2)")
	var sqlErr sqlite3.Error
	if mode == "already_rolled_back" {
		if !errors.As(insertErr, &sqlErr) || sqlErr.ExtendedCode != sqlite3.ErrConstraintTrigger {
			t.Fatalf("automatic rollback insert = %v", insertErr)
		}
		if err := conn.Raw(func(raw any) error {
			if !raw.(*sqlite3.SQLiteConn).AutoCommit() {
				t.Error("trigger did not roll back physical transaction")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	} else if insertErr != nil {
		t.Fatal(insertErr)
	}
	denied := mode == "denied_pool" || mode == "denied_pinned"
	if denied {
		if err := conn.Raw(func(raw any) error {
			raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(func(op int, arg, _, _ string) int {
				if op == sqlite3.SQLITE_TRANSACTION && arg == "ROLLBACK" {
					return sqlite3.SQLITE_DENY
				}
				return sqlite3.SQLITE_OK
			})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	rollbackErr := tx.Rollback()
	switch {
	case denied:
		if !errors.As(rollbackErr, &sqlErr) || sqlErr.Code != sqlite3.ErrAuth {
			t.Fatalf("rollback = %v, want authorization error", rollbackErr)
		}
	case mode == "already_rolled_back":
		if !errors.As(rollbackErr, &sqlErr) || sqlErr.Code != sqlite3.ErrError {
			t.Fatalf("rollback = %v, want no-active-transaction error", rollbackErr)
		}
	default:
		if rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
	}
	if err := tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("finished transaction = %v", err)
	}
	if mode == "denied_pinned" {
		if err := conn.Raw(func(raw any) error {
			if raw.(*sqlite3.SQLiteConn).IsValid() {
				t.Error("failed rollback left pinned connection valid")
			}
			if _, err := stmt.(driver.StmtExecContext).ExecContext(ctx, nil); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("pinned statement exec = %v", err)
			}
			if _, err := stmt.(driver.StmtQueryContext).QueryContext(ctx, nil); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("pinned statement query = %v", err)
			}
			if err := cursor.Next(make([]driver.Value, 1)); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("pinned cursor = %v", err)
			}
			if err := cursor.Close(); err != nil {
				return err
			}
			return stmt.Close()
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sample").Scan(&count); !errors.Is(err, driver.ErrBadConn) {
			t.Errorf("pinned read = %d, %v; want refused before reading", count, err)
		}
	}
	if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
		t.Fatal(err)
	}
	next, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = next.Close() })
	// Inspect identity before querying. No cancellation, lifetime expiry or
	// explicit ErrBadConn callback can disguise failed pool eviction here.
	if err := next.Raw(func(raw any) error {
		c := raw.(*sqlite3.SQLiteConn)
		if (c == original) == denied {
			t.Error("pool retained rejected connection or discarded healthy connection")
		}
		if !c.IsValid() || !c.AutoCommit() {
			t.Error("next connection is not healthy and outside a transaction")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := next.QueryRowContext(ctx, "SELECT count(*) FROM sample").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed row leaked: count=%d, err=%v", count, err)
	}
	valid, err := next.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = valid.Rollback() })
	if _, err := valid.Exec("INSERT INTO sample VALUES(3)"); err != nil {
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
	rows, err := reopened.Query("SELECT id FROM sample ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rows.Close() })
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []int{1, 3}) {
		t.Fatalf("durable rows = %v, want [1 3]", ids)
	}
}
