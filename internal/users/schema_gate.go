// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"errors"
	"fmt"
)

// systemSchemaReader is the highest reader level this build understands.
// system.db carries the level it needs in PRAGMA user_version; 0 is a file
// from before the gate existed.
//
// Raise this only for a change an older build would misread, and raise the
// stamp before or with that change (migrate does it first). Additive tables
// and columns do not raise it. An older build then refuses the file rather
// than working from data it does not understand.
//
// This guards against accidents, not attacks: system.db is plaintext and
// anyone who can edit it can edit the stamp. Builds before the gate
// (v1.1.0-alpha.2 and earlier) ignore it.
const systemSchemaReader = 1

// ErrSystemSchemaNewer means system.db needs a newer GoPMgr than this one.
// Its text reaches the command line as written.
var ErrSystemSchemaNewer = errors.New("users: this data was last opened by a newer GoPMgr; install that version or a later one")

// systemSchemaReadable is the gate's policy: which stamps this build may
// work with.
func systemSchemaReadable(stamp int) error {
	if stamp < 0 {
		return fmt.Errorf("users: invalid system.db schema stamp %d", stamp)
	}
	if stamp > systemSchemaReader {
		return fmt.Errorf("%w (needs reader level %d, this build reads up to %d)", ErrSystemSchemaNewer, stamp, systemSchemaReader)
	}
	return nil
}

func readSystemSchemaStamp(ctx context.Context, q accountWriter) (int, error) {
	var stamp int
	if err := q.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&stamp); err != nil {
		return 0, fmt.Errorf("users: read system.db schema stamp: %w", err)
	}
	return stamp, nil
}

// checkSystemSchema runs at the start of every write transaction, so a
// store that was opened before a newer build marked the file stops writing
// to it.
func checkSystemSchema(ctx context.Context, q accountWriter) error {
	stamp, err := readSystemSchemaStamp(ctx, q)
	if err != nil {
		return err
	}
	return systemSchemaReadable(stamp)
}

// raiseSystemSchemaStamp marks the file as needing at least level. It never
// lowers a stamp and writes nothing when the file is already there.
func raiseSystemSchemaStamp(ctx context.Context, q accountWriter, level int) error {
	stamp, err := readSystemSchemaStamp(ctx, q)
	if err != nil {
		return err
	}
	if stamp >= level {
		return nil
	}
	// PRAGMA takes no bound parameters; level is an integer.
	if _, err := q.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, level)); err != nil {
		return fmt.Errorf("users: write system.db schema stamp: %w", err)
	}
	return nil
}

// gateSystemSchema refuses a file this build cannot read (inWriteTx checks)
// and otherwise stamps it, in one transaction, before any migration runs.
func (s *Store) gateSystemSchema() error {
	return s.inWriteTx("schema gate", func(ctx context.Context, q accountWriter) error {
		return raiseSystemSchemaStamp(ctx, q, systemSchemaReader)
	})
}
