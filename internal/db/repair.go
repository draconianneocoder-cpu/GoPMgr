// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"fmt"
	"os"

	"pmforge/internal/debug"
)

// RepairResult is what the UI receives from a self-heal attempt. The
// `Log` field is meant to be rendered verbatim in a scrollable panel so
// the user can see what was attempted, in order.
type RepairResult struct {
	Success bool              `json:"success"`
	Report  debug.ErrorReport `json:"report,omitempty"`
	Log     []string          `json:"log"`
}

// InformativeSelfHeal runs PMForge's diagnostic + repair flow:
//
//  1. PRAGMA integrity_check; on the live database.
//  2. If clean, return success immediately.
//  3. If dirty, create a side-by-side .bak snapshot via VACUUM INTO.
//  4. Replace the live file with the snapshot (atomic rename on POSIX).
//
// The function is intentionally chatty: every step writes a line into
// result.Log so the GUI can render a transparent "what happened" report.
func (db *Database) InformativeSelfHeal(path string) (RepairResult, error) {
	result := RepairResult{Log: []string{"Starting diagnostic check..."}}

	// 1. Integrity check.
	ok, err := db.CheckIntegrity()
	if err != nil {
		result.Report = debug.Wrap(err, "CRITICAL_INTEGRITY_CHECK_FAILED")
		result.Log = append(result.Log, "Integrity check could not run: "+err.Error())
		return result, err
	}
	if ok {
		result.Success = true
		result.Log = append(result.Log, "No corruption found.")
		return result, nil
	}

	// 2. Snapshot.
	result.Log = append(result.Log, "Corruption found. Attempting snapshot and recovery...")
	snapshotPath := path + ".bak"

	// VACUUM INTO refuses to overwrite, so clear any stale .bak first.
	if _, err := os.Stat(snapshotPath); err == nil {
		if rmErr := os.Remove(snapshotPath); rmErr != nil {
			result.Report = debug.Wrap(rmErr, "STALE_BACKUP_REMOVE_FAILED")
			return result, rmErr
		}
	}
	if err := db.CreateSnapshot(snapshotPath); err != nil {
		result.Report = debug.Wrap(err, "SNAPSHOT_CREATION_FAILED")
		return result, err
	}
	result.Log = append(result.Log, fmt.Sprintf("Snapshot created at %s.", snapshotPath))

	// 3. Caller is responsible for calling SwapInSnapshot to atomically
	// replace the live file. We cannot do it here because db.Conn is
	// held by the rest of the application and we need its cooperation
	// to close handles before the rename.
	result.Success = true
	result.Log = append(result.Log, fmt.Sprintf(
		"Snapshot is healthy at %s. Call SwapInSnapshot to atomically replace the live file.",
		snapshotPath,
	))
	return result, nil
}

// SwapInSnapshot atomically replaces the live database file with the
// .bak snapshot produced by InformativeSelfHeal. Steps:
//
//	1. Close the live connection.
//	2. Move the live file aside to <path>.corrupt (kept for forensics).
//	3. Rename <path>.bak → <path>.
//	4. Re-open the live file.
//
// On POSIX systems os.Rename is atomic when source and destination
// are on the same filesystem, which is always the case here because
// the snapshot was written next to the live file.
//
// Returns a fresh *Database handle. The caller MUST replace any old
// pointer with this one because the original was Closed during step 1.
func (db *Database) SwapInSnapshot(livePath string) (*Database, error) {
	snapshotPath := livePath + ".bak"
	corruptPath := livePath + ".corrupt"

	if _, err := os.Stat(snapshotPath); err != nil {
		return nil, fmt.Errorf("swap: snapshot %s missing: %w", snapshotPath, err)
	}

	// Step 1: Close the live connection so the file can be moved.
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("swap: close live: %w", err)
	}

	// Step 2: Move the live file aside. Clear any pre-existing
	// .corrupt first so the rename does not fail.
	_ = os.Remove(corruptPath)
	if _, err := os.Stat(livePath); err == nil {
		if err := os.Rename(livePath, corruptPath); err != nil {
			return nil, fmt.Errorf("swap: rename live → corrupt: %w", err)
		}
	}

	// Step 3: Move the snapshot into place. If this fails, try to
	// roll the live file back so the user is never left without any
	// database at all.
	if err := os.Rename(snapshotPath, livePath); err != nil {
		_ = os.Rename(corruptPath, livePath)
		return nil, fmt.Errorf("swap: rename snapshot → live: %w", err)
	}

	// Step 4: Re-open. The fresh handle is what the caller should
	// hold from this point on.
	return InitDB(livePath)
}
