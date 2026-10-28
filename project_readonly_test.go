// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/cli"
	"gopmgr/internal/crypto"
	"gopmgr/internal/db"
	"gopmgr/internal/sqlitedriver"
)

// App-level tests for read-only project access (Slice B,
// docs/testing/read-only-project-access.md). Each one reads through the real
// App method and uses files that the old open-and-migrate path changes: an
// older schema, which Migrate upgrades, and a DELETE-mode file, which the
// writable connection switches to WAL.

// readOnlyFixture is a signed-in App with one project file of each kind.
type readOnlyFixture struct {
	app                                 *App
	dek                                 []byte
	projectsDir                         string
	current, older, future, otherKey    string
	olderName, futureName, otherKeyName string
}

func keyDSN(t *testing.T, path string, dek []byte) string {
	t.Helper()
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		t.Fatal(err)
	}
	return path + "?_pragma_key=x'" + hexKey + "'"
}

// writeOlderProjectFile writes a pre-Launchpad project (no currency,
// minor-unit or *_unixnano columns, no audit_events) in DELETE journal mode,
// with two charts and one document of its own.
func writeOlderProjectFile(t *testing.T, dsn string) {
	t.Helper()
	conn, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	conn.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`PRAGMA journal_mode = DELETE`,
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
		`INSERT INTO project (id, name, status, phase, budget) VALUES ('prj_old', 'Old Harbour', 'active', 'execution', 12.34)`,
		`INSERT INTO charts (id, project_id, kind, title) VALUES ('c1', 'prj_old', 'gantt', 'Plan'), ('c2', 'prj_old', 'wbs', 'Scope')`,
		`INSERT INTO documents (id, project_id, kind, title) VALUES ('d1', 'prj_old', 'charter', 'Charter')`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("older fixture: %v\n%s", err, stmt)
		}
	}
}

// toDeleteMode switches a closed project file to DELETE journal mode and
// removes its sidecars.
func toDeleteMode(t *testing.T, path string, dek []byte, stamp int) {
	t.Helper()
	conn, err := sql.Open(sqlitedriver.Name, keyDSN(t, path, dek))
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	var mode string
	if err := conn.QueryRow("PRAGMA journal_mode = DELETE").Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("journal_mode: %q %v", mode, err)
	}
	if stamp != 0 {
		if _, err := conn.Exec("PRAGMA user_version = 2"); err != nil {
			t.Fatal(err)
		}
	}
	_ = conn.Close()
	for _, side := range []string{"-wal", "-shm"} {
		_ = os.Remove(path + side)
	}
}

func newReadOnlyFixture(t *testing.T) *readOnlyFixture {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatal(err)
	}
	acceptNoRecoveryCodes(t, app)
	s := app.currentSession()
	dek, err := app.sessionDEK(s)
	if err != nil {
		t.Fatal(err)
	}
	f := &readOnlyFixture{app: app, dek: dek, projectsDir: filepath.Join(s.user.DataDir, "projects")}

	current, err := app.CreateProject("Current", "")
	if err != nil {
		t.Fatal(err)
	}
	future, err := app.CreateProject("Future", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatal(err)
	}
	f.current, f.future = current.Path, future.Path
	toDeleteMode(t, f.current, dek, 0)
	toDeleteMode(t, f.future, dek, 2)
	f.futureName = projectDisplayName(filepath.Base(filepath.Dir(f.future)))

	if f.older, err = reserveProjectFolder(f.projectsDir, "20250101-000000-old-harbour"); err != nil {
		t.Fatal(err)
	}
	writeOlderProjectFile(t, keyDSN(t, f.older, dek))
	f.olderName = "old-harbour"

	if f.otherKey, err = reserveProjectFolder(f.projectsDir, "20250101-000001-other-key"); err != nil {
		t.Fatal(err)
	}
	other := make([]byte, 32)
	for i := range other {
		other[i] = 0x5A
	}
	writeOlderProjectFile(t, keyDSN(t, f.otherKey, other))
	f.otherKeyName = "other-key"
	return f
}

func (f *readOnlyFixture) digests(t *testing.T) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	for _, p := range []string{f.current, f.older, f.future, f.otherKey} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[p] = sha256.Sum256(b)
	}
	return out
}

