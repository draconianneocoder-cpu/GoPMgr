// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"pmforge/internal/crypto"
	"pmforge/internal/db"
	"pmforge/internal/sigma/service"
	"pmforge/internal/users"
)

// =========================================================
// helpers
// =========================================================

// requireUser returns the active session pointer under a read lock.
// The returned pointer is safe to dereference for the caller's
// lifetime — *users.Account is not freed by Logout (Go GC), and the
// fields the GUI reads are immutable after Login.
func (a *App) requireUser() *users.Account {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.user
}

// userDir returns the signed-in user's data directory, or "" if nobody is
// signed in. Used to open native file pickers in the user's own folder
// rather than a shared/last-used location. (This sets only the dialog's
// initial directory; it is a nudge, not a hard boundary - project contents
// are protected by per-user encryption regardless.)
func (a *App) userDir() string {
	if u := a.requireUser(); u != nil {
		return u.DataDir
	}
	return ""
}

// requireDEKLocked returns a copy of the active user's unlocked DEK.
// Caller must hold a.mu for reading or writing.
func (a *App) requireDEKLocked() ([]byte, error) {
	if len(a.dek) != crypto.DEKSize {
		return nil, errors.New("database key is locked; sign in again")
	}
	dek := make([]byte, len(a.dek))
	copy(dek, a.dek)
	return dek, nil
}

// requireDB returns the open *db.Database under a read lock. A
// concurrent Logout/CloseProject may Close the returned handle
// before the caller's query runs; the caller receives "sql:
// database is closed" rather than a crash. Acceptable for a
// single-user desktop app; see DEVELOPER_HANDBOOK.md §6.
func (a *App) requireDB() *db.Database {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.db
}

// requireSigmaSvc returns the Sigma service under a read lock so callers
// are guaranteed to see nil (not a partially-initialised pointer) if
// CloseProject or Logout has run concurrently.
func (a *App) requireSigmaSvc() *service.ProjectService {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sigmaSvc
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return absA == absB
	}
	return a == b
}
