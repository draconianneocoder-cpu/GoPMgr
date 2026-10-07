// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Standby successor and takeover (ADR-005 parts 2 and 2b). The super
// administrator names a standby, who holds the administrator key but acts as
// a subordinate, and sets how long the super administrator may go without
// signing in before a takeover. Once that period has passed, the standby can
// become the super administrator; with no standby, or once the standby has
// also gone the period without signing in, any enabled administrator can,
// without the key. Whoever takes over picks a reason, which protects the
// former super administrator from being disabled, deleted, or demoted for a
// while.

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

// ErrNotStandby is returned when a standby who has signed in within the
// takeover period is named and someone else tries to take over.
var ErrNotStandby = errors.New("users: only the standby can take over")

// TakeoverReason is a reason chosen at takeover. Days is how long the former
// super administrator is protected (owner decisions, 2026-10-05).
type TakeoverReason struct {
	Code  string
	Label string
	Days  int
}

// TakeoverReasons lists the reasons a takeover can give, in display order.
var TakeoverReasons = []TakeoverReason{
	{Code: "vacation", Label: "Vacation", Days: 30},
	{Code: "parental_leave", Label: "Parental leave (maternity or paternity)", Days: 180},
	{Code: "medical_leave", Label: "Medical or convalescence leave", Days: 90},
	{Code: "left_employment", Label: "No longer an employee", Days: 7},
	{Code: "other", Label: "Other", Days: 30},
}

// MaxTakeoverNoteLength is the longest takeover note, in characters.
const MaxTakeoverNoteLength = 500

// ErrTakeoverReason is returned for a takeover reason that is not one of
// TakeoverReasons.
var ErrTakeoverReason = errors.New("users: choose a reason for the takeover")

// ErrTakeoverNoteRequired is returned when the reason "other" has no note.
var ErrTakeoverNoteRequired = errors.New("users: a takeover for another reason needs a note")

// ErrTakeoverNoteTooLong is returned for a note longer than
// MaxTakeoverNoteLength characters.
var ErrTakeoverNoteTooLong = fmt.Errorf("users: the takeover note can be at most %d characters", MaxTakeoverNoteLength)

// ErrProtected matches a ProtectionError.
var ErrProtected = errors.New("users: the account is protected after a takeover")

// ProtectionError is returned when an action would disable, delete, or
// demote a former super administrator still protected after a takeover.
type ProtectionError struct {
	Username string
	Reason   string // the reason's label
	Until    time.Time
}

func (e *ProtectionError) Error() string {
	return fmt.Sprintf("users: %s is protected after a takeover until %s", e.Username, e.Until.Format(time.RFC3339))
}

// Is reports whether target is ErrProtected.
func (e *ProtectionError) Is(target error) bool { return target == ErrProtected }

// Protection is a former super administrator's protection after a takeover.
type Protection struct {
	Username string
	Reason   string // the reason's label
	Until    time.Time
}

func takeoverReason(code string) (TakeoverReason, bool) {
	for _, r := range TakeoverReasons {
		if r.Code == code {
			return r, true
		}
	}
	return TakeoverReason{}, false
}

// reasonLabel returns code's label, or code itself for one this version
// does not know.
func reasonLabel(code string) string {
	if r, ok := takeoverReason(code); ok {
		return r.Label
	}
	return code
}

// superSuccessionSchema holds the standby, when they were named, and the
// takeover period, and each former super administrator's protection after
// a takeover. The period's range and the reasons are checked in Go, not by
// a CHECK, so they can change without a table rebuild; the period is
// clamped when read.
const superSuccessionSchema = `
CREATE TABLE IF NOT EXISTS super_succession (
	id               INTEGER PRIMARY KEY CHECK (id = 1),
	standby          TEXT REFERENCES users(username) ON DELETE SET NULL,
	takeover_days    INTEGER NOT NULL,
	changed_at       TEXT NOT NULL,
	standby_named_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS takeover_protection (
	username        TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
	reason          TEXT NOT NULL,
	note            TEXT NOT NULL DEFAULT '',
	protected_until TEXT NOT NULL,
	taken_over_by   TEXT NOT NULL,
	created_at      TEXT NOT NULL
);
`

