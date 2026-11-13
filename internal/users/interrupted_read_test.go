// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"strings"
	"testing"
	"time"
)

// failRowDuringRead replaces table with a view of the same rows in which
// reading column fails, with SQLite's "integer overflow", on the rows that
// satisfy failWhen and on no others. A query against the name then starts
// normally and fails while its rows are being read, which is how a damaged
// page or an I/O error appears to database/sql: Next returns false and only
// Rows.Err reports why.
func failRowDuringRead(t *testing.T, store *Store, table, column, failWhen string) {
	t.Helper()
	rows, err := store.conn.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		if name == column {
			name = `CASE WHEN abs(-9223372036854775807 - (` + failWhen + `)) THEN ` + name + ` END AS ` + name
		}
		columns = append(columns, name)
	}
	iterationErr := rows.Err()
	if err := rows.Close(); err != nil || iterationErr != nil {
		t.Fatalf("columns of %s: %v, %v", table, iterationErr, err)
	}
	if len(columns) == 0 {
		t.Fatalf("table %s has no columns", table)
	}
	mustExec(t, store, `ALTER TABLE `+table+` RENAME TO `+table+`_stored`)
	mustExec(t, store, `CREATE VIEW `+table+` AS SELECT `+strings.Join(columns, ", ")+` FROM `+table+`_stored ORDER BY rowid`)
}

func requireInterruptedRead(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s reported success after its read stopped part-way; the rows not read were treated as checked", what)
	}
	if !strings.Contains(err.Error(), "integer overflow") {
		t.Fatalf("%s error = %v, want the read failure", what, err)
	}
}

// The administrator session opens every sealed DEK to find those sealed to
// a planted key. If the list stops part-way, the rows not reached have not
// been checked, so the session must fail and not report a clean result.
func TestSealedDEKCheckRefusesAnInterruptedRead(t *testing.T) {
	t.Parallel()
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	addUser(t, store, "carol", nil)
	session := openEscrow(t, store, "alice", aliceDEK)
	check := func() error {
		return store.inWriteTx("check", func(ctx context.Context, q accountWriter) error {
			return checkSealedDEKsTx(ctx, q, session)
		})
	}
	if n := countRows(t, store, "sealed_deks", "escrow_key_id = ?", session.id); n != 3 {
		t.Fatalf("sealed DEKs before the check = %d, want alice's, bob's and carol's", n)
	}
	if err := check(); err != nil {
		t.Fatalf("check with readable rows: %v", err)
	}

	failRowDuringRead(t, store, "sealed_deks", "sealed", "username = 'carol'")

	requireInterruptedRead(t, "the sealed-DEK check", check())
}

// The same for the account keys an administrator attested: one whose
// attestation was not read has not been verified.
func TestAttestationCheckRefusesAnInterruptedRead(t *testing.T) {
	t.Parallel()
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	addUser(t, store, "bob", session)
	addUser(t, store, "carol", session)
	check := func() error {
		return store.inWriteTx("check", func(ctx context.Context, q accountWriter) error {
			return adminDutiesTx(ctx, q, session)
		})
	}
	if n := countRows(t, store, "personal_keys", "attested_escrow_key_id = ?", session.id); n < 2 {
		t.Fatalf("attested account keys before the check = %d, want bob's and carol's", n)
	}
	if err := check(); err != nil {
		t.Fatalf("check with readable rows: %v", err)
	}

	failRowDuringRead(t, store, "personal_keys", "attestation", "username = 'carol'")

	requireInterruptedRead(t, "the attestation check", check())
}

// A protection that was not read must not be reported as absent.
func TestActiveProtectionsRefuseAnInterruptedRead(t *testing.T) {
	t.Parallel()
	store, _, _, _ := takenOver(t, "vacation", "")
	list := func() (protections []Protection, err error) {
		err = store.inWriteTx("check", func(ctx context.Context, q accountWriter) error {
			protections, err = activeProtectionsTx(ctx, q, time.Now().UTC())
			return err
		})
		return protections, err
	}
	protections, err := list()
	if err != nil || len(protections) != 1 || protections[0].Username != "alice" {
		t.Fatalf("protections with readable rows = %+v, %v; want alice's", protections, err)
	}

	failRowDuringRead(t, store, "takeover_protection", "username", "username = 'alice'")

	protections, err = list()
	requireInterruptedRead(t, "listing protections", err)
	if len(protections) != 0 {
		t.Fatalf("an interrupted read returned %+v", protections)
	}
}
