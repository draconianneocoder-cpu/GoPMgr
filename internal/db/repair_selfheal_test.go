// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// seedRows writes enough rows that the database spans several pages,
// a precondition for both corruption helpers below (byte offsets
// target specific pages past the 100-byte header).
func seedRows(t *testing.T, d *Database) {
	t.Helper()
	for i := range 50 {
		if _, err := d.UpsertProject(Project{Name: fmt.Sprintf("P%d", i), Status: "active", Phase: "execution"}); err != nil {
			t.Fatalf("seed rows: %v", err)
		}
	}
}

// corruptLightly closes d and flips a single byte in the database's
// second page. Confirmed by direct experimentation (not assumed):
// PRAGMA integrity_check reports this as corruption (ok=false,
// err=nil) on the ORIGINAL file, but VACUUM INTO -- which rebuilds
// every page via B-tree traversal -- produces a genuinely clean copy;
// checkSnapshotIntegrity on the resulting .bak passes. This is the
// fixture for InformativeSelfHeal's "Corruption found -> Snapshot is
// healthy" success path.
func corruptLightly(t *testing.T, d *Database, path string) {
	t.Helper()
	seedRows(t, d)
	if err := d.Close(); err != nil {
		t.Fatalf("close before corrupting: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live file: %v", err)
	}
	const offset = 4097 // one byte into the second page
	if offset >= len(b) {
		t.Fatalf("fixture file too small (%d bytes) for the corruption offset", len(b))
	}
	b[offset] ^= 0xFF
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}
}

// corruptSeverely closes d and flips bytes across several widely
// separated pages. Confirmed by direct experimentation: this is
// corrupt enough that PRAGMA integrity_check still reports ok=false
// without erroring (same as corruptLightly), but VACUUM INTO itself
// now fails ("database disk image is malformed") rather than healing
// it -- a genuinely distinct, previously 0%-covered branch
// (InformativeSelfHeal's SNAPSHOT_CREATION_FAILED report) from the
// lighter fixture's success path. The two fixtures together show
// VACUUM INTO's healing behavior is corruption-severity-dependent,
// not a guarantee.
func corruptSeverely(t *testing.T, d *Database, path string) {
	t.Helper()
	seedRows(t, d)
	if err := d.Close(); err != nil {
		t.Fatalf("close before corrupting: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live file: %v", err)
	}
	for _, off := range []int{4096, 8192, 12000, len(b) - 100} {
		if off >= 0 && off+3 <= len(b) {
			b[off] ^= 0xFF
			b[off+1] ^= 0xFF
			b[off+2] ^= 0xFF
		}
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}
}

// TestSwapInSnapshot_RejectsCorruptButParseableSnapshotBeforeClosingLive
// covers a branch distinct from the existing
// TestSwapInSnapshotRejectsInvalidSnapshotBeforeClosingLive: that test's
// "not a sqlite database" fixture fails checkSnapshotIntegrity's
// initial PRAGMA *query* (a structurally unparseable file), never
// reaching the `result != "ok"` string comparison at all. A snapshot
// that SQLite can still open and query -- corrupted the same way as
// corruptLightly's fixture -- reaches that comparison specifically;
// confirmed by direct experimentation that fault-seeding this
// comparison away leaves the other test still passing (it never
// exercises this branch), which is why this test exists as a
// separate case.
func TestSwapInSnapshot_RejectsCorruptButParseableSnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	snapshotPath := livePath + ".snapshot-source"
	sd, err := InitDB(snapshotPath)
	if err != nil {
		t.Fatalf("InitDB(snapshot source): %v", err)
	}
	corruptLightly(t, sd, snapshotPath)
	if err := os.Rename(snapshotPath, livePath+".bak"); err != nil {
		t.Fatalf("rename corrupted snapshot into place: %v", err)
	}

	if _, err := d.SwapInSnapshot(livePath); err == nil || !strings.Contains(err.Error(), "snapshot integrity") {
		t.Fatalf("SwapInSnapshot error = %v, want snapshot integrity error", err)
	}

	ok, err := d.CheckIntegrity()
	if err != nil {
		t.Fatalf("live handle should remain usable after snapshot preflight failure: %v", err)
	}
	if !ok {
		t.Fatal("live database failed integrity check after snapshot preflight failure")
	}
}

