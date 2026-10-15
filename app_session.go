// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"gopmgr/internal/auth"
	"gopmgr/internal/users"
	"log"
	"strings"
)

// =========================================================
// Accounts & session
// =========================================================

// HasAnyAdmin reports whether at least one administrator account exists.
// Safe to call without signing in — used by App Settings to decide
// whether to offer the administrator claim.
func (a *App) HasAnyAdmin() (bool, error) {
	return a.store.HasAnyAdmin()
}

// AccountSetupWire tells the sign-in screen which first-run state this
// machine is in. It carries no account names.
type AccountSetupWire struct {
	HasAccounts bool `json:"has_accounts"`
	HasAdmin    bool `json:"has_admin"`
}

// AccountSetup reports whether any account and any administrator exist.
// Safe to call without signing in: the sign-in screen offers account
// creation only when there are no accounts, and points an install whose
// accounts predate the administrator role to Become administrator.
func (a *App) AccountSetup() (AccountSetupWire, error) {
	hasAccounts, err := a.store.HasAnyAccount()
	if err != nil {
		return AccountSetupWire{}, err
	}
	hasAdmin, err := a.store.HasAnyAdmin()
	if err != nil {
		return AccountSetupWire{}, err
	}
	return AccountSetupWire{HasAccounts: hasAccounts, HasAdmin: hasAdmin}, nil
}

// CreateAccount provisions a new user, signing them in only when nobody
// is signed in.
//
// The rule is users.Store.CreateAccountAs's: on a machine with no
// accounts, anyone may create the first account and it is always an
// administrator (isAdmin is ignored). After that, only a signed-in
// administrator may create accounts, and isAdmin is honored. Earlier
// versions let anyone create accounts while no administrator existed,
// which let a second person make themselves administrator whenever the
// first skipped the admin option.
//
// Returns the account record (no password material).
func (a *App) CreateAccount(username, displayName, password string, isAdmin bool) (users.Account, error) {
	s := a.currentSession()
	callerUsername := ""
	var escrow *users.EscrowSession
	if s.user != nil {
		callerUsername = s.user.Username
		if s.user.IsAdmin {
			escrow = a.openSessionEscrow(s)
		}
	}
	defer escrow.Close()
	// ADR-001: the account's DEK is created with it, wrapped under the
	// password, and returned here. ADR-004: it is enrolled in escrow in the
	// same transaction, with the creating administrator's escrow key.
	acc, principal, dek, err := a.store.CreateAccountWithPrincipal(callerUsername, username, displayName, password, isAdmin, escrow)
	if errors.Is(err, users.ErrNotAdmin) {
		return users.Account{}, errors.New("account creation requires administrator privileges")
	}
	if errors.Is(err, users.ErrNotSuper) {
		return users.Account{}, errors.New("only the super administrator can create administrator accounts")
	}
	if errors.Is(err, users.ErrUserFolderExists) {
		return users.Account{}, fmt.Errorf("a folder for %q is left over in GoPMgr's data folder (%s); remove or rename it, or choose another username", username, a.store.RootDir())
	}
	if errors.Is(err, users.ErrReservedUsername) {
		return users.Account{}, fmt.Errorf("%q is reserved for GoPMgr's own files; choose another username", username)
	}
	if errors.Is(err, users.ErrWindowsDeviceName) {
		return users.Account{}, fmt.Errorf("%q is a name Windows keeps for devices, so it can't be a folder; choose another username", username)
	}
	if errors.Is(err, users.ErrPasswordTooShort) {
		return users.Account{}, fmt.Errorf("password must be at least %d characters", users.MinPasswordLength)
	}
	if err != nil {
		return users.Account{}, err
	}
	if s.user != nil {
		// An administrator created it; their session stays, and the new
		// user's DEK is zeroed rather than dropped (ADR-001).
		zeroBytes(dek)
		return acc, nil
	}
	// The first account: it creates the escrow key (ADR-004), and it is
	// signed in with the identity and key its creation returned, unless the
	// session changed meanwhile.
	a.enrollEscrow(acc.Username, dek)
	sessionGap("first account")
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.epoch != s.epoch {
		zeroBytes(dek)
		return users.Account{}, errors.New("your account was created, but signing in was interrupted because the session changed; sign in with your new password")
	}
	a.setSessionLocked(acc, principal, dek)
	return acc, nil
}