// migrateSuperSuccession creates the tables and adds standby_named_at to a
// super_succession table made before it existed. Idempotent.
func (s *Store) migrateSuperSuccession() error {
	if _, err := s.conn.Exec(superSuccessionSchema); err != nil {
		return err
	}
	rows, err := s.conn.Query(`PRAGMA table_info(super_succession)`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid, pk int
		var name, typ, notnull string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "standby_named_at" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.conn.Exec(`ALTER TABLE super_succession ADD COLUMN standby_named_at TEXT NOT NULL DEFAULT ''`)
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
	// StandbyInactiveDays is the same for the standby, counted from the
	// later of their last sign-in and the time they were named.
	StandbyInactiveDays int
	// Protections lists former super administrators still protected after
	// a takeover.
	Protections []Protection
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
	// standbyLastActive is the later of the standby's last sign-in and the
	// time they were named; standbyNamedAt is the stored naming time.
	standbyLastActive time.Time
	standbyNamedAt    string
}

// laterStamp returns the latest of the RFC 3339 stamps that parse, or the
// zero time.
func laterStamp(stamps ...string) time.Time {
	var latest time.Time
	for _, stamp := range stamps {
		if t, err := time.Parse(time.RFC3339Nano, stamp); err == nil && t.After(latest) {
			latest = t
		}
	}
	return latest
}

// wholeDaysSince returns the whole days from t to now, or 0 when t is zero
// or not in the past.
func wholeDaysSince(t, now time.Time) int {
	if t.IsZero() || !now.After(t) {
		return 0
	}
	return int(now.Sub(t).Hours() / 24)
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
	err = q.QueryRowContext(ctx, `SELECT standby, takeover_days, standby_named_at FROM super_succession WHERE id = 1`).
		Scan(&standby, &st.days, &st.standbyNamedAt)
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
			var lastLogin string
			if err := q.QueryRowContext(ctx, `SELECT last_login FROM users WHERE username = ?`, st.standby).Scan(&lastLogin); err != nil {
				return successionState{}, fmt.Errorf("users: read standby activity: %w", err)
			}
			st.standbyLastActive = laterStamp(lastLogin, st.standbyNamedAt)
		}
	}

	if super != "" {
		var lastLogin, assignedAt string
		if err := q.QueryRowContext(ctx,
			`SELECT u.last_login, s.assigned_at FROM super_admin s JOIN users u ON u.username = s.username WHERE s.id = 1`,
		).Scan(&lastLogin, &assignedAt); err != nil {
			return successionState{}, fmt.Errorf("users: read super administrator activity: %w", err)
		}
		st.lastActive = laterStamp(lastLogin, assignedAt)
	}
	return st, nil
}

// takeoverRefusal returns why actor, an enabled administrator, cannot take
// over at now, or nil when they can. While a standby is named only they can,
// until they too have gone the period without signing in. A last-active
// time in the future (the clock was moved back) is never due.
func (st successionState) takeoverRefusal(actor string, now time.Time) error {
	switch {
	case st.super == "" || actor == st.super:
		return ErrTargetIsSuper
	case st.standby != "" && actor != st.standby && now.Before(st.standbyLastActive.AddDate(0, 0, st.days)):
		return ErrNotStandby
	case now.Before(st.lastActive.AddDate(0, 0, st.days)):
		return ErrTakeoverNotDue
	}
	return nil
}

