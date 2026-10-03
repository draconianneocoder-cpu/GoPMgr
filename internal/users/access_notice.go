// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Telling users about administrator access (ADR-004 decision 2). Each
// admin_access event is shown to the account it names at sign-in until the
// user acknowledges it; access_notices keeps the newest event id they have
// acknowledged. account_events ids come from AUTOINCREMENT, so a later
// access always has a higher id and is never counted as already seen.

// ErrAcknowledgeUnknownAccess is returned when an acknowledgement names an
// access newer than any recorded for the account.
var ErrAcknowledgeUnknownAccess = errors.New("users: no such access to acknowledge")

// AccessHistory returns every recorded administrator access to username's
// data, newest first.
func (s *Store) AccessHistory(username string) ([]AccountEvent, error) {
	rows, err := s.conn.Query(
		`SELECT id, occurred_at, actor, username, action, detail FROM account_events
		 WHERE username = ? AND action = ? ORDER BY id DESC`,
		username, AccountAdminAccess)
	if err != nil {
		return nil, fmt.Errorf("users: read access history: %w", err)
	}
	return scanAccountEvents(rows)
}

// UnreadAccessNotices returns the administrator accesses to username's data
// that username has not acknowledged, newest first.
func (s *Store) UnreadAccessNotices(username string) ([]AccountEvent, error) {
	rows, err := s.conn.Query(
		`SELECT id, occurred_at, actor, username, action, detail FROM account_events
		 WHERE username = ? AND action = ?
		   AND id > COALESCE((SELECT acknowledged_through FROM access_notices WHERE username = ?), 0)
		 ORDER BY id DESC`,
		username, AccountAdminAccess, username)
	if err != nil {
		return nil, fmt.Errorf("users: read access notices: %w", err)
	}
	return scanAccountEvents(rows)
}

// AcknowledgeAccessNotices records that username has read the notice of
// every access to their data up to and including event throughID, the
// newest one they were shown; an access recorded after that stays unread.
// The acknowledgement is recorded as access_notice_read. throughID must
// name one of username's accesses (ErrAcknowledgeUnknownAccess otherwise);
// the mark never moves back, and acknowledging nothing new records nothing.
func (s *Store) AcknowledgeAccessNotices(username string, throughID int64) error {
	return s.inWriteTx("access notice", func(ctx context.Context, q accountWriter) error {
		var exists int
		if err := q.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM account_events WHERE id = ? AND username = ? AND action = ?`,
			throughID, username, AccountAdminAccess).Scan(&exists); err != nil {
			return fmt.Errorf("users: read access: %w", err)
		}
		if exists == 0 {
			return ErrAcknowledgeUnknownAccess
		}
		var current int64
		err := q.QueryRowContext(ctx, `SELECT acknowledged_through FROM access_notices WHERE username = ?`, username).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("users: read access notice: %w", err)
		}
		if throughID <= current {
			return nil
		}
		var count int
		if err := q.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM account_events WHERE username = ? AND action = ? AND id > ? AND id <= ?`,
			username, AccountAdminAccess, current, throughID).Scan(&count); err != nil {
			return fmt.Errorf("users: count accesses: %w", err)
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO access_notices (username, acknowledged_through) VALUES (?, ?)
			 ON CONFLICT(username) DO UPDATE SET acknowledged_through = excluded.acknowledged_through`,
			username, throughID); err != nil {
			return fmt.Errorf("users: store access notice: %w", err)
		}
		detail := "1 access"
		if count != 1 {
			detail = fmt.Sprintf("%d accesses", count)
		}
		return recordAccountEvent(ctx, q, username, username, AccountAccessNoticeRead, detail)
	})
}
