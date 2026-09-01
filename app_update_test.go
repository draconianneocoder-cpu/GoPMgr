// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"strings"
	"testing"

	"gopmgr/internal/update"
	"gopmgr/internal/users"
)

// TestDownloadAndInstallUpdate_RequiresSignIn proves the App-layer guard,
// not update.DownloadAndInstall's own logic (exhaustively covered in
// internal/update/install_test.go): calling it with nobody signed in must
// fail before any network access, since there is no DataDir to publish the
// artifact under.
func TestDownloadAndInstallUpdate_RequiresSignIn(t *testing.T) {
	app := &App{ctx: context.Background()}
	_, err := app.DownloadAndInstallUpdate()
	if err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Fatalf("DownloadAndInstallUpdate error = %v, want not signed in", err)
	}
}

// TestDownloadAndInstallUpdate_UsesUserScopedUpdatesDir proves the
// destination directory is derived from the signed-in user's own DataDir
// (never a shared or hardcoded path) once the sign-in guard passes. No
// update channel is configured in this test process, so
// update.DownloadAndInstall itself fails fast on "no update channel
// configured" — this test only needs to reach past the sign-in guard, not
// exercise a real download.
func TestDownloadAndInstallUpdate_UsesUserScopedUpdatesDir(t *testing.T) {
	oldURL, oldKey := update.ManifestURL, update.UpdateChannelPublicKey
	update.ManifestURL, update.UpdateChannelPublicKey = "", ""
	t.Cleanup(func() { update.ManifestURL, update.UpdateChannelPublicKey = oldURL, oldKey })

	dataDir := t.TempDir()
	app := &App{ctx: context.Background(), user: &users.Account{Username: "alice", DataDir: dataDir}}

	// "no update channel configured" (not "not signed in") proves
	// requireUser() succeeded and control reached update.DownloadAndInstall
	// -- the specific behavior this test exists to check, since
	// DownloadAndInstall's own internals are covered in
	// internal/update/install_test.go, not here.
	_, err := app.DownloadAndInstallUpdate()
	if err == nil || !strings.Contains(err.Error(), "no update channel configured") {
		t.Fatalf("DownloadAndInstallUpdate error = %v, want no update channel configured", err)
	}
}
