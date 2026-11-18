// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ErrGeneratedIDCollision means a newly generated ID already names a stored
// record. The create is refused and nothing is written, so the existing record,
// its references and its audit history stay as they were. With 128 random
// bits this points to a broken entropy source, so it is not retried.
var ErrGeneratedIDCollision = errors.New("generated ID is already in use; nothing was saved")

// newID returns a URL-safe identifier: prefix, an underscore, and 128 random
// bits encoded as 32 lowercase hexadecimal characters. Existing stored IDs
// remain opaque strings; this changes only newly generated identifiers.
func newID(prefix string) (string, error) {
	var buf [16]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf[:]), nil
}

// refuseTakenIDTx returns ErrGeneratedIDCollision when countQuery, a constant
// "SELECT COUNT(*) FROM <table> WHERE id = ?", finds id inside tx. Saves whose
// write transaction does not otherwise read the record call it before writing.
func refuseTakenIDTx(tx *sql.Tx, countQuery, id string) error {
	var n int
	if err := tx.QueryRow(countQuery, id).Scan(&n); err != nil {
		return fmt.Errorf("check generated id: %w", err)
	}
	if n > 0 {
		return ErrGeneratedIDCollision
	}
	return nil
}
