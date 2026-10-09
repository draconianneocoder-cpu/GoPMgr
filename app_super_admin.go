// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gopmgr/internal/printing"
	"gopmgr/internal/users"
)

// The super administrator, standby successor, and takeover (ADR-005).

// AdminRolesWire tells the Admin panel who the super administrator and the
// standby are, whether the signed-in administrator can take over, the
// reasons a takeover can give, who is protected after one, and whether the
// administrator key is waiting to be replaced.
type AdminRolesWire struct {
	Super               string                    `json:"super"`
	Standby             string                    `json:"standby"`
	StandbyHoldsKey     bool                      `json:"standby_holds_key"`
	TakeoverDays        int                       `json:"takeover_days"`
	SuperInactiveDays   int                       `json:"super_inactive_days"`
	StandbyInactiveDays int                       `json:"standby_inactive_days"`
	CanTakeOver         bool                      `json:"can_take_over"`
	SuperHoldsKey       bool                      `json:"super_holds_key"`
	RotationPending     bool                      `json:"rotation_pending"`
	RecoveryCodeExists  bool                      `json:"recovery_code_exists"`
	RecoveryConfirmed   bool                      `json:"recovery_code_confirmed"`
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
		SuperHoldsKey:       s.SuperHoldsKey,
		RotationPending:     s.RotationPending,
		RecoveryCodeExists:  s.RecoveryCodeExists,
		RecoveryConfirmed:   s.RecoveryCodeConfirmed,
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

// AdminRotationWire reports a replacement of the administrator key.
type AdminRotationWire struct {
	Resealed int `json:"resealed"`
	Reenroll int `json:"reenroll"`
}

// AdminReplaceEscrowKey replaces the administrator key (ADR-005 part 3).
// Requires the super administrator, holding a usable key. keepStandby,
// when not empty, is the standby the super administrator chose to keep
// the key; any other standby loses it.
func (a *App) AdminReplaceEscrowKey(keepStandby string) (AdminRotationWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return AdminRotationWire{}, errors.New("administrator privileges required")
	}
	var res users.RotationResult
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		res, err = a.store.ReplaceEscrowKey(caller.Username, dek, keepStandby)
		return err
	})
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return AdminRotationWire{}, errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return AdminRotationWire{}, errors.New("only the super administrator can replace the administrator key")
	case errors.Is(err, users.ErrNoEscrowGrant):
		return AdminRotationWire{}, errors.New("you don't hold a usable administrator key, so you can't replace it")
	case errors.Is(err, users.ErrStandbyChanged):
		return AdminRotationWire{}, errors.New("the standby has changed; reload the Admin panel and try again")
	case err != nil:
		return AdminRotationWire{}, err
	}
	return AdminRotationWire{Resealed: res.Resealed, Reenroll: res.Reenroll}, nil
}

// runPendingRotation replaces the administrator key now when a change just
// marked it for replacement and the signed-in administrator is the one to
// do it. A failure is logged; the next sign-in of a key holder retries.
func (a *App) runPendingRotation(username string) {
	err := a.withSessionDEK(func(dek []byte) error {
		return a.store.RunPendingRotation(username, dek)
	})
	if err != nil {
		log.Printf("replacing the administrator key after an account change failed; it stays pending: %v", err)
	}
}

// The administrator key's recovery code (ADR-005 part 3b). The code is
// never logged: it opens every user's data sealed to the key.

// AdminCreateEscrowRecoveryCode makes a recovery code for the administrator
// key, replacing any earlier one, and returns it to be shown once. Requires
// the super administrator, holding a usable key.
func (a *App) AdminCreateEscrowRecoveryCode() (string, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return "", errors.New("administrator privileges required")
	}
	var code string
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		code, err = a.store.CreateEscrowRecoveryCode(caller.Username, dek)
		return err
	})
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return "", errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return "", errors.New("only the super administrator can make a recovery code")
	case errors.Is(err, users.ErrNoEscrowGrant):
		return "", errors.New("you don't hold a usable administrator key, so you can't make a recovery code for it")
	}
	return code, err
}

// AdminConfirmEscrowRecoveryCode marks the recovery code as kept once the
// super administrator types it back.
func (a *App) AdminConfirmEscrowRecoveryCode(code string) error {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	err := a.store.ConfirmEscrowRecoveryCode(caller.Username, code)
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return errors.New("only the super administrator can confirm the recovery code")
	case errors.Is(err, users.ErrInvalidEscrowRecoveryCode), errors.Is(err, users.ErrNoRecoveryCode):
		return errors.New("that doesn't match the recovery code; check it and try again")
	}
	return err
}

