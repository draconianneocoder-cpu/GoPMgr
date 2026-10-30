// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGetProjectRejectsMissingRequiredColumns(t *testing.T) {
	for _, column := range []string{"id", "name", "description", "status", "phase", "start_date", "end_date", "budget", "owner", "created_at", "updated_at"} {
		for _, empty := range []bool{false, true} {
			name := column + "/populated"
			if empty {
				name = column + "/empty"
			}
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "malformed.gopmgr")
				key := testDEK(t, 0x6c)
				d, err := InitEncryptedDB(path, key)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = d.Close() })
				if !empty {
					if _, err := d.UpsertProject(Project{Name: "Malformed", BudgetMinorUnits: 12345}); err != nil {
						t.Fatal(err)
					}
				}
				omitSQLiteColumnDB(t, d, "project", column)
				freezeDeleteJournal(t, d)
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
				before := stampDBFile(t, path)
				ro, err := OpenEncryptedReadOnly(path, key)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = ro.Close() })
				if _, err := ro.GetProject(); err == nil || errors.Is(err, ErrNoProject) || !strings.Contains(err.Error(), column) {
					t.Fatalf("missing %s: got %v, want contextual schema error", column, err)
				}
				if err := ro.Close(); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, stampDBFile(t, path)) {
					t.Fatal("rejected project read changed source")
				}
			})
		}
	}
}

func TestGetProjectDefaultsOnlyMissingAdditiveColumns(t *testing.T) {
	optional := []string{"industry", "sub_category", "methodology", "country_code", "time_zone", "budget_minor_units", "currency_code"}
	cases := [][]string{nil, optional}
	for _, column := range optional {
		cases = append(cases, []string{column})
	}
	for _, missing := range cases {
		name := strings.Join(missing, "+")
		if name == "" {
			name = "all-present"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy.gopmgr")
			key := testDEK(t, 0x6b)
			d, err := InitEncryptedDB(path, key)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Close() })
			want, err := d.UpsertProject(Project{Name: "Legacy", Owner: "ada", BudgetMinorUnits: 12345, Industry: "health", SubCategory: "care", Methodology: "agile", CountryCode: "GB", TimeZone: "Europe/London", CurrencyCode: "EUR"})
			if err != nil {
				t.Fatal(err)
			}
			for _, column := range missing {
				omitSQLiteColumnDB(t, d, "project", column)
				switch column {
				case "industry":
					want.Industry = ""
				case "sub_category":
					want.SubCategory = ""
				case "methodology":
					want.Methodology = ""
				case "country_code":
					want.CountryCode = "US"
				case "time_zone":
					want.TimeZone = "America/New_York"
				case "currency_code":
					want.CurrencyCode = "USD"
				}
			}
			freezeDeleteJournal(t, d)
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			before := stampDBFile(t, path)
			ro, err := OpenEncryptedReadOnly(path, key)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ro.Close() })
			got, err := ro.GetProject()
			if err != nil || !reflect.DeepEqual(got, want) || got.BudgetMinorUnits != 12345 || got.Budget != 123.45 {
				t.Fatalf("project = %+v, %v; want %+v", got, err, want)
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, stampDBFile(t, path)) {
				t.Fatal("legacy project read changed source")
			}
		})
	}
}

