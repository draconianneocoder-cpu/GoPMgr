// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"

	"gopmgr/internal/sqlitedriver"
)

type stakeholderAtomicFixture struct {
	path string
	key  []byte
}

func newStakeholderAtomicFixture(t *testing.T, encrypted bool) stakeholderAtomicFixture {
	t.Helper()
	f := stakeholderAtomicFixture{path: filepath.Join(t.TempDir(), "stakeholders.gopmgr")}
	if encrypted {
		f.key = bytes.Repeat([]byte{0x42}, 32) // Disposable fixture key.
	}
	return f
}

func (f stakeholderAtomicFixture) open(t *testing.T) *Database {
	t.Helper()
	var d *Database
	var err error
	if f.key != nil {
		d, err = InitEncryptedDB(f.path, f.key)
	} else {
		d, err = InitDB(f.path)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	d.Conn.SetMaxOpenConns(1)
	d.Conn.SetMaxIdleConns(1)
	return d
}

// Reopen without running migrations, which could repair or hide a failed write.
func (f stakeholderAtomicFixture) readOnly(t *testing.T) *Database {
	t.Helper()
	dsn := (&url.URL{Scheme: "file", Path: f.path}).String() + "?mode=ro&_query_only=1"
	if f.key != nil {
		dsn += fmt.Sprintf("&_pragma_key=x'%x'", f.key)
	}
	c, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return &Database{Conn: c, Path: f.path}
}

func stakeholderRawConn(t *testing.T, d *Database, fn func(*sqlite3.SQLiteConn)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := d.Conn.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Raw(func(raw any) error { fn(raw.(*sqlite3.SQLiteConn)); return nil })
	if err := errors.Join(err, c.Close()); err != nil {
		t.Fatal(err)
	}
}

// Compare all stored columns, not the normalized DTO (which omits integer timestamps).
func stakeholderStoredRows(t *testing.T, d *Database) [][]any {
	t.Helper()
	rows, err := d.Conn.Query(`SELECT * FROM stakeholders ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out [][]any
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		out = append(out, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func checkStakeholderTimestampPairs(t *testing.T, d *Database, id string) {
	t.Helper()
	var created, updated string
	var createdNS, updatedNS int64
	if err := d.Conn.QueryRow(`SELECT created_at, updated_at, created_at_unixnano,
		updated_at_unixnano FROM stakeholders WHERE id = ?`, id).
		Scan(&created, &updated, &createdNS, &updatedNS); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		text string
		ns   int64
	}{{created, createdNS}, {updated, updatedNS}} {
		parsed, err := time.Parse(time.RFC3339Nano, pair.text)
		if err != nil || parsed.UnixNano() != pair.ns {
			t.Errorf("timestamp pair disagrees: %q / %d, parse error %v", pair.text, pair.ns, err)
		}
	}
}

func TestSaveStakeholderFailureIsAtomic(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, update := range []bool{false, true} {
			for _, fault := range []string{"begin", "write", "readback", "scan", "commit"} {
				t.Run(fmt.Sprintf("encrypted=%t/update=%t/%s", encrypted, update, fault), func(t *testing.T) {
					f := newStakeholderAtomicFixture(t, encrypted)
					d := f.open(t)
					project, err := d.UpsertProject(Project{Name: "Stakeholder atomicity"})
					if err != nil {
						t.Fatal(err)
					}
					draft := Stakeholder{
						ProjectID: project.ID, Name: "Changed stakeholder", Role: "Engineer",
						Organisation: "Fixture", Email: "fixture@example.invalid", Phone: "123",
						Category: StakeholderVendor, HourlyRate: 88.25, HourlyRateMinorUnits: 8825,
						ContractValue: 1200.99, ContractValueMinorUnits: 120099,
						Availability: 0.5, Notes: "Changed notes", CreatedAt: "caller ignored", UpdatedAt: "caller ignored",
					}
					historical := time.Date(2001, 2, 3, 4, 5, 6, 123456789, time.UTC)
					if update {
						original, err := d.SaveStakeholder(Stakeholder{ProjectID: project.ID, Name: "Original"})
						if err != nil {
							t.Fatal(err)
						}
						draft.ID = original.ID
						if _, err := d.Conn.Exec(`UPDATE stakeholders SET created_at = ?, updated_at = ?,
							created_at_unixnano = ?, updated_at_unixnano = ? WHERE id = ?`,
							historical.Format(time.RFC3339Nano), historical.Format(time.RFC3339Nano),
							historical.UnixNano(), historical.UnixNano(), draft.ID); err != nil {
							t.Fatal(err)
						}
					}
					before := stakeholderStoredRows(t, d)
					hits := 0
					if fault == "scan" {
						event := "INSERT"
						if update {
							event = "UPDATE"
						}
						if _, err := d.Conn.Exec(`CREATE TRIGGER corrupt_readback AFTER ` + event + ` ON stakeholders
							BEGIN UPDATE stakeholders SET availability = 'invalid' WHERE id = NEW.id; END`); err != nil {
							t.Fatal(err)
						}
					} else {
						stakeholderRawConn(t, d, func(c *sqlite3.SQLiteConn) {
							if fault == "commit" {
								c.RegisterCommitHook(func() int { hits++; return 1 })
								return
							}
							c.RegisterAuthorizer(func(op int, arg, _, _ string) int {
								if (fault == "begin" && op == sqlite3.SQLITE_TRANSACTION && arg == "BEGIN") ||
									(fault == "write" && op == sqlite3.SQLITE_INSERT && arg == "stakeholders") ||
									(fault == "readback" && op == sqlite3.SQLITE_SELECT) {
									hits++
									return sqlite3.SQLITE_DENY
								}
								return sqlite3.SQLITE_OK
							})
						})
					}
					got, saveErr := d.SaveStakeholder(draft)
					stakeholderRawConn(t, d, func(c *sqlite3.SQLiteConn) {
						c.RegisterAuthorizer(nil)
						c.RegisterCommitHook(nil)
						if !c.AutoCommit() {
							t.Error("failed save left an active transaction")
						}
					})
					if fault == "scan" {
						if saveErr == nil || !strings.Contains(saveErr.Error(), "converting driver.Value") {
							t.Errorf("want scan-conversion error, got %v", saveErr)
						}
						if _, err := d.Conn.Exec(`DROP TRIGGER corrupt_readback`); err != nil {
							t.Fatal(err)
						}
					} else {
						var sqlErr sqlite3.Error
						wantCode := sqlite3.ErrAuth
						if fault == "commit" {
							wantCode = sqlite3.ErrConstraint
						}
						if hits != 1 || !errors.As(saveErr, &sqlErr) || sqlErr.Code != wantCode {
							t.Errorf("fault %s: hits=%d error=%v, want one injected %v", fault, hits, saveErr, wantCode)
						}
						if fault == "commit" && sqlErr.ExtendedCode != sqlite3.ErrConstraintCommitHook {
							t.Errorf("commit extended code = %v, want commit-hook rejection", sqlErr.ExtendedCode)
						}
					}
					if got != (Stakeholder{}) {
						t.Errorf("failed save returned a record: %#v", got)
					}
					if err := d.Close(); err != nil {
						t.Fatal(err)
					}
					ro := f.readOnly(t)
					if after := stakeholderStoredRows(t, ro); !reflect.DeepEqual(after, before) {
						t.Errorf("failed save changed durable state:\n got %#v\nwant %#v", after, before)
					}
					if err := ro.Close(); err != nil {
						t.Fatal(err)
					}
					d = f.open(t)
					saved, err := d.SaveStakeholder(draft)
					if err != nil {
						t.Fatalf("explicit retry: %v", err)
					}
					if !strings.HasPrefix(saved.ID, "stk_") || (update && saved.ID != draft.ID) {
						t.Fatalf("retry identity: %#v", saved)
					}
					if update && saved.CreatedAt != historical.Format(time.RFC3339Nano) {
						t.Error("retry replaced historical creation timestamp")
					}
					want := draft
					want.ID, want.CreatedAt, want.UpdatedAt = saved.ID, saved.CreatedAt, saved.UpdatedAt
					if !reflect.DeepEqual(saved, want) {
						t.Errorf("retry fields:\n got %#v\nwant %#v", saved, want)
					}
					checkStakeholderTimestampPairs(t, d, saved.ID)
					committed := stakeholderStoredRows(t, d)
					if len(committed) != 1 {
						t.Errorf("retry persisted %d rows, want one", len(committed))
					}
					if err := d.Close(); err != nil {
						t.Fatal(err)
					}
					ro = f.readOnly(t)
					persisted, err := ro.GetStakeholder(saved.ID)
					if err != nil || !reflect.DeepEqual(persisted, saved) ||
						!reflect.DeepEqual(stakeholderStoredRows(t, ro), committed) {
						t.Fatalf("committed retry changed on reopen: got %#v error=%v", persisted, err)
					}
					checkStakeholderTimestampPairs(t, ro, saved.ID)
				})
			}
		}
	}
}

func TestSaveStakeholderForeignProjectCollisionRollsBack(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(fmt.Sprintf("encrypted=%t", encrypted), func(t *testing.T) {
			f := newStakeholderAtomicFixture(t, encrypted)
			d := f.open(t)
			p, err := d.UpsertProject(Project{Name: "Owner"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES ('other', 'Other')`); err != nil {
				t.Fatal(err)
			}
			original, err := d.SaveStakeholder(Stakeholder{ProjectID: p.ID, Name: "Owned"})
			if err != nil {
				t.Fatal(err)
			}
			before := stakeholderStoredRows(t, d)
			draft := original
			draft.ProjectID, draft.Name = "other", "Must not persist"
			got, err := d.SaveStakeholder(draft)
			if !errors.Is(err, ErrNoStakeholder) || got != (Stakeholder{}) {
				t.Errorf("foreign collision: record=%#v error=%v", got, err)
			}
			stakeholderRawConn(t, d, func(c *sqlite3.SQLiteConn) {
				if !c.AutoCommit() {
					t.Error("collision left an active transaction")
				}
			})
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			if after := stakeholderStoredRows(t, f.readOnly(t)); !reflect.DeepEqual(after, before) {
				t.Errorf("foreign row changed:\n got %#v\nwant %#v", after, before)
			}
		})
	}
}

