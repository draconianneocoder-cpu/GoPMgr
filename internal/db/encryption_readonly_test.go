// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopmgr/internal/crypto"
)

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close %s: %v", dst, err)
	}
}

// A copy opened for reading cannot be written on any pooled connection.
func TestEncryptedCopyOpensReadOnlyOnEveryConnection(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "project.gopmgr")
	dek := testDEK(t, 0x5a)
	d, err := InitEncryptedDB(original, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if _, err := d.Conn.Exec(`CREATE TABLE probe (x INTEGER); INSERT INTO probe VALUES (1);`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	copyPath := filepath.Join(dir, "copy.gopmgr")
	copyTestFile(t, original, copyPath)

	ro, err := OpenEncryptedCopyReadOnly(copyPath, dek)
	if err != nil {
		t.Fatalf("OpenEncryptedCopyReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	var n int
	if err := ro.Conn.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("read the copy: %d rows, %v; want 1", n, err)
	}

	ctx := context.Background()
	first, err := ro.Conn.Conn(ctx)
	if err != nil {
		t.Fatalf("first connection: %v", err)
	}
	defer func() { _ = first.Close() }()
	second, err := ro.Conn.Conn(ctx)
	if err != nil {
		t.Fatalf("second connection: %v", err)
	}
	defer func() { _ = second.Close() }()
	if _, err := first.ExecContext(ctx, `INSERT INTO probe VALUES (2)`); err == nil {
		t.Error("the first pooled connection wrote to the copy")
	}
	if _, err := second.ExecContext(ctx, `UPDATE probe SET x = 3`); err == nil {
		t.Error("the second pooled connection wrote to the copy")
	}
	if _, err := ro.Conn.Exec(`CREATE TABLE other (y)`); err == nil {
		t.Error("the copy's schema changed")
	}
}

func TestDamagedEncryptedCopyIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "copy.gopmgr")
	dek := testDEK(t, 0x5b)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteAt(make([]byte, 64), 5000); err != nil {
		t.Fatalf("damage: %v", err)
	}
	_ = f.Close()

	if ro, err := OpenEncryptedCopyReadOnly(path, dek); err == nil {
		_ = ro.Close()
		t.Fatal("a damaged copy was opened")
	}
}

func TestReadOnlyFileURIEncodesThePath(t *testing.T) {
	got, err := readOnlyFileURI("/tmp/a b/p.gopmgr", "ABCD")
	if err != nil {
		t.Fatal(err)
	}
	want := "file:///tmp/a%20b/p.gopmgr?mode=ro&_query_only=1&_foreign_keys=on&_busy_timeout=5000&_pragma_key=x'ABCD'"
	if got != want {
		t.Fatalf("uri = %s", got)
	}
	win, err := readOnlyFileURI("C:/a b/p.gopmgr", "ABCD")
	if err != nil {
		t.Fatal(err)
	}
	wantWin := "file:///C:/a%20b/p.gopmgr?mode=ro&_query_only=1&_foreign_keys=on&_busy_timeout=5000&_pragma_key=x'ABCD'"
	if win != wantWin {
		t.Fatalf("windows uri = %s", win)
	}
	if _, err := readOnlyFileURI("/tmp/a?b.gopmgr", "ABCD"); err == nil {
		t.Fatal("a question mark in the path was accepted")
	}
}

func TestOpenEncryptedReadOnlyDoesNotCreateAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.gopmgr")
	if _, err := OpenEncryptedReadOnly(path, testDEK(t, 0x11)); !os.IsNotExist(err) {
		t.Fatalf("err = %v, want not exist", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a missing path was created")
	}
}

func TestOpenEncryptedReadOnlyRejectsADirectory(t *testing.T) {
	if _, err := OpenEncryptedReadOnly(t.TempDir(), testDEK(t, 0x12)); err == nil {
		t.Fatal("a directory was opened")
	}
}

