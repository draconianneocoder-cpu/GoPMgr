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