// TestSwapInSnapshot_HappyPathReplacesLiveAndPreservesOriginalForForensics
// is the plain (unencrypted) counterpart to
// TestSwapInEncryptedSnapshotPreservesEncryptionAndReopensWithDEK,
// previously entirely missing: repair_test.go only covered
// SwapInSnapshot's three preflight-rejection paths (all before the
// live connection is even closed), never its actual success path.
// Asserts the doc comment's "kept for forensics" claim by content --
// a distinguishing row written to the live DB *after* the snapshot was
// taken must survive only in the .corrupt file, not silently vanish
// or leak into the restored live database.
func TestSwapInSnapshot_HappyPathReplacesLiveAndPreservesOriginalForForensics(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(Project{Name: "BeforeSnapshot", Status: "active", Phase: "execution"}); err != nil {
		t.Fatalf("UpsertProject initial: %v", err)
	}
	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	// Mutate the live DB after the snapshot was taken so the snapshot
	// and the live state diverge -- this divergence is what proves
	// which one the swap actually promotes and which it preserves.
	p, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject before mutation: %v", err)
	}
	p.Name = "AfterSnapshotMutation"
	if _, err := d.UpsertProject(p); err != nil {
		t.Fatalf("UpsertProject mutated: %v", err)
	}

	fresh, err := d.SwapInSnapshot(livePath)
	if err != nil {
		t.Fatalf("SwapInSnapshot: %v", err)
	}
	defer fresh.Close()

	got, err := fresh.GetProject()
	if err != nil {
		t.Fatalf("GetProject after swap: %v", err)
	}
	if got.Name != "BeforeSnapshot" {
		t.Errorf("live project after swap = %q, want %q (snapshot state)", got.Name, "BeforeSnapshot")
	}

	corruptPath := livePath + ".corrupt"
	corruptDB, err := InitDB(corruptPath)
	if err != nil {
		t.Fatalf("open .corrupt for forensics check: %v", err)
	}
	defer corruptDB.Close()
	corruptProject, err := corruptDB.GetProject()
	if err != nil {
		t.Fatalf("GetProject on .corrupt: %v", err)
	}
	if corruptProject.Name != "AfterSnapshotMutation" {
		t.Errorf(".corrupt project = %q, want %q -- forensics copy does not contain the pre-swap live state",
			corruptProject.Name, "AfterSnapshotMutation")
	}
}

