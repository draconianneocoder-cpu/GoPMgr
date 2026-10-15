// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"

	"gopmgr/internal/users"
)

// Session admission (docs/design/wbs-authenticated-account-identity.md,
// credential coherence step 4). Who is signed in changes only through
// setSessionLocked and clearSessionLocked, and each change advances
// App.epoch. An operation that reads the session, then does slow or
// file-system work, captures a sessionRef first and then:
//
//   - takes the session's key only through sessionDEK(s), which refuses once
//     the session has changed, so one account's name, folder, or path is
//     never paired with another account's key; and
//   - publishes a session, a project, or administrator access only if the
//     epoch is still s.epoch, so a sign-out or another sign-in that finished
//     meanwhile is never overtaken by a late completion.
//
// Work that already holds the key may still finish after a sign-out; it is
// late but stays within the one account it started for.

// errSessionChanged is returned when the signed-in session changed between
// the start of an operation and its use of the session.
var errSessionChanged = errors.New("you signed out or switched accounts while this was in progress, so it was stopped; try again")

// sessionRef is the signed-in session as one locked read saw it. user is
// nil when nobody was signed in.
type sessionRef struct {
	user      *users.Account
	principal users.Principal
	epoch     uint64
}

// sessionGap runs where an operation has captured the session and is about
// to use or publish it. Tests replace it to sign out or switch accounts
// there.
var sessionGap = func(op string) {}

// currentSession returns the signed-in session.
func (a *App) currentSession() sessionRef {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return sessionRef{user: a.user, principal: a.principal, epoch: a.epoch}
}

// sessionDEK returns a copy of s's DEK, or errSessionChanged if s is no
// longer the signed-in session. The caller must zero the copy.
func (a *App) sessionDEK(s sessionRef) ([]byte, error) {
	sessionGap("key")
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessionDEKLocked(s)
}

// sessionDEKLocked is sessionDEK for a caller holding a.mu.
func (a *App) sessionDEKLocked(s sessionRef) ([]byte, error) {
	if s.user == nil {
		return nil, errors.New("not signed in")
	}
	if a.epoch != s.epoch {
		return nil, errSessionChanged
	}
	return a.requireDEKLocked()
}

// withSessionDEK calls fn with a copy of s's DEK, zeroed after, or returns
// errSessionChanged without calling it.
func (a *App) withSessionDEK(s sessionRef, fn func(dek []byte) error) error {
	dek, err := a.sessionDEK(s)
	if err != nil {
		return err
	}
	defer zeroBytes(dek)
	return fn(dek)
}

// setSessionLocked signs acc in, with the principal its sign-in or creation
// established and its DEK, which the session now owns. Any administrator
// access, project, and per-session state of the previous session goes.
// Must hold a.mu for writing.
func (a *App) setSessionLocked(acc users.Account, principal users.Principal, dek []byte) {
	a.clearSessionLocked()
	a.user = &acc
	a.principal = principal
	a.dek = dek
}

// clearSessionLocked signs out: it closes any open project, ends any
// administrator access, zeroes the DEK (ADR-001), and drops the session's
// pending recovery codes and choices. Must hold a.mu for writing.
func (a *App) clearSessionLocked() {
	if a.db != nil {
		_ = a.db.Close()
	}
	a.db = nil
	a.dbPath = ""
	a.adminSvc = nil
	a.sigmaSvc = nil
	a.openCheck = nil
	a.stopAdminAccessLocked()
	a.user = nil
	a.principal = users.Principal{}
	zeroBytes(a.dek)
	a.dek = nil
	a.pendingCodes = nil
	a.noCodesAcceptedFor = ""
	a.epoch++
}
