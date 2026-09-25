// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"testing"

	"gopmgr/internal/auth"
)

const (
	oldPassword = "original-password"
	newPassword = "replacement-password"
)

// newPasswordStore returns a store with alice and her DEK, unlocked with
// oldPassword.
func newPasswordStore(t *testing.T) (*Store, []byte) {
	t.Helper()
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", oldPassword, true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	dek, err := store.UnlockDEK("alice", oldPassword)
	if err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}
	return store, dek
}

// passwordRow returns alice's stored hash and password wrap.
func passwordRow(t *testing.T, store *Store) (hash, wrapped string) {
	t.Helper()
	if err := store.conn.QueryRow(
		`SELECT password_hash, wrapped_dek_pw FROM users WHERE username = 'alice'`,
	).Scan(&hash, &wrapped); err != nil {
		t.Fatalf("read password row: %v", err)
	}
	return hash, wrapped
}

func TestChangePasswordKeepsTheSameKey(t *testing.T) {
	store, dek := newPasswordStore(t)
	codes, err := store.IssueRecoveryCodes("alice", dek)
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}

	if err := store.ChangePassword("alice", oldPassword, newPassword); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if _, err := store.Authenticate("alice", oldPassword); !errors.Is(err, auth.ErrMismatch) {
		t.Fatalf("Authenticate with the old password: err = %v, want auth.ErrMismatch", err)
	}
	if _, err := store.Authenticate("alice", newPassword); err != nil {
		t.Fatalf("Authenticate with the new password: %v", err)
	}
	got, err := store.UnlockDEK("alice", newPassword)
	if err != nil {
		t.Fatalf("UnlockDEK with the new password: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("the new password unwraps a different key; encrypted projects would be lost")
	}

	// A recovery code issued before the change still recovers the same key.
	if err := store.ResetWithRecoveryCode("alice", codes[0], "after-recovery-password"); err != nil {
		t.Fatalf("ResetWithRecoveryCode: %v", err)
	}
	recovered, err := store.UnlockDEK("alice", "after-recovery-password")
	if err != nil || !bytes.Equal(recovered, dek) {
		t.Fatalf("key after a recovery reset differs (err %v)", err)
	}
}

func TestChangePasswordRefusalsChangeNothing(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		next     string
		username string
		want     error
	}{
		{"wrong current password", "not-the-password", newPassword, "alice", auth.ErrMismatch},
		{"new password too short", oldPassword, "short", "alice", ErrPasswordTooShort},
		{"empty new password", oldPassword, "", "alice", ErrPasswordTooShort},
		{"no such account", oldPassword, newPassword, "nobody", ErrNoSuchUser},
		{"invalid username", oldPassword, newPassword, "../alice", ErrNoSuchUser},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newPasswordStore(t)
			hash, wrapped := passwordRow(t, store)

			if err := store.ChangePassword(tc.username, tc.current, tc.next); !errors.Is(err, tc.want) {
				t.Fatalf("ChangePassword: err = %v, want %v", err, tc.want)
			}
			if h, w := passwordRow(t, store); h != hash || w != wrapped {
				t.Fatal("a refused change modified the stored password or wrap")
			}
			if _, err := store.Authenticate("alice", oldPassword); err != nil {
				t.Fatalf("old password stopped working after a refusal: %v", err)
			}
		})
	}
}

// TestChangePasswordWorksWithAnOlderHashAndLeavesLastLogin covers an
// account whose hash uses weaker Argon2id parameters: going through
// Authenticate would re-hash it and break the compare-and-swap.
func TestChangePasswordWorksWithAnOlderHashAndLeavesLastLogin(t *testing.T) {
	store, dek := newPasswordStore(t)
	if _, err := store.conn.Exec(
		`UPDATE users SET password_hash = ?, last_login = '2026-01-01T00:00:00Z' WHERE username = 'alice'`,
		weakPasswordHash(oldPassword),
	); err != nil {
		t.Fatalf("store weak hash: %v", err)
	}

	if err := store.ChangePassword("alice", oldPassword, newPassword); err != nil {
		t.Fatalf("ChangePassword with an older hash: %v", err)
	}
	if got := lastLogin(t, store, "alice"); got != "2026-01-01T00:00:00Z" {
		t.Fatalf("last_login = %q, want it unchanged", got)
	}
	got, err := store.UnlockDEK("alice", newPassword)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("key after change differs (err %v)", err)
	}
}

// TestChangePasswordDoesNotOverwriteAConcurrentChange changes the password
// between ChangePassword's read and its write.
func TestChangePasswordDoesNotOverwriteAConcurrentChange(t *testing.T) {
	store, dek := newPasswordStore(t)
	previous := beforePasswordSwap
	t.Cleanup(func() { beforePasswordSwap = previous })
	beforePasswordSwap = func() {
		beforePasswordSwap = func() {}
		if err := store.ChangePassword("alice", oldPassword, "changed-elsewhere"); err != nil {
			t.Errorf("concurrent ChangePassword: %v", err)
		}
	}

	if err := store.ChangePassword("alice", oldPassword, newPassword); !errors.Is(err, ErrPasswordChangedElsewhere) {
		t.Fatalf("ChangePassword racing another change: err = %v, want ErrPasswordChangedElsewhere", err)
	}
	if _, err := store.Authenticate("alice", "changed-elsewhere"); err != nil {
		t.Fatalf("the other change was overwritten: %v", err)
	}
	if _, err := store.Authenticate("alice", newPassword); !errors.Is(err, auth.ErrMismatch) {
		t.Fatalf("the refused change took effect: err = %v", err)
	}
	got, err := store.UnlockDEK("alice", "changed-elsewhere")
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("key after the other change differs (err %v)", err)
	}
}

func TestChangePasswordReportsACorruptWrap(t *testing.T) {
	store, _ := newPasswordStore(t)
	if _, err := store.conn.Exec(`UPDATE users SET wrapped_dek_pw = 'bm90IGEgd3JhcA==' WHERE username = 'alice'`); err != nil {
		t.Fatalf("corrupt wrap: %v", err)
	}
	hash, wrapped := passwordRow(t, store)
	if err := store.ChangePassword("alice", oldPassword, newPassword); !errors.Is(err, ErrPasswordWrapCorrupt) {
		t.Fatalf("ChangePassword with a corrupt wrap: err = %v, want ErrPasswordWrapCorrupt", err)
	}
	if h, w := passwordRow(t, store); h != hash || w != wrapped {
		t.Fatal("a refused change modified the stored password or wrap")
	}
}

func TestCreateAccountAsRefusesAShortPassword(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccountAs("", "alice", "Alice", "short", false); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("CreateAccountAs with a short password: err = %v, want ErrPasswordTooShort", err)
	}
	assertNoAccountFolder(t, store, "alice")
}
