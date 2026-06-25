// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"pmforge/internal/agile"
	"pmforge/internal/db"
	"pmforge/internal/users"
)

// TestDefaultAppSettingsEnablesAutoSave guards the brand-new-user default:
// auto-save on at 60s. If this flips to 0 (off) by accident, new users would
// silently lose the timed safety net.
func TestDefaultAppSettingsEnablesAutoSave(t *testing.T) {
	if got := defaultAppSettings().AutoSaveSeconds; got != 60 {
		t.Fatalf("default AutoSaveSeconds = %d, want 60", got)
	}
}

// TestLoadGlobalAppSettingsFallsBackToDefaults verifies that when no settings
// file is reachable (here: no signed-in user, so appSettingsPath errors)
// loadGlobalAppSettings hands back the defaults rather than a zero struct that
// would read as auto-save off.
func TestLoadGlobalAppSettingsFallsBackToDefaults(t *testing.T) {
	app := &App{}
	s := app.loadGlobalAppSettings()
	if s.AutoSaveSeconds != 60 {
		t.Fatalf("fallback AutoSaveSeconds = %d, want 60", s.AutoSaveSeconds)
	}
}

func TestResetAppSettingsRemovesSavedPreferences(t *testing.T) {
	dir := t.TempDir()
	app := &App{user: &users.Account{Username: "alice", DataDir: dir}}
	if err := app.SaveAppSettings(AppSettings{
		DefaultFont:     "Custom",
		DefaultTheme:    "archival",
		AppTheme:        "light",
		AutoSaveSeconds: 15,
	}); err != nil {
		t.Fatalf("SaveAppSettings: %v", err)
	}

	got, err := app.ResetAppSettings()
	if err != nil {
		t.Fatalf("ResetAppSettings: %v", err)
	}
	if got != defaultAppSettings() {
		t.Fatalf("ResetAppSettings = %+v, want %+v", got, defaultAppSettings())
	}
	if _, err := os.Stat(filepath.Join(dir, "app-settings.json")); !os.IsNotExist(err) {
		t.Fatalf("app-settings.json still exists or stat failed: %v", err)
	}
}

func TestResetProjectSettingsRestoresDefaults(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "project.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	custom := db.UserSettings{
		ExportTheme:      "archival",
		AutoRepair:       false,
		CertPath:         "/tmp/cert.p12",
		SignatureEnabled: true,
		DefaultFont:      "Custom",
		AgileEnabled:     true,
	}
	if err := d.SaveSettings(custom); err != nil {
		t.Fatalf("SaveSettings custom: %v", err)
	}
	agile.PackEnabled.Store(true)
	app := &App{db: d}

	got, err := app.ResetProjectSettings()
	if err != nil {
		t.Fatalf("ResetProjectSettings: %v", err)
	}
	if want := db.DefaultUserSettings(); got != want {
		t.Fatalf("ResetProjectSettings = %+v, want %+v", got, want)
	}
	persisted, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after reset: %v", err)
	}
	if want := db.DefaultUserSettings(); persisted != want {
		t.Fatalf("persisted settings = %+v, want %+v", persisted, want)
	}
	if agile.PackEnabled.Load() {
		t.Fatal("Agile cache stayed enabled after reset")
	}
}
