// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package catalog owns a signed-in user's reusable suppliers and items. It is
// intentionally separate from system.db (which is readable before login) and
// from individual project files (which must remain portable and self-contained).
package catalog

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	appcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/sqlitedriver"
)

const (
	maxSearchLength = 200
	maxFieldLength  = 1000
	maxResults      = 100
)

var ErrConflict = errors.New("catalog: record changed by another save")

type Vendor struct {
	ID, Name, Address, Phone, Fax, Email, PrimaryContact, Notes string
	Version                                                     int64
	Archived                                                    bool
	CreatedAt, UpdatedAt                                        string
}

type Item struct {
	ID, Name, SKU, Kind, DefaultUnit, Description string
	Version                                       int64
	Archived                                      bool
	CreatedAt, UpdatedAt                          string
}

type Store struct{ db *sql.DB }

// Open opens or creates the encrypted per-user catalog. It accepts the
// session DEK rather than a caller-provided database key so this package can
// domain-separate its SQLCipher key without exposing that concern to callers.
func Open(path string, sessionDEK []byte) (*Store, error) {
	if strings.ContainsAny(path, "?#") {
		return nil, fmt.Errorf("catalog: path contains an illegal character")
	}
	key, err := appcrypto.DeriveSubkey(sessionDEK, "gopmgr/catalog/sqlcipher/v1")
	if err != nil {
		return nil, err
	}
	defer zero(key)
	hexKey, err := appcrypto.KeyspecHex(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("catalog: create directory: %w", err)
	}
	dsn := path + "?_pragma_key=x'" + hexKey + "'&_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
	db, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		return nil, fmt.Errorf("catalog: open: %w", err)
	}
	if _, err = db.Exec("PRAGMA temp_store = MEMORY"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("catalog: connect: %w", err)
	}
	s := &Store{db: db}
	if err = s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = privateFiles(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS catalog_vendors (
			id TEXT PRIMARY KEY, version INTEGER NOT NULL, name TEXT NOT NULL,
			address TEXT NOT NULL DEFAULT '', phone TEXT NOT NULL DEFAULT '', fax TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '', primary_contact TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '',
			archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN (0,1)),
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_catalog_vendors_name ON catalog_vendors(name);
		CREATE TABLE IF NOT EXISTS catalog_items (
			id TEXT PRIMARY KEY, version INTEGER NOT NULL, name TEXT NOT NULL,
			sku TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT '', default_unit TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '', archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN (0,1)),
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_catalog_items_name ON catalog_items(name);
		CREATE INDEX IF NOT EXISTS idx_catalog_items_sku ON catalog_items(sku);`)
	if err != nil {
		return fmt.Errorf("catalog: migrate: %w", err)
	}
	return nil
}

func (s *Store) ListVendors(query string, includeArchived bool) ([]Vendor, error) {
	needle, err := searchNeedle(query)
	if err != nil {
		return nil, err
	}
	where := ""
	args := []any{}
	if !includeArchived {
		where = "archived=0"
	}
	if needle != "" {
		if where != "" {
			where += " AND "
		}
		where += "(name LIKE ? OR primary_contact LIKE ? OR email LIKE ? OR phone LIKE ?)"
		like := "%" + needle + "%"
		args = append(args, like, like, like, like)
	}
	q := `SELECT id,version,name,address,phone,fax,email,primary_contact,notes,archived,created_at,updated_at FROM catalog_vendors`
	if where != "" {
		q += " WHERE " + where
	}
	q += " ORDER BY name COLLATE NOCASE, id LIMIT 100"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Vendor
	for rows.Next() {
		var v Vendor
		var archived int
		if err := rows.Scan(&v.ID, &v.Version, &v.Name, &v.Address, &v.Phone, &v.Fax, &v.Email, &v.PrimaryContact, &v.Notes, &archived, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		v.Archived = archived != 0
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ListItems(query string, includeArchived bool) ([]Item, error) {
	needle, err := searchNeedle(query)
	if err != nil {
		return nil, err
	}
	where := ""
	args := []any{}
	if !includeArchived {
		where = "archived=0"
	}
	if needle != "" {
		if where != "" {
			where += " AND "
		}
		where += "(name LIKE ? OR sku LIKE ? OR kind LIKE ? OR default_unit LIKE ?)"
		like := "%" + needle + "%"
		args = append(args, like, like, like, like)
	}
	q := `SELECT id,version,name,sku,kind,default_unit,description,archived,created_at,updated_at FROM catalog_items`
	if where != "" {
		q += " WHERE " + where
	}
	q += " ORDER BY name COLLATE NOCASE, id LIMIT 100"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Item
	for rows.Next() {
		var v Item
		var archived int
		if err := rows.Scan(&v.ID, &v.Version, &v.Name, &v.SKU, &v.Kind, &v.DefaultUnit, &v.Description, &archived, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		v.Archived = archived != 0
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SaveVendor(v Vendor) (Vendor, error) {
	if err := validateVendor(v); err != nil {
		return Vendor{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if v.ID == "" {
		id, err := newID("vendor")
		if err != nil {
			return Vendor{}, err
		}
		v.ID, v.Version, v.CreatedAt = id, 1, now
		_, err = s.db.Exec(`INSERT INTO catalog_vendors (id,version,name,address,phone,fax,email,primary_contact,notes,archived,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, v.ID, v.Version, v.Name, v.Address, v.Phone, v.Fax, v.Email, v.PrimaryContact, v.Notes, boolInt(v.Archived), now, now)
		if err != nil {
			return Vendor{}, err
		}
		v.UpdatedAt = now
		return v, nil
	}
	if v.Version < 1 {
		return Vendor{}, errors.New("catalog: vendor version is required")
	}
	result, err := s.db.Exec(`UPDATE catalog_vendors SET version=version+1,name=?,address=?,phone=?,fax=?,email=?,primary_contact=?,notes=?,archived=?,updated_at=? WHERE id=? AND version=?`, v.Name, v.Address, v.Phone, v.Fax, v.Email, v.PrimaryContact, v.Notes, boolInt(v.Archived), now, v.ID, v.Version)
	if err != nil {
		return Vendor{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Vendor{}, err
	}
	if n != 1 {
		return Vendor{}, ErrConflict
	}
	v.Version++
	v.UpdatedAt = now
	return v, nil
}

