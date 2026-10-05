// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Standby successor and takeover (ADR-005 part 2). The super administrator
// names a standby, who holds the administrator key but acts as a
// subordinate, and sets how long the super administrator may go without
// signing in before a takeover. Once that period has passed, the standby can
// become the super administrator; with no standby, any enabled
// administrator can, without the key.

// Takeover period bounds, in days. ADR-005 sets the default and the
// minimum; the maximum is a sanity bound.
const (
	DefaultTakeoverDays = 30
	MinTakeoverDays     = 7
	MaxTakeoverDays     = 365
)

// ErrTakeoverPeriod is returned for a takeover period outside
// MinTakeoverDays to MaxTakeoverDays.
var ErrTakeoverPeriod = fmt.Errorf("users: the takeover period must be %d to %d days", MinTakeoverDays, MaxTakeoverDays)

// ErrTakeoverNotDue is returned when the super administrator has signed in
// within the takeover period.
var ErrTakeoverNotDue = errors.New("users: the super administrator has signed in within the takeover period")

// ErrNotStandby is returned when a standby is named and someone else tries
// to take over.
var ErrNotStandby = errors.New("users: only the standby can take over")

// superSuccessionSchema holds the standby and the takeover period. The
// period's range is checked in Go, not by a CHECK, so it can change without
// a table rebuild; it is clamped when read.
const superSuccessionSchema = `
CREATE TABLE IF NOT EXISTS super_succession (
	id            INTEGER PRIMARY KEY CHECK (id = 1),
	standby       TEXT REFERENCES users(username) ON DELETE SET NULL,
	takeover_days INTEGER NOT NULL,
	changed_at    TEXT NOT NULL
);
`

func (s *Store) migrateSuperSuccession() error {
	_, err := s.conn.Exec(superSuccessionSchema)
	return err
}

// Succession is the administrator roles as one administrator sees them.
type Succession struct {
	Super           string
	Standby         string
	StandbyHoldsKey bool
	TakeoverDays    int
	// SuperInactiveDays is the whole days since the later of the super
	// administrator's last sign-in and the time they got the role: they
	// have not signed in for at least this long.
	SuperInactiveDays int
	// CanTakeOver reports whether the administrator who asked can become
	// the super administrator now.
	CanTakeOver bool
}

type successionState struct {
	super, standby string
	days           int
	// lastActive is the later of the super administrator's last sign-in
	// and the time they got the role; zero when neither can be read.
	lastActive time.Time
}

// readSuccessionTx returns the super administrator (assigning one if
// needed), the standby, and the takeover period. A standby who is the super
// administrator, or is not an enabled administrator, is cleared and
// recorded: the column is plaintext and is never trusted on its own.
func readSuccessionTx(ctx context.Context, q accountWriter) (successionState, error) {
	super, err := ensureSuperTx(ctx, q)
	if err != nil {
		return successionState{}, err
	}
	st := successionState{super: super, days: DefaultTakeoverDays}
	var standby sql.NullString
	err = q.QueryRowContext(ctx, `SELECT standby, takeover_days FROM super_succession WHERE id = 1`).Scan(&standby, &st.days)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return successionState{}, fmt.Errorf("users: read standby: %w", err)
	}
	st.days = min(max(st.days, MinTakeoverDays), MaxTakeoverDays)

	if standby.String != "" {
		switch err := requireEnabledAdmin(ctx, q, standby.String); {
		case standby.String == super:
			if err := clearStandbyTx(ctx, q, super, super, "became the super administrator"); err != nil {
				return successionState{}, err
			}
		case errors.Is(err, ErrNotAdmin):
			if err := removeGrantTx(ctx, q, standby.String); err != nil {
				return successionState{}, err
			}
			if err := clearStandbyTx(ctx, q, super, standby.String, "no longer an administrator who can sign in"); err != nil {
				return successionState{}, err
			}
		case err != nil:
			return successionState{}, err
		default:
			st.standby = standby.String
		}
	}

	if super != "" {
		var lastLogin, assignedAt string
		if err := q.QueryRowContext(ctx,
			`SELECT u.last_login, s.assigned_at FROM super_admin s JOIN users u ON u.username = s.username WHERE s.id = 1`,
		).Scan(&lastLogin, &assignedAt); err != nil {
			return successionState{}, fmt.Errorf("users: read super administrator activity: %w", err)
		}
		for _, stamp := range []string{lastLogin, assignedAt} {
			if t, err := time.Parse(time.RFC3339Nano, stamp); err == nil && t.After(st.lastActive) {
				st.lastActive = t
			}
		}
	}
	return st, nil
}