func TestReadOnlyClosedWALAllowsCoordinationOnly(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, perm := range []os.FileMode{0o600, 0o444} {
			name := "plaintext"
			if encrypted {
				name = "encrypted"
			}
			if perm == 0o444 {
				name += "/read-only-main"
			} else {
				name += "/writable-main"
			}
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "wal.gopmgr")
				key := testDEK(t, 0x69)
				var d *Database
				var err error
				if encrypted {
					d, err = InitEncryptedDB(path, key)
				} else {
					d, err = InitDB(path)
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = d.Close() })
				if _, err := d.UpsertProject(Project{Name: "WAL"}); err != nil {
					t.Fatal(err)
				}
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, perm); err != nil {
					t.Fatal(err)
				}
				before := stampDBFile(t, path)
				if before.Wal || before.Shm {
					t.Fatal("fixture must have a closed, checkpointed WAL database")
				}
				var ro *Database
				if encrypted {
					ro, err = OpenEncryptedReadOnly(path, key)
				} else {
					ro, err = OpenReadOnly(path)
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = ro.Close() })
				p, err := ro.GetProject()
				if err != nil || p.Name != "WAL" {
					t.Fatalf("project = %+v, %v", p, err)
				}
				var mode string
				if err := ro.Conn.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
					t.Fatalf("journal mode = %q, %v", mode, err)
				}
				during := stampDBFile(t, path)
				if !during.Wal || !during.Shm {
					t.Fatal("fixture did not exercise SQLite coordination creation")
				}
				assertUnchanged := func(phase string) {
					t.Helper()
					after := stampDBFile(t, path)
					if !bytes.Equal(before.Data, after.Data) || before.Perm != after.Perm {
						t.Fatalf("%s changed main bytes or permissions", phase)
					}
					wal, err := os.ReadFile(path + "-wal")
					if err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					// SQLite's WAL_HDRSIZE is 32; even one database frame
					// exceeds it. A read may create coordination, never frames.
					if len(wal) != 0 && len(wal) != 32 {
						t.Fatalf("%s wrote WAL data: %d bytes", phase, len(wal))
					}
					assertOnlyProjectCoordinationFiles(t, path)
				}
				assertUnchanged("read")
				if err := ro.Close(); err != nil {
					t.Fatal(err)
				}
				assertUnchanged("reader close")
			})
		}
	}
}

func assertOnlyProjectCoordinationFiles(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(path)
	for _, entry := range entries {
		if entry.Name() != base && entry.Name() != base+"-wal" && entry.Name() != base+"-shm" {
			t.Errorf("read created unexpected sibling %s", entry.Name())
		}
	}
}

