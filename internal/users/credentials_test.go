// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"testing"

	"gopmgr/internal/auth"
	"gopmgr/internal/crypto"
)

// otherProcess opens a second Store on store's data root, as another
// GoPMgr process would.
func otherProcess(t *testing.T, store *Store) *Store {
	t.Helper()
	other, err := Open(store.RootDir())
	if err != nil {
		t.Fatalf("open a second store: %v", err)
	}
	t.Cleanup(func() { _ = other.Close() })
	return other
}

// betweenCheckAndCommit runs change once, after a sign-in's password work
// and before its write transaction, on the first attempt only unless
// every is set.
func betweenCheckAndCommit(t *testing.T, every bool, change func()) {
	t.Helper()
	old := beforeCredentialCommit
	ran := false
	beforeCredentialCommit = func() {
		if every || !ran {
			ran = true
			change()
		}
	}
	t.Cleanup(func() { beforeCredentialCommit = old })
}

// accountCredentials is what a refused sign-in must leave untouched.
type accountCredentials struct {
	lastLogin, hash, wrapped string
	id                       string
	generation               int64
	pins, sealed, codes      int
}

func readAccountCredentials(t *testing.T, store *Store, username string) accountCredentials {
	t.Helper()
	var c accountCredentials
	if err := store.conn.QueryRow(
		`SELECT u.last_login, u.password_hash, u.wrapped_dek_pw, i.id, i.generation
		 FROM users u JOIN account_identity_registry i ON i.username = u.username WHERE u.username = ?`, username,
	).Scan(&c.lastLogin, &c.hash, &c.wrapped, &c.id, &c.generation); err != nil {
		t.Fatalf("read %s: %v", username, err)
	}
	c.pins = countRows(t, store, "escrow_pins", "username = ?", username)
	c.sealed = countRows(t, store, "sealed_deks", "username = ?", username)
	c.codes = countRows(t, store, "recovery_codes", "username = ?", username)
	return c
}

func registryEntry(t *testing.T, store *Store, username string) (string, int64) {
	t.Helper()
	var id string
	var generation int64
	if err := store.conn.QueryRow(`SELECT id, generation FROM account_identity_registry WHERE username = ?`, username).Scan(&id, &generation); err != nil {
		t.Fatalf("read %s's identity: %v", username, err)
	}
	return id, generation
}

// replaceBob purges bob and creates him again with the same password, on
// another process, and returns the new account's DEK.
func replaceBob(t *testing.T, other *Store, withKey bool) []byte {
	t.Helper()
	if err := other.PurgeAccount("alice", "bob"); err != nil {
		t.Fatalf("purge bob: %v", err)
	}
	if !withKey {
		if _, err := other.CreateAccount("bob", "Bob again", statusPassword, false); err != nil {
			t.Fatalf("create bob again: %v", err)
		}
		return nil
	}
	_, dek, err := other.CreateAccountWithKey("alice", "bob", "Bob again", statusPassword, false, nil)
	if err != nil {
		t.Fatalf("create bob again: %v", err)
	}
	t.Cleanup(func() { clear(dek) })
	return dek
}

// A sign-in whose account is purged and created again, with the same
// password, while the password is being checked writes nothing to the new
// account and returns no key: it is refused like an unknown user.
func TestSignInIsRefusedWhenTheAccountIsReplaced(t *testing.T) {
	for _, tc := range []struct {
		name    string
		withKey bool
	}{
		{"account with a key", true},
		// The old attempt has made a key for bob and would plant its wrap.
		{"account with no key yet", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, aliceDEK := escrowStore(t)
			if tc.withKey {
				addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
			} else if _, err := store.CreateAccount("bob", "Bob", statusPassword, false); err != nil {
				t.Fatalf("create bob: %v", err)
			}
			oldID, _ := registryEntry(t, store, "bob")
			other := otherProcess(t, store)
			var newDEK []byte
			var replaced accountCredentials
			var events int
			betweenCheckAndCommit(t, false, func() {
				newDEK = replaceBob(t, other, tc.withKey)
				replaced = readAccountCredentials(t, other, "bob")
				events = countRows(t, other, "account_events", "1 = 1")
			})

			_, _, dek, err := store.SignIn("bob", statusPassword)
			if !errors.Is(err, ErrNoSuchUser) || dek != nil {
				t.Fatalf("sign-in across a replacement = %x, %v; want no key and ErrNoSuchUser", dek, err)
			}
			after := readAccountCredentials(t, store, "bob")
			if after.id == oldID {
				t.Fatal("the replacement kept the old identity")
			}
			if after != replaced || after.lastLogin != "" {
				t.Fatalf("the refused sign-in wrote to the new account: %+v, was %+v", after, replaced)
			}
			if !tc.withKey && after.wrapped != "" {
				t.Fatal("the refused sign-in planted its key wrap on the new account")
			}
			if n := countRows(t, store, "account_events", "1 = 1"); n != events {
				t.Fatalf("the refused sign-in recorded %d events", n-events)
			}
			if tc.withKey {
				// The counts above would not show a sealed key overwritten
				// with the old key: open it.
				got, err := openSealedDEK(store, openEscrow(t, store, "alice", aliceDEK), "bob")
				if err != nil || !bytes.Equal(got, newDEK) {
					t.Fatalf("the new account's sealed key = %x, %v; want its own key", got, err)
				}
			}

			// The new account's own sign-in gets its own key.
			_, principal, got, err := store.SignIn("bob", statusPassword)
			if err != nil || principal.ID() != after.id {
				t.Fatalf("signing in to the new account = %v, %v", principal, err)
			}
			if tc.withKey && !bytes.Equal(got, newDEK) {
				t.Fatal("the new account's sign-in returned another key")
			}
			clear(got)
		})
	}
}

