// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/printing"
)

// superApp returns an app signed in as alice, the super administrator,
// with bob a subordinate administrator who has signed in, and carol a
// standard account.
func superApp(t *testing.T) *App {
	t.Helper()
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	for _, name := range []string{"bob", "carol"} {
		if _, err := app.CreateAccount(name, name, escrowTestPassword, false); err != nil {
			t.Fatalf("CreateAccount(%s): %v", name, err)
		}
	}
	if err := app.AdminSetUserRole("bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	return app
}

func TestAppSubordinatesManageStandardAccountsOnly(t *testing.T) {
	app := superApp(t)
	if roles, err := app.AdminRoles(); err != nil || roles.Super != "alice" {
		t.Fatalf("AdminRoles = %+v, %v; want alice", roles, err)
	}
	switchUser(t, app, "bob")

	wantNotSuper := "only the super administrator can change administrators or open users' data"
	for name, call := range map[string]func() error{
		"promote":           func() error { return app.AdminSetUserRole("carol", true) },
		"demote the super":  func() error { return app.AdminSetUserRole("alice", false) },
		"disable the super": func() error { return app.AdminSetUserDisabled("alice", true) },
		"delete the super":  func() error { return app.AdminPurgeUser("alice", "alice") },
	} {
		if err := call(); err == nil || err.Error() != wantNotSuper {
			t.Errorf("%s by a subordinate: err = %v, want %q", name, err, wantNotSuper)
		}
	}
	if _, err := app.CreateAccount("zed", "Zed", escrowTestPassword, true); err == nil || err.Error() != "only the super administrator can create administrator accounts" {
		t.Errorf("subordinate creating an administrator: err = %v", err)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil || err.Error() != "only the super administrator can open users' data" {
		t.Errorf("subordinate opening data: err = %v", err)
	}
	if _, err := app.AdminHandOverSuper("carol"); err == nil || err.Error() != "only the super administrator can hand over the role" {
		t.Errorf("subordinate handing over: err = %v", err)
	}
	if _, err := app.AdminIssueRecoveryCodes("alice", escrowTestPassword); err == nil || err.Error() != wantNotSuper {
		t.Errorf("subordinate issuing an administrator's codes: err = %v", err)
	}

	// Standard accounts are theirs to manage.
	if _, err := app.CreateAccount("dave", "Dave", escrowTestPassword, false); err != nil {
		t.Fatalf("subordinate creating a standard account: %v", err)
	}
	if err := app.AdminSetUserDisabled("carol", true); err != nil {
		t.Fatalf("subordinate disabling a standard account: %v", err)
	}
	if err := app.AdminSetUserDisabled("carol", false); err != nil {
		t.Fatalf("subordinate enabling a standard account: %v", err)
	}
}

func TestAppHandOverMovesTheRoleAndEndsOpenAccess(t *testing.T) {
	app := superApp(t)
	if err := app.AdminOpenUserData("carol", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	app.mu.RLock()
	key := app.access.dek
	app.mu.RUnlock()

	result, err := app.AdminHandOverSuper("bob")
	if err != nil || !result.KeyPassed {
		t.Fatalf("AdminHandOverSuper(bob) = %+v, %v; want the key passed", result, err)
	}
	app.mu.RLock()
	access := app.access
	app.mu.RUnlock()
	if access != nil || !isZero(key) {
		t.Fatal("the former super administrator's open access was not ended")
	}
	if _, err := app.AdminListUserProjects(); err == nil {
		t.Fatal("the former super administrator can still read the account they had open")
	}
	if roles, _ := app.AdminRoles(); roles.Super != "bob" {
		t.Fatalf("super administrator = %q, want bob", roles.Super)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil {
		t.Fatal("the former super administrator opened data")
	}

	switchUser(t, app, "bob")
	if err := app.AdminOpenUserData("carol", "checking"); err != nil {
		t.Fatalf("new super administrator opening data: %v", err)
	}
	want := "carol must be an administrator who can sign in"
	if _, err := app.AdminHandOverSuper("carol"); err == nil || err.Error() != want {
		t.Fatalf("hand-over to a standard account: err = %v, want %q", err, want)
	}
}

func isZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func TestAppStandbyAndTakeover(t *testing.T) {
	app := superApp(t)
	for _, tc := range []struct {
		standby string
		days    int
		want    string
	}{
		{"carol", 30, "carol must be an administrator who can sign in"},
		{"alice", 30, "you can't be your own standby"},
		{"bob", 6, "the takeover period must be 7 to 365 days"},
	} {
		if _, err := app.AdminSetStandby(tc.standby, tc.days); err == nil || err.Error() != tc.want {
			t.Errorf("AdminSetStandby(%s, %d): err = %v, want %q", tc.standby, tc.days, err, tc.want)
		}
	}
	result, err := app.AdminSetStandby("bob", 30)
	if err != nil || !result.KeyPassed {
		t.Fatalf("AdminSetStandby(bob) = %+v, %v; want the key passed", result, err)
	}
	roles, err := app.AdminRoles()
	if err != nil || roles.Super != "alice" || roles.Standby != "bob" || !roles.StandbyHoldsKey || roles.TakeoverDays != 30 ||
		roles.CanTakeOver || roles.Protections == nil || len(roles.Protections) != 0 {
		t.Fatalf("AdminRoles = %+v, %v; want alice super, bob standby with the key, 30 days, no protections", roles, err)
	}

	// Signing in as bob stamps a recent sign-in for him, not for alice.
	switchUser(t, app, "bob")
	if _, err := app.AdminSetStandby("bob", 30); err == nil || err.Error() != "only the super administrator can name a standby" {
		t.Errorf("the standby naming a standby: err = %v", err)
	}
	if _, err := app.AdminTakeOverSuper("vacation", ""); err == nil || err.Error() != "the super administrator has signed in within the takeover period, so the role can't be taken over" {
		t.Fatalf("early takeover: err = %v", err)
	}
	conn := systemDB(t, app)
	old := time.Now().UTC().Add(-31 * 24 * time.Hour).Format(time.RFC3339Nano)
	for _, q := range []string{`UPDATE users SET last_login = ? WHERE username = 'alice'`, `UPDATE super_admin SET assigned_at = ?`} {
		if _, err := conn.Exec(q, old); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if roles, err := app.AdminRoles(); err != nil || !roles.CanTakeOver || roles.SuperInactiveDays != 31 {
		t.Fatalf("AdminRoles = %+v, %v; want the takeover offered after 31 days", roles, err)
	}
	for _, tc := range []struct{ reason, note, want string }{
		{"", "", "choose a reason for the takeover"},
		{"other", "  ", "describe the reason in the note"},
		{"vacation", strings.Repeat("x", 501), "the note can be at most 500 characters"},
	} {
		if _, err := app.AdminTakeOverSuper(tc.reason, tc.note); err == nil || err.Error() != tc.want {
			t.Errorf("AdminTakeOverSuper(%q): err = %v, want %q", tc.reason, err, tc.want)
		}
	}
	took, err := app.AdminTakeOverSuper("vacation", "")
	if err != nil || !took.KeyHeld {
		t.Fatalf("AdminTakeOverSuper = %+v, %v; want the role with the key", took, err)
	}
	roles, _ = app.AdminRoles()
	if roles.Super != "bob" || roles.Standby != "" || roles.CanTakeOver ||
		len(roles.Protections) != 1 || roles.Protections[0].Username != "alice" || roles.Protections[0].Reason != "Vacation" {
		t.Fatalf("AdminRoles after the takeover = %+v; want bob super, no standby, alice protected", roles)
	}
	if len(roles.TakeoverReasons) != 5 || roles.TakeoverReasons[0] != (AdminTakeoverReasonWire{Code: "vacation", Label: "Vacation", Days: 30}) {
		t.Fatalf("takeover reasons = %+v", roles.TakeoverReasons)
	}
	until, err := time.Parse(time.RFC3339, roles.Protections[0].Until)
	if err != nil {
		t.Fatalf("protection end %q: %v", roles.Protections[0].Until, err)
	}
	wantProtected := "alice is protected until " + until.Local().Format("January 2, 2006") +
		" after the takeover (Vacation), so they can't be disabled, deleted, or removed as an administrator until then"
	for name, call := range map[string]func() error{
		"disable": func() error { return app.AdminSetUserDisabled("alice", true) },
		"demote":  func() error { return app.AdminSetUserRole("alice", false) },
		"delete":  func() error { return app.AdminPurgeUser("alice", "alice") },
	} {
		if err := call(); err == nil || err.Error() != wantProtected {
			t.Errorf("%s the protected former super administrator: err = %v, want %q", name, err, wantProtected)
		}
	}
	if _, err := app.AdminTakeOverSuper("vacation", ""); err == nil || err.Error() != "you are already the super administrator" {
		t.Fatalf("second takeover: err = %v", err)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err != nil {
		t.Fatalf("the new super administrator opening data: %v", err)
	}

	// The former super administrator is a subordinate, and only a standby
	// named by bob could take over from him.
	switchUser(t, app, "alice")
	if _, err := app.AdminTakeOverSuper("vacation", ""); err == nil || err.Error() != "the super administrator has signed in within the takeover period, so the role can't be taken over" {
		t.Fatalf("former super administrator taking back: err = %v", err)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil || err.Error() != "only the super administrator can open users' data" {
		t.Fatalf("former super administrator opening data: err = %v", err)
	}
}

func TestAppStandbyRolesNeedAnAdministrator(t *testing.T) {
	app := superApp(t)
	switchUser(t, app, "carol")
	for name, call := range map[string]func() error{
		"roles":    func() error { _, err := app.AdminRoles(); return err },
		"standby":  func() error { _, err := app.AdminSetStandby("bob", 30); return err },
		"takeover": func() error { _, err := app.AdminTakeOverSuper("vacation", ""); return err },
	} {
		if err := call(); err == nil || err.Error() != "administrator privileges required" {
			t.Errorf("%s by a standard account: err = %v", name, err)
		}
	}
}

func TestAppReplaceEscrowKeyAndPendingReplacement(t *testing.T) {
	app := superApp(t)
	conn := systemDB(t, app)
	retired := func() int {
		t.Helper()
		var n int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM escrow_keys WHERE retired_at <> ''`).Scan(&n); err != nil {
			t.Fatalf("count retired keys: %v", err)
		}
		return n
	}
	if _, err := app.AdminSetStandby("bob", 30); err != nil {
		t.Fatalf("AdminSetStandby(bob): %v", err)
	}

	if _, err := app.AdminReplaceEscrowKey("carol"); err == nil || err.Error() != "the standby has changed; reload the Admin panel and try again" {
		t.Fatalf("keeping someone who is not the standby: err = %v", err)
	}
	got, err := app.AdminReplaceEscrowKey("bob")
	if err != nil || got != (AdminRotationWire{Resealed: 3}) || retired() != 1 {
		t.Fatalf("AdminReplaceEscrowKey(bob) = %+v, %v (retired %d); want 3 accounts moved", got, err, retired())
	}
	if roles, _ := app.AdminRoles(); !roles.StandbyHoldsKey || !roles.SuperHoldsKey || roles.RotationPending {
		t.Fatalf("AdminRoles = %+v; want both holding the new key and nothing pending", roles)
	}

	// Disabling the standby, who holds the key, replaces it at once.
	if err := app.AdminSetUserDisabled("bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	if roles, _ := app.AdminRoles(); retired() != 2 || roles.RotationPending || !roles.SuperHoldsKey {
		t.Fatalf("after disabling the standby: retired %d, roles %+v; want the key replaced at once", retired(), roles)
	}
	if err := app.AdminSetUserDisabled("bob", false); err != nil {
		t.Fatalf("enable bob: %v", err)
	}

	switchUser(t, app, "bob")
	if _, err := app.AdminReplaceEscrowKey(""); err == nil || err.Error() != "only the super administrator can replace the administrator key" {
		t.Fatalf("subordinate replacing the key: err = %v", err)
	}
	switchUser(t, app, "carol")
	if _, err := app.AdminReplaceEscrowKey(""); err == nil || err.Error() != "administrator privileges required" {
		t.Fatalf("standard account replacing the key: err = %v", err)
	}

	// A super administrator whose key no longer opens cannot replace it.
	if _, err := conn.Exec(`UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`); err != nil {
		t.Fatal(err)
	}
	switchUser(t, app, "alice")
	if _, err := app.AdminReplaceEscrowKey(""); err == nil || err.Error() != "you don't hold a usable administrator key, so you can't replace it" {
		t.Fatalf("replacing without a usable key: err = %v", err)
	}
}

// A hand-over leaves the key waiting for the new super administrator's
// sign-in, and the Admin panel can say so.
func TestAppHandOverLeavesTheKeyReplacementPending(t *testing.T) {
	app := superApp(t)
	if _, err := app.AdminHandOverSuper("bob"); err != nil {
		t.Fatalf("AdminHandOverSuper(bob): %v", err)
	}
	if roles, err := app.AdminRoles(); err != nil || !roles.RotationPending || roles.Super != "bob" || !roles.SuperHoldsKey {
		t.Fatalf("AdminRoles after the hand-over = %+v, %v; want the replacement pending for bob", roles, err)
	}
	switchUser(t, app, "bob")
	if roles, err := app.AdminRoles(); err != nil || roles.RotationPending {
		t.Fatalf("AdminRoles after bob's sign-in = %+v, %v; want the key replaced", roles, err)
	}
}

func TestAppRecoveryCode(t *testing.T) {
	app := superApp(t)
	code, err := app.AdminCreateEscrowRecoveryCode()
	if err != nil || len(code) != 35 {
		t.Fatalf("AdminCreateEscrowRecoveryCode = %q, %v; want a 35-character code", code, err)
	}
	if roles, _ := app.AdminRoles(); !roles.RecoveryCodeExists || roles.RecoveryConfirmed {
		t.Fatalf("AdminRoles = %+v; want a code that is not yet confirmed", roles)
	}
	if err := app.AdminConfirmEscrowRecoveryCode("AAAA"); err == nil || err.Error() != "that doesn't match the recovery code; check it and try again" {
		t.Fatalf("confirming a wrong code: err = %v", err)
	}
	if err := app.AdminConfirmEscrowRecoveryCode(code); err != nil {
		t.Fatalf("confirming the code: %v", err)
	}
	if roles, _ := app.AdminRoles(); !roles.RecoveryConfirmed {
		t.Fatal("the confirmed code is not reported as confirmed")
	}

	conn := systemDB(t, app)
	if _, err := conn.Exec(`DELETE FROM escrow_grants`); err != nil {
		t.Fatal(err)
	}
	if err := app.AdminRecoverEscrowKey("ABCDEFGH-ABCDEFGH-ABCDEFGH-ABCDEFGH"); err == nil || err.Error() != "that recovery code doesn't open the administrator key" {
		t.Fatalf("restoring with a wrong code: err = %v", err)
	}
	if err := app.AdminRecoverEscrowKey(code); err != nil {
		t.Fatalf("AdminRecoverEscrowKey: %v", err)
	}
	if roles, _ := app.AdminRoles(); !roles.SuperHoldsKey {
		t.Fatal("the super administrator does not hold the restored key")
	}
	if err := app.AdminOpenUserData("carol", "checking after recovery"); err != nil {
		t.Fatalf("opening data with the restored key: %v", err)
	}

	switchUser(t, app, "bob")
	if _, err := app.AdminCreateEscrowRecoveryCode(); err == nil || err.Error() != "only the super administrator can make a recovery code" {
		t.Fatalf("a subordinate making a code: err = %v", err)
	}
	if err := app.AdminRecoverEscrowKey(code); err == nil || err.Error() != "only the super administrator can restore the administrator key" {
		t.Fatalf("a subordinate restoring the key: err = %v", err)
	}
}

// A new key is made only when nobody holds the old one, and each account is
// told at its next sign-in, which seals its data to the new key.
func TestAppNewEscrowKey(t *testing.T) {
	app := superApp(t)
	conn := systemDB(t, app)
	if _, err := app.AdminCreateNewEscrowKey("lost"); err == nil || err.Error() != "you hold the administrator key, so it isn't lost; replace it instead" {
		t.Fatalf("a new key while holding the key: err = %v", err)
	}
	if _, err := app.AdminSetStandby("bob", 30); err != nil {
		t.Fatalf("AdminSetStandby(bob): %v", err)
	}
	if _, err := conn.Exec(`DELETE FROM escrow_grants WHERE admin_username = 'alice'`); err != nil {
		t.Fatal(err)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil || err.Error() != "you don't hold the administrator key; under Succession, restore it with its recovery code or make a new one" {
		t.Fatalf("opening data without the key: err = %v", err)
	}
	want := "bob, your standby successor, holds the administrator key, so it isn't lost. To make a new one anyway, first remove bob as standby, which removes their key"
	if _, err := app.AdminCreateNewEscrowKey("lost"); err == nil || err.Error() != want {
		t.Fatalf("a new key while the standby holds it: err = %v", err)
	}
	if _, err := app.AdminSetStandby("", 30); err != nil {
		t.Fatalf("removing the standby: %v", err)
	}
	// A subordinate administrator's grant, as a row left over or forged.
	if _, err := conn.Exec(`INSERT INTO escrow_grants (admin_username, escrow_key_id, sealed, granted_at)
		SELECT 'bob', id, x'00', '2026-10-09T00:00:00Z' FROM escrow_keys WHERE retired_at = ''`); err != nil {
		t.Fatal(err)
	}
	want = "bob holds the administrator key, so it isn't lost. To make a new one anyway, first demote or disable bob, which removes their key"
	if _, err := app.AdminCreateNewEscrowKey("lost"); err == nil || err.Error() != want {
		t.Fatalf("a new key while a subordinate holds it: err = %v", err)
	}
	if _, err := conn.Exec(`DELETE FROM escrow_grants WHERE admin_username = 'bob'`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AdminCreateNewEscrowKey("  "); err == nil || err.Error() != "say why a new administrator key is needed" {
		t.Fatalf("a new key without a reason: err = %v", err)
	}
	if _, err := app.AdminCreateNewEscrowKey(strings.Repeat("x", 501)); err == nil || err.Error() != "the reason can be at most 500 characters" {
		t.Fatalf("a new key with a long reason: err = %v", err)
	}
	if n, err := app.AdminCreateNewEscrowKey("the recovery code is lost"); err != nil || n != 2 {
		t.Fatalf("AdminCreateNewEscrowKey = %d, %v; want bob and carol to be told", n, err)
	}
	if roles, _ := app.AdminRoles(); !roles.SuperHoldsKey || roles.RotationPending {
		t.Fatalf("AdminRoles = %+v; want alice holding the new key and nothing pending", roles)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil || err.Error() != "carol hasn't signed in since the current administrator key was made, so their data can't be opened yet" {
		t.Fatalf("opening carol's data before she signs in again: err = %v", err)
	}

	switchUser(t, app, "carol")
	notices, err := app.MyDataAccessNotices()
	if err != nil || len(notices) != 1 || notices[0].Action != "escrow_key_new" || notices[0].Actor != "alice" || notices[0].Detail != "the recovery code is lost" {
		t.Fatalf("carol's notices = %+v, %v; want alice's new key", notices, err)
	}
	if err := app.AcknowledgeDataAccessNotices(notices[0].ID); err != nil {
		t.Fatalf("AcknowledgeDataAccessNotices: %v", err)
	}
	switchUser(t, app, "bob")
	if _, err := app.AdminCreateNewEscrowKey("again"); err == nil || err.Error() != "only the super administrator can make a new administrator key" {
		t.Fatalf("a subordinate making a new key: err = %v", err)
	}
	switchUser(t, app, "alice")
	if err := app.AdminOpenUserData("carol", "checking after she signed in"); err != nil {
		t.Fatalf("opening carol's data after she signed in: %v", err)
	}
}

func TestAppPrintsTheRecoveryCodeOnlyAfterCheckingIt(t *testing.T) {
	type job struct{ printer, title, text string }
	var jobs []job
	oldPrinters, oldPrint := printers, printText
	t.Cleanup(func() { printers, printText = oldPrinters, oldPrint })
	printers = func(context.Context) ([]printing.Printer, error) {
		return []printing.Printer{{Name: "Desk", Connection: printing.Local, Default: true}, {Name: "Office", Connection: printing.Network}}, nil
	}
	printText = func(_ context.Context, printer, title, text string) error {
		if printer != "Desk" && printer != "Office" {
			return printing.ErrNoSuchPrinter
		}
		jobs = append(jobs, job{printer, title, text})
		return nil
	}

	app := superApp(t)
	code, err := app.AdminCreateEscrowRecoveryCode()
	if err != nil {
		t.Fatalf("AdminCreateEscrowRecoveryCode: %v", err)
	}
	list, err := app.AdminPrinters()
	if err != nil || len(list) != 2 || list[0] != (AdminPrinterWire{Name: "Desk", Connection: "local", Default: true}) {
		t.Fatalf("AdminPrinters = %+v, %v", list, err)
	}

	if err := app.AdminPrintEscrowRecoveryCode("ABCDEFGH-ABCDEFGH-ABCDEFGH-ABCDEFGH", "Desk"); err == nil || err.Error() != "that isn't the administrator key's recovery code" {
		t.Fatalf("printing a wrong code: err = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatal("a code that is not the key's was printed")
	}
	if err := app.AdminPrintEscrowRecoveryCode(code, "Elsewhere"); err == nil || err.Error() != `"Elsewhere" isn't a printer this computer can print on paper with; choose another` {
		t.Fatalf("printing on an unknown printer: err = %v", err)
	}
	if err := app.AdminPrintEscrowRecoveryCode(code, "Desk"); err != nil {
		t.Fatalf("AdminPrintEscrowRecoveryCode: %v", err)
	}
	if len(jobs) != 1 || jobs[0].printer != "Desk" || jobs[0].title != "GoPMgr administrator recovery" || !strings.Contains(jobs[0].text, "    "+code+"\n") {
		t.Fatalf("printed %+v; want the page with the code on Desk under a title without it", jobs)
	}
	for _, r := range jobs[0].text {
		if r > 127 {
			t.Fatalf("the page has a non-ASCII character %q", r)
		}
	}
	if roles, _ := app.AdminRoles(); roles.RecoveryConfirmed {
		t.Fatal("printing confirmed the code; only typing it back does")
	}

	switchUser(t, app, "bob")
	if _, err := app.AdminPrinters(); err == nil {
		t.Fatal("a subordinate listed printers for the recovery code")
	}
	if err := app.AdminPrintEscrowRecoveryCode(code, "Desk"); err == nil || err.Error() != "only the super administrator can print the recovery code" {
		t.Fatalf("a subordinate printing: err = %v", err)
	}
}

func TestAppPrintersWithoutAPrintService(t *testing.T) {
	oldPrinters := printers
	t.Cleanup(func() { printers = oldPrinters })
	printers = func(context.Context) ([]printing.Printer, error) { return nil, printing.ErrUnsupported }
	app := superApp(t)
	if list, err := app.AdminPrinters(); err != nil || list == nil || len(list) != 0 {
		t.Fatalf("AdminPrinters = %#v, %v; want an empty list so the panel says to write it down", list, err)
	}
}