func TestLegacyListsOrderInstantsWithoutBackfilling(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Legacy lists"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ name, stamp string }{
		{"offset", "2026-01-05T01:00:00+02:00"},
		{"whole", "2026-01-05T00:00:00Z"},
		{"fraction", "2026-01-05T00:00:00.5Z"},
		{"nano", "2026-01-05T00:00:00.500000001Z"},
		{"malformed", "invalid"},
	} {
		c, err := d.SaveChart(Chart{ProjectID: p.ID, Kind: "cpm", Title: row.name})
		if err != nil {
			t.Fatal(err)
		}
		doc, err := d.SaveDocument(Document{ProjectID: p.ID, Kind: "charter", Title: row.name})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Conn.Exec(`UPDATE charts SET updated_at=? WHERE id=?`, row.stamp, c.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Conn.Exec(`UPDATE documents SET updated_at=? WHERE id=?`, row.stamp, doc.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"charts", "documents"} {
		omitSQLiteColumnDB(t, d, table, "updated_at_unixnano")
	}
	freezeDeleteJournal(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	before := stampDBFile(t, d.Path)
	ro, err := OpenReadOnly(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	charts, err := ro.ListCharts(p.ID, "cpm")
	if err != nil {
		t.Fatal(err)
	}
	docs, err := ro.ListDocuments(p.ID, "charter")
	if err != nil {
		t.Fatal(err)
	}
	var chartNames, docNames []string
	for _, c := range charts {
		chartNames = append(chartNames, c.Title)
	}
	for _, doc := range docs {
		docNames = append(docNames, doc.Title)
	}
	want := []string{"nano", "fraction", "whole", "offset", "malformed"}
	if !reflect.DeepEqual(chartNames, want) || !reflect.DeepEqual(docNames, want) {
		t.Fatalf("charts/docs = %v/%v, want %v", chartNames, docNames, want)
	}
	if rows, err := ro.ListCharts("other-project", ""); err != nil || len(rows) != 0 {
		t.Fatalf("project filter = %v, %v", rows, err)
	}
	if rows, err := ro.ListDocuments(p.ID, "other-kind"); err != nil || len(rows) != 0 {
		t.Fatalf("kind filter = %v, %v", rows, err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, d.Path)) {
		t.Fatal("list ordering changed source")
	}
}

func TestReadOnlySeesLiveWALAndKeepsWriterUsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.gopmgr")
	key := testDEK(t, 0x6a)
	w, err := InitEncryptedDB(path, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	w.Conn.SetMaxOpenConns(1)
	if _, err := w.Conn.Exec(`PRAGMA wal_autocheckpoint = 0`); err != nil {
		t.Fatal(err)
	}
	var autoCheckpoint int
	if err := w.Conn.QueryRow(`PRAGMA wal_autocheckpoint`).Scan(&autoCheckpoint); err != nil || autoCheckpoint != 0 {
		t.Fatalf("automatic checkpoint = %d, %v", autoCheckpoint, err)
	}
	if _, err := w.UpsertProject(Project{Name: "First"}); err != nil {
		t.Fatal(err)
	}
	var busy, logged, checkpointed int
	if err := w.Conn.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logged, &checkpointed); err != nil || busy != 0 || logged != 0 || checkpointed != 0 {
		t.Fatalf("fixture checkpoint = %d/%d/%d, %v", busy, logged, checkpointed, err)
	}
	mainBeforeCommit := stampDBFile(t, path)
	if _, err := w.Conn.Exec(`UPDATE project SET name = 'Committed in WAL'`); err != nil {
		t.Fatal(err)
	}
	if p, err := w.GetProject(); err != nil || p.Name != "Committed in WAL" {
		t.Fatalf("writer's committed row = %+v, %v", p, err)
	}
	before := stampDBFile(t, path)
	if !bytes.Equal(mainBeforeCommit.Data, before.Data) || mainBeforeCommit.Perm != before.Perm {
		t.Fatal("fixture checkpointed target update before opening reader")
	}
	walBefore, err := os.ReadFile(path + "-wal")
	if err != nil || len(walBefore) <= 32 {
		t.Fatalf("fixture lacks committed WAL frames: %d bytes, %v", len(walBefore), err)
	}
	assertUnchanged := func(phase string) {
		t.Helper()
		after := stampDBFile(t, path)
		walAfter, err := os.ReadFile(path + "-wal")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before.Data, after.Data) || before.Perm != after.Perm || !bytes.Equal(walBefore, walAfter) {
			t.Fatalf("%s changed main/WAL bytes or permissions", phase)
		}
		assertOnlyProjectCoordinationFiles(t, path)
	}
	ro, err := OpenEncryptedReadOnly(path, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	p, err := ro.GetProject()
	if err != nil || p.Name != "Committed in WAL" {
		t.Fatalf("project = %+v, err %v", p, err)
	}
	assertUnchanged("first read")
	if _, err := w.Conn.Exec(`UPDATE project SET name = 'Writer still works'`); err != nil {
		t.Fatal(err)
	}
	before = stampDBFile(t, path)
	walBefore, err = os.ReadFile(path + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	if p, err := ro.GetProject(); err != nil || p.Name != "Writer still works" {
		t.Fatalf("second read = %+v, %v", p, err)
	}
	assertUnchanged("second read")
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	assertUnchanged("reader close")
}

func TestReadOnlyModeProtectsEveryConnectionWithoutQueryOnly(t *testing.T) {
	path, key := sealedProjectMissingColumn(t, "currency_code")
	before := stampDBFile(t, path)
	ro, err := OpenEncryptedReadOnly(path, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	ctx := context.Background()
	first, err := ro.Conn.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := ro.Conn.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	for i, conn := range []*sql.Conn{first, second} {
		if _, err := conn.ExecContext(ctx, `PRAGMA query_only = OFF`); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, `UPDATE project SET name = 'Changed'`); err == nil {
			t.Errorf("connection %d bypassed mode=ro", i)
		}
	}
	if err := ro.Migrate(); err == nil {
		t.Error("read-only Migrate succeeded")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampDBFile(t, path)) {
		t.Fatal("failed writes changed file")
	}
}
