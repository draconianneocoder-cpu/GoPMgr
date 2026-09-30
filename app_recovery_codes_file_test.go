// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// recoveryFileApp signs alice in and returns real recovery codes from the
// generator, so the format check is tested against what users actually see.
func recoveryFileApp(t *testing.T) (*App, []string) {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	codes, err := app.IssueRecoveryCodes()
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	return app, codes
}

// fakeSaveDialog answers with path and records the options it was given.
func fakeSaveDialog(path string, got *wailsruntime.SaveDialogOptions, calls *int) exportDestinationRuntime {
	return exportDestinationRuntime{saveFileDialog: func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
		*calls++
		if got != nil {
			*got = opts
		}
		return path, nil
	}}
}

func TestSaveRecoveryCodesFileWritesThePrivateFileTheUserChose(t *testing.T) {
	app, codes := recoveryFileApp(t)
	remembered := t.TempDir()
	app.rememberExportDirectory(remembered)

	dest := filepath.Join(t.TempDir(), "my-codes.txt")
	var opts wailsruntime.SaveDialogOptions
	calls := 0
	before := time.Now().Format("2006-01-02")
	path, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog(dest, &opts, &calls))
	after := time.Now().Format("2006-01-02")
	if err != nil || path != dest {
		t.Fatalf("save = %q, %v; want %q", path, err, dest)
	}
	// The run may cross midnight; the date is one of the two.
	created := before
	if !strings.HasSuffix(opts.DefaultFilename, "-"+before+".txt") {
		created = after
	}

	body, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	want := "GoPMgr recovery codes for alice\nCreated " + created + "\n\n" + strings.Join(codes, "\n") + "\n"
	if string(body) != want {
		t.Fatalf("saved body = %q, want %q", body, want)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved file mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}

	home, _ := os.UserHomeDir()
	if wantName := "gopmgr-recovery-codes-alice-" + created + ".txt"; opts.DefaultDirectory != home || opts.DefaultFilename != wantName {
		t.Fatalf("dialog opened with %q / %q; want the home folder and %s", opts.DefaultDirectory, opts.DefaultFilename, wantName)
	}
	if got := app.requireUser().LastExportDirectory; got != remembered {
		t.Fatalf("remembered export directory changed to %q, want it left at %q", got, remembered)
	}
}

func TestSaveRecoveryCodesFileRefusesAnythingButRecoveryCodes(t *testing.T) {
	app, codes := recoveryFileApp(t)
	nine := append(append([]string{}, codes...), codes[0])
	for _, tc := range []struct {
		name     string
		username string
		codes    []string
	}{
		{"no codes", "alice", nil},
		{"more than eight codes", "alice", nine},
		{"a code without its dash", "alice", []string{strings.ReplaceAll(codes[0], "-", "")}},
		{"arbitrary text", "alice", []string{"secret notes"}},
		{"an invalid username", "../alice", codes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			if _, err := app.saveRecoveryCodesFileWithRuntime(tc.username, tc.codes, fakeSaveDialog(filepath.Join(t.TempDir(), "x.txt"), nil, &calls)); err == nil {
				t.Fatal("save succeeded, want a refusal")
			}
			if calls != 0 {
				t.Fatal("the save dialog opened for a refused request")
			}
		})
	}

	t.Run("not signed in", func(t *testing.T) {
		if err := app.Logout(); err != nil {
			t.Fatalf("Logout: %v", err)
		}
		calls := 0
		if _, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog(filepath.Join(t.TempDir(), "x.txt"), nil, &calls)); err == nil || calls != 0 {
			t.Fatalf("save while signed out = %v with %d dialogs, want a refusal and no dialog", err, calls)
		}
	})
}

func TestSaveRecoveryCodesFileCancelWritesNothing(t *testing.T) {
	app, codes := recoveryFileApp(t)
	calls := 0
	if _, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog("", nil, &calls)); !errors.Is(err, ErrExportCancelled) {
		t.Fatalf("cancelled save = %v, want ErrExportCancelled", err)
	}
}

func TestSaveRecoveryCodesFileNeverOverwritesAFile(t *testing.T) {
	app, codes := recoveryFileApp(t)
	dest := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(dest, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	calls := 0
	if _, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog(dest, nil, &calls)); !errors.Is(err, ErrExportDestinationExists) {
		t.Fatalf("save over an existing file = %v, want ErrExportDestinationExists", err)
	}
	if body, _ := os.ReadFile(dest); string(body) != "keep me" {
		t.Fatalf("existing file was changed to %q", body)
	}
}

// Renewal shows codes from PrepareRecoveryCodes before they are confirmed;
// the save method must accept that set too, not only the first one.
func TestSaveRecoveryCodesFileAcceptsRenewedCodes(t *testing.T) {
	app, _ := recoveryFileApp(t)
	renewed, err := app.PrepareRecoveryCodes("correct horse battery staple")
	if err != nil {
		t.Fatalf("PrepareRecoveryCodes: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "renewed.txt")
	calls := 0
	if _, err := app.saveRecoveryCodesFileWithRuntime("alice", renewed, fakeSaveDialog(dest, nil, &calls)); err != nil {
		t.Fatalf("save renewed codes: %v", err)
	}
	body, err := os.ReadFile(dest)
	if err != nil || !strings.HasSuffix(string(body), "\n\n"+strings.Join(renewed, "\n")+"\n") {
		t.Fatalf("renewed file = %q, %v; want it to end with the renewed codes", body, err)
	}
}

func TestSaveRecoveryCodesFileExtension(t *testing.T) {
	app, codes := recoveryFileApp(t)
	dir := t.TempDir()
	calls := 0
	path, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog(filepath.Join(dir, "codes"), nil, &calls))
	if err != nil || path != filepath.Join(dir, "codes.txt") {
		t.Fatalf("save without an extension = %q, %v; want codes.txt", path, err)
	}
	upper := filepath.Join(dir, "Backup.TXT")
	if path, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog(upper, nil, &calls)); err != nil || path != upper {
		t.Fatalf("save as .TXT = %q, %v; want %q unchanged", path, err, upper)
	}
	if _, err := app.saveRecoveryCodesFileWithRuntime("alice", codes, fakeSaveDialog(filepath.Join(dir, "codes.pdf"), nil, &calls)); err == nil {
		t.Fatal("save as .pdf succeeded, want a refusal")
	}
	if _, err := os.Stat(filepath.Join(dir, "codes.pdf")); !os.IsNotExist(err) {
		t.Fatalf("a .pdf file was written (stat err %v)", err)
	}
}

func TestSaveRecoveryCodesFileNeedsTheWindow(t *testing.T) {
	app, codes := recoveryFileApp(t)
	if _, err := app.SaveRecoveryCodesFile("alice", codes); err == nil {
		t.Fatal("SaveRecoveryCodesFile with no window succeeded, want a refusal")
	}
}
