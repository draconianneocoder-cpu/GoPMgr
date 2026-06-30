// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"pmforge/internal/debug"
	"pmforge/internal/exportsafe"
)

// LogAction writes a row to audit_log using SQLite's strftime() default
// for the timestamp (millisecond-precision UTC). This is the function
// the signature-event and admin workflows call.
func (db *Database) LogAction(actor, action, targetID, details string) error {
	_, err := db.Conn.Exec(
		`INSERT INTO audit_log(actor, action, target_id, details) VALUES (?, ?, ?, ?)`,
		actor, action, targetID, details,
	)
	if err != nil {
		// Re-wrap so callers can attribute audit-log failures via
		// debug.Report(err). Returning a plain error is also acceptable.
		return debug.Wrap(err, "AUDIT_LOG_WRITE_FAILED").ToError()
	}
	return nil
}

// AuditEventInput is the caller-supplied content for one
// tamper-evident audit event. JSON fields may be empty, in which case
// they canonicalise to JSON null.
type AuditEventInput struct {
	ProjectID             string
	EventType             string
	EntityType            string
	EntityID              string
	BeforeJSON            string
	AfterJSON             string
	UserID                string
	SessionID             string
	SignatureStatus       string
	SignatureBlobOptional string
}

// AuditEvent is one persisted hash-chain audit event.
type AuditEvent struct {
	ID                    string
	ProjectID             string
	SequenceNumber        int64
	PreviousEventHash     string
	EventHash             string
	EventType             string
	EntityType            string
	EntityID              string
	BeforeCanonicalJSON   string
	AfterCanonicalJSON    string
	UserID                string
	SessionID             string
	TimestampUTC          string
	SignatureStatus       string
	SignatureBlobOptional string
}

// AuditVerification reports whether a project's audit_events hash
// chain is intact.
type AuditVerification struct {
	ProjectID            string
	CheckedEvents        int
	Valid                bool
	FirstInvalidSequence int64
	FirstInvalidEventID  string
	FirstInvalidReason   string
	TerminalEventHash    string
}

// AppendAuditEvent canonicalises and appends one project audit event,
// chaining it to the previous event hash for that project.
func (db *Database) AppendAuditEvent(in AuditEventInput) (AuditEvent, error) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return AuditEvent{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	event, err := appendAuditEventTx(tx, in)
	if err != nil {
		return AuditEvent{}, err
	}
	err = tx.Commit()
	if err != nil {
		return AuditEvent{}, err
	}
	return event, nil
}

func appendAuditEventTx(tx *sql.Tx, in AuditEventInput) (AuditEvent, error) {
	if in.ProjectID == "" {
		return AuditEvent{}, fmt.Errorf("audit event: project_id is required")
	}
	if in.EventType == "" {
		return AuditEvent{}, fmt.Errorf("audit event: event_type is required")
	}
	if in.EntityType == "" {
		return AuditEvent{}, fmt.Errorf("audit event: entity_type is required")
	}
	before, err := canonicalJSON(in.BeforeJSON)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("canonical before json: %w", err)
	}
	after, err := canonicalJSON(in.AfterJSON)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("canonical after json: %w", err)
	}
	if in.SignatureStatus == "" {
		in.SignatureStatus = "unsigned"
	}
	id, err := newID("audit")
	if err != nil {
		return AuditEvent{}, err
	}

	var previousHash string
	var previousSeq sql.NullInt64
	row := tx.QueryRow(
		`SELECT sequence_number, event_hash
		 FROM audit_events
		 WHERE project_id = ?
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		in.ProjectID,
	)
	switch scanErr := row.Scan(&previousSeq, &previousHash); scanErr {
	case nil:
	case sql.ErrNoRows:
	default:
		return AuditEvent{}, scanErr
	}
	seq := int64(1)
	if previousSeq.Valid {
		seq = previousSeq.Int64 + 1
	}

	event := AuditEvent{
		ID:                    id,
		ProjectID:             in.ProjectID,
		SequenceNumber:        seq,
		PreviousEventHash:     previousHash,
		EventType:             in.EventType,
		EntityType:            in.EntityType,
		EntityID:              in.EntityID,
		BeforeCanonicalJSON:   before,
		AfterCanonicalJSON:    after,
		UserID:                in.UserID,
		SessionID:             in.SessionID,
		TimestampUTC:          time.Now().UTC().Format(time.RFC3339Nano),
		SignatureStatus:       in.SignatureStatus,
		SignatureBlobOptional: in.SignatureBlobOptional,
	}
	event.EventHash = eventHash(event)

	_, err = tx.Exec(
		`INSERT INTO audit_events(
			id, project_id, sequence_number, previous_event_hash, event_hash,
			event_type, entity_type, entity_id, before_canonical_json,
			after_canonical_json, user_id, session_id, timestamp_utc,
			signature_status, signature_blob_optional
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.ProjectID, event.SequenceNumber, event.PreviousEventHash,
		event.EventHash, event.EventType, event.EntityType, event.EntityID,
		event.BeforeCanonicalJSON, event.AfterCanonicalJSON, event.UserID,
		event.SessionID, event.TimestampUTC, event.SignatureStatus,
		event.SignatureBlobOptional,
	)
	if err != nil {
		return AuditEvent{}, err
	}
	return event, nil
}

