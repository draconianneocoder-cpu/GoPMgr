// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrAccountIdentity means identity metadata is incompatible or inconsistent.
// Reopening must not silently issue replacement identities.
var ErrAccountIdentity = errors.New("users: invalid account identity state")

// Exact definitions are checked on reopen, including guards. NULL username
// means permanently retired, never a reusable allocation slot.
var accountIdentitySchema = []struct{ name, kind, definition string }{
	{"account_identity_meta", "table", `CREATE TABLE account_identity_meta(singleton INTEGER PRIMARY KEY CHECK(singleton=1),version INTEGER NOT NULL CHECK(version=1))`},
	{"account_identity_registry", "table", `CREATE TABLE account_identity_registry(
id TEXT NOT NULL PRIMARY KEY CHECK(typeof(id)='text' AND length(id)=68 AND instr(id,char(0))=0 AND substr(id,1,4)='ac1:' AND substr(id,5) NOT GLOB '*[^0-9a-f]*'),
username TEXT UNIQUE REFERENCES users(username) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED,
generation INTEGER,
CHECK((username IS NULL AND generation IS NULL) OR (typeof(username)='text' AND typeof(generation)='integer' AND generation>=1)))`},
	{"account_identity_meta_insert", "trigger", `CREATE TRIGGER account_identity_meta_insert BEFORE INSERT ON account_identity_meta
WHEN EXISTS(SELECT 1 FROM account_identity_meta) OR NEW.singleton IS NOT 1 OR NEW.version IS NOT 1
BEGIN SELECT RAISE(ABORT,'invalid identity readiness'); END`},
	{"account_identity_meta_update", "trigger", `CREATE TRIGGER account_identity_meta_update BEFORE UPDATE ON account_identity_meta
BEGIN SELECT RAISE(ABORT,'immutable identity readiness'); END`},
	{"account_identity_meta_delete", "trigger", `CREATE TRIGGER account_identity_meta_delete BEFORE DELETE ON account_identity_meta
BEGIN SELECT RAISE(ABORT,'immutable identity readiness'); END`},
	{"account_identity_registry_insert", "trigger", `CREATE TRIGGER account_identity_registry_insert BEFORE INSERT ON account_identity_registry
WHEN typeof(NEW.id)!='text' OR length(NEW.id)!=68 OR instr(NEW.id,char(0))!=0 OR substr(NEW.id,1,4)!='ac1:' OR substr(NEW.id,5) GLOB '*[^0-9a-f]*'
OR typeof(NEW.username)!='text' OR typeof(NEW.generation)!='integer' OR NEW.generation!=1
OR EXISTS(SELECT 1 FROM account_identity_registry WHERE id=NEW.id OR username=NEW.username)
BEGIN SELECT RAISE(ABORT,'invalid identity allocation'); END`},
	{"account_identity_registry_update", "trigger", `CREATE TRIGGER account_identity_registry_update BEFORE UPDATE ON account_identity_registry
WHEN NEW.id IS NOT OLD.id OR OLD.username IS NULL OR NOT (
(NEW.username IS OLD.username AND typeof(OLD.generation)='integer' AND OLD.generation>=1 AND OLD.generation<9223372036854775807
AND typeof(NEW.generation)='integer' AND NEW.generation=OLD.generation+1)
OR (NEW.username IS NULL AND NEW.generation IS NULL))
BEGIN SELECT RAISE(ABORT,'invalid identity transition'); END`},
	{"account_identity_registry_delete", "trigger", `CREATE TRIGGER account_identity_registry_delete BEFORE DELETE ON account_identity_registry
BEGIN SELECT RAISE(ABORT,'account identities cannot be reused'); END`},
	{"account_identity_users_insert", "trigger", `CREATE TRIGGER account_identity_users_insert BEFORE INSERT ON users
WHEN EXISTS(SELECT 1 FROM users WHERE username=NEW.username)
OR NOT EXISTS(SELECT 1 FROM account_identity_meta WHERE singleton=1 AND version=1)
OR NOT EXISTS(SELECT 1 FROM account_identity_registry WHERE username=NEW.username AND generation=1)
BEGIN SELECT RAISE(ABORT,'account creation requires an identity-capable version'); END`},
	{"account_identity_users_delete", "trigger", `CREATE TRIGGER account_identity_users_delete BEFORE DELETE ON users
BEGIN UPDATE account_identity_registry SET username=NULL,generation=NULL WHERE username=OLD.username; END`},
	{"account_identity_users_rename", "trigger", `CREATE TRIGGER account_identity_users_rename BEFORE UPDATE OF username ON users
WHEN NEW.username IS NOT OLD.username
BEGIN SELECT RAISE(ABORT,'account identity cannot be renamed'); END`},
	{"account_identity_users_state", "trigger", `CREATE TRIGGER account_identity_users_state AFTER UPDATE OF disabled,is_admin,password_hash,wrapped_dek_pw ON users
WHEN NEW.disabled IS NOT OLD.disabled OR NEW.is_admin IS NOT OLD.is_admin
OR NEW.password_hash IS NOT OLD.password_hash OR NEW.wrapped_dek_pw IS NOT OLD.wrapped_dek_pw
BEGIN
SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM account_identity_registry WHERE username=OLD.username)
THEN RAISE(ABORT,'missing account identity') END;
UPDATE account_identity_registry SET generation=generation+1 WHERE username=OLD.username;
END`},
}

