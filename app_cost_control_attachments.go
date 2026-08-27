// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"gopmgr/internal/db"
	"gopmgr/internal/export"
	"gopmgr/internal/exportfs"
)

// CostEntryAttachmentWire is the Wails-safe boundary for attachment
// metadata. It never carries blob bytes -- attachment content moves through
// the native file dialog (upload) or ExportCostEntryAttachmentsZip
// (download), never as a JSON-encoded byte array.
type CostEntryAttachmentWire struct {
	ID          string `json:"id"`
	CostEntryID string `json:"cost_entry_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	CreatedAt   string `json:"created_at"`
}

func costEntryAttachmentWire(a db.CostEntryAttachment) CostEntryAttachmentWire {
	return CostEntryAttachmentWire{ID: a.ID, CostEntryID: a.CostEntryID, Filename: a.Filename, ContentType: a.ContentType, SizeBytes: a.SizeBytes, SHA256: a.SHA256, CreatedAt: a.CreatedAt}
}

// AttachCostEntryFile prompts the user to pick a file through the native
// file dialog and stores it as a bounded attachment on an existing ledger
// row.
func (a *App) AttachCostEntryFile(costEntryID string) (CostEntryAttachmentWire, error) {
	if a.ctx == nil {
		return CostEntryAttachmentWire{}, errors.New("no context (Wails not started)")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Attach file to ledger entry",
		DefaultDirectory: a.userDir(),
	})
	if err != nil {
		return CostEntryAttachmentWire{}, err
	}
	if path == "" {
		return CostEntryAttachmentWire{}, ErrExportCancelled
	}
	return a.attachCostEntryFileAtPath(costEntryID, path)
}

// attachCostEntryFileAtPath is AttachCostEntryFile's testable core: reads
// path through a size-limited reader (failing fast with
// db.ErrAttachmentTooLarge before buffering an oversized file in full) and
// stores it as a bounded attachment. Separated from the OpenFileDialog
// wrapper the same way ImportMSPDIChartWithOptions separates from
// importScheduleFileWithOptions, since a dialog call cannot be driven from a
// headless test.
func (a *App) attachCostEntryFileAtPath(costEntryID, path string) (CostEntryAttachmentWire, error) {
	d := a.requireDB()
	if d == nil {
		return CostEntryAttachmentWire{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return CostEntryAttachmentWire{}, err
	}
	if err := costControlMutationAllowed(p); err != nil {
		return CostEntryAttachmentWire{}, err
	}
	data, err := readBoundedFile(path, db.MaxAttachmentBytes())
	if err != nil {
		return CostEntryAttachmentWire{}, err
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	saved, err := d.SaveCostEntryAttachment(p.ID, costEntryID, filepath.Base(path), contentType, data)
	if err != nil {
		return CostEntryAttachmentWire{}, err
	}
	return costEntryAttachmentWire(saved), nil
}

// ListCostEntryAttachments returns metadata for every attachment on one
// ledger row.
func (a *App) ListCostEntryAttachments(costEntryID string) ([]CostEntryAttachmentWire, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	rows, err := d.ListCostEntryAttachments(p.ID, costEntryID)
	if err != nil {
		return nil, err
	}
	out := make([]CostEntryAttachmentWire, 0, len(rows))
	for _, r := range rows {
		out = append(out, costEntryAttachmentWire(r))
	}
	return out, nil
}

// ExportCostEntryAttachmentsZip prompts the user for a destination and
// writes every attachment across the project's ledger into a single ZIP,
// alongside a manifest.json metadata sidecar (project-cost-ledger-scope.md
// item 3's "explicit no-overwrite ZIP attachment export" requirement). It
// never replaces an existing file at the chosen destination: publication
// goes through the same exportfs no-overwrite primitive every other export
// in this application uses. Attachments are streamed one at a time (see
// export.WriteAttachmentsZIP), so memory use is bounded by a single
// attachment's size, not the project's total attachment budget.
func (a *App) ExportCostEntryAttachmentsZip() (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return "", err
	}
	attachments, err := d.ListCostEntryAttachmentsForProject(p.ID)
	if err != nil {
		return "", err
	}
	if len(attachments) == 0 {
		return "", errors.New("this project has no ledger attachments to export")
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil {
		return "", err
	}
	entriesByID := make(map[string]db.CostEntry, len(entries))
	for _, e := range entries {
		entriesByID[e.ID] = e
	}

	seen := make(map[string]bool, len(attachments))
	sources := make([]export.AttachmentZIPSource, 0, len(attachments))
	for _, att := range attachments {
		entry := entriesByID[att.CostEntryID]
		zipName := uniqueAttachmentZipEntryName(seen, att.Filename, att.ID)
		att := att // capture for the closure below
		sources = append(sources, export.AttachmentZIPSource{
			ZipEntryName: zipName,
			Manifest: export.AttachmentManifestRow{
				ZipEntryName: zipName, Filename: att.Filename, ContentType: att.ContentType, SizeBytes: att.SizeBytes, SHA256: att.SHA256,
				CostEntryID: att.CostEntryID, CostEntryDate: entry.CostDate, CostEntryDescription: entry.Description,
				ItemName: entry.ItemName, SKU: entry.SKU, SupplierName: entry.SupplierName, InvoiceReference: entry.InvoiceReference,
			},
			Fetch: func() ([]byte, error) {
				_, data, err := d.GetCostEntryAttachmentBlob(p.ID, att.ID)
				return data, err
			},
		})
	}

	path, err := a.selectExportDestination(
		filepath.Join(u.DataDir, "exports"),
		fmt.Sprintf("%s-Ledger-Attachments-%s.zip", sanitizeFilename(p.Name), time.Now().UTC().Format("20060102-150405")),
		".zip", "Export ledger attachments",
	)
	if err != nil {
		return "", err
	}
	if err := exportfs.WriteNewPrivateStream(path, func(f *os.File) error {
		return export.WriteAttachmentsZIP(f, sources)
	}); err != nil {
		return "", err
	}
	return path, nil
}

// uniqueAttachmentZipEntryName places filename under attachments/ inside the
// archive, disambiguating with the attachment's own ID when two attachments
// (necessarily from different ledger rows -- SaveCostEntryAttachment already
// sanitizes to a bare basename, so a collision means two different rows
// happened to use the same filename) would otherwise collide.
func uniqueAttachmentZipEntryName(seen map[string]bool, filename, attachmentID string) string {
	name := "attachments/" + filename
	if !seen[name] {
		seen[name] = true
		return name
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	disambiguated := fmt.Sprintf("attachments/%s-%s%s", base, attachmentID, ext)
	seen[disambiguated] = true
	return disambiguated
}

// readBoundedFile reads path into memory, refusing anything larger than
// limit bytes without ever holding more than limit+1 bytes at once --
// db.ErrAttachmentTooLarge is returned before the full (potentially huge)
// file is buffered, matching the io.LimitReader(r, limit+1) convention used
// elsewhere in this codebase for bounding untrusted-size reads (see
// internal/db/backup_restore.go's readZipEntry).
func readBoundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- path chosen by the user via the native OS file dialog.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return readBoundedReader(f, limit)
}

// readBoundedReader is readBoundedFile's core, split out so a test can
// prove the io.LimitReader bound itself holds -- by wrapping a counting
// io.Reader around a source far larger than limit and asserting how many
// bytes were actually consumed -- without needing an on-disk fixture of
// comparable size, and independently of SaveCostEntryAttachment's own
// downstream size check (which would reject an oversized result the same
// way regardless of how much was read to produce it).
func readBoundedReader(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read attachment file: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, db.ErrAttachmentTooLarge
	}
	return data, nil
}
