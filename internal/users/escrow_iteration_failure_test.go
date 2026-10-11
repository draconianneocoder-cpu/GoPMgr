// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/crypto"
)

var errEscrowIteration = errors.New("injected escrow enumeration failure")

// Only the selected enumeration uses the synthetic connection. All writes
// and other reads still use the store's real BEGIN IMMEDIATE transaction.
type escrowIterationWriter struct {
	accountWriter
	query  string
	db     *sql.DB
	rows   *escrowIterationRows
	result *sql.Rows
}

func (w *escrowIterationWriter) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if strings.Join(strings.Fields(query), " ") != w.query {
		return w.accountWriter.QueryContext(ctx, query, args...)
	}
	if w.rows != nil {
		return nil, errors.New("escrow enumeration unexpectedly repeated")
	}
	realRows, err := w.accountWriter.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	columns, values, err := readEscrowIterationRows(realRows)
	if err != nil {
		return nil, err
	}
	if len(values) < 2 {
		return nil, errors.New("iteration fixture needs at least two real rows")
	}
	w.rows = &escrowIterationRows{columns: columns, first: values[0]}
	w.db = sql.OpenDB(&escrowIterationConnector{rows: w.rows})
	w.result, err = w.db.QueryContext(ctx, query)
	return w.result, err
}

type escrowIterationConnector struct{ rows *escrowIterationRows }

func (c *escrowIterationConnector) Connect(context.Context) (driver.Conn, error) {
	return &escrowIterationConn{rows: c.rows}, nil
}

func (c *escrowIterationConnector) Driver() driver.Driver { return escrowIterationDriver{} }

type escrowIterationDriver struct{}

func (escrowIterationDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the escrow iteration connector")
}

type escrowIterationConn struct{ rows *escrowIterationRows }

func (c *escrowIterationConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("escrow iteration fixture does not prepare statements")
}

func (c *escrowIterationConn) Begin() (driver.Tx, error) {
	return nil, errors.New("escrow iteration fixture does not own the transaction")
}

func (c *escrowIterationConn) Close() error { return nil }

func (c *escrowIterationConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return c.rows, nil
}

type escrowIterationRows struct {
	columns   []string
	first     []driver.Value
	delivered bool
	failed    bool
	closed    bool
}

func (r *escrowIterationRows) Columns() []string { return r.columns }

func (r *escrowIterationRows) Close() error {
	r.closed = true
	return nil // A successful Close must not hide the earlier Next error.
}

func (r *escrowIterationRows) Next(dest []driver.Value) error {
	if !r.delivered {
		copy(dest, r.first)
		r.delivered = true
		return nil
	}
	r.failed = true
	return errEscrowIteration
}

func readEscrowIterationRows(rows *sql.Rows) ([]string, [][]driver.Value, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, errors.Join(err, rows.Close())
	}
	var values [][]driver.Value
	for rows.Next() {
		raw := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, errors.Join(err, rows.Close())
		}
		row := make([]driver.Value, len(columns))
		for i := range raw {
			row[i] = raw[i]
		}
		values = append(values, row)
	}
	return columns, values, errors.Join(rows.Err(), rows.Close())
}

// Compare full persisted rows, including encrypted blobs, rather than counts
// that could miss a replacement key, grant, or attestation.
func escrowIterationState(t *testing.T, store *Store) map[string][][]driver.Value {
	t.Helper()
	queries := map[string]string{
		"keys":          `SELECT * FROM escrow_keys ORDER BY id`,
		"grants":        `SELECT * FROM escrow_grants ORDER BY admin_username`,
		"recovery":      `SELECT * FROM escrow_recovery ORDER BY escrow_key_id`,
		"sealed DEKs":   `SELECT * FROM sealed_deks ORDER BY username`,
		"pins":          `SELECT * FROM escrow_pins ORDER BY username`,
		"personal keys": `SELECT * FROM personal_keys ORDER BY username`,
		"pending":       `SELECT * FROM escrow_rotation_pending ORDER BY id`,
		"events":        `SELECT * FROM account_events ORDER BY id`,
	}
	state := make(map[string][][]driver.Value, len(queries))
	for name, query := range queries {
		rows, err := store.conn.Query(query)
		if err != nil {
			t.Fatalf("snapshot %s: %v", name, err)
		}
		_, state[name], err = readEscrowIterationRows(rows)
		if err != nil {
			t.Fatalf("snapshot %s: %v", name, err)
		}
	}
	return state
}

