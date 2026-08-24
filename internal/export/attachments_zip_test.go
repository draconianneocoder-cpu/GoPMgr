// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func TestWriteAttachmentsZIPWritesEntriesAndManifest(t *testing.T) {
	var fetchCalls []string
	sources := []AttachmentZIPSource{
		{
			ZipEntryName: "attachments/invoice.pdf",
			Manifest:     AttachmentManifestRow{ZipEntryName: "attachments/invoice.pdf", Filename: "invoice.pdf", SizeBytes: 5, CostEntryID: "cost_1", ItemName: "Rebar 10mm", SKU: "RB-10", SupplierName: "Acme Steel"},
			Fetch:        func() ([]byte, error) { fetchCalls = append(fetchCalls, "invoice.pdf"); return []byte("PDF12"), nil },
		},
		{
			ZipEntryName: "attachments/photo.jpg",
			Manifest:     AttachmentManifestRow{ZipEntryName: "attachments/photo.jpg", Filename: "photo.jpg", SizeBytes: 3, CostEntryID: "cost_2"},
			Fetch:        func() ([]byte, error) { fetchCalls = append(fetchCalls, "photo.jpg"); return []byte("JPG"), nil },
		},
	}

	var buf bytes.Buffer
	if err := WriteAttachmentsZIP(&buf, sources); err != nil {
		t.Fatalf("WriteAttachmentsZIP: %v", err)
	}
	if got := len(fetchCalls); got != 2 || fetchCalls[0] != "invoice.pdf" || fetchCalls[1] != "photo.jpg" {
		t.Fatalf("fetch order/count = %v, want [invoice.pdf photo.jpg] called once each in order", fetchCalls)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open written zip: %v", err)
	}
	files := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %q: %v", f.Name, err)
		}
		files[f.Name] = string(data)
	}
	if files["attachments/invoice.pdf"] != "PDF12" || files["attachments/photo.jpg"] != "JPG" {
		t.Fatalf("zip contents = %#v", files)
	}
	var manifest []AttachmentManifestRow
	if err := json.Unmarshal([]byte(files["manifest.json"]), &manifest); err != nil {
		t.Fatalf("decode manifest.json: %v", err)
	}
	if len(manifest) != 2 || manifest[0].Filename != "invoice.pdf" || manifest[0].SupplierName != "Acme Steel" || manifest[1].Filename != "photo.jpg" {
		t.Fatalf("manifest = %#v", manifest)
	}
	// The manifest must never carry vendor address/contact fields -- proven
	// structurally here by AttachmentManifestRow having no such field at
	// all, so there's nothing to assert is empty; this test instead pins
	// that only the expected identity/display fields round-trip.
}

func TestWriteAttachmentsZIPPropagatesFetchError(t *testing.T) {
	boom := errors.New("read failed")
	sources := []AttachmentZIPSource{
		{ZipEntryName: "attachments/a.txt", Fetch: func() ([]byte, error) { return nil, boom }},
	}
	var buf bytes.Buffer
	err := WriteAttachmentsZIP(&buf, sources)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("WriteAttachmentsZIP error = %v, want wrapped %v", err, boom)
	}
}
