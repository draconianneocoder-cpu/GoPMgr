// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const rulePassword = "passphrase-long"

// accountRoles returns every stored account's admin flag by username.
func accountRoles(t *testing.T, store *Store) map[string]bool {
	t.Helper()
	accs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	roles := make(map[string]bool, len(accs))
	for _, acc := range accs {
		roles[acc.Username] = acc.IsAdmin
	}
	return roles
}

func assertNoAccountFolder(t *testing.T, store *Store, username string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(store.RootDir(), username)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("folder for refused account %q: stat err = %v, want not exist", username, err)
	}
}

// holdAccountRuleWindow pauses every CreateAccountAs between its rule
// check and its insert. Without the write lock, every racing caller passes
// the check during the pause; with it, the others wait at BEGIN IMMEDIATE.
func holdAccountRuleWindow(t *testing.T) {
	t.Helper()
	previous := afterAccountRuleCheck
	afterAccountRuleCheck = func() { time.Sleep(300 * time.Millisecond) }
	t.Cleanup(func() { afterAccountRuleCheck = previous })
}

func TestHasAnyAccount(t *testing.T) {
	store := openTestStore(t)
	if got, err := store.HasAnyAccount(); err != nil || got {
		t.Fatalf("HasAnyAccount on empty store = %v, %v; want false, nil", got, err)
	}
	if _, err := store.CreateAccount("alice", "Alice", rulePassword, false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if got, err := store.HasAnyAccount(); err != nil || !got {
		t.Fatalf("HasAnyAccount with one account = %v, %v; want true, nil", got, err)
	}
}

func TestCreateAccountAsMakesFirstAccountAdmin(t *testing.T) {
	store := openTestStore(t)
	acc, err := store.CreateAccountAs("", "alice", "Alice", rulePassword, false)
	if err != nil {
		t.Fatalf("CreateAccountAs: %v", err)
	}
	if !acc.IsAdmin {
		t.Fatal("returned first account is not an administrator")
	}
	if roles := accountRoles(t, store); !roles["alice"] {
		t.Fatalf("stored roles = %v, want alice as administrator", roles)
	}
}

func TestCreateAccountAsRefusesNonAdminCallers(t *testing.T) {
	for _, state := range []struct {
		name      string
		seedAdmin bool
	}{
		{"with an administrator", true},
		// Installs whose accounts predate the first-account rule.
		{"with accounts but no administrator", false},
	} {
		for _, caller := range []string{"", "bob", "nobody"} {
			t.Run(fmt.Sprintf("%s/caller %q", state.name, caller), func(t *testing.T) {
				store := openTestStore(t)
				if _, err := store.CreateAccount("alice", "Alice", rulePassword, state.seedAdmin); err != nil {
					t.Fatalf("seed alice: %v", err)
				}
				if _, err := store.CreateAccount("bob", "Bob", rulePassword, false); err != nil {
					t.Fatalf("seed bob: %v", err)
				}

				_, err := store.CreateAccountAs(caller, "eve", "Eve", rulePassword, true)
				if !errors.Is(err, ErrNotAdmin) {
					t.Fatalf("CreateAccountAs by %q: err = %v, want ErrNotAdmin", caller, err)
				}
				if roles := accountRoles(t, store); len(roles) != 2 {
					t.Fatalf("accounts after refusal = %v, want only alice and bob", roles)
				}
				assertNoAccountFolder(t, store, "eve")
			})
		}
	}
}

func TestCreateAccountAsHonorsRoleForAdminCaller(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccountAs("", "alice", "Alice", rulePassword, false); err != nil {
		t.Fatalf("first account: %v", err)
	}
	if acc, err := store.CreateAccountAs("alice", "bob", "Bob", rulePassword, false); err != nil || acc.IsAdmin {
		t.Fatalf("standard account = %+v, %v; want a non-admin account", acc, err)
	}
	if acc, err := store.CreateAccountAs("alice", "carol", "Carol", rulePassword, true); err != nil || !acc.IsAdmin {
		t.Fatalf("admin account = %+v, %v; want an administrator", acc, err)
	}
	want := map[string]bool{"alice": true, "bob": false, "carol": true}
	if got := accountRoles(t, store); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stored roles = %v, want %v", got, want)
	}
}

// TestCreateAccountAsReadsCallerRoleFromStore proves the caller's role is
// read when the account is created, not trusted from an earlier moment.
func TestCreateAccountAsReadsCallerRoleFromStore(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccountAs("", "alice", "Alice", rulePassword, false); err != nil {
		t.Fatalf("first account: %v", err)
	}
	if _, err := store.CreateAccountAs("alice", "bob", "Bob", rulePassword, false); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	if err := store.SetAdmin("bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	if _, err := store.CreateAccountAs("bob", "carol", "Carol", rulePassword, false); err != nil {
		t.Fatalf("promoted caller refused: %v", err)
	}

	if err := store.SetAdmin("bob", false); err != nil {
		t.Fatalf("demote bob: %v", err)
	}
	if _, err := store.CreateAccountAs("bob", "dave", "Dave", rulePassword, false); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("demoted caller: err = %v, want ErrNotAdmin", err)
	}
	assertNoAccountFolder(t, store, "dave")
}

// TestCreateAccountAsConcurrentFirstAccounts races several unauthenticated
// callers on an empty store: exactly one may take the first-account path.
func TestCreateAccountAsConcurrentFirstAccounts(t *testing.T) {
	const contenders = 3
	store := openTestStore(t)
	holdAccountRuleWindow(t)

	start := make(chan struct{})
	errs := make([]error, contenders)
	var wg sync.WaitGroup
	for i := range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = store.CreateAccountAs("", fmt.Sprintf("user%d", i), "", rulePassword, false)
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrNotAdmin):
			assertNoAccountFolder(t, store, fmt.Sprintf("user%d", i))
		default:
			t.Fatalf("contender %d: unexpected error %v", i, err)
		}
	}
	roles := accountRoles(t, store)
	if winners != 1 || len(roles) != 1 {
		t.Fatalf("winners = %d, accounts = %v; want exactly one account", winners, roles)
	}
	for name, isAdmin := range roles {
		if !isAdmin {
			t.Fatalf("sole account %q is not an administrator", name)
		}
	}
}

// TestCreateAccountAsConcurrentCaseVariants races "Carol" against "carol":
// the case-insensitive duplicate check runs inside the same transaction as
// the insert, so only one can be created.
func TestCreateAccountAsConcurrentCaseVariants(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccountAs("", "alice", "Alice", rulePassword, false); err != nil {
		t.Fatalf("first account: %v", err)
	}
	holdAccountRuleWindow(t)

	names := []string{"Carol", "carol", "CAROL"}
	start := make(chan struct{})
	errs := make([]error, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = store.CreateAccountAs("alice", name, "", rulePassword, false)
		}()
	}
	close(start)
	wg.Wait()

	created := 0
	for i, err := range errs {
		switch {
		case err == nil:
			created++
		case errors.Is(err, ErrUserExists):
		default:
			t.Fatalf("%s: unexpected error %v", names[i], err)
		}
	}
	if roles := accountRoles(t, store); created != 1 || len(roles) != 2 {
		t.Fatalf("created = %d, accounts = %v; want alice plus one Carol", created, roles)
	}
}
