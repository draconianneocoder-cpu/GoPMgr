// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gopmgr/internal/wbs"
)

const WBSReceiptLimit = 256

const wbsStorageSchema = `CREATE TABLE wbs_copy(id INTEGER PRIMARY KEY CHECK(id=1),copy_id TEXT NOT NULL,project_id TEXT NOT NULL);
CREATE TABLE wbs_records(chart_id TEXT PRIMARY KEY REFERENCES charts(id),revision TEXT NOT NULL);
CREATE TABLE wbs_receipts(chart_id TEXT NOT NULL REFERENCES wbs_records(chart_id),request_id TEXT NOT NULL,fingerprint TEXT NOT NULL,revision TEXT NOT NULL,audit_id TEXT NOT NULL,PRIMARY KEY(chart_id,request_id));`

var (
	ErrWBSBinding       = errors.New("wbs store: invalid binding")
	ErrWBSRequestReuse  = errors.New("wbs store: request identity reused")
	ErrWBSAssociations  = errors.New("wbs store: associations require source incarnation support")
	ErrWBSProtected     = errors.New("wbs store: dedicated writer required")
	ErrWBSIndeterminate = errors.New("wbs store: commit outcome requires reconciliation")
)

// WBSBinding must come from a trusted session owner, not a wire request or
// mutable project metadata. This storage foundation does not authenticate users.
type WBSBinding struct{ ProjectID, ActorID, LifetimeID string }

// WBSStore pins one trusted binding until Close. It does not own the shared
// database pool. No application caller or automatic migration activates it.
type WBSStore struct {
	mu      sync.RWMutex
	db      *Database
	binding WBSBinding
	copyID  string
	closed  bool
}

type WBSWrite struct {
	CopyID           string
	ChartID          string
	LifetimeID       string
	RequestID        string
	ExpectedRevision string // "0" for explicit creation.
	Create           bool
	Title            string
	Data             string // Immutable submitted JSON; its revision equals ExpectedRevision.
}

type WBSReceipt struct {
	RequestID, Revision, AuditID string
	Replayed                     bool
}

type WBSSnapshot struct {
	CopyID, LifetimeID string
	Chart              Chart
}