// BecomeAdmin promotes the currently signed-in user to administrator,
// but only if no administrator account exists yet. Once any admin
// exists this method returns an error — use AdminSetUserRole instead.
func (a *App) BecomeAdmin() error {
	s := a.currentSession()
	caller := s.user
	if caller == nil {
		return errors.New("not signed in")
	}
	err := a.store.ClaimAdmin(caller.Username)
	if errors.Is(err, users.ErrAdminExists) {
		return errors.New("an administrator already exists; ask them to grant you admin rights")
	}
	if err != nil {
		return err
	}
	// The first administrator creates the escrow key (ADR-004).
	_ = a.withSessionDEK(s, func(dek []byte) error {
		a.enrollEscrow(caller.Username, dek)
		return nil
	})
	// Admin-only methods read the role from the session, so update it here
	// or the new administrator would be refused until they sign in again,
	// but only in the session that claimed it. Replace the account rather
	// than editing it: requireUser hands the same pointer to other
	// goroutines.
	sessionGap("become admin")
	a.mu.Lock()
	if a.user != nil && a.epoch == s.epoch {
		promoted := *a.user
		promoted.IsAdmin = true
		a.user = &promoted
	}
	a.mu.Unlock()
	return nil
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

// AdminSetUserDisabled disables or enables another account. A disabled
// account keeps its projects and encryption key but cannot sign in.
// Requires an administrator; administrators cannot disable themselves.
func (a *App) AdminSetUserDisabled(username string, disabled bool) error {
	s := a.currentSession()
	caller := s.user
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	if strings.EqualFold(caller.Username, username) {
		return errors.New("administrators cannot disable their own account")
	}
	err := a.store.SetDisabled(caller.Username, username, disabled)
	if err == nil && disabled {
		a.runPendingRotation(s)
	}
	if msg := superRoleMessage(err); msg != nil {
		return msg
	}
	if errors.Is(err, users.ErrNotAdmin) {
		return errors.New("administrator privileges required")
	}
	if errors.Is(err, users.ErrLastAdmin) {
		return errors.New("this is the only administrator who can sign in; make someone else an administrator first")
	}
	return err
}

// superRoleMessage words the super administrator's refusals (ADR-005), or
// returns nil for any other error.
func superRoleMessage(err error) error {
	var protected *users.ProtectionError
	switch {
	case errors.Is(err, users.ErrNotSuper):
		return errors.New("only the super administrator can change administrators or open users' data")
	case errors.Is(err, users.ErrTargetIsSuper):
		return errors.New("the super administrator can't be changed; they must hand the role to another administrator first")
	case errors.As(err, &protected):
		return fmt.Errorf("%s is protected until %s after the takeover (%s), so they can't be disabled, deleted, or removed as an administrator until then",
			protected.Username, protected.Until.Local().Format("January 2, 2006"), protected.Reason)
	}
	return nil
}

// AdminPurgeUser permanently deletes another account and its folder: its
// projects (encrypted or not), certificates, exports, and recovery codes.
// confirmation must be the username exactly as stored, typed by the
// administrator. Requires an administrator; administrators cannot delete
// themselves.
func (a *App) AdminPurgeUser(username, confirmation string) error {
	s := a.currentSession()
	caller := s.user
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	if strings.EqualFold(caller.Username, username) {
		return errors.New("administrators cannot delete their own account")
	}
	if confirmation != username {
		return errors.New("type the username exactly to confirm permanent deletion")
	}
	err := a.store.PurgeAccount(caller.Username, username)
	if err == nil || errors.Is(err, users.ErrPurgeIncomplete) {
		a.runPendingRotation(s)
	}
	if msg := superRoleMessage(err); msg != nil {
		return msg
	}
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return errors.New("administrator privileges required")
	case errors.Is(err, users.ErrFolderShared):
		return fmt.Errorf("another account's name differs from %q only in letter case, so the two may share a folder; disable the account instead", username)
	case errors.Is(err, users.ErrLastAdmin):
		return errors.New("this is the only administrator who can sign in; make someone else an administrator first")
	case errors.Is(err, users.ErrReservedUsername):
		return fmt.Errorf("%q's folder is GoPMgr's own log folder, so it cannot be deleted; disable the account instead", username)
	case errors.Is(err, users.ErrPurgeIncomplete):
		return fmt.Errorf("the account was deleted, but part of its folder could not be removed; remove it by hand (%v)", err)
	}
	return err
}

