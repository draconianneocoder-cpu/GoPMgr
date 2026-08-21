// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"gopmgr/internal/exportfs"
)

var (
	// ErrExportCancelled is a neutral user decision, not an export failure.
	// Frontend callers keep the current status unchanged when this is returned.
	ErrExportCancelled = errors.New("export cancelled")
	// ErrExportDestinationExists prevents an export from replacing a user file
	// or a derived report artifact the save dialog did not confirm.
	ErrExportDestinationExists = exportfs.ErrDestinationExists
)

type exportDestinationRuntime struct {
	saveFileDialog func(context.Context, wailsruntime.SaveDialogOptions) (string, error)
}

func productionExportDestinationRuntime() exportDestinationRuntime {
	return exportDestinationRuntime{saveFileDialog: wailsruntime.SaveFileDialog}
}

// selectExportDestination prompts a desktop user for a new output path. Direct
// and headless callers have no Wails context, so they retain the historic
// private exports-directory behavior for compatibility and deterministic tests.
func (a *App) selectExportDestination(defaultDirectory, defaultFilename, extension, title string) (string, error) {
	return a.selectExportDestinationWithRuntime(
		defaultDirectory,
		defaultFilename,
		extension,
		title,
		productionExportDestinationRuntime(),
	)
}

func (a *App) selectExportDestinationWithRuntime(
	defaultDirectory, defaultFilename, extension, title string,
	runtime exportDestinationRuntime,
) (string, error) {
	if filepath.Ext(defaultFilename) != extension {
		return "", fmt.Errorf("default export name must end in %q", extension)
	}
	if err := os.MkdirAll(defaultDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create default exports directory: %w", err)
	}
	if a.ctx == nil {
		return filepath.Join(defaultDirectory, defaultFilename), nil
	}
	if runtime.saveFileDialog == nil {
		return "", errors.New("export destination dialog is required")
	}
	path, err := runtime.saveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultDirectory:     defaultDirectory,
		DefaultFilename:      defaultFilename,
		Title:                title,
		Filters:              []wailsruntime.FileFilter{{DisplayName: strings.TrimPrefix(strings.ToUpper(extension), ".") + " files", Pattern: "*" + extension}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("choose export destination: %w", err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ErrExportCancelled
	}
	if filepath.Ext(path) != extension {
		return "", fmt.Errorf("export destination must use the %s extension", extension)
	}
	return path, nil
}

func ensureExportDestinationsAvailable(paths ...string) error {
	return exportfs.EnsureAvailable(paths...)
}

func writeNewPrivateExport(path string, data []byte) error {
	return exportfs.WriteNewPrivate(path, data)
}

func exportArtifactName(path string) string {
	return filepath.Base(path)
}
