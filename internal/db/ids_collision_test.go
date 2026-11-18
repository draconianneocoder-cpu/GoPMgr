// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"reflect"
	"testing"
)

// collisionBytes are the 16 entropy bytes the refused create reads; every
// generator encodes them as this hexadecimal body.
const (
	collisionBytes = "0123456789abcdef"
	collisionBody  = "30313233343536373839616263646566"
)

// BL-2: a create whose generated ID already names a stored record is
// refused, and nothing changes: not the record, its audit history, nor the
// table. Record A is stored under the literal ID the fixed entropy encodes
// to; the next create then generates exactly that ID. Each row is a distinct
// save path, so a missing guard in any one of them fails its own row.
func TestGeneratedIDCollisionIsRefused(t *testing.T) {
	type saver struct {
		table string
		// seedA stores record A under the given ID and returns how to read it.
		seedA func(d *Database, projectID, id string) (func() (any, error), error)
		// create saves a new record with an empty ID, so it generates one.
		create func(d *Database, projectID string) error
	}
	cases := map[string]saver{
		"project": {
			table: "project",
			seedA: func(d *Database, _ string, id string) (func() (any, error), error) {
				_, err := d.UpsertProject(Project{ID: id, Name: "A"})
				return func() (any, error) { return getProjectByIDForTest(d, id) }, err
			},
			create: func(d *Database, _ string) error { _, err := d.UpsertProject(Project{Name: "B"}); return err },
		},
		"chart": {
			table: "charts",
			seedA: func(d *Database, projectID, id string) (func() (any, error), error) {
				_, err := d.SaveChart(Chart{ID: id, ProjectID: projectID, Kind: "cpm", Title: "A"})
				return func() (any, error) { return d.GetChart(id) }, err
			},
			create: func(d *Database, projectID string) error {
				_, err := d.SaveChart(Chart{ProjectID: projectID, Kind: "cpm", Title: "B"})
				return err
			},
		},
		"document": {
			table: "documents",
			seedA: func(d *Database, projectID, id string) (func() (any, error), error) {
				_, err := d.SaveDocument(Document{ID: id, ProjectID: projectID, Kind: "charter_word", Title: "A"})
				return func() (any, error) { return d.GetDocument(id) }, err
			},
			create: func(d *Database, projectID string) error {
				_, err := d.SaveDocument(Document{ProjectID: projectID, Kind: "charter_word", Title: "B"})
				return err
			},
		},
		"scenario": {
			table: "scenarios",
			seedA: func(d *Database, projectID, id string) (func() (any, error), error) {
				_, err := d.SaveScenario(Scenario{ID: id, ProjectID: projectID, Name: "A"})
				return func() (any, error) { return d.GetScenario(id) }, err
			},
			create: func(d *Database, projectID string) error {
				_, err := d.SaveScenario(Scenario{ProjectID: projectID, Name: "B"})
				return err
			},
		},
		"stakeholder": {
			table: "stakeholders",
			seedA: func(d *Database, projectID, id string) (func() (any, error), error) {
				_, err := d.SaveStakeholder(Stakeholder{ID: id, ProjectID: projectID, Name: "A"})
				return func() (any, error) { return d.GetStakeholder(id) }, err
			},
			create: func(d *Database, projectID string) error {
				_, err := d.SaveStakeholder(Stakeholder{ProjectID: projectID, Name: "B"})
				return err
			},
		},
		"resource calendar": {
			table: "resource_calendars",
			seedA: func(d *Database, projectID, id string) (func() (any, error), error) {
				_, err := d.SaveResourceCalendar(ResourceCalendar{ID: id, ProjectID: projectID, Resource: "A"})
				return func() (any, error) { return d.GetResourceCalendar(id) }, err
			},
			create: func(d *Database, projectID string) error {
				_, err := d.SaveResourceCalendar(ResourceCalendar{ProjectID: projectID, Resource: "B"})
				return err
			},
		},
	}
	prefixes := map[string]string{
		"project": "prj", "chart": "chart", "document": "doc", "scenario": "scn",
		"stakeholder": "stk", "resource calendar": "rcal",
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := newBackupTestDB(t)
			projectID := ""
			if name != "project" {
				p, err := d.UpsertProject(Project{Name: "Collision"})
				if err != nil {
					t.Fatalf("UpsertProject: %v", err)
				}
				projectID = p.ID
			}
			idA := prefixes[name] + "_" + collisionBody
			readA, err := tc.seedA(d, projectID, idA)
			if err != nil {
				t.Fatalf("seed record A: %v", err)
			}
			before, err := readA()
			if err != nil {
				t.Fatalf("read A: %v", err)
			}
			eventsBefore := auditEventsForTest(t, d)

			// The create reads the fixed bytes first; anything after reads
			// real entropy.
			original := rand.Reader
			rand.Reader = io.MultiReader(bytes.NewReader([]byte(collisionBytes)), original)
			t.Cleanup(func() { rand.Reader = original })

			if err := tc.create(d, projectID); !errors.Is(err, ErrGeneratedIDCollision) {
				t.Fatalf("create under a taken generated ID: error = %v, want ErrGeneratedIDCollision", err)
			}
			after, err := readA()
			if err != nil {
				t.Fatalf("read A after: %v", err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("record A changed:\nbefore %+v\nafter  %+v", before, after)
			}
			if got := auditEventsForTest(t, d); !reflect.DeepEqual(eventsBefore, got) {
				t.Fatalf("audit history changed: %d events before, %d after", len(eventsBefore), len(got))
			}
			var rows int
			if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM ` + tc.table).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			if rows != 1 {
				t.Fatalf("%s has %d rows, want only record A", tc.table, rows)
			}
			// The refused save left no transaction open, and the next create,
			// now under real entropy, succeeds.
			if inUse := d.Conn.Stats().InUse; inUse != 0 {
				t.Fatalf("%d connections still in use after the refused save", inUse)
			}
			if err := tc.create(d, projectID); err != nil {
				t.Fatalf("create after the refusal: %v", err)
			}
		})
	}
}

func getProjectByIDForTest(d *Database, id string) (Project, error) {
	tx, err := d.Conn.Begin()
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback() }()
	return getProjectByIDTx(tx, id)
}

// auditEventsForTest lists every audit event, whichever project it names.
func auditEventsForTest(t *testing.T, d *Database) []AuditEvent {
	t.Helper()
	var out []AuditEvent
	rows, err := d.Conn.Query(`SELECT DISTINCT project_id FROM audit_events ORDER BY project_id`)
	if err != nil {
		t.Fatal(err)
	}
	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	for _, p := range projects {
		events, err := d.ListAuditEvents(p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, events...)
	}
	return out
}
