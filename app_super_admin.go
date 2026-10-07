// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"time"

	"gopmgr/internal/users"
)

// The super administrator, standby successor, and takeover (ADR-005).

// AdminRolesWire tells the Admin panel who the super administrator and the
// standby are, whether the signed-in administrator can take over, the
// reasons a takeover can give, and who is protected after one.
type AdminRolesWire struct {
	Super               string                    `json:"super"`
	Standby             string                    `json:"standby"`
	StandbyHoldsKey     bool                      `json:"standby_holds_key"`
	TakeoverDays        int                       `json:"takeover_days"`
	SuperInactiveDays   int                       `json:"super_inactive_days"`
	StandbyInactiveDays int                       `json:"standby_inactive_days"`
	CanTakeOver         bool                      `json:"can_take_over"`
	TakeoverReasons     []AdminTakeoverReasonWire `json:"takeover_reasons"`
	Protections         []AdminProtectionWire     `json:"protections"`
}

// AdminTakeoverReasonWire is one reason a takeover can give.
type AdminTakeoverReasonWire struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Days  int    `json:"days"`
}

// AdminProtectionWire is a former super administrator protected after a
// takeover; Until is RFC 3339.
type AdminProtectionWire struct {
	Username string `json:"username"`
	Reason   string `json:"reason"`
	Until    string `json:"until"`
}

// AdminRoles returns the administrator roles. Requires an administrator.
func (a *App) AdminRoles() (AdminRolesWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return AdminRolesWire{}, errors.New("administrator privileges required")
	}
	s, err := a.store.Succession(caller.Username)
	if errors.Is(err, users.ErrNotAdmin) {
		return AdminRolesWire{}, errors.New("administrator privileges required")
	}
	if err != nil {
		return AdminRolesWire{}, err
	}
	wire := AdminRolesWire{
		Super:               s.Super,
		Standby:             s.Standby,
		StandbyHoldsKey:     s.StandbyHoldsKey,
		TakeoverDays:        s.TakeoverDays,
		SuperInactiveDays:   s.SuperInactiveDays,
		StandbyInactiveDays: s.StandbyInactiveDays,
		CanTakeOver:         s.CanTakeOver,
		TakeoverReasons:     []AdminTakeoverReasonWire{},
		Protections:         []AdminProtectionWire{},
	}
	for _, r := range users.TakeoverReasons {
		wire.TakeoverReasons = append(wire.TakeoverReasons, AdminTakeoverReasonWire{Code: r.Code, Label: r.Label, Days: r.Days})
	}
	for _, p := range s.Protections {
		wire.Protections = append(wire.Protections, AdminProtectionWire{Username: p.Username, Reason: p.Reason, Until: p.Until.Format(time.RFC3339)})
	}
	return wire, nil
}

// AdminHandOverResultWire reports whether the administrator key went to
// the other administrator: at a hand-over, or when naming a standby.
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
	case err != nil:
		return AdminHandOverResultWire{}, targetError(err, username, "become the super administrator")
	}
	// The former super administrator no longer holds the key.
	a.mu.Lock()
	a.stopAdminAccessLocked()
	a.mu.Unlock()
	return AdminHandOverResultWire{KeyPassed: keyPassed}, nil
}

// AdminSetStandby names username as the standby successor, or removes the
// standby when username is empty, and sets how many days the super
// administrator may go without signing in before a takeover. Requires the
// super administrator.
func (a *App) AdminSetStandby(username string, days int) (AdminHandOverResultWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return AdminHandOverResultWire{}, errors.New("administrator privileges required")
	}
	var keyPassed bool
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		keyPassed, err = a.store.SetStandby(caller.Username, dek, username, days)
		return err
	})
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return AdminHandOverResultWire{}, errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return AdminHandOverResultWire{}, errors.New("only the super administrator can name a standby")
	case errors.Is(err, users.ErrTargetIsSuper):
		return AdminHandOverResultWire{}, errors.New("you can't be your own standby")
	case errors.Is(err, users.ErrTakeoverPeriod):
		return AdminHandOverResultWire{}, fmt.Errorf("the takeover period must be %d to %d days", users.MinTakeoverDays, users.MaxTakeoverDays)
	case err != nil:
		return AdminHandOverResultWire{}, targetError(err, username, "be the standby")
	}
	return AdminHandOverResultWire{KeyPassed: keyPassed}, nil
}

// AdminTakeOverResultWire reports whether the new super administrator
// holds the administrator key after a takeover.
type AdminTakeOverResultWire struct {
	KeyHeld bool `json:"key_held"`
}

// AdminTakeOverSuper makes the signed-in administrator the super
// administrator once the super administrator has not signed in for the
// takeover period: only the standby while one is named and has signed in
// within the period, otherwise any administrator. reason is a code from
// AdminRoles' takeover_reasons; "other" needs a note.
func (a *App) AdminTakeOverSuper(reason, note string) (AdminTakeOverResultWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return AdminTakeOverResultWire{}, errors.New("administrator privileges required")
	}
	var keyHeld bool
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		keyHeld, err = a.store.TakeOverSuper(caller.Username, dek, reason, note)
		return err
	})
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return AdminTakeOverResultWire{}, errors.New("administrator privileges required")
	case errors.Is(err, users.ErrTargetIsSuper):
		return AdminTakeOverResultWire{}, errors.New("you are already the super administrator")
	case errors.Is(err, users.ErrNotStandby):
		return AdminTakeOverResultWire{}, errors.New("only the standby successor can take over")
	case errors.Is(err, users.ErrTakeoverNotDue):
		return AdminTakeOverResultWire{}, errors.New("the super administrator has signed in within the takeover period, so the role can't be taken over")
	case errors.Is(err, users.ErrTakeoverReason):
		return AdminTakeOverResultWire{}, errors.New("choose a reason for the takeover")
	case errors.Is(err, users.ErrTakeoverNoteRequired):
		return AdminTakeOverResultWire{}, errors.New("describe the reason in the note")
	case errors.Is(err, users.ErrTakeoverNoteTooLong):
		return AdminTakeOverResultWire{}, fmt.Errorf("the note can be at most %d characters", users.MaxTakeoverNoteLength)
	case err != nil:
		return AdminTakeOverResultWire{}, err
	}
	return AdminTakeOverResultWire{KeyHeld: keyHeld}, nil
}

// targetError turns the store's refusals about the other administrator in
// a hand-over or standby change into plain messages; role is what they
// would have become.
func targetError(err error, username, role string) error {
	switch {
	case errors.Is(err, users.ErrTargetNotAdmin):
		return fmt.Errorf("%s must be an administrator who can sign in", username)
	case errors.Is(err, users.ErrNoPersonalKey):
		return fmt.Errorf("%s must sign in once before they can %s", username, role)
	case errors.Is(err, users.ErrPersonalKeyNotAttested):
		return fmt.Errorf("%s can't %s: their account key has been changed since an administrator last checked it. The change is recorded in the account history", username, role)
	}
	return err
}
