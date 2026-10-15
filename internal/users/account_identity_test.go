// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"

	"gopmgr/internal/sqlitedriver"
)

// These tests are serial: the existing entropy and account-rule hooks are global.
// The legacy fixture exercises only the identity phase, not atomicity of Open's
// unrelated historical migrations. Its child rows expose accidental rebuilding
// of users with foreign keys enabled.
func legacyAccountIdentityStore(t *testing.T, names ...string) *Store {
	t.Helper()
	s := accountIdentityHandle(t, t.TempDir(), true)
	const legacy = `
CREATE TABLE users (
 username TEXT PRIMARY KEY, display_name TEXT NOT NULL, password_hash TEXT NOT NULL,
 data_dir TEXT NOT NULL, created_at TEXT NOT NULL, last_login TEXT NOT NULL DEFAULT '',
 wrapped_dek_pw TEXT NOT NULL DEFAULT '', is_admin INTEGER NOT NULL DEFAULT 0,
 last_export_directory TEXT NOT NULL DEFAULT '', disabled INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE recovery_codes (
 id INTEGER PRIMARY KEY, username TEXT REFERENCES users(username) ON DELETE CASCADE,
 code_hash TEXT, used INTEGER, used_at TEXT, created_at TEXT, wrapped_dek TEXT
);
CREATE TABLE personal_keys (
 username TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
 public_key BLOB, wrapped_private_key BLOB, attestation BLOB
);
CREATE TABLE escrow_keys (id TEXT PRIMARY KEY, public_key BLOB);
CREATE TABLE sealed_deks (
 username TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
 escrow_key_id TEXT REFERENCES escrow_keys(id), sealed BLOB
);
CREATE TABLE account_events (
 id INTEGER PRIMARY KEY, occurred_at TEXT, actor TEXT, username TEXT, action TEXT, detail TEXT
);
INSERT INTO escrow_keys VALUES('legacy-key', x'001122ff');`
	identityExec(t, s, legacy)
	for i, name := range names {
		identityExec(t, s, `INSERT INTO users VALUES(?,?,?,?,?,?,?,?,?,?)`, name, "display "+name,
			"hash "+name, "/old/root/"+name, "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z",
			"password wrap "+name, i%2, "/exports/"+name, i%2)
		identityExec(t, s, `INSERT INTO recovery_codes VALUES(?,?,?,?,?,?,?)`, i+1, name,
			"recovery hash "+name, i%2, "used timestamp", "created timestamp", "recovery wrap "+name)
		identityExec(t, s, `INSERT INTO personal_keys VALUES(?,?,?,?)`, name, []byte{0, byte(i), 255}, []byte("private "+name), []byte("attestation "+name))
		identityExec(t, s, `INSERT INTO sealed_deks VALUES(?,?,?)`, name, "legacy-key", []byte("sealed "+name))
		identityExec(t, s, `INSERT INTO account_events VALUES(?,?,?,?,?,?)`, i+1, "2026-01-01T00:00:00Z", name, name, AccountCreated, "legacy event")
	}
	return s
}

func accountIdentityHandle(t *testing.T, root string, foreignKeys bool) *Store {
	t.Helper()
	dsn := fmt.Sprintf("%s?_foreign_keys=%t&_journal_mode=WAL&_busy_timeout=5000", filepath.Join(root, "system.db"), foreignKeys)
	conn, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	s := &Store{conn: conn, rootDir: root}
	t.Cleanup(func() { _ = s.Close() })
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	var enabled bool
	if err := conn.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil || enabled != foreignKeys {
		t.Fatalf("foreign_keys = %v, err=%v; want %v", enabled, err, foreignKeys)
	}
	return s
}

func identityExec(t *testing.T, s *Store, query string, args ...any) {
	t.Helper()
	if _, err := s.conn.Exec(query, args...); err != nil {
		t.Fatalf("fixture SQL: %v", err)
	}
}