// AdminListAccountEvents returns the history of disabled, enabled, and
// deleted accounts, newest first. Requires an administrator.
func (a *App) AdminListAccountEvents() ([]users.AccountEvent, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return nil, errors.New("administrator privileges required")
	}
	return a.store.AccountEvents()
}

// AdminSetUserRole promotes or demotes a user's administrator status.
// Requires the caller to be an administrator. Callers cannot change
// their own role (to prevent accidental self-demotion).
func (a *App) AdminSetUserRole(username string, isAdmin bool) error {
	s := a.currentSession()
	caller := s.user
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	if strings.EqualFold(caller.Username, username) {
		return errors.New("administrators cannot change their own role")
	}
	var err error
	if isAdmin {
		// A new administrator is a subordinate and gets no key (ADR-005).
		err = a.store.PromoteAdmin(caller.Username, nil, username)
	} else {
		err = a.store.SetAdmin(caller.Username, username, isAdmin)
		if err == nil {
			a.runPendingRotation(s)
		}
	}
	if msg := superRoleMessage(err); msg != nil {
		return msg
	}
	if errors.Is(err, users.ErrNotAdmin) {
		return errors.New("administrator privileges required")
	}
	if errors.Is(err, users.ErrNoPersonalKey) {
		return fmt.Errorf("%s must sign in once before they can be made an administrator", username)
	}
	return err
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
	// An administrator's codes are the super administrator's to issue
	// (ADR-005).
	if target, err := a.accountByName(username); err == nil && target.IsAdmin {
		if err := a.store.RequireSuper(caller.Username); err != nil {
			if msg := superRoleMessage(err); msg != nil {
				return nil, msg
			}
			return nil, errors.New("administrator privileges required")
		}
	}
	// The password unlocks the account's DEK so the codes can wrap it; the
	// roles and the account are checked again where the codes are stored.
	codes, err := a.store.IssueRecoveryCodesWithPassword(caller.Username, username, password)
	switch {
	case errors.Is(err, users.ErrNoSuchUser), errors.Is(err, auth.ErrMismatch), errors.Is(err, users.ErrPasswordWrapCorrupt):
		return nil, errors.New("could not unlock the account's key (wrong password?)")
	case errors.Is(err, users.ErrNotAdmin):
		return nil, errors.New("administrator privileges required")
	case errors.Is(err, users.ErrSignInContended):
		return nil, errors.New("the account changed while its key was being unlocked; try again")
	}
	if msg := superRoleMessage(err); msg != nil {
		return nil, msg
	}
	return codes, err
}

// Login authenticates and stores the user as the active session.
// Returns a generic error on bad credentials — the message is shaped
// by the frontend so usernames cannot be enumerated by error
// inspection.
//
// users.Store.SignIn checks the password, unlocks the DEK, records the
// sign-in, and runs the escrow session for one account incarnation, so a
// username purged and created again while the password is being checked
// is never signed in to with the old attempt's key or writes.
//
// The session is published only if no sign-out or other sign-in finished
// while the password was being checked (session_state.go): a late sign-in
// never overtakes one. Refuses if a project is already open, so that
// project is never left live under another user's DEK. Not reachable
// through the shipped frontend (sign-out always calls Logout first), but
// checked in the same critical section as the assignment it guards, after
// the slow password work, so a concurrent OpenProject cannot slip in
// between. A refused call has already recorded the sign-in and derived the
// DEK, which is zeroed at once.
func (a *App) Login(username, password string) (users.Account, error) {
	s := a.currentSession()
	acc, principal, dek, err := a.store.SignIn(username, password)
	if err != nil {
		// Collapse both "no such user" and "password mismatch" into
		// one error so the timing/message is identical.
		if errors.Is(err, users.ErrNoSuchUser) || errors.Is(err, auth.ErrMismatch) {
			return users.Account{}, errors.New("invalid credentials")
		}
		// Only returned after the password matched, so it tells nothing
		// to someone who does not know it.
		if errors.Is(err, users.ErrAccountDisabled) {
			return users.Account{}, errors.New("this account is disabled; ask your administrator to enable it")
		}
		if errors.Is(err, users.ErrSignInContended) {
			return users.Account{}, errors.New("the account changed while signing in; try again")
		}
		return users.Account{}, err
	}
	sessionGap("sign-in")
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.epoch != s.epoch {
		// A sign-out or another sign-in finished while this one was
		// checking the password; it wins.
		zeroBytes(dek)
		return users.Account{}, errors.New("signing in was interrupted because you signed out or another account signed in; try again")
	}
	if a.db != nil {
		zeroBytes(dek)
		return users.Account{}, errors.New("a project is already open; close it before signing in as a different user")
	}
	a.setSessionLocked(acc, principal, dek)
	return acc, nil
}