// VerifyAuditChain recomputes every event hash for a project and
// checks the previous-hash links and sequence continuity.
func (db *Database) VerifyAuditChain(projectID string) (AuditVerification, error) {
	report := AuditVerification{ProjectID: projectID, Valid: true}
	rows, err := db.Conn.Query(
		`SELECT id, project_id, sequence_number, previous_event_hash, event_hash,
		        event_type, entity_type, entity_id, before_canonical_json,
		        after_canonical_json, user_id, session_id, timestamp_utc,
		        signature_status, signature_blob_optional
		 FROM audit_events
		 WHERE project_id = ?
		 ORDER BY sequence_number ASC`,
		projectID,
	)
	if err != nil {
		return report, err
	}
	defer func() { _ = rows.Close() }()

	var previousHash string
	expectedSeq := int64(1)
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(
			&event.ID, &event.ProjectID, &event.SequenceNumber,
			&event.PreviousEventHash, &event.EventHash, &event.EventType,
			&event.EntityType, &event.EntityID, &event.BeforeCanonicalJSON,
			&event.AfterCanonicalJSON, &event.UserID, &event.SessionID,
			&event.TimestampUTC, &event.SignatureStatus,
			&event.SignatureBlobOptional,
		); err != nil {
			return report, err
		}
		report.CheckedEvents++
		if event.SequenceNumber != expectedSeq {
			return invalidAudit(report, event, "sequence gap"), nil
		}
		if event.PreviousEventHash != previousHash {
			return invalidAudit(report, event, "previous hash mismatch"), nil
		}
		if got := eventHash(event); got != event.EventHash {
			return invalidAudit(report, event, "event hash mismatch"), nil
		}
		previousHash = event.EventHash
		report.TerminalEventHash = previousHash
		expectedSeq++
	}
	if err := rows.Err(); err != nil {
		return report, err
	}
	return report, nil
}

// ListAuditEvents returns the raw hash-chain events for a project in
// sequence order. It is used for compliance evidence exports and must
// not attempt to repair or normalize damaged rows.
func (db *Database) ListAuditEvents(projectID string) ([]AuditEvent, error) {
	rows, err := db.Conn.Query(
		`SELECT id, project_id, sequence_number, previous_event_hash, event_hash,
		        event_type, entity_type, entity_id, before_canonical_json,
		        after_canonical_json, user_id, session_id, timestamp_utc,
		        signature_status, signature_blob_optional
		 FROM audit_events
		 WHERE project_id = ?
		 ORDER BY sequence_number ASC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(
			&event.ID, &event.ProjectID, &event.SequenceNumber,
			&event.PreviousEventHash, &event.EventHash, &event.EventType,
			&event.EntityType, &event.EntityID, &event.BeforeCanonicalJSON,
			&event.AfterCanonicalJSON, &event.UserID, &event.SessionID,
			&event.TimestampUTC, &event.SignatureStatus,
			&event.SignatureBlobOptional,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func invalidAudit(report AuditVerification, event AuditEvent, reason string) AuditVerification {
	report.Valid = false
	report.FirstInvalidSequence = event.SequenceNumber
	report.FirstInvalidEventID = event.ID
	report.FirstInvalidReason = reason
	return report
}

func canonicalJSON(raw string) (string, error) {
	if raw == "" {
		return "null", nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", err
	}
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func eventHash(event AuditEvent) string {
	payload := struct {
		ProjectID             string `json:"project_id"`
		SequenceNumber        int64  `json:"sequence_number"`
		PreviousEventHash     string `json:"previous_event_hash"`
		EventType             string `json:"event_type"`
		EntityType            string `json:"entity_type"`
		EntityID              string `json:"entity_id"`
		BeforeCanonicalJSON   string `json:"before_canonical_json"`
		AfterCanonicalJSON    string `json:"after_canonical_json"`
		UserID                string `json:"user_id"`
		SessionID             string `json:"session_id"`
		TimestampUTC          string `json:"timestamp_utc"`
		SignatureStatus       string `json:"signature_status"`
		SignatureBlobOptional string `json:"signature_blob_optional"`
	}{
		ProjectID:             event.ProjectID,
		SequenceNumber:        event.SequenceNumber,
		PreviousEventHash:     event.PreviousEventHash,
		EventType:             event.EventType,
		EntityType:            event.EntityType,
		EntityID:              event.EntityID,
		BeforeCanonicalJSON:   event.BeforeCanonicalJSON,
		AfterCanonicalJSON:    event.AfterCanonicalJSON,
		UserID:                event.UserID,
		SessionID:             event.SessionID,
		TimestampUTC:          event.TimestampUTC,
		SignatureStatus:       event.SignatureStatus,
		SignatureBlobOptional: event.SignatureBlobOptional,
	}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(append([]byte(event.PreviousEventHash), canonical...))
	return hex.EncodeToString(sum[:])
}

// ExportAuditCSV dumps the audit_log table to a CSV file at the given
// path. Used by the `--export-audit` CLI flag.
func (db *Database) ExportAuditCSV(path string) (err error) {
	rows, err := db.Conn.Query(
		`SELECT id, ts, actor, action, target_id, details FROM audit_log ORDER BY id ASC`,
	)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := rows.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- CLI/user-selected audit export destination.
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	w := csv.NewWriter(f)

	if err := w.Write([]string{"id", "ts", "actor", "action", "target_id", "details"}); err != nil {
		return err
	}

	for rows.Next() {
		var (
			id                                 int64
			ts, actor, action, target, details string
		)
		if err := rows.Scan(&id, &ts, &actor, &action, &target, &details); err != nil {
			return err
		}
		// actor/action/target/details carry user-controlled text (usernames,
		// project and document names), so neutralize them against formula
		// injection (CWE-1236). id and ts are app-generated.
		if err := w.Write([]string{
			fmt.Sprintf("%d", id), ts,
			exportsafe.Cell(actor), exportsafe.Cell(action),
			exportsafe.Cell(target), exportsafe.Cell(details),
		}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return nil
}