// OpenWBSStore explicitly initializes the dormant storage schema atomically.
// An existing copy identity survives reopen; partial initialization is refused.
func OpenWBSStore(ctx context.Context, d *Database, binding WBSBinding) (*WBSStore, error) {
	for _, v := range []string{binding.ProjectID, binding.ActorID, binding.LifetimeID} {
		if !wbsIdentifier(v) {
			return nil, ErrWBSBinding
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := d.Conn.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := tx.ExecContext(ctx, `UPDATE project SET id=id WHERE id=?`, binding.ProjectID)
	if err != nil {
		return nil, err
	}
	matched, err := locked.RowsAffected()
	if err != nil {
		return nil, err
	}
	if matched != 1 {
		return nil, ErrWBSBinding
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('wbs_copy','wbs_records','wbs_receipts')`).Scan(&count); err != nil {
		return nil, err
	}
	if count != 0 && count != 3 {
		return nil, ErrWBSBinding
	}
	if count == 0 {
		var protected int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM charts WHERE kind='wbs_v2'`).Scan(&protected); err != nil {
			return nil, err
		}
		if protected != 0 {
			return nil, ErrWBSBinding
		}
		_, err = tx.ExecContext(ctx, wbsStorageSchema)
		if err != nil {
			return nil, err
		}
		var entropy [16]byte
		if _, err = io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO wbs_copy(id,copy_id,project_id) VALUES(1,?,?)`, hex.EncodeToString(entropy[:]), binding.ProjectID); err != nil {
			return nil, err
		}
	}
	for _, definition := range strings.Split(wbsStorageSchema, ";") {
		definition = strings.TrimSpace(definition)
		if definition == "" {
			continue
		}
		name := strings.SplitN(strings.TrimPrefix(definition, "CREATE TABLE "), "(", 2)[0]
		var actual string
		if err = tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&actual); err != nil {
			return nil, err
		}
		if actual != definition {
			return nil, fmt.Errorf("%w: incompatible %s schema", ErrWBSBinding, name)
		}
	}
	var copyID, projectID, actualProject string
	if err = tx.QueryRowContext(ctx, `SELECT copy_id,project_id FROM wbs_copy WHERE id=1`).Scan(&copyID, &projectID); err != nil {
		return nil, fmt.Errorf("wbs copy metadata: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM project LIMIT 1`).Scan(&actualProject); err != nil {
		return nil, err
	}
	if !wbsIdentifier(copyID) || projectID != binding.ProjectID || actualProject != binding.ProjectID {
		return nil, ErrWBSBinding
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM charts WHERE kind='wbs_v2' UNION SELECT chart_id FROM wbs_records`)
	if err != nil {
		return nil, err
	}
	var chartIDs []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		chartIDs = append(chartIDs, id)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for _, id := range chartIDs {
		if _, _, err = readWBSRecord(ctx, tx, id, binding.ProjectID); err != nil {
			return nil, err
		}
	}
	if err = commitWBS(tx, conn); err != nil {
		return nil, err
	}
	return &WBSStore{db: d, binding: binding, copyID: copyID}, nil
}

func (s *WBSStore) Close() { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true }

func (s *WBSStore) Read(ctx context.Context, chartID string) (WBSSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || !wbsIdentifier(chartID) {
		return WBSSnapshot{}, ErrWBSBinding
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.Conn.BeginTx(ctx, nil)
	if err != nil {
		return WBSSnapshot{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = s.checkCopy(ctx, tx, false); err != nil {
		return WBSSnapshot{}, err
	}
	c, _, err := readWBSRecord(ctx, tx, chartID, s.binding.ProjectID)
	if err != nil {
		return WBSSnapshot{}, err
	}
	return WBSSnapshot{CopyID: s.copyID, LifetimeID: s.binding.LifetimeID, Chart: c}, nil
}

func (s *WBSStore) Save(ctx context.Context, r WBSWrite) (WBSReceipt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || r.CopyID != s.copyID || r.LifetimeID != s.binding.LifetimeID || !wbsIdentifier(r.ChartID) || !wbsIdentifier(r.RequestID) || !utf8.ValidString(r.Title) || len(r.Title) > wbs.MaxTextBytes {
		return WBSReceipt{}, ErrWBSBinding
	}
	a, err := wbs.Parse([]byte(r.Data))
	if err != nil {
		return WBSReceipt{}, err
	}
	if a.Revision != r.ExpectedRevision || (r.Create && r.ExpectedRevision != "0") {
		return WBSReceipt{}, wbs.ErrRevision
	}
	// Freeze and normalize semantic optional values before fingerprinting.
	normalizeWBSNode(a.Root)
	data, err := jsonv2.Marshal(a)
	if err != nil {
		return WBSReceipt{}, err
	}
	fingerprintBytes, err := json.Marshal(struct {
		Version                                            int
		Copy, Project, Chart, Actor, Expected, Title, Data string
		Create                                             bool
	}{1, s.copyID, s.binding.ProjectID, r.ChartID, s.binding.ActorID, r.ExpectedRevision, r.Title, string(data), r.Create})
	if err != nil {
		return WBSReceipt{}, err
	}
	sum := sha256.Sum256(fingerprintBytes)
	fingerprint := hex.EncodeToString(sum[:])
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := s.db.Conn.Conn(ctx)
	if err != nil {
		return WBSReceipt{}, err
	}
	defer func() { _ = conn.Close() }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return WBSReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// The first database statement reserves the SQLite writer before any read
	// snapshot, avoiding a deferred-read-to-write upgrade under contention.
	if err = s.checkCopy(ctx, tx, true); err != nil {
		return WBSReceipt{}, err
	}
	var retained string
	receipt := WBSReceipt{RequestID: r.RequestID}
	err = tx.QueryRowContext(ctx, `SELECT fingerprint,revision,audit_id FROM wbs_receipts WHERE chart_id=? AND request_id=?`, r.ChartID, r.RequestID).Scan(&retained, &receipt.Revision, &receipt.AuditID)
	if err == nil {
		if retained != fingerprint {
			return WBSReceipt{}, ErrWBSRequestReuse
		}
		receipt.Replayed = true
		return receipt, nil
	}
	if err != sql.ErrNoRows {
		return WBSReceipt{}, err
	}
	if len(a.ActivityLinks) != 0 || len(a.CostLinks) != 0 {
		return WBSReceipt{}, ErrWBSAssociations
	}
	before := Chart{}
	current := "0"
	if r.Create {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM charts WHERE id=?`, r.ChartID).Scan(&n); err != nil {
			return WBSReceipt{}, err
		}
		if n != 0 {
			return WBSReceipt{}, wbs.ErrRevisionConflict
		}
	} else {
		before, current, err = readWBSRecord(ctx, tx, r.ChartID, s.binding.ProjectID)
		if err != nil {
			return WBSReceipt{}, err
		}
	}
	if current != r.ExpectedRevision {
		return WBSReceipt{}, wbs.ErrRevisionConflict
	}
	next, err := wbs.NextRevision(current)
	if err != nil {
		return WBSReceipt{}, err
	}
	a.Revision = next
	data, err = jsonv2.Marshal(a)
	if err != nil {
		return WBSReceipt{}, err
	}
	if _, err = wbs.Parse(data); err != nil {
		return WBSReceipt{}, err
	}
	now := captureTimestamp()
	if r.Create {
		_, err = tx.ExecContext(ctx, `INSERT INTO charts(id,project_id,kind,title,data,config,template_id,created_at,updated_at,created_at_unixnano,updated_at_unixnano) VALUES(?,?,'wbs_v2',?,?,'{}','',?,?,?,?)`, r.ChartID, s.binding.ProjectID, r.Title, string(data), now.text, now.text, now.unixNano, now.unixNano)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO wbs_records(chart_id,revision) VALUES(?,?)`, r.ChartID, next)
		}
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE charts SET title=?,data=?,updated_at=?,updated_at_unixnano=? WHERE id=? AND project_id=? AND kind='wbs_v2'`, r.Title, string(data), now.text, now.unixNano, r.ChartID, s.binding.ProjectID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE wbs_records SET revision=? WHERE chart_id=? AND revision=?`, next, r.ChartID, current)
		}
	}
	if err != nil {
		return WBSReceipt{}, err
	}
	after, err := getChartTx(tx, r.ChartID)
	if err != nil {
		return WBSReceipt{}, err
	}
	beforeJSON := ""
	eventType := "chart.create"
	if !r.Create {
		beforeJSON, err = chartAuditJSON(before)
		if err != nil {
			return WBSReceipt{}, err
		}
		eventType = "chart.update"
	}
	afterJSON, err := chartAuditJSON(after)
	if err != nil {
		return WBSReceipt{}, err
	}
	event, err := appendAuditEventTx(tx, AuditEventInput{ProjectID: s.binding.ProjectID, EventType: eventType, EntityType: "chart", EntityID: r.ChartID, UserID: s.binding.ActorID, SessionID: s.binding.LifetimeID, BeforeJSON: beforeJSON, AfterJSON: afterJSON})
	if err != nil {
		return WBSReceipt{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO wbs_receipts(chart_id,request_id,fingerprint,revision,audit_id) VALUES(?,?,?,?,?)`, r.ChartID, r.RequestID, fingerprint, next, event.ID)
	if err != nil {
		return WBSReceipt{}, err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM wbs_receipts WHERE chart_id=? AND request_id NOT IN (SELECT request_id FROM wbs_receipts WHERE chart_id=? ORDER BY length(revision) DESC,revision COLLATE BINARY DESC LIMIT ?)`, r.ChartID, r.ChartID, WBSReceiptLimit)
	if err != nil {
		return WBSReceipt{}, err
	}
	if err = commitWBS(tx, conn); err != nil {
		return WBSReceipt{}, err
	}
	return WBSReceipt{RequestID: r.RequestID, Revision: next, AuditID: event.ID}, nil
}

// Retain and discard the dedicated connection on an indeterminate commit,
// independently of the driver's transaction-cleanup defense. A commit error
// must never be labeled a safe retry.
func commitWBS(tx *sql.Tx, conn *sql.Conn) error {
	err := tx.Commit()
	if err == nil {
		return nil
	}
	discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
	if discardErr != nil && !errors.Is(discardErr, driver.ErrBadConn) && !errors.Is(discardErr, sql.ErrConnDone) {
		return errors.Join(ErrWBSIndeterminate, err, discardErr)
	}
	return errors.Join(ErrWBSIndeterminate, err)
}

func (s *WBSStore) checkCopy(ctx context.Context, tx *sql.Tx, write bool) error {
	if write {
		result, err := tx.ExecContext(ctx, `UPDATE wbs_copy SET copy_id=copy_id WHERE id=1 AND copy_id=? AND project_id=?`, s.copyID, s.binding.ProjectID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrWBSBinding
		}
	}
	var copyID, projectID string
	if err := tx.QueryRowContext(ctx, `SELECT copy_id,project_id FROM wbs_copy WHERE id=1`).Scan(&copyID, &projectID); err != nil {
		return err
	}
	if copyID != s.copyID || projectID != s.binding.ProjectID {
		return ErrWBSBinding
	}
	var actualProject string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM project LIMIT 1`).Scan(&actualProject); err != nil {
		return err
	}
	if actualProject != projectID {
		return ErrWBSBinding
	}
	return nil
}