// A password changed while a sign-in checks the old one: the attempt starts
// again against the new hash, so the old password is refused and nothing
// is written.
func TestSignInWithAPasswordChangedMeanwhileIsRefused(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
	other := otherProcess(t, store)
	betweenCheckAndCommit(t, false, func() {
		if err := other.ChangePassword("bob", statusPassword, "a brand new password"); err != nil {
			t.Fatalf("change bob's password: %v", err)
		}
	})
	_, _, dek, err := store.SignIn("bob", statusPassword)
	if !errors.Is(err, auth.ErrMismatch) || dek != nil {
		t.Fatalf("sign-in with the old password = %x, %v; want auth.ErrMismatch", dek, err)
	}
	if got := readAccountCredentials(t, store, "bob"); got.lastLogin != "" {
		t.Fatal("the refused sign-in recorded a sign-in")
	}
}

// A key wrap changed meanwhile under the same password (no supported
// writer does this, but nothing else catches it): the attempt starts again
// and returns the key the stored wrap holds.
func TestSignInReturnsTheKeyOfTheCurrentWrap(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
	other := otherProcess(t, store)
	replacement, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	betweenCheckAndCommit(t, false, func() {
		wrap, err := crypto.WrapKey(replacement, statusPassword)
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, other, `UPDATE users SET wrapped_dek_pw = ? WHERE username = 'bob'`, wrap)
	})
	_, _, dek, err := store.SignIn("bob", statusPassword)
	if err != nil || !bytes.Equal(dek, replacement) {
		t.Fatalf("sign-in = %x, %v; want the key of the wrap stored now", dek, err)
	}
	clear(dek)
}

// Two sign-ins of an account with no key yet: the second to commit finds
// the first's key and returns that one, so the account has one key.
func TestConcurrentFirstSignInsShareOneKey(t *testing.T) {
	store := newDEKTestStore(t)
	other := otherProcess(t, store)
	var first []byte
	betweenCheckAndCommit(t, false, func() {
		dek, err := unlockDEK(other, "alice", "p4ssw0rd-original")
		if err != nil {
			t.Fatalf("the other sign-in: %v", err)
		}
		first = dek
	})
	dek, err := unlockDEK(store, "alice", "p4ssw0rd-original")
	if err != nil || !bytes.Equal(dek, first) {
		t.Fatalf("second sign-in = %x, %v; want the first sign-in's key %x", dek, err, first)
	}
}

// Disabled meanwhile: refused as disabled, nothing written. A role change
// meanwhile is not a credential change: the sign-in goes ahead and reports
// the role stored now.
func TestSignInRechecksTheAccountAtCommit(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
	other := otherProcess(t, store)

	betweenCheckAndCommit(t, false, func() {
		if err := other.SetDisabled("alice", "bob", true); err != nil {
			t.Fatalf("disable bob: %v", err)
		}
	})
	if _, _, dek, err := store.SignIn("bob", statusPassword); !errors.Is(err, ErrAccountDisabled) || dek != nil {
		t.Fatalf("sign-in of an account disabled meanwhile = %x, %v; want ErrAccountDisabled", dek, err)
	}
	if got := readAccountCredentials(t, store, "bob"); got.lastLogin != "" {
		t.Fatal("the refused sign-in recorded a sign-in")
	}

	if err := store.SetDisabled("alice", "bob", false); err != nil {
		t.Fatal(err)
	}
	betweenCheckAndCommit(t, false, func() {
		if err := other.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
			t.Fatalf("promote bob: %v", err)
		}
	})
	acc, principal, dek, err := store.SignIn("bob", statusPassword)
	clear(dek)
	if err != nil || !acc.IsAdmin {
		t.Fatalf("sign-in across a promotion = %+v, %v; want it to succeed as an administrator", acc, err)
	}
	if _, generation := registryEntry(t, store, "bob"); principal.Generation() != generation {
		t.Fatalf("principal generation %d, registry %d", principal.Generation(), generation)
	}
}

