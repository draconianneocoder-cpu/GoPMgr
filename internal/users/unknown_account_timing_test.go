// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"errors"
	"testing"

	"gopmgr/internal/auth"
)

// A failed sign-in does one password check whether or not the username
// exists: a real one for a known name, verifyUnknownAccount for any other.
// Counting the calls proves the work is done without timing the test.
func TestSignInSpendsAPasswordCheckOnAnUnknownName(t *testing.T) {
	store, _ := escrowStore(t)
	requireSerial(t)
	calls := 0
	old := verifyUnknownAccount
	verifyUnknownAccount = func(password string) {
		calls++
		old(password)
	}
	t.Cleanup(func() { verifyUnknownAccount = old })

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
			calls = 0
			_, _, dek, err := store.SignInHeadless(tc.username, tc.password)
			clear(dek)
			if !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) {
				t.Fatalf("SignIn(%q) error = %v, want %v", tc.username, err, tc.wantErr)
			}
			if calls != tc.wantCalls {
				t.Fatalf("SignIn(%q) checked the unknown-account hash %d times, want %d", tc.username, calls, tc.wantCalls)
			}
		})
	}
}