func (f *readOnlyFixture) requireUnchanged(t *testing.T, before map[string][32]byte) {
	t.Helper()
	after := f.digests(t)
	for p, d := range before {
		if after[p] != d {
			t.Errorf("%s changed on a read-only path", filepath.Base(filepath.Dir(p)))
		}
	}
	d, _, err := db.OpenEncryptedReadOnly(f.current, f.dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	var mode string
	if err := d.Conn.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "delete" {
		t.Errorf("current project journal mode = %q (%v), want delete", mode, err)
	}
}

func TestProjectsOverviewNeverWritesProjects(t *testing.T) { // B-03, B-11, B-12
	f := newReadOnlyFixture(t)
	before := f.digests(t)

	got, err := f.app.ProjectsOverview()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]ProjectSummary{}
	for _, s := range got {
		byPath[s.Path] = s
	}
	if s := byPath[f.older]; !s.Readable || s.Name != "Old Harbour" || s.Status != "active" || s.Charts != 2 || s.Documents != 1 {
		t.Errorf("older project summary = %+v", s)
	}
	if s := byPath[f.current]; !s.Readable || s.Name != "Current" || s.UnreadableReason != "" {
		t.Errorf("current project summary = %+v", s)
	}
	if s := byPath[f.future]; s.Readable || s.UnreadableReason != "unsupported_format" {
		t.Errorf("future-format project summary = %+v, want unreadable with unsupported_format", s)
	}
	if s := byPath[f.otherKey]; s.Readable || s.UnreadableReason != "unreadable" {
		t.Errorf("other-key project summary = %+v, want unreadable with reason unreadable", s)
	}
	f.requireUnchanged(t, before)
}

func TestProjectDeletionRequestNeverWritesProjects(t *testing.T) { // B-05, B-10
	f := newReadOnlyFixture(t)
	before := f.digests(t)
	s := f.app.currentSession()

	req, err := f.app.projectDeletionRequest(s, f.older, f.projectsDir)
	if err != nil {
		t.Fatalf("older project: %v", err)
	}
	if req.ProjectID != "prj_old" || req.ProjectName != "Old Harbour" || !req.AuditValid || req.AuditEvents != 0 {
		t.Errorf("older project request = %+v", req)
	}

	req, err = f.app.projectDeletionRequest(s, f.current, f.projectsDir)
	if err != nil {
		t.Fatalf("current project: %v", err)
	}
	d, _, err := db.OpenEncryptedReadOnly(f.current, f.dek)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetProject()
	chain, err := d.VerifyAuditChain(p.ID)
	_ = d.Close()
	if err != nil || req.ProjectID != p.ID || req.AuditEvents != chain.CheckedEvents || req.AuditTerminalHash != chain.TerminalEventHash || !req.AuditValid {
		t.Errorf("current project request = %+v, want the chain %+v", req, chain)
	}

	// Decided behavior: a project with a newer format stamp is not
	// migrated to read its deletion record, so preparing its deletion fails.
	if _, err := f.app.projectDeletionRequest(s, f.future, f.projectsDir); !errors.Is(err, db.ErrUnsupportedProjectFormat) {
		t.Errorf("future-format project: err = %v, want ErrUnsupportedProjectFormat", err)
	}
	f.requireUnchanged(t, before)
}

func TestAdminProjectCopyNeverMigrates(t *testing.T) { // B-04
	f := newReadOnlyFixture(t)
	before := f.digests(t)
	dst := filepath.Join(t.TempDir(), "copy.gopmgr")

	if _, _, err := openProjectCopy(f.older, dst, f.dek); err == nil || !strings.Contains(err.Error(), "older file format") {
		t.Fatalf("older project copy: err = %v, want the older-format refusal", err)
	}
	if b, _ := os.ReadFile(dst); sha256.Sum256(b) != before[f.older] {
		t.Error("opening the older copy changed the copy")
	}

	dst2 := filepath.Join(t.TempDir(), "copy2.gopmgr")
	view, proj, err := openProjectCopy(f.current, dst2, f.dek)
	if err != nil {
		t.Fatalf("current project copy: %v", err)
	}
	if proj.Name != "Current" {
		t.Errorf("copy project = %+v", proj)
	}
	if _, err := view.UpsertProject(db.Project{Name: "changed"}); err == nil {
		t.Error("the administrator view accepted a write")
	}
	_ = view.Close()
	if b, _ := os.ReadFile(dst2); sha256.Sum256(b) != before[f.current] {
		t.Error("viewing the current copy changed the copy")
	}
	f.requireUnchanged(t, before)
}

