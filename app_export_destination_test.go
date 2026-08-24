// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestSelectExportDestinationCancellationIsNeutral(t *testing.T) {
	app := &App{ctx: context.Background()}
	called := false
	_, err := app.selectExportDestinationWithRuntime(t.TempDir(), "report.pdf", ".pdf", "Export", exportDestinationRuntime{saveFileDialog: func(context.Context, wailsruntime.SaveDialogOptions) (string, error) { called = true; return "", nil }})
	if !called {
		t.Fatal("save dialog was not called")
	}
	if !errors.Is(err, ErrExportCancelled) {
		t.Fatalf("error = %v, want export cancellation", err)
	}
}

func TestSelectExportDestinationRejectsMisleadingExtension(t *testing.T) {
	app := &App{ctx: context.Background()}
	_, err := app.selectExportDestinationWithRuntime(t.TempDir(), "report.pdf", ".pdf", "Export", exportDestinationRuntime{saveFileDialog: func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return filepath.Join(t.TempDir(), "report.txt"), nil
	}})
	if err == nil {
		t.Fatal("expected extension error")
	}
}

// TestSelectExportDestinationHeadlessPathIgnoresRememberedDirectory proves
// the a.ctx == nil fallback never consults the remembered directory, even
// when one is on record -- Gate B's concern that folding the lookup into
// the deterministic headless path would make it depend on session history.
func TestSelectExportDestinationHeadlessPathIgnoresRememberedDirectory(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := app.store.SetLastExportDirectory("alice", t.TempDir()); err != nil {
		t.Fatalf("SetLastExportDirectory: %v", err)
	}
	defaultDir := t.TempDir()
	path, err := app.selectExportDestinationWithRuntime(defaultDir, "report.pdf", ".pdf", "Export", exportDestinationRuntime{})
	if err != nil {
		t.Fatalf("headless selectExportDestinationWithRuntime: %v", err)
	}
	if filepath.Dir(path) != defaultDir {
		t.Fatalf("headless path = %q, want directory %q (the remembered directory must not apply)", path, defaultDir)
	}
}

// TestSelectExportDestinationRemembersAndReusesChosenDirectory covers the
// full remembered-directory lifecycle: first dialog call has no remembered
// directory yet (uses the caller's default), the chosen directory is
// persisted to the store AND reflected on a.user without racing
// requireUser's read lock, and the next dialog call defaults there.
func TestSelectExportDestinationRemembersAndReusesChosenDirectory(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	app.ctx = context.Background()
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	firstDefaultDir := t.TempDir()
	chosenDir := t.TempDir()
	var firstDialogDefault string
	chosenPath := filepath.Join(chosenDir, "report.pdf")
	_, err := app.selectExportDestinationWithRuntime(firstDefaultDir, "report.pdf", ".pdf", "Export", exportDestinationRuntime{
		saveFileDialog: func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
			firstDialogDefault = opts.DefaultDirectory
			return chosenPath, nil
		},
	})
	if err != nil {
		t.Fatalf("first selectExportDestinationWithRuntime: %v", err)
	}
	if firstDialogDefault != firstDefaultDir {
		t.Fatalf("first dialog default = %q, want the caller's default %q (nothing remembered yet)", firstDialogDefault, firstDefaultDir)
	}

	persisted, err := app.store.Authenticate("alice", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Authenticate to check persistence: %v", err)
	}
	if persisted.LastExportDirectory != chosenDir {
		t.Fatalf("persisted LastExportDirectory = %q, want %q", persisted.LastExportDirectory, chosenDir)
	}
	if got := app.requireUser().LastExportDirectory; got != chosenDir {
		t.Fatalf("in-memory a.user.LastExportDirectory = %q, want %q (must update without a fresh login)", got, chosenDir)
	}

	var secondDialogDefault string
	_, err = app.selectExportDestinationWithRuntime(firstDefaultDir, "report2.pdf", ".pdf", "Export", exportDestinationRuntime{
		saveFileDialog: func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
			secondDialogDefault = opts.DefaultDirectory
			return "", nil // cancel; only the default matters for this assertion
		},
	})
	if !errors.Is(err, ErrExportCancelled) {
		t.Fatalf("second call error = %v, want ErrExportCancelled", err)
	}
	if secondDialogDefault != chosenDir {
		t.Fatalf("second dialog default = %q, want the remembered directory %q", secondDialogDefault, chosenDir)
	}
}

// TestSelectExportDestinationFallsBackWhenRememberedDirectoryIsGone covers
// the case where the remembered directory was removed (e.g. an unmounted
// removable/synced volume) between exports: the dialog must fall back to
// the caller's own default rather than pointing at a nonexistent path.
func TestSelectExportDestinationFallsBackWhenRememberedDirectoryIsGone(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	app.ctx = context.Background()
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	gone := filepath.Join(t.TempDir(), "no-longer-mounted")
	if err := app.store.SetLastExportDirectory("alice", gone); err != nil {
		t.Fatalf("SetLastExportDirectory: %v", err)
	}

	defaultDir := t.TempDir()
	var dialogDefault string
	_, err := app.selectExportDestinationWithRuntime(defaultDir, "report.pdf", ".pdf", "Export", exportDestinationRuntime{
		saveFileDialog: func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
			dialogDefault = opts.DefaultDirectory
			return "", nil
		},
	})
	if !errors.Is(err, ErrExportCancelled) {
		t.Fatalf("error = %v, want ErrExportCancelled", err)
	}
	if dialogDefault != defaultDir {
		t.Fatalf("dialog default = %q, want fallback to the caller's default %q (remembered directory no longer exists)", dialogDefault, defaultDir)
	}
}
