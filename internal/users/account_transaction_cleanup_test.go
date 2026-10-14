// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"
)

func TestAccountTransactionCleanup(t *testing.T) {
	for _, mode := range []string{"audit_denied", "commit_denied", "commit_rollback_ok", "callback_error", "panic", "panic_rollback_ok", "rollback_ok", "automatic_rollback"} {
		t.Run(mode, func(t *testing.T) {
			store := newStatusStore(t)
			store.conn.SetMaxOpenConns(1)
			store.conn.SetMaxIdleConns(1)
			// Use real history rows so the audit comparison also checks preservation.
			if err := store.SetDisabled("alice", "bob", true); err != nil {
				t.Fatal(err)
			}
			if err := store.SetDisabled("alice", "bob", false); err != nil {
				t.Fatal(err)
			}
			baseline, err := store.AccountEvents()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "audit_denied" || mode == "rollback_ok" || mode == "automatic_rollback" {
				action := "ABORT"
				if mode == "automatic_rollback" {
					action = "ROLLBACK"
				}
				if _, err := store.conn.Exec(`CREATE TRIGGER reject_status BEFORE INSERT ON account_events WHEN NEW.action='disabled' BEGIN SELECT RAISE(` + action + `,'reject status audit'); END`); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			conn, err := store.conn.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			var original *sqlite3.SQLiteConn
			rollbackDenied := mode != "rollback_ok" && mode != "panic_rollback_ok" && mode != "automatic_rollback" && mode != "commit_rollback_ok"
			commitDenied := mode == "commit_denied" || mode == "commit_rollback_ok"
			denials := 0
			if err := conn.Raw(func(raw any) error {
				original = raw.(*sqlite3.SQLiteConn)
				original.RegisterAuthorizer(func(op int, arg, _, _ string) int {
					if op == sqlite3.SQLITE_TRANSACTION && ((arg == "ROLLBACK" && rollbackDenied) || (arg == "COMMIT" && commitDenied)) {
						denials++
						return sqlite3.SQLITE_DENY
					}
					return sqlite3.SQLITE_OK
				})
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("callback rejected")
			panicMode := mode == "panic" || mode == "panic_rollback_ok"
			var operationErr error
			if mode == "callback_error" || panicMode {
				func() {
					defer func() {
						if got := recover(); got != nil || panicMode {
							if !panicMode || got != failure {
								t.Fatalf("panic = %v, want original sentinel", got)
							}
						}
					}()
					operationErr = store.inWriteTx("cleanup regression", func(ctx context.Context, q accountWriter) error {
						if err := setDisabledTx(ctx, q, "alice", "bob", true); err != nil {
							return err
						}
						if panicMode {
							panic(failure)
						}
						return failure
					})
				}()
				if !panicMode && !errors.Is(operationErr, failure) {
					t.Fatalf("callback error lost: %v", operationErr)
				}
			} else {
				operationErr = store.SetDisabled("alice", "bob", true)
				var sqlErr sqlite3.Error
				want := sqlite3.ErrConstraint
				if commitDenied {
					want = sqlite3.ErrAuth
				}
				if !errors.As(operationErr, &sqlErr) || sqlErr.Code != want {
					t.Fatalf("operation error = %v, want %v", operationErr, want)
				}
			}
			if !panicMode && (rollbackDenied || mode == "automatic_rollback") {
				joined, ok := operationErr.(interface{ Unwrap() []error })
				if !ok || len(joined.Unwrap()) != 2 {
					t.Fatalf("operation and cleanup errors not both preserved: %v", operationErr)
				}
				if commitDenied {
					var commitErr sqlite3.Error
					first := joined.Unwrap()[0]
					if !strings.HasPrefix(first.Error(), "users: commit account status change:") || !errors.As(first, &commitErr) || commitErr.Code != sqlite3.ErrAuth {
						t.Fatalf("original COMMIT error lost: %v", first)
					}
				}
				var rollbackErr sqlite3.Error
				want := sqlite3.ErrAuth
				if mode == "automatic_rollback" {
					want = sqlite3.ErrError
				}
				if !errors.As(joined.Unwrap()[1], &rollbackErr) || rollbackErr.Code != want {
					t.Fatalf("cleanup error = %v, want %v", joined.Unwrap()[1], want)
				}
			}
			wantDenials := 0
			if rollbackDenied {
				wantDenials = 1
			}
			if commitDenied {
				wantDenials++
			}
			if denials != wantDenials {
				t.Fatalf("native denials = %d, want %d", denials, wantDenials)
			}
			next, err := store.conn.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = next.Close() })
			discarded := rollbackDenied || mode == "automatic_rollback"
			if err := next.Raw(func(raw any) error {
				c := raw.(*sqlite3.SQLiteConn)
				if (c == original) == discarded {
					t.Error("unsafe connection retained or healthy rollback discarded")
				}
				if !c.AutoCommit() {
					t.Error("active transaction leaked to next operation")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			var disabled int
			if err := next.QueryRowContext(ctx, `SELECT disabled FROM users WHERE username='bob'`).Scan(&disabled); err != nil || disabled != 0 {
				t.Errorf("rejected status leaked: disabled=%d err=%v", disabled, err)
			}
			if err := next.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := store.AccountEvents()
			if err != nil || !reflect.DeepEqual(got, baseline) {
				t.Errorf("rejected audit leaked: got=%v want=%v err=%v", got, baseline, err)
			}
			if t.Failed() {
				return // Do not let recovery or fixture teardown hide the failure.
			}
			if !discarded {
				// Remove the test-only COMMIT denial only after proving reuse and state.
				c, err := store.conn.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if err := c.Raw(func(raw any) error {
					raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(nil)
					return nil
				}); err != nil {
					_ = c.Close()
					t.Fatal(err)
				}
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.conn.Exec(`DROP TRIGGER IF EXISTS reject_status`); err != nil {
				t.Fatal(err)
			}
			if err := store.SetDisabled("alice", "bob", true); err != nil {
				t.Fatalf("subsequent valid write: %v", err)
			}
			persisted, err := store.AccountEvents()
			if err != nil || len(persisted) != len(baseline)+1 || persisted[0].Action != AccountDisabled || persisted[0].Username != "bob" || !reflect.DeepEqual(persisted[1:], baseline) {
				t.Fatalf("valid audit = %v, err=%v", persisted, err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(store.RootDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			got, err = reopened.AccountEvents()
			if err != nil || !reflect.DeepEqual(got, persisted) {
				t.Fatalf("reopened audit = %v, want=%v err=%v", got, persisted, err)
			}
			if err := reopened.conn.QueryRow(`SELECT disabled FROM users WHERE username='bob'`).Scan(&disabled); err != nil || disabled != 1 {
				t.Fatalf("reopened status = %d, err=%v", disabled, err)
			}
		})
	}
}
