// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

// seedRepairFixtureProject creates and opens an encrypted project seeded
// with 300 stakeholders (enough rows to span the file's data pages well
// past the schema/settings/project pages OpenProject's own startup
// queries touch), then closes it and returns the file path with a
// pristine copy of its bytes for corruption-and-restore round trips.
//
// The specific byte offsets used by the two tests below
// (TestRepairAndSwapHealsReachableLightCorruption,
// TestRepairAndSwapCanFailToHealEvenWhenReached) were derived from an
// exhaustive page-by-page corruption sweep against this exact fixture
// (one byte flipped per 4096-byte page, OpenProject/ListStakeholders/
// RepairAndSwap outcome recorded, byte restored, next page) — see
// docs/beta-release-backlog.md's "Confirm self-heal is reachable on
// real encrypted corruption" entry for the full sweep results. Both
// offsets were confirmed to reproduce identically across repeated runs.
// If a future schema migration changes the page layout, these offsets
// may need to be re-derived the same way — the existing precedent for
// this kind of fixed-offset fixture is internal/db/repair_selfheal_test.go's
// corruptLightly (offset 4097 into a plaintext database).
func seedRepairFixtureProject(t *testing.T) (app *App, path string, pristine []byte) {
	t.Helper()
	app = newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	path = mustOpenProject(t, app, "Proj")

	for i := range 300 {
		if _, err := app.SaveStakeholder(db.Stakeholder{Name: fmt.Sprintf("S%d", i), Category: db.StakeholderTeam}); err != nil {
			t.Fatalf("seed stakeholder %d: %v", i, err)
		}
	}
	if err := app.CloseProject(); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}

	pristine, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pristine file: %v", err)
	}
	return app, path, pristine
}

func corruptByteAt(t *testing.T, path string, pristine []byte, offset int) {
	t.Helper()
	corrupted := make([]byte, len(pristine))
	copy(corrupted, pristine)
	if offset >= len(corrupted) {
		t.Fatalf("corruption offset %d is beyond file size %d — fixture layout has changed, re-derive via the page sweep", offset, len(corrupted))
	}
	corrupted[offset] ^= 0xFF
	if err := os.WriteFile(path, corrupted, 0o600); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}
}

// TestRepairAndSwapHealsReachableLightCorruption proves App.RepairAndSwap
// is reachable and can successfully heal real encrypted-project
// corruption — closing the open question in docs/beta-release-backlog.md
// of whether RepairAndSwap's own db != nil precondition ever holds after
// realistic corruption. It does: single-byte corruption in page 3 (of
// this fixture) leaves OpenProject and a real query both succeeding,
// then RepairAndSwap detects the underlying corruption during its own
// VACUUM INTO snapshot pass and heals it, after which the project is
// fully queryable again.
func TestRepairAndSwapHealsReachableLightCorruption(t *testing.T) {
	app, path, pristine := seedRepairFixtureProject(t)
	const page3Offset = 3*4096 + 1
	corruptByteAt(t, path, pristine, page3Offset)

	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject: want success on this corruption pattern, got %v", err)
	}
	if _, err := app.ListStakeholders(""); err != nil {
		t.Fatalf("ListStakeholders before repair: want success, got %v", err)
	}

	result, err := app.RepairAndSwap()
	if err != nil {
		t.Fatalf("RepairAndSwap: %v", err)
	}
	if !result.Success {
		t.Fatalf("RepairAndSwap: want Success=true, got %+v", result)
	}

	if _, err := app.ListStakeholders(""); err != nil {
		t.Fatalf("ListStakeholders after repair: want success, got %v", err)
	}
}

// TestRepairAndSwapCanFailToHealEvenWhenReached pins current, real
// behavior discovered by the same sweep: RepairAndSwap being reachable
// does not guarantee it can heal what it finds. This corruption pattern
// (page 99) leaves OpenProject succeeding but a real query failing —
// genuinely user-visible corruption, exactly the scenario RepairAndSwap
// exists for — yet RepairAndSwap's own VACUUM INTO snapshot attempt
// fails with the same underlying error rather than producing a healthy
// copy. This mirrors the existing, already-documented distinction in
// internal/db/repair_selfheal_test.go between light corruption (VACUUM
// INTO heals it) and severe corruption (VACUUM INTO itself fails) —
// this test demonstrates that distinction is real on the encrypted path
// too, not merely a plaintext-SQLite characteristic. This is pinned as
// current behavior, not asserted as a defect: no test can assert VACUUM
// INTO must always heal arbitrary corruption. It exists so a future
// change that makes healing silently give up (or start succeeding) here
// is a deliberate, reviewed change rather than an unnoticed regression.
func TestRepairAndSwapCanFailToHealEvenWhenReached(t *testing.T) {
	app, path, pristine := seedRepairFixtureProject(t)
	const page99Offset = 99*4096 + 1
	corruptByteAt(t, path, pristine, page99Offset)

	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject: want success on this corruption pattern, got %v", err)
	}
	if _, err := app.ListStakeholders(""); err == nil {
		t.Fatal("ListStakeholders: want a real error proving this corruption is user-visible, got nil")
	}

	result, err := app.RepairAndSwap()
	// Pin the specific failure the sweep observed — CreateSnapshot's own
	// VACUUM INTO failing during InformativeSelfHeal's snapshot step —
	// not just "some error, or Success left false for any reason". A
	// future change that makes RepairAndSwap fail earlier or later for
	// an unrelated cause would otherwise slip past a bare err==nil check.
	if err == nil {
		t.Fatalf("RepairAndSwap: want a snapshot-creation error on this pinned-unhealable corruption pattern, got nil (result=%+v) — if this now heals, that's a real improvement worth its own test update, not a silent pass", result)
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("RepairAndSwap: want the underlying SQLite corruption error, got %v", err)
	}
	if result.Report.Context != "SNAPSHOT_CREATION_FAILED" {
		t.Fatalf("RepairAndSwap: want Report.Context=SNAPSHOT_CREATION_FAILED (InformativeSelfHeal's snapshot step), got %q", result.Report.Context)
	}
	if result.Success {
		t.Fatal("RepairAndSwap: want Success=false alongside the snapshot-creation error")
	}
}
