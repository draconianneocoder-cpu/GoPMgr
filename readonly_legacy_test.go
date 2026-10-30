// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/agile"
	"gopmgr/internal/cli"
	"gopmgr/internal/db"
)

func TestHeadlessPlaintextReadsPreserveLegacyFile(t *testing.T) {
	for _, command := range []string{"check", "stats", "schema", "audit", "export"} {
		t.Run(command, func(t *testing.T) {
			d := seedHeadlessProject(t)
			omitAppSQLiteColumn(t, d, "project", "currency_code")
			freezeAppDeleteJournal(t, d)
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			removeAppSQLiteSidecars(t, d.Path)
			before := stampAppDBFile(t, d.Path)
			cfg := &cli.Config{ProjectPath: d.Path}
			switch command {
			case "check":
				cfg.CheckOnly = true
			case "stats":
				cfg.ShowStats = true
			case "schema":
				cfg.SchemaDump = true
			case "audit":
				cfg.ExportAuditPath = filepath.Join(t.TempDir(), "audit.csv")
			case "export":
				cfg.ExportPath = filepath.Join(t.TempDir(), "schedule.csv")
			}
			ro, err := openHeadlessDB(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ro.GetProject(); err != nil {
				t.Error(err)
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, stampAppDBFile(t, d.Path)) {
				t.Fatal("headless read changed a legacy project")
			}
		})
	}
}

func TestReadOnlySourceTraversalKeepsProjectAndExportIdentity(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		name := "plaintext"
		if encrypted {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			key := bytes.Repeat([]byte{0x74}, 32)
			target := filepath.Join(t.TempDir(), "project.gopmgr")
			decoy := filepath.Join(t.TempDir(), "project.gopmgr")
			var wanted db.Project
			for _, fixture := range []struct{ path, name string }{{target, "Selected target"}, {decoy, "Wrong decoy"}} {
				var d *db.Database
				var err error
				if encrypted {
					d, err = db.InitEncryptedDB(fixture.path, key)
				} else {
					d, err = db.InitDB(fixture.path)
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = d.Close() })
				p, err := d.UpsertProject(db.Project{Name: fixture.name})
				if err != nil {
					t.Fatal(err)
				}
				if fixture.path == target {
					wanted = p
				}
				freezeAppDeleteJournal(t, d)
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
			}
			child := filepath.Join(filepath.Dir(target), "child")
			if err := os.Mkdir(child, 0o700); err != nil {
				t.Fatal(err)
			}
			jump := filepath.Join(filepath.Dir(decoy), "jump")
			if err := os.Symlink(child, jump); err != nil {
				t.Fatal(err)
			}
			source := jump + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "project.gopmgr"
			canonicalTarget, err := filepath.EvalSymlinks(target)
			if err != nil {
				t.Fatal(err)
			}
			beforeTarget, beforeDecoy := stampAppDBFile(t, target), stampAppDBFile(t, decoy)
			var ro *db.Database
			if encrypted {
				ro, err = db.OpenEncryptedReadOnly(source, key)
			} else {
				ro, err = db.OpenReadOnly(source)
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ro.Close() })
			p, err := ro.GetProject()
			if err != nil || p.ID != wanted.ID || p.Name != wanted.Name || ro.Path != canonicalTarget {
				t.Fatalf("opened %+v at %q, %v; want %+v at %q", p, ro.Path, err, wanted, canonicalTarget)
			}
			for _, kind := range []string{"schedule", "audit"} {
				export := func(path string) error {
					if kind == "audit" {
						return ro.ExportAuditCSV(path)
					}
					return runHeadlessExport(&cli.Config{ExportPath: path, ExportFormat: "csv"}, ro)
				}
				if err := export(filepath.Join(t.TempDir(), "report.csv")); err != nil {
					t.Fatalf("legitimate %s export: %v", kind, err)
				}
				for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
					if err := export(target + suffix); err == nil {
						t.Fatalf("%s export overwrote selected project %q", kind, suffix)
					}
					if !reflect.DeepEqual(beforeTarget, stampAppDBFile(t, target)) || !reflect.DeepEqual(beforeDecoy, stampAppDBFile(t, decoy)) {
						t.Fatal("read/export changed target or decoy")
					}
					if suffix != "" {
						if _, err := os.Stat(target + suffix); !os.IsNotExist(err) {
							t.Fatalf("journal destination %q: %v", suffix, err)
						}
					}
				}
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLegacyPortfolioPreservesContractAndFractionalLabour(t *testing.T) {
	for _, missing := range [][]string{{"hourly_rate_minor_units"}, {"contract_value_minor_units"}, {"availability"}, {"hourly_rate_minor_units", "contract_value_minor_units", "availability"}} {
		t.Run(strings.Join(missing, ","), func(t *testing.T) {
			d, project := newPortfolioEVMTestProject(t, "2026-01-05")
			if _, err := d.SaveStakeholder(db.Stakeholder{ProjectID: project.ID, Name: "Ada", HourlyRateMinorUnits: 12345, Availability: 0.5}); err != nil {
				t.Fatal(err)
			}
			if _, err := agile.NewStore(d.Conn, project.ID).SaveWorkItem(agile.WorkItem{Title: "Build", Points: 1.5, Assignee: "Ada"}); err != nil {
				t.Fatal(err)
			}
			for _, column := range missing {
				omitAppSQLiteColumn(t, d, "stakeholders", column)
			}
			if len(missing) == 1 && missing[0] == "availability" {
				// Existing integer money stays authoritative even when the
				// compatibility floats have stale, disagreeing values.
				if _, err := d.Conn.Exec(`UPDATE stakeholders SET hourly_rate = 1 WHERE name = 'Ada'; UPDATE stakeholders SET contract_value = 1 WHERE name = 'Supplier'`); err != nil {
					t.Fatal(err)
				}
			}
			freezeAppDeleteJournal(t, d)
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			removeAppSQLiteSidecars(t, d.Path)
			before := stampAppDBFile(t, d.Path)
			// A separately migrated copy is a compatibility oracle; literal
			// totals independently protect against shared conversion mistakes.
			copyPath := filepath.Join(t.TempDir(), "migrated.gopmgr")
			if err := os.WriteFile(copyPath, before.Data, 0o600); err != nil {
				t.Fatal(err)
			}
			migrated, err := db.InitDB(copyPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = migrated.Close() })
			ro, err := openHeadlessDB(&cli.Config{CheckOnly: true, ProjectPath: d.Path})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ro.Close() })
			asOf := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
			got, err := portfolioProjectMetrics(ro, project, "", asOf)
			if err != nil {
				t.Fatal(err)
			}
			want, err := portfolioProjectMetrics(migrated, project, "", asOf)
			if err != nil {
				t.Fatal(err)
			}
			if got.BudgetedCostMinorUnits != 100000 || got.CommittedCostMinorUnits != 38518 {
				t.Fatalf("budget/committed = %d/%d, want 100000/38518 (20000 contract + round(12345 * 1.5))", got.BudgetedCostMinorUnits, got.CommittedCostMinorUnits)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("read-only metrics %+v differ from migrated %+v", got, want)
			}
			people, err := ro.ListStakeholders(project.ID, "team")
			if err != nil || len(people) != 1 {
				t.Fatalf("team = %+v, %v", people, err)
			}
			availability := 0.5
			for _, column := range missing {
				if column == "availability" {
					availability = 1
				}
			}
			if people[0].Availability != availability || people[0].HourlyRateMinorUnits != 12345 {
				t.Fatalf("legacy defaults/value = %+v", people[0])
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, stampAppDBFile(t, d.Path)) {
				t.Fatal("portfolio read altered legacy source")
			}
		})
	}
}

