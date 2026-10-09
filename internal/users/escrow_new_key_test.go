// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const newKeyReason = "alice left and the recovery code is lost"

// lostKeyStore returns a machine where bob, with no standby named, took over
// from alice, so nobody holds the administrator key: alice and dave are
// subordinate administrators, carol a standard account.
func lostKeyStore(t *testing.T) (store *Store, deks map[string][]byte) {
	t.Helper()
	store, aliceDEK := escrowStore(t)
	deks = map[string][]byte{"alice": aliceDEK}
	for _, name := range []string{"bob", "carol", "dave"} {
		deks[name] = addUser(t, store, name, openEscrow(t, store, "alice", aliceDEK))
	}
	for _, name := range []string{"bob", "dave"} {
		if err := store.PromoteAdmin("alice", aliceDEK, name); err != nil {
			t.Fatalf("PromoteAdmin(%s): %v", name, err)
		}
	}
	superInactiveFor(t, store, 31*day)
	if _, err := store.TakeOverSuper("bob", deks["bob"], "left_employment", ""); err != nil {
		t.Fatalf("TakeOverSuper(bob): %v", err)
	}
	if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
		t.Fatalf("%d grants after the takeover, want none", n)
	}
	return store, deks
}

func mustCreateNewKey(t *testing.T, store *Store, actor string, dek []byte) int {
	t.Helper()
	n, err := store.CreateNewEscrowKey(actor, dek, newKeyReason)
	if err != nil {
		t.Fatalf("CreateNewEscrowKey(%s): %v", actor, err)
	}
	return n
}

func unreadNotices(t *testing.T, store *Store, username string) []AccountEvent {
	t.Helper()
	notices, err := store.UnreadAccessNotices(username)
	if err != nil {
		t.Fatalf("UnreadAccessNotices(%s): %v", username, err)
	}
	return notices
}

// A new key replaces one nobody holds. Nobody can open an account's data
// until it signs in, and each account is told who made the key and why.
func TestANewKeyReplacesALostKeyAndEveryAccountIsTold(t *testing.T) {
	store, deks := lostKeyStore(t)
	oldKey := activeKeyID(t, store)
	if _, err := store.OpenUserForAdmin("bob", deks["bob"], "carol", "checking"); !errors.Is(err, ErrNoEscrowGrant) {
		t.Fatalf("opening carol's data before the new key = %v, want ErrNoEscrowGrant", err)
	}

	if n := mustCreateNewKey(t, store, "bob", deks["bob"]); n != 3 {
		t.Fatalf("CreateNewEscrowKey = %d accounts, want 3 (alice, carol, dave)", n)
	}
	newKey := activeKeyID(t, store)
	if newKey == oldKey {
		t.Fatal("the active key did not change")
	}
	session := openEscrow(t, store, "bob", deks["bob"])
	if got, err := openSealedDEK(store, session, "bob"); err != nil || !bytes.Equal(got, deks["bob"]) {
		t.Fatalf("bob's own DEK with the new key = %x, %v", got, err)
	}
	if rotationPending(t, store) {
		t.Fatal("the replacement is still pending after the new key")
	}
	for table, want := range map[string]int{"escrow_grants": 1, "sealed_deks": 1, "escrow_pins": 1} {
		if n := countRows(t, store, table, bobOnly(table)); n != want {
			t.Errorf("%s has %d rows for bob, want %d", table, n, want)
		}
		if n := countRows(t, store, table, "1 = 1"); n != want {
			t.Errorf("%s has %d rows, want only bob's", table, n)
		}
	}

	// Until carol signs in, nobody can open her data.
	if _, err := store.OpenUserForAdmin("bob", deks["bob"], "carol", "checking"); !errors.Is(err, ErrNotEnrolled) {
		t.Fatalf("opening carol's data before she signs in = %v, want ErrNotEnrolled", err)
	}

	for _, name := range []string{"alice", "carol", "dave"} {
		notices := unreadNotices(t, store, name)
		if len(notices) != 1 || notices[0].Action != AccountEscrowKeyNew || notices[0].Actor != "bob" || notices[0].Detail != newKeyReason {
			t.Fatalf("%s's notices = %+v; want bob's new key with its reason", name, notices)
		}
	}
	if notices := unreadNotices(t, store, "bob"); len(notices) != 0 {
		t.Fatalf("bob's own notices = %+v; the creator is not told of their own key", notices)
	}
	requireEvent(t, store, "bob escrow_key_new bob")

	// Each kind of account seals to the new key at its next sign-in: a
	// standard account, a subordinate administrator, and the former super
	// administrator.
	for _, name := range []string{"carol", "dave", "alice"} {
		if err := store.EnrollAdminSession(name, deks[name]); err != nil {
			t.Fatalf("EnrollAdminSession(%s): %v", name, err)
		}
		if got, err := openSealedDEK(store, session, name); err != nil || !bytes.Equal(got, deks[name]) {
			t.Fatalf("%s's DEK with the new key after signing in = %x, %v", name, got, err)
		}
		requireEvent(t, store, name+" escrow_reenrolled "+name)
	}
	dek, err := store.OpenUserForAdmin("bob", deks["bob"], "carol", "checking")
	if err != nil || !bytes.Equal(dek, deks["carol"]) {
		t.Fatalf("OpenUserForAdmin(carol) after she signed in = %x, %v", dek, err)
	}
	clear(dek)

	// The notice stays until acknowledged, then is recorded as read.
	notices := unreadNotices(t, store, "carol")
	if len(notices) != 2 {
		t.Fatalf("carol's notices = %+v; want the access and the new key", notices)
	}
	if err := store.AcknowledgeAccessNotices("carol", notices[0].ID); err != nil {
		t.Fatalf("AcknowledgeAccessNotices: %v", err)
	}
	if left := unreadNotices(t, store, "carol"); len(left) != 0 {
		t.Fatalf("carol's notices after acknowledging = %+v", left)
	}
	if got := newestEvent(t, store, AccountAccessNoticeRead); got.Username != "carol" || got.Detail != "1 access and 1 new administrator key" {
		t.Fatalf("notice read = %+v", got)
	}
}