func TestSaveStakeholderPreservesDefaultsAndExactMoney(t *testing.T) {
	cases := []struct {
		name                     string
		draft                    Stakeholder
		wantRate, wantContract   float64
		wantRateMinor, wantMinor int64
		wantCategory             StakeholderCategory
		wantAvailability         float64
	}{
		{"zero defaults", Stakeholder{}, 0, 0, 0, 0, StakeholderTeam, 1},
		{"legacy major units", Stakeholder{Availability: -1, HourlyRate: 88.25, ContractValue: 1200.99},
			88.25, 1200.99, 8825, 120099, StakeholderTeam, 1},
		{"exact minor units win", Stakeholder{ID: "caller-id", Category: StakeholderExternal, Availability: 0.25,
			HourlyRate: 1, HourlyRateMinorUnits: 8825, ContractValue: 2, ContractValueMinorUnits: -120099},
			88.25, -1200.99, 8825, -120099, StakeholderExternal, 0.25},
		{"int64 boundaries", Stakeholder{HourlyRateMinorUnits: math.MaxInt64, ContractValueMinorUnits: math.MinInt64},
			float64(math.MaxInt64) / 100, float64(math.MinInt64) / 100,
			math.MaxInt64, math.MinInt64, StakeholderTeam, 1},
	}
	for _, encrypted := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("encrypted=%t/%s", encrypted, tc.name), func(t *testing.T) {
				f := newStakeholderAtomicFixture(t, encrypted)
				d := f.open(t)
				p, err := d.UpsertProject(Project{Name: "Compatibility"})
				if err != nil {
					t.Fatal(err)
				}
				draft := tc.draft
				draft.ProjectID, draft.Name = p.ID, "Compatibility stakeholder"
				draft.CreatedAt, draft.UpdatedAt = "caller ignored", "caller ignored"
				saved, err := d.SaveStakeholder(draft)
				if err != nil {
					t.Fatal(err)
				}
				if saved.ID == "" || (draft.ID != "" && saved.ID != draft.ID) ||
					saved.Category != tc.wantCategory || saved.Availability != tc.wantAvailability ||
					saved.HourlyRate != tc.wantRate || saved.ContractValue != tc.wantContract ||
					saved.HourlyRateMinorUnits != tc.wantRateMinor || saved.ContractValueMinorUnits != tc.wantMinor {
					t.Fatalf("identity/defaults/money changed: %#v", saved)
				}
				var rate, contract int64
				if err := d.Conn.QueryRow(`SELECT hourly_rate_minor_units, contract_value_minor_units
					FROM stakeholders WHERE id = ?`, saved.ID).Scan(&rate, &contract); err != nil {
					t.Fatal(err)
				}
				if rate != tc.wantRateMinor || contract != tc.wantMinor {
					t.Fatalf("stored minor units lost precision: %d / %d", rate, contract)
				}
				checkStakeholderTimestampPairs(t, d, saved.ID)
				before := stakeholderStoredRows(t, d)
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
				ro := f.readOnly(t)
				got, err := ro.GetStakeholder(saved.ID)
				if err != nil || !reflect.DeepEqual(got, saved) || !reflect.DeepEqual(stakeholderStoredRows(t, ro), before) {
					t.Fatalf("compatibility round-trip: got %#v error=%v", got, err)
				}
			})
		}
	}
}