func (s *Store) SaveItem(v Item) (Item, error) {
	if err := validateItem(v); err != nil {
		return Item{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if v.ID == "" {
		id, err := newID("item")
		if err != nil {
			return Item{}, err
		}
		v.ID, v.Version, v.CreatedAt = id, 1, now
		_, err = s.db.Exec(`INSERT INTO catalog_items (id,version,name,sku,kind,default_unit,description,archived,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, v.ID, v.Version, v.Name, v.SKU, v.Kind, v.DefaultUnit, v.Description, boolInt(v.Archived), now, now)
		if err != nil {
			return Item{}, err
		}
		v.UpdatedAt = now
		return v, nil
	}
	if v.Version < 1 {
		return Item{}, errors.New("catalog: item version is required")
	}
	result, err := s.db.Exec(`UPDATE catalog_items SET version=version+1,name=?,sku=?,kind=?,default_unit=?,description=?,archived=?,updated_at=? WHERE id=? AND version=?`, v.Name, v.SKU, v.Kind, v.DefaultUnit, v.Description, boolInt(v.Archived), now, v.ID, v.Version)
	if err != nil {
		return Item{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Item{}, err
	}
	if n != 1 {
		return Item{}, ErrConflict
	}
	v.Version++
	v.UpdatedAt = now
	return v, nil
}

func validateVendor(v Vendor) error {
	if v.Name = strings.TrimSpace(v.Name); v.Name == "" {
		return errors.New("catalog: vendor name is required")
	}
	if tooLong(v.Name, v.Address, v.Phone, v.Fax, v.Email, v.PrimaryContact, v.Notes) {
		return fmt.Errorf("catalog: field exceeds %d characters", maxFieldLength)
	}
	return nil
}
func validateItem(v Item) error {
	if v.Name = strings.TrimSpace(v.Name); v.Name == "" {
		return errors.New("catalog: item name is required")
	}
	v.DefaultUnit = strings.TrimSpace(v.DefaultUnit)
	if tooLong(v.Name, v.SKU, v.Kind, v.DefaultUnit, v.Description) {
		return fmt.Errorf("catalog: field exceeds %d characters", maxFieldLength)
	}
	return nil
}
func tooLong(values ...string) bool {
	for _, v := range values {
		if len(v) > maxFieldLength {
			return true
		}
	}
	return false
}
func searchNeedle(query string) (string, error) {
	q := strings.TrimSpace(query)
	if len(q) > maxSearchLength {
		return "", fmt.Errorf("catalog: search exceeds %d characters", maxSearchLength)
	}
	return q, nil
}
func newID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func privateFiles(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	for _, p := range []string{path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