func TestLegacyScheduleKeepsChronologyAndHeadlessExportData(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "legacy"}[legacy], func(t *testing.T) {
			d, project := newPortfolioEVMTestProject(t, "2026-01-05")
			for _, row := range []struct {
				id, stamp, data string
				nanos           int64
			}{
				{"older", "2026-01-05T00:00:00Z", `{"nodes":[{"id":"old","label":"Old","duration":1}],"edges":[]}`, 1767571200000000000},
				{"newer", "2026-01-05T00:00:00.5Z", `{"nodes":[{"id":"new","label":"Newest","duration":2}],"edges":[]}`, 1767571200500000000},
			} {
				if _, err := d.Conn.Exec(`INSERT INTO charts(id,project_id,kind,title,data,updated_at,updated_at_unixnano) VALUES (?,?,'cpm',?,?,?,?)`, row.id, project.ID, row.id, row.data, row.stamp, row.nanos); err != nil {
					t.Fatal(err)
				}
			}
			if legacy {
				omitAppSQLiteColumn(t, d, "charts", "updated_at_unixnano")
			}
			freezeAppDeleteJournal(t, d)
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			ro, err := db.OpenReadOnly(d.Path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ro.Close() })
			tasks, err := loadCurrentProjectSchedule(ro, project.ID)
			if err != nil || len(tasks) != 1 || tasks["new"] == nil {
				t.Fatalf("schedule = %+v, %v; want newest chart", tasks, err)
			}
			out := filepath.Join(t.TempDir(), "schedule.csv")
			if err := runHeadlessExport(&cli.Config{ExportPath: out, ExportFormat: "csv"}, ro); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(out)
			if err != nil || !strings.Contains(string(data), "Newest") || strings.Contains(string(data), "Old,") {
				t.Fatalf("export = %s, %v", data, err)
			}
		})
	}
}

