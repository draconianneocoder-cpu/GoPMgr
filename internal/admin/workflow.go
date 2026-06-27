// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package admin implements the Administrative Pack workflows: document
// control, secure archiving, and signature-event logging.
package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"pmforge/internal/db"
	"pmforge/internal/debug"
)

// Service is the entry point for admin-pack workflows. Construct one
// per project using NewService.
type Service struct {
	DB *db.Database
}

// NewService binds an admin service to a database handle.
func NewService(d *db.Database) *Service { return &Service{DB: d} }

// SecureArchive creates a .pmba bundle next to the project file. The
// active signing certificate (if any) is included. Returns the path of
// the archive written.
func (s *Service) SecureArchive(projectPath string) (string, error) {
	settings, err := s.DB.GetSettings()
	if err != nil {
		return "", debug.Wrap(err, "ARCHIVE_SETTINGS_LOAD_FAILED").ToError()
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("PMForge_Archive_%s.pmba", timestamp)

	certs := []string{}
	if settings.CertPath != "" {
		certs = append(certs, settings.CertPath)
	}

	if err := s.DB.CreateArchivalBundle(backupName, certs); err != nil {
		report := debug.Wrap(err, "ARCHIVAL_BUNDLE_ERROR")
		_ = s.DB.LogAction("System", "ARCHIVE_FAILED", projectPath, report.Message)
		return "", fmt.Errorf("security backup failed: %s", report.Message)
	}

	if err := s.DB.LogAction("System", "ARCHIVE_CREATED", projectPath, backupName); err != nil {
		if removeErr := os.Remove(backupName); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", fmt.Errorf("archive audit failed: %w; remove unaudited archive: %v", err, removeErr)
		}
		return "", fmt.Errorf("archive audit failed: %w", err)
	}
	return backupName, nil
}

// LogSignatureEvent records the outcome of a digital-signature attempt
// against a document. Always returns nil; signature outcomes must not
// fail the caller's broader workflow just because the audit write
// failed (we still attempt to log to stderr via debug.Wrap).
func (s *Service) LogSignatureEvent(docID string, success bool, err error) {
	status := "SUCCESS"
	details := "Digital signature applied successfully."
	if !success {
		status = "FAILED"
		details = fmt.Sprintf("Signature failed: %v", err)
		debug.Wrap(err, "PDF_SIGNATURE_ERROR") // logs to the persistent log file as a side effect
	}
	_ = s.DB.LogAction("System", "SIGNATURE_EVENT", docID, fmt.Sprintf("[%s] %s", status, details))
	s.logSignatureCheckpoint(docID, status, details)
}

func (s *Service) logSignatureCheckpoint(docID, status, details string) {
	doc, err := s.DB.GetDocument(docID)
	if err != nil {
		debug.Wrap(err, "SIGNATURE_AUDIT_DOCUMENT_LOOKUP_FAILED")
		return
	}
	signatureStatus := "signed"
	if status != "SUCCESS" {
		signatureStatus = "failed"
	}
	payload, err := json.Marshal(struct {
		DocumentID string `json:"document_id"`
		Status     string `json:"status"`
		Details    string `json:"details"`
	}{
		DocumentID: docID,
		Status:     status,
		Details:    details,
	})
	if err != nil {
		debug.Wrap(err, "SIGNATURE_AUDIT_PAYLOAD_FAILED")
		return
	}
	if _, err := s.DB.AppendAuditEvent(db.AuditEventInput{
		ProjectID:       doc.ProjectID,
		EventType:       "document.signature",
		EntityType:      "document",
		EntityID:        doc.ID,
		AfterJSON:       string(payload),
		SignatureStatus: signatureStatus,
	}); err != nil {
		debug.Wrap(err, "SIGNATURE_AUDIT_EVENT_FAILED")
	}
}