func TestEscrowEnumerationFailureRollsBack(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
	}{
		{"reseal", `SELECT username, sealed FROM sealed_deks WHERE escrow_key_id = ?`},
		{"reattest", `SELECT username, public_key, attestation FROM personal_keys WHERE attested_escrow_key_id = ?`},
		{"new_key", `SELECT username FROM users WHERE username <> ? ORDER BY username`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, dek := escrowStore(t)
			old := openEscrow(t, store, "alice", dek)
			addUser(t, store, "bob", old)
			addUser(t, store, "carol", old)
			mustCreateRecoveryCode(t, store, "alice", dek)
			mustExec(t, store, `INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, 'fixture', '2026-10-10T00:00:00Z')`)
			before := escrowIterationState(t, store)
			oldID := activeKeyID(t, store)

			id, err := crypto.NewEscrowKeyID()
			if err != nil {
				t.Fatal(err)
			}
			private, public, err := crypto.GenerateEscrowKeyPair()
			if err != nil {
				t.Fatal(err)
			}
			next := &EscrowSession{id: id, public: public, private: private}
			t.Cleanup(next.Close)
			fault := &escrowIterationWriter{query: tc.query}
			t.Cleanup(func() {
				if fault.db != nil {
					if err := fault.db.Close(); err != nil {
						t.Errorf("close iteration fixture: %v", err)
					}
				}
			})
			err = store.inWriteTx("escrow iteration regression", func(ctx context.Context, q accountWriter) error {
				fault.accountWriter = q
				if tc.name == "new_key" {
					_, err := newEscrowKeyTx(ctx, fault, "alice", dek, "iteration regression")
					return err
				}
				// The rotation helpers run after retirement and insertion. Keep
				// those real writes inside the transaction being challenged.
				if _, err := q.ExecContext(ctx, `UPDATE escrow_keys SET retired_at = ? WHERE id = ?`, nowStamp(), old.id); err != nil {
					return err
				}
				if _, err := q.ExecContext(ctx, `INSERT INTO escrow_keys (id, kem_id, public_key, created_at) VALUES (?, ?, ?, ?)`, next.id, crypto.EscrowKEMID, next.public, nowStamp()); err != nil {
					return err
				}
				var result RotationResult
				if tc.name == "reseal" {
					return resealTx(ctx, fault, old, next, &result)
				}
				if err := resealTx(ctx, fault, old, next, &result); err != nil {
					return err
				}
				return reattestTx(ctx, fault, old, next)
			})
			if fault.rows == nil || !fault.rows.delivered || !fault.rows.failed || !fault.rows.closed {
				t.Fatal("fixture did not deliver a row, fail iteration, and close")
			}
			if !errors.Is(err, errEscrowIteration) {
				t.Errorf("enumeration error = %v, want the iteration sentinel", err)
			}
			if got := activeKeyID(t, store); got != oldID {
				t.Error("iteration failure changed the active escrow key")
			}
			after := escrowIterationState(t, store)
			for name, want := range before {
				if !reflect.DeepEqual(after[name], want) {
					t.Errorf("iteration failure changed persisted %s", name)
				}
			}
		})
	}
}

