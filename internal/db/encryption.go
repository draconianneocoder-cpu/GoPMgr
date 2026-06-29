// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"pmforge/internal/crypto"
)

const sqliteHeader = "SQLite format 3\x00"

// InitEncryptedDB opens (or creates) a SQLCipher-encrypted PMForge
// project database with the user's 32-byte DEK as a raw keyspec.
func InitEncryptedDB(path string, dek []byte) (*Database, error) {
	dsn, err := encryptedDSN(path, dek)
	if err != nil {
		return nil, err
	}
	return initDBWithDSN(path, dsn)
}

// IsEncryptedFile reports whether path does not expose SQLite's
// plaintext file header. Missing files and stat/read failures are
// returned to the caller.
func IsEncryptedFile(path string) (bool, error) {
	f, err := os.Open(path) // #nosec G304 -- caller supplies a PMForge database path.
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, len(sqliteHeader))
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}
	if n < len(sqliteHeader) {
		return false, nil
	}
	return string(header) != sqliteHeader, nil
}

// MigratePlaintextToEncrypted converts a plaintext .pmforge file to
// SQLCipher in place and retains the original as
// <path>.pre-encryption.bak. It mirrors the repair swap pattern:
// verify source, export to an encrypted sibling, verify destination,
// then rename.
func MigratePlaintextToEncrypted(path string, dek []byte) (backupPath string, err error) {
	// Validate the DEK length up front (cheap, no key material derived) so a
	// bad DEK is rejected before any filesystem work, matching InitEncryptedDB.
	if len(dek) != crypto.DEKSize {
		return "", crypto.ErrBadDEK
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("db: migration source is not a regular file: %s", path)
	}
	encrypted, err := IsEncryptedFile(path)
	if err != nil {
		return "", err
	}
	if encrypted {
		return "", fmt.Errorf("db: migration source is already encrypted: %s", path)
	}

	backupPath = path + ".pre-encryption.bak"
	if _, err := os.Stat(backupPath); err == nil {
		return "", fmt.Errorf("db: encryption backup already exists: %s", backupPath)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	encryptedPath := path + ".encrypted.tmp"
	if err := removeSQLiteFileSet(encryptedPath); err != nil {
		return "", fmt.Errorf("db: clear encrypted temp: %w", err)
	}
	defer func() {
		if err != nil {
			_ = removeSQLiteFileSet(encryptedPath)
		}
	}()

	plain, err := InitDB(path)
	if err != nil {
		return "", err
	}
	// Derive the hex keyspec only here, immediately before it is used, so
	// the raw-key string (which a Go string cannot zero) lives for the
	// shortest possible window rather than across the pre-flight checks.
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		_ = plain.Close()
		return "", err
	}
	if err := exportEncryptedCopy(plain.Conn, encryptedPath, hexKey); err != nil {
		_ = plain.Close()
		return "", err
	}
	if err := plain.Close(); err != nil {
		return "", err
	}

	verified, err := InitEncryptedDB(encryptedPath, dek)
	if err != nil {
		return "", fmt.Errorf("db: verify encrypted migration: %w", err)
	}
	if err := requireEncryptedIntegrity(verified.Conn); err != nil {
		_ = verified.Close()
		return "", err
	}
	if err := prepareForRename(verified.Conn); err != nil {
		_ = verified.Close()
		return "", err
	}
	if err := verified.Close(); err != nil {
		return "", err
	}

	if err := removeSQLiteSidecars(path); err != nil {
		return "", fmt.Errorf("db: clear source sidecars: %w", err)
	}
	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("db: retain plaintext backup: %w", err)
	}
	if err := os.Rename(encryptedPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return "", fmt.Errorf("db: publish encrypted database: %w", err)
	}
	if err := removeSQLiteSidecars(encryptedPath); err != nil {
		return "", fmt.Errorf("db: clear encrypted temp sidecars: %w", err)
	}
	if err := ensurePrivateSQLiteFiles(path); err != nil {
		return "", err
	}
	if err := chmodIfExists(backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

func encryptedDSN(path string, dek []byte) (string, error) {
	// go-sqlcipher treats everything after the first '?' as DSN query options
	// and does not URL-decode the path, so a '?' (or fragment '#') in the path
	// would let the path inject or override _pragma_* options — including the
	// key. Confined PMForge project paths never contain these characters;
	// reject rather than emit an ambiguous DSN.
	if strings.ContainsAny(path, "?#") {
		return "", fmt.Errorf("db: project path contains an illegal character: %q", path)
	}
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		return "", err
	}
	return path + "?_pragma_key=x'" + hexKey + "'", nil
}

func exportEncryptedCopy(conn *sql.DB, encryptedPath, hexKey string) error {
	ok, err := checkIntegrity(conn)
	if err != nil {
		return fmt.Errorf("db: source integrity: %w", err)
	}
	if !ok {
		return errors.New("db: source integrity_check returned non-ok")
	}
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return fmt.Errorf("db: checkpoint source: %w", err)
	}
	attach := fmt.Sprintf(
		"ATTACH DATABASE '%s' AS encrypted KEY \"x'%s'\"",
		sqlQuote(encryptedPath),
		hexKey,
	)
	if _, err := conn.Exec(attach); err != nil {
		return fmt.Errorf("db: attach encrypted target: %w", err)
	}
	attached := true
	defer func() {
		if attached {
			_, _ = conn.Exec("DETACH DATABASE encrypted")
		}
	}()
	var ignored sql.NullString
	if err := conn.QueryRow("SELECT sqlcipher_export('encrypted')").Scan(&ignored); err != nil {
		return fmt.Errorf("db: sqlcipher_export: %w", err)
	}
	var userVersion int
	if err := conn.QueryRow("PRAGMA main.user_version").Scan(&userVersion); err != nil {
		return fmt.Errorf("db: read user_version: %w", err)
	}
	if _, err := conn.Exec(fmt.Sprintf("PRAGMA encrypted.user_version = %d", userVersion)); err != nil {
		return fmt.Errorf("db: copy user_version: %w", err)
	}
	if _, err := conn.Exec("DETACH DATABASE encrypted"); err != nil {
		return fmt.Errorf("db: detach encrypted target: %w", err)
	}
	attached = false
	return nil
}

func requireEncryptedIntegrity(conn *sql.DB) error {
	ok, err := checkIntegrity(conn)
	if err != nil {
		return fmt.Errorf("db: encrypted integrity: %w", err)
	}
	if !ok {
		return errors.New("db: encrypted integrity_check returned non-ok")
	}
	rows, err := conn.Query("PRAGMA cipher_integrity_check")
	if err != nil {
		return fmt.Errorf("db: cipher_integrity_check: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return errors.New("db: cipher_integrity_check reported failures")
	}
	return rows.Err()
}

func prepareForRename(conn *sql.DB) error {
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return err
	}
	if _, err := conn.Exec("PRAGMA journal_mode = DELETE;"); err != nil {
		return err
	}
	return nil
}

func checkIntegrity(conn *sql.DB) (bool, error) {
	var result string
	if err := conn.QueryRow("PRAGMA integrity_check;").Scan(&result); err != nil {
		return false, err
	}
	return result == "ok", nil
}

func removeSQLiteFileSet(path string) error {
	if err := removeIfExists(path); err != nil {
		return err
	}
	return removeSQLiteSidecars(path)
}

func removeSQLiteSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := removeIfExists(path + suffix); err != nil {
			return err
		}
	}
	return nil
}

func chmodIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Chmod(path, 0o600)
}

func sqlQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