func bobOnly(table string) string {
	if table == "escrow_grants" {
		return "admin_username = 'bob'"
	}
	return "username = 'bob'"
}

// The creator's own record of the key is not a notice, so it cannot be
// acknowledged as one.
func TestTheCreatorsOwnRecordIsNotANotice(t *testing.T) {
	store, deks := lostKeyStore(t)
	mustCreateNewKey(t, store, "bob", deks["bob"])
	own := newestEvent(t, store, AccountEscrowKeyNew)
	if own.Username != "bob" || !strings.HasPrefix(own.Detail, newKeyReason+"; 3 accounts") {
		t.Fatalf("newest new-key event = %+v; want bob's own record", own)
	}
	if err := store.AcknowledgeAccessNotices("bob", own.ID); !errors.Is(err, ErrAcknowledgeUnknownAccess) {
		t.Fatalf("acknowledging the creator's own record = %v, want ErrAcknowledgeUnknownAccess", err)
	}
}

// Nothing changes when the key is not lost or the request is not allowed.
func TestANewKeyIsRefusedWhenItIsNotNeeded(t *testing.T) {
	store, aliceDEK, bobDEK, daveDEK := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "dave", 30)
	before := activeKeyID(t, store)

	if _, err := store.CreateNewEscrowKey("alice", aliceDEK, newKeyReason); !errors.Is(err, ErrHoldsEscrowKey) {
		t.Fatalf("a new key while alice holds hers = %v, want ErrHoldsEscrowKey", err)
	}
	if _, err := store.CreateNewEscrowKey("bob", bobDEK, newKeyReason); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("a new key by a subordinate = %v, want ErrNotSuper", err)
	}
	for _, reason := range []string{"", " \n\t", strings.Repeat("é", MaxNewKeyReasonLength+1)} {
		want := ErrNewKeyReasonRequired
		if strings.TrimSpace(reason) != "" {
			want = ErrNewKeyReasonTooLong
		}
		if _, err := store.CreateNewEscrowKey("alice", aliceDEK, reason); !errors.Is(err, want) {
			t.Errorf("a new key with a %d-character reason = %v, want %v", len([]rune(reason)), err, want)
		}
	}

	// alice's key is lost, but dave, the standby, still holds it.
	mustExec(t, store, `DELETE FROM escrow_grants WHERE admin_username = 'alice'`)
	_, err := store.CreateNewEscrowKey("alice", aliceDEK, newKeyReason)
	var held *KeyHeldError
	if !errors.As(err, &held) || !errors.Is(err, ErrKeyStillHeld) || held.Holder != "dave" || !held.Standby ||
		err.Error() != "users: dave holds the administrator key" {
		t.Fatalf("a new key while the standby holds the key = %v; want dave, the standby, named", err)
	}
	if activeKeyID(t, store) != before || countRows(t, store, "escrow_keys", "1 = 1") != 1 ||
		countRows(t, store, "account_events", "action = 'escrow_key_new'") != 0 ||
		countRows(t, store, "sealed_deks", "1 = 1") != 4 {
		t.Fatal("a refused new key changed the key, the sealed keys, or the history")
	}

	// Removing dave as standby removes his key, so the key is lost.
	mustSetStandby(t, store, "alice", aliceDEK, "", 30)
	mustCreateNewKey(t, store, "alice", aliceDEK)

	// dave can be named standby again before he signs in, and his sign-in
	// then seals him to the new key and opens his copy of it.
	mustSetStandby(t, store, "alice", aliceDEK, "dave", 30)
	if err := store.EnrollAdminSession("dave", daveDEK); err != nil {
		t.Fatalf("EnrollAdminSession(dave): %v", err)
	}
	if session := openEscrow(t, store, "dave", daveDEK); session.id != activeKeyID(t, store) {
		t.Fatal("dave's copy is not the new key")
	}
	requireEvent(t, store, "alice personal_key_trusted dave")
}

