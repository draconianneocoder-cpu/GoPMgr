// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package users owns PMForge's local-multi-user system. It provides:
//
//   - A "system database" at ~/Documents/PMForge/system.db that lists
//     every PMForge account on this machine (username, display name,
//     password hash, data directory).
//   - Per-user folders at ~/Documents/PMForge/<username>/ that hold
//     each user's projects, certificates, and export output. Folders
//     are chmod'd to 0700 on POSIX so other OS accounts cannot read.
//   - A login flow (Authenticate) and an account-creation flow
//     (CreateAccount) that the GUI and CLI both call.
//
// PMForge does NOT use OS user accounts. Multiple PMForge users on the
// same OS account is the supported model.
package users

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"pmforge/internal/auth"
	"pmforge/internal/sqlitedriver"
)

// ErrUserExists is returned by CreateAccount when the username is taken.
var ErrUserExists = errors.New("users: username already exists")

// ErrInvalidUsername is returned for usernames that fail validation.
var ErrInvalidUsername = errors.New("users: invalid username")

// ErrNoSuchUser is returned by Authenticate when the username does not
// exist. Callers SHOULD merge this with ErrMismatch in the UI to avoid
// leaking which usernames are valid.
var ErrNoSuchUser = errors.New("users: no such user")

// usernameRE accepts 3–32 chars of letters/digits/underscore/hyphen.
// Disallows path separators so it can safely be used as a folder name.
var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

// Account is the persisted user record.
type Account struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	DataDir     string    `json:"data_dir"`
	CreatedAt   time.Time `json:"created_at"`
	LastLogin   time.Time `json:"last_login"`
}

// Store is the connection to system.db. Construct one per process via
// Open(rootDir) and call Close before exit.
type Store struct {
	conn    *sql.DB
	rootDir string // ~/Documents/PMForge (absolute)
}

// Open opens (or creates) the system database at rootDir/system.db and
// runs the schema migration. rootDir is created if missing.
func Open(rootDir string) (*Store, error) {
	if err := ensurePrivateDir(rootDir); err != nil {
		return nil, fmt.Errorf("users: mkdir root: %w", err)
	}

	dbPath := filepath.Join(rootDir, "system.db")
	conn, err := sql.Open(sqlitedriver.Name, dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(`PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;`); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, fmt.Errorf("users: enable pragmas: %w; close: %v", err, closeErr)
		}
		return nil, fmt.Errorf("users: enable pragmas: %w", err)
	}

	s := &Store{conn: conn, rootDir: rootDir}
	if err := s.migrate(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, fmt.Errorf("users: migrate: %w; close: %v", err, closeErr)
		}
		return nil, fmt.Errorf("users: migrate: %w", err)
	}
	if err := ensurePrivateSQLiteFiles(dbPath); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, fmt.Errorf("users: private database file: %w; close: %v", err, closeErr)
		}
		return nil, fmt.Errorf("users: private database file: %w", err)
	}
	return s, nil
}

// Close releases the system DB connection.
func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// RootDir returns the configured PMForge root (~/Documents/PMForge).
func (s *Store) RootDir() string { return s.rootDir }