// The principal names the identity signed in to and the generation after
// the sign-in's own writes: storing a stronger hash and a new key each
// change it.
func TestSignInPrincipalMatchesTheRegistry(t *testing.T) {
	store := newDEKTestStore(t)
	mustExec(t, store, `UPDATE users SET password_hash = ? WHERE username = 'alice'`, weakPasswordHash("p4ssw0rd-original"))
	id, before := registryEntry(t, store, "alice")

	acc, principal, dek, err := store.SignInHeadless("alice", "p4ssw0rd-original")
	clear(dek)
	if err != nil {
		t.Fatalf("SignInHeadless: %v", err)
	}
	_, after := registryEntry(t, store, "alice")
	// The rehash and the new key are two protected changes.
	if principal.ID() != id || principal.Username() != "alice" || principal.Generation() != after || after != before+2 {
		t.Fatalf("principal %+v (identity %s, generation %d -> %d); want this identity and the generation after both changes", principal, id, before, after)
	}
	if acc.Username != "alice" {
		t.Fatalf("account = %+v", acc)
	}
	if st := readAccountKeyState(t, store, "alice"); st.wrappedDEK == "" || auth.NeedsRehash(st.passwordHash) {
		t.Fatalf("the sign-in did not store its key and stronger hash: %+v", st)
	}
}

// A sign-in that keeps finding the account changed gives up and writes
// nothing.
func TestSignInGivesUpWhenTheAccountKeepsChanging(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
	other := otherProcess(t, store)
	betweenCheckAndCommit(t, true, func() {
		hash, err := auth.HashPassword(statusPassword)
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, other, `UPDATE users SET password_hash = ? WHERE username = 'bob'`, hash)
	})
	if _, _, dek, err := store.SignIn("bob", statusPassword); !errors.Is(err, ErrSignInContended) || dek != nil {
		t.Fatalf("sign-in = %x, %v; want ErrSignInContended", dek, err)
	}
	if got := readAccountCredentials(t, store, "bob"); got.lastLogin != "" {
		t.Fatal("a contended sign-in recorded a sign-in")
	}
}

// The desktop sign-in runs the administrator's escrow session, which makes
// the escrow key; headless maintenance does not. Headless still enrolls a key
// it makes for an account that had none.
func TestOnlyTheDesktopSignInRunsTheEscrowSession(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", statusPassword, true); err != nil {
		t.Fatal(err)
	}
	_, _, dek, err := store.SignInHeadless("alice", statusPassword)
	clear(dek)
	if err != nil {
		t.Fatalf("SignInHeadless: %v", err)
	}
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 0 {
		t.Fatalf("headless sign-in made %d escrow keys", n)
	}
	_, _, dek, err = store.SignIn("alice", statusPassword)
	clear(dek)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 1 {
		t.Fatalf("the desktop sign-in made %d escrow keys, want 1", n)
	}

	aliceDEK, err := unlockDEK(store, "alice", statusPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(aliceDEK)
	if _, err := store.CreateAccount("bob", "Bob", statusPassword, false); err != nil {
		t.Fatal(err)
	}
	_, _, bobDEK, err := store.SignInHeadless("bob", statusPassword)
	if err != nil {
		t.Fatalf("SignInHeadless(bob): %v", err)
	}
	defer clear(bobDEK)
	if got, err := openSealedDEK(store, openEscrow(t, store, "alice", aliceDEK), "bob"); err != nil || !bytes.Equal(got, bobDEK) {
		t.Fatalf("bob's sealed key after headless sign-in = %x, %v; want the key it made", got, err)
	}
}