// AdminRecoverEscrowKey restores the administrator key to the super
// administrator from its recovery code.
func (a *App) AdminRecoverEscrowKey(code string) error {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	err := a.withSessionDEK(func(dek []byte) error {
		return a.store.RecoverEscrowKey(caller.Username, dek, code)
	})
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return errors.New("only the super administrator can restore the administrator key")
	case errors.Is(err, users.ErrNoRecoveryCode):
		return errors.New("the administrator key has no recovery code")
	case errors.Is(err, users.ErrInvalidEscrowRecoveryCode):
		return errors.New("that recovery code doesn't open the administrator key")
	case errors.Is(err, users.ErrEscrowMismatch):
		return errors.New("the recovery code can't be used: the key it opens isn't the one your account trusts. The account history has the details")
	}
	return err
}

// AdminCreateNewEscrowKey makes a new administrator key when nobody holds
// the old one (ADR-005 part 3c) and returns how many other accounts are
// sealed to it at their next sign-in; each is told why at that sign-in.
// Requires the super administrator, with no usable key.
func (a *App) AdminCreateNewEscrowKey(reason string) (int, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return 0, errors.New("administrator privileges required")
	}
	var others int
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		others, err = a.store.CreateNewEscrowKey(caller.Username, dek, reason)
		return err
	})
	var held *users.KeyHeldError
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return 0, errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return 0, errors.New("only the super administrator can make a new administrator key")
	case errors.Is(err, users.ErrNewKeyReasonRequired):
		return 0, errors.New("say why a new administrator key is needed")
	case errors.Is(err, users.ErrNewKeyReasonTooLong):
		return 0, fmt.Errorf("the reason can be at most %d characters", users.MaxNewKeyReasonLength)
	case errors.Is(err, users.ErrHoldsEscrowKey):
		return 0, errors.New("you hold the administrator key, so it isn't lost; replace it instead")
	case errors.As(err, &held) && held.Standby:
		return 0, fmt.Errorf("%s, your standby successor, holds the administrator key, so it isn't lost. To make a new one anyway, first remove %s as standby, which removes their key", held.Holder, held.Holder)
	case errors.As(err, &held):
		return 0, fmt.Errorf("%s holds the administrator key, so it isn't lost. To make a new one anyway, first demote or disable %s, which removes their key", held.Holder, held.Holder)
	case errors.Is(err, users.ErrNoEscrowGrant):
		return 0, errors.New("there is no administrator key yet; it's made the next time you sign in")
	case err != nil:
		return 0, err
	}
	return others, nil
}

// AdminPrinterWire is a printer the recovery code can be printed on.
type AdminPrinterWire struct {
	Name       string `json:"name"`
	Connection string `json:"connection"` // "local" or "network"
	Default    bool   `json:"default"`
}

// printers and printText are the print service, replaced in tests so
// nothing is printed.
var (
	printers  = printing.Printers
	printText = printing.Print
)

// AdminPrinters lists the printers that print on paper, the default first.
// An empty list means the recovery code has to be written down. Requires
// the super administrator.
func (a *App) AdminPrinters() ([]AdminPrinterWire, error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return nil, errors.New("administrator privileges required")
	}
	if err := a.store.RequireSuper(caller.Username); err != nil {
		return nil, errors.New("only the super administrator can print the recovery code")
	}
	list, err := printers(context.Background())
	if errors.Is(err, printing.ErrUnsupported) {
		return []AdminPrinterWire{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]AdminPrinterWire, 0, len(list))
	for _, p := range list {
		out = append(out, AdminPrinterWire{Name: p.Name, Connection: string(p.Connection), Default: p.Default})
	}
	return out, nil
}

// recoveryPage is the printed page: ASCII only, the code on its own line.
func recoveryPage(code, username string, made time.Time) string {
	return "GoPMgr administrator key recovery code\n\n" +
		"    " + code + "\n\n" +
		"Made " + made.Format("2006-01-02") + " by " + username + ".\n\n" +
		"Keep this page somewhere safe, away from this computer. Together with\n" +
		"GoPMgr's account file, this code opens every user's data.\n\n" +
		"It stops working when the administrator key is replaced; GoPMgr then\n" +
		"reminds the super administrator to make a new one.\n"
}

// AdminPrintEscrowRecoveryCode prints the recovery code on printer, after
// checking it is the administrator key's code. Printing does not confirm
// it: typing it back does. The job title says nothing about the code.
func (a *App) AdminPrintEscrowRecoveryCode(code, printer string) error {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	err := a.store.CheckEscrowRecoveryCode(caller.Username, code)
	switch {
	case errors.Is(err, users.ErrNotAdmin):
		return errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNotSuper):
		return errors.New("only the super administrator can print the recovery code")
	case errors.Is(err, users.ErrInvalidEscrowRecoveryCode), errors.Is(err, users.ErrNoRecoveryCode):
		return errors.New("that isn't the administrator key's recovery code")
	case err != nil:
		return err
	}
	err = printText(context.Background(), printer, "GoPMgr administrator recovery", recoveryPage(code, caller.Username, time.Now()))
	if errors.Is(err, printing.ErrNoSuchPrinter) {
		return fmt.Errorf("%q isn't a printer this computer can print on paper with; choose another", printer)
	}
	return err
}