// takeoverRefusal returns why actor, an enabled administrator, cannot take
// over at now, or nil when they can. A last-active time in the future (the
// clock was moved back) is never due.
func (st successionState) takeoverRefusal(actor string, now time.Time) error {
	switch {
	case st.super == "" || actor == st.super:
		return ErrTargetIsSuper
	case st.standby != "" && actor != st.standby:
		return ErrNotStandby
	case now.Before(st.lastActive.AddDate(0, 0, st.days)):
		return ErrTakeoverNotDue
	}
	return nil
}

// clearStandbyTx clears the standby if it is username, recording why. It
// leaves grants alone: callers remove one where the account no longer
// should hold the key.
func clearStandbyTx(ctx context.Context, q accountWriter, actor, username, detail string) error {
	res, err := q.ExecContext(ctx,
		`UPDATE super_succession SET standby = NULL, changed_at = ? WHERE id = 1 AND standby = ?`, nowStamp(), username)
	if err != nil {
		return fmt.Errorf("users: clear standby: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return recordAccountEvent(ctx, q, actor, username, AccountStandbyRemoved, detail)
	}
	return nil
}

// holdsActiveGrantTx reports whether username holds a grant of the active
// escrow key. It says nothing about whether the grant opens; their own
// sign-in checks that and removes one that does not.
func holdsActiveGrantTx(ctx context.Context, q accountWriter, username string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM escrow_grants g JOIN escrow_keys k ON k.id = g.escrow_key_id AND k.retired_at = ''
		 WHERE g.admin_username = ?`, username).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("users: read escrow grant: %w", err)
	}
	return n > 0, nil
}

// passKeyTx grants actor's administrator key to target, an enabled
// administrator with a personal key, under the same attestation check as
// any grant. It reports false, granting nothing, when there is no active
// escrow key or actor's own grant does not open (openGrantTx records a
// mismatch). A target whose personal key fails its attestation returns
// ErrPersonalKeyNotAttested.
func passKeyTx(ctx context.Context, q accountWriter, actor string, actorDEK []byte, target string) (bool, error) {
	if _, ok, err := activeEscrowKey(ctx, q); err != nil || !ok {
		return false, err
	}
	session, err := openGrantTx(ctx, q, actor, actorDEK)
	switch {
	case errors.Is(err, ErrNoEscrowGrant), errors.Is(err, ErrEscrowMismatch):
		return false, nil
	case err != nil:
		return false, err
	}
	defer session.Close()
	var public, attestation []byte
	var attestedKeyID string
	if err := q.QueryRowContext(ctx,
		`SELECT public_key, attestation, attested_escrow_key_id FROM personal_keys WHERE username = ?`,
		target).Scan(&public, &attestation, &attestedKeyID); err != nil {
		return false, fmt.Errorf("users: read personal key: %w", err)
	}
	if err := attestAndGrantTx(ctx, q, session, actor, target, public, attestation, attestedKeyID); err != nil {
		return false, err
	}
	return true, nil
}

func keyDetail(passed bool, yes, no string) string {
	if passed {
		return yes
	}
	return no
}

// Succession returns the administrator roles for actor, who must be an
// enabled administrator.
func (s *Store) Succession(actor string) (Succession, error) {
	var out Succession
	err := s.inWriteTx("succession", func(ctx context.Context, q accountWriter) error {
		if err := requireEnabledAdmin(ctx, q, actor); err != nil {
			return err
		}
		st, err := readSuccessionTx(ctx, q)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		out = Succession{
			Super:        st.super,
			Standby:      st.standby,
			TakeoverDays: st.days,
			CanTakeOver:  st.takeoverRefusal(actor, now) == nil,
		}
		if !st.lastActive.IsZero() && now.After(st.lastActive) {
			out.SuperInactiveDays = int(now.Sub(st.lastActive).Hours() / 24)
		}
		if st.standby != "" {
			out.StandbyHoldsKey, err = holdsActiveGrantTx(ctx, q, st.standby)
		}
		return err
	})
	return out, err
}

// SetStandby names standby as the super administrator's successor and sets
// the takeover period, on behalf of actor, the super administrator. An
// empty standby removes the current one. The standby must be an enabled
// administrator who has signed in since enrollment began; actor's
// administrator key is passed to them under the attestation check, and a
// replaced standby's grant is removed. If actor's key cannot be opened the
// standby is still named; keyPassed reports whether the standby holds the
// key afterwards.
func (s *Store) SetStandby(actor string, actorDEK []byte, standby string, days int) (keyPassed bool, err error) {
	if days < MinTakeoverDays || days > MaxTakeoverDays {
		return false, ErrTakeoverPeriod
	}
	err = s.inWriteTx("standby change", func(ctx context.Context, q accountWriter) error {
		keyPassed = false
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		st, err := readSuccessionTx(ctx, q)
		if err != nil {
			return err
		}
		if standby != "" {
			if standby == actor {
				return ErrTargetIsSuper
			}
			if err := requireEnabledAdmin(ctx, q, standby); errors.Is(err, ErrNotAdmin) {
				return ErrTargetNotAdmin
			} else if err != nil {
				return err
			}
			if err := requirePersonalKeyTx(ctx, q, standby); err != nil {
				return err
			}
		}

		if st.standby != "" && st.standby != standby {
			if err := removeGrantTx(ctx, q, st.standby); err != nil {
				return err
			}
			detail := "removed by the super administrator"
			if standby != "" {
				detail = "replaced by " + standby
			}
			if err := clearStandbyTx(ctx, q, actor, st.standby, detail); err != nil {
				return err
			}
		}
		if standby != "" {
			if _, err := passKeyTx(ctx, q, actor, actorDEK, standby); err != nil {
				return err
			}
			// Report what the standby holds, not whether this call passed
			// the key: one named again keeps their earlier grant even when
			// actor's own key does not open now.
			if keyPassed, err = holdsActiveGrantTx(ctx, q, standby); err != nil {
				return err
			}
		}

		var named any
		if standby != "" {
			named = standby
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO super_succession (id, standby, takeover_days, changed_at) VALUES (1, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET standby = excluded.standby, takeover_days = excluded.takeover_days, changed_at = excluded.changed_at`,
			named, days, nowStamp()); err != nil {
			return fmt.Errorf("users: store standby: %w", err)
		}

		if standby != "" {
			return recordAccountEvent(ctx, q, actor, standby, AccountStandbyNamed,
				fmt.Sprintf("takes over after %d days without a sign-in by the super administrator; %s", days,
					keyDetail(keyPassed, "administrator key passed", "the administrator key could not be passed")))
		}
		if days != st.days {
			return recordAccountEvent(ctx, q, actor, actor, AccountTakeoverPeriodChanged, fmt.Sprintf("%d days", days))
		}
		return nil
	})
	if errors.Is(err, ErrPersonalKeyNotAttested) {
		s.recordAttestationFailure(actor, standby)
	}
	return keyPassed, err
}

