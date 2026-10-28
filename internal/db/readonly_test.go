// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"

	"gopmgr/internal/crypto"
	"gopmgr/internal/sqlitedriver"
)

// Tests for read-only project access (Slice B, docs/testing/read-only-project-access.md).
// Every non-mutation test uses a file the writable path provably changes:
// an older schema (Migrate adds columns and backfills money) or a DELETE-mode
// file (the writable DSN switches it to WAL).

func roTestKey(seed byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = seed + byte(i)
	}
	return k
}

func roRawDSN(t *testing.T, path string, dek []byte, extra string) string {
	t.Helper()
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		t.Fatal(err)
	}
	return path + "?_pragma_key=x'" + hexKey + "'" + extra
}

// writeOlderProject writes an encrypted project with a pre-Launchpad schema:
// the project table without the columns Migrate later added (currency,
// minor-unit money, industry and calendar columns), charts and documents
// without the *_unixnano columns, and no audit_events table. It holds one
// project with a nonzero float budget that Migrate would backfill into
// budget_minor_units, two charts and one document of its own, and one chart
// of another project id that counts must not include. The file is left in
// DELETE journal mode.
func writeOlderProject(t *testing.T, path string, dek []byte) {
	t.Helper()
	conn, err := sql.Open(sqlitedriver.Name, roRawDSN(t, path, dek, "&_journal_mode=DELETE"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	conn.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`CREATE TABLE project (id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'planning', phase TEXT NOT NULL DEFAULT 'initiation',
			start_date TEXT NOT NULL DEFAULT '', end_date TEXT NOT NULL DEFAULT '',
			budget NUMERIC NOT NULL DEFAULT 0, owner TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE charts (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL, title TEXT NOT NULL,
			data TEXT NOT NULL DEFAULT '{}', config TEXT NOT NULL DEFAULT '{}', template_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE documents (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL, title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '', template_id TEXT NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'draft', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO project (id, name, status, phase, start_date, end_date, budget)
			VALUES ('prj_old', 'Old Harbour', 'active', 'execution', '2025-01-06', '2025-12-19', 12.34)`,
		`INSERT INTO charts (id, project_id, kind, title) VALUES ('c1', 'prj_old', 'gantt', 'Plan'), ('c2', 'prj_old', 'wbs', 'Scope'), ('c3', 'prj_other', 'gantt', 'Stray')`,
		`INSERT INTO documents (id, project_id, kind, title) VALUES ('d1', 'prj_old', 'charter', 'Charter')`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("older fixture: %v\n%s", err, stmt)
		}
	}
}

