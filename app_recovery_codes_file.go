// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"gopmgr/internal/users"
)

// recoveryCodeFormat matches a code as users.IssueRecoveryCodes returns it:
// two groups of eight base32 characters joined by a dash.
var recoveryCodeFormat = regexp.MustCompile(`^[A-Z2-7]{8}-[A-Z2-7]{8}$`)

// SaveRecoveryCodesFile writes freshly shown recovery codes to a text file
// the user picks in a save dialog, and returns its path. Account creation and
// the Admin panel used a browser blob download, which was never shown to
// produce a file in the desktop window; this uses the same native save
// dialog as the other exports.
//
// Unlike project exports, the dialog opens in the home folder and the chosen
// folder is not remembered: a recovery file should not default to GoPMgr's
// own data folder beside the encrypted projects, nor become the default for
// later exports. It needs a window, so it never falls back to a default path.
func (a *App) SaveRecoveryCodesFile(username string, codes []string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("saving recovery codes needs the application window")
	}
	return a.saveRecoveryCodesFileWithRuntime(username, codes, productionExportDestinationRuntime())
}

func (a *App) saveRecoveryCodesFileWithRuntime(username string, codes []string, runtime exportDestinationRuntime) (string, error) {
	if a.requireUser() == nil {
		return "", errors.New("not signed in")
	}
	if err := users.ValidateUsername(username); err != nil {
		return "", err
	}
	if len(codes) == 0 || len(codes) > users.RecoveryCodeCount {
		return "", fmt.Errorf("expected 1 to %d recovery codes, got %d", users.RecoveryCodeCount, len(codes))
	}
	for _, code := range codes {
		if !recoveryCodeFormat.MatchString(code) {
			return "", errors.New("not a recovery code")
		}
	}
	if runtime.saveFileDialog == nil {
		return "", errors.New("save dialog is required")
	}

	// The date tells sets apart after a renewal, in the default name and in
	// the file itself, since the file may be renamed.
	created := time.Now().Format("2006-01-02")
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	path, err := runtime.saveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultDirectory:     home,
		DefaultFilename:      "gopmgr-recovery-codes-" + username + "-" + created + ".txt",
		Title:                "Save recovery codes",
		Filters:              []wailsruntime.FileFilter{{DisplayName: "Text files", Pattern: "*.txt"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("choose where to save the recovery codes: %w", err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ErrExportCancelled
	}
	// Some platforms' dialogs return a typed name without the filter's
	// extension; add it. ".TXT" is still a text file; any other extension
	// is refused.
	switch ext := filepath.Ext(path); {
	case strings.EqualFold(ext, ".txt"):
	case ext == "":
		path += ".txt"
	default:
		return "", errors.New("recovery codes are saved as a .txt file")
	}

	body := "GoPMgr recovery codes for " + username + "\nCreated " + created + "\n\n" + strings.Join(codes, "\n") + "\n"
	if err := writeNewPrivateExport(path, []byte(body)); err != nil {
		return "", err
	}
	return path, nil
}