// An escrow failure rolls back only the enrollment: the sign-in and the key
// it made are kept, and the next sign-in enrolls.
func TestSignInKeepsGoingWhenEscrowFails(t *testing.T) {
	store, _ := escrowStore(t)
	if _, err := store.CreateAccount("bob", "Bob", statusPassword, false); err != nil {
		t.Fatal(err)
	}
	mustExec(t, store, `CREATE TRIGGER block_sealing BEFORE INSERT ON sealed_deks BEGIN SELECT RAISE(ABORT, 'sealing blocked'); END`)
	_, _, dek, err := store.SignIn("bob", statusPassword)
	clear(dek)
	if err != nil {
		t.Fatalf("SignIn with escrow failing: %v", err)
	}
	if got := readAccountCredentials(t, store, "bob"); got.lastLogin == "" || got.wrapped == "" || got.sealed != 0 || got.pins != 0 {
		t.Fatalf("after a failed enrollment: %+v; want the sign-in and key kept, the enrollment rolled back", got)
	}
	mustExec(t, store, `DROP TRIGGER block_sealing`)
	_, _, dek, err = store.SignIn("bob", statusPassword)
	clear(dek)
	if got := readAccountCredentials(t, store, "bob"); err != nil || got.sealed != 1 {
		t.Fatalf("the next sign-in = %v, %+v; want it enrolled", err, got)
	}
}

// Recovery codes issued with an account's password wrap that account's key
// and are stored only for the incarnation whose key they wrap.
func TestIssueRecoveryCodesWithPassword(t *testing.T) {
	t.Run("they restore the account's key", func(t *testing.T) {
		store, aliceDEK := escrowStore(t)
		bobDEK := addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
		codes, err := store.IssueRecoveryCodesWithPassword("alice", "bob", statusPassword)
		if err != nil || len(codes) == 0 {
			t.Fatalf("IssueRecoveryCodesWithPassword = %d codes, %v", len(codes), err)
		}
		if err := store.ResetWithRecoveryCode("bob", codes[0], "after-the-reset"); err != nil {
			t.Fatal(err)
		}
		if got, err := unlockDEK(store, "bob", "after-the-reset"); err != nil || !bytes.Equal(got, bobDEK) {
			t.Fatalf("key after the reset = %x, %v; want bob's key", got, err)
		}
		if _, err := store.IssueRecoveryCodesWithPassword("alice", "bob", "wrong password"); !errors.Is(err, auth.ErrMismatch) {
			t.Fatalf("with a wrong password: %v, want auth.ErrMismatch", err)
		}
	})
	t.Run("not for a replacement account", func(t *testing.T) {
		store, aliceDEK := escrowStore(t)
		addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
		other := otherProcess(t, store)
		betweenCheckAndCommit(t, false, func() { replaceBob(t, other, true) })
		if _, err := store.IssueRecoveryCodesWithPassword("alice", "bob", statusPassword); !errors.Is(err, ErrNoSuchUser) {
			t.Fatalf("across a replacement: %v, want ErrNoSuchUser", err)
		}
		if got := readAccountCredentials(t, store, "bob"); got.codes != 0 {
			t.Fatalf("codes wrapping the old key were stored for the new account: %+v", got)
		}
	})
	t.Run("roles are checked where the codes are stored", func(t *testing.T) {
		store, aliceDEK := escrowStore(t)
		addUser(t, store, "bob", openEscrow(t, store, "alice", aliceDEK))
		addUser(t, store, "dave", openEscrow(t, store, "alice", aliceDEK))
		if err := store.PromoteAdmin("alice", aliceDEK, "dave"); err != nil {
			t.Fatal(err)
		}
		other := otherProcess(t, store)
		// bob becomes an administrator after dave's request started: only
		// the super administrator may issue his codes now.
		betweenCheckAndCommit(t, false, func() {
			if err := other.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
				t.Fatal(err)
			}
		})
		if _, err := store.IssueRecoveryCodesWithPassword("dave", "bob", statusPassword); !errors.Is(err, ErrNotSuper) {
			t.Fatalf("a subordinate issuing an administrator's codes: %v, want ErrNotSuper", err)
		}
		if got := readAccountCredentials(t, store, "bob"); got.codes != 0 {
			t.Fatal("codes were stored")
		}

		// dave is disabled after his request for carol's codes started.
		addUser(t, store, "carol", openEscrow(t, store, "alice", aliceDEK))
		betweenCheckAndCommit(t, false, func() {
			if err := other.SetDisabled("alice", "dave", true); err != nil {
				t.Fatal(err)
			}
		})
		if _, err := store.IssueRecoveryCodesWithPassword("dave", "carol", statusPassword); !errors.Is(err, ErrNotAdmin) {
			t.Fatalf("a disabled administrator issuing codes: %v, want ErrNotAdmin", err)
		}
		if got := readAccountCredentials(t, store, "carol"); got.codes != 0 {
			t.Fatal("codes were stored")
		}
	})
}
