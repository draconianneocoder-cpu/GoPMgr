// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"gopmgr/internal/sqlitedriver"
)

const escrowTestPassword = "passphrase-long"

// systemDB opens a second connection to the app's system.db, as another
// process (or someone editing the file) would.
func systemDB(t *testing.T, app *App) *sql.DB {
	t.Helper()
	conn, err := sql.Open(sqlitedriver.Name, filepath.Join(app.store.RootDir(), "system.db"))
	if err != nil {
		t.Fatalf("open system.db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func rowCount(t *testing.T, conn *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func switchUser(t *testing.T, app *App, username string) {
	t.Helper()
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := app.Login(username, escrowTestPassword); err != nil {
		t.Fatalf("Login(%s): %v", username, err)
	}
}

func TestAppEnrollsEveryAccountInEscrow(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	conn := systemDB(t, app)
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM escrow_keys`); n != 1 {
		t.Fatalf("%d escrow keys after the first account, want 1", n)
	}
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM escrow_grants WHERE admin_username = 'alice'`); n != 1 {
		t.Fatal("the first administrator holds no grant")
	}

	if _, err := app.CreateAccount("bob", "Bob", escrowTestPassword, false); err != nil {
		t.Fatalf("CreateAccount(bob): %v", err)
	}
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM personal_keys WHERE username = 'bob' AND attested_escrow_key_id <> ''`); n != 1 {
		t.Fatal("an account alice created was not attested with her escrow key")
	}

	// An account older than its DEK enrolls when the DEK is made at sign-in
	// (users.Store.UnlockDEK); one that already has a DEK, at sign-in itself.
	if _, err := app.store.CreateAccount("carol", "Carol", escrowTestPassword, false); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	for _, table := range []string{"personal_keys", "escrow_pins", "sealed_deks"} {
		if _, err := conn.Exec(`DELETE FROM ` + table + ` WHERE username = 'bob'`); err != nil {
			t.Fatalf("unenroll bob: %v", err)
		}
	}
	switchUser(t, app, "carol")
	switchUser(t, app, "bob")
	for _, username := range []string{"alice", "bob", "carol"} {
		if n := rowCount(t, conn, `SELECT COUNT(*) FROM sealed_deks WHERE username = ?`, username); n != 1 {
			t.Errorf("%s's DEK is not sealed", username)
		}
	}
}

func TestAppPromotionAndStatusChangesManageTheGrant(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	conn := systemDB(t, app)
	grants := func() int {
		return rowCount(t, conn, `SELECT COUNT(*) FROM escrow_grants WHERE admin_username = 'carol'`)
	}
	if _, err := app.store.CreateAccount("carol", "Carol", escrowTestPassword, false); err != nil {
		t.Fatalf("create carol: %v", err)
	}

	want := "carol must sign in once before they can be made an administrator"
	if err := app.AdminSetUserRole("carol", true); err == nil || err.Error() != want {
		t.Fatalf("promoting an account that never signed in: err = %v, want %q", err, want)
	}
	switchUser(t, app, "carol")
	switchUser(t, app, "alice")
	if err := app.AdminSetUserRole("carol", true); err != nil || grants() != 1 {
		t.Fatalf("promote carol = %v with %d grants, want 1", err, grants())
	}
	if err := app.AdminSetUserDisabled("carol", true); err != nil || grants() != 0 {
		t.Fatalf("disable carol = %v with %d grants, want 0", err, grants())
	}
	if err := app.AdminSetUserDisabled("carol", false); err != nil || grants() != 1 {
		t.Fatalf("enable carol = %v with %d grants, want 1", err, grants())
	}
	if err := app.AdminSetUserRole("carol", false); err != nil || grants() != 0 {
		t.Fatalf("demote carol = %v with %d grants, want 0", err, grants())
	}

	// dave's attested personal key is replaced in system.db.
	if _, err := app.CreateAccount("dave", "Dave", escrowTestPassword, false); err != nil {
		t.Fatalf("CreateAccount(dave): %v", err)
	}
	if _, err := conn.Exec(`UPDATE personal_keys SET public_key = (SELECT public_key FROM personal_keys WHERE username = 'carol') WHERE username = 'dave'`); err != nil {
		t.Fatalf("swap dave's personal key: %v", err)
	}
	want = "dave was not made an administrator: their account key has been changed since an administrator last checked it. The change is recorded in the account history"
	if err := app.AdminSetUserRole("dave", true); err == nil || err.Error() != want {
		t.Fatalf("promoting a swapped key: err = %v, want %q", err, want)
	}
}

// is_admin is a plain column. Signing in, as the administrator or as the
// account itself, never grants the escrow key to an account made an
// administrator by editing system.db.
func TestAppSignInNeverGrantsTheEscrowKey(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", escrowTestPassword, false); err != nil {
		t.Fatalf("CreateAccount(bob): %v", err)
	}
	conn := systemDB(t, app)
	if _, err := conn.Exec(`UPDATE users SET is_admin = 1 WHERE username = 'bob'`); err != nil {
		t.Fatalf("make bob an administrator: %v", err)
	}
	switchUser(t, app, "alice")
	switchUser(t, app, "bob")
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM escrow_grants WHERE admin_username = 'bob'`); n != 0 {
		t.Fatal("bob was granted the escrow key by a sign-in")
	}
}