// writeCurrentProject creates a project through the writable path, adds a
// chart, and leaves the file closed with no -wal or -shm. With deleteMode it
// is then switched to DELETE journal mode.
func writeCurrentProject(t *testing.T, path string, dek []byte, deleteMode bool) string {
	t.Helper()
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.UpsertProject(Project{Name: "Current", Status: "planning", Phase: "initiation"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveChart(Chart{ProjectID: p.ID, Kind: "gantt", Title: "Plan", Data: "{}", Config: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if deleteMode {
		conn, err := sql.Open(sqlitedriver.Name, roRawDSN(t, path, dek, ""))
		if err != nil {
			t.Fatal(err)
		}
		var mode string
		if err := conn.QueryRow("PRAGMA journal_mode = DELETE").Scan(&mode); err != nil || mode != "delete" {
			t.Fatalf("switch to DELETE: %q %v", mode, err)
		}
		_ = conn.Close()
	}
	for _, side := range []string{"-wal", "-shm"} {
		_ = os.Remove(path + side)
	}
	return p.ID
}

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

// journalMode reads a file's journal mode through a read-only connection.
func journalMode(t *testing.T, path string, dek []byte) string {
	t.Helper()
	d, _, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	var mode string
	if err := d.Conn.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	return mode
}

func TestOpenReadOnlyLeavesOlderProjectUnchanged(t *testing.T) { // B-02, B-11, B-12
	dek := roTestKey(1)
	path := filepath.Join(t.TempDir(), "old.gopmgr")
	writeOlderProject(t, path, dek)
	before := fileDigest(t, path)

	d, state, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatalf("OpenEncryptedReadOnly: %v", err)
	}
	if state != SchemaOlder {
		t.Errorf("state = %v, want SchemaOlder", state)
	}
	ov, err := d.ReadProjectOverview()
	if err != nil {
		t.Fatalf("ReadProjectOverview: %v", err)
	}
	want := ProjectOverview{ID: "prj_old", Name: "Old Harbour", Status: "active", Phase: "execution",
		StartDate: "2025-01-06", EndDate: "2025-12-19", Charts: 2, Documents: 1}
	if ov != want {
		t.Errorf("overview = %+v, want %+v", ov, want)
	}
	chain, err := d.VerifyAuditChainIfPresent(ov.ID)
	if err != nil || !chain.Valid || chain.CheckedEvents != 0 || chain.TerminalEventHash != "" {
		t.Errorf("audit chain without audit_events = %+v, %v; want an empty valid chain", chain, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if fileDigest(t, path) != before {
		t.Error("reading an older project changed its main file")
	}
	if mode := journalMode(t, path, dek); mode != "delete" {
		t.Errorf("journal mode = %q, want delete", mode)
	}
	for _, side := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(path + side); err == nil {
			t.Errorf("reading a DELETE-mode file created %s", side)
		}
	}
	// The writable path would have upgraded it: proves this fixture can
	// detect a regression back to InitEncryptedDB.
	w, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	p, err := w.GetProject()
	_ = w.Close()
	if err != nil || p.BudgetMinorUnits != 1234 {
		t.Fatalf("writable open: project %+v %v, want the budget backfilled to 1234", p, err)
	}
	if fileDigest(t, path) == before {
		t.Fatal("the writable open did not change the fixture, so the test above proves nothing")
	}
}

func TestOpenReadOnlyCurrentProject(t *testing.T) { // B-01
	dek := roTestKey(2)
	path := filepath.Join(t.TempDir(), "current.gopmgr")
	id := writeCurrentProject(t, path, dek, true)
	before := fileDigest(t, path)

	d, state, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	if state != SchemaCurrent {
		t.Errorf("state = %v, want SchemaCurrent", state)
	}
	p, err := d.GetProject()
	if err != nil || p.ID != id {
		t.Errorf("GetProject = %+v, %v", p, err)
	}
	if _, err := d.VerifyAuditChain(id); err != nil {
		t.Errorf("VerifyAuditChain: %v", err)
	}
	_ = d.Close()
	if fileDigest(t, path) != before {
		t.Error("reading a current project changed its main file")
	}
	if mode := journalMode(t, path, dek); mode != "delete" {
		t.Errorf("journal mode = %q, want delete", mode)
	}
}

func TestOpenReadOnlyTypedFailures(t *testing.T) { // B-06, B-07, B-08, B-09, B-16
	dir := t.TempDir()
	dek := roTestKey(3)
	current := filepath.Join(dir, "current.gopmgr")
	writeCurrentProject(t, current, dek, true)

	future := filepath.Join(dir, "future.gopmgr")
	writeCurrentProject(t, future, dek, true)
	conn, err := sql.Open(sqlitedriver.Name, roRawDSN(t, future, dek, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	damaged := filepath.Join(dir, "damaged.gopmgr")
	writeCurrentProject(t, damaged, dek, true)
	raw, err := os.ReadFile(damaged)
	if err != nil {
		t.Fatal(err)
	}
	for i := 4096; i < len(raw) && i < 4096*3; i++ {
		raw[i] ^= 0xA5 // pages 2 and 3: page 1 still decrypts
	}
	if err := os.WriteFile(damaged, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "link.gopmgr")
	if err := os.Symlink(current, link); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.gopmgr")

	cases := []struct {
		name string
		path string
		key  []byte
		want error
	}{
		{"missing file", missing, dek, ErrProjectNotFound},
		{"wrong key", current, roTestKey(9), ErrProjectUnreadable},
		{"future format stamp", future, dek, ErrUnsupportedProjectFormat},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var before [32]byte
			if c.path != missing {
				before = fileDigest(t, c.path)
			}
			d, _, err := OpenEncryptedReadOnly(c.path, c.key)
			if d != nil {
				_ = d.Close()
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if c.path == missing {
				if _, statErr := os.Stat(missing); statErr == nil {
					t.Fatal("opening a missing project created it")
				}
				return
			}
			if fileDigest(t, c.path) != before {
				t.Error("a failed read-only open changed the file")
			}
		})
	}

	t.Run("symlink", func(t *testing.T) {
		d, _, err := OpenEncryptedReadOnly(link, dek)
		if d != nil {
			_ = d.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("err = %v, want a refusal of the symlink", err)
		}
	})

	t.Run("damaged pages", func(t *testing.T) {
		before := fileDigest(t, damaged)
		_, err := OpenEncryptedCopyReadOnly(damaged, dek)
		if !errors.Is(err, ErrCopyDamaged) {
			t.Fatalf("err = %v, want ErrCopyDamaged", err)
		}
		if fileDigest(t, damaged) != before {
			t.Error("checking a damaged file changed it")
		}
	})
}

func TestOpenReadOnlyRefusesEveryWrite(t *testing.T) { // B-18
	dek := roTestKey(4)
	path := filepath.Join(t.TempDir(), "p.gopmgr")
	writeCurrentProject(t, path, dek, true)
	before := fileDigest(t, path)
	d, _, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := d.Conn.Conn(t.Context())
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = conn.Close() }()
			// Turning query_only off must not make the connection writable:
			// mode=ro is the guard.
			if _, err := conn.ExecContext(t.Context(), "PRAGMA query_only = 0"); err != nil {
				errs <- err
				return
			}
			_, err = conn.ExecContext(t.Context(), "UPDATE project SET name = 'changed'")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err == nil || !strings.Contains(err.Error(), "readonly") {
			t.Errorf("write through a pooled read-only connection: err = %v, want a read-only refusal", err)
		}
	}
	if _, err := d.UpsertProject(Project{Name: "x"}); err == nil {
		t.Error("UpsertProject through a read-only Database succeeded")
	}
	if fileDigest(t, path) != before {
		t.Error("refused writes still changed the file")
	}

	// Migrate writes nothing to a current file, so try it where it must
	// write: an older file, which it would upgrade.
	older := filepath.Join(t.TempDir(), "old.gopmgr")
	writeOlderProject(t, older, dek)
	olderBefore := fileDigest(t, older)
	o, _, err := OpenEncryptedReadOnly(older, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = o.Close() }()
	if err := o.Migrate(); err == nil || !strings.Contains(err.Error(), "readonly") {
		t.Errorf("Migrate of an older file through a read-only Database: err = %v, want a read-only refusal", err)
	}
	if fileDigest(t, older) != olderBefore {
		t.Error("a refused Migrate changed the older file")
	}
}

func TestOpenReadOnlyReadsCommittedWALAndKeepsSidecarsPrivate(t *testing.T) { // B-13, B-14
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	dek := roTestKey(5)
	path := filepath.Join(t.TempDir(), "p.gopmgr")
	writeCurrentProject(t, path, dek, false)

	// A writer commits rows that stay in the -wal: autocheckpoint is off.
	w, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	wc, err := w.Conn.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wc.Close() }()
	if _, err := wc.ExecContext(t.Context(), "PRAGMA wal_autocheckpoint = 0"); err != nil {
		t.Fatal(err)
	}
	p, err := w.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := w.SaveChart(Chart{ProjectID: p.ID, Kind: "gantt", Title: "WAL only", Data: "{}", Config: "{}"}); err != nil {
			t.Fatal(err)
		}
	}
	mainBefore := fileDigest(t, path)

	d, _, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	ov, err := d.ReadProjectOverview()
	_ = d.Close()
	if err != nil || ov.Charts != 4 {
		t.Fatalf("overview = %+v, %v; want 4 charts, 3 of them only in the WAL", ov, err)
	}
	if fileDigest(t, path) != mainBefore {
		t.Error("the read-only reader changed the main file")
	}
	mainInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"-wal", "-shm"} {
		info, err := os.Stat(path + side)
		if err != nil {
			t.Fatalf("%s: %v", side, err)
		}
		if info.Mode().Perm() != mainInfo.Mode().Perm() || info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode = %v, want the main file's private %v", side, info.Mode().Perm(), mainInfo.Mode().Perm())
		}
	}
}

