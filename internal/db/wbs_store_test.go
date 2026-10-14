// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gopmgr/internal/wbs"
)

func wbsFixture(t *testing.T, encrypted bool) (*Database, *WBSStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wbs.gopmgr")
	var d *Database
	var err error
	if encrypted {
		d, err = InitEncryptedDB(path, testDEK(t, 0x42))
	} else {
		d, err = InitDB(path)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	p, err := d.UpsertProject(Project{Name: "WBS transactions"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := OpenWBSStore(context.Background(), d, WBSBinding{p.ID, "actor-incarnation-1", "lifetime-1"})
	if err != nil {
		t.Fatal(err)
	}
	return d, s
}

func wbsRequest(s *WBSStore, id, expected, title string) WBSWrite {
	return WBSWrite{CopyID: s.copyID, ChartID: "scope", LifetimeID: s.binding.LifetimeID, RequestID: id, ExpectedRevision: expected, Create: expected == "0", Title: title, Data: fmt.Sprintf(`{"schema_version":2,"revision":%q,"root":{"id":"root","title":%q}}`, expected, title)}
}

func mustWBSSave(t *testing.T, s *WBSStore, r WBSWrite) WBSReceipt {
	t.Helper()
	receipt, err := s.Save(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func TestWBSStoreReplayAndReopen(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(strconv.FormatBool(encrypted), func(t *testing.T) {
			d, s := wbsFixture(t, encrypted)
			first := wbsRequest(s, "request-1", "0", "café 東京")
			receipt := mustWBSSave(t, s, first)
			if receipt.Revision != "1" || receipt.Replayed {
				t.Fatalf("create receipt %+v", receipt)
			}
			mustWBSSave(t, s, wbsRequest(s, "request-2", "1", "newer"))
			replay := mustWBSSave(t, s, first)
			if !replay.Replayed || replay.Revision != "1" || replay.AuditID != receipt.AuditID {
				t.Fatalf("replay %+v", replay)
			}
			changed := first
			changed.Title = "metadata changed"
			if _, err := s.Save(context.Background(), changed); !errors.Is(err, ErrWBSRequestReuse) {
				t.Fatalf("changed fingerprint: %v", err)
			}
			s.Close()
			if _, err := s.Save(context.Background(), first); !errors.Is(err, ErrWBSBinding) {
				t.Fatalf("closed store: %v", err)
			}
			if _, err := d.GetProject(); err != nil {
				t.Fatal("Close closed shared DB", err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			var reopened *Database
			var err error
			if encrypted {
				reopened, err = InitEncryptedDB(d.Path, testDEK(t, 0x42))
			} else {
				reopened, err = InitDB(d.Path)
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			next, err := OpenWBSStore(context.Background(), reopened, WBSBinding{s.binding.ProjectID, s.binding.ActorID, "lifetime-2"})
			if err != nil {
				t.Fatal(err)
			}
			first.LifetimeID = next.binding.LifetimeID
			replay = mustWBSSave(t, next, first)
			if replay.Revision != "1" || !replay.Replayed {
				t.Fatalf("reopened replay %+v", replay)
			}
			got, err := next.Read(context.Background(), "scope")
			if err != nil {
				t.Fatal(err)
			}
			a, err := wbs.Parse([]byte(got.Chart.Data))
			if err != nil || a.Revision != "2" || a.Root.Title != "newer" || got.CopyID != s.copyID {
				t.Fatalf("snapshot %+v: %v", a, err)
			}
			other, err := OpenWBSStore(context.Background(), reopened, WBSBinding{s.binding.ProjectID, "actor-incarnation-2", "lifetime-3"})
			if err != nil {
				t.Fatal(err)
			}
			first.LifetimeID = other.binding.LifetimeID
			if _, err := other.Save(context.Background(), first); !errors.Is(err, ErrWBSRequestReuse) {
				t.Fatalf("actor reuse: %v", err)
			}
			report, err := reopened.VerifyAuditChain(s.binding.ProjectID)
			if err != nil || !report.Valid || report.CheckedEvents != 3 {
				t.Fatalf("audit %+v %v", report, err)
			}
			if encrypted {
				requireEncryptedHeader(t, reopened.Path)
				requireCipherIntegrity(t, reopened)
			}
		})
	}
}

func TestWBSStoreConcurrentWriters(t *testing.T) {
	d, s := wbsFixture(t, false)
	mustWBSSave(t, s, wbsRequest(s, "create", "0", "initial"))
	secondDB, err := InitDB(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondDB.Close() })
	other, err := OpenWBSStore(context.Background(), secondDB, WBSBinding{s.binding.ProjectID, s.binding.ActorID, "lifetime-2"})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i, store := range []*WBSStore{s, other} {
		wg.Add(1)
		go func(i int, store *WBSStore) {
			defer wg.Done()
			<-start
			_, err := store.Save(context.Background(), wbsRequest(store, fmt.Sprint("write-", i), "1", fmt.Sprint("winner-", i)))
			results <- err
		}(i, store)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, wbs.ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected writer error %v", err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins %d conflicts %d", wins, conflicts)
	}
	report, err := d.VerifyAuditChain(s.binding.ProjectID)
	if err != nil || !report.Valid || report.CheckedEvents != 3 {
		t.Fatalf("audit %+v %v", report, err)
	}
}

func TestWBSStoreReceiptRetentionAndRollback(t *testing.T) {
	d, s := wbsFixture(t, false)
	old := wbsRequest(s, "request-1", "0", "one")
	mustWBSSave(t, s, old)
	for i := 2; i <= 256; i++ {
		mustWBSSave(t, s, wbsRequest(s, fmt.Sprint("request-", i), strconv.Itoa(i-1), "same"))
	}
	if _, err := d.Conn.Exec(`CREATE TRIGGER fail_prune BEFORE DELETE ON wbs_receipts BEGIN SELECT RAISE(ABORT,'prune failed'); END`); err != nil {
		t.Fatal(err)
	}
	r := wbsRequest(s, "request-257", "256", "last")
	if _, err := s.Save(context.Background(), r); err == nil {
		t.Fatal("prune failure committed")
	}
	snapshot, err := s.Read(context.Background(), "scope")
	if err != nil {
		t.Fatal(err)
	}
	a, err := wbs.Parse([]byte(snapshot.Chart.Data))
	if err != nil || a.Revision != "256" {
		t.Fatalf("rollback %+v %v", a, err)
	}
	if _, err := d.Conn.Exec(`DROP TRIGGER fail_prune`); err != nil {
		t.Fatal(err)
	}
	if !mustWBSSave(t, s, old).Replayed {
		t.Fatal("retained receipt not replayed")
	}
	mustWBSSave(t, s, r)
	if _, err := s.Save(context.Background(), old); !errors.Is(err, wbs.ErrRevisionConflict) {
		t.Fatalf("expired create receipt: %v", err)
	}
	var count int
	if err := d.Conn.QueryRow(`SELECT count(*) FROM wbs_receipts`).Scan(&count); err != nil || count != 256 {
		t.Fatalf("receipt count %d: %v", count, err)
	}
	report, err := d.VerifyAuditChain(s.binding.ProjectID)
	if err != nil || !report.Valid || report.CheckedEvents != 258 {
		t.Fatalf("audit %+v %v", report, err)
	}
}

func TestWBSStoreRefusals(t *testing.T) {
	d, s := wbsFixture(t, false)
	original := wbsRequest(s, "create", "0", "one")
	mustWBSSave(t, s, original)
	for _, tc := range []struct{ name, sql string }{
		{"audit", `CREATE TRIGGER fail BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT,'audit failed'); END`},
		{"receipt", `CREATE TRIGGER fail BEFORE INSERT ON wbs_receipts BEGIN SELECT RAISE(ABORT,'receipt failed'); END`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := d.Conn.Exec(tc.sql); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Save(context.Background(), wbsRequest(s, "fail", "1", "changed")); err == nil {
				t.Fatal("failed write accepted")
			}
			if _, err := d.Conn.Exec(`DROP TRIGGER fail`); err != nil {
				t.Fatal(err)
			}
			snap, err := s.Read(context.Background(), "scope")
			if err != nil || snap.Chart.Title != "one" {
				t.Fatalf("rollback %+v %v", snap, err)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*WBSWrite)
		want   error
	}{
		{"copy", func(r *WBSWrite) { r.CopyID = "foreign" }, ErrWBSBinding},
		{"lifetime", func(r *WBSWrite) { r.LifetimeID = "old" }, ErrWBSBinding},
		{"revision", func(r *WBSWrite) { r.ExpectedRevision = "2" }, wbs.ErrRevision},
		{"association", func(r *WBSWrite) {
			r.Data = `{"schema_version":2,"revision":"1","root":{"id":"root"},"cost_links":[{"node_id":"root","cost_entry_id":"missing"}]}`
		}, ErrWBSAssociations},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := wbsRequest(s, "refused", "1", "one")
			tc.mutate(&r)
			if _, err := s.Save(context.Background(), r); !errors.Is(err, tc.want) {
				t.Fatalf("want %v got %v", tc.want, err)
			}
		})
	}
	for _, payload := range []Chart{{ID: "scope", Kind: "wbs", Data: `{"root":{"id":"legacy"}}`}, {Kind: "wbs_v2", Data: "{}"}, {Kind: "wbs", Data: `{"schema_version":999,"root":{},"unknown":true}`}} {
		if _, err := d.SaveChart(payload); !errors.Is(err, ErrWBSProtected) {
			t.Fatalf("generic write accepted: %v", err)
		}
	}
	if err := d.DeleteChart("scope"); !errors.Is(err, ErrWBSProtected) {
		t.Fatalf("generic delete accepted: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE charts SET kind='wbs',data='{}' WHERE id='scope'`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveChart(Chart{ID: "scope", Kind: "wbs", Data: "{}"}); !errors.Is(err, ErrWBSProtected) {
		t.Fatalf("membership bypass: %v", err)
	}
}

func TestWBSStoreInitializationAndNormalization(t *testing.T) {
	d, s := wbsFixture(t, false)
	r := wbsRequest(s, "first", "0", "same")
	mustWBSSave(t, s, r)
	r.Data = `{"schema_version":2,"revision":"0","root":{"id":"root","title":"same","children":[],"dictionary":{}},"activity_links":[],"cost_links":[]}`
	if !mustWBSSave(t, s, r).Replayed {
		t.Fatal("equivalent optional fields differ")
	}
	if _, err := d.Conn.Exec(`DELETE FROM wbs_copy`); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenWBSStore(context.Background(), d, s.binding); err == nil {
		t.Fatal("missing copy metadata silently recreated")
	}
	if _, err := s.Read(context.Background(), "scope"); err == nil {
		t.Fatal("missing copy metadata accepted")
	}
	if _, err := s.Save(context.Background(), wbsRequest(s, "next", "1", "bad")); err == nil {
		t.Fatal("missing copy metadata accepted write")
	}
}

func TestWBSStoreBoundaryFailures(t *testing.T) {
	d, s := wbsFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Save(ctx, wbsRequest(s, "cancelled", "0", "one")); err == nil {
		t.Fatal("cancelled create committed")
	}
	var count int
	if err := d.Conn.QueryRow(`SELECT count(*) FROM charts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cancel mutation %d %v", count, err)
	}
	r := wbsRequest(s, "first", "0", "one")
	mustWBSSave(t, s, r)
	if _, err := d.Conn.Exec(`UPDATE wbs_records SET revision='2' WHERE chart_id='scope'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(context.Background(), "scope"); !errors.Is(err, ErrWBSBinding) {
		t.Fatalf("inconsistent revision accepted: %v", err)
	}
	if _, err := s.Save(context.Background(), wbsRequest(s, "next", "1", "two")); !errors.Is(err, ErrWBSBinding) {
		t.Fatalf("inconsistent revision write: %v", err)
	}
	max := "18446744073709551615"
	data := fmt.Sprintf(`{"schema_version":2,"revision":%q,"root":{"id":"root"}}`, max)
	if _, err := d.Conn.Exec(`UPDATE charts SET data=? WHERE id='scope'`, data); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`UPDATE wbs_records SET revision=? WHERE chart_id='scope'`, max); err != nil {
		t.Fatal(err)
	}
	request := wbsRequest(s, "overflow", max, "bad")
	if _, err := s.Save(context.Background(), request); !errors.Is(err, wbs.ErrRevisionOverflow) {
		t.Fatalf("overflow: %v", err)
	}
	if err := d.Conn.QueryRow(`SELECT count(*) FROM wbs_receipts`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("refusal receipts %d %v", count, err)
	}
}

func TestWBSStoreConcurrentInitialization(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "init"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := InitDB(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	start := make(chan struct{})
	results := make(chan *WBSStore, 2)
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for _, database := range []*Database{d, other} {
		wg.Add(1)
		go func(database *Database) {
			defer wg.Done()
			<-start
			s, err := OpenWBSStore(context.Background(), database, WBSBinding{p.ID, "actor", "life"})
			results <- s
			failures <- err
		}(database)
	}
	close(start)
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	copyID := ""
	for store := range results {
		if copyID != "" && store.copyID != copyID {
			t.Fatal("initializers created different copies")
		}
		copyID = store.copyID
	}
}

func TestWBSStoreCommitFailureAndPartialSchema(t *testing.T) {
	d, s := wbsFixture(t, false)
	_, err := d.Conn.Exec(`CREATE TABLE receipt_parent(id TEXT PRIMARY KEY);
	CREATE TABLE receipt_child(id TEXT REFERENCES receipt_parent(id) DEFERRABLE INITIALLY DEFERRED);
	CREATE TRIGGER fail_commit AFTER INSERT ON wbs_receipts BEGIN INSERT INTO receipt_child VALUES('missing'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Save(context.Background(), wbsRequest(s, "uncertain", "0", "one")); !errors.Is(err, ErrWBSIndeterminate) {
		t.Fatalf("commit failure: %v", err)
	}
	if _, err = d.Conn.Exec(`DROP TRIGGER fail_commit`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = d.Conn.QueryRow(`SELECT count(*) FROM charts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("commit rollback %d %v", count, err)
	}
	mustWBSSave(t, s, wbsRequest(s, "first", "0", "one"))
	if _, err = d.Conn.Exec(`UPDATE wbs_records SET revision='2'`); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenWBSStore(context.Background(), d, s.binding); !errors.Is(err, ErrWBSBinding) {
		t.Fatalf("corrupt initialization: %v", err)
	}
	partial := newBackupTestDB(t)
	p, err := partial.UpsertProject(Project{Name: "partial"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = partial.Conn.Exec(`CREATE TABLE wbs_copy(id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenWBSStore(context.Background(), partial, WBSBinding{p.ID, "actor", "life"}); !errors.Is(err, ErrWBSBinding) {
		t.Fatalf("partial schema: %v", err)
	}
	if err = partial.Conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='wbs_records'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial schema mutated %d %v", count, err)
	}
}

func TestWBSStoreEncodingAndSchemaCounterexamples(t *testing.T) {
	d, s := wbsFixture(t, false)
	a := wbs.Aggregate{SchemaVersion: 2, Revision: "0", Root: &wbs.Node{ID: "root"}}
	for i := 0; i < 25; i++ {
		a.Root.Children = append(a.Root.Children, &wbs.Node{ID: fmt.Sprint(i), Title: strings.Repeat("&", 60_000)})
	}
	raw, err := jsonv2.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	r := wbsRequest(s, "escape", "0", "escaped")
	r.Data = string(raw)
	mustWBSSave(t, s, r)
	if _, err = s.Read(context.Background(), "scope"); err != nil {
		t.Fatalf("successful write unreadable: %v", err)
	}
	a.Revision = "9"
	a.Root.Children = nil
	for i := 0; i < 127; i++ {
		a.Root.Children = append(a.Root.Children, &wbs.Node{ID: fmt.Sprint(i), Title: strings.Repeat("x", wbs.MaxTextBytes)})
	}
	// Nonempty note adds its field syntax as well as text bytes.
	a.Root.Note = "x"
	withNote, err := jsonv2.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	padding := wbs.MaxPayloadBytes - len(withNote) + 1
	if padding < 1 || padding > wbs.MaxTextBytes {
		t.Fatalf("unexpected padding %d", padding)
	}
	a.Root.Note = strings.Repeat("x", padding)
	raw, err = jsonv2.Marshal(a)
	if err != nil || len(raw) != wbs.MaxPayloadBytes {
		t.Fatalf("boundary len%d %v", len(raw), err)
	}
	if _, err = wbs.Parse(raw); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Conn.Exec(`UPDATE charts SET data=? WHERE id='scope'`, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Conn.Exec(`UPDATE wbs_records SET revision='9' WHERE chart_id='scope'`); err != nil {
		t.Fatal(err)
	}
	r = wbsRequest(s, "digit-growth", "9", "bounded")
	r.Data = string(raw)
	if _, err = s.Save(context.Background(), r); !errors.Is(err, wbs.ErrLimit) {
		t.Fatalf("revision digit expansion: %v", err)
	}
	if _, err = s.Read(context.Background(), "scope"); err != nil {
		t.Fatalf("refused save damaged prior payload: %v", err)
	}
	if _, err = d.Conn.Exec(`DROP TABLE wbs_receipts; CREATE TABLE wbs_receipts(chart_id TEXT NOT NULL,request_id TEXT NOT NULL,revision TEXT NOT NULL,fingerprint TEXT NOT NULL,audit_id TEXT NOT NULL,PRIMARY KEY(chart_id,request_id))`); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenWBSStore(context.Background(), d, s.binding); !errors.Is(err, ErrWBSBinding) {
		t.Fatalf("incompatible schema accepted: %v", err)
	}
}
