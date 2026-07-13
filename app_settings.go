// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pmforge/internal/agile"
	"pmforge/internal/db"
	"pmforge/internal/documents"
	"time"
)

// =========================================================
// V1 settings (kept for compat with the V1 Settings panel)
// =========================================================

func (a *App) GetSettings() (db.UserSettings, error) {
	d := a.requireDB()
	if d == nil {
		return db.UserSettings{}, errors.New("no project open")
	}
	return d.GetSettings()
}

func (a *App) SaveSettings(s db.UserSettings) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.SaveSettings(s)
}

func (a *App) ResetProjectSettings() (db.UserSettings, error) {
	d := a.requireDB()
	if d == nil {
		return db.UserSettings{}, errors.New("no project open")
	}
	defaults := db.DefaultUserSettings()
	if err := d.SaveSettings(defaults); err != nil {
		return db.UserSettings{}, err
	}
	agile.PackEnabled.Store(defaults.AgileEnabled)
	documents.UseFont(nil, "")
	return defaults, nil
}

type auditVerificationReportFile struct {
	ProjectID            string `json:"project_id"`
	GeneratedAtUTC       string `json:"generated_at_utc"`
	CheckedEvents        int    `json:"checked_events"`
	Valid                bool   `json:"valid"`
	FirstInvalidSequence int64  `json:"first_invalid_sequence,omitempty"`
	FirstInvalidEventID  string `json:"first_invalid_event_id,omitempty"`
	FirstInvalidReason   string `json:"first_invalid_reason,omitempty"`
	TerminalEventHash    string `json:"terminal_event_hash,omitempty"`
}

type auditRepairEvidenceEventFile struct {
	ID                    string `json:"id"`
	ProjectID             string `json:"project_id"`
	SequenceNumber        int64  `json:"sequence_number"`
	PreviousEventHash     string `json:"previous_event_hash"`
	EventHash             string `json:"event_hash"`
	EventType             string `json:"event_type"`
	EntityType            string `json:"entity_type"`
	EntityID              string `json:"entity_id"`
	BeforeCanonicalJSON   string `json:"before_canonical_json"`
	AfterCanonicalJSON    string `json:"after_canonical_json"`
	UserID                string `json:"user_id"`
	SessionID             string `json:"session_id"`
	TimestampUTC          string `json:"timestamp_utc"`
	SignatureStatus       string `json:"signature_status"`
	SignatureBlobOptional string `json:"signature_blob_optional,omitempty"`
	SignatureBlobLength   int    `json:"signature_blob_length"`
}

type auditRepairEvidenceFile struct {
	ProjectID      string                         `json:"project_id"`
	GeneratedAtUTC string                         `json:"generated_at_utc"`
	Verification   auditVerificationReportFile    `json:"verification"`
	Events         []auditRepairEvidenceEventFile `json:"events"`
}

// ExportAuditVerificationReport writes a private JSON artifact describing
// the current project's tamper-evident audit-chain verification result.
func (a *App) ExportAuditVerificationReport() (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	verification, err := d.VerifyAuditChain(proj.ID)
	if err != nil {
		return "", err
	}
	report := auditVerificationReportFile{
		ProjectID:            verification.ProjectID,
		GeneratedAtUTC:       time.Now().UTC().Format(time.RFC3339Nano),
		CheckedEvents:        verification.CheckedEvents,
		Valid:                verification.Valid,
		FirstInvalidSequence: verification.FirstInvalidSequence,
		FirstInvalidEventID:  verification.FirstInvalidEventID,
		FirstInvalidReason:   verification.FirstInvalidReason,
		TerminalEventHash:    verification.TerminalEventHash,
	}
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-audit-verification-%s.json",
		sanitizeFilename(proj.Name), time.Now().UTC().Format("20060102-150405")))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// ExportAuditRepairEvidence preserves the current raw audit_events rows
// beside their verification result. It intentionally does not mutate or
// repair the project database.
func (a *App) ExportAuditRepairEvidence() (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	verification, err := d.VerifyAuditChain(proj.ID)
	if err != nil {
		return "", err
	}
	events, err := d.ListAuditEvents(proj.ID)
	if err != nil {
		return "", err
	}
	generatedAt := time.Now().UTC()
	evidence := auditRepairEvidenceFile{
		ProjectID:      proj.ID,
		GeneratedAtUTC: generatedAt.Format(time.RFC3339Nano),
		Verification: auditVerificationReportFile{
			ProjectID:            verification.ProjectID,
			GeneratedAtUTC:       generatedAt.Format(time.RFC3339Nano),
			CheckedEvents:        verification.CheckedEvents,
			Valid:                verification.Valid,
			FirstInvalidSequence: verification.FirstInvalidSequence,
			FirstInvalidEventID:  verification.FirstInvalidEventID,
			FirstInvalidReason:   verification.FirstInvalidReason,
			TerminalEventHash:    verification.TerminalEventHash,
		},
		Events: make([]auditRepairEvidenceEventFile, 0, len(events)),
	}
	for _, event := range events {
		evidence.Events = append(evidence.Events, auditRepairEvidenceEventFile{
			ID:                    event.ID,
			ProjectID:             event.ProjectID,
			SequenceNumber:        event.SequenceNumber,
			PreviousEventHash:     event.PreviousEventHash,
			EventHash:             event.EventHash,
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
			SignatureBlobLength:   len(event.SignatureBlobOptional),
		})
	}
	bytes, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-audit-repair-evidence-%s.json",
		sanitizeFilename(proj.Name), generatedAt.Format("20060102-150405")))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

func (a *App) SecureArchive(projectPath string) (string, error) {
	// Confine to the user's own projects folder before archiving, so the
	// backup is always written next to a project the caller actually owns.
	clean, _, err := a.projectPathFor(projectPath)
	if err != nil {
		return "", err
	}
	a.mu.RLock()
	svc := a.adminSvc
	a.mu.RUnlock()
	if svc == nil {
		return "", errors.New("no project open")
	}
	return svc.SecureArchive(clean)
}
