// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"errors"
	"reflect"
	"testing"

	"gopmgr/internal/auth"
)

// countUnknownAccountChecks counts verifyUnknownAccount's calls, still
// doing the work, until the test ends.
func countUnknownAccountChecks(t *testing.T) *int {
	t.Helper()
	requireSerial(t)
	calls := new(int)
	old := verifyUnknownAccount
	verifyUnknownAccount = func(password string) {
		*calls++
		old(password)
	}
	t.Cleanup(func() { verifyUnknownAccount = old })
	return calls
}

// The hook the counting tests replace must be the real check, or they
// would pass with sign-in failing fast.
func TestUnknownAccountCheckIsTheAuthCheck(t *testing.T) {
	got := reflect.ValueOf(verifyUnknownAccount).Pointer()
	want := reflect.ValueOf(auth.VerifyUnknownAccount).Pointer()
	if got != want {
		t.Fatal("verifyUnknownAccount is not auth.VerifyUnknownAccount")
	}
}

// A failed sign-in does one password check whether or not the username
// exists: a real one for a known name, verifyUnknownAccount for any other.
// Counting the calls proves the work is done without timing the test.
func TestSignInSpendsAPasswordCheckOnAnUnknownName(t *testing.T) {
	store, _ := escrowStore(t)
	calls := countUnknownAccountChecks(t)

	cases := []struct {
		name      string
		username  string
		password  string
		wantErr   error
		wantCalls int
	}{
		{"unknown name", "nobody", statusPassword, ErrNoSuchUser, 1},
		{"name that cannot exist", "no such/name", statusPassword, ErrNoSuchUser, 1},
		{"empty name", "", "", ErrNoSuchUser, 1},
		{"wrong password", "alice", "wrong-password", auth.ErrMismatch, 0},
		{"right password", "alice", statusPassword, nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			*calls = 0
			_, _, dek, err := store.SignInHeadless(tc.username, tc.password)
			clear(dek)
			if !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) {
				t.Fatalf("SignIn(%q) error = %v, want %v", tc.username, err, tc.wantErr)
			}
			if *calls != tc.wantCalls {
				t.Fatalf("SignIn(%q) checked the unknown-account hash %d times, want %d", tc.username, *calls, tc.wantCalls)
			}
		})
	}
}

// A recovery-code reset with a code that matches nothing makes
// RecoveryCodeCount checks whatever the name: the account's unused codes
// and, for the rest, the unknown-account check.
func TestRecoveryResetSpendsAFullSetOfChecks(t *testing.T) {
	store, _, codes := storeFromTemplate(t, templates.rotation)
	calls := countUnknownAccountChecks(t)
	const newPassword = "new-passphrase-long"
	steps := []struct {
		name      string
		username  string
		code      string
		password  string
		wantErr   error
		wantCalls int
	}{
		{"unknown name", "nobody", "AAAA-BBBB-CCCC", newPassword, ErrInvalidRecoveryCode, RecoveryCodeCount},
		{"name that cannot exist", "no such/name", "AAAA-BBBB-CCCC", newPassword, ErrInvalidRecoveryCode, RecoveryCodeCount},
		{"wrong code, full set", "alice", "AAAA-BBBB-CCCC", newPassword, ErrInvalidRecoveryCode, 0},
		{"weak new password, impossible name", "no such/name", "AAAA-BBBB-CCCC", "short", nil, 0},
		{"right code", "alice", codes[0], newPassword, nil, 0},
		{"wrong code, one used", "alice", "AAAA-BBBB-CCCC", newPassword, ErrInvalidRecoveryCode, 1},
	}
	for _, s := range steps {
		*calls = 0
		err := store.ResetWithRecoveryCode(s.username, s.code, s.password)
		switch {
		case s.wantErr != nil && !errors.Is(err, s.wantErr):
			t.Fatalf("%s: error = %v, want %v", s.name, err, s.wantErr)
		case s.wantErr == nil && s.password == newPassword && err != nil:
			t.Fatalf("%s: error = %v, want success", s.name, err)
		case s.password != newPassword && (err == nil || errors.Is(err, ErrInvalidRecoveryCode)):
			t.Fatalf("%s: error = %v, want the password refused", s.name, err)
		}
		if *calls != s.wantCalls {
			t.Fatalf("%s: %d unknown-account checks, want %d", s.name, *calls, s.wantCalls)
		}
	}
}
