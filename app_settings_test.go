// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

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
