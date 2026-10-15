// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/users"
)

// atGap runs change once, the first time an operation reaches the session
// gap named op (session_state.go): after it captured the session and
// before it uses or publishes it.
func atGap(t *testing.T, op string, change func()) {
	t.Helper()
	old := sessionGap
	ran := false
	sessionGap = func(at string) {
		if at == op && !ran {
			ran = true
			change()
		}
	}
	t.Cleanup(func() { sessionGap = old })
}

// aliceAndBob returns an app with alice, the first account (an
// administrator, signed in), and bob, a standard account she created.
func aliceAndBob(t *testing.T) *App {
	t.Helper()
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", escrowTestPassword, false); err != nil {
		t.Fatalf("CreateAccount(bob): %v", err)
	}
	return app
}

func signedIn(app *App) (string, users.Principal, []byte) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	name := ""
	if app.user != nil {
		name = app.user.Username
	}
	return name, app.principal, append([]byte(nil), app.dek...)
}

func registryIdentity(t *testing.T, conn *sql.DB, username string) (string, int64) {
	t.Helper()
	var id string
	var generation int64
	if err := conn.QueryRow(`SELECT id, generation FROM account_identity_registry WHERE username = ?`, username).Scan(&id, &generation); err != nil {
		t.Fatalf("read %s's identity: %v", username, err)
	}
	return id, generation
}

func signInOrFail(t *testing.T, app *App, username string) {
	t.Helper()
	if _, err := app.Login(username, escrowTestPassword); err != nil {
		t.Fatalf("Login(%s): %v", username, err)
	}
}

// A sign-out that finishes while a sign-in checks the password wins: the
// late sign-in publishes nothing.
func TestALateSignInDoesNotOvertakeASignOut(t *testing.T) {
	app := aliceAndBob(t)
	if err := app.Logout(); err != nil {
		t.Fatal(err)
	}
	atGap(t, "sign-in", func() { _ = app.Logout() })
	if _, err := app.Login("alice", escrowTestPassword); err == nil || !strings.Contains(err.Error(), "signing in was interrupted") {
		t.Fatalf("Login across a sign-out = %v, want it interrupted", err)
	}
	if name, principal, dek := signedIn(app); name != "" || principal != (users.Principal{}) || len(dek) != 0 {
		t.Fatalf("after the interrupted sign-in: user %q, principal %+v, %d key bytes; want nobody", name, principal, len(dek))
	}
}

// Another sign-in that finishes first wins too, and the session holds its
// account, identity, and key.
func TestALateSignInDoesNotOvertakeAnotherSignIn(t *testing.T) {
	app := aliceAndBob(t)
	conn := systemDB(t, app)
	if err := app.Logout(); err != nil {
		t.Fatal(err)
	}
	atGap(t, "sign-in", func() { signInOrFail(t, app, "bob") })
	if _, err := app.Login("alice", escrowTestPassword); err == nil || !strings.Contains(err.Error(), "signing in was interrupted") {
		t.Fatalf("Login across bob's sign-in = %v, want it interrupted", err)
	}
	name, principal, dek := signedIn(app)
	_, _, bobDEK, err := app.store.SignInHeadless("bob", escrowTestPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(bobDEK)
	id, _ := registryIdentity(t, conn, "bob")
	if name != "bob" || principal.ID() != id || !bytes.Equal(dek, bobDEK) {
		t.Fatalf("session = %q, principal %s, key matches bob's: %v; want bob's session intact", name, principal.ID(), bytes.Equal(dek, bobDEK))
	}
}

// The session holds the identity its sign-in or creation established, and
// nothing after sign-out.
func TestTheSessionHoldsTheSignedInIdentity(t *testing.T) {
	app := newAdminTestApp(t)
	conn := systemDB(t, app)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatal(err)
	}
	// The creation wrote the key wrap after inserting the row, so the
	// identity's generation is 2 by the time it is signed in.
	id, generation := registryIdentity(t, conn, "alice")
	if _, principal, _ := signedIn(app); principal.ID() != id || principal.Generation() != generation || generation != 2 || principal.Username() != "alice" {
		t.Fatalf("first account's principal %+v; registry %s at generation %d, want generation 2", principal, id, generation)
	}

	if _, err := app.CreateAccount("bob", "Bob", escrowTestPassword, false); err != nil {
		t.Fatal(err)
	}
	switchUser(t, app, "bob")
	id, generation = registryIdentity(t, conn, "bob")
	if _, principal, _ := signedIn(app); principal.ID() != id || principal.Generation() != generation {
		t.Fatalf("bob's principal %+v; registry %s at generation %d", principal, id, generation)
	}

	if err := app.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, principal, _ := signedIn(app); principal != (users.Principal{}) {
		t.Fatalf("principal after sign-out = %+v, want none", principal)
	}
}

