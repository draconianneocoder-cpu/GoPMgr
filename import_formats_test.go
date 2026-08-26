// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportScheduleFileRejectsBinaryFormats verifies the binary/serialized
// project formats (.mpp, .pod) and the legacy .mpx text format return a
// clear, actionable message pointing at the MS Project XML interchange path,
// rather than failing opaquely. These branches run before any project DB is
// needed, so a bare App is sufficient.
func TestImportScheduleFileRejectsBinaryFormats(t *testing.T) {
	app := &App{}
	cases := map[string]string{
		"/tmp/schedule.mpp": "Microsoft Project XML",
		"/tmp/schedule.MPP": "Microsoft Project XML",
		"/tmp/legacy.mpx":   "Microsoft Project XML",
		"/tmp/plan.pod":     "Microsoft Project XML",
	}
	for path, want := range cases {
		_, err := app.importScheduleFile(path)
		if err == nil {
			t.Errorf("%s: expected an error, got nil", path)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q should mention %q", path, err.Error(), want)
		}
	}
}

// TestImportScheduleFileRejectsOversizedFile proves the early os.Stat-based
// refusal: a file reported larger than maxMSPDIImportSize is refused before
// any read is attempted. The file is sparse (os.Truncate, no real bytes
// written) since only its reported size matters for this branch.
func TestImportScheduleFileRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.xml")
	f, err := os.Create(path) // #nosec G304 -- test-owned temp path.
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := f.Truncate(maxMSPDIImportSize + 1); err != nil {
		t.Fatalf("truncate temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	app := &App{}
	_, err = app.importScheduleFile(path)
	if err == nil {
		t.Fatal("expected an error for an oversized import file, got nil")
	}
	if !strings.Contains(err.Error(), "MSPDI import limit") {
		t.Errorf("error %q should mention the MSPDI import limit", err.Error())
	}
}

// Proves oversized refusal at a shrunk cap with a real (non-sparse) file.
// Fault-seeding showed this passes even without io.LimitReader -- the
// post-read length check catches it -- so what this pins is "oversized
// input never reaches the XML parser," not bounded peak memory.
func TestImportScheduleFileBoundsReadRegardlessOfStatSize(t *testing.T) {
	original := maxMSPDIImportSize
	maxMSPDIImportSize = 16
	t.Cleanup(func() { maxMSPDIImportSize = original })

	path := filepath.Join(t.TempDir(), "small-but-over-cap.xml")
	content := []byte("<Project>this is more than sixteen bytes</Project>")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	app := &App{}
	_, err := app.importScheduleFile(path)
	if err == nil {
		t.Fatal("expected an error once the file exceeds the (shrunk) import limit, got nil")
	}
	if !strings.Contains(err.Error(), "MSPDI import limit") {
		t.Errorf("error %q should mention the MSPDI import limit", err.Error())
	}
}