func TestHeadlessReadCommandsNeverWriteProjects(t *testing.T) { // B-20 (CLI)
	path := filepath.Join(t.TempDir(), "plain.gopmgr")
	writeOlderProjectFile(t, path)
	digest := func() [32]byte {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(b)
	}
	before := digest()

	for _, cfg := range []*cli.Config{
		{ProjectPath: path, ShowStats: true},
		{ProjectPath: path, ExportAuditPath: filepath.Join(t.TempDir(), "audit.csv")},
	} {
		if _, err := openHeadlessDB(cfg); !errors.Is(err, db.ErrProjectNeedsUpgrade) {
			t.Errorf("%+v: err = %v, want ErrProjectNeedsUpgrade", cfg, err)
		}
	}
	for _, cfg := range []*cli.Config{{ProjectPath: path, CheckOnly: true}, {ProjectPath: path, SchemaDump: true}} {
		d, err := openHeadlessDB(cfg)
		if err != nil {
			t.Fatalf("%+v: %v", cfg, err)
		}
		if ok, err := d.CheckIntegrity(); err != nil || !ok {
			t.Errorf("integrity on a read-only handle: %v %v", ok, err)
		}
		if _, err := d.DumpSchema(); err != nil {
			t.Errorf("schema dump on a read-only handle: %v", err)
		}
		_ = d.Close()
	}
	if digest() != before {
		t.Fatal("a headless read command changed the project")
	}

	// Vacuum is a write: it still opens writable and upgrades the file.
	d, err := openHeadlessDB(&cli.Config{ProjectPath: path, Vacuum: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	if digest() == before {
		t.Error("the writable vacuum open did not upgrade the older file")
	}
}

// portfolioProjectMetrics must work through a read-only handle on a project
// with data: its committed cost, EVM and work-item reads may not need a
// write. The writable result is the comparison; the read-only call must
// leave the file unchanged.
func TestPortfolioProjectMetricsOnAReadOnlyHandle(t *testing.T) { // B-03, B-20
	d, project := newPortfolioEVMTestProject(t, "2026-01-05")
	savePortfolioEVMChart(t, d, project.ID, `{"nodes":[{"id":"task-1","label":"Build","duration":10,
		"percent_complete":50,"budgeted_cost_minor_units":10000,"actual_cost_minor_units":4000}],"edges":[]}`)
	asOf := time.Date(2026, 1, 12, 12, 0, 0, 0, time.UTC)
	want := mustPortfolioProjectMetrics(t, d, project, asOf)
	if _, err := d.Conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	path := d.Path
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(b)

	ro, state, err := db.OpenReadOnly(path)
	if err != nil || state != db.SchemaCurrent {
		t.Fatalf("OpenReadOnly: %v %v", state, err)
	}
	defer func() { _ = ro.Close() }()
	p, err := ro.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	got, err := portfolioProjectMetrics(ro, p, "", asOf)
	if err != nil {
		t.Fatalf("portfolioProjectMetrics on a read-only handle: %v", err)
	}
	if got != want {
		t.Errorf("read-only metrics = %+v, want %+v", got, want)
	}
	if b, _ := os.ReadFile(path); sha256.Sum256(b) != before {
		t.Error("portfolio metrics changed the project file")
	}
}

// The administrator copy in its real form (B-04): the owner's project is in
// WAL mode and held open by a writer whose newest commits are only in the
// -wal. openProjectCopy copies the main file, -wal and -shm as they stand;
// the read-only view must see those commits and leave the copy's main file
// as copied.
func TestAdminProjectCopyOfALiveWALProject(t *testing.T) { // B-04, B-13
	f := newReadOnlyFixture(t)
	owner := filepath.Join(filepath.Dir(f.current), "..", "live")
	if err := os.MkdirAll(owner, 0o700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(owner, "project.gopmgr")
	w, err := db.InitEncryptedDB(src, f.dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	conn, err := w.Conn.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(t.Context(), "PRAGMA wal_autocheckpoint = 0"); err != nil {
		t.Fatal(err)
	}
	p, err := w.UpsertProject(db.Project{Name: "Live", Status: "active", Phase: "execution"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := w.SaveChart(db.Chart{ProjectID: p.ID, Kind: "gantt", Title: "only in the WAL", Data: "{}", Config: "{}"}); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(src + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("the owner's commits are not in the -wal: %v", err)
	}
	srcMain, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy.gopmgr")
	view, proj, err := openProjectCopy(src, dst, f.dek)
	if err != nil {
		t.Fatalf("openProjectCopy: %v", err)
	}
	if proj.Name != "Live" {
		t.Errorf("project = %+v", proj)
	}
	charts, err := view.ListCharts(p.ID, "")
	_ = view.Close()
	if err != nil || len(charts) != 3 {
		t.Fatalf("the view sees %d charts (%v), want the 3 committed only in the -wal", len(charts), err)
	}
	copied, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(copied) != sha256.Sum256(srcMain) {
		t.Error("viewing the copy changed its main file (it was checkpointed or migrated)")
	}
}

// A project whose project-table page is damaged opens, then fails on the
// read; the list must still say "damaged", list the healthy project, and
// leave the damaged file as it was.
func TestProjectsOverviewReportsADamagedProject(t *testing.T) { // B-09
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatal(err)
	}
	acceptNoRecoveryCodes(t, app)
	dek, err := app.sessionDEK(app.currentSession())
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := app.CreateProject("Healthy", "")
	if err != nil {
		t.Fatal(err)
	}
	damaged, err := app.CreateProject("Damaged", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatal(err)
	}
	toDeleteMode(t, healthy.Path, dek, 0)
	toDeleteMode(t, damaged.Path, dek, 0)

	d, _, err := db.OpenEncryptedReadOnly(damaged.Path, dek)
	if err != nil {
		t.Fatal(err)
	}
	var root, pageSize int
	err = d.Conn.QueryRow("SELECT rootpage FROM sqlite_master WHERE type = 'table' AND name = 'project'").Scan(&root)
	if err == nil {
		err = d.Conn.QueryRow("PRAGMA page_size").Scan(&pageSize)
	}
	_ = d.Close()
	if err != nil || root < 2 {
		t.Fatalf("project root page: %d %v", root, err)
	}
	raw, err := os.ReadFile(damaged.Path)
	if err != nil {
		t.Fatal(err)
	}
	for i := (root-1)*pageSize + 100; i < (root-1)*pageSize+200; i++ {
		raw[i] ^= 0xA5
	}
	if err := os.WriteFile(damaged.Path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(raw)

	list, err := app.ProjectsOverview()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ProjectSummary{}
	for _, s := range list {
		got[s.Path] = s
	}
	if s := got[damaged.Path]; s.Readable || s.UnreadableReason != "damaged" {
		t.Errorf("damaged project: readable=%v reason=%q, want unreadable with reason \"damaged\"", s.Readable, s.UnreadableReason)
	}
	if s := got[healthy.Path]; !s.Readable || s.Name != "Healthy" || s.UnreadableReason != "" {
		t.Errorf("healthy project: %+v, want it listed as readable", s)
	}
	after, err := os.ReadFile(damaged.Path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(after) != before {
		t.Error("listing projects changed the damaged file")
	}
}

// unreadable_reason is read by the frontend, so its values are a wire
// contract.
func TestProjectUnreadableReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{db.ErrProjectNotFound, "not_found"},
		{db.ErrProjectUnreadable, "unreadable"},
		{db.ErrProjectDamaged, "damaged"},
		{db.ErrProjectBusy, "busy"},
		{db.ErrProjectInaccessible, "inaccessible"},
		{db.ErrProjectNeedsRecovery, "needs_recovery"},
		{db.ErrUnsupportedProjectFormat, "unsupported_format"},
		{db.ErrNoProject, "no_project"},
		{errors.New("something else"), "error"},
	}
	for _, c := range cases {
		wrapped := fmt.Errorf("open %s: %w", "p.gopmgr", c.err)
		if got := projectUnreadableReason(wrapped); got != c.want {
			t.Errorf("projectUnreadableReason(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// Every headless command, the writing ones included, refuses a symlinked
// project path and leaves the target untouched.
func TestHeadlessCommandsRefuseASymlinkedProject(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "plain.gopmgr")
	writeOlderProjectFile(t, target)
	link := filepath.Join(dir, "link.gopmgr")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(b)
	for _, cfg := range []*cli.Config{
		{ProjectPath: link, CheckOnly: true},
		{ProjectPath: link, SchemaDump: true},
		{ProjectPath: link, ShowStats: true},
		{ProjectPath: link, Repair: true},
		{ProjectPath: link, Vacuum: true},
	} {
		d, err := openHeadlessDB(cfg)
		if d != nil {
			_ = d.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("%+v: err = %v, want a refusal of the symlink", cfg, err)
		}
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(after) != before {
		t.Error("a headless command wrote through the symlink")
	}
}
