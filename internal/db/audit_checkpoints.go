// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
)

func appendApprovalCheckpointTx(tx *sql.Tx, projectID, entityType, entityID, approvalType, approvedJSON string) (AuditEvent, error) {
	return appendApprovalCheckpointForUserTx(tx, projectID, entityType, entityID, approvalType, approvedJSON, "")
}

func appendApprovalCheckpointForUserTx(tx *sql.Tx, projectID, entityType, entityID, approvalType, approvedJSON, userID string) (AuditEvent, error) {
	canonicalApproved, err := canonicalJSON(approvedJSON)
	if err != nil {
		return AuditEvent{}, err
	}
	hash := sha256.Sum256([]byte(canonicalApproved))
	payloadHash := hex.EncodeToString(hash[:])
	now := nowTimestamp()

	payload, err := json.Marshal(struct {
		ApprovalType string `json:"approval_type"`
		EntityType   string `json:"entity_type"`
		EntityID     string `json:"entity_id"`
		PayloadHash  string `json:"payload_hash"`
		TimestampUTC string `json:"timestamp_utc"`
	}{
		ApprovalType: approvalType,
		EntityType:   entityType,
		EntityID:     entityID,
		PayloadHash:  payloadHash,
		TimestampUTC: now,
	})
	if err != nil {
		return AuditEvent{}, err
	}
	signatureBlob, err := json.Marshal(struct {
		Algorithm      string `json:"algorithm"`
		CheckpointType string `json:"checkpoint_type"`
		PayloadHash    string `json:"payload_hash"`
		SignedAtUTC    string `json:"signed_at_utc"`
	}{
		Algorithm:      "SHA-256",
		CheckpointType: "approval",
		PayloadHash:    payloadHash,
		SignedAtUTC:    now,
	})
	if err != nil {
		return AuditEvent{}, err
	}

	return appendAuditEventTx(tx, AuditEventInput{
		ProjectID:             projectID,
		EventType:             entityType + ".approval_checkpoint",
		EntityType:            entityType,
		EntityID:              entityID,
		AfterJSON:             string(payload),
		UserID:                userID,
		SignatureStatus:       "signed",
		SignatureBlobOptional: string(signatureBlob),
	})
}
