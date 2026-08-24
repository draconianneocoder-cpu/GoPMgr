// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
		// Headless/CLI/test callers keep the historic deterministic path.
		// The remembered-directory lookup below is deliberately gated to
		// the interactive dialog branch only: folding it in here would make
		// headless exports and every test that relies on this exact path
		// depend on whatever a previous interactive session last chose.
		return filepath.Join(defaultDirectory, defaultFilename), nil
	}
	if runtime.saveFileDialog == nil {
		return "", errors.New("export destination dialog is required")
	}
	dialogDirectory := defaultDirectory
	if remembered := a.rememberedExportDirectory(); remembered != "" {
		dialogDirectory = remembered
	}
	path, err := runtime.saveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultDirectory:     dialogDirectory,
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
	a.rememberExportDirectory(filepath.Dir(path))
	return path, nil
}

// rememberedExportDirectory returns the signed-in user's last-chosen export
// directory if one is on record and still exists as a directory, or "" to
// fall back to the caller's own default (e.g. the directory was on a
// removable/synced volume that is no longer mounted).
func (a *App) rememberedExportDirectory() string {
	u := a.requireUser()
	if u == nil || u.LastExportDirectory == "" {
		return ""
	}
	info, err := os.Stat(u.LastExportDirectory)
	if err != nil || !info.IsDir() {
		return ""
	}
	return u.LastExportDirectory
}

// rememberExportDirectory persists dir as the signed-in user's new
// last-chosen export directory. Failure is logged and otherwise ignored --
// remembering the directory is a convenience, not a condition for the
// export that just succeeded. a.user is replaced with a new *Account value
// rather than mutated in place: requireUser hands out the existing pointer
// under only a read lock, so writing through it without the write lock
// would race any concurrent reader.
func (a *App) rememberExportDirectory(dir string) {
	u := a.requireUser()
	if u == nil || dir == "" || dir == u.LastExportDirectory {
		return
	}
	if a.store == nil {
		return
	}
	if err := a.store.SetLastExportDirectory(u.Username, dir); err != nil {
		log.Printf("remember export directory for %s: %v", u.Username, err)
		return
	}
	updated := *u
	updated.LastExportDirectory = dir
	a.mu.Lock()
	if a.user != nil && a.user.Username == u.Username {
		a.user = &updated
	}
	a.mu.Unlock()
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