func TestOpenReadOnlyAlongsideAWriter(t *testing.T) { // B-14
	dek := roTestKey(6)
	path := filepath.Join(t.TempDir(), "p.gopmgr")
	writeCurrentProject(t, path, dek, false)
	w, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	p, err := w.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 40; i++ {
			if _, err := w.SaveChart(Chart{ProjectID: p.ID, Kind: "gantt", Title: "w", Data: "{}", Config: "{}"}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	last := 0
	for i := 0; i < 40; i++ {
		d, _, err := OpenEncryptedReadOnly(path, dek)
		if errors.Is(err, ErrProjectBusy) {
			continue
		}
		if err != nil {
			t.Fatalf("read-only open during writes: %v", err)
		}
		ov, err := d.ReadProjectOverview()
		_ = d.Close()
		if err != nil && !errors.Is(err, ErrProjectBusy) {
			t.Fatalf("read during writes: %v", err)
		}
		if err == nil {
			if ov.Charts < last {
				t.Fatalf("chart count went backwards: %d after %d", ov.Charts, last)
			}
			last = ov.Charts
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("writer: %v", err)
	}
	if ok, err := w.CheckIntegrity(); err != nil || !ok {
		t.Fatalf("integrity after concurrent reads: %v %v", ok, err)
	}
}

func TestOpenReadOnlyClosesItsFiles(t *testing.T) { // B-17
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("counts open descriptors through /dev/fd")
	}
	dek := roTestKey(7)
	dir := t.TempDir()
	path := filepath.Join(dir, "p.gopmgr")
	writeCurrentProject(t, path, dek, false)
	countFDs := func() int {
		entries, err := os.ReadDir("/dev/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	// Warm up the driver and the cached current schema first.
	if d, _, err := OpenEncryptedReadOnly(path, dek); err == nil {
		_ = d.Close()
	}
	start := countFDs()
	for i := 0; i < 100; i++ {
		d, _, err := OpenEncryptedReadOnly(path, dek)
		if err == nil {
			_, _ = d.ReadProjectOverview()
			_ = d.Close()
		}
		if d2, _, err := OpenEncryptedReadOnly(path, roTestKey(8)); err == nil {
			_ = d2.Close() // a wrong key fails inside the open
		}
	}
	if end := countFDs(); end > start+2 {
		t.Errorf("open descriptors grew from %d to %d over 200 opens", start, end)
	}
}

func TestReadOnlyDSN(t *testing.T) {
	cases := []struct {
		path, want, wantErr string
	}{
		{"/Users/a b/x%41/p.gopmgr", "file:/Users/a b/x%2541/p.gopmgr?mode=ro&_query_only=1&_busy_timeout=5000", ""},
		{"/tmp/p?.gopmgr", "", "illegal character"},
		{"/tmp/p#.gopmgr", "", "illegal character"},
		{"//server/share/p.gopmgr", "", "network paths"},
	}
	for _, c := range cases {
		got, err := readOnlyDSN(c.path)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("readOnlyDSN(%q) err = %v, want %q", c.path, err, c.wantErr)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("readOnlyDSN(%q) = %q, %v; want %q", c.path, got, err, c.want)
		}
	}
}

// A '%' in a folder name must not be decoded: "x%41" and "xA" are different
// folders and each must open its own project.
func TestOpenReadOnlyPercentInPath(t *testing.T) {
	dek := roTestKey(10)
	root := t.TempDir()
	escaped := filepath.Join(root, "x%41", "p.gopmgr")
	decoded := filepath.Join(root, "xA", "p.gopmgr")
	for _, p := range []string{escaped, decoded} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeCurrentProject(t, escaped, dek, true)
	writeOlderProject(t, decoded, dek)
	d, state, err := OpenEncryptedReadOnly(escaped, dek)
	if err != nil {
		t.Fatal(err)
	}
	ov, err := d.ReadProjectOverview()
	_ = d.Close()
	if err != nil || ov.Name != "Current" || state != SchemaCurrent {
		t.Fatalf("x%%41 opened %+v (%v, %v); want its own current project", ov, state, err)
	}
}

// The current schema is what a fresh writable open produces; a file the
// writable path just created must classify as current.
func TestCurrentSchemaMatchesAFreshProject(t *testing.T) {
	dek := roTestKey(11)
	path := filepath.Join(t.TempDir(), "fresh.gopmgr")
	writeCurrentProject(t, path, dek, false)
	d, state, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if state != SchemaCurrent {
		t.Fatal("a freshly created project does not classify as current")
	}
	have, err := d.tableColumns()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := currentSchema()
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != len(have) {
		t.Errorf("current schema has %d tables, a fresh project %d", len(expected), len(have))
	}
	if _, ok := expected["project"]["currency_code"]; !ok {
		t.Error("current schema is missing project.currency_code")
	}
}

func TestOpenEncryptedCopyReadOnlyRefusesOlderSchema(t *testing.T) { // B-04
	dek := roTestKey(12)
	path := filepath.Join(t.TempDir(), "copy.gopmgr")
	writeOlderProject(t, path, dek)
	before := fileDigest(t, path)
	if _, err := OpenEncryptedCopyReadOnly(path, dek); !errors.Is(err, ErrProjectNeedsUpgrade) {
		t.Fatalf("err = %v, want ErrProjectNeedsUpgrade", err)
	}
	if fileDigest(t, path) != before {
		t.Error("opening an older administrator copy changed it")
	}
}

// Each schema check must stand on its own: a file missing only one table, or
// only one column, is Older. (The older fixture lacks both, which would hide
// either check being removed.)
func TestSchemaClassificationDetectsEachGap(t *testing.T) {
	cases := []struct {
		name  string
		alter []string
	}{
		{"one table missing", []string{`DROP TABLE cost_reserves`}},
		{"one column missing", []string{
			`ALTER TABLE stakeholders RENAME TO stakeholders_full`,
			`CREATE TABLE stakeholders AS SELECT id, project_id, name FROM stakeholders_full WHERE 0`,
			`DROP TABLE stakeholders_full`,
		}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dek := roTestKey(byte(20 + i))
			path := filepath.Join(t.TempDir(), "p.gopmgr")
			writeCurrentProject(t, path, dek, true)
			conn, err := sql.Open(sqlitedriver.Name, roRawDSN(t, path, dek, ""))
			if err != nil {
				t.Fatal(err)
			}
			conn.SetMaxOpenConns(1)
			for _, stmt := range c.alter {
				if _, err := conn.Exec(stmt); err != nil {
					t.Fatalf("%s: %v", stmt, err)
				}
			}
			_ = conn.Close()
			d, state, err := OpenEncryptedReadOnly(path, dek)
			if err != nil {
				t.Fatal(err)
			}
			_ = d.Close()
			if state != SchemaOlder {
				t.Errorf("state = %v, want SchemaOlder", state)
			}
		})
	}
}

// A file that exists but cannot be opened is not "missing". Observed on
// macOS: a mode-000 file fails with SQLITE_CANTOPEN; a WAL-mode file with no
// -shm, in a folder the reader cannot write, fails with SQLITE_READONLY
// because SQLite cannot create the -shm. Both are ErrProjectInaccessible, and
// neither changes the file.
func TestOpenReadOnlyInaccessibleFiles(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced for a non-root user")
	}
	dek := roTestKey(30)
	dir := filepath.Join(t.TempDir(), "readonly-folder")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	wal := filepath.Join(dir, "wal.gopmgr")
	writeCurrentProject(t, wal, dek, false)
	walBefore := fileDigest(t, wal)
	noRead := filepath.Join(t.TempDir(), "no-read.gopmgr")
	writeCurrentProject(t, noRead, dek, true)
	noReadBefore := fileDigest(t, noRead)
	if err := os.Chmod(noRead, 0o000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
		_ = os.Chmod(noRead, 0o600)
	})
	for _, path := range []string{wal, noRead} {
		d, _, err := OpenEncryptedReadOnly(path, dek)
		if d != nil {
			_ = d.Close()
		}
		if !errors.Is(err, ErrProjectInaccessible) || errors.Is(err, ErrProjectNotFound) {
			t.Errorf("%s: err = %v, want ErrProjectInaccessible", filepath.Base(path), err)
		}
	}
	_ = os.Chmod(dir, 0o700)
	_ = os.Chmod(noRead, 0o600)
	if fileDigest(t, wal) != walBefore || fileDigest(t, noRead) != noReadBefore {
		t.Error("a refused open changed a file")
	}
}

