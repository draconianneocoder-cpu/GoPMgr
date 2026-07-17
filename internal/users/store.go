// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package users owns PMForge's local-multi-user system. It provides:
//
//   - A "system database" at <data-root>/system.db that lists every
//     PMForge account on this machine (username, display name, password
//     hash, data directory). The data root is ~/Library/Application
//     Support/PMForge on macOS and ~/Documents/PMForge elsewhere; see
//     DefaultRootDir.
//   - Per-user folders at <data-root>/<username>/ that hold each user's
//     projects, certificates, and export output. Folders are chmod'd to
//     0700 on POSIX so other OS accounts cannot read.
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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

// ErrNotAdmin is returned when the caller is not an administrator.
var ErrNotAdmin = errors.New("users: administrator privileges required")

// ErrLastAdmin is returned when an operation would leave the system
// with no administrator at all (e.g. demoting the only admin).
var ErrLastAdmin = errors.New("users: cannot remove the last administrator")

// Account is the persisted user record.
type Account struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	DataDir     string    `json:"data_dir"`
	CreatedAt   time.Time `json:"created_at"`
	LastLogin   time.Time `json:"last_login"`
	IsAdmin     bool      `json:"is_admin"`
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
	// Per-connection pragmas ride in the DSN so every connection the
	// *sql.DB pool opens gets them — a one-off conn.Exec would bind
	// foreign_keys=ON to a single physical connection only.
	conn, err := sql.Open(sqlitedriver.Name, dbPath+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
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
	if err := s.migrateDEKColumns(); err != nil {
		return err
	}
	// Admin role — additive column; safe to run on existing databases.
	return s.migrateAdminColumn()
}

// migrateAdminColumn adds is_admin to pre-admin databases. Idempotent.
func (s *Store) migrateAdminColumn() error {
	var cols []string
	rows, err := s.conn.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, typ, notnull string
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range cols {
		if c == "is_admin" {
			return nil // already present
		}
	}
	_, err = s.conn.Exec(`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`)
	return err
}

// ValidateUsername returns ErrInvalidUsername if name doesn't conform
// to the policy (3–32 chars; letters/digits/underscore/hyphen only).
func ValidateUsername(name string) error {
	if !usernameRE.MatchString(name) {
		return ErrInvalidUsername
	}
	return nil
}

// HasAnyAdmin reports whether at least one administrator account exists.
// Safe to call without authentication; used by the login and account-
// creation screens to decide whether to show the admin claim prompt.
func (s *Store) HasAnyAdmin() (bool, error) {
	var n int
	err := s.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&n)
	return n > 0, err
}

// SetAdmin promotes or demotes username. It returns ErrLastAdmin if the
// operation would leave the system with zero administrators (i.e. demoting
// the last admin). Demoting a non-admin is a no-op and never returns
// ErrLastAdmin.
func (s *Store) SetAdmin(username string, isAdmin bool) error {
	if !isAdmin {
		// Only guard the last-admin case when the target is currently an admin.
		var targetIsAdmin int
		if err := s.conn.QueryRow(`SELECT is_admin FROM users WHERE username = ?`, username).Scan(&targetIsAdmin); err != nil {
			return err
		}
		if targetIsAdmin == 1 {
			var n int
			if err := s.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&n); err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastAdmin
			}
		}
	}
	_, err := s.conn.Exec(`UPDATE users SET is_admin = ? WHERE username = ?`, boolToInt(isAdmin), username)
	return err
}

