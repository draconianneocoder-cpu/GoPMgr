// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"gopmgr/internal/crypto"
)

func mustCreateRecoveryCode(t *testing.T, store *Store, actor string, dek []byte) string {
	t.Helper()
	code, err := store.CreateEscrowRecoveryCode(actor, dek)
	if err != nil {
		t.Fatalf("CreateEscrowRecoveryCode: %v", err)
	}
	return code
}

func TestARecoveryCodeRestoresTheSameKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	carolDEK := addUser(t, store, "carol", nil)
	keyID := activeKeyID(t, store)

	code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	if !regexp.MustCompile(`^[A-Z2-7]{8}(-[A-Z2-7]{8}){3}$`).MatchString(code) {
		t.Fatalf("code %q is not four groups of eight base32 characters", code)
	}
	if got := mustSuccession(t, store, "alice"); !got.RecoveryCodeExists || got.RecoveryCodeConfirmed {
		t.Fatalf("Succession = %+v; want a code, not yet confirmed", got)
	}
	mistyped := "Q" + code[1:]
	if code[0] == 'Q' {
		mistyped = "R" + code[1:]
	}
	if err := store.ConfirmEscrowRecoveryCode("alice", mistyped); !errors.Is(err, ErrInvalidEscrowRecoveryCode) {
		t.Fatalf("confirming a mistyped code = %v, want ErrInvalidEscrowRecoveryCode", err)
	}
	if mustSuccession(t, store, "alice").RecoveryCodeConfirmed {
		t.Fatal("a mistyped confirmation confirmed the code")
	}
	if err := store.ConfirmEscrowRecoveryCode("alice", "  "+strings.ToLower(strings.ReplaceAll(code, "-", " "))+" "); err != nil {
		t.Fatalf("confirming the code as typed: %v", err)
	}
	if !mustSuccession(t, store, "alice").RecoveryCodeConfirmed {
		t.Fatal("the confirmed code is not marked confirmed")
	}

	// Nobody can open the key any more.
	mustExec(t, store, `DELETE FROM escrow_grants`)
	if err := store.RecoverEscrowKey("alice", aliceDEK, code); err != nil {
		t.Fatalf("RecoverEscrowKey: %v", err)
	}
	if activeKeyID(t, store) != keyID {
		t.Fatal("recovery replaced the key; it must restore the same one")
	}
	if got, err := openSealedDEK(store, openEscrow(t, store, "alice", aliceDEK), "carol"); err != nil || !bytes.Equal(got, carolDEK) {
		t.Fatalf("carol's DEK with the restored key = %x, %v", got, err)
	}
	requireEvent(t, store, "alice escrow_key_recovered alice")
	requireEvent(t, store, "alice escrow_recovery_code_created alice")
}

func TestARecoveryCodeIsReadForgivingly(t *testing.T) {
	raw, code, err := newEscrowRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	for _, typed := range []string{code, strings.ToLower(code), strings.ReplaceAll(code, "-", ""), " " + strings.ReplaceAll(code, "-", " ") + " "} {
		got, err := parseEscrowRecoveryCode(typed)
		if err != nil || !bytes.Equal(got, raw) {
			t.Errorf("parse(%q) = %x, %v; want the code", typed, got, err)
		}
	}
	// The code never contains 0, 1, or 8; written down, they are read as
	// the letters O, I, and B.
	const letters = "OIBAOIBA-OIBAOIBA-OIBAOIBA-OIBAOIBA"
	want, err := parseEscrowRecoveryCode(letters)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := parseEscrowRecoveryCode("018a018a-018A018A 018A018A 018a018a"); err != nil || !bytes.Equal(got, want) {
		t.Errorf("parse with 0, 1, and 8 = %x, %v; want %s", got, err, letters)
	}
	for _, typed := range []string{"", "hello", code[:20], code + "A", code + "AAAAAAAA"} {
		if _, err := parseEscrowRecoveryCode(typed); !errors.Is(err, ErrInvalidEscrowRecoveryCode) {
			t.Errorf("parse(%q) = %v, want ErrInvalidEscrowRecoveryCode", typed, err)
		}
	}
}