// The first account is created even if the session changes while it is,
// but it is signed in only when it has not.
func TestTheFirstAccountIsNotSignedInAfterTheSessionChanged(t *testing.T) {
	app := newAdminTestApp(t)
	atGap(t, "first account", func() { _ = app.Logout() })
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err == nil || !strings.Contains(err.Error(), "your account was created") {
		t.Fatalf("CreateAccount across a sign-out = %v, want created but not signed in", err)
	}
	if name, _, dek := signedIn(app); name != "" || len(dek) != 0 {
		t.Fatalf("signed in as %q after the interrupted first sign-in", name)
	}
	signInOrFail(t, app, "alice")
}

// Administrator access that finishes opening after a sign-out is not kept:
// no key survives the session, though the access stays recorded.
func TestAdministratorAccessIsNotKeptAfterSignOut(t *testing.T) {
	app := aliceAndBob(t)
	conn := systemDB(t, app)
	atGap(t, "administrator access", func() { _ = app.Logout() })
	err := app.AdminOpenUserData("bob", "checking")
	if err == nil || !strings.Contains(err.Error(), "it was closed; the access is still recorded") {
		t.Fatalf("AdminOpenUserData across a sign-out = %v", err)
	}
	app.mu.RLock()
	access := app.access
	app.mu.RUnlock()
	if access != nil {
		t.Fatal("the access was kept after sign-out")
	}
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM account_events WHERE action = 'admin_access' AND username = 'bob'`); n != 1 {
		t.Fatalf("%d accesses recorded, want 1", n)
	}
	signInOrFail(t, app, "alice")
	if _, err := app.AdminListUserProjects(); err == nil || err.Error() != "no account is open for administrator access" {
		t.Fatalf("AdminListUserProjects in the next session = %v", err)
	}
}

// Access belongs to the session that opened it, even if a session change
// somehow left it in place.
func TestAdministratorAccessBelongsToItsSession(t *testing.T) {
	app := aliceAndBob(t)
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatal(err)
	}
	app.mu.Lock()
	app.epoch++ // a session change that did not stop the access
	app.mu.Unlock()
	if _, err := app.AdminListUserProjects(); err == nil || err.Error() != "no account is open for administrator access" {
		t.Fatalf("AdminListUserProjects with access from another session = %v", err)
	}
}

// The same holds for a project copy being viewed.
func TestAViewedProjectBelongsToItsSession(t *testing.T) {
	app := aliceAndBob(t)
	switchUser(t, app, "bob")
	acceptNoRecoveryCodes(t, app)
	file, err := app.CreateProject("Bob's plan", "")
	if err != nil {
		t.Fatal(err)
	}
	switchUser(t, app, "alice")
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AdminViewUserProject(file.Path); err != nil {
		t.Fatalf("AdminViewUserProject: %v", err)
	}
	if _, err := app.AdminViewSchedule(); err != nil {
		t.Fatalf("AdminViewSchedule: %v", err)
	}
	app.mu.Lock()
	app.epoch++
	app.mu.Unlock()
	if _, err := app.AdminViewSchedule(); err == nil || err.Error() != "no project is open for viewing" {
		t.Fatalf("AdminViewSchedule with a copy from another session = %v", err)
	}
}

// An escrow operation never pairs alice's name with bob's key. Before the
// key was pinned to the session, the store opened alice's grant with bob's
// key, found it unusable, and deleted it.
func TestAnEscrowOperationAcrossASessionSwitchLeavesTheGrant(t *testing.T) {
	app := aliceAndBob(t)
	conn := systemDB(t, app)
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM escrow_grants WHERE admin_username = 'alice'`); n != 1 {
		t.Fatalf("alice holds %d grants before, want 1", n)
	}
	atGap(t, "key", func() { switchUser(t, app, "bob") })
	if _, err := app.AdminReplaceEscrowKey(""); !errors.Is(err, errSessionChanged) {
		t.Fatalf("AdminReplaceEscrowKey across a switch = %v, want errSessionChanged", err)
	}
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM escrow_grants WHERE admin_username = 'alice'`); n != 1 {
		t.Fatal("alice's grant was removed")
	}
	if n := rowCount(t, conn, `SELECT COUNT(*) FROM account_events WHERE action = 'escrow_key_mismatch'`); n != 0 {
		t.Fatalf("%d key mismatches recorded", n)
	}
}

// Recovery codes are never stored for alice wrapping bob's key.
func TestRecoveryCodesAcrossASessionSwitchAreNotIssued(t *testing.T) {
	app := aliceAndBob(t)
	conn := systemDB(t, app)
	before := rowCount(t, conn, `SELECT COUNT(*) FROM recovery_codes WHERE username = 'alice'`)
	atGap(t, "key", func() { switchUser(t, app, "bob") })
	if _, err := app.IssueRecoveryCodes(); !errors.Is(err, errSessionChanged) {
		t.Fatalf("IssueRecoveryCodes across a switch = %v, want errSessionChanged", err)
	}
	if after := rowCount(t, conn, `SELECT COUNT(*) FROM recovery_codes WHERE username = 'alice'`); after != before {
		t.Fatalf("alice's codes changed from %d to %d", before, after)
	}
}

// A project in alice's folder is never encrypted with bob's key, which
// would lock alice out of it.
func TestEncryptingAcrossASessionSwitchLeavesTheProjectAlone(t *testing.T) {
	app := aliceAndBob(t)
	if _, err := app.IssueRecoveryCodes(); err != nil {
		t.Fatal(err)
	}
	path := createPlaintextProjectForMigration(t, app)
	atGap(t, "encrypt project", func() { switchUser(t, app, "bob") })
	if _, err := app.EncryptProjectAtRest(path); !errors.Is(err, errSessionChanged) {
		t.Fatalf("EncryptProjectAtRest across a switch = %v, want errSessionChanged", err)
	}
	if encrypted, err := db.IsEncryptedFile(path); err != nil || encrypted {
		t.Fatalf("alice's project encrypted = %v, %v; want still plaintext", encrypted, err)
	}
}

// A project in alice's folder is not opened in bob's session, and signing
// out leaves no project state behind.
func TestOpeningAcrossASessionSwitchOpensNothing(t *testing.T) {
	app := aliceAndBob(t)
	acceptNoRecoveryCodes(t, app)
	file, err := app.CreateProject("Plan", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.OpenProject(file.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CheckOpenProject(); err != nil {
		t.Fatal(err)
	}
	if err := app.Logout(); err != nil {
		t.Fatal(err)
	}
	app.mu.RLock()
	leftover := app.db != nil || app.dbPath != "" || app.adminSvc != nil || app.sigmaSvc != nil || app.openCheck != nil
	app.mu.RUnlock()
	if leftover {
		t.Fatal("sign-out left project state behind")
	}

	signInOrFail(t, app, "alice")
	atGap(t, "open project", func() { switchUser(t, app, "bob") })
	if _, err := app.OpenProject(file.Path); !errors.Is(err, errSessionChanged) {
		t.Fatalf("OpenProject across a switch = %v, want errSessionChanged", err)
	}
	app.mu.RLock()
	open := app.db != nil
	app.mu.RUnlock()
	if open {
		t.Fatal("alice's project was opened in bob's session")
	}
}

// A sign-in over another session drops that session's prepared codes.
func TestSigningInDropsThePreviousSessionsPreparedCodes(t *testing.T) {
	app := aliceAndBob(t)
	if _, err := app.PrepareRecoveryCodes(escrowTestPassword); err != nil {
		t.Fatal(err)
	}
	signInOrFail(t, app, "bob")
	signInOrFail(t, app, "alice")
	if err := app.ConfirmRecoveryCodes(); err == nil || err.Error() != "there are no new codes to save; create them again" {
		t.Fatalf("ConfirmRecoveryCodes in a later session = %v, want nothing to save", err)
	}
}

// Updates to the session made after slow work land only in the session
// that did the work.
func TestSessionUpdatesStayInTheirSession(t *testing.T) {
	app := aliceAndBob(t)
	atGap(t, "export directory", func() { switchUser(t, app, "bob") })
	app.rememberExportDirectory(t.TempDir())
	if u := app.CurrentUser(); u == nil || u.Username != "bob" || u.LastExportDirectory != "" {
		t.Fatalf("bob's session after alice's export = %+v", u)
	}

	conn := systemDB(t, app)
	if _, err := conn.Exec(`UPDATE users SET is_admin = 0 WHERE username = 'alice'`); err != nil {
		t.Fatal(err)
	}
	switchUser(t, app, "bob")
	atGap(t, "become admin", func() { switchUser(t, app, "alice") })
	if err := app.BecomeAdmin(); err != nil {
		t.Fatalf("BecomeAdmin: %v", err)
	}
	if u := app.CurrentUser(); u == nil || u.Username != "alice" || u.IsAdmin {
		t.Fatalf("alice's session after bob claimed the role = %+v, want alice, not an administrator", u)
	}
}

// A security backup is of the open project and is written next to the
// given path: never one account's project into another's folder.
func TestASecurityBackupAcrossASessionSwitchWritesNothing(t *testing.T) {
	app := aliceAndBob(t)
	switchUser(t, app, "bob")
	acceptNoRecoveryCodes(t, app)
	bobs, err := app.CreateProject("Bob's plan", "")
	if err != nil {
		t.Fatal(err)
	}
	switchUser(t, app, "alice")
	acceptNoRecoveryCodes(t, app)
	alices, err := app.CreateProject("Alice's plan", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.OpenProject(alices.Path); err != nil {
		t.Fatal(err)
	}
	atGap(t, "secure archive", func() {
		switchUser(t, app, "bob")
		if _, err := app.OpenProject(bobs.Path); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := app.SecureArchive(alices.Path); !errors.Is(err, errSessionChanged) {
		t.Fatalf("SecureArchive across a switch = %v, want errSessionChanged", err)
	}
	if found, _ := filepath.Glob(filepath.Join(filepath.Dir(alices.Path), "GoPMgr_Archive_*")); len(found) != 0 {
		t.Fatalf("a backup was written into alice's folder: %v", found)
	}
}

// A deletion across a switch deletes nothing and records nothing in either
// account's deletion log.
func TestADeletionAcrossASessionSwitchDeletesNothing(t *testing.T) {
	app := aliceAndBob(t)
	acceptNoRecoveryCodes(t, app)
	file, err := app.CreateProject("Plan", "")
	if err != nil {
		t.Fatal(err)
	}
	atGap(t, "delete project", func() { switchUser(t, app, "bob") })
	if err := app.DeleteProject(file.Path); !errors.Is(err, errSessionChanged) {
		t.Fatalf("DeleteProject across a switch = %v, want errSessionChanged", err)
	}
	if _, err := os.Stat(file.Path); err != nil {
		t.Fatalf("alice's project is gone: %v", err)
	}
	for _, name := range []string{"bob", "alice"} {
		if name == "alice" {
			switchUser(t, app, "alice")
		}
		if deletions, err := app.ListProjectDeletions(); err != nil || len(deletions) != 0 {
			t.Fatalf("%s's deletion log = %+v, %v; want nothing recorded", name, deletions, err)
		}
	}
}

// Codes prepared in one session are not kept for the next one.
func TestPreparedCodesAcrossASessionChangeAreNotKept(t *testing.T) {
	app := aliceAndBob(t)
	atGap(t, "prepare codes", func() { switchUser(t, app, "alice") })
	if _, err := app.PrepareRecoveryCodes(escrowTestPassword); !errors.Is(err, errSessionChanged) {
		t.Fatalf("PrepareRecoveryCodes across a new session = %v, want errSessionChanged", err)
	}
	if err := app.ConfirmRecoveryCodes(); err == nil || err.Error() != "there are no new codes to save; create them again" {
		t.Fatalf("ConfirmRecoveryCodes = %v, want nothing to save", err)
	}
}

// A project copy that finishes opening after the administrator signed in
// again, and opened the same account again, is not attached to the new
// access.
func TestAViewFromAnEarlierSessionIsNotAttached(t *testing.T) {
	app := aliceAndBob(t)
	switchUser(t, app, "bob")
	acceptNoRecoveryCodes(t, app)
	file, err := app.CreateProject("Bob's plan", "")
	if err != nil {
		t.Fatal(err)
	}
	switchUser(t, app, "alice")
	if err := app.AdminOpenUserData("bob", "first look"); err != nil {
		t.Fatal(err)
	}
	atGap(t, "view copy", func() {
		switchUser(t, app, "alice")
		if err := app.AdminOpenUserData("bob", "second look"); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := app.AdminViewUserProject(file.Path); err == nil || err.Error() != "administrator access has stopped" {
		t.Fatalf("AdminViewUserProject across a new session = %v", err)
	}
	if _, err := app.AdminViewSchedule(); err == nil || err.Error() != "no project is open for viewing" {
		t.Fatalf("the new access has a view attached: %v", err)
	}
}