// DeleteAccount removes the account row from system.db. The foreign key
// CASCADE on recovery_codes and the wrapped-DEK column on users are
// handled automatically. The user's data directory is NOT removed —
// project files remain on disk and an administrator can access them
// through the filesystem.
//
// Returns ErrLastAdmin if deleting this account would leave no admins.
func (s *Store) DeleteAccount(username string) error {
	// Guard against orphaning the system by deleting the last admin.
	var isAdmin int
	if err := s.conn.QueryRow(`SELECT is_admin FROM users WHERE username = ?`, username).Scan(&isAdmin); err != nil {
		return err
	}
	if isAdmin == 1 {
		var n int
		if err := s.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&n); err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	_, err := s.conn.Exec(`DELETE FROM users WHERE username = ?`, username)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateAccount provisions a new user: hashes the password, creates
// ~/Documents/PMForge/<username>/{projects,certs,exports}/, and
// records the account in system.db.
//
// isAdmin marks the new account as an administrator. If an
// administrator already exists, callers MUST enforce that only an
// existing admin can set isAdmin=true (or at all create the account).
//
// Returns ErrUserExists if username is already taken.
func (s *Store) CreateAccount(username, displayName, password string, isAdmin bool) (Account, error) {
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
		`INSERT INTO users (username, display_name, password_hash, data_dir, created_at, is_admin)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		username, strings.TrimSpace(displayName), hash, dataDir, now.Format(time.RFC3339Nano), boolToInt(isAdmin),
	)
	if err != nil {
		return Account{}, err
	}

	return Account{
		Username:    username,
		DisplayName: strings.TrimSpace(displayName),
		DataDir:     dataDir,
		CreatedAt:   now,
		IsAdmin:     isAdmin,
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
		isAdmin   int
	)
	err := s.conn.QueryRow(
		`SELECT username, display_name, password_hash, data_dir, created_at, last_login, is_admin
		 FROM users WHERE username = ?`,
		username,
	).Scan(&acc.Username, &acc.DisplayName, &hash, &acc.DataDir, &createdAt, &lastLogin, &isAdmin)
	if err == sql.ErrNoRows {
		return Account{}, ErrNoSuchUser
	}
	if err != nil {
		return Account{}, err
	}

	if err := auth.VerifyPassword(password, hash); err != nil {
		return Account{}, err
	}

	acc.IsAdmin = isAdmin == 1

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
// by the GUI's user-switcher dropdown and admin panel.
func (s *Store) List() ([]Account, error) {
	rows, err := s.conn.Query(
		`SELECT username, display_name, data_dir, created_at, last_login, is_admin
		 FROM users ORDER BY username ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Account
	for rows.Next() {
		var (
			a                    Account
			createdAt, lastLogin string
			isAdmin              int
		)
		if err := rows.Scan(&a.Username, &a.DisplayName, &a.DataDir, &createdAt, &lastLogin, &isAdmin); err != nil {
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
		a.IsAdmin = isAdmin == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// DefaultRootDir returns the canonical PMForge data root on the current
// platform. $XDG_DATA_HOME overrides everywhere (Linux convention and a
// test hook). Otherwise:
//
//   - macOS: ~/Library/Application Support/PMForge. The old default,
//     ~/Documents/PMForge, is both iCloud-synced (so system.db can sync
//     between Macs or be evicted to a dataless placeholder) and TCC-
//     protected, which broke first-run account creation and code-signing.
//     Application Support is the Apple-sanctioned location for app data and
//     is neither synced nor privacy-gated.
//   - Linux / Windows: ~/Documents/PMForge (unchanged).
func DefaultRootDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "PMForge"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "PMForge"), nil
	}
	return filepath.Join(home, "Documents", "PMForge"), nil
}

// legacyMacRootDir returns the pre-relocation macOS data directory
// (~/Documents/PMForge). It returns "" on non-macOS hosts, when an explicit
// $XDG_DATA_HOME override is in effect, or when the home directory cannot be
// resolved — i.e. whenever there is nothing to migrate from.
func legacyMacRootDir() string {
	if runtime.GOOS != "darwin" || os.Getenv("XDG_DATA_HOME") != "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Documents", "PMForge")
}

// MigrateLegacyRoot moves a pre-relocation macOS install into newRoot. It is
// a no-op unless the host is macOS, newRoot has no system.db yet, and the
// legacy ~/Documents/PMForge location does have one. When it runs it copies
// the legacy tree into newRoot (leaving the original untouched, so an
// iCloud-evicted or half-synced source can never cause data loss and the
// user can delete the old copy at leisure) and reports whether a migration
// happened. Safe to call on every startup: once newRoot has a system.db it
// returns (false, nil) immediately.
func MigrateLegacyRoot(newRoot string) (bool, error) {
	return migrateLegacyRoot(legacyMacRootDir(), newRoot)
}

func migrateLegacyRoot(legacy, newRoot string) (bool, error) {
	if legacy == "" || legacy == newRoot {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(newRoot, "system.db")); err == nil {
		return false, nil // new location already initialised — nothing to do
	}
	if _, err := os.Stat(filepath.Join(legacy, "system.db")); err != nil {
		return false, nil // no legacy install to migrate
	}
	if err := copyTree(legacy, newRoot); err != nil {
		return false, fmt.Errorf("users: migrate legacy data root: %w", err)
	}
	return true, nil
}

// copyTree recursively copies the regular files and directories under src
// into dst, preserving permission bits. Symlinks are skipped (PMForge's data
// tree contains none). Reading each source file materialises any iCloud
// dataless placeholder, so the copy always contains real bytes.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode()&fs.ModeSymlink != 0:
			return nil // skip symlinks
		case info.Mode().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			return nil // skip sockets, devices, and other irregular files
		}
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src) // #nosec G304 -- src is under the user's own legacy data root.
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec G304 -- dst is under the user's own data root.
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
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
