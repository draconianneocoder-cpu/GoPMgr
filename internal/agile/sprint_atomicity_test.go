// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"

	"gopmgr/internal/db"
	"gopmgr/internal/sqlitedriver"
)

func TestSaveSprintFailureIsAtomic(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, update := range []bool{false, true} {
			for _, fault := range []string{"begin", "write", "readback", "commit"} {
				t.Run(fmt.Sprintf("encrypted=%t/update=%t/%s", encrypted, update, fault), func(t *testing.T) {
					testSprintFailureIsAtomic(t, encrypted, update, fault)
				})
			}
		}
	}
}

func testSprintFailureIsAtomic(t *testing.T, encrypted, update bool, fault string) {
	path := filepath.Join(t.TempDir(), "sprint-atomic.gopmgr")
	key := bytes.Repeat([]byte{0x42}, 32) // Disposable fixture key, not account material.
	open := func() *db.Database {
		t.Helper()
		var d *db.Database
		var err error
		if encrypted {
			d, err = db.InitEncryptedDB(path, key)
		} else {
			d, err = db.InitDB(path)
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		d.Conn.SetMaxOpenConns(1)
		d.Conn.SetMaxIdleConns(1)
		return d
	}
	d := open()
	project, err := d.UpsertProject(db.Project{ID: "sprint-atomic-project", Name: "Disposable sprint atomicity"})
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(d.Conn, project.ID)
	draft := Sprint{
		ProjectID: "caller-cannot-override", Name: "Changed sprint", Goal: "Exact goal",
		Status: SprintComplete, StartDate: "2026-10-18", EndDate: "2026-11-01", Capacity: 21,
	}
	historical := time.Date(2001, 2, 3, 4, 5, 6, 123000000, time.UTC)
	if update {
		original, err := store.SaveSprint(Sprint{Name: "Original sprint", Goal: "Original goal", Capacity: 3})
		if err != nil {
			t.Fatal(err)
		}
		draft.ID = original.ID
		// DB-generated timestamps have millisecond precision. A fixed old value
		// exposes accidental rewrites even when both saves run in one millisecond.
		if _, err := d.Conn.Exec(`UPDATE agile_sprints SET created_at = ? WHERE id = ? AND project_id = ?`,
			historical.Format(time.RFC3339Nano), original.ID, project.ID); err != nil {
			t.Fatal(err)
		}
	}
	before, err := store.ListSprints()
	if err != nil {
		t.Fatal(err)
	}
	if update && (len(before) != 1 || !before[0].CreatedAt.Equal(historical)) {
		t.Fatalf("historical fixture not established: %#v", before)
	}

	// Hook and save use the same physical connection. Release the sql.Conn
	// before SaveSprint so the one-connection pool remains usable.
	withRaw := func(fn func(*sqlite3.SQLiteConn)) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := d.Conn.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = conn.Raw(func(raw any) error { fn(raw.(*sqlite3.SQLiteConn)); return nil })
		if err := errors.Join(err, conn.Close()); err != nil {
			t.Fatal(err)
		}
	}
	hits := 0
	withRaw(func(c *sqlite3.SQLiteConn) {
		if fault == "commit" {
			c.RegisterCommitHook(func() int { hits++; return 1 })
			return
		}
		c.RegisterAuthorizer(func(op int, arg, _, _ string) int {
			if (fault == "begin" && op == sqlite3.SQLITE_TRANSACTION && arg == "BEGIN") ||
				(fault == "write" && op == sqlite3.SQLITE_INSERT && arg == "agile_sprints") ||
				(fault == "readback" && op == sqlite3.SQLITE_SELECT) {
				hits++
				return sqlite3.SQLITE_DENY
			}
			return sqlite3.SQLITE_OK
		})
	})
	got, saveErr := store.SaveSprint(draft)
	withRaw(func(c *sqlite3.SQLiteConn) {
		c.RegisterAuthorizer(nil)
		c.RegisterCommitHook(nil)
		if !c.AutoCommit() {
			t.Error("failed save left an active transaction")
		}
	})
	var sqlErr sqlite3.Error
	wantCode := sqlite3.ErrAuth
	if fault == "commit" {
		wantCode = sqlite3.ErrConstraint
	}
	if hits != 1 || !errors.As(saveErr, &sqlErr) || sqlErr.Code != wantCode {
		t.Errorf("fault %s: hits=%d error=%v, want one injected %v", fault, hits, saveErr, wantCode)
	}
	if fault == "commit" && sqlErr.ExtendedCode != sqlite3.ErrConstraintCommitHook {
		t.Errorf("commit extended code = %v, want commit-hook rejection", sqlErr.ExtendedCode)
	}
	if got != (Sprint{}) {
		t.Errorf("failed save returned a record: %#v", got)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	// Check the actual file without migrations before retrying the operation.
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?mode=ro&_query_only=1"
	if encrypted {
		dsn += fmt.Sprintf("&_pragma_key=x'%x'", key)
	}
	ro, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	after, err := NewStore(ro, project.ID).ListSprints()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("failed save changed durable state:\n got %#v\nwant %#v", after, before)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}

	d = open()
	store = NewStore(d.Conn, project.ID)
	saved, err := store.SaveSprint(draft)
	if err != nil {
		t.Fatalf("explicit retry: %v", err)
	}
	if saved.ID == "" || (update && saved.ID != draft.ID) || saved.CreatedAt.IsZero() {
		t.Fatalf("retry identity/timestamp: %#v", saved)
	}
	if update && !saved.CreatedAt.Equal(historical) {
		t.Error("retry replaced historical creation timestamp")
	}
	want := draft
	want.ID, want.ProjectID, want.CreatedAt = saved.ID, project.ID, saved.CreatedAt
	if !reflect.DeepEqual(saved, want) {
		t.Errorf("retry fields:\n got %#v\nwant %#v", saved, want)
	}
	items, err := store.ListSprints()
	if err != nil || len(items) != 1 || !reflect.DeepEqual(items[0], saved) {
		t.Fatalf("retry must persist exactly its returned record: items=%#v error=%v", items, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	persisted, err := NewStore(reopened, project.ID).ListSprints()
	if err != nil || !reflect.DeepEqual(persisted, items) {
		t.Fatalf("committed retry did not survive reopen: items=%#v error=%v", persisted, err)
	}
}