// A wrong code is refused and recorded, and neither the history nor
// anything else written keeps the code.
func TestAWrongRecoveryCodeIsRefusedAndTheCodeIsNeverRecorded(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	_, other, err := newEscrowRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, store, `DELETE FROM escrow_grants`)
	for _, typed := range []string{other, "hello"} {
		if err := store.RecoverEscrowKey("alice", aliceDEK, typed); !errors.Is(err, ErrInvalidEscrowRecoveryCode) {
			t.Fatalf("RecoverEscrowKey(%q) = %v, want ErrInvalidEscrowRecoveryCode", typed, err)
		}
	}
	if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
		t.Fatal("a wrong code granted the key")
	}
	if n := countRows(t, store, "account_events", "action = 'escrow_key_mismatch' AND detail = 'a recovery code that does not open the administrator key was entered'"); n != 2 {
		t.Fatalf("%d failed attempts recorded, want 2", n)
	}
	if err := store.RecoverEscrowKey("alice", aliceDEK, code); err != nil {
		t.Fatalf("RecoverEscrowKey: %v", err)
	}
	canonical := strings.ReplaceAll(code, "-", "")
	events, _ := store.AccountEvents()
	for _, e := range events {
		for _, field := range []string{e.Actor, e.Username, e.Action, e.Detail} {
			if strings.Contains(field, code) || strings.Contains(field, canonical) || strings.Contains(field, other) {
				t.Fatalf("event %+v contains a recovery code", e)
			}
		}
	}
}