// With no administrator key at all, the first administrator sign-in makes
// one; a new key is not a second way to make the first.
func TestANewKeyNeedsAnEarlierKey(t *testing.T) {
	store := openTestStore(t)
	_, dek, err := store.CreateAccountWithKey("", "alice", "Alice", statusPassword, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(dek) })
	mustExec(t, store, `DELETE FROM escrow_grants`)
	mustExec(t, store, `DELETE FROM sealed_deks`)
	mustExec(t, store, `DELETE FROM escrow_pins`)
	mustExec(t, store, `DELETE FROM escrow_keys`)
	if _, err := store.CreateNewEscrowKey("alice", dek, newKeyReason); !errors.Is(err, ErrNoEscrowGrant) {
		t.Fatalf("a new key with no key ever made = %v, want ErrNoEscrowGrant", err)
	}
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 0 {
		t.Fatalf("%d keys made, want none", n)
	}
}

// A system.db with keys but none active (changed outside GoPMgr) is
// repaired by a new key.
func TestANewKeyRepairsADatabaseWithNoActiveKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	mustExec(t, store, `UPDATE escrow_keys SET retired_at = '2026-10-01T00:00:00Z'`)
	mustCreateNewKey(t, store, "alice", aliceDEK)
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 2 {
		t.Fatalf("%d keys, want the old one and the new one", n)
	}
	openEscrow(t, store, "alice", aliceDEK)
}

// When every administrator is disabled outside GoPMgr, the account that
// claims the role can make a new key: a disabled administrator's leftover
// grant does not count as holding it.
func TestTheClaimantAfterEveryAdministratorIsDisabledCanMakeANewKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	carolDEK := addUser(t, store, "carol", openEscrow(t, store, "alice", aliceDEK))
	mustExec(t, store, `UPDATE users SET disabled = 1 WHERE username = 'alice'`)
	if err := store.ClaimAdmin("carol"); err != nil {
		t.Fatalf("ClaimAdmin(carol): %v", err)
	}
	if super := mustSuper(t, store); super != "carol" {
		t.Fatalf("super administrator = %q, want carol", super)
	}
	if n := mustCreateNewKey(t, store, "carol", carolDEK); n != 1 {
		t.Fatalf("CreateNewEscrowKey = %d, want 1 (alice)", n)
	}
	if got := newestEvent(t, store, AccountEscrowKeyNew); got.Detail != newKeyReason+"; 1 account sealed to it at its next sign-in" {
		t.Fatalf("carol's record = %q", got.Detail)
	}
	openEscrow(t, store, "carol", carolDEK)
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
		t.Fatal("the disabled administrator kept a grant")
	}
}

