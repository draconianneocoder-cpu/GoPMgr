// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"errors"

	"gopmgr/internal/db"
)

// The unencrypted copy encryption keeps beside a project can be deleted from
// Project Settings once the encrypted project has opened in this session
// (owner decision of 2026-11-14, SEC-04 item 2). GoPMgr never deletes it on
// its own. The deletion is not recorded in the project's audit chain or the
// deletion log: encrypting is not recorded either, the deletion log covers
// whole projects, and a new audit event type needs its own approval
// (DEC-AUDIT).

var errOpenToDeleteCopy = errors.New("open the encrypted project first; the unencrypted copy can be deleted once it has opened")

// PlaintextCopyPath returns the path of the unencrypted copy of the project
// at path when the copy, or a file left from it, still exists and the
// encrypted project is the one open in this session; otherwise "".
func (a *App) PlaintextCopyPath(path string) (string, error) {
	clean, _, err := a.projectPathFor(path)
	if err != nil {
		return "", err
	}
	if !a.encryptedProjectIsOpen(clean) {
		return "", nil
	}
	has, err := db.HasPlaintextBackup(clean)
	if err != nil || !has {
		return "", err
	}
	return db.PlaintextBackupPath(clean), nil
}

// DeletePlaintextCopy deletes the unencrypted copy of the project at path
// and returns the deleted path. The encrypted project must be the one open
// in this session, and must open again read-only with the session's key and
// pass its integrity checks first (db.RemovePlaintextBackup).
func (a *App) DeletePlaintextCopy(path string) (string, error) {
	clean, s, err := a.projectPathFor(path)
	if err != nil {
		return "", err
	}
	if !a.encryptedProjectIsOpen(clean) {
		return "", errOpenToDeleteCopy
	}
	var removed string
	err = a.withSessionDEK(s, func(dek []byte) error {
		var err error
		removed, err = db.RemovePlaintextBackup(clean, dek)
		return err
	})
	return removed, err
}

// encryptedProjectIsOpen reports whether the session's open project is the
// encrypted file at clean: OpenProject succeeded on it, including its
// compliance audit check.
func (a *App) encryptedProjectIsOpen(clean string) bool {
	a.mu.RLock()
	open := a.db != nil && samePath(a.dbPath, clean)
	a.mu.RUnlock()
	if !open {
		return false
	}
	encrypted, err := db.IsEncryptedFile(clean)
	return err == nil && encrypted
}