func TestOnlyTheSuperAdministratorMakesOrUsesACode(t *testing.T) {
	store, aliceDEK, bobDEK := superStore(t)
	code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	if _, err := store.CreateEscrowRecoveryCode("bob", bobDEK); !errors.Is(err, ErrNotSuper) {
		t.Errorf("a subordinate making a code = %v, want ErrNotSuper", err)
	}
	if err := store.ConfirmEscrowRecoveryCode("bob", code); !errors.Is(err, ErrNotSuper) {
		t.Errorf("a subordinate confirming the code = %v, want ErrNotSuper", err)
	}
	if err := store.RecoverEscrowKey("bob", bobDEK, code); !errors.Is(err, ErrNotSuper) {
		t.Errorf("a subordinate using the code = %v, want ErrNotSuper", err)
	}
	mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`)
	if _, err := store.CreateEscrowRecoveryCode("alice", aliceDEK); !errors.Is(err, ErrNoEscrowGrant) {
		t.Errorf("making a code without a usable key = %v, want ErrNoEscrowGrant", err)
	}
	requireEvent(t, store, "alice escrow_key_mismatch alice") // committed
}

func TestANewCodeOrANewKeyVoidsTheOldCode(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	first := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	second := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	if e := latestEvent(t, store); e.Action != AccountEscrowRecoveryCreated || e.Detail != "replaces the earlier one" {
		t.Fatalf("latest event = %+v; want the replacement recorded", e)
	}
	if err := store.ConfirmEscrowRecoveryCode("alice", first); !errors.Is(err, ErrInvalidEscrowRecoveryCode) {
		t.Fatalf("the replaced code = %v, want ErrInvalidEscrowRecoveryCode", err)
	}
	if err := store.ConfirmEscrowRecoveryCode("alice", second); err != nil {
		t.Fatalf("the new code: %v", err)
	}

	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, ""); err != nil {
		t.Fatalf("ReplaceEscrowKey: %v", err)
	}
	if got := mustSuccession(t, store, "alice"); got.RecoveryCodeExists || got.RecoveryCodeConfirmed {
		t.Fatalf("Succession = %+v; a new key has no code until one is made", got)
	}
	// The old key's wrapped private half is gone from system.db, not just
	// unused.
	if n := countRows(t, store, "escrow_recovery", "1 = 1"); n != 0 {
		t.Fatalf("%d recovery rows remain after the key was replaced", n)
	}
	if err := store.RecoverEscrowKey("alice", aliceDEK, second); !errors.Is(err, ErrNoRecoveryCode) {
		t.Fatalf("the old key's code after a replacement = %v, want ErrNoRecoveryCode", err)
	}
}

// A recovery row forged into system.db, with another key and another code,
// restores nothing: the key must be the one the super administrator's own
// pin trusts.
func TestAForgedRecoveryRowRestoresNothing(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	keyID := activeKeyID(t, store)
	attackerPriv, attackerPub := attackerKeyPair(t)
	raw, code, err := newEscrowRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := crypto.WrapEscrowRecovery(raw, keyID, attackerPriv)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, store, `INSERT INTO escrow_recovery (escrow_key_id, wrapped, confirmed, created_at, created_by) VALUES (?, ?, 1, '2026-10-07T00:00:00Z', 'alice')`, keyID, wrapped)
	mustExec(t, store, `DELETE FROM escrow_grants`)

	if err := store.RecoverEscrowKey("alice", aliceDEK, code); !errors.Is(err, ErrEscrowMismatch) {
		t.Fatalf("a forged row = %v, want ErrEscrowMismatch", err)
	}
	// Also swapping the public key in escrow_keys does not help: the pin
	// is the account's own.
	mustExec(t, store, `UPDATE escrow_keys SET public_key = ? WHERE id = ?`, attackerPub, keyID)
	if err := store.RecoverEscrowKey("alice", aliceDEK, code); !errors.Is(err, ErrEscrowMismatch) {
		t.Fatalf("a forged row and public key = %v, want ErrEscrowMismatch", err)
	}
	if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
		t.Fatal("a forged recovery row granted a key")
	}
}

// Recovery never trusts on first use: the super administrator needs a pin
// of the active key, and the table must name the key the code opens.
func TestRecoveryNeedsThePinAndTheTableToAgree(t *testing.T) {
	cases := map[string]func(store *Store){
		"no pin": func(store *Store) {
			mustExec(t, store, `DELETE FROM escrow_pins WHERE username = 'alice'`)
		},
		"a pin of another key": func(store *Store) {
			mustExec(t, store, `INSERT INTO escrow_keys (id, kem_id, public_key, created_at, retired_at) VALUES ('old', 32, x'00', 'x', 'y')`)
			mustExec(t, store, `UPDATE escrow_pins SET escrow_key_id = 'old' WHERE username = 'alice'`)
		},
		"an altered pin": func(store *Store) {
			mustExec(t, store, `UPDATE escrow_pins SET pin = x'00' WHERE username = 'alice'`)
		},
		"a swapped public key": func(store *Store) {
			_, pub := attackerKeyPair(t)
			mustExec(t, store, `UPDATE escrow_keys SET public_key = ? WHERE retired_at = ''`, pub)
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			store, aliceDEK := escrowStore(t)
			code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
			mustExec(t, store, `DELETE FROM escrow_grants`)
			tamper(store)
			if err := store.RecoverEscrowKey("alice", aliceDEK, code); !errors.Is(err, ErrEscrowMismatch) {
				t.Fatalf("RecoverEscrowKey = %v, want ErrEscrowMismatch", err)
			}
			if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
				t.Fatal("the key was granted anyway")
			}
			requireEvent(t, store, "alice escrow_key_mismatch alice")
		})
	}
}

// Restoring the key runs a pending replacement, which voids the code.
func TestRecoveryRunsAPendingReplacement(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
	keyID := activeKeyID(t, store)
	mustExec(t, store, `DELETE FROM escrow_grants`)
	mustExec(t, store, `INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, 'test', '2026-10-07T00:00:00Z')`)

	if err := store.RecoverEscrowKey("alice", aliceDEK, code); err != nil {
		t.Fatalf("RecoverEscrowKey: %v", err)
	}
	if activeKeyID(t, store) == keyID || rotationPending(t, store) {
		t.Fatal("the pending replacement did not run after the recovery")
	}
	openEscrow(t, store, "alice", aliceDEK)
	if got := mustSuccession(t, store, "alice"); got.RecoveryCodeExists {
		t.Fatal("the code survived the replacement")
	}
}