// enrollEscrow enrolls a signed-in account in administrator escrow
// (ADR-004) and, for an administrator, does the escrow checks listed on
// users.Store.EnrollAdminSession. A failure never stops the sign-in: it is
// logged, and the next sign-in tries again.
func (a *App) enrollEscrow(username string, dek []byte) {
	if err := a.store.EnrollAdminSession(username, dek); err != nil {
		log.Printf("escrow enrollment of %s failed; retried at next sign-in: %v", username, err)
	}
}

// openSessionEscrow opens the signed-in administrator's escrow key for one
// operation, or returns nil when they hold no grant or it does not open;
// the caller then goes ahead without it. The caller must Close the result
// (Close is safe on nil).
func (a *App) openSessionEscrow(s sessionRef) *users.EscrowSession {
	var escrow *users.EscrowSession
	err := a.withSessionDEK(s, func(dek []byte) error {
		var err error
		escrow, err = a.store.OpenEscrow(s.user.Username, dek)
		return err
	})
	if err != nil && !errors.Is(err, users.ErrNoEscrowGrant) {
		log.Printf("open escrow key: %v", err)
	}
	return escrow
}

// zeroBytes overwrites b in place. ADR-001 requires the session DEK be
// zeroed wherever it's dropped rather than merely dereferenced, so the
// key doesn't linger in the heap for a swap file or core dump to catch.
// Shared by every DEK-dropping site (Logout, shutdown, and Login's
// TOCTOU-refusal path below) so each new call site adds one line, not
// a duplicated loop.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ChangePassword replaces the signed-in user's password. The encryption
// key is re-wrapped, not replaced, so the session, encrypted projects, and
// recovery codes are unaffected.
func (a *App) ChangePassword(currentPassword, newPassword string) error {
	u := a.requireUser()
	if u == nil {
		return errors.New("not signed in")
	}
	err := a.store.ChangePassword(u.Username, currentPassword, newPassword)
	switch {
	case errors.Is(err, auth.ErrMismatch):
		return errors.New("current password is incorrect")
	case errors.Is(err, users.ErrPasswordTooShort):
		return fmt.Errorf("new password must be at least %d characters", users.MinPasswordLength)
	case errors.Is(err, users.ErrPasswordChangedElsewhere):
		return errors.New("your password was changed elsewhere; sign out and sign in again")
	case errors.Is(err, users.ErrPasswordWrapCorrupt):
		return errors.New("your stored encryption key could not be read with this password, so nothing was changed; sign out and use a recovery code")
	}
	return err
}

// IssueRecoveryCodes generates 8 fresh recovery codes for the
// currently-signed-in user and returns the plaintext codes ONCE.
// The GUI MUST show them to the user immediately and warn that they
// will not be visible again — only their Argon2id hashes are
// persisted.
//
// Calling this rotates the user's existing unused codes at once, so it is
// used only by account creation, where there are no older codes to lose.
// App Settings and Project Settings use PrepareRecoveryCodes and
// ConfirmRecoveryCodes, which keep the old codes until the new ones are
// saved.
func (a *App) IssueRecoveryCodes() ([]string, error) {
	s := a.currentSession()
	if s.user == nil {
		return nil, errors.New("not signed in")
	}
	// ADR-001: wrap the session DEK into every code so a recovery
	// reset can re-wrap the same DEK (encrypted projects survive). The key
	// is the one of the session whose user the codes are stored for, and a
	// copy, so a concurrent Logout cannot zero it mid-wrap. Codes issued
	// without it would reset the password with a new DEK and orphan
	// encrypted projects, so a missing key is refused, never tolerated.
	dek, err := a.sessionDEK(s)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(dek)
	return a.store.IssueRecoveryCodes(s.user.Username, dek)
}