func identityQuery(t *testing.T, s *Store, query string) [][]any {
	t.Helper()
	rows, err := s.conn.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	result := make([][]any, 0)
	for rows.Next() {
		row := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range row {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// Capture every persisted table, including audit sequences and child metadata,
// and actual schema SQL. Assertions do not copy the production migration SQL.
func identitySnapshot(t *testing.T, s *Store) map[string][][]any {
	t.Helper()
	state := map[string][][]any{
		"schema": identityQuery(t, s, `SELECT type,name,tbl_name,sql FROM sqlite_master ORDER BY name`),
	}
	for _, row := range identityQuery(t, s, `SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`) {
		name := row[0].(string)
		state[name] = identityQuery(t, s, `SELECT * FROM "`+strings.ReplaceAll(name, `"`, `""`)+`" ORDER BY rowid`)
	}
	return state
}

func assertIdentitySnapshot(t *testing.T, s *Store, want map[string][][]any) {
	t.Helper()
	if got := identitySnapshot(t, s); !reflect.DeepEqual(got, want) {
		for name, rows := range want {
			if !reflect.DeepEqual(got[name], rows) {
				t.Errorf("persisted %s changed: got=%v want=%v", name, got[name], rows)
			}
		}
		for name := range got {
			if _, exists := want[name]; !exists {
				t.Errorf("unexpected persisted table %s", name)
			}
		}
		t.FailNow()
	}
}

func readAccountIdentity(t *testing.T, s *Store, name string) (string, int64) {
	t.Helper()
	var id string
	var generation int64
	if err := s.conn.QueryRow(`SELECT id,generation FROM account_identity_registry WHERE username=?`, name).Scan(&id, &generation); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^ac1:[0-9a-f]{64}$`).MatchString(id) || generation < 1 {
		t.Fatalf("invalid active identity: id=%q generation=%d", id, generation)
	}
	return id, generation
}

func assertRetiredAccountIdentity(t *testing.T, s *Store, id string) {
	t.Helper()
	var name sql.NullString
	var generation sql.NullInt64
	if err := s.conn.QueryRow(`SELECT username,generation FROM account_identity_registry WHERE id=?`, id).Scan(&name, &generation); err != nil {
		t.Fatal(err)
	}
	if name.Valid || generation.Valid {
		t.Fatalf("retired identity retains association: username=%v generation=%v", name, generation)
	}
}

// Deliberate corruption only: temporarily remove exactly one production guard,
// then restore its actual sqlite_master definition, without a test-side copy.
func corruptAccountIdentity(t *testing.T, s *Store, guard, query string, args ...any) {
	t.Helper()
	var definition string
	if err := s.conn.QueryRow(`SELECT sql FROM sqlite_master WHERE type='trigger' AND name=?`, guard).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	identityExec(t, s, `DROP TRIGGER "`+guard+`"`)
	identityExec(t, s, query, args...)
	identityExec(t, s, definition)
}

func TestAccountIdentityMigrationPreservesLegacyRows(t *testing.T) {
	s := legacyAccountIdentityStore(t, "Alice", "alice", "disabled_user")
	before := identitySnapshot(t, s)
	if err := s.migrateAccountIdentity(); err != nil {
		t.Fatal(err)
	}
	for table, want := range before {
		if table != "schema" && !reflect.DeepEqual(identitySnapshot(t, s)[table], want) {
			t.Fatalf("migration changed legacy %s", table)
		}
	}
	wantNames := []string{"Alice", "alice", "disabled_user"}
	ids := make(map[string]bool)
	for _, name := range wantNames {
		id, generation := readAccountIdentity(t, s, name)
		if ids[id] || generation != 1 {
			t.Fatalf("backfill identity duplicate=%v generation=%d", ids[id], generation)
		}
		ids[id] = true
	}
	if got := identityQuery(t, s, `SELECT username FROM account_identity_registry ORDER BY username`); len(got) != len(wantNames) {
		t.Fatalf("registry coverage = %v", got)
	}
	if got := identityQuery(t, s, `SELECT singleton,version FROM account_identity_meta`); !reflect.DeepEqual(got, [][]any{{int64(1), int64(1)}}) {
		t.Fatalf("readiness = %v", got)
	}
	wantObjects := []string{
		"account_identity_meta", "account_identity_registry",
		"account_identity_meta_insert", "account_identity_meta_update", "account_identity_meta_delete",
		"account_identity_registry_insert", "account_identity_registry_update", "account_identity_registry_delete",
		"account_identity_users_insert", "account_identity_users_delete", "account_identity_users_state", "account_identity_users_rename",
	}
	slices.Sort(wantObjects)
	var gotObjects []string
	for _, row := range identityQuery(t, s, `SELECT name FROM sqlite_master WHERE name GLOB 'account_identity_*' ORDER BY name`) {
		gotObjects = append(gotObjects, row[0].(string))
	}
	if !reflect.DeepEqual(gotObjects, wantObjects) {
		t.Fatalf("identity objects = %v, want %v", gotObjects, wantObjects)
	}
	if rows := identityQuery(t, s, `PRAGMA foreign_key_check`); len(rows) != 0 {
		t.Fatalf("foreign key violations = %v", rows)
	}
	persisted := identitySnapshot(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := accountIdentityHandle(t, s.RootDir(), true)
	if err := reopened.migrateAccountIdentity(); err != nil {
		t.Fatal(err)
	}
	assertIdentitySnapshot(t, reopened, persisted)
}

func TestAccountIdentityConcurrentInitialization(t *testing.T) {
	s := legacyAccountIdentityStore(t, "Alice", "alice", "carol")
	other := accountIdentityHandle(t, s.RootDir(), true)
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, handle := range []*Store{s, other} {
		workers.Go(func() {
			<-start
			results <- handle.migrateAccountIdentity()
		})
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent identity initialization: %v", err)
		}
	}
	for _, name := range []string{"Alice", "alice", "carol"} {
		id, generation := readAccountIdentity(t, s, name)
		otherID, otherGeneration := readAccountIdentity(t, other, name)
		if id != otherID || generation != 1 || otherGeneration != 1 {
			t.Fatalf("initializers disagree for %s", name)
		}
	}
	before := identitySnapshot(t, s)
	if err := other.migrateAccountIdentity(); err != nil {
		t.Fatal(err)
	}
	assertIdentitySnapshot(t, s, before)
}

func TestAccountIdentityBackfillFailureRollsBackEntirePhase(t *testing.T) {
	for _, mode := range []string{"entropy", "collision"} {
		t.Run(mode, func(t *testing.T) {
			s := legacyAccountIdentityStore(t, "alice", "bob", "carol")
			before := identitySnapshot(t, s)
			data := bytes.Repeat([]byte{0x37}, 64)
			if mode == "entropy" {
				data = data[:39] // one identity, then a short second read
			}
			reader := bytes.NewReader(data)
			restore := replaceRecoveryRandReader(t, reader)
			t.Cleanup(restore)
			err := s.migrateAccountIdentity()
			restore()
			if mode == "entropy" {
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("mid-backfill error = %v, want short entropy read", err)
				}
			} else {
				var constraint sqlite3.Error
				if !errors.As(err, &constraint) || constraint.Code != sqlite3.ErrConstraint {
					t.Fatalf("mid-backfill collision = %v, want constraint refusal", err)
				}
			}
			if reader.Len() != 0 {
				t.Fatal("failure did not reach the second allocation")
			}
			assertIdentitySnapshot(t, s, before)
			// No retry is part of the protocol. A separate explicit migration
			// after restoring real entropy must still initialize cleanly.
			if err := s.migrateAccountIdentity(); err != nil {
				t.Fatalf("explicit subsequent migration: %v", err)
			}
			for _, name := range []string{"alice", "bob", "carol"} {
				readAccountIdentity(t, s, name)
			}
		})
	}
}

func TestAccountIdentityPartialOrUnknownSchemaFailsClosed(t *testing.T) {
	for _, ddl := range []string{
		`CREATE TABLE account_identity_meta(singleton INTEGER,version INTEGER)`,
		`CREATE TABLE account_identity_registry(id TEXT)`,
		`CREATE VIEW account_identity_registry AS SELECT username FROM users`,
		`CREATE TABLE account_identity_future(data TEXT)`,
		`CREATE TRIGGER account_identity_users_insert BEFORE INSERT ON users BEGIN SELECT 1; END`,
	} {
		t.Run(ddl, func(t *testing.T) {
			s := legacyAccountIdentityStore(t, "alice")
			identityExec(t, s, ddl)
			before := identitySnapshot(t, s)
			if err := s.migrateAccountIdentity(); !errors.Is(err, ErrAccountIdentity) {
				t.Fatalf("partial migration error = %v", err)
			}
			assertIdentitySnapshot(t, s, before)
			reopened, err := Open(s.RootDir())
			if reopened != nil {
				_ = reopened.Close()
			}
			if !errors.Is(err, ErrAccountIdentity) {
				t.Fatalf("Open partial identity state = %v", err)
			}
			// Early validation must precede unrelated legacy migrations.
			assertIdentitySnapshot(t, s, before)
		})
	}
}

func TestAccountIdentityDamagedStateIsNotHealed(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(*testing.T, *Store)
	}{
		{"missing_metadata", func(t *testing.T, s *Store) {
			corruptAccountIdentity(t, s, "account_identity_meta_delete", `DELETE FROM account_identity_meta`)
		}},
		{"extra_metadata", func(t *testing.T, s *Store) {
			identityExec(t, s, `PRAGMA ignore_check_constraints=ON`)
			corruptAccountIdentity(t, s, "account_identity_meta_insert", `INSERT INTO account_identity_meta VALUES(2,1)`)
			identityExec(t, s, `PRAGMA ignore_check_constraints=OFF`)
		}},
		{"unknown_version", func(t *testing.T, s *Store) {
			identityExec(t, s, `PRAGMA ignore_check_constraints=ON`)
			corruptAccountIdentity(t, s, "account_identity_meta_update", `UPDATE account_identity_meta SET version=2`)
			identityExec(t, s, `PRAGMA ignore_check_constraints=OFF`)
		}},
		{"coverage_loss", func(t *testing.T, s *Store) {
			corruptAccountIdentity(t, s, "account_identity_registry_delete", `DELETE FROM account_identity_registry WHERE username='alice'`)
		}},
		{"changed_guard", func(t *testing.T, s *Store) {
			identityExec(t, s, `DROP TRIGGER account_identity_users_state`)
			identityExec(t, s, `CREATE TRIGGER account_identity_users_state AFTER UPDATE ON users BEGIN SELECT 1; END`)
		}},
		{"unknown_object", func(t *testing.T, s *Store) {
			identityExec(t, s, `CREATE TABLE account_identity_future(data TEXT)`)
		}},
		{"embedded_NUL_identity", func(t *testing.T, s *Store) {
			id, _ := readAccountIdentity(t, s, "alice")
			identityExec(t, s, `PRAGMA ignore_check_constraints=ON`)
			corruptAccountIdentity(t, s, "account_identity_registry_update", `UPDATE account_identity_registry SET id=? WHERE username='alice'`, id+"\x00unexpected")
			identityExec(t, s, `PRAGMA ignore_check_constraints=OFF`)
		}},
	}
	for _, guard := range []string{
		"account_identity_users_insert", "account_identity_users_delete", "account_identity_users_state", "account_identity_users_rename",
		"account_identity_registry_insert", "account_identity_registry_update", "account_identity_registry_delete",
		"account_identity_meta_insert", "account_identity_meta_update", "account_identity_meta_delete",
	} {
		cases = append(cases, struct {
			name    string
			corrupt func(*testing.T, *Store)
		}{
			"missing_" + guard, func(t *testing.T, s *Store) { identityExec(t, s, `DROP TRIGGER `+guard) },
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := openTestStore(t)
			if _, err := s.CreateAccount("alice", "Alice", rulePassword, false); err != nil {
				t.Fatal(err)
			}
			tc.corrupt(t, s)
			before := identitySnapshot(t, s)
			if err := s.migrateAccountIdentity(); !errors.Is(err, ErrAccountIdentity) {
				t.Fatalf("damaged identity migration = %v", err)
			}
			assertIdentitySnapshot(t, s, before)
			reopened, err := Open(s.RootDir())
			if reopened != nil {
				_ = reopened.Close()
			}
			if !errors.Is(err, ErrAccountIdentity) {
				t.Fatalf("damaged identity Open = %v", err)
			}
			assertIdentitySnapshot(t, s, before)
		})
	}
}

func TestAccountIdentityBothCreatorsPreserveTheirContracts(t *testing.T) {
	for _, creator := range []string{"helper", "production"} {
		t.Run(creator, func(t *testing.T) {
			s := openTestStore(t)
			var acc Account
			var err error
			if creator == "helper" {
				acc, err = s.CreateAccount("alice", "Alice", rulePassword, false)
			} else {
				var dek []byte
				acc, dek, err = s.CreateAccountWithKey("", "alice", "Alice", rulePassword, false, nil)
				defer clear(dek)
				if err == nil && len(dek) != 32 {
					t.Fatalf("creation DEK length = %d", len(dek))
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if acc.IsAdmin != (creator == "production") {
				t.Fatalf("first account admin=%v, creator=%s", acc.IsAdmin, creator)
			}
			id, _ := readAccountIdentity(t, s, "alice")
			events, err := s.AccountEvents()
			if err != nil {
				t.Fatal(err)
			}
			if creator == "helper" && len(events) != 0 {
				t.Fatalf("helper wrote creation audit: %v", events)
			}
			if creator == "production" && (len(events) != 1 || events[0].Action != AccountCreated || events[0].Actor != "alice" || events[0].Username != "alice") {
				t.Fatalf("production creation audit = %v", events)
			}
			encoded, err := json.Marshal(acc)
			if err != nil {
				t.Fatal(err)
			}
			var dto map[string]any
			if err := json.Unmarshal(encoded, &dto); err != nil {
				t.Fatal(err)
			}
			wantFields := []string{"username", "display_name", "data_dir", "last_export_directory", "created_at", "last_login", "is_admin", "disabled"}
			if len(dto) != len(wantFields) || bytes.Contains(encoded, []byte(id)) {
				t.Fatalf("account DTO gained identity fields: %s", encoded)
			}
			for _, field := range wantFields {
				if _, ok := dto[field]; !ok {
					t.Fatalf("account DTO missing %s", field)
				}
			}
			before := identitySnapshot(t, s)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.RootDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			assertIdentitySnapshot(t, reopened, before)
		})
	}
}

func TestAccountIdentityPurgeAndRecreateRetainsTombstone(t *testing.T) {
	s := newStatusStore(t)
	oldID, _ := readAccountIdentity(t, s, "bob")
	if err := s.PurgeAccount("alice", "bob"); err != nil {
		t.Fatal(err)
	}
	assertRetiredAccountIdentity(t, s, oldID)
	assertNoAccountFolder(t, s, "bob")
	assertEvents(t, s, "alice purged bob")
	if _, err := s.CreateAccountAs("alice", "bob", "New Bob", rulePassword, false); err != nil {
		t.Fatal(err)
	}
	newID, _ := readAccountIdentity(t, s, "bob")
	if newID == oldID {
		t.Fatal("purge/recreate reused the retired identity")
	}
	assertRetiredAccountIdentity(t, s, oldID)
	assertEvents(t, s, "alice purged bob", "alice created bob")
}

func TestAccountIdentityRawDeleteRetiresWithForeignKeysOnOrOff(t *testing.T) {
	for _, fk := range []bool{true, false} {
		t.Run(fmt.Sprintf("foreign_keys_%t", fk), func(t *testing.T) {
			s := legacyAccountIdentityStore(t, "alice", "bob")
			if err := s.migrateAccountIdentity(); err != nil {
				t.Fatal(err)
			}
			id, _ := readAccountIdentity(t, s, "bob")
			writer := accountIdentityHandle(t, s.RootDir(), fk)
			identityExec(t, writer, `DELETE FROM users WHERE username='bob'`)
			assertRetiredAccountIdentity(t, s, id)
		})
	}
}

func TestAccountIdentityOldWritersAndReplacementAreRefused(t *testing.T) {
	s := legacyAccountIdentityStore(t, "alice", "bob")
	if err := s.migrateAccountIdentity(); err != nil {
		t.Fatal(err)
	}
	id, _ := readAccountIdentity(t, s, "alice")
	queries := []struct {
		name, query string
		args        []any
	}{
		{"old_insert", `INSERT INTO users(username,display_name,password_hash,data_dir,created_at) VALUES('carol','Carol','hash','/carol','now')`, nil},
		{"users_replace", `INSERT OR REPLACE INTO users SELECT * FROM users WHERE username='alice'`, nil},
		{"registry_replace", `INSERT OR REPLACE INTO account_identity_registry(id,username,generation) VALUES(?,?,1)`, []any{id, "alice"}},
		{"registry_delete", `DELETE FROM account_identity_registry WHERE id=?`, []any{id}},
		{"rename", `UPDATE OR REPLACE users SET username='carol' WHERE username='alice'`, nil},
		{"meta_replace", `INSERT OR REPLACE INTO account_identity_meta VALUES(1,1)`, nil},
		{"meta_update", `UPDATE account_identity_meta SET version=1`, nil},
		{"meta_delete", `DELETE FROM account_identity_meta`, nil},
	}
	for _, tc := range queries {
		t.Run(tc.name, func(t *testing.T) {
			for _, fk := range []bool{true, false} {
				t.Run(fmt.Sprintf("foreign_keys_%t", fk), func(t *testing.T) {
					writer := accountIdentityHandle(t, s.RootDir(), fk)
					before := identitySnapshot(t, s)
					if _, err := writer.conn.Exec(tc.query, tc.args...); err == nil {
						t.Fatal("unsupported account writer succeeded")
					}
					assertIdentitySnapshot(t, s, before)
				})
			}
		})
	}
}

func TestAccountIdentityLiveCreatorsRequireReadiness(t *testing.T) {
	for _, creator := range []string{"helper", "production"} {
		t.Run(creator, func(t *testing.T) {
			s := openTestStore(t)
			corruptAccountIdentity(t, s, "account_identity_meta_delete", `DELETE FROM account_identity_meta`)
			before := identitySnapshot(t, s)
			var acc Account
			var dek []byte
			var err error
			if creator == "helper" {
				acc, err = s.CreateAccount("alice", "Alice", rulePassword, false)
			} else {
				acc, dek, err = s.CreateAccountWithKey("", "alice", "Alice", rulePassword, false, nil)
			}
			defer clear(dek)
			if err == nil || !strings.Contains(err.Error(), "identity-capable version") || acc != (Account{}) || len(dek) != 0 {
				t.Fatalf("creation without readiness: account=%v DEK length=%d err=%v", acc, len(dek), err)
			}
			assertIdentitySnapshot(t, s, before)
			assertNoAccountFolder(t, s, "alice")
		})
	}
}

func TestAccountIdentityConcurrentFullOpenMigratesLegacyStore(t *testing.T) {
	s := newStatusStore(t)
	// Keep the complete preceding schema and real credentials. Populated child
	// row preservation is covered separately by the identity-phase fixture.
	// Remove only this slice's additive objects to model its legacy boundary.
	for _, row := range identityQuery(t, s, `SELECT name FROM sqlite_master WHERE type='trigger' AND name GLOB 'account_identity_*'`) {
		identityExec(t, s, `DROP TRIGGER "`+row[0].(string)+`"`)
	}
	identityExec(t, s, `DROP TABLE account_identity_registry; DROP TABLE account_identity_meta`)
	before := identitySnapshot(t, s)
	start := make(chan struct{})
	type result struct {
		store *Store
		err   error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			<-start
			opened, err := Open(s.RootDir())
			results <- result{opened, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var ids []string
	for r := range results {
		if r.store != nil {
			t.Cleanup(func() { _ = r.store.Close() })
		}
		if r.err != nil {
			t.Errorf("concurrent Open: %v", r.err)
			continue
		}
		id, generation := readAccountIdentity(t, r.store, "bob")
		if generation != 1 {
			t.Errorf("initial generation=%d, want 1", generation)
		}
		ids = append(ids, id)
	}
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatalf("concurrent opens did not converge: %v", ids)
	}
	after := identitySnapshot(t, s)
	for table, rows := range before {
		if table != "schema" && !reflect.DeepEqual(after[table], rows) {
			t.Errorf("full Open changed legacy %s: got=%v want=%v", table, after[table], rows)
		}
	}
}

func TestAccountIdentityAllocationRequiresAtomicUserInsert(t *testing.T) {
	s := openTestStore(t)
	before := identitySnapshot(t, s)
	err := s.inWriteTx("abandoned identity allocation", func(ctx context.Context, q accountWriter) error {
		return allocateAccountIdentity(ctx, q, "alice")
	})
	if err == nil {
		t.Fatal("allocation without a user committed despite deferred foreign key")
	}
	assertIdentitySnapshot(t, s, before)
	err = s.inWriteTx("identity and user", func(ctx context.Context, q accountWriter) error {
		if err := allocateAccountIdentity(ctx, q, "alice"); err != nil {
			return err
		}
		_, err := q.ExecContext(ctx, `INSERT INTO users(username,display_name,password_hash,data_dir,created_at) VALUES('alice','Alice','hash','/alice','now')`)
		return err
	})
	if err != nil {
		t.Fatalf("deferred allocation plus user: %v", err)
	}
	readAccountIdentity(t, s, "alice")
}

func TestAccountIdentityMalformedTokenIsRefused(t *testing.T) {
	s := openTestStore(t)
	before := identitySnapshot(t, s)
	canonical := "ac1:" + strings.Repeat("a", 64)
	for _, token := range []string{
		canonical + "\x00unexpected", "ac1:" + strings.Repeat("A", 64),
		"ac1:" + strings.Repeat("a", 63), "ac1:" + strings.Repeat("a", 65),
	} {
		t.Run(fmt.Sprintf("%q", token), func(t *testing.T) {
			// Inspect the INSERT inside a transaction so a deferred FK failure
			// cannot conceal a token guard that incorrectly accepts this input.
			sentinel := errors.New("test rollback")
			err := s.inWriteTx("malformed identity", func(ctx context.Context, q accountWriter) error {
				_, err := q.ExecContext(ctx, `INSERT INTO account_identity_registry(id,username,generation) VALUES(?,'alice',1)`, token)
				if err == nil {
					t.Error("malformed identity INSERT accepted")
					return sentinel
				}
				return err
			})
			var constraint sqlite3.Error
			if !errors.As(err, &constraint) || constraint.Code != sqlite3.ErrConstraint {
				t.Fatalf("token refusal = %v, want constraint error", err)
			}
			assertIdentitySnapshot(t, s, before)
		})
	}
}

func TestAccountIdentityCreationEntropyAndCollisionLeaveNoEffects(t *testing.T) {
	for _, mode := range []string{"first_entropy", "entropy", "active_collision", "retired_collision"} {
		t.Run(mode, func(t *testing.T) {
			s := openTestStore(t)
			caller := ""
			var collisionID string
			if mode != "first_entropy" {
				if _, err := s.CreateAccount("alice", "Alice", rulePassword, true); err != nil {
					t.Fatal(err)
				}
				caller = "alice"
				collisionID, _ = readAccountIdentity(t, s, "alice")
			}
			if mode == "retired_collision" {
				if _, err := s.CreateAccount("former", "Former", rulePassword, false); err != nil {
					t.Fatal(err)
				}
				collisionID, _ = readAccountIdentity(t, s, "former")
				if err := s.PurgeAccount("alice", "former"); err != nil {
					t.Fatal(err)
				}
				assertRetiredAccountIdentity(t, s, collisionID)
			}
			var reader io.Reader = failingRecoveryReader{}
			var fixed *bytes.Reader
			if strings.Contains(mode, "collision") {
				data, err := hex.DecodeString(strings.TrimPrefix(collisionID, "ac1:"))
				if err != nil {
					t.Fatal(err)
				}
				fixed = bytes.NewReader(data)
				reader = fixed
			}
			before := identitySnapshot(t, s)
			previous := afterAccountRuleCheck
			var restore func()
			calls := 0
			afterAccountRuleCheck = func() {
				calls++
				restore = replaceRecoveryRandReader(t, reader)
			}
			t.Cleanup(func() {
				afterAccountRuleCheck = previous
				if restore != nil {
					restore()
				}
			})
			acc, dek, err := s.CreateAccountWithKey(caller, "target", "Target", rulePassword, false, nil)
			if restore != nil {
				restore()
			}
			afterAccountRuleCheck = previous
			defer clear(dek)
			if calls != 1 || err == nil || acc != (Account{}) || len(dek) != 0 {
				t.Fatalf("creation refusal: callback=%d account=%v DEK length=%d err=%v", calls, acc, len(dek), err)
			}
			if !strings.Contains(err.Error(), "account identity") {
				t.Fatalf("refusal missed identity allocation: %v", err)
			}
			if fixed != nil {
				var constraint sqlite3.Error
				if fixed.Len() != 0 || !errors.As(err, &constraint) || constraint.Code != sqlite3.ErrConstraint {
					t.Fatalf("collision retried or wrong refusal: bytes left=%d err=%v", fixed.Len(), err)
				}
			}
			assertIdentitySnapshot(t, s, before)
			assertNoAccountFolder(t, s, "target")
		})
	}
}

func TestAccountIdentityCreatorLaterFailureRollsBackAllocation(t *testing.T) {
	for _, creator := range []string{"helper", "production"} {
		t.Run(creator, func(t *testing.T) {
			s := openTestStore(t)
			identityExec(t, s, `CREATE TRIGGER reject_identity_test_user BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT,'identity test rejects user'); END`)
			before := identitySnapshot(t, s)
			var err error
			if creator == "helper" {
				_, err = s.CreateAccount("alice", "Alice", rulePassword, false)
			} else {
				var dek []byte
				_, dek, err = s.CreateAccountWithKey("", "alice", "Alice", rulePassword, false, nil)
				defer clear(dek)
			}
			if err == nil || !strings.Contains(err.Error(), "identity test rejects user") {
				t.Fatalf("later user insert refusal = %v", err)
			}
			assertIdentitySnapshot(t, s, before)
			assertNoAccountFolder(t, s, "alice")
		})
	}
}

func TestAccountIdentityGenerationTracksOnlyChangedProtectedFields(t *testing.T) {
	s := legacyAccountIdentityStore(t, "alice")
	if err := s.migrateAccountIdentity(); err != nil {
		t.Fatal(err)
	}
	id, generation := readAccountIdentity(t, s, "alice")
	for _, tc := range []struct {
		name, assignment string
		delta            int64
	}{
		{"cosmetics", `display_name='New display',data_dir='/moved',last_login='later',last_export_directory='/new/export',created_at='cosmetic'`, 0},
		{"all_noop", `disabled=disabled,is_admin=is_admin,password_hash=password_hash,wrapped_dek_pw=wrapped_dek_pw`, 0},
		{"disable", `disabled=1`, 1},
		{"disable_noop", `disabled=1`, 0},
		{"enable_ABA", `disabled=0`, 1},
		{"promote", `is_admin=1`, 1},
		{"demote", `is_admin=0`, 1},
		{"password", `password_hash='different password hash'`, 1},
		{"password_wrap", `wrapped_dek_pw='different password wrap'`, 1},
		{"multiple_protected_fields_once", `disabled=1,is_admin=1,password_hash='third hash',wrapped_dek_pw='third wrap'`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			identityExec(t, s, `UPDATE users SET `+tc.assignment+` WHERE username='alice'`)
			generation += tc.delta
			gotID, gotGeneration := readAccountIdentity(t, s, "alice")
			if gotID != id || gotGeneration != generation {
				t.Fatalf("identity=(%s,%d), want (%s,%d)", gotID, gotGeneration, id, generation)
			}
		})
	}
}

func TestAccountIdentityGenerationExhaustionAbortsWholeStatement(t *testing.T) {
	for _, conflict := range []string{"", "OR IGNORE", "OR FAIL", "OR REPLACE"} {
		for _, target := range []string{"single_user", "all_users", "all_registry"} {
			t.Run(conflict+"/"+target, func(t *testing.T) {
				s := legacyAccountIdentityStore(t, "a_first", "z_last")
				if err := s.migrateAccountIdentity(); err != nil {
					t.Fatal(err)
				}
				// Test-only corruption seeds the boundary without billions of
				// protected updates. Restore the exact guard before the action.
				corruptAccountIdentity(t, s, "account_identity_registry_update", `UPDATE account_identity_registry SET generation=? WHERE username='z_last'`, int64(math.MaxInt64))
				// A genuine earlier-row audit effect must also roll back under
				// FAIL/IGNORE. This exposes partial statement retention.
				identityExec(t, s, `CREATE TRIGGER identity_test_update_event AFTER UPDATE ON users BEGIN INSERT INTO account_events(occurred_at,actor,username,action,detail) VALUES('now','test',NEW.username,'disabled','test'); END`)
				before := identitySnapshot(t, s)
				query := `UPDATE ` + conflict + ` users SET disabled=1-disabled`
				if target == "single_user" {
					query += ` WHERE username='z_last'`
				}
				if target == "all_registry" {
					query = `UPDATE ` + conflict + ` account_identity_registry SET generation=generation+1`
				}
				if _, err := s.conn.Exec(query); err == nil {
					t.Fatal("generation exhaustion did not abort")
				}
				assertIdentitySnapshot(t, s, before)
				// An exhausted incarnation can still be permanently retired.
				id, generation := readAccountIdentity(t, s, "z_last")
				if generation != math.MaxInt64 {
					t.Fatal("boundary seed lost")
				}
				identityExec(t, s, `DELETE FROM users WHERE username='z_last'`)
				assertRetiredAccountIdentity(t, s, id)
			})
		}
	}
}

func TestAccountIdentityExhaustedStatusChangePreservesAudit(t *testing.T) {
	s := newStatusStore(t)
	corruptAccountIdentity(t, s, "account_identity_registry_update", `UPDATE account_identity_registry SET generation=? WHERE username='bob'`, int64(math.MaxInt64))
	before := identitySnapshot(t, s)
	if err := s.SetDisabled("alice", "bob", true); err == nil {
		t.Fatal("status change at generation limit succeeded")
	}
	assertIdentitySnapshot(t, s, before)
}

func TestAccountIdentityCreationRollbackCoversAuditAndCommit(t *testing.T) {
	for _, mode := range []string{"helper_commit", "production_commit", "production_audit"} {
		t.Run(mode, func(t *testing.T) {
			s := openTestStore(t)
			if mode == "production_audit" {
				identityExec(t, s, `CREATE TRIGGER identity_test_reject_audit BEFORE INSERT ON account_events BEGIN SELECT RAISE(ABORT,'identity audit refusal'); END`)
			} else {
				identityExec(t, s, `CREATE TABLE identity_test_parent(id TEXT PRIMARY KEY);
CREATE TABLE identity_test_child(ref TEXT REFERENCES identity_test_parent(id) DEFERRABLE INITIALLY DEFERRED);
CREATE TRIGGER identity_test_reject_commit AFTER INSERT ON users BEGIN INSERT INTO identity_test_child(ref) VALUES('missing'); END`)
			}
			before := identitySnapshot(t, s)
			var acc Account
			var dek []byte
			var err error
			if mode == "helper_commit" {
				acc, err = s.CreateAccount("alice", "Alice", rulePassword, false)
			} else {
				acc, dek, err = s.CreateAccountWithKey("", "alice", "Alice", rulePassword, false, nil)
			}
			defer clear(dek)
			want := "commit account creation"
			if mode == "production_audit" {
				want = "identity audit refusal"
			}
			if err == nil || !strings.Contains(err.Error(), want) || acc != (Account{}) || len(dek) != 0 {
				t.Fatalf("creation failure: account=%v, DEK length=%d, err=%v; want %s", acc, len(dek), err, want)
			}
			assertIdentitySnapshot(t, s, before)
			assertNoAccountFolder(t, s, "alice")
		})
	}
}
