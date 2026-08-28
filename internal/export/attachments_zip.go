// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrAttachmentIntegrity means fetched attachment bytes do not match the
// stored metadata that the ZIP manifest would otherwise describe.
var ErrAttachmentIntegrity = errors.New("attachment integrity check failed")

// AttachmentManifestRow is one entry in an attachments ZIP's manifest.json.
// It intentionally carries only what already lives on the ledger row's own
// display snapshot -- item/SKU/supplier NAME and invoice reference -- never
// supplier address or contact details, which project-cost-ledger-scope.md
// excludes from ordinary financial exports and which CostEntry never stores
// in the first place.
type AttachmentManifestRow struct {
	ZipEntryName         string `json:"zip_entry_name"`
	Filename             string `json:"filename"`
	ContentType          string `json:"content_type"`
	SizeBytes            int64  `json:"size_bytes"`
	SHA256               string `json:"sha256"`
	CostEntryID          string `json:"cost_entry_id"`
	CostEntryDate        string `json:"cost_entry_date"`
	CostEntryDescription string `json:"cost_entry_description"`
	ItemName             string `json:"item_name,omitempty"`
	SKU                  string `json:"sku,omitempty"`
	SupplierName         string `json:"supplier_name,omitempty"`
	InvoiceReference     string `json:"invoice_reference,omitempty"`
}

// AttachmentZIPSource is one attachment to stream into the archive.
// ZipEntryName must already be sanitized and unique among the sources
// passed to WriteAttachmentsZIP -- this package does not second-guess a
// caller-supplied archive path. Fetch is called at most once, in order,
// so only one attachment's bytes are ever resident in memory at a time
// regardless of how many sources there are.
type AttachmentZIPSource struct {
	ZipEntryName string
	Manifest     AttachmentManifestRow
	Fetch        func() ([]byte, error)
}

// WriteAttachmentsZIP streams every source into w as a ZIP archive plus a
// top-level manifest.json describing each entry, fetching one attachment's
// bytes at a time rather than buffering the whole export. It writes nothing
// about vendor address/contact detail, matching AttachmentManifestRow's
// contract. Each fetched blob is checked against its stored byte count and
// SHA-256 before its archive entry is created. Atomic publication and cleanup
// on a later-source failure belong to exportfs.WriteNewPrivateStream.
func WriteAttachmentsZIP(w io.Writer, sources []AttachmentZIPSource) error {
	zw := zip.NewWriter(w)
	manifest := make([]AttachmentManifestRow, 0, len(sources))
	for _, source := range sources {
		expectedDigest, err := validateAttachmentManifest(source)
		if err != nil {
			return err
		}
		if source.Fetch == nil {
			return fmt.Errorf("fetch attachment %q: fetch function is required", source.ZipEntryName)
		}
		data, err := source.Fetch()
		if err != nil {
			return fmt.Errorf("fetch attachment %q: %w", source.ZipEntryName, err)
		}
		if int64(len(data)) != source.Manifest.SizeBytes {
			return fmt.Errorf("verify attachment %q: %w: stored size %d, fetched size %d", source.ZipEntryName, ErrAttachmentIntegrity, source.Manifest.SizeBytes, len(data))
		}
		actualDigest := sha256.Sum256(data)
		if actualDigest != expectedDigest {
			return fmt.Errorf("verify attachment %q: %w: SHA-256 differs from stored metadata", source.ZipEntryName, ErrAttachmentIntegrity)
		}
		entry, err := zw.Create(source.ZipEntryName)
		if err != nil {
			return fmt.Errorf("create zip entry %q: %w", source.ZipEntryName, err)
		}
		if _, err := entry.Write(data); err != nil {
			return fmt.Errorf("write zip entry %q: %w", source.ZipEntryName, err)
		}
		manifest = append(manifest, source.Manifest)
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode attachments manifest: %w", err)
	}
	manifestEntry, err := zw.Create("manifest.json")
	if err != nil {
		return fmt.Errorf("create manifest.json entry: %w", err)
	}
	if _, err := manifestEntry.Write(manifestJSON); err != nil {
		return fmt.Errorf("write manifest.json entry: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finalize attachments zip: %w", err)
	}
	return nil
}

func validateAttachmentManifest(source AttachmentZIPSource) ([sha256.Size]byte, error) {
	if source.Manifest.ZipEntryName != source.ZipEntryName {
		return [sha256.Size]byte{}, fmt.Errorf("verify attachment %q: %w: manifest archive name differs", source.ZipEntryName, ErrAttachmentIntegrity)
	}
	if source.Manifest.SizeBytes < 0 {
		return [sha256.Size]byte{}, fmt.Errorf("verify attachment %q: %w: stored size is negative", source.ZipEntryName, ErrAttachmentIntegrity)
	}
	encodedDigest := source.Manifest.SHA256
	if len(encodedDigest) != sha256.Size*2 {
		return [sha256.Size]byte{}, fmt.Errorf("verify attachment %q: %w: stored SHA-256 is malformed", source.ZipEntryName, ErrAttachmentIntegrity)
	}
	decodedDigest, err := hex.DecodeString(encodedDigest)
	if err != nil || len(decodedDigest) != sha256.Size {
		return [sha256.Size]byte{}, fmt.Errorf("verify attachment %q: %w: stored SHA-256 is malformed", source.ZipEntryName, ErrAttachmentIntegrity)
	}
	var expectedDigest [sha256.Size]byte
	copy(expectedDigest[:], decodedDigest)
	return expectedDigest, nil
}
