// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/exportfs"
)

func attachmentManifest(zipEntryName, filename string, data []byte) AttachmentManifestRow {
	digest := sha256.Sum256(data)
	return AttachmentManifestRow{
		ZipEntryName: zipEntryName,
		Filename:     filename,
		SizeBytes:    int64(len(data)),
		SHA256:       hex.EncodeToString(digest[:]),
	}
}

func TestWriteAttachmentsZIPWritesEntriesAndManifest(t *testing.T) {
	var fetchCalls []string
	invoiceManifest := attachmentManifest("attachments/invoice.pdf", "invoice.pdf", []byte("PDF12"))
	invoiceManifest.CostEntryID, invoiceManifest.ItemName, invoiceManifest.SKU, invoiceManifest.SupplierName = "cost_1", "Rebar 10mm", "RB-10", "Acme Steel"
	photoManifest := attachmentManifest("attachments/photo.jpg", "photo.jpg", []byte("JPG"))
	photoManifest.CostEntryID = "cost_2"
	sources := []AttachmentZIPSource{
		{
			ZipEntryName: "attachments/invoice.pdf",
			Manifest:     invoiceManifest,
			Fetch:        func() ([]byte, error) { fetchCalls = append(fetchCalls, "invoice.pdf"); return []byte("PDF12"), nil },
		},
		{
			ZipEntryName: "attachments/photo.jpg",
			Manifest:     photoManifest,
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
	if want := []AttachmentManifestRow{invoiceManifest, photoManifest}; !reflect.DeepEqual(manifest, want) {
		t.Fatalf("manifest = %#v, want %#v", manifest, want)
	}
	// The manifest must never carry vendor address/contact fields -- proven
	// structurally here by AttachmentManifestRow having no such field at
	// all, so there's nothing to assert is empty; this test instead pins
	// that only the expected identity/display fields round-trip.
}

func TestWriteAttachmentsZIPPropagatesFetchError(t *testing.T) {
	boom := errors.New("read failed")
	content := []byte("data")
	sources := []AttachmentZIPSource{
		{ZipEntryName: "attachments/a.txt", Manifest: attachmentManifest("attachments/a.txt", "a.txt", content), Fetch: func() ([]byte, error) { return nil, boom }},
	}
	var buf bytes.Buffer
	err := WriteAttachmentsZIP(&buf, sources)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("WriteAttachmentsZIP error = %v, want wrapped %v", err, boom)
	}
}

func TestWriteAttachmentsZIPRejectsInvalidIntegrityMetadataBeforeWritingEntry(t *testing.T) {
	content := []byte("expected")
	valid := attachmentManifest("attachments/a.txt", "a.txt", content)
	tests := []struct {
		name   string
		modify func(*AttachmentManifestRow)
		fetch  []byte
	}{
		{"missing hash", func(row *AttachmentManifestRow) { row.SHA256 = "" }, content},
		{"malformed hash", func(row *AttachmentManifestRow) { row.SHA256 = "not-a-sha256" }, content},
		{"negative size", func(row *AttachmentManifestRow) { row.SizeBytes = -1 }, content},
		{"manifest archive name differs", func(row *AttachmentManifestRow) { row.ZipEntryName = "attachments/other.txt" }, content},
		{"size mismatch", func(row *AttachmentManifestRow) { row.SizeBytes++ }, content},
		{"digest mismatch", func(row *AttachmentManifestRow) {}, []byte("corrupt!")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := valid
			tt.modify(&row)
			var buf bytes.Buffer
			err := WriteAttachmentsZIP(&buf, []AttachmentZIPSource{{
				ZipEntryName: "attachments/a.txt", Manifest: row,
				Fetch: func() ([]byte, error) { return tt.fetch, nil },
			}})
			if !errors.Is(err, ErrAttachmentIntegrity) {
				t.Fatalf("WriteAttachmentsZIP error = %v, want ErrAttachmentIntegrity", err)
			}
			if buf.Len() != 0 {
				t.Fatalf("buffer length after first-source integrity failure = %d, want 0", buf.Len())
			}
		})
	}
}

func TestWriteAttachmentsZIPRejectsNilFetch(t *testing.T) {
	content := []byte("data")
	err := WriteAttachmentsZIP(io.Discard, []AttachmentZIPSource{{
		ZipEntryName: "attachments/a.txt", Manifest: attachmentManifest("attachments/a.txt", "a.txt", content),
	}})
	if err == nil || !strings.Contains(err.Error(), "fetch function is required") {
		t.Fatalf("WriteAttachmentsZIP error = %v, want required fetch error", err)
	}
}

func TestWriteAttachmentsZIPLaterIntegrityFailureDoesNotPublishExport(t *testing.T) {
	validData := []byte("first")
	expectedSecond := []byte("second")
	path := filepath.Join(t.TempDir(), "attachments.zip")
	err := exportfs.WriteNewPrivateStream(path, func(f *os.File) error {
		return WriteAttachmentsZIP(f, []AttachmentZIPSource{
			{ZipEntryName: "attachments/first.txt", Manifest: attachmentManifest("attachments/first.txt", "first.txt", validData), Fetch: func() ([]byte, error) { return validData, nil }},
			{ZipEntryName: "attachments/second.txt", Manifest: attachmentManifest("attachments/second.txt", "second.txt", expectedSecond), Fetch: func() ([]byte, error) { return []byte("corupt"), nil }},
		})
	})
	if !errors.Is(err, ErrAttachmentIntegrity) {
		t.Fatalf("WriteNewPrivateStream error = %v, want ErrAttachmentIntegrity", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("published attachment ZIP stat error = %v, want not exist", statErr)
	}
	temporary, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".gopmgr-export-*"))
	if err != nil {
		t.Fatalf("glob export temporary files: %v", err)
	}
	if len(temporary) != 0 {
		t.Fatalf("export temporary files = %v, want none", temporary)
	}
}
