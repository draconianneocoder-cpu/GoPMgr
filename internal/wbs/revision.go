// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package wbs

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

var (
	ErrRevision         = errors.New("wbs: invalid revision")
	ErrRevisionOverflow = errors.New("wbs: revision exhausted")
	ErrRevisionConflict = errors.New("wbs: revision conflict")
	ErrIdentityMismatch = errors.New("wbs: identity mismatch")
)

func revisionValue(s string) (uint64, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(v, 10) != s {
		return 0, ErrRevision
	}
	return v, nil
}

// NextRevision accepts canonical uint64 decimal strings, including zero.
// This helper does not issue authoritative revisions or commit a write.
func NextRevision(current string) (string, error) {
	v, err := revisionValue(current)
	if err != nil {
		return "", err
	}
	if v == math.MaxUint64 {
		return "", ErrRevisionOverflow
	}
	return strconv.FormatUint(v+1, 10), nil
}

// Identity belongs to the server-bound operation context, not imported scope.
// Matching it does not authorize a caller or establish a database CAS boundary.
type Identity struct {
	CopyID     string
	ProjectID  string
	ChartID    string
	LifetimeID string
}

// CheckWriteContext is a pure predicate. A future writer must perform this
// check with current authorization and revision comparison inside its transaction.
func CheckWriteContext(current, requested Identity, revision, expected string) error {
	for _, identity := range []Identity{current, requested} {
		for _, s := range []string{identity.CopyID, identity.ProjectID, identity.ChartID, identity.LifetimeID} {
			if err := identifier(s); err != nil {
				return fmt.Errorf("write identity: %w", err)
			}
		}
	}
	if _, err := revisionValue(revision); err != nil {
		return err
	}
	if _, err := revisionValue(expected); err != nil {
		return err
	}
	if current != requested {
		return ErrIdentityMismatch
	}
	if revision != expected {
		return ErrRevisionConflict
	}
	return nil
}
