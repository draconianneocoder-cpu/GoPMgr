// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
)

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
// contract.
func WriteAttachmentsZIP(w io.Writer, sources []AttachmentZIPSource) error {
	zw := zip.NewWriter(w)
	manifest := make([]AttachmentManifestRow, 0, len(sources))
	for _, source := range sources {
		data, err := source.Fetch()
		if err != nil {
			return fmt.Errorf("fetch attachment %q: %w", source.ZipEntryName, err)
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
