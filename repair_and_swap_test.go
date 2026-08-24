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
// Used only by the two corruption-sweep tests below
// (TestRepairAndSwapHealsReachableLightCorruption,
// TestRepairAndSwapCanFailToHealEvenWhenReached) —
// TestRepairAndSwapReportsSwapFailureForBadBakFile needs no corruption
// and builds its own minimal fixture instead.
//
// The specific byte offsets used by those two tests were derived from an
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
//
// This already happened once: the sigma_projects FK-bug schema
// migration (2026-08-17, adding sigma_projects.project_id and its
// rebuild path) shifted this fixture's page layout enough that page
// 99 — TestRepairAndSwapCanFailToHealEvenWhenReached's original
// severe-corruption offset — moved into a page Migrate() itself now
// fails to open, an outcome that test doesn't exercise. Re-swept pages
// 90-160 with a temporary throwaway test (not committed) and found pages 113
// and 114 as valid candidates. Page 113 reproduced the same "OpenProject and the first query both
// succeed, RepairAndSwap's own VACUUM INTO fails" outcome across 3
// repeated runs; TestRepairAndSwapHealsReachableLightCorruption's page
// 3 offset was unaffected and confirmed still passing, unchanged.
//
// This happened a third time on 2026-08-23: the project-cost-ledger-scope.md
// item 3 migration (cost_entries procurement columns + the new
// cost_entry_attachments table) shifted the layout again, moving page 113
// into a page Migrate() now fails to open. Re-swept pages 90-200 with a
// temporary throwaway test (not committed) and found pages 119 and 120 as
// valid candidates; page 119 reproduced the same reachable-but-unhealable
// outcome across 3 repeated runs. Page 3 was unaffected and confirmed still
// passing, unchanged.
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
// (page 119 — re-derived 2026-08-23 after the project-cost-ledger-scope.md
// item 3 migration (cost_entries procurement columns + the new
// cost_entry_attachments table) again shifted this fixture's page layout;
// the prior sweep had found page 113, which after this migration lands on
// a page Migrate() itself now fails to open, an OpenProject-failure outcome
// this test doesn't exercise) leaves OpenProject succeeding but a real query failing —
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
	const page119Offset = 119*4096 + 1
	corruptByteAt(t, path, pristine, page119Offset)

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

// TestRepairAndSwapReportsSwapFailureForBadBakFile covers RepairAndSwap's
// "Swap failed" branch (app_documents.go) — reached when InformativeSelfHeal
// reports the live database as already healthy (so it creates no snapshot
// of its own) but a *pre-existing* .bak file is present from some other
// source. This is not a contrived fixture: RepairAndSwap's own comment
// documents that it detects "do the swap" by checking for a .bak file's
// mere presence, not by parsing InformativeSelfHeal's log — so any
// leftover .bak (an interrupted prior repair, a manual copy, a synced
// file from another device) drives this exact path on the next repair
// attempt. A .bak that isn't a valid encrypted snapshot must be reported
// as a failed swap, not silently accepted or allowed to corrupt the live
// file the swap step already renamed aside.
func TestRepairAndSwapReportsSwapFailureForBadBakFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	path := mustOpenProject(t, app, "Proj")
	if _, err := app.SaveStakeholder(db.Stakeholder{Name: "Dana", Category: db.StakeholderTeam}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}

	if err := os.WriteFile(path+".bak", []byte("not a valid sqlite snapshot"), 0o600); err != nil {
		t.Fatalf("plant bogus .bak: %v", err)
	}

	result, err := app.RepairAndSwap()
	if err == nil {
		t.Fatalf("RepairAndSwap: want an error for a bogus .bak snapshot, got nil (result=%+v)", result)
	}
	if !strings.Contains(err.Error(), "swap:") {
		t.Fatalf("RepairAndSwap: want the swap-step error wrapped through, got %v", err)
	}
	found := false
	for _, line := range result.Log {
		if strings.HasPrefix(line, "Swap failed:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("RepairAndSwap: want a \"Swap failed:\" log line, got %v", result.Log)
	}

	// The live database must still be open and usable — a failed swap
	// must never have touched it (SwapInEncryptedSnapshot verifies the
	// snapshot before it ever closes or renames the live file).
	if _, err := app.ListStakeholders(""); err != nil {
		t.Fatalf("ListStakeholders after failed swap: want the live db untouched and usable, got %v", err)
	}
}
