// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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

type calendarAtomicFixture struct {
	path string
	key  []byte
}

func newCalendarAtomicFixture(t *testing.T, encrypted bool) calendarAtomicFixture {
	t.Helper()
	f := calendarAtomicFixture{path: filepath.Join(t.TempDir(), "calendars.gopmgr")}
	if encrypted {
		f.key = bytes.Repeat([]byte{0x42}, 32) // Disposable fixture key.
	}
	return f
}

func (f calendarAtomicFixture) open(t *testing.T) *Database {
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

// Reopen without migrations, which could repair or hide a failed write.
func (f calendarAtomicFixture) readOnly(t *testing.T) *Database {
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

func calendarRawConn(t *testing.T, d *Database, fn func(*sqlite3.SQLiteConn)) {
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

// Include raw JSON and integer timestamps omitted by the normalized DTO.
func calendarStoredRows(t *testing.T, d *Database) [][]any {
	t.Helper()
	rows, err := d.Conn.Query(`SELECT * FROM resource_calendars ORDER BY id`)
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

func checkCalendarTimestampPairs(t *testing.T, d *Database, id string) {
	t.Helper()
	var created, updated string
	var createdNS, updatedNS int64
	if err := d.Conn.QueryRow(`SELECT created_at, updated_at, created_at_unixnano,
		updated_at_unixnano FROM resource_calendars WHERE id = ?`, id).
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

func TestSaveResourceCalendarFailureIsAtomic(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, update := range []bool{false, true} {
			for _, fault := range []string{"begin", "write", "readback", "scan", "weekly_capacity", "overrides", "skill_tags", "notes", "missing", "scope", "commit"} {
				t.Run(fmt.Sprintf("encrypted=%t/update=%t/%s", encrypted, update, fault), func(t *testing.T) {
					f := newCalendarAtomicFixture(t, encrypted)
					d := f.open(t)
					p, err := d.UpsertProject(Project{Name: "Calendar atomicity"})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES ('other', 'Other')`); err != nil {
						t.Fatal(err)
					}
					draft := ResourceCalendar{
						ProjectID: p.ID, Resource: "alice", Name: "Changed calendar", DefaultCapacity: 0.75,
						WeeklyCapacity: map[int]float64{0: 0.5, 4: 0}, Overrides: map[int]float64{-1: 0, 12: 1.25},
						SkillTags: []string{"qa", "piping"}, Notes: map[int]string{-1: "leave", 12: "午前"},
					}
					historical := time.Date(2001, 2, 3, 4, 5, 6, 123456789, time.UTC)
					if update {
						original, err := d.SaveResourceCalendar(ResourceCalendar{ProjectID: p.ID, Resource: "original"})
						if err != nil {
							t.Fatal(err)
						}
						draft.ID = original.ID
						if _, err := d.Conn.Exec(`UPDATE resource_calendars SET created_at = ?, updated_at = ?,
							created_at_unixnano = ?, updated_at_unixnano = ? WHERE id = ?`,
							historical.Format(time.RFC3339Nano), historical.Format(time.RFC3339Nano),
							historical.UnixNano(), historical.UnixNano(), draft.ID); err != nil {
							t.Fatal(err)
						}
					}
					before := calendarStoredRows(t, d)
					hits := 0
					trigger := ""
					switch fault {
					case "scan":
						trigger = "UPDATE resource_calendars SET default_capacity = 'invalid' WHERE id = NEW.id;"
					case "weekly_capacity", "overrides", "skill_tags", "notes":
						trigger = "UPDATE resource_calendars SET " + fault + " = '{' WHERE id = NEW.id;"
					case "missing":
						trigger = "DELETE FROM resource_calendars WHERE id = NEW.id;"
					case "scope":
						trigger = "UPDATE resource_calendars SET project_id = 'other' WHERE id = NEW.id;"
					}
					if trigger != "" {
						event := "INSERT"
						if update {
							event = "UPDATE"
						}
						if _, err := d.Conn.Exec(`CREATE TRIGGER corrupt_readback AFTER ` + event + ` ON resource_calendars BEGIN ` + trigger + ` END`); err != nil {
							t.Fatal(err)
						}
					} else {
						calendarRawConn(t, d, func(c *sqlite3.SQLiteConn) {
							if fault == "commit" {
								c.RegisterCommitHook(func() int { hits++; return 1 })
								return
							}
							c.RegisterAuthorizer(func(op int, arg, _, _ string) int {
								if (fault == "begin" && op == sqlite3.SQLITE_TRANSACTION && arg == "BEGIN") ||
									(fault == "write" && op == sqlite3.SQLITE_INSERT && arg == "resource_calendars") ||
									(fault == "readback" && op == sqlite3.SQLITE_SELECT) {
									hits++
									return sqlite3.SQLITE_DENY
								}
								return sqlite3.SQLITE_OK
							})
						})
					}
					got, saveErr := d.SaveResourceCalendar(draft)
					calendarRawConn(t, d, func(c *sqlite3.SQLiteConn) {
						c.RegisterAuthorizer(nil)
						c.RegisterCommitHook(nil)
						if !c.AutoCommit() {
							t.Error("failed save left an active transaction")
						}
					})
					switch fault {
					case "scan":
						if saveErr == nil || !strings.Contains(saveErr.Error(), "converting driver.Value") {
							t.Errorf("want scan-conversion error, got %v", saveErr)
						}
					case "weekly_capacity", "overrides", "skill_tags", "notes":
						var syntaxErr *json.SyntaxError
						if !errors.As(saveErr, &syntaxErr) {
							t.Errorf("want JSON syntax error for %s, got %v", fault, saveErr)
						}
					case "missing", "scope":
						if !errors.Is(saveErr, ErrNoResourceCalendar) {
							t.Errorf("want missing scoped calendar, got %v", saveErr)
						}
					default:
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
					if !reflect.DeepEqual(got, ResourceCalendar{}) {
						t.Errorf("failed save returned a record: %#v", got)
					}
					if trigger != "" {
						if _, err := d.Conn.Exec(`DROP TRIGGER corrupt_readback`); err != nil {
							t.Fatal(err)
						}
					}
					if err := d.Close(); err != nil {
						t.Fatal(err)
					}
					ro := f.readOnly(t)
					if after := calendarStoredRows(t, ro); !reflect.DeepEqual(after, before) {
						t.Errorf("failed save changed durable state:\n got %#v\nwant %#v", after, before)
					}
					if err := ro.Close(); err != nil {
						t.Fatal(err)
					}
					d = f.open(t)
					saved, err := d.SaveResourceCalendar(draft)
					if err != nil {
						t.Fatalf("explicit retry: %v", err)
					}
					if !strings.HasPrefix(saved.ID, "rcal_") || (update && saved.ID != draft.ID) {
						t.Fatalf("retry identity: %#v", saved)
					}
					if update && !saved.CreatedAt.Equal(historical) {
						t.Error("retry replaced historical creation timestamp")
					}
					want := draft
					want.ID, want.CreatedAt, want.UpdatedAt = saved.ID, saved.CreatedAt, saved.UpdatedAt
					if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() || !reflect.DeepEqual(saved, want) {
						t.Errorf("retry fields:\n got %#v\nwant %#v", saved, want)
					}
					checkCalendarTimestampPairs(t, d, saved.ID)
					committed := calendarStoredRows(t, d)
					if len(committed) != 1 {
						t.Errorf("retry persisted %d rows, want one", len(committed))
					}
					if err := d.Close(); err != nil {
						t.Fatal(err)
					}
					ro = f.readOnly(t)
					persisted, err := ro.GetResourceCalendar(saved.ID)
					if err != nil || !reflect.DeepEqual(persisted, saved) || !reflect.DeepEqual(calendarStoredRows(t, ro), committed) {
						t.Fatalf("committed retry changed on reopen: got %#v error=%v", persisted, err)
					}
					checkCalendarTimestampPairs(t, ro, saved.ID)
				})
			}
		}
	}
}

func TestSaveResourceCalendarRejectsForeignProjectCollision(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(fmt.Sprintf("encrypted=%t", encrypted), func(t *testing.T) {
			f := newCalendarAtomicFixture(t, encrypted)
			d := f.open(t)
			p, err := d.UpsertProject(Project{Name: "Owner"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES ('other', 'Other')`); err != nil {
				t.Fatal(err)
			}
			original, err := d.SaveResourceCalendar(ResourceCalendar{ProjectID: p.ID, Resource: "owned"})
			if err != nil {
				t.Fatal(err)
			}
			before := calendarStoredRows(t, d)
			updates := 0
			calendarRawConn(t, d, func(c *sqlite3.SQLiteConn) {
				c.RegisterUpdateHook(func(op int, _, table string, _ int64) {
					if op == sqlite3.SQLITE_UPDATE && table == "resource_calendars" {
						updates++
					}
				})
			})
			draft := original
			draft.ProjectID, draft.Name, draft.DefaultCapacity = "other", "Must not persist", 0.25
			got, err := d.SaveResourceCalendar(draft)
			calendarRawConn(t, d, func(c *sqlite3.SQLiteConn) {
				c.RegisterUpdateHook(nil)
				if !c.AutoCommit() {
					t.Error("collision left an active transaction")
				}
			})
			if !errors.Is(err, ErrNoResourceCalendar) || !reflect.DeepEqual(got, ResourceCalendar{}) || updates != 0 {
				t.Errorf("foreign collision: record=%#v error=%v calendar updates=%d", got, err, updates)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			if after := calendarStoredRows(t, f.readOnly(t)); !reflect.DeepEqual(after, before) {
				t.Errorf("foreign row changed:\n got %#v\nwant %#v", after, before)
			}
		})
	}
}

func TestSaveResourceCalendarPreservesDefaults(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, capacity := range []float64{0, -1, 0.25} {
			t.Run(fmt.Sprintf("encrypted=%t/capacity=%g", encrypted, capacity), func(t *testing.T) {
				f := newCalendarAtomicFixture(t, encrypted)
				d := f.open(t)
				p, err := d.UpsertProject(Project{Name: "Defaults"})
				if err != nil {
					t.Fatal(err)
				}
				saved, err := d.SaveResourceCalendar(ResourceCalendar{ID: "caller-id", ProjectID: p.ID, Resource: "alice", DefaultCapacity: capacity})
				if err != nil {
					t.Fatal(err)
				}
				wantCapacity := capacity
				if capacity <= 0 {
					wantCapacity = 1
				}
				if saved.ID != "caller-id" || saved.ProjectID != p.ID || saved.Resource != "alice" || saved.Name != "alice" || saved.DefaultCapacity != wantCapacity ||
					!reflect.DeepEqual(saved.WeeklyCapacity, map[int]float64{}) || !reflect.DeepEqual(saved.Overrides, map[int]float64{}) ||
					!reflect.DeepEqual(saved.SkillTags, []string{}) || !reflect.DeepEqual(saved.Notes, map[int]string{}) {
					t.Fatalf("identity/defaults changed: %#v", saved)
				}
				var weekly, overrides, tags, notes string
				if err := d.Conn.QueryRow(`SELECT weekly_capacity, overrides, skill_tags, notes FROM resource_calendars WHERE id = ?`, saved.ID).
					Scan(&weekly, &overrides, &tags, &notes); err != nil {
					t.Fatal(err)
				}
				if weekly != "{}" || overrides != "{}" || tags != "[]" || notes != "{}" {
					t.Fatalf("stored JSON defaults changed: %q %q %q %q", weekly, overrides, tags, notes)
				}
				checkCalendarTimestampPairs(t, d, saved.ID)
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
				got, err := f.readOnly(t).GetResourceCalendar(saved.ID)
				if err != nil || !reflect.DeepEqual(got, saved) {
					t.Fatalf("defaults round-trip: got %#v error=%v", got, err)
				}
			})
		}
	}
}

func TestSaveResourceCalendarRejectsUnencodableCapacity(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, column := range []string{"weekly", "overrides"} {
			for _, value := range []float64{math.NaN(), math.Inf(1)} {
				t.Run(fmt.Sprintf("encrypted=%t/%s/%v", encrypted, column, value), func(t *testing.T) {
					f := newCalendarAtomicFixture(t, encrypted)
					d := f.open(t)
					p, err := d.UpsertProject(Project{Name: "Invalid capacity"})
					if err != nil {
						t.Fatal(err)
					}
					draft := ResourceCalendar{ProjectID: p.ID, Resource: "alice"}
					if column == "weekly" {
						draft.WeeklyCapacity = map[int]float64{0: value}
					} else {
						draft.Overrides = map[int]float64{0: value}
					}
					got, err := d.SaveResourceCalendar(draft)
					var unsupported *json.UnsupportedValueError
					if !errors.As(err, &unsupported) || !reflect.DeepEqual(got, ResourceCalendar{}) {
						t.Errorf("unencodable input: got %#v error=%v", got, err)
					}
					if err := d.Close(); err != nil {
						t.Fatal(err)
					}
					if rows := calendarStoredRows(t, f.readOnly(t)); len(rows) != 0 {
						t.Fatalf("invalid input persisted rows: %#v", rows)
					}
				})
			}
		}
	}
}
