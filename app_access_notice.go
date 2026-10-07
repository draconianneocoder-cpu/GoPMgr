// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"

	"gopmgr/internal/users"
)

// Telling users that an administrator opened their data (ADR-004 decision
// 2). Each method acts only on the signed-in user's own account: none takes
// a username.

// MyDataAccessNotices returns the administrator accesses to the signed-in
// user's data that they have not acknowledged, newest first.
func (a *App) MyDataAccessNotices() ([]users.AccountEvent, error) {
	user := a.requireUser()
	if user == nil {
		return nil, errors.New("not signed in")
	}
	notices, err := a.store.UnreadAccessNotices(user.Username)
	if notices == nil && err == nil {
		notices = []users.AccountEvent{}
	}
	return notices, err
}

// AcknowledgeDataAccessNotices records that the signed-in user has read the
// notice of every access up to throughID, the newest they were shown.
func (a *App) AcknowledgeDataAccessNotices(throughID int64) error {
	user := a.requireUser()
	if user == nil {
		return errors.New("not signed in")
	}
	err := a.store.AcknowledgeAccessNotices(user.Username, throughID)
	if errors.Is(err, users.ErrAcknowledgeUnknownAccess) {
		return errors.New("that access to your data was not found; reload the notice")
	}
	return err
}

// MyDataAccessHistory returns every recorded administrator access to the
// signed-in user's data, newest first.
func (a *App) MyDataAccessHistory() ([]users.AccountEvent, error) {
	user := a.requireUser()
	if user == nil {
		return nil, errors.New("not signed in")
	}
	history, err := a.store.AccessHistory(user.Username)
	if history == nil && err == nil {
		history = []users.AccountEvent{}
	}
	return history, err
}