// TakeOverSuper makes actor, an enabled administrator, the super
// administrator once the super administrator has not signed in for the
// takeover period: only the standby when one is named, otherwise any
// administrator (ADR-005). The former super administrator becomes a
// subordinate and their grant is removed; the standby is cleared. keyHeld
// reports whether actor holds a usable administrator key; a takeover with
// no standby leaves nobody holding it.
func (s *Store) TakeOverSuper(actor string, actorDEK []byte) (keyHeld bool, err error) {
	err = s.inWriteTx("super administrator takeover", func(ctx context.Context, q accountWriter) error {
		keyHeld = false
		if err := requireEnabledAdmin(ctx, q, actor); err != nil {
			return err
		}
		st, err := readSuccessionTx(ctx, q)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := st.takeoverRefusal(actor, now); err != nil {
			return err
		}

		if _, err := q.ExecContext(ctx,
			`UPDATE super_admin SET username = ?, assigned_at = ? WHERE id = 1`, actor, now.Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("users: store super administrator: %w", err)
		}
		if err := clearStandbyTx(ctx, q, actor, actor, "became the super administrator"); err != nil {
			return err
		}
		if err := removeGrantTx(ctx, q, st.super); err != nil {
			return err
		}
		session, err := openGrantTx(ctx, q, actor, actorDEK)
		switch {
		case errors.Is(err, ErrNoEscrowGrant), errors.Is(err, ErrEscrowMismatch):
		case err != nil:
			return err
		default:
			session.Close()
			keyHeld = true
		}

		inactive := "no sign-in on record"
		if !st.lastActive.IsZero() {
			inactive = fmt.Sprintf("no sign-in for at least %d days", int(now.Sub(st.lastActive).Hours()/24))
		}
		return recordAccountEvent(ctx, q, actor, st.super, AccountSuperTakenOver,
			inactive+"; "+keyDetail(keyHeld, "administrator key held", "the new super administrator does not hold the administrator key"))
	})
	return keyHeld, err
}