// Cancelling a read mid-iteration (B-15) leaves nothing changed, and the file
// opens cleanly afterwards.
func TestOpenReadOnlyCancelledRead(t *testing.T) {
	dek := roTestKey(31)
	path := filepath.Join(t.TempDir(), "old.gopmgr")
	writeOlderProject(t, path, dek)
	before := fileDigest(t, path)
	d, _, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	rows, err := d.Conn.QueryContext(ctx, `SELECT c.id FROM charts c, charts d, charts e`)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("no first row")
	}
	cancel()
	for rows.Next() {
	}
	_ = rows.Close()
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if fileDigest(t, path) != before {
		t.Error("a cancelled read changed the file")
	}
	again, state, err := OpenEncryptedReadOnly(path, dek)
	if err != nil || state != SchemaOlder {
		t.Fatalf("reopen after a cancelled read: %v %v", state, err)
	}
	_ = again.Close()
}

// The SQLCipher key travels inside the read-only URI; it must not appear in
// a typed error or in what SQLite reports as the database file name.
func TestOpenReadOnlyKeepsTheKeyOutOfErrorsAndNames(t *testing.T) {
	dek := roTestKey(32)
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "p.gopmgr")
	writeCurrentProject(t, path, dek, true)
	d, _, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := d.Conn.Query("PRAGMA database_list")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(file, hexKey) {
			t.Errorf("database_list file %q contains the key", file)
		}
	}
	_ = rows.Close()
	_ = d.Close()
	for _, p := range []string{path, filepath.Join(dir, "missing.gopmgr")} {
		other := roTestKey(33)
		if _, _, err := OpenEncryptedReadOnly(p, other); err != nil && strings.Contains(err.Error(), hexKey) {
			t.Errorf("error %q contains a key", err)
		}
		if _, _, err := OpenEncryptedReadOnly(p, dek); err != nil && strings.Contains(err.Error(), hexKey) {
			t.Errorf("error %q contains the key", err)
		}
	}
}

