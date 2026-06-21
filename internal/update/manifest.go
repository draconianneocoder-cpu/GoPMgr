// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Manifest is the schema of the JSON document the update server
// publishes at <ManifestURL>. The server's release pipeline signs
// the canonical JSON payload (Payload field, raw bytes) with an
// Ed25519 key and base64-encodes the signature into `signature`.
//
// Verification is two steps:
//
//  1. Decode the base64 signature.
//  2. ed25519.Verify(pubkey, payloadBytes, sig).
//
// Reasoning: Ed25519 is fast, fixed-size, and entirely stdlib.
// RSA would require shipping a CA bundle or hard-coding a key; the
// public-key-only model fits PMForge's local-first ethos.
type Manifest struct {
	// Wire form: {"payload": "<base64-json>", "signature": "<base64-sig>"}
	PayloadB64   string `json:"payload"`
	SignatureB64 string `json:"signature"`
}

// Payload is what's inside Manifest.PayloadB64 once decoded.
type Payload struct {
	LatestVersion string `json:"latest_version"`  // e.g. "1.2.0"
	ReleaseNotes  string `json:"release_notes"`   // human-readable, markdown
	DownloadURL   string `json:"download_url"`    // optional; GUI shows a "Download" button
	SHA256        string `json:"sha256,omitempty"` // optional digest of the download artifact
	PublishedAt   string `json:"published_at"`    // RFC3339
}

// ErrInvalidSignature is returned by VerifyManifest when the
// Ed25519 signature doesn't match the embedded public key. Callers
// MUST treat this as a hard failure — never use unsigned payload
// data for "is there an update?" decisions.
var ErrInvalidSignature = errors.New("update: manifest signature invalid")

// VerifyManifest decodes the signed manifest, checks the Ed25519
// signature against publicKey, and returns the decoded Payload on
// success. On signature failure returns ErrInvalidSignature.
func VerifyManifest(raw []byte, publicKey ed25519.PublicKey) (Payload, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return Payload{}, fmt.Errorf("update: bad public key length %d", len(publicKey))
	}

	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Payload{}, fmt.Errorf("update: parse manifest: %w", err)
	}

	payloadBytes, err := base64.StdEncoding.DecodeString(m.PayloadB64)
	if err != nil {
		return Payload{}, fmt.Errorf("update: decode payload: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(m.SignatureB64)
	if err != nil {
		return Payload{}, fmt.Errorf("update: decode signature: %w", err)
	}

	if !ed25519.Verify(publicKey, payloadBytes, sig) {
		return Payload{}, ErrInvalidSignature
	}

	var p Payload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return Payload{}, fmt.Errorf("update: parse payload: %w", err)
	}
	return p, nil
}
