// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// An administrator can take an account out of use in one of two ways
// (owner decision, 2026-09-24):
//
//   - Disable keeps the account, its key wraps, and its folder, and blocks
//     sign-in. Enable undoes it.
//   - Purge deletes the account and its folder: its projects (encrypted or
//     not), certificates, exports, and recovery codes. It cannot be undone.
//
// Each one is recorded in account_events, an append-only table in
// system.db, in the same transaction as the change it records.

// ErrAccountDisabled is returned by Authenticate for a disabled account
// whose password is correct, so it reveals nothing to someone who does
// not know the password.
var ErrAccountDisabled = errors.New("users: account is disabled")

// ErrPurgeIncomplete is returned by PurgeAccount when the account was
// deleted but its folder could not be fully removed.
var ErrPurgeIncomplete = errors.New("users: account deleted but its folder was not fully removed")

// Account event actions.
const (
	AccountDisabled         = "disabled"
	AccountEnabled          = "enabled"
	AccountPurged           = "purged"
	AccountFolderNotRemoved = "folder_not_removed"
)

// AccountEvent is one entry in the account history.
type AccountEvent struct {
	ID         int64     `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	Actor      string    `json:"actor"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`
	Detail     string    `json:"detail"`
}

// migrateAccountStatus adds the disabled column and the account_events
// table. Idempotent. The triggers stop the app from editing or deleting
// history; anyone who can write system.db directly can still change it.
func (s *Store) migrateAccountStatus() error {
	cols, err := s.usersColumns()
	if err != nil {
		return err
	}
	if !slices.Contains(cols, "disabled") {
		if _, err := s.conn.Exec(`ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	_, err = s.conn.Exec(`
	CREATE TABLE IF NOT EXISTS account_events (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		occurred_at TEXT NOT NULL,
		actor       TEXT NOT NULL,
		username    TEXT NOT NULL,
		action      TEXT NOT NULL CHECK (action IN ('disabled', 'enabled', 'purged', 'folder_not_removed')),
		detail      TEXT NOT NULL DEFAULT ''
	);
	CREATE TRIGGER IF NOT EXISTS account_events_no_update BEFORE UPDATE ON account_events
	BEGIN SELECT RAISE(ABORT, 'account_events is append-only'); END;
	CREATE TRIGGER IF NOT EXISTS account_events_no_delete BEFORE DELETE ON account_events
	BEGIN SELECT RAISE(ABORT, 'account_events is append-only'); END;
	`)
	return err
}

