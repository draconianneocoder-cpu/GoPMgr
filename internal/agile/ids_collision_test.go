// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"
)

// BL-2: a create whose generated ID already names a stored row is refused
// and leaves that row unchanged. Row A is stored under the literal ID the
// fixed entropy "0123456789abcdef" encodes to; the next create generates
// exactly that ID. Each case is a distinct upsert, so a guard missing from
// any one of them fails its own case.
func TestGeneratedIDCollisionIsRefused(t *testing.T) {
	const body = "30313233343536373839616263646566"
	cases := map[string]struct {
		table  string
		seedA  func(s *Store, boardID, id string) (func() (any, error), error)
		create func(s *Store, boardID string) error
	}{
		"column": {
			table: "agile_columns",
			seedA: func(s *Store, boardID, id string) (func() (any, error), error) {
				_, err := s.SaveColumn(Column{ID: id, BoardID: boardID, Name: "A", OrderIdx: 9})
				return func() (any, error) { return findByID(s.ListColumns(boardID))(id) }, err
			},
			create: func(s *Store, boardID string) error {
				_, err := s.SaveColumn(Column{BoardID: boardID, Name: "B", OrderIdx: 10})
				return err
			},
		},
		"work item": {
			table: "agile_work_items",
			seedA: func(s *Store, _ string, id string) (func() (any, error), error) {
				_, err := s.SaveWorkItem(WorkItem{ID: id, Title: "A"})
				return func() (any, error) { return s.GetWorkItem(id) }, err
			},
			create: func(s *Store, _ string) error { _, err := s.SaveWorkItem(WorkItem{Title: "B"}); return err },
		},
		"sprint": {
			table: "agile_sprints",
			seedA: func(s *Store, _ string, id string) (func() (any, error), error) {
				_, err := s.SaveSprint(Sprint{ID: id, Name: "A"})
				return func() (any, error) { return s.GetSprint(id) }, err
			},
			create: func(s *Store, _ string) error { _, err := s.SaveSprint(Sprint{Name: "B"}); return err },
		},
		"deployment": {
			table: "agile_deployments",
			seedA: func(s *Store, _ string, id string) (func() (any, error), error) {
				_, err := s.SaveDeployment(Deployment{ID: id, Version: "A", TS: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
				return func() (any, error) { return findByID(s.ListDeployments(time.Time{}))(id) }, err
			},
			create: func(s *Store, _ string) error {
				_, err := s.SaveDeployment(Deployment{Version: "B", TS: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)})
				return err
			},
		},
	}
	prefixes := map[string]string{"column": "col", "work item": "wi", "sprint": "sprint", "deployment": "deploy"}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d, store, _ := newAgileTestStore(t)
			board, err := store.EnsureDefaultBoard()
			if err != nil {
				t.Fatalf("EnsureDefaultBoard: %v", err)
			}
			id := prefixes[name] + "_" + body
			readA, err := tc.seedA(store, board.ID, id)
			if err != nil {
				t.Fatalf("seed row A: %v", err)
			}
			before, err := readA()
			if err != nil {
				t.Fatalf("read A: %v", err)
			}
			var rowsBefore int
			if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM ` + tc.table).Scan(&rowsBefore); err != nil {
				t.Fatal(err)
			}

			original := rand.Reader
			rand.Reader = io.MultiReader(bytes.NewReader([]byte("0123456789abcdef")), original)
			t.Cleanup(func() { rand.Reader = original })

			if err := tc.create(store, board.ID); !errors.Is(err, ErrGeneratedIDCollision) {
				t.Fatalf("create under a taken generated ID: error = %v, want ErrGeneratedIDCollision", err)
			}
			after, err := readA()
			if err != nil {
				t.Fatalf("read A after: %v", err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("row A changed:\nbefore %+v\nafter  %+v", before, after)
			}
			var rowsAfter int
			if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM ` + tc.table).Scan(&rowsAfter); err != nil {
				t.Fatal(err)
			}
			if rowsAfter != rowsBefore {
				t.Fatalf("%s rows %d -> %d, want unchanged", tc.table, rowsBefore, rowsAfter)
			}
			if inUse := d.Conn.Stats().InUse; inUse != 0 {
				t.Fatalf("%d connections still in use after the refused save", inUse)
			}
			if err := tc.create(store, board.ID); err != nil {
				t.Fatalf("create after the refusal: %v", err)
			}
		})
	}
}

// findByID returns a lookup of one element of a listed result by its ID.
func findByID[T interface{ Column | Deployment }](list []T, err error) func(string) (any, error) {
	return func(id string) (any, error) {
		if err != nil {
			return nil, err
		}
		for _, v := range list {
			if rowID(v) == id {
				return v, nil
			}
		}
		return nil, fmt.Errorf("no row %s", id)
	}
}

func rowID(v any) string {
	switch r := v.(type) {
	case Column:
		return r.ID
	case Deployment:
		return r.ID
	}
	return ""
}