func TestOpenEncryptedReadOnlyOpensMode0444AndRefusesWrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my projects")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "project.gopmgr")
	dek := testDEK(t, 0x44)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`CREATE TABLE probe (x INTEGER); INSERT INTO probe VALUES (7);`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpsertProject(Project{Name: "Readable", Owner: "ada"}); err != nil {
		t.Fatal(err)
	}
	freezeDeleteJournal(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	removeSQLiteSidecarsForTest(t, path)

	writable := stampDBFile(t, path)
	ro, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := ro.Conn.QueryRow(`SELECT x FROM probe`).Scan(&n); err != nil || n != 7 {
		t.Fatalf("probe = %d, %v", n, err)
	}
	if _, err := ro.Conn.Exec(`INSERT INTO probe VALUES (8)`); err == nil {
		t.Fatal("a read-only connection inserted a row")
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writable, stampDBFile(t, path)) {
		t.Fatal("the writable-mode file changed during a read-only open")
	}

	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	ro, err = OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatalf("mode 0444 open: %v", err)
	}
	project, err := ro.GetProject()
	if err != nil || project.Name != "Readable" {
		t.Fatalf("project = %+v, %v", project, err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := InitEncryptedDB(path, dek); err == nil {
		t.Fatal("InitEncryptedDB wrote a mode 0444 file")
	}
}

func TestOpenEncryptedReadOnlyRejectsTheWrongKeyWithoutChangingTheFile(t *testing.T) {
	path, _ := sealedProjectMissingColumn(t, "currency_code")
	before := stampDBFile(t, path)
	if _, err := OpenEncryptedReadOnly(path, testDEK(t, 0x99)); err == nil {
		t.Fatal("wrong key opened the file")
	}
	if _, err := OpenEncryptedReadOnly(path, []byte("short")); !errors.Is(err, crypto.ErrBadDEK) {
		t.Fatalf("short key err = %v", err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, path)) {
		t.Fatal("a rejected open changed the file")
	}
}

func TestGetProjectReadsAMissingCurrencyColumnWithoutAddingIt(t *testing.T) {
	path, dek := sealedProjectMissingColumn(t, "currency_code")
	before := stampDBFile(t, path)
	ro, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	project, err := ro.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	if project.Name != "Legacy" || project.CurrencyCode != "USD" {
		t.Fatalf("project = %+v", project)
	}
	var mode string
	if err := ro.Conn.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "delete" {
		t.Fatalf("journal = %s", mode)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, path)) {
		t.Fatalf("stamp changed: before perm %v wal %v, after perm %v wal %v", before.Perm, before.Wal, stampDBFile(t, path).Perm, stampDBFile(t, path).Wal)
	}
	if columnPresent(t, path, dek, "project", "currency_code") {
		t.Fatal("currency_code was added")
	}
}

func TestWritableOpenStillAddsAMissingCurrencyColumn(t *testing.T) {
	path, dek := sealedProjectMissingColumn(t, "currency_code")
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if !columnPresentOn(t, d, "project", "currency_code") {
		t.Fatal("writable open left currency_code missing")
	}
}

func TestEncryptedCopyReadOnlyDoesNotAddAMissingColumn(t *testing.T) {
	path, dek := sealedProjectMissingColumn(t, "currency_code")
	before := stampDBFile(t, path)
	ro, err := OpenEncryptedCopyReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	project, err := ro.GetProject()
	if err != nil || project.Name != "Legacy" {
		t.Fatalf("project = %+v, %v", project, err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, path)) {
		t.Fatal("the administrator copy open changed the file")
	}
	if columnPresent(t, path, dek, "project", "currency_code") {
		t.Fatal("the copy open added currency_code")
	}
}

func TestReadAuditChainLeavesAMissingTableMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.gopmgr")
	dek := testDEK(t, 0x72)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpsertProject(Project{Name: "No Audit", Owner: "ada"}); err != nil {
		t.Fatal(err)
	}
	project, err := d.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`DROP TABLE audit_events`); err != nil {
		t.Fatal(err)
	}
	freezeDeleteJournal(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	removeSQLiteSidecarsForTest(t, path)
	before := stampDBFile(t, path)

	ro, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	report, err := ro.ReadAuditChain(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || report.CheckedEvents != 0 || report.ProjectID != project.ID {
		t.Fatalf("report = %+v", report)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, path)) {
		t.Fatal("reading a missing audit table changed the file")
	}
	if tablePresent(t, path, dek, "audit_events") {
		t.Fatal("audit_events was created")
	}
}