// TestSwapInSnapshot_NoExistingLiveFileSucceeds exercises the
// `else if !os.IsNotExist(err)` branch's implicit companion: when
// os.Stat(livePath) reports the file doesn't exist at all, no
// live-to-corrupt rename is attempted (movedLive stays false) and the
// snapshot is promoted directly. This is a general-purpose-helper
// contract, not a scenario the app's own call site
// (app_documents.go's RepairAndSwap) is known to exercise --
// InformativeSelfHeal always operates on a path it just used to open
// a live database, so livePath exists on that path today. Simulated
// here via external deletion of the live file, the only way to reach
// this branch deterministically.
func TestSwapInSnapshot_NoExistingLiveFileSucceeds(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(Project{Name: "OnlyProject", Status: "active", Phase: "execution"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := os.Remove(livePath); err != nil {
		t.Fatalf("simulate external deletion of live file: %v", err)
	}

	fresh, err := d.SwapInSnapshot(livePath)
	if err != nil {
		t.Fatalf("SwapInSnapshot with no existing live file: %v", err)
	}
	defer fresh.Close()

	if _, err := os.Stat(livePath + ".corrupt"); !os.IsNotExist(err) {
		t.Errorf(".corrupt file exists (%v) even though there was no live file to preserve", err)
	}
	if _, err := fresh.GetProject(); err != nil {
		t.Errorf("fresh handle unusable after swap: %v", err)
	}
}

// TestInformativeSelfHeal_CleanDatabaseSucceedsWithoutSnapshotting
// covers the fast path: a healthy database must report success
// immediately and must NOT create a .bak snapshot -- creating one
// unconditionally would misrepresent "no corruption" as "corruption
// was found and repaired" in the log the GUI renders verbatim.
func TestInformativeSelfHeal_CleanDatabaseSucceedsWithoutSnapshotting(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer d.Close()

	result, err := d.InformativeSelfHeal(livePath)
	if err != nil {
		t.Fatalf("InformativeSelfHeal: %v", err)
	}
	if !result.Success {
		t.Error("Success = false for a clean database")
	}
	if !strings.Contains(strings.Join(result.Log, "\n"), "No corruption found.") {
		t.Errorf("log = %v, want a \"No corruption found.\" entry", result.Log)
	}
	if _, err := os.Stat(livePath + ".bak"); !os.IsNotExist(err) {
		t.Errorf(".bak snapshot was created for a clean database: stat err = %v", err)
	}
}

// TestInformativeSelfHeal_LightCorruptionProducesHealthySnapshot covers
// the actual repair-succeeds path, using corruptLightly's fixture
// (confirmed by direct experimentation to make PRAGMA integrity_check
// fail while VACUUM INTO still produces a clean copy -- see that
// helper's comment). Also seeds a stale .bak file first, since VACUUM
// INTO refuses to overwrite an existing target -- proving the
// stale-removal step actually ran is necessary evidence, not just
// that the function didn't error.
func TestInformativeSelfHeal_LightCorruptionProducesHealthySnapshot(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	corruptLightly(t, d, livePath)

	if err := os.WriteFile(livePath+".bak", []byte("stale leftover from a previous repair attempt"), 0o600); err != nil {
		t.Fatalf("seed stale .bak: %v", err)
	}

	d2, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("reopen corrupted file: %v", err)
	}
	defer d2.Close()

	result, err := d2.InformativeSelfHeal(livePath)
	if err != nil {
		t.Fatalf("InformativeSelfHeal on lightly-corrupt database: %v", err)
	}
	if !result.Success {
		t.Fatalf("Success = false, log = %v", result.Log)
	}
	logJoined := strings.Join(result.Log, "\n")
	if !strings.Contains(logJoined, "Corruption found.") {
		t.Errorf("log = %v, want a \"Corruption found.\" entry", result.Log)
	}
	if !strings.Contains(logJoined, "Snapshot is healthy") {
		t.Errorf("log = %v, want a \"Snapshot is healthy\" entry", result.Log)
	}
	if err := checkSnapshotIntegrity(livePath + ".bak"); err != nil {
		t.Errorf("resulting .bak snapshot is not healthy: %v (stale .bak was not properly replaced)", err)
	}
}

// TestInformativeSelfHeal_SevereCorruptionReportsSnapshotCreationFailed
// covers the previously 0%-covered SNAPSHOT_CREATION_FAILED branch,
// using corruptSeverely's fixture -- corruption bad enough that VACUUM
// INTO itself fails, not just PRAGMA integrity_check. Distinct from
// the light-corruption success path above: the GUI's repair panel
// must be able to tell "we found and fixed it" from "we found it but
// couldn't create a snapshot to recover from," since only the first
// tells the user it's now safe to call SwapInSnapshot.
func TestInformativeSelfHeal_SevereCorruptionReportsSnapshotCreationFailed(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	corruptSeverely(t, d, livePath)

	d2, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("reopen corrupted file: %v", err)
	}
	defer d2.Close()

	result, err := d2.InformativeSelfHeal(livePath)
	if err == nil {
		t.Fatal("InformativeSelfHeal succeeded despite VACUUM INTO being unable to snapshot severe corruption")
	}
	if result.Success {
		t.Error("Success = true despite the snapshot-creation failure")
	}
	if result.Report.Context != "SNAPSHOT_CREATION_FAILED" {
		t.Errorf("report context = %q, want SNAPSHOT_CREATION_FAILED", result.Report.Context)
	}
}

