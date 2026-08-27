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

// TestImportScheduleFileRejectsOversizedRealFileAtShrunkCap proves oversized
// refusal at a shrunk cap using a real (non-sparse) file, complementing the
// sparse-file fast-path test above. It does NOT prove independence from the
// os.Stat fast path -- this file's real byte count matches what Stat
// reports, both exceed the cap, so the shipped os.Stat check catches it
// first; instrumentation (added, run, then reverted during a later
// assurance review -- see internal/crypto/pdf_sign_test.go's equivalent
// test for the full decomposition and internal/fonts' for why the
// second-layer post-read check specifically matters there) confirmed the
// second-layer (io.LimitReader + post-read length check) code is never
// reached by this test, or by the sparse-file test above. What the second
// layer alone proves rests on separate os.Stat-disabled fault-seeding, not
// on either test in this file, and that fault-seeding is weaker evidence
// here than in the two sibling packages: this package's bare App{} test
// harness hits its own unconditional "no project open" guard before ever
// reaching XML parsing, so a fault-seed that disables only the post-read
// check can't show what specifically would go wrong downstream (a
// truncated-but-plausible XML parse, a silent accept) the way the P12 and
// TTF sibling tests could. The os.Stat-vs-actual-size TOCTOU scenario named
// in the guard's code comment is accordingly unproven by any test here; it
// rests on io.LimitReader's documented stdlib contract, not independent
// evidence. No exactly-at-cap boundary test exists in this file either
// (added to the two sibling packages during the same review) -- a `>` ->
// `>=` mutation on this guard's boundary would go undetected; disclosed as
// a residual gap rather than added here to avoid mixing this review's
// scope with a behavior-preserving-only pass over already-shipped code.
func TestImportScheduleFileRejectsOversizedRealFileAtShrunkCap(t *testing.T) {
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