func (s *Store) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS users (
		username      TEXT PRIMARY KEY,
		display_name  TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		data_dir      TEXT NOT NULL,
		created_at    TEXT NOT NULL,
		last_login    TEXT NOT NULL DEFAULT ''
	);
	`
	if _, err := s.conn.Exec(schema); err != nil {
		return err
	}
	// V2.x — recovery codes table (recovery.go).
	if err := s.migrateRecoveryTable(); err != nil {
		return err
	}
	// V3 / ADR-001 — wrapped-DEK columns (dek.go).
	return s.migrateDEKColumns()
}

// ValidateUsername returns ErrInvalidUsername if name doesn't conform
// to the policy (3–32 chars; letters/digits/underscore/hyphen only).
func ValidateUsername(name string) error {
	if !usernameRE.MatchString(name) {
		return ErrInvalidUsername
	}
	return nil
}

// CreateAccount provisions a new user: hashes the password, creates
// ~/Documents/PMForge/<username>/{projects,certs,exports}/, and
// records the account in system.db.
//
// Returns ErrUserExists if username is already taken.
func (s *Store) CreateAccount(username, displayName, password string) (Account, error) {
	if err := ValidateUsername(username); err != nil {
		return Account{}, err
	}

	// Check duplicate — case-insensitive so that "Alice" and "alice" cannot
	// coexist and collide on case-insensitive filesystems (e.g. macOS APFS).
	var count int
	if err := s.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE lower(username) = lower(?)`, username).Scan(&count); err != nil {
		return Account{}, err
	}
	if count > 0 {
		return Account{}, ErrUserExists
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return Account{}, err
	}

	dataDir := filepath.Join(s.rootDir, username)
	for _, sub := range []string{"", "projects", "certs", "exports"} {
		path := filepath.Join(dataDir, sub)
		if err := ensurePrivateDir(path); err != nil {
			return Account{}, fmt.Errorf("users: provision %s: %w", path, err)
		}
	}

	now := time.Now().UTC()
	_, err = s.conn.Exec(
		`INSERT INTO users (username, display_name, password_hash, data_dir, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		username, strings.TrimSpace(displayName), hash, dataDir, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Account{}, err
	}

	return Account{
		Username:    username,
		DisplayName: displayName,
		DataDir:     dataDir,
		CreatedAt:   now,
	}, nil
}

// Authenticate verifies username + password against system.db. On
// success it updates last_login and returns the Account. On failure it
// returns ErrNoSuchUser or auth.ErrMismatch — callers should map both
// to the same "invalid credentials" message in the UI.
//
// If the stored hash was produced with weaker parameters than the
// current defaults, the function transparently re-hashes the password
// before returning.
func (s *Store) Authenticate(username, password string) (Account, error) {
	if err := ValidateUsername(username); err != nil {
		return Account{}, ErrNoSuchUser
	}

	var (
		acc       Account
		hash      string
		createdAt string
		lastLogin string
	)
	err := s.conn.QueryRow(
		`SELECT username, display_name, password_hash, data_dir, created_at, last_login
		 FROM users WHERE username = ?`,
		username,
	).Scan(&acc.Username, &acc.DisplayName, &hash, &acc.DataDir, &createdAt, &lastLogin)
	if err == sql.ErrNoRows {
		return Account{}, ErrNoSuchUser
	}
	if err != nil {
		return Account{}, err
	}

	if err := auth.VerifyPassword(password, hash); err != nil {
		return Account{}, err
	}

	// Parse timestamps (silently ignore parse errors — they're cosmetic).
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		acc.CreatedAt = t
	}
	if lastLogin != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastLogin); err == nil {
			acc.LastLogin = t
		}
	}

	// Update last_login.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.conn.Exec(`UPDATE users SET last_login = ? WHERE username = ?`, now, username); err != nil {
		return Account{}, fmt.Errorf("users: update last_login: %w", err)
	}

	// Transparent re-hash if parameters have been strengthened.
	if auth.NeedsRehash(hash) {
		newHash, err := auth.HashPassword(password)
		if err != nil {
			return Account{}, fmt.Errorf("users: rehash password: %w", err)
		}
		if _, err := s.conn.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, newHash, username); err != nil {
			return Account{}, fmt.Errorf("users: persist password rehash: %w", err)
		}
	}

	return acc, nil
}

// List returns every account on the system, ordered by username. Used
// by the GUI's user-switcher dropdown.
func (s *Store) List() ([]Account, error) {
	rows, err := s.conn.Query(
		`SELECT username, display_name, data_dir, created_at, last_login
		 FROM users ORDER BY username ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Account
	for rows.Next() {
		var (
			a                    Account
			createdAt, lastLogin string
		)
		if err := rows.Scan(&a.Username, &a.DisplayName, &a.DataDir, &createdAt, &lastLogin); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			a.CreatedAt = t
		}
		if lastLogin != "" {
			if t, err := time.Parse(time.RFC3339Nano, lastLogin); err == nil {
				a.LastLogin = t
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DefaultRootDir returns the canonical PMForge data root on the
// current platform. It prefers $XDG_DATA_HOME on Linux but falls back
// to ~/Documents/PMForge everywhere.
func DefaultRootDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "PMForge"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents", "PMForge"), nil
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700) // #nosec G302 -- this is a private directory mode, not a file mode.
}

func ensurePrivateSQLiteFiles(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if err := os.Chmod(sidecar, 0o600); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
