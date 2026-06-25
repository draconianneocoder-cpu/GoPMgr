// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

func TestGetSettingsDefaults(t *testing.T) {
	d := newBackupTestDB(t)

	s, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.ExportTheme != "modern" {
		t.Errorf("ExportTheme default: got %q, want %q", s.ExportTheme, "modern")
	}
	if !s.AutoRepair {
		t.Error("AutoRepair default: want true, got false")
	}
	if s.AgileEnabled {
		t.Error("AgileEnabled default: want false, got true")
	}
}

func TestDefaultUserSettingsMatchesEmptyProjectDefaults(t *testing.T) {
	d := newBackupTestDB(t)

	got, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if want := DefaultUserSettings(); got != want {
		t.Fatalf("GetSettings defaults = %+v, want %+v", got, want)
	}
}

func TestSaveSettingsAgileEnabledRoundtrip(t *testing.T) {
	d := newBackupTestDB(t)

	base, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings (base): %v", err)
	}

	base.AgileEnabled = true
	if err := d.SaveSettings(base); err != nil {
		t.Fatalf("SaveSettings (enable): %v", err)
	}

	got, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings (after enable): %v", err)
	}
	if !got.AgileEnabled {
		t.Error("AgileEnabled: want true after save, got false")
	}

	got.AgileEnabled = false
	if err := d.SaveSettings(got); err != nil {
		t.Fatalf("SaveSettings (disable): %v", err)
	}

	got2, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings (after disable): %v", err)
	}
	if got2.AgileEnabled {
		t.Error("AgileEnabled: want false after save, got true")
	}
}

func TestSettingsAgileEnabledColumnExists(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("settings")
	if err != nil {
		t.Fatalf("columnSet: %v", err)
	}
	if _, ok := cols["agile_enabled"]; !ok {
		t.Error("agile_enabled column not found in settings table after migration")
	}
}

func TestSaveSettingsPreservesAllFields(t *testing.T) {
	d := newBackupTestDB(t)

	in := UserSettings{
		DefaultPassword:  "",
		ExportTheme:      "archival",
		AutoRepair:       false,
		CertPath:         "/some/cert.p12",
		SignatureEnabled: true,
		DefaultFont:      "Helvetica",
		AgileEnabled:     true,
	}
	if err := d.SaveSettings(in); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	out, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}

	if out.ExportTheme != in.ExportTheme {
		t.Errorf("ExportTheme: got %q, want %q", out.ExportTheme, in.ExportTheme)
	}
	if out.AutoRepair != in.AutoRepair {
		t.Errorf("AutoRepair: got %v, want %v", out.AutoRepair, in.AutoRepair)
	}
	if out.CertPath != in.CertPath {
		t.Errorf("CertPath: got %q, want %q", out.CertPath, in.CertPath)
	}
	if out.SignatureEnabled != in.SignatureEnabled {
		t.Errorf("SignatureEnabled: got %v, want %v", out.SignatureEnabled, in.SignatureEnabled)
	}
	if out.DefaultFont != in.DefaultFont {
		t.Errorf("DefaultFont: got %q, want %q", out.DefaultFont, in.DefaultFont)
	}
	if out.AgileEnabled != in.AgileEnabled {
		t.Errorf("AgileEnabled: got %v, want %v", out.AgileEnabled, in.AgileEnabled)
	}
}
