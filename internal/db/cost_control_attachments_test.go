// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"strings"
	"testing"
)

func seedAttachmentTestEntry(t *testing.T, d *Database) (projectID, costEntryID string) {
	t.Helper()
	p, err := d.UpsertProject(Project{Name: "Attachments"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Invoice", AmountMinorUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	return p.ID, entry.ID
}

func TestSaveCostEntryAttachmentStoresAndReturnsMetadata(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)

	a, err := d.SaveCostEntryAttachment(projectID, costEntryID, "invoice.pdf", "application/pdf", []byte("hello world"))
	if err != nil {
		t.Fatalf("SaveCostEntryAttachment: %v", err)
	}
	if a.Filename != "invoice.pdf" || a.ContentType != "application/pdf" || a.SizeBytes != int64(len("hello world")) || a.SHA256 == "" || a.CreatedAt == "" || a.ID == "" {
		t.Fatalf("attachment metadata = %#v", a)
	}

	listed, err := d.ListCostEntryAttachments(projectID, costEntryID)
	if err != nil || len(listed) != 1 || listed[0].ID != a.ID {
		t.Fatalf("ListCostEntryAttachments = %#v, %v", listed, err)
	}

	got, data, err := d.GetCostEntryAttachmentBlob(projectID, a.ID)
	if err != nil {
		t.Fatalf("GetCostEntryAttachmentBlob: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("blob data = %q", data)
	}
	if got.ID != a.ID {
		t.Fatalf("blob metadata id = %q, want %q", got.ID, a.ID)
	}
}

func TestSaveCostEntryAttachmentSanitizesFilename(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)

	a, err := d.SaveCostEntryAttachment(projectID, costEntryID, "../../etc/passwd", "text/plain", []byte("x"))
	if err != nil {
		t.Fatalf("SaveCostEntryAttachment with traversal filename: %v", err)
	}
	if a.Filename != "passwd" {
		t.Fatalf("sanitized filename = %q, want %q (basename only)", a.Filename, "passwd")
	}

	for _, bad := range []string{"", ".", "..", "   "} {
		if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, bad, "text/plain", []byte("x")); !errors.Is(err, ErrAttachmentFilenameInvalid) {
			t.Errorf("filename %q: err = %v, want ErrAttachmentFilenameInvalid", bad, err)
		}
	}
}

func TestSaveCostEntryAttachmentEnforcesBounds(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)

	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "empty.txt", "text/plain", nil); !errors.Is(err, ErrAttachmentEmpty) {
		t.Fatalf("empty attachment err = %v, want ErrAttachmentEmpty", err)
	}

	oversized := make([]byte, maxAttachmentBytes+1)
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "big.bin", "application/octet-stream", oversized); !errors.Is(err, ErrAttachmentTooLarge) {
		t.Fatalf("oversized attachment err = %v, want ErrAttachmentTooLarge", err)
	}

	for i := range maxAttachmentsPerEntry {
		if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "f.txt", "text/plain", []byte("x")); err != nil {
			t.Fatalf("seed attachment %d: %v", i, err)
		}
	}
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "one-too-many.txt", "text/plain", []byte("x")); !errors.Is(err, ErrAttachmentEntryFull) {
		t.Fatalf("11th attachment err = %v, want ErrAttachmentEntryFull", err)
	}
}

// TestSaveCostEntryAttachmentAcceptsFileExactlyAtCap proves the boundary is
// `>`, not `>=`: an attachment of exactly maxAttachmentBytes must be
// accepted. This is reachable on the normal Wails upload path, not just
// direct API use: readBoundedFile (app_cost_control_attachments.go) passes
// through data up to and including its limit, and that limit is
// MaxAttachmentBytes() == maxAttachmentBytes here, so a legitimate
// max-size upload reaches this exact boundary.
func TestSaveCostEntryAttachmentAcceptsFileExactlyAtCap(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)

	exact := make([]byte, maxAttachmentBytes)
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "exact.bin", "application/octet-stream", exact); err != nil {
		t.Fatalf("attachment exactly at the cap: %v", err)
	}
}

