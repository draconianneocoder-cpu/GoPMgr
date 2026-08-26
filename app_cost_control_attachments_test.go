// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/export"
	"gopmgr/internal/exportfs"
)

func TestAttachCostEntryFileAtPathStoresAndLists(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Attachments")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	entry, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Invoice", Amount: "10.00"})
	if err != nil {
		t.Fatalf("SaveCostEntry: %v", err)
	}

	src := filepath.Join(t.TempDir(), "invoice.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.4 fake content"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	attached, err := app.attachCostEntryFileAtPath(entry.ID, src)
	if err != nil {
		t.Fatalf("attachCostEntryFileAtPath: %v", err)
	}
	if attached.Filename != "invoice.pdf" || attached.SizeBytes != int64(len("%PDF-1.4 fake content")) || attached.ID == "" {
		t.Fatalf("attached = %#v", attached)
	}

	listed, err := app.ListCostEntryAttachments(entry.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != attached.ID {
		t.Fatalf("ListCostEntryAttachments = %#v, %v", listed, err)
	}
}

func TestExportCostEntryAttachmentsZipWritesManifestAndFiles(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Attachments zip")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	entryA, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Rebar delivery", Amount: "10.00", ItemName: "Rebar 10mm", SKU: "RB-10", SupplierName: "Acme Steel"})
	if err != nil {
		t.Fatalf("SaveCostEntry A: %v", err)
	}
	entryB, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-21", Description: "Second invoice", Amount: "20.00"})
	if err != nil {
		t.Fatalf("SaveCostEntry B: %v", err)
	}

	fixtureDir := t.TempDir()
	srcA := filepath.Join(fixtureDir, "receipt.pdf")
	if err := os.WriteFile(srcA, []byte("receipt-a-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	dupDir := filepath.Join(fixtureDir, "dup")
	if err := os.MkdirAll(dupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Same filename as srcA, in a different directory: must not collide in the zip.
	srcB := filepath.Join(dupDir, "receipt.pdf")
	if err := os.WriteFile(srcB, []byte("receipt-b-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := app.attachCostEntryFileAtPath(entryA.ID, srcA); err != nil {
		t.Fatalf("attach A: %v", err)
	}
	attB, err := app.attachCostEntryFileAtPath(entryB.ID, srcB)
	if err != nil {
		t.Fatalf("attach B: %v", err)
	}

	path, err := app.ExportCostEntryAttachmentsZip()
	if err != nil {
		t.Fatalf("ExportCostEntryAttachmentsZip: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("exported zip stat = %v, %v, want mode 600", info, err)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()
	files := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %q: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry %q: %v", f.Name, err)
		}
		files[f.Name] = string(data)
	}
	if len(files) != 3 { // 2 attachments + manifest.json
		t.Fatalf("zip entries = %v, want 2 attachments + manifest.json", files)
	}
	if files["attachments/receipt.pdf"] != "receipt-a-bytes" {
		t.Fatalf("first receipt.pdf entry = %q", files["attachments/receipt.pdf"])
	}
	wantDisambiguated := "attachments/receipt-" + attB.ID + ".pdf"
	if files[wantDisambiguated] != "receipt-b-bytes" {
		t.Fatalf("disambiguated second receipt.pdf = %q at %q, entries = %v", files[wantDisambiguated], wantDisambiguated, files)
	}

	var manifest []export.AttachmentManifestRow
	if err := json.Unmarshal([]byte(files["manifest.json"]), &manifest); err != nil {
		t.Fatalf("decode manifest.json: %v", err)
	}
	if len(manifest) != 2 {
		t.Fatalf("manifest rows = %d, want 2", len(manifest))
	}
	var manifestA export.AttachmentManifestRow
	for _, row := range manifest {
		if row.CostEntryID == entryA.ID {
			manifestA = row
		}
	}
	if manifestA.ItemName != "Rebar 10mm" || manifestA.SKU != "RB-10" || manifestA.SupplierName != "Acme Steel" || manifestA.CostEntryDescription != "Rebar delivery" {
		t.Fatalf("manifest row for entry A = %#v", manifestA)
	}

	// No-overwrite: ExportCostEntryAttachmentsZip publishes through
	// exportfs.WriteNewPrivateStream, which refuses to replace an existing
	// file at the destination. Prove that guarantee holds for this exact
	// produced path (not just exportfs's own generic tests) by attempting a
	// second publish at the identical path and confirming it is refused
	// rather than silently overwriting the first export.
	if err := exportfs.WriteNewPrivateStream(path, func(f *os.File) error {
		_, writeErr := f.Write([]byte("would-be second export"))
		return writeErr
	}); !errors.Is(err, exportfs.ErrDestinationExists) {
		t.Fatalf("second publish at the same attachments-zip path: err = %v, want ErrDestinationExists", err)
	}
}

func TestExportCostEntryAttachmentsZipRejectsWhenNoAttachments(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "No attachments")
	if _, err := app.ExportCostEntryAttachmentsZip(); err == nil {
		t.Fatal("ExportCostEntryAttachmentsZip with no attachments: want error, got nil")
	}
}

func TestAttachCostEntryFileAtPathRejectsOversizedFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Attachments oversized")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	entry, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Invoice", Amount: "10.00"})
	if err != nil {
		t.Fatalf("SaveCostEntry: %v", err)
	}

	src := filepath.Join(t.TempDir(), "big.bin")
	f, err := os.Create(src)
	if err != nil {
		t.Fatalf("create fixture file: %v", err)
	}
	if err := f.Truncate(db.MaxAttachmentBytes() + 1); err != nil {
		t.Fatalf("truncate fixture file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close fixture file: %v", err)
	}

	if _, err := app.attachCostEntryFileAtPath(entry.ID, src); !errors.Is(err, db.ErrAttachmentTooLarge) {
		t.Fatalf("attachCostEntryFileAtPath oversized: err = %v, want ErrAttachmentTooLarge", err)
	}
}
