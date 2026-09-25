// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"testing"

	"gopmgr/internal/auth"
)

// newRotationStore returns a store with alice, her DEK, and her first
// set of recovery codes.
func newRotationStore(t *testing.T) (*Store, []byte, []string) {
	t.Helper()
	store, dek := newPasswordStore(t)
	codes, err := store.IssueRecoveryCodes("alice", dek)
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	return store, dek, codes
}

// resetRecoversKey resets alice's password with code and reports whether
// the reset worked and unwrapped want.
func resetRecoversKey(t *testing.T, store *Store, code string, want []byte) bool {
	t.Helper()
	if err := store.ResetWithRecoveryCode("alice", code, "recovered-password"); err != nil {
		return false
	}
	got, err := store.UnlockDEK("alice", "recovered-password")
	if err != nil {
		t.Fatalf("UnlockDEK after a reset: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("a recovery code recovered a different key; encrypted projects would be lost")
	}
	return true
}

func TestPreparedRecoveryCodesLeaveTheOldCodesWorking(t *testing.T) {
	store, dek, oldCodes := newRotationStore(t)
	pending, err := store.PrepareRecoveryCodes("alice", oldPassword)
	if err != nil {
		t.Fatalf("PrepareRecoveryCodes: %v", err)
	}
	if got := len(pending.Codes()); got != RecoveryCodeCount {
		t.Fatalf("prepared %d codes, want %d", got, RecoveryCodeCount)
	}
	if n, err := store.RemainingRecoveryCodes("alice"); err != nil || n != RecoveryCodeCount {
		t.Fatalf("unused codes after prepare = %d, %v; want the old %d", n, err, RecoveryCodeCount)
	}
	if !resetRecoversKey(t, store, oldCodes[0], dek) {
		t.Fatal("an old code stopped working before the new codes were confirmed")
	}
}

func TestConfirmedRecoveryCodesReplaceTheOldOnes(t *testing.T) {
	store, dek, oldCodes := newRotationStore(t)
	pending, err := store.PrepareRecoveryCodes("alice", oldPassword)
	if err != nil {
		t.Fatalf("PrepareRecoveryCodes: %v", err)
	}
	if err := store.ConfirmRecoveryCodes(pending); err != nil {
		t.Fatalf("ConfirmRecoveryCodes: %v", err)
	}
	if resetRecoversKey(t, store, oldCodes[0], dek) {
		t.Fatal("an old code still works after the new codes were confirmed")
	}
	if !resetRecoversKey(t, store, pending.Codes()[0], dek) {
		t.Fatal("a new code does not work after confirmation")
	}
	if unused, legacy, err := store.RecoveryCodeStatus("alice"); err != nil || unused != RecoveryCodeCount-1 || legacy {
		t.Fatalf("status after using one new code = %d, legacy %v, %v; want %d, false", unused, legacy, err, RecoveryCodeCount-1)
	}
}

func TestPrepareRecoveryCodesNeedsTheCurrentPassword(t *testing.T) {
	store, dek, oldCodes := newRotationStore(t)
	if _, err := store.PrepareRecoveryCodes("alice", "not-the-password"); !errors.Is(err, auth.ErrMismatch) {
		t.Fatalf("PrepareRecoveryCodes with a wrong password: err = %v, want auth.ErrMismatch", err)
	}
	if _, err := store.PrepareRecoveryCodes("nobody", oldPassword); !errors.Is(err, ErrNoSuchUser) {
		t.Fatalf("PrepareRecoveryCodes for an unknown account: err = %v, want ErrNoSuchUser", err)
	}
	if !resetRecoversKey(t, store, oldCodes[0], dek) {
		t.Fatal("a refused prepare changed the stored codes")
	}
}

// TestConfirmRecoveryCodesRefusesAfterTheCodesChanged covers a code being
// used, or new codes issued, between prepare and confirm.
func TestConfirmRecoveryCodesRefusesAfterTheCodesChanged(t *testing.T) {
	for name, change := range map[string]func(t *testing.T, store *Store, dek []byte, oldCodes []string){
		"a code was used": func(t *testing.T, store *Store, dek []byte, oldCodes []string) {
			if err := store.ResetWithRecoveryCode("alice", oldCodes[0], "elsewhere-password"); err != nil {
				t.Fatalf("reset: %v", err)
			}
		},
		"codes were issued elsewhere": func(t *testing.T, store *Store, dek []byte, oldCodes []string) {
			if _, err := store.IssueRecoveryCodes("alice", dek); err != nil {
				t.Fatalf("issue: %v", err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			store, dek, oldCodes := newRotationStore(t)
			pending, err := store.PrepareRecoveryCodes("alice", oldPassword)
			if err != nil {
				t.Fatalf("PrepareRecoveryCodes: %v", err)
			}
			change(t, store, dek, oldCodes)

			if err := store.ConfirmRecoveryCodes(pending); !errors.Is(err, ErrRecoveryCodesChanged) {
				t.Fatalf("ConfirmRecoveryCodes after a change: err = %v, want ErrRecoveryCodesChanged", err)
			}
			if resetRecoversKey(t, store, pending.Codes()[0], dek) {
				t.Fatal("a refused confirm stored the prepared codes")
			}
		})
	}
}

func TestRecoveryCodeStatusFlagsLegacyCodes(t *testing.T) {
	store, _ := newPasswordStore(t)
	if _, err := store.IssueRecoveryCodes("alice", nil); err != nil {
		t.Fatalf("IssueRecoveryCodes without a key: %v", err)
	}
	unused, legacy, err := store.RecoveryCodeStatus("alice")
	if err != nil || unused != RecoveryCodeCount || !legacy {
		t.Fatalf("status with legacy codes = %d, legacy %v, %v; want %d, true", unused, legacy, err, RecoveryCodeCount)
	}
}

func TestPrepareRecoveryCodesRefusesACorruptWrap(t *testing.T) {
	store, dek, oldCodes := newRotationStore(t)
	for _, wrap := range []string{"", "bm90IGEgd3JhcA=="} {
		if _, err := store.conn.Exec(`UPDATE users SET wrapped_dek_pw = ? WHERE username = 'alice'`, wrap); err != nil {
			t.Fatalf("set wrap %q: %v", wrap, err)
		}
		if _, err := store.PrepareRecoveryCodes("alice", oldPassword); !errors.Is(err, ErrPasswordWrapCorrupt) {
			t.Fatalf("PrepareRecoveryCodes with wrap %q: err = %v, want ErrPasswordWrapCorrupt", wrap, err)
		}
	}
	if !resetRecoversKey(t, store, oldCodes[0], dek) {
		t.Fatal("a refused prepare changed the stored codes")
	}
}

func TestConfirmRecoveryCodesNeedsAPreparedSet(t *testing.T) {
	store := openTestStore(t)
	if err := store.ConfirmRecoveryCodes(nil); err == nil {
		t.Fatal("ConfirmRecoveryCodes(nil): got nil, want error")
	}
}