// TestInformativeSelfHeal_StaleBackupRemovalFailureIsReported forces
// os.Remove(snapshotPath) to fail (a non-empty directory sitting where
// a stale .bak file is expected) and confirms the specific
// STALE_BACKUP_REMOVE_FAILED report code, not a generic error --
// the GUI's repair panel needs to distinguish "couldn't even start
// repairing" from every other failure shape in this flow.
func TestInformativeSelfHeal_StaleBackupRemovalFailureIsReported(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	corruptSeverely(t, d, livePath)

	staleBak := livePath + ".bak"
	if err := os.Mkdir(staleBak, 0o700); err != nil {
		t.Fatalf("seed non-empty stale .bak directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staleBak, "marker"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write marker inside stale .bak dir: %v", err)
	}

	d2, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("reopen corrupted file: %v", err)
	}
	defer d2.Close()

	result, err := d2.InformativeSelfHeal(livePath)
	if err == nil {
		t.Fatal("InformativeSelfHeal succeeded despite an unremovable stale .bak")
	}
	if result.Report.Context != "STALE_BACKUP_REMOVE_FAILED" {
		t.Errorf("report context = %q, want STALE_BACKUP_REMOVE_FAILED", result.Report.Context)
	}
}

// TestSwapInSnapshot_RollsBackLiveFileWhenFinalRenameFails is the
// highest-consequence untested branch in this package: if the final
// rename (snapshot -> live) fails AFTER the live file was already
// moved aside to .corrupt, the code must roll the .corrupt file back
// to the live path so the user is never left with zero database
// files -- worse than the corruption they started with. This
// specific interleaving (fail the second of two same-directory
// renames without touching the first) has no portable POSIX
// permission-based trigger: both renames share one parent directory's
// write permission, so blocking the directory blocks step 1 too, and
// no hook exists to intercept between the two calls. Confirmed
// forceable only via macOS's non-POSIX chflags(UF_IMMUTABLE), which
// blocks renaming a specific file regardless of directory
// permissions; syscall.Chflags does not exist on Linux, where this
// repository's CI runs (ubuntu-24.04), so this test is darwin-only
// and skips elsewhere -- the rollback branch itself remains
// unverified in CI, disclosed explicitly rather than silently
// skipped without comment.
func TestSwapInSnapshot_RollsBackLiveFileWhenFinalRenameFails(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("rollback-on-final-rename-failure is only forceable via macOS's chflags(UF_IMMUTABLE); unverified on this GOOS")
	}

	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(Project{Name: "OriginalLiveState", Status: "active", Phase: "execution"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	const ufImmutable = 0x2
	if err := syscall.Chflags(livePath+".bak", ufImmutable); err != nil {
		t.Fatalf("chflags(UF_IMMUTABLE) on snapshot: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Chflags(livePath+".bak", 0) })

	_, err = d.SwapInSnapshot(livePath)
	if err == nil {
		t.Fatal("SwapInSnapshot succeeded despite an immutable snapshot file blocking the final rename")
	}
	if strings.Contains(err.Error(), "rollback live:") {
		t.Fatalf("rollback itself failed: %v", err)
	}

	// The rollback must have restored the ORIGINAL live file, not left
	// the user with no database at all.
	restored, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("live path unusable after rollback: %v", err)
	}
	defer restored.Close()
	p, err := restored.GetProject()
	if err != nil {
		t.Fatalf("GetProject on rolled-back live file: %v", err)
	}
	if p.Name != "OriginalLiveState" {
		t.Errorf("rolled-back live project = %q, want %q", p.Name, "OriginalLiveState")
	}
	if _, err := os.Stat(livePath + ".corrupt"); !os.IsNotExist(err) {
		t.Errorf(".corrupt still exists after a successful rollback (should have been renamed back to live): stat err = %v", err)
	}
}
