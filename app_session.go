// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"pmforge/internal/auth"
	"pmforge/internal/users"
	"strings"
)

// =========================================================
// Accounts & session
// =========================================================

// ListUsers returns every account on the machine. Used by the login
// screen if you want a user-picker variant later.
func (a *App) ListUsers() ([]users.Account, error) {
	return a.store.List()
}

// HasAnyAdmin reports whether at least one administrator account exists.
// Safe to call without signing in — used by the login and account-
// creation screens to decide whether to show the admin claim prompt.
func (a *App) HasAnyAdmin() (bool, error) {
	return a.store.HasAnyAdmin()
}

// CreateAccount provisions a new user and signs the new user in.
//
// isAdmin marks the account as an administrator. The call is gated:
//   - If no admin exists yet: any caller may create an account with any
//     role (first-user bootstrap).
//   - If an admin already exists: the caller must be signed in as an
//     admin. Non-admin or unauthenticated callers receive an error.
//
// Returns the account record (no password material).
func (a *App) CreateAccount(username, displayName, password string, isAdmin bool) (users.Account, error) {
	hasAdmin, err := a.store.HasAnyAdmin()
	if err != nil {
		return users.Account{}, err
	}
	if hasAdmin {
		// An admin already exists — only admins may create new accounts.
		caller := a.requireUser()
		if caller == nil || !caller.IsAdmin {
			return users.Account{}, errors.New("account creation requires administrator privileges")
		}
	}
	acc, err := a.store.CreateAccount(username, displayName, password, isAdmin)
	if err != nil {
		return users.Account{}, err
	}
	// ADR-001: unlock (here: lazily create) the per-user DEK while we
	// hold the verified password — the only moment that is possible.
	dek, err := a.store.UnlockDEK(username, password)
	if err != nil {
		return users.Account{}, err
	}
	// Only auto-sign-in when no one is currently logged in (i.e. this
	// is a self-registration or the first-user case). When an admin
	// creates an account on behalf of another user, the admin session
	// remains active.
	a.mu.Lock()
	if a.user == nil {
		a.user = &acc
		a.dek = dek
	}
	a.mu.Unlock()
	return acc, nil
}

// BecomeAdmin promotes the currently signed-in user to administrator,
// but only if no administrator account exists yet. Once any admin
// exists this method returns an error — use AdminSetUserRole instead.
func (a *App) BecomeAdmin() error {
	caller := a.requireUser()
	if caller == nil {
		return errors.New("not signed in")
	}
	hasAdmin, err := a.store.HasAnyAdmin()
	if err != nil {
		return err
	}
	if hasAdmin {
		return errors.New("an administrator already exists; ask them to grant you admin rights")
	}
	return a.store.SetAdmin(caller.Username, true)
}

// AdminListUsers returns every account, including admin status. Requires
// the caller to be an administrator.
func (a *App) AdminListUsers() ([]users.Account, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return nil, errors.New("administrator privileges required")
	}
	return a.store.List()
}

// AdminDeleteUser removes an account. Requires the caller to be an
// administrator. Callers cannot delete their own account.
func (a *App) AdminDeleteUser(username string) error {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	if strings.EqualFold(caller.Username, username) {
		return errors.New("administrators cannot delete their own account")
	}
	return a.store.DeleteAccount(username)
}

// AdminSetUserRole promotes or demotes a user's administrator status.
// Requires the caller to be an administrator. Callers cannot change
// their own role (to prevent accidental self-demotion).
func (a *App) AdminSetUserRole(username string, isAdmin bool) error {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	if strings.EqualFold(caller.Username, username) {
		return errors.New("administrators cannot change their own role")
	}
	return a.store.SetAdmin(username, isAdmin)
}

// AdminIssueRecoveryCodes issues one-time recovery codes for the named
// account and returns them for the administrator to hand to the user.
// Requires the caller to be an administrator and the account's current
// password (the one the admin set at creation) so the codes wrap the user's
// data-encryption key — giving an admin-created account the same recovery
// footing as a self-registered one. This adds no new password oracle: Login
// already verifies passwords for any username at the same Argon2id cost.
func (a *App) AdminIssueRecoveryCodes(username, password string) ([]string, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return nil, errors.New("administrator privileges required")
	}
	// Unlock (here: unwrap the just-created) DEK so the codes can wrap it.
	dek, err := a.store.UnlockDEK(username, password)
	if err != nil {
		return nil, errors.New("could not unlock the account's key (wrong password?)")
	}
	return a.store.IssueRecoveryCodes(username, dek)
}

// Login authenticates and stores the user as the active session.
// Returns a generic error on bad credentials — the message is shaped
// by the frontend so usernames cannot be enumerated by error
// inspection.
func (a *App) Login(username, password string) (users.Account, error) {
	acc, err := a.store.Authenticate(username, password)
	if err != nil {
		// Collapse both "no such user" and "password mismatch" into
		// one error so the timing/message is identical.
		if errors.Is(err, users.ErrNoSuchUser) || errors.Is(err, auth.ErrMismatch) {
			return users.Account{}, errors.New("invalid credentials")
		}
		return users.Account{}, err
	}
	// ADR-001: unlock the per-user DEK with the verified password.
	// Lazy generation covers accounts that predate the key hierarchy.
	dek, err := a.store.UnlockDEK(username, password)
	if err != nil {
		return users.Account{}, err
	}
	a.mu.Lock()
	a.user = &acc
	a.dek = dek
	a.mu.Unlock()
	return acc, nil
}

// IssueRecoveryCodes generates 8 fresh recovery codes for the
// currently-signed-in user and returns the plaintext codes ONCE.
// The GUI MUST show them to the user immediately and warn that they
// will not be visible again — only their Argon2id hashes are
// persisted.
//
// Calling this rotates the user's existing unused codes.
func (a *App) IssueRecoveryCodes() ([]string, error) {
	u := a.requireUser()
	if u == nil {
		return nil, errors.New("not signed in")
	}
	// ADR-001: wrap the session DEK into every code so a recovery
	// reset can re-wrap the same DEK (encrypted projects survive).
	// requireDEKLocked returns a deep copy, preventing a concurrent
	// Logout from zeroing the backing array mid-wrap.
	a.mu.RLock()
	dek, _ := a.requireDEKLocked() // nil if encryption not yet enabled; valid
	a.mu.RUnlock()
	return a.store.IssueRecoveryCodes(u.Username, dek)
}

// RemainingRecoveryCodes returns the count of unused recovery codes
// for the active user. The GUI nags at 0 or 1.
func (a *App) RemainingRecoveryCodes() (int, error) {
	u := a.requireUser()
	if u == nil {
		return 0, errors.New("not signed in")
	}
	return a.store.RemainingRecoveryCodes(u.Username)
}

// ResetWithRecoveryCode is the "forgot password" flow. It does NOT
// require an active session — the user lands on the login screen,
// clicks "use a recovery code", enters username + code + new
// password, and we verify + rotate atomically.
func (a *App) ResetWithRecoveryCode(username, code, newPassword string) error {
	return a.store.ResetWithRecoveryCode(username, code, newPassword)
}

// Logout clears the active session and closes any open project.
func (a *App) Logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
		a.dbPath = ""
		a.adminSvc = nil
	}
	a.user = nil
	// ADR-001: zero the session DEK before dropping it.
	for i := range a.dek {
		a.dek[i] = 0
	}
	a.dek = nil
	return nil
}

// CurrentUser returns the active session or nil. Used by the GUI on
// initial mount to skip the login screen if we already have a user.
func (a *App) CurrentUser() *users.Account {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.user
}
