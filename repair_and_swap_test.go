// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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
	if !result.Success || !result.Swapped || result.DamagedCopy != path+".corrupt" {
		t.Fatalf("RepairAndSwap: want Success, Swapped, and DamagedCopy=%s, got %+v", path+".corrupt", result)
	}
	if _, err := os.Stat(result.DamagedCopy); err != nil {
		t.Fatalf("damaged copy not kept at %s: %v", result.DamagedCopy, err)
	}

	if got, err := app.ListStakeholders(""); err != nil || len(got) != 300 {
		t.Fatalf("ListStakeholders after repair: want 300 rows, got %d, %v", len(got), err)
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

// A .bak left beside a healthy project (an interrupted earlier repair, the
// command-line --repair, a manual or synced copy) is an older copy of the
// project. RepairAndSwap must never swap it in: before 2026-09-29 it swapped
// in any .bak it found, so a valid stale snapshot silently rolled a healthy
// project back, and an invalid one produced a "Swap failed" error.
func TestRepairAndSwapIgnoresALeftoverSnapshotOnAHealthyProject(t *testing.T) {
	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, app *App, path string)
	}{
		{"valid older snapshot", func(t *testing.T, app *App, path string) {
			if err := app.requireDB().CreateSnapshot(path + ".bak"); err != nil {
				t.Fatalf("CreateSnapshot: %v", err)
			}
		}},
		{"not a database", func(t *testing.T, _ *App, path string) {
			if err := os.WriteFile(path+".bak", []byte("not a valid sqlite snapshot"), 0o600); err != nil {
				t.Fatalf("plant bogus .bak: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newEncryptionProjectTestApp(t)
			if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
				t.Fatalf("CreateAccount: %v", err)
			}
			path := mustOpenProject(t, app, "Proj")
			if _, err := app.SaveStakeholder(db.Stakeholder{Name: "Before", Category: db.StakeholderTeam}); err != nil {
				t.Fatalf("SaveStakeholder: %v", err)
			}
			tc.plant(t, app, path)
			if _, err := app.SaveStakeholder(db.Stakeholder{Name: "After", Category: db.StakeholderTeam}); err != nil {
				t.Fatalf("SaveStakeholder: %v", err)
			}

			result, err := app.RepairAndSwap()
			if err != nil || !result.Success || result.Swapped || result.Snapshot != "" {
				t.Fatalf("RepairAndSwap on a healthy project = %+v, %v; want success with nothing swapped", result, err)
			}
			got, err := app.ListStakeholders("")
			if err != nil || len(got) != 2 {
				t.Fatalf("stakeholders after repair = %d, %v; want both, including the one added after the leftover snapshot", len(got), err)
			}
			if _, err := os.Stat(path + ".corrupt"); !os.IsNotExist(err) {
				t.Fatalf("a healthy project was moved aside to .corrupt (stat err %v)", err)
			}
		})
	}
}

// TestRepairAndSwapBlocksOtherCallsUntilTheSwapIsDone tries to close the
// project between the heal and the swap. RepairAndSwap holds the write lock
// for the whole operation, so the close must wait; without the lock it would
// run first and the swap would install a handle for a project no longer open.
func TestRepairAndSwapBlocksOtherCallsUntilTheSwapIsDone(t *testing.T) {
	app, path, pristine := seedRepairFixtureProject(t)
	corruptByteAt(t, path, pristine, 3*4096+1)
	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}

	closed := make(chan error, 1)
	closedEarly := false
	previous := afterRepairHeal
	t.Cleanup(func() { afterRepairHeal = previous })
	afterRepairHeal = func() {
		go func() { closed <- app.CloseProject() }()
		select {
		case err := <-closed:
			closedEarly = true
			t.Errorf("CloseProject ran between the heal and the swap (err %v)", err)
		case <-time.After(300 * time.Millisecond):
		}
	}

	result, err := app.RepairAndSwap()
	if closedEarly {
		t.FailNow()
	}
	if err != nil || !result.Swapped {
		t.Fatalf("RepairAndSwap = %+v, %v; want a swap", result, err)
	}
	if err := <-closed; err != nil {
		t.Fatalf("CloseProject after the repair: %v", err)
	}
	if app.requireDB() != nil {
		t.Fatal("the project is still open after CloseProject ran")
	}
}

