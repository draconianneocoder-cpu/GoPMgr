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

// The super administrator (ADR-005). One administrator is the super
// administrator; every other administrator is a subordinate who manages
// standard accounts only. Only the super administrator changes
// administrators, holds the administrator (escrow) key, and opens users'
// data. The role is a plaintext row, so it never unlocks the key by itself:
// every key operation also needs the caller's own usable grant.

// ErrNotSuper is returned when an administrator who is not the super
// administrator tries something only the super administrator may do.
var ErrNotSuper = errors.New("users: only the super administrator can do this")

// ErrTargetIsSuper is returned when an action would demote, disable, or
// delete the super administrator, who must hand over the role first.
var ErrTargetIsSuper = errors.New("users: the super administrator must hand over the role first")

// superAdminSchema holds the one super administrator. The username is not
// deleted with the account: deleting the super administrator is refused.
const superAdminSchema = `
CREATE TABLE IF NOT EXISTS super_admin (
	id          INTEGER PRIMARY KEY CHECK (id = 1),
	username    TEXT NOT NULL REFERENCES users(username),
	assigned_at TEXT NOT NULL
);
`

func (s *Store) migrateSuperAdmin() error {
	_, err := s.conn.Exec(superAdminSchema)
	return err
}

// ensureSuperTx returns the super administrator, assigning one first when
// the role names nobody who is an administrator able to sign in: the
// earliest-created enabled administrator (owner decision, 2026-10-05). It
// never reassigns the role while it names an enabled administrator, and
// records every assignment. With no enabled administrator it returns "".
func ensureSuperTx(ctx context.Context, q accountWriter) (string, error) {
	var current string
	err := q.QueryRowContext(ctx, `SELECT username FROM super_admin WHERE id = 1`).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("users: read super administrator: %w", err)
	}
	if current != "" {
		if err := requireEnabledAdmin(ctx, q, current); err == nil {
			return current, nil
		} else if !errors.Is(err, ErrNotAdmin) {
			return "", err
		}
	}

	chosen, err := earliestEnabledAdmin(ctx, q)
	if err != nil || chosen == "" {
		return "", err
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO super_admin (id, username, assigned_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET username = excluded.username, assigned_at = excluded.assigned_at`,
		chosen, nowStamp()); err != nil {
		return "", fmt.Errorf("users: store super administrator: %w", err)
	}
	detail := "the earliest administrator"
	if current != "" {
		detail = "the earliest administrator, since " + current + " is no longer an administrator who can sign in"
	}
	if err := recordAccountEvent(ctx, q, chosen, chosen, AccountSuperAssigned, detail); err != nil {
		return "", err
	}
	return chosen, nil
}

// earliestEnabledAdmin returns the enabled administrator created first, or
// "" when there is none. created_at is RFC 3339 text, which does not sort
// as a string, so it is compared as a time; ties go to the username.
func earliestEnabledAdmin(ctx context.Context, q accountWriter) (string, error) {
	rows, err := q.QueryContext(ctx, `SELECT username, created_at FROM users WHERE is_admin = 1 AND disabled = 0`)
	if err != nil {
		return "", fmt.Errorf("users: list administrators: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var best string
	var bestAt time.Time
	for rows.Next() {
		var name, created string
		if err := rows.Scan(&name, &created); err != nil {
			return "", err
		}
		at, err := time.Parse(time.RFC3339Nano, created)
		if err != nil {
			at = time.Time{} // unreadable dates sort first, then by name
		}
		if best == "" || at.Before(bestAt) || (at.Equal(bestAt) && name < best) {
			best, bestAt = name, at
		}
	}
	return best, rows.Err()
}

// requireSuperTx returns ErrNotAdmin unless actor is an enabled
// administrator, and ErrNotSuper unless they are the super administrator.
func requireSuperTx(ctx context.Context, q accountWriter, actor string) error {
	if err := requireEnabledAdmin(ctx, q, actor); err != nil {
		return err
	}
	super, err := ensureSuperTx(ctx, q)
	if err != nil {
		return err
	}
	if super != actor {
		return ErrNotSuper
	}
	return nil
}

// requireAccountActionTx checks that actor may change username's account
// (disable, enable, delete, or change its role): any enabled administrator
// for a standard account; only the super administrator for an
// administrator; nobody for the super administrator.
func requireAccountActionTx(ctx context.Context, q accountWriter, actor, username string, targetIsAdmin bool) error {
	if !targetIsAdmin {
		return requireEnabledAdmin(ctx, q, actor)
	}
	if err := requireSuperTx(ctx, q, actor); err != nil {
		return err
	}
	if username == actor {
		return ErrTargetIsSuper
	}
	return nil
}

// RequireSuper returns ErrNotAdmin or ErrNotSuper unless actor is the super
// administrator, read from system.db.
func (s *Store) RequireSuper(actor string) error {
	return s.inWriteTx("super administrator check", func(ctx context.Context, q accountWriter) error {
		return requireSuperTx(ctx, q, actor)
	})
}

// HandOverSuper makes target, an enabled administrator who has signed in
// since enrollment began, the super administrator in actor's place; actor
// becomes a subordinate. actor's administrator key is passed to target
// under the same attestation check as any grant, and actor's own grant is
// removed. If actor's key cannot be opened the role still moves. keyPassed
// reports whether target holds the key afterwards (a standby passed over in
// a takeover may already hold it), and the history says the same; a target
// whose personal key fails its attestation is refused
// (ErrPersonalKeyNotAttested) and nothing changes.
func (s *Store) HandOverSuper(actor string, actorDEK []byte, target string) (keyPassed bool, err error) {
	err = s.inWriteTx("super administrator hand-over", func(ctx context.Context, q accountWriter) error {
		keyPassed = false
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		if target == actor {
			return ErrTargetIsSuper
		}
		if err := requireEnabledAdmin(ctx, q, target); errors.Is(err, ErrNotAdmin) {
			return ErrTargetNotAdmin
		} else if err != nil {
			return err
		}
		if err := requirePersonalKeyTx(ctx, q, target); err != nil {
			return err
		}
		// If actor's key does not open, the role moves without it.
		if _, err := passKeyTx(ctx, q, actor, actorDEK, target); err != nil {
			return err
		}
		// Report what target holds, not whether this call passed the key.
		var err error
		if keyPassed, err = holdsActiveGrantTx(ctx, q, target); err != nil {
			return err
		}

		if _, err := q.ExecContext(ctx,
			`UPDATE super_admin SET username = ?, assigned_at = ? WHERE id = 1`, target, nowStamp()); err != nil {
			return fmt.Errorf("users: store super administrator: %w", err)
		}
		if err := clearStandbyTx(ctx, q, actor, target, "became the super administrator"); err != nil {
			return err
		}
		if err := clearProtectionTx(ctx, q, target); err != nil {
			return err
		}
		if err := removeGrantTx(ctx, q, actor); err != nil {
			return err
		}
		return recordAccountEvent(ctx, q, actor, target, AccountSuperHandedOver,
			keyDetail(keyPassed, "administrator key passed", "the administrator key could not be passed"))
	})
	if errors.Is(err, ErrPersonalKeyNotAttested) {
		s.recordAttestationFailure(actor, target)
	}
	return keyPassed, err
}