func TestEscrowEnumerationFailureInSavepointKeepsTrigger(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
	}{
		{"reseal", `SELECT username, sealed FROM sealed_deks WHERE escrow_key_id = ?`},
		{"reattest", `SELECT username, public_key, attestation FROM personal_keys WHERE attested_escrow_key_id = ?`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, dek := escrowStore(t)
			old := openEscrow(t, store, "alice", dek)
			bobDEK := addUser(t, store, "bob", old)
			addUser(t, store, "carol", old)
			mustCreateRecoveryCode(t, store, "alice", dek)
			mustExec(t, store, `INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, 'fixture', '2026-10-10T00:00:00Z')`)
			before := escrowIterationState(t, store)
			reason := "automatic rotation iteration failure: " + tc.name
			const detail = "iteration savepoint trigger"
			fault := &escrowIterationWriter{query: tc.query}
			t.Cleanup(func() {
				if fault.db != nil {
					if err := fault.db.Close(); err != nil {
						t.Errorf("close iteration fixture: %v", err)
					}
				}
			})
			var next *EscrowSession
			var result RotationResult
			err := store.inWriteTx("automatic escrow iteration regression", func(ctx context.Context, q accountWriter) error {
				// This event belongs to the triggering action, outside the
				// rotation savepoint but inside the same real transaction.
				if err := recordAccountEvent(ctx, q, "alice", "alice", AccountTakeoverPeriodChanged, detail); err != nil {
					return err
				}
				fault.accountWriter = q
				var err error
				next, result, err = rotateInSavepointTx(ctx, fault, old, "alice", dek, nil, reason)
				next.Close()
				return err
			})
			if err != nil {
				t.Fatalf("automatic rotation should commit the triggering action: %v", err)
			}
			if next != nil || !reflect.DeepEqual(result, RotationResult{}) {
				t.Error("failed automatic rotation returned a replacement session or result")
			}
			if fault.rows == nil || !fault.rows.delivered || !fault.rows.failed || !fault.rows.closed || fault.result == nil {
				t.Fatal("fixture did not deliver a row, fail iteration, and close")
			}
			if !errors.Is(fault.result.Err(), errEscrowIteration) {
				t.Errorf("rows.Err() = %v, want the iteration sentinel", fault.result.Err())
			}
			if err := fault.result.Close(); err != nil {
				t.Errorf("rows.Close() = %v, want nil despite the iteration failure", err)
			}

			after := escrowIterationState(t, store)
			for name, want := range before {
				if name == "pending" || name == "events" {
					continue
				}
				if !reflect.DeepEqual(after[name], want) {
					t.Errorf("savepoint rollback changed persisted %s", name)
				}
			}
			if len(after["events"]) != len(before["events"])+1 {
				t.Fatal("automatic rotation did not preserve exactly one triggering event")
			}
			if !reflect.DeepEqual(after["events"][:len(before["events"])], before["events"]) {
				t.Error("savepoint rollback changed the pre-existing account history")
			}
			events, err := store.AccountEvents()
			if err != nil {
				t.Fatal(err)
			}
			trigger := events[0]
			if trigger.Actor != "alice" || trigger.Username != "alice" || trigger.Action != AccountTakeoverPeriodChanged || trigger.Detail != detail || trigger.OccurredAt.IsZero() {
				t.Error("committed event is not the triggering action")
			}
			if len(after["pending"]) != 1 {
				t.Fatal("failed automatic rotation did not leave exactly one pending marker")
			}
			var gotReason, markedAt string
			if err := store.conn.QueryRow(`SELECT reason, marked_at FROM escrow_rotation_pending WHERE id = 1`).Scan(&gotReason, &markedAt); err != nil {
				t.Fatal(err)
			}
			if gotReason != reason || markedAt == "" || markedAt == "2026-10-10T00:00:00Z" {
				t.Error("failed automatic rotation did not refresh the pending reason and timestamp")
			}

			reopened := openEscrow(t, store, "alice", dek)
			if reopened.id != old.id || !bytes.Equal(reopened.private, old.private) {
				t.Fatal("old administrator grant no longer opens the original key")
			}
			openedDEK, err := openSealedDEK(store, reopened, "bob")
			matches := bytes.Equal(openedDEK, bobDEK)
			clear(openedDEK)
			if err != nil || !matches {
				t.Fatal("original key no longer opens the account's sealed DEK")
			}
		})
	}
}
