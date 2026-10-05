// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"

	"gopmgr/internal/users"
)

// The super administrator (ADR-005).

// AdminRolesWire tells the Admin panel who the super administrator is.
type AdminRolesWire struct {
	Super string `json:"super"`
}

// AdminRoles returns the administrator roles. Requires an administrator.
func (a *App) AdminRoles() (AdminRolesWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return AdminRolesWire{}, errors.New("administrator privileges required")
	}
	super, err := a.store.SuperAdmin()
	return AdminRolesWire{Super: super}, err
}

// AdminHandOverResultWire reports a hand-over: whether the administrator
// key went with the role.
type AdminHandOverResultWire struct {
	KeyPassed bool `json:"key_passed"`
}

// AdminHandOverSuper makes username the super administrator in place of
// the signed-in super administrator, who becomes a subordinate. Any data
// they had open is closed first.
func (a *App) AdminHandOverSuper(username string) (AdminHandOverResultWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return AdminHandOverResultWire{}, errors.New("administrator privileges required")
	}
	var keyPassed bool
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		keyPassed, err = a.store.HandOverSuper(caller.Username, dek, username)
		return err
	})
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return AdminHandOverResultWire{}, errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return AdminHandOverResultWire{}, errors.New("only the super administrator can hand over the role")
	case errors.Is(err, users.ErrTargetIsSuper):
		return AdminHandOverResultWire{}, errors.New("you are already the super administrator")
	case errors.Is(err, users.ErrTargetNotAdmin):
		return AdminHandOverResultWire{}, fmt.Errorf("%s must be an administrator who can sign in", username)
	case errors.Is(err, users.ErrNoPersonalKey):
		return AdminHandOverResultWire{}, fmt.Errorf("%s must sign in once before they can become the super administrator", username)
	case errors.Is(err, users.ErrPersonalKeyNotAttested):
		return AdminHandOverResultWire{}, fmt.Errorf("%s did not become the super administrator: their account key has been changed since an administrator last checked it. The change is recorded in the account history", username)
	case err != nil:
		return AdminHandOverResultWire{}, err
	}
	// The former super administrator no longer holds the key.
	a.mu.Lock()
	a.stopAdminAccessLocked()
	a.mu.Unlock()
	return AdminHandOverResultWire{KeyPassed: keyPassed}, nil
}