func TestReadProbesSkipMissingTablesAndRejectAnUnknownCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.gopmgr")
	dek := testDEK(t, 0x73)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	project, err := d.UpsertProject(Project{Name: "Probe", Owner: "ada"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`DROP TABLE charts`); err != nil {
		t.Fatal(err)
	}
	charts, err := d.CountProjectRecords("charts", project.ID)
	if err != nil || charts != 0 {
		t.Fatalf("charts = %d, %v", charts, err)
	}
	documents, err := d.CountProjectRecords("documents", project.ID)
	if err != nil || documents != 0 {
		t.Fatalf("documents = %d, %v", documents, err)
	}
	if _, err := d.CountProjectRecords("settings", project.ID); err == nil {
		t.Fatal("an unknown count table was accepted")
	}
	ready, err := d.HasTableColumns("project", "id", "name")
	if err != nil || !ready {
		t.Fatalf("project columns ready = %v, %v", ready, err)
	}
	ready, err = d.HasTableColumns("project", "id", "not_a_real_column")
	if err != nil || ready {
		t.Fatalf("missing column ready = %v, %v", ready, err)
	}
	ready, err = d.HasTableColumns("charts", "id")
	if err != nil || ready {
		t.Fatalf("dropped charts table ready = %v, %v", ready, err)
	}
	if _, err := d.Conn.Exec(`DROP TABLE project`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.GetProject(); !errors.Is(err, ErrNoProject) {
		t.Fatalf("GetProject after drop = %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenEncryptedReadOnlyThroughASymlinkDoesNotChangeTheTarget(t *testing.T) {
	path, dek := sealedProjectMissingColumn(t, "currency_code")
	link := filepath.Join(filepath.Dir(path), "link.gopmgr")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	before := stampDBFile(t, path)
	ro, err := OpenEncryptedReadOnly(link, dek)
	if err != nil {
		t.Fatal(err)
	}
	project, err := ro.GetProject()
	if err != nil || project.Name != "Legacy" {
		t.Fatalf("project = %+v, %v", project, err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, path)) {
		t.Fatal("opening through a symlink changed the target")
	}
}

type dbFileStamp struct {
	Data []byte
	Perm os.FileMode
	Wal  bool
	Shm  bool
}

func stampDBFile(t *testing.T, path string) dbFileStamp {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	_, walErr := os.Lstat(path + "-wal")
	_, shmErr := os.Lstat(path + "-shm")
	return dbFileStamp{
		Data: data,
		Perm: info.Mode().Perm(),
		Wal:  walErr == nil,
		Shm:  shmErr == nil,
	}
}

func sealedProjectMissingColumn(t *testing.T, column string) (string, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "project.gopmgr")
	dek := testDEK(t, 0x71)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpsertProject(Project{Name: "Legacy", Owner: "ada", Budget: 12.5}); err != nil {
		t.Fatal(err)
	}
	omitSQLiteColumnDB(t, d, "project", column)
	freezeDeleteJournal(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	removeSQLiteSidecarsForTest(t, path)
	return path, dek
}

func freezeDeleteJournal(t *testing.T, d *Database) {
	t.Helper()
	if _, err := d.Conn.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := d.Conn.QueryRow(`PRAGMA journal_mode = DELETE`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "delete" {
		t.Fatalf("journal_mode = %s", mode)
	}
}

func removeSQLiteSidecarsForTest(t *testing.T, path string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func omitSQLiteColumnDB(t *testing.T, d *Database, table, drop string) {
	t.Helper()
	sqlIdent(t, table)
	sqlIdent(t, drop)
	if _, err := d.Conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Conn.Exec(`PRAGMA foreign_keys = ON`) }()
	var info string
	switch table {
	case "project":
		info = `PRAGMA table_info(project)`
	case "stakeholders":
		info = `PRAGMA table_info(stakeholders)`
	case "charts":
		info = `PRAGMA table_info(charts)`
	case "documents":
		info = `PRAGMA table_info(documents)`
	default:
		t.Fatalf("unsupported table %s", table)
	}
	rows, err := d.Conn.Query(info)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if name == drop {
			continue
		}
		names = append(names, sqlIdent(t, name))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no columns left")
	}
	keep := table + "_keep"
	create := `CREATE TABLE ` + sqlIdent(t, keep) + ` AS SELECT ` + joinIdents(names) + ` FROM ` + sqlIdent(t, table)
	if _, err := d.Conn.Exec(create); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`DROP TABLE ` + sqlIdent(t, table)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`ALTER TABLE ` + sqlIdent(t, keep) + ` RENAME TO ` + sqlIdent(t, table)); err != nil {
		t.Fatal(err)
	}
}

func joinIdents(names []string) string {
	out := names[0]
	for _, name := range names[1:] {
		out += ", " + name
	}
	return out
}

func sqlIdent(t *testing.T, name string) string {
	t.Helper()
	if name == "" {
		t.Fatal("empty identifier")
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			t.Fatalf("refusing identifier %q", name)
		}
	}
	return `"` + name + `"`
}

func columnPresent(t *testing.T, path string, dek []byte, table, column string) bool {
	t.Helper()
	d, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	return columnPresentOn(t, d, table, column)
}

func columnPresentOn(t *testing.T, d *Database, table, column string) bool {
	t.Helper()
	var n int
	err := d.Conn.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`,
		table, column,
	).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func tablePresent(t *testing.T, path string, dek []byte, table string) bool {
	t.Helper()
	d, err := OpenEncryptedReadOnly(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	var n int
	if err := d.Conn.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		table,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}
