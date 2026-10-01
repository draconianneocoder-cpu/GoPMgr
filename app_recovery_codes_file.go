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
	return a.saveRecoveryCodes(username, codes, runtime, recoveryCodesText)
}

// recoveryCodesFormat is one way of saving a set of recovery codes. Every
// format shares saveRecoveryCodes' checks and its new-private-file write.
type recoveryCodesFormat struct {
	ext    string // lower case, with the dot
	stem   string // default file name before "<username>-<date><ext>"
	title  string
	filter wailsruntime.FileFilter
	render func(username, created string, codes []string) ([]byte, error)
}

var recoveryCodesText = recoveryCodesFormat{
	ext:    ".txt",
	stem:   "gopmgr-recovery-codes-",
	title:  "Save recovery codes",
	filter: wailsruntime.FileFilter{DisplayName: "Text files", Pattern: "*.txt"},
	render: func(username, created string, codes []string) ([]byte, error) {
		return []byte("GoPMgr recovery codes for " + username + "\nCreated " + created + "\n\n" + strings.Join(codes, "\n") + "\n"), nil
	},
}

func (a *App) saveRecoveryCodes(username string, codes []string, runtime exportDestinationRuntime, format recoveryCodesFormat) (string, error) {
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
	body, err := format.render(username, created, codes)
	if err != nil {
		return "", fmt.Errorf("prepare the recovery codes: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	path, err := runtime.saveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultDirectory:     home,
		DefaultFilename:      format.stem + username + "-" + created + format.ext,
		Title:                format.title,
		Filters:              []wailsruntime.FileFilter{format.filter},
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
	// extension; add it. Upper case is the same extension; any other
	// extension is refused.
	switch ext := filepath.Ext(path); {
	case strings.EqualFold(ext, format.ext):
	case ext == "":
		path += format.ext
	default:
		return "", fmt.Errorf("choose a file name ending in %s", format.ext)
	}

	if err := writeNewPrivateExport(path, body); err != nil {
		return "", err
	}
	return path, nil
}