func readWBSRecord(ctx context.Context, tx *sql.Tx, id, projectID string) (Chart, string, error) {
	c, err := getChartTx(tx, id)
	if err != nil {
		return Chart{}, "", err
	}
	if c.ProjectID != projectID || c.Kind != "wbs_v2" {
		return Chart{}, "", ErrWBSBinding
	}
	var revision string
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM wbs_records WHERE chart_id=?`, id).Scan(&revision); err != nil {
		return Chart{}, "", err
	}
	a, err := wbs.Parse([]byte(c.Data))
	if err != nil {
		return Chart{}, "", err
	}
	if a.Revision != revision {
		return Chart{}, "", ErrWBSBinding
	}
	return c, revision, nil
}

func normalizeWBSNode(n *wbs.Node) {
	if n.Dictionary != nil && *n.Dictionary == (wbs.Dictionary{}) {
		n.Dictionary = nil
	}
	for _, child := range n.Children {
		normalizeWBSNode(child)
	}
}

func wbsIdentifier(v string) bool {
	return utf8.ValidString(v) && len(v) <= wbs.MaxIdentifierBytes && strings.TrimSpace(v) != ""
}

func protectedWBSPayload(kind, data string) bool {
	if kind == "wbs_v2" {
		return true
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal([]byte(data), &envelope) != nil {
		return false
	}
	_, root := envelope["root"]
	_, schema := envelope["schema_version"]
	return root && schema
}

func protectedWBSChartTx(tx *sql.Tx, c Chart) (bool, error) {
	if protectedWBSPayload(c.Kind, c.Data) {
		return true, nil
	}
	var table int
	if err := tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='wbs_records'`).Scan(&table); err != nil {
		return false, err
	}
	if table == 0 {
		return false, nil
	}
	var n int
	err := tx.QueryRow(`SELECT count(*) FROM wbs_records WHERE chart_id=?`, c.ID).Scan(&n)
	return n != 0, err
}