// protectionTx returns username's protection after a takeover if it has not
// ended at now. An unreadable end time is treated as ended: only someone
// editing system.db can write one, and they could delete the row as easily.
func protectionTx(ctx context.Context, q accountWriter, username string, now time.Time) (*ProtectionError, error) {
	var reason, until string
	err := q.QueryRowContext(ctx, `SELECT reason, protected_until FROM takeover_protection WHERE username = ?`, username).Scan(&reason, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("users: read takeover protection: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, until)
	if err != nil || !now.Before(t) {
		return nil, nil
	}
	return &ProtectionError{Username: username, Reason: reasonLabel(reason), Until: t}, nil
}

// requireUnprotectedTx returns a ProtectionError when username is a former
// super administrator still protected after a takeover. Disabling,
// deleting, and demoting call it; enabling and hand-over do not.
func requireUnprotectedTx(ctx context.Context, q accountWriter, username string) error {
	p, err := protectionTx(ctx, q, username, time.Now().UTC())
	if err != nil {
		return err
	}
	if p != nil {
		return p
	}
	return nil
}

// clearProtectionTx removes username's protection, when they become the
// super administrator again: a later step-down of their own is not shielded.
func clearProtectionTx(ctx context.Context, q accountWriter, username string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM takeover_protection WHERE username = ?`, username); err != nil {
		return fmt.Errorf("users: clear takeover protection: %w", err)
	}
	return nil
}

// activeProtectionsTx lists the protections that have not ended at now.
func activeProtectionsTx(ctx context.Context, q accountWriter, now time.Time) ([]Protection, error) {
	rows, err := q.QueryContext(ctx, `SELECT username FROM takeover_protection ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("users: list takeover protections: %w", err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var out []Protection
	for _, name := range names {
		p, err := protectionTx(ctx, q, name, now)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, Protection{Username: p.Username, Reason: p.Reason, Until: p.Until})
		}
	}
	return out, nil
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
			Super:               st.super,
			Standby:             st.standby,
			TakeoverDays:        st.days,
			SuperInactiveDays:   wholeDaysSince(st.lastActive, now),
			StandbyInactiveDays: wholeDaysSince(st.standbyLastActive, now),
			CanTakeOver:         st.takeoverRefusal(actor, now) == nil,
		}
		if st.standby != "" {
			if out.StandbyHoldsKey, err = holdsActiveGrantTx(ctx, q, st.standby); err != nil {
				return err
			}
		}
		out.Protections, err = activeProtectionsTx(ctx, q, now)
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

		// The standby's inactivity counts from when they were named, so
		// saving the same standby with a new period does not restart it.
		var named any
		namedAt := ""
		if standby != "" {
			named = standby
			namedAt = st.standbyNamedAt
			if standby != st.standby {
				namedAt = nowStamp()
			}
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO super_succession (id, standby, takeover_days, changed_at, standby_named_at) VALUES (1, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET standby = excluded.standby, takeover_days = excluded.takeover_days,
			   changed_at = excluded.changed_at, standby_named_at = excluded.standby_named_at`,
			named, days, nowStamp(), namedAt); err != nil {
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
// takeover period: only the standby while one is named and has signed in
// within the period, otherwise any administrator (ADR-005). reason is one
// of TakeoverReasons' codes ("other" needs a note); it is recorded and
// protects the former super administrator from being disabled, deleted, or
// demoted for its number of days. The former super administrator becomes a
// subordinate and their grant is removed. A standby who takes over is no
// longer the standby; a standby passed over stays named and keeps their key
// (owner decision, 2026-10-07). keyHeld reports whether actor holds a
// usable administrator key.
func (s *Store) TakeOverSuper(actor string, actorDEK []byte, reasonCode, note string) (keyHeld bool, err error) {
	reason, ok := takeoverReason(reasonCode)
	if !ok {
		return false, ErrTakeoverReason
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxTakeoverNoteLength {
		return false, ErrTakeoverNoteTooLong
	}
	if reason.Code == "other" && note == "" {
		return false, ErrTakeoverNoteRequired
	}
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
		if err := clearProtectionTx(ctx, q, actor); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO takeover_protection (username, reason, note, protected_until, taken_over_by, created_at) VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(username) DO UPDATE SET reason = excluded.reason, note = excluded.note, protected_until = excluded.protected_until,
			   taken_over_by = excluded.taken_over_by, created_at = excluded.created_at`,
			st.super, reason.Code, note, now.AddDate(0, 0, reason.Days).Format(time.RFC3339Nano), actor, now.Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("users: store takeover protection: %w", err)
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

		// The detail names the reason by its label: history rows cannot be
		// corrected, and the panel shows them as written.
		parts := []string{"no sign-in on record"}
		if !st.lastActive.IsZero() {
			parts[0] = fmt.Sprintf("no sign-in for at least %d days", wholeDaysSince(st.lastActive, now))
		}
		if st.standby != "" && actor != st.standby {
			parts = append(parts, fmt.Sprintf("the standby, %s, had no sign-in for at least %d days", st.standby, wholeDaysSince(st.standbyLastActive, now)))
		}
		why := "reason: " + reason.Label
		if note != "" {
			why += ": " + note
		}
		parts = append(parts, why,
			keyDetail(keyHeld, "administrator key held", "the new super administrator does not hold the administrator key"))
		return recordAccountEvent(ctx, q, actor, st.super, AccountSuperTakenOver, strings.Join(parts, "; "))
	})
	return keyHeld, err
}