// failSwapWith replaces swapSnapshot for one test with a swap that runs
// fail (which may close the live handle or move files) and returns its error.
func failSwapWith(t *testing.T, fail func(d *db.Database, path string) error) {
	t.Helper()
	previous := swapSnapshot
	t.Cleanup(func() { swapSnapshot = previous })
	swapSnapshot = func(d *db.Database, path string, _ []byte, _ bool) (*db.Database, error) {
		return nil, fail(d, path)
	}
}

// openHealableProject opens the repair fixture with light corruption, so
// RepairAndSwap heals it and reaches the swap.
func openHealableProject(t *testing.T) (*App, string) {
	t.Helper()
	app, path, pristine := seedRepairFixtureProject(t)
	corruptByteAt(t, path, pristine, 3*4096+1)
	if _, err := app.OpenProject(path); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	return app, path
}

// A swap that fails after closing the live handle (here, the rename of the
// live file) used to leave a.db on the closed handle, so every later call
// failed with "sql: database is closed". The project file is still in place,
// so it is reopened and keeps working.
func TestRepairAndSwapReopensTheProjectWhenTheSwapFailsAfterClosingIt(t *testing.T) {
	app, _ := openHealableProject(t)
	swapErr := errors.New("swap: rename live → corrupt: permission denied")
	failSwapWith(t, func(d *db.Database, _ string) error {
		_ = d.Close()
		return swapErr
	})

	result, err := app.RepairAndSwap()
	if !errors.Is(err, swapErr) || errors.Is(err, ErrRepairClosedProject) || result.Swapped {
		t.Fatalf("RepairAndSwap = %+v, %v; want the swap error, nothing swapped, project still open", result, err)
	}
	if got, err := app.ListStakeholders(""); err != nil || len(got) != 300 {
		t.Fatalf("ListStakeholders after the failed swap = %d rows, %v; want the reopened project's 300", len(got), err)
	}
}

// If nothing is left at the project path (a rename failed and so did its
// rollback, leaving the data at <path>.corrupt), reopening would create an
// empty project there. The project is closed instead and nothing is created.
func TestRepairAndSwapClosesTheProjectWhenItCannotBeReopened(t *testing.T) {
	app, path := openHealableProject(t)
	failSwapWith(t, func(d *db.Database, path string) error {
		_ = d.Close()
		if err := os.Rename(path, path+".corrupt"); err != nil {
			t.Fatalf("move live file aside: %v", err)
		}
		return errors.New("swap: rename snapshot → live: disk full; rollback live: disk full")
	})

	_, err := app.RepairAndSwap()
	if !errors.Is(err, ErrRepairClosedProject) {
		t.Fatalf("RepairAndSwap = %v, want ErrRepairClosedProject", err)
	}
	// ProjectRepairPanel.svelte matches this phrase to forget the project.
	if !strings.Contains(err.Error(), "the project was closed") {
		t.Fatalf("closed-project error text = %q, want it to contain \"the project was closed\"", err.Error())
	}
	if app.requireDB() != nil {
		t.Fatal("the project is still open on a handle after it could not be reopened")
	}
	if _, err := app.ListStakeholders(""); err == nil || !strings.Contains(err.Error(), "no project open") {
		t.Fatalf("ListStakeholders after the project was closed = %v, want \"no project open\"", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("an empty project was created at %s after the failed swap (stat err %v)", path, err)
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("the project data at .corrupt was touched: %v", err)
	}
}

// A swap that fails before closing anything (for example, the snapshot
// failed its check) keeps the same, still-working handle.
func TestRepairAndSwapKeepsTheOpenHandleWhenTheSwapFailsBeforeClosingIt(t *testing.T) {
	app, _ := openHealableProject(t)
	before := app.requireDB()
	swapErr := errors.New("swap: encrypted snapshot integrity: file is not a database")
	failSwapWith(t, func(*db.Database, string) error { return swapErr })

	if _, err := app.RepairAndSwap(); !errors.Is(err, swapErr) {
		t.Fatalf("RepairAndSwap = %v, want the swap error", err)
	}
	if app.requireDB() != before {
		t.Fatal("a still-open handle was replaced after a swap that closed nothing")
	}
	if _, err := app.ListStakeholders(""); err != nil {
		t.Fatalf("ListStakeholders after the failed swap: %v", err)
	}
}
