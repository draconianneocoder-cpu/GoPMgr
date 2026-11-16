// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// The unencrypted copy MigratePlaintextToEncrypted keeps beside an encrypted
// project is the whole project in the clear. GoPMgr never deletes it on its
// own; the user may, from Project Settings, once the encrypted project has
// opened (owner decision of 2026-11-14, SEC-04 item 2).

// ErrNoPlaintextBackup reports that no unencrypted copy is left.
var ErrNoPlaintextBackup = errors.New("db: no unencrypted copy of this project")

// PlaintextBackupPath is where MigratePlaintextToEncrypted keeps the
// unencrypted copy of projectPath.
func PlaintextBackupPath(projectPath string) string {
	return projectPath + ".pre-encryption.bak"
}

// plaintextBackupFiles is the copy and the SQLite sidecars a tool that
// opened it may have left, which can hold its pages too.
func plaintextBackupFiles(projectPath string) []string {
	backup := PlaintextBackupPath(projectPath)
	return []string{backup, backup + "-wal", backup + "-shm"}
}

// HasPlaintextBackup reports whether any file of projectPath's unencrypted
// copy (the copy or a sidecar) still exists.
func HasPlaintextBackup(projectPath string) (bool, error) {
	for _, f := range plaintextBackupFiles(projectPath) {
		if _, err := os.Lstat(f); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

// RemovePlaintextBackup deletes projectPath's unencrypted copy and its
// sidecars, and returns the copy's path. Before removing anything it proves
// that the encrypted project at projectPath opens read-only with dek and
// passes SQLite's and SQLCipher's integrity checks, and that every file to be
// removed is a regular file, the copy itself an unencrypted SQLite database.
// Sidecars are removed before the copy, so a failure part-way leaves the copy
// in place and the offer to delete it still shown. Deleting is not secure
// erasure: the data may remain recoverable from the disk or from backups.
func RemovePlaintextBackup(projectPath string, dek []byte) (string, error) {
	backup := PlaintextBackupPath(projectPath)
	var present []string
	for _, f := range plaintextBackupFiles(projectPath) {
		info, err := os.Lstat(f)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("db: %s is not a regular file; it was left alone", f)
		}
		present = append(present, f)
	}
	if len(present) == 0 {
		return "", ErrNoPlaintextBackup
	}
	if present[0] == backup {
		if err := requireSQLiteHeader(backup); err != nil {
			return "", err
		}
	}

	project, err := OpenEncryptedReadOnly(projectPath, dek)
	if err != nil {
		return "", fmt.Errorf("db: the encrypted project did not open, so its unencrypted copy was kept: %w", err)
	}
	checkErr := requireEncryptedIntegrity(project.Conn)
	closeErr := project.Close()
	if err := errors.Join(checkErr, closeErr); err != nil {
		return "", fmt.Errorf("db: the encrypted project failed its integrity check, so its unencrypted copy was kept: %w", err)
	}

	// The copy last: while it exists the offer stays, so nothing is hidden.
	for i := len(present) - 1; i >= 0; i-- {
		if err := os.Remove(present[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("db: delete %s: %w", present[i], err)
		}
	}
	return backup, nil
}

// requireSQLiteHeader refuses a file that does not start with SQLite's
// unencrypted header, so nothing else that happens to have the copy's name
// is deleted.
func requireSQLiteHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	header := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(f, header); err != nil || string(header) != sqliteHeader {
		return fmt.Errorf("db: %s is not an unencrypted GoPMgr copy; it was left alone", path)
	}
	return nil
}