// usersColumns returns the users table's column names.
func (s *Store) usersColumns() ([]string, error) {
	rows, err := s.conn.Query(`SELECT name FROM pragma_table_info('users')`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// inWriteTx runs fn inside a BEGIN IMMEDIATE transaction on one connection
// and commits if fn succeeds. BEGIN IMMEDIATE takes SQLite's write lock at
// once, so checks fn makes still hold when it writes, even against another
// GoPMgr process on the same data root. op names the operation in errors.
func (s *Store) inWriteTx(op string, fn func(ctx context.Context, q accountWriter) error) error {
	ctx := context.Background()
	conn, err := s.conn.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("users: begin %s: %w", op, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()
	if err := fn(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("users: commit %s: %w", op, err)
	}
	committed = true
	return nil
}

// enabledAdminCount counts administrators who can sign in. Every
// last-administrator guard uses it, so no sequence of demotions,
// disablings, and purges can leave a machine whose only administrators
// are disabled.
func enabledAdminCount(ctx context.Context, q accountWriter) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE is_admin = 1 AND disabled = 0`).Scan(&n)
	return n, err
}

// accountRole reads username's role and status inside a transaction.
func accountRole(ctx context.Context, q accountWriter, username string) (isAdmin, disabled bool, err error) {
	var admin, off int
	err = q.QueryRowContext(ctx, `SELECT is_admin, disabled FROM users WHERE username = ?`, username).Scan(&admin, &off)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, ErrNoSuchUser
	}
	return admin == 1, off == 1, err
}

// guardLastEnabledAdmin returns ErrLastAdmin if username is an enabled
// administrator and the only one.
func guardLastEnabledAdmin(ctx context.Context, q accountWriter, isAdmin, disabled bool) error {
	if !isAdmin || disabled {
		return nil
	}
	n, err := enabledAdminCount(ctx, q)
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func recordAccountEvent(ctx context.Context, q accountWriter, actor, username, action, detail string) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO account_events (occurred_at, actor, username, action, detail) VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano), actor, username, action, detail,
	)
	return err
}

// SetDisabled disables or enables username on behalf of actor. Disabling
// the last enabled administrator returns ErrLastAdmin. A call that changes
// nothing records nothing.
func (s *Store) SetDisabled(actor, username string, disabled bool) error {
	return s.inWriteTx("account status change", func(ctx context.Context, q accountWriter) error {
		isAdmin, current, err := accountRole(ctx, q, username)
		if err != nil {
			return err
		}
		if current == disabled {
			return nil
		}
		if disabled {
			if err := guardLastEnabledAdmin(ctx, q, isAdmin, current); err != nil {
				return err
			}
		}
		if _, err := q.ExecContext(ctx, `UPDATE users SET disabled = ? WHERE username = ?`, boolToInt(disabled), username); err != nil {
			return err
		}
		action := AccountEnabled
		if disabled {
			action = AccountDisabled
		}
		return recordAccountEvent(ctx, q, actor, username, action, "")
	})
}

// PurgeAccount deletes username's account and then its folder, on behalf
// of actor. The account row (with its recovery codes, which cascade) and
// the "purged" event are committed together first; if the folder then
// cannot be fully removed, a "folder_not_removed" event is added and
// ErrPurgeIncomplete returned with the folder's path. Purging the last
// enabled administrator returns ErrLastAdmin.
//
// An account whose folder is one of GoPMgr's own (an account named "logs"
// from before that name was reserved) is refused with ErrReservedUsername:
// deleting its folder would delete the app's logs. Disable it instead.
func (s *Store) PurgeAccount(actor, username string) error {
	for _, reserved := range reservedFolderNames {
		if strings.EqualFold(username, reserved) {
			return ErrReservedUsername
		}
	}
	// The folder path is built only from a name that passes
	// ValidateUsername (letters, digits, _ and -, so a single path element
	// directly in the data root) and, below, exists in system.db.
	if err := ValidateUsername(username); err != nil {
		return ErrNoSuchUser
	}
	dataDir := filepath.Join(s.rootDir, username)

	err := s.inWriteTx("account deletion", func(ctx context.Context, q accountWriter) error {
		isAdmin, disabled, err := accountRole(ctx, q, username)
		if err != nil {
			return err
		}
		if err := guardLastEnabledAdmin(ctx, q, isAdmin, disabled); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, username); err != nil {
			return err
		}
		return recordAccountEvent(ctx, q, actor, username, AccountPurged, "")
	})
	if err != nil {
		return err
	}

	// RemoveAll does not follow symlinks, so a link inside the folder is
	// removed without touching what it points to.
	if err := os.RemoveAll(dataDir); err != nil {
		detail := fmt.Sprintf("%s: %v", dataDir, err)
		recErr := s.inWriteTx("account deletion record", func(ctx context.Context, q accountWriter) error {
			return recordAccountEvent(ctx, q, actor, username, AccountFolderNotRemoved, detail)
		})
		if recErr != nil {
			return fmt.Errorf("%w: %s (recording this also failed: %v)", ErrPurgeIncomplete, detail, recErr)
		}
		return fmt.Errorf("%w: %s", ErrPurgeIncomplete, detail)
	}
	return nil
}

// AccountEvents returns the account history, newest first.
func (s *Store) AccountEvents() ([]AccountEvent, error) {
	rows, err := s.conn.Query(`SELECT id, occurred_at, actor, username, action, detail FROM account_events ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []AccountEvent
	for rows.Next() {
		var (
			ev         AccountEvent
			occurredAt string
		)
		if err := rows.Scan(&ev.ID, &occurredAt, &ev.Actor, &ev.Username, &ev.Action, &ev.Detail); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, occurredAt); err == nil {
			ev.OccurredAt = t
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