func TestSaveCostEntryAttachmentEnforcesProjectBudget(t *testing.T) {
	// The real 200 MiB budget is far larger than maxAttachmentBytes (10
	// MiB), so no single call can approach it -- lower the budget for this
	// test rather than allocate/write on that order. maxAttachmentTotalBytesPerProject
	// is a var precisely so this boundary is provable without that cost.
	previous := maxAttachmentTotalBytesPerProject
	maxAttachmentTotalBytesPerProject = 10
	t.Cleanup(func() { maxAttachmentTotalBytesPerProject = previous })

	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)
	types, err := d.ListCostTypes(projectID)
	if err != nil {
		t.Fatal(err)
	}
	otherEntry, err := d.SaveCostEntry(CostEntry{ProjectID: projectID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-21", Description: "Second invoice", AmountMinorUnits: 100})
	if err != nil {
		t.Fatal(err)
	}

	// Land exactly on the boundary: budget is 10, this attachment is
	// exactly 10 bytes (0+10 > 10 is false) -- proves the boundary is `>`,
	// not `>=`, rather than only exercising "well under."
	exact := make([]byte, maxAttachmentTotalBytesPerProject)
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "a.bin", "application/octet-stream", exact); err != nil {
		t.Fatalf("SaveCostEntryAttachment exactly at the lowered budget: %v", err)
	}
	// A second attachment (even on a different cost entry in the same
	// project) that would push the running total even one byte over the
	// aggregate cap must be rejected -- the budget is project-wide, not
	// per-entry.
	if _, err := d.SaveCostEntryAttachment(projectID, otherEntry.ID, "b.bin", "application/octet-stream", []byte("x")); !errors.Is(err, ErrAttachmentProjectBudget) {
		t.Fatalf("over-budget attachment err = %v, want ErrAttachmentProjectBudget", err)
	}
}

func TestSaveCostEntryAttachmentRejectsWrongProjectOrMissingEntry(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)

	if _, err := d.SaveCostEntryAttachment(projectID, "does-not-exist", "f.txt", "text/plain", []byte("x")); !errors.Is(err, ErrCostEntryNotFound) {
		t.Fatalf("missing entry err = %v, want ErrCostEntryNotFound", err)
	}
	if _, err := d.SaveCostEntryAttachment("some-other-project", costEntryID, "f.txt", "text/plain", []byte("x")); !errors.Is(err, ErrCostEntryNotFound) {
		t.Fatalf("cross-project entry err = %v, want ErrCostEntryNotFound", err)
	}

	// Cross-project blob read must also fail closed.
	a, err := d.SaveCostEntryAttachment(projectID, costEntryID, "f.txt", "text/plain", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.GetCostEntryAttachmentBlob("some-other-project", a.ID); err == nil {
		t.Fatal("GetCostEntryAttachmentBlob across projects: want error, got nil")
	}
}

func TestCostEntryAttachmentAuditRecordsMetadataOnly(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)
	secret := "SECRET-BLOB-CONTENT-MUST-NOT-LEAK-INTO-AUDIT"
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "f.txt", "text/plain", []byte(secret)); err != nil {
		t.Fatal(err)
	}
	events, err := d.ListAuditEvents(projectID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.EventType != "cost_entry_attachment.create" {
			continue
		}
		found = true
		if strings.Contains(e.AfterCanonicalJSON, secret) {
			t.Fatalf("audit event leaked blob bytes: %s", e.AfterCanonicalJSON)
		}
		if !strings.Contains(e.AfterCanonicalJSON, "f.txt") {
			t.Fatalf("audit event missing filename metadata: %s", e.AfterCanonicalJSON)
		}
	}
	if !found {
		t.Fatal("no cost_entry_attachment.create audit event found")
	}
}

func TestListCostEntryAttachmentsForProject(t *testing.T) {
	d := newCostControlTestDB(t)
	projectID, costEntryID := seedAttachmentTestEntry(t, d)
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "a.txt", "text/plain", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveCostEntryAttachment(projectID, costEntryID, "b.txt", "text/plain", []byte("b")); err != nil {
		t.Fatal(err)
	}
	got, err := d.ListCostEntryAttachmentsForProject(projectID)
	if err != nil || len(got) != 2 {
		t.Fatalf("ListCostEntryAttachmentsForProject = %#v, %v, want 2", got, err)
	}
}