func TestBrokenScheduleIsNotAnEmptyExportOrUnavailableEVM(t *testing.T) {
	for _, broken := range []string{"chart data", "chart schema"} {
		t.Run(broken, func(t *testing.T) {
			d, project := newPortfolioEVMTestProject(t, "2026-01-05")
			if broken == "chart data" {
				savePortfolioEVMChart(t, d, project.ID, `{"nodes":"corrupt"}`)
			} else if _, err := d.Conn.Exec(`ALTER TABLE charts RENAME COLUMN data TO broken_data`); err != nil {
				t.Fatal(err)
			}
			if _, err := portfolioProjectMetrics(d, project, "", time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)); err == nil {
				t.Error("broken schedule became unavailable EVM")
			}
			out := filepath.Join(t.TempDir(), "schedule.csv")
			if err := runHeadlessExport(&cli.Config{ExportPath: out, ExportFormat: "csv"}, d); err == nil {
				t.Error("broken schedule exported successfully")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("failed export created output: %v", err)
			}
		})
	}
}

func TestAdminLegacyCostsKeepAmountsAndDefaultOnlyMissingDetail(t *testing.T) {
	optional := []string{"quantity_milli_units", "unit", "item_name", "sku", "supplier_name", "invoice_reference"}
	cases := append([]string{}, optional...)
	cases = append(cases, "all", "missing entries", "missing baselines", "malformed entries", "malformed baselines")
	for _, missing := range cases {
		t.Run(missing, func(t *testing.T) {
			app, path := viewerFixtureAppWithPreparation(t, func(d *db.Database) {
				switch missing {
				case "missing entries":
					if _, err := d.Conn.Exec(`DROP TABLE cost_entries`); err != nil {
						t.Fatal(err)
					}
				case "missing baselines":
					if _, err := d.Conn.Exec(`DROP TABLE cost_baseline_snapshots`); err != nil {
						t.Fatal(err)
					}
				case "malformed entries":
					if _, err := d.Conn.Exec(`DELETE FROM cost_entries; ALTER TABLE cost_entries RENAME COLUMN amount_minor_units TO missing_amount`); err != nil {
						t.Fatal(err)
					}
				case "malformed baselines":
					if _, err := d.Conn.Exec(`ALTER TABLE cost_baseline_snapshots RENAME COLUMN planned_minor_units TO missing_amount`); err != nil {
						t.Fatal(err)
					}
				case "all":
					for _, col := range optional {
						omitAppSQLiteColumn(t, d, "cost_entries", col)
					}
				default:
					omitAppSQLiteColumn(t, d, "cost_entries", missing)
				}
			})
			before := stampAppDBFile(t, path)
			copyBefore := stampAppDBFile(t, app.access.view.Path)
			got, err := app.AdminViewCosts()
			if strings.HasPrefix(missing, "malformed") {
				if err == nil {
					t.Fatal("malformed present table was accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if missing == "missing entries" {
					if len(got.Entries) != 0 {
						t.Fatal("absent ledger is not empty")
					}
				} else {
					if len(got.Entries) != 1 || got.Entries[0].Amount != "500.00" {
						t.Fatalf("lost ledger amount: %+v", got.Entries)
					}
					wantSupplier := "Acme Steel"
					if missing == "supplier_name" || missing == "all" {
						wantSupplier = ""
					}
					if got.Entries[0].SupplierName != wantSupplier {
						t.Fatalf("supplier = %q", got.Entries[0].SupplierName)
					}
				}
			}
			if !reflect.DeepEqual(before, stampAppDBFile(t, path)) || !reflect.DeepEqual(copyBefore, stampAppDBFile(t, app.access.view.Path)) {
				t.Fatal("administrator read altered owner or copy")
			}
		})
	}
}

func TestLegacyStatsAndAuditExportDoNotRequireAdditiveColumnsOrTables(t *testing.T) {
	d := seedHeadlessProject(t)
	for _, table := range []string{"charts", "documents"} {
		omitAppSQLiteColumn(t, d, table, "updated_at_unixnano")
	}
	if _, err := d.Conn.Exec(`DROP TABLE audit_events; DROP TABLE audit_log; DROP TABLE stakeholders`); err != nil {
		t.Fatal(err)
	}
	freezeAppDeleteJournal(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	before := stampAppDBFile(t, d.Path)
	ro, err := db.OpenReadOnly(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	if err := printHeadlessStats(ro); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "audit.csv")
	if err := ro.ExportAuditCSV(out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "id,ts,actor,action,target_id,details\n" {
		t.Fatalf("audit = %q, %v", data, err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampAppDBFile(t, d.Path)) {
		t.Fatal("legacy read changed source")
	}
}

func TestStatsRejectsMalformedPresentTablesEvenWhenEmpty(t *testing.T) {
	for _, table := range []string{"charts", "documents", "stakeholders", "audit_events"} {
		for _, populated := range []bool{false, true} {
			t.Run(table+map[bool]string{false: "/empty", true: "/populated"}[populated], func(t *testing.T) {
				d := seedHeadlessProject(t)
				p, err := d.GetProject()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.Conn.Exec("DROP TABLE " + table + "; CREATE TABLE " + table + " (project_id TEXT)"); err != nil {
					t.Fatal(err)
				}
				if populated {
					if _, err := d.Conn.Exec("INSERT INTO "+table+" (project_id) VALUES (?)", p.ID); err != nil {
						t.Fatal(err)
					}
				}
				if err := printHeadlessStats(d); err == nil {
					t.Fatal("malformed table was accepted as valid statistics")
				}
			})
		}
	}
}

func TestHeadlessExportsCannotOverwriteProjectFiles(t *testing.T) {
	for _, kind := range []string{"schedule", "audit"} {
		for _, alias := range []string{"direct", "symlink", "hardlink", "parent symlink", "dangling symlink", "symlink parent traversal", "relative dangling traversal", "-wal", "-shm", "-journal", "-WAL", "-SHM", "-JOURNAL"} {
			t.Run(kind+"/"+alias, func(t *testing.T) {
				d := seedHeadlessProject(t)
				// A broken guard must fail an assertion rather than SIGBUS
				// during cleanup of an overwritten, mmap-backed SHM file.
				freezeAppDeleteJournal(t, d)
				export := func(path string) error {
					if kind == "audit" {
						return d.ExportAuditCSV(path)
					}
					return runHeadlessExport(&cli.Config{ExportPath: path, ExportFormat: "csv"}, d)
				}
				if err := export(filepath.Join(t.TempDir(), "control.csv")); err != nil {
					t.Fatalf("legitimate export: %v", err)
				}
				control := filepath.Join(t.TempDir(), "existing.csv")
				if err := os.WriteFile(control, []byte("old export"), 0o600); err != nil {
					t.Fatal(err)
				}
				controlLink := filepath.Join(t.TempDir(), "export-link")
				if err := os.Symlink(control, controlLink); err != nil {
					t.Fatal(err)
				}
				if err := export(controlLink); err != nil {
					t.Fatalf("legitimate output alias: %v", err)
				}
				danglingDir := t.TempDir()
				goodDangling := filepath.Join(danglingDir, "export-link")
				if err := os.Symlink("new-report.csv", goodDangling); err != nil {
					t.Fatal(err)
				}
				if err := export(goodDangling); err != nil {
					t.Fatalf("legitimate dangling output: %v", err)
				}
				if _, err := os.Stat(filepath.Join(danglingDir, "new-report.csv")); err != nil {
					t.Fatal(err)
				}
				target := d.Path
				wantRejected := true
				switch alias {
				case "symlink":
					target = filepath.Join(t.TempDir(), "alias")
					if err := os.Symlink(d.Path, target); err != nil {
						t.Fatal(err)
					}
				case "hardlink":
					target = filepath.Join(t.TempDir(), "alias")
					if err := os.Link(d.Path, target); err != nil {
						t.Fatal(err)
					}
				case "parent symlink":
					parent := filepath.Join(t.TempDir(), "linked-dir")
					if err := os.Symlink(filepath.Dir(d.Path), parent); err != nil {
						t.Fatal(err)
					}
					target = filepath.Join(parent, filepath.Base(d.Path)+"-wal")
				case "dangling symlink":
					target = filepath.Join(t.TempDir(), "dangling")
					if err := os.Symlink(d.Path+"-wal", target); err != nil {
						t.Fatal(err)
					}
				case "symlink parent traversal", "relative dangling traversal":
					subdir := filepath.Join(filepath.Dir(d.Path), "subdir")
					if err := os.Mkdir(subdir, 0o700); err != nil {
						t.Fatal(err)
					}
					linkDir := t.TempDir()
					jump := filepath.Join(linkDir, "jump")
					if err := os.Symlink(subdir, jump); err != nil {
						t.Fatal(err)
					}
					// Preserve traversal for the OS; filepath.Join would
					// remove jump/.. before following the symlink.
					relative := "jump" + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(d.Path)
					target = linkDir + string(os.PathSeparator) + relative + "-wal"
					goodOutput := linkDir + string(os.PathSeparator) + relative + "-report.csv"
					if alias == "relative dangling traversal" {
						target = filepath.Join(linkDir, "blocked-link")
						if err := os.Symlink(relative+"-wal", target); err != nil {
							t.Fatal(err)
						}
						goodOutput = filepath.Join(linkDir, "good-link")
						if err := os.Symlink(relative+"-report.csv", goodOutput); err != nil {
							t.Fatal(err)
						}
					}
					if err := export(goodOutput); err != nil {
						t.Fatalf("legitimate traversed output: %v", err)
					}
					if _, err := os.Stat(d.Path + "-report.csv"); err != nil {
						t.Fatal(err)
					}
				case "-wal", "-shm", "-journal":
					target += alias
				case "-WAL", "-SHM", "-JOURNAL":
					target += alias
					original, err := os.Stat(d.Path)
					if err != nil {
						t.Fatal(err)
					}
					caseVariant, err := os.Stat(filepath.Join(filepath.Dir(d.Path), "PROJECT.PMFORGE"))
					if err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					wantRejected = caseVariant != nil && os.SameFile(original, caseVariant)
				}
				before := stampAppDBFile(t, d.Path)
				if err := export(target); (err != nil) != wantRejected {
					t.Fatalf("export error = %v, want rejection %v", err, wantRejected)
				}
				if !reflect.DeepEqual(before, stampAppDBFile(t, d.Path)) {
					t.Fatal("rejected export changed project")
				}
			})
		}
	}
}