// The old key's recovery code no longer restores anything: its row goes
// with the old key.
func TestANewKeyVoidsTheOldRecoveryCode(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	if err := store.ConfirmEscrowRecoveryCode("alice", code); err != nil {
		t.Fatal(err)
	}
	mustExec(t, store, `DELETE FROM escrow_grants`)
	mustCreateNewKey(t, store, "alice", aliceDEK)
	if n := countRows(t, store, "escrow_recovery", "1 = 1"); n != 0 {
		t.Fatalf("%d recovery rows after a new key, want none", n)
	}
	if got := mustSuccession(t, store, "alice"); got.RecoveryCodeExists {
		t.Fatal("the new key reports the old key's recovery code")
	}
	mustExec(t, store, `DELETE FROM escrow_grants`)
	if err := store.RecoverEscrowKey("alice", aliceDEK, code); !errors.Is(err, ErrNoRecoveryCode) {
		t.Fatalf("restoring with the old code = %v, want ErrNoRecoveryCode", err)
	}
}

// A replacement of the new key before an account signs in again still
// leaves the account told and sealed at its next sign-in.
func TestAnAccountIsToldAndSealedAfterTheNewKeyIsReplacedToo(t *testing.T) {
	store, deks := lostKeyStore(t)
	mustCreateNewKey(t, store, "bob", deks["bob"])
	if _, err := store.ReplaceEscrowKey("bob", deks["bob"], ""); err != nil {
		t.Fatalf("ReplaceEscrowKey: %v", err)
	}
	if notices := unreadNotices(t, store, "carol"); len(notices) != 1 || notices[0].Action != AccountEscrowKeyNew {
		t.Fatalf("carol's notices = %+v; want the new key", notices)
	}
	if err := store.EnrollAdminSession("carol", deks["carol"]); err != nil {
		t.Fatalf("EnrollAdminSession(carol): %v", err)
	}
	if got, err := openSealedDEK(store, openEscrow(t, store, "bob", deks["bob"]), "carol"); err != nil || !bytes.Equal(got, deks["carol"]) {
		t.Fatalf("carol's DEK with the replaced key = %x, %v", got, err)
	}
}

// The whole change is one transaction: a failure while telling the last
// account leaves the old key and everyone's sealed keys in place.
func TestAFailedNewKeyChangesNothing(t *testing.T) {
	store, deks := lostKeyStore(t)
	before := activeKeyID(t, store)
	sealed := countRows(t, store, "sealed_deks", "1 = 1")
	pins := countRows(t, store, "escrow_pins", "1 = 1")
	mustExec(t, store, `CREATE TEMP TRIGGER fail_new_key BEFORE INSERT ON account_events
		WHEN NEW.action = 'escrow_key_new' AND NEW.username = 'dave'
		BEGIN SELECT RAISE(ABORT, 'injected failure'); END`)
	if _, err := store.CreateNewEscrowKey("bob", deks["bob"], newKeyReason); err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("CreateNewEscrowKey with a failing record = %v, want the injected failure", err)
	}
	if activeKeyID(t, store) != before || countRows(t, store, "escrow_keys", "1 = 1") != 1 {
		t.Fatal("a failed new key changed the keys")
	}
	if countRows(t, store, "sealed_deks", "1 = 1") != sealed || countRows(t, store, "escrow_pins", "1 = 1") != pins ||
		countRows(t, store, "account_events", "action = 'escrow_key_new'") != 0 || !rotationPending(t, store) {
		t.Fatal("a failed new key removed sealed keys, pins, or the pending replacement, or recorded a notice")
	}
}

func TestNoticeReadDetail(t *testing.T) {
	for _, tc := range []struct {
		accesses, keys int
		want           string
	}{
		{1, 0, "1 access"},
		{3, 0, "3 accesses"},
		{0, 1, "1 new administrator key"},
		{0, 2, "2 new administrator keys"},
		{2, 1, "2 accesses and 1 new administrator key"},
	} {
		if got := noticeReadDetail(tc.accesses, tc.keys); got != tc.want {
			t.Errorf("noticeReadDetail(%d, %d) = %q, want %q", tc.accesses, tc.keys, got, tc.want)
		}
	}
}

// The reason's limit counts characters, not bytes.
func TestANewKeyReasonIsCountedInCharacters(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	mustExec(t, store, `DELETE FROM escrow_grants`)
	reason := strings.Repeat("é", MaxNewKeyReasonLength)
	if _, err := store.CreateNewEscrowKey("alice", aliceDEK, reason); err != nil {
		t.Fatalf("a %d-character reason = %v, want it accepted", MaxNewKeyReasonLength, err)
	}
	if got := newestEvent(t, store, AccountEscrowKeyNew); !strings.HasPrefix(got.Detail, reason+";") {
		t.Fatal("the recorded detail does not start with the reason")
	}
}