// RecoveryCodeStatusWire tells App Settings whether the signed-in user
// still has a safe way back in.
type RecoveryCodeStatusWire struct {
	Unused int  `json:"unused"`
	Total  int  `json:"total"`
	Legacy bool `json:"legacy"`
	// EncryptionReady says whether creating or encrypting a project would be
	// allowed now (encryptionReadiness).
	EncryptionReady bool `json:"encryption_ready"`
}

// RecoveryCodeStatus reports the signed-in user's unused recovery codes,
// and whether any is a legacy code that cannot recover encrypted projects.
func (a *App) RecoveryCodeStatus() (RecoveryCodeStatusWire, error) {
	u := a.requireUser()
	if u == nil {
		return RecoveryCodeStatusWire{}, errors.New("not signed in")
	}
	unused, legacy, err := a.store.RecoveryCodeStatus(u.Username)
	if err != nil {
		return RecoveryCodeStatusWire{}, err
	}
	return RecoveryCodeStatusWire{
		Unused:          unused,
		Total:           users.RecoveryCodeCount,
		Legacy:          legacy,
		EncryptionReady: a.encryptionReadiness(u.Username) == nil,
	}, nil
}

// AcceptEncryptionWithoutRecoveryCodes records that the signed-in user
// understands that projects they create or encrypt until they sign out
// cannot be recovered if they forget their password. It does not cover
// legacy codes, which must be replaced.
func (a *App) AcceptEncryptionWithoutRecoveryCodes() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.user == nil {
		return errors.New("not signed in")
	}
	a.noCodesAcceptedFor = a.user.Username
	return nil
}

// PrepareRecoveryCodes makes a new set of recovery codes for the signed-in
// user, after checking their current password, and returns them to show
// once. Nothing is stored and the old codes keep working until
// ConfirmRecoveryCodes; DiscardRecoveryCodes or signing out drops them.
func (a *App) PrepareRecoveryCodes(currentPassword string) ([]string, error) {
	s := a.currentSession()
	u := s.user
	if u == nil {
		return nil, errors.New("not signed in")
	}
	pending, err := a.store.PrepareRecoveryCodes(u.Username, currentPassword)
	switch {
	case errors.Is(err, auth.ErrMismatch):
		return nil, errors.New("current password is incorrect")
	case errors.Is(err, users.ErrPasswordWrapCorrupt):
		return nil, errors.New("your stored encryption key could not be read with this password, so no codes were made; sign out and use a recovery code")
	case err != nil:
		return nil, err
	}
	sessionGap("prepare codes")
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.epoch != s.epoch {
		return nil, errSessionChanged
	}
	a.pendingCodes = pending
	return pending.Codes(), nil
}

// ConfirmRecoveryCodes stores the codes from PrepareRecoveryCodes,
// replacing the signed-in user's old codes, once the user has saved them.
func (a *App) ConfirmRecoveryCodes() error {
	a.mu.Lock()
	pending := a.pendingCodes
	a.pendingCodes = nil
	user := a.user
	a.mu.Unlock()
	if user == nil {
		return errors.New("not signed in")
	}
	if pending == nil || pending.Username() != user.Username {
		return errors.New("there are no new codes to save; create them again")
	}
	err := a.store.ConfirmRecoveryCodes(pending)
	if errors.Is(err, users.ErrRecoveryCodesChanged) {
		return errors.New("your recovery codes changed while these were on screen, so they were not saved; create new ones again")
	}
	return err
}

// DiscardRecoveryCodes drops prepared codes without storing them; the old
// codes keep working.
func (a *App) DiscardRecoveryCodes() {
	a.mu.Lock()
	a.pendingCodes = nil
	a.mu.Unlock()
}

// ResetWithRecoveryCode is the "forgot password" flow. It does NOT
// require an active session — the user lands on the login screen,
// clicks "use a recovery code", enters username + code + new
// password, and we verify + rotate atomically.
func (a *App) ResetWithRecoveryCode(username, code, newPassword string) error {
	return a.store.ResetWithRecoveryCode(username, code, newPassword)
}

// Logout clears the active session and closes any open project. An
// operation still in progress for the session can no longer publish into
// or take the key of whatever session comes next (session_state.go).
func (a *App) Logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clearSessionLocked()
	return nil
}

// CurrentUser returns the active session or nil. Used by the GUI on
// initial mount to skip the login screen if we already have a user.
func (a *App) CurrentUser() *users.Account {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.user
}