// corruptTableRoot flips bytes inside the root page of table, past the page
// header, so the file still opens and the damage is found only when that
// table is read.
func corruptTableRoot(t *testing.T, path string, dek []byte, table string) {
	t.Helper()
	d, _, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	var root, pageSize int
	err = d.Conn.QueryRow("SELECT rootpage FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&root)
	if err == nil {
		err = d.Conn.QueryRow("PRAGMA page_size").Scan(&pageSize)
	}
	_ = d.Close()
	if err != nil || root < 2 {
		t.Fatalf("root page of %s: %d %v", table, root, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	off := (root - 1) * pageSize
	for i := off + 100; i < off+200; i++ {
		raw[i] ^= 0xA5
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Damage the open does not reach is found by the read that does, and is
// still reported as damage, not as an untyped error.
func TestReadProjectOverviewReportsDamage(t *testing.T) { // B-09
	dek := roTestKey(33)
	path := filepath.Join(t.TempDir(), "p.gopmgr")
	writeCurrentProject(t, path, dek, true)
	corruptTableRoot(t, path, dek, "project")
	before := fileDigest(t, path)

	d, state, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatalf("open: %v (the damage is past the schema, so the open should succeed)", err)
	}
	defer func() { _ = d.Close() }()
	if state != SchemaCurrent {
		t.Errorf("state = %v, want SchemaCurrent", state)
	}
	if _, err := d.ReadProjectOverview(); !errors.Is(err, ErrProjectDamaged) {
		t.Fatalf("ReadProjectOverview err = %v, want ErrProjectDamaged", err)
	}
	if _, err := d.VerifyAuditChainIfPresent("prj_x"); err != nil && !errors.Is(err, ErrProjectDamaged) {
		t.Errorf("VerifyAuditChainIfPresent err = %v, want nil or ErrProjectDamaged", err)
	}
	if fileDigest(t, path) != before {
		t.Error("reading a damaged project changed it")
	}
}

func TestClassifySQLiteError(t *testing.T) {
	plain := errors.New("not from SQLite")
	cases := []struct {
		code sqlite3.ErrNo
		want error
	}{
		{sqlite3.ErrNotADB, ErrProjectUnreadable},
		{sqlite3.ErrCorrupt, ErrProjectDamaged},
		{sqlite3.ErrBusy, ErrProjectBusy},
		{sqlite3.ErrLocked, ErrProjectBusy},
		{sqlite3.ErrCantOpen, ErrProjectInaccessible},
		{sqlite3.ErrReadonly, ErrProjectInaccessible},
	}
	for _, ext := range []sqlite3.ErrNoExtended{sqlite3.ErrReadonlyRollback, sqlite3.ErrReadonlyRecovery} {
		got := classifySQLiteError(sqlite3.Error{Code: sqlite3.ErrReadonly, ExtendedCode: ext})
		if !errors.Is(got, ErrProjectNeedsRecovery) || errors.Is(got, ErrProjectInaccessible) {
			t.Errorf("extended code %d: got %v, want ErrProjectNeedsRecovery only", ext, got)
		}
	}
	for _, c := range cases {
		src := sqlite3.Error{Code: c.code}
		got := classifySQLiteError(src)
		if !errors.Is(got, c.want) {
			t.Errorf("code %d: got %v, want %v", c.code, got, c.want)
		}
		var keep sqlite3.Error
		if !errors.As(got, &keep) || keep.Code != c.code {
			t.Errorf("code %d: the SQLite error is not kept wrapped: %v", c.code, got)
		}
	}
	if got := classifySQLiteError(plain); got != plain {
		t.Errorf("an unrelated error changed: %v", got)
	}
	if got := classifySQLiteError(sqlite3.Error{Code: sqlite3.ErrConstraint}); errors.Is(got, ErrProjectDamaged) || errors.Is(got, ErrProjectBusy) {
		t.Errorf("a constraint error was given a typed open error: %v", got)
	}
}

// A writer that dies mid-transaction leaves a hot rollback journal. A
// read-only open cannot roll it back; it must say so, not blame permissions,
// and leave both files as they were.
func TestOpenReadOnlyHotJournal(t *testing.T) {
	dek := roTestKey(40)
	dir := t.TempDir()
	path := filepath.Join(dir, "p.gopmgr")
	writeCurrentProject(t, path, dek, true)
	conn, err := sql.Open(sqlitedriver.Name, roRawDSN(t, path, dek, ""))
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	tx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"PRAGMA cache_size = 1",
		"CREATE TABLE junk(x)",
		"INSERT INTO junk SELECT randomblob(2000) FROM (WITH RECURSIVE c(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM c WHERE i < 2000) SELECT i FROM c)",
	} {
		if _, err := tx.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// Copy the file and its journal mid-transaction: the copy is what a
	// crash leaves behind.
	crash := filepath.Join(dir, "crash.gopmgr")
	for _, side := range []string{"", "-journal"} {
		b, err := os.ReadFile(path + side)
		if err != nil {
			t.Fatalf("read %q: %v", side, err)
		}
		if err := os.WriteFile(crash+side, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_ = tx.Rollback()
	before, journal := fileDigest(t, crash), fileDigest(t, crash+"-journal")

	d, _, err := OpenEncryptedReadOnly(crash, dek)
	if d != nil {
		_ = d.Close()
	}
	if !errors.Is(err, ErrProjectNeedsRecovery) || errors.Is(err, ErrProjectInaccessible) {
		t.Fatalf("err = %v, want ErrProjectNeedsRecovery", err)
	}
	if fileDigest(t, crash) != before || fileDigest(t, crash+"-journal") != journal {
		t.Error("the read-only open changed the file or its journal")
	}
}

// A failure while building the reference schema must not be kept: the next
// open retries and succeeds.
func TestCurrentSchemaRetriesAfterAFailure(t *testing.T) {
	saved := buildCurrentSchema
	current.mu.Lock()
	cached := current.schema
	current.schema = nil
	current.mu.Unlock()
	t.Cleanup(func() {
		buildCurrentSchema = saved
		current.mu.Lock()
		current.schema = cached
		current.mu.Unlock()
	})

	path := filepath.Join(t.TempDir(), "p.gopmgr")
	dek := roTestKey(41)
	writeCurrentProject(t, path, dek, true)

	boom := errors.New("transient")
	buildCurrentSchema = func() (map[string]map[string]struct{}, error) { return nil, boom }
	if _, _, err := OpenEncryptedReadOnly(path, dek); !errors.Is(err, boom) {
		t.Fatalf("first open: err = %v, want the injected failure", err)
	}
	buildCurrentSchema = saved
	d, state, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatalf("second open after the failure cleared: %v", err)
	}
	_ = d.Close()
	if state != SchemaCurrent {
		t.Errorf("state = %v, want SchemaCurrent", state)
	}
}

// InspectEncryptedReadOnly skips the schema comparison but not the key or
// format checks.
func TestInspectEncryptedReadOnly(t *testing.T) {
	dir := t.TempDir()
	dek := roTestKey(42)
	older := filepath.Join(dir, "older.gopmgr")
	writeOlderProject(t, older, dek)
	before := fileDigest(t, older)
	d, err := InspectEncryptedReadOnly(older, dek)
	if err != nil {
		t.Fatalf("older project: %v", err)
	}
	if _, err := d.ReadProjectOverview(); err != nil {
		t.Errorf("overview of an older project: %v", err)
	}
	_ = d.Close()
	if fileDigest(t, older) != before {
		t.Error("inspecting an older project changed it")
	}
	if _, err := InspectEncryptedReadOnly(older, roTestKey(43)); !errors.Is(err, ErrProjectUnreadable) {
		t.Errorf("wrong key: err = %v, want ErrProjectUnreadable", err)
	}
	future := filepath.Join(dir, "future.gopmgr")
	writeCurrentProject(t, future, dek, true)
	conn, err := sql.Open(sqlitedriver.Name, roRawDSN(t, future, dek, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("PRAGMA user_version = 3"); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if _, err := InspectEncryptedReadOnly(future, dek); !errors.Is(err, ErrUnsupportedProjectFormat) {
		t.Errorf("future stamp: err = %v, want ErrUnsupportedProjectFormat", err)
	}
}