func accountIdentityObjectCount(ctx context.Context, q accountWriter) (int, error) {
	var count int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name GLOB 'account_identity_*'`).Scan(&count)
	return count, err
}

// Validate initialized state before any older migration can write to it.
func (s *Store) checkExistingAccountIdentity() error {
	return s.inWriteTx("account identity validation", func(ctx context.Context, q accountWriter) error {
		count, err := accountIdentityObjectCount(ctx, q)
		if err != nil || count == 0 {
			return err
		}
		return validateAccountIdentity(ctx, q)
	})
}

func (s *Store) migrateAccountIdentity() error {
	return s.inWriteTx("account identity migration", func(ctx context.Context, q accountWriter) error {
		count, err := accountIdentityObjectCount(ctx, q)
		if err != nil {
			return err
		}
		if count != 0 {
			return validateAccountIdentity(ctx, q)
		}
		for _, object := range accountIdentitySchema {
			if _, err := q.ExecContext(ctx, object.definition); err != nil {
				return fmt.Errorf("create %s: %w", object.name, err)
			}
		}
		rows, err := q.QueryContext(ctx, `SELECT username FROM users ORDER BY username`)
		if err != nil {
			return err
		}
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				_ = rows.Close()
				return err
			}
			names = append(names, name)
		}
		iterationErr := rows.Err()
		if err := rows.Close(); err != nil {
			return errors.Join(iterationErr, err)
		}
		if iterationErr != nil {
			return iterationErr
		}
		for _, name := range names {
			if err := allocateAccountIdentity(ctx, q, name); err != nil {
				return err
			}
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO account_identity_meta(singleton,version) VALUES(1,1)`); err != nil {
			return err
		}
		return validateAccountIdentity(ctx, q)
	})
}

func allocateAccountIdentity(ctx context.Context, q accountWriter, username string) error {
	var random [32]byte
	if _, err := io.ReadFull(rand.Reader, random[:]); err != nil {
		return fmt.Errorf("users: read account identity entropy: %w", err)
	}
	id := "ac1:" + hex.EncodeToString(random[:])
	if _, err := q.ExecContext(ctx, `INSERT INTO account_identity_registry(id,username,generation) VALUES(?,?,1)`, id, username); err != nil {
		return fmt.Errorf("users: allocate account identity: %w", err)
	}
	return nil
}

func validateAccountIdentity(ctx context.Context, q accountWriter) error {
	count, err := accountIdentityObjectCount(ctx, q)
	if err != nil {
		return err
	}
	if count != len(accountIdentitySchema) {
		return fmt.Errorf("%w: incomplete or unknown schema", ErrAccountIdentity)
	}
	for _, object := range accountIdentitySchema {
		var actual string
		if err := q.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type=? AND name=?`, object.kind, object.name).Scan(&actual); err != nil {
			return fmt.Errorf("%w: read %s: %w", ErrAccountIdentity, object.name, err)
		}
		if actual != strings.TrimSpace(object.definition) {
			return fmt.Errorf("%w: incompatible %s", ErrAccountIdentity, object.name)
		}
	}
	var total, ready int
	if err := q.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(singleton=1 AND version=1),0) FROM account_identity_meta`).Scan(&total, &ready); err != nil {
		return err
	}
	if total != 1 || ready != 1 {
		return fmt.Errorf("%w: invalid readiness marker", ErrAccountIdentity)
	}
	for _, query := range []string{
		`SELECT count(*) FROM account_identity_registry WHERE typeof(id)!='text' OR length(id)!=68 OR instr(id,char(0))!=0 OR substr(id,1,4)!='ac1:' OR substr(id,5) GLOB '*[^0-9a-f]*'
OR (username IS NULL AND generation IS NOT NULL)
OR (username IS NOT NULL AND (typeof(username)!='text' OR typeof(generation)!='integer' OR generation<1))`,
		`SELECT count(*) FROM users u WHERE NOT EXISTS(SELECT 1 FROM account_identity_registry i WHERE i.username=u.username)`,
		`SELECT count(*) FROM account_identity_registry i WHERE i.username IS NOT NULL AND NOT EXISTS(SELECT 1 FROM users u WHERE u.username=i.username)`,
	} {
		var invalid int
		if err := q.QueryRowContext(ctx, query).Scan(&invalid); err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%w: inconsistent account bindings", ErrAccountIdentity)
		}
	}
	return nil
}
