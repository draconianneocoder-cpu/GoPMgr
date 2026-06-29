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
	if s.ComplianceMode {
		t.Error("ComplianceMode default: want false, got true")
	}
	if s.SignatureMethod != SignatureMethodNone {
		t.Errorf("SignatureMethod default: got %q, want %q", s.SignatureMethod, SignatureMethodNone)
	}
	if s.GPGKeyID != "" {
		t.Errorf("GPGKeyID default: got %q, want empty", s.GPGKeyID)
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
		SignatureMethod:  SignatureMethodGnuPG,
		GPGKeyID:         "pmforge@example.test",
		DefaultFont:      "Helvetica",
		AgileEnabled:     true,
		ComplianceMode:   true,
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
	if out.SignatureMethod != in.SignatureMethod {
		t.Errorf("SignatureMethod: got %q, want %q", out.SignatureMethod, in.SignatureMethod)
	}
	if out.GPGKeyID != in.GPGKeyID {
		t.Errorf("GPGKeyID: got %q, want %q", out.GPGKeyID, in.GPGKeyID)
	}
	if out.DefaultFont != in.DefaultFont {
		t.Errorf("DefaultFont: got %q, want %q", out.DefaultFont, in.DefaultFont)
	}
	if out.AgileEnabled != in.AgileEnabled {
		t.Errorf("AgileEnabled: got %v, want %v", out.AgileEnabled, in.AgileEnabled)
	}
	if out.ComplianceMode != in.ComplianceMode {
		t.Errorf("ComplianceMode: got %v, want %v", out.ComplianceMode, in.ComplianceMode)
	}
}

func TestSettingsComplianceModeColumnExists(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("settings")
	if err != nil {
		t.Fatalf("columnSet: %v", err)
	}
	if _, ok := cols["compliance_mode"]; !ok {
		t.Error("compliance_mode column not found in settings table after migration")
	}
}

func TestSettingsSignatureMethodColumnsExist(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("settings")
	if err != nil {
		t.Fatalf("columnSet: %v", err)
	}
	for _, name := range []string{"signature_method", "gpg_key_id"} {
		if _, ok := cols[name]; !ok {
			t.Errorf("%s column not found in settings table after migration", name)
		}
	}
}

func TestSaveSettingsDerivesLegacySignatureEnabledFromMethod(t *testing.T) {
	d := newBackupTestDB(t)

	in := DefaultUserSettings()
	in.SignatureMethod = SignatureMethodGnuPG
	if err := d.SaveSettings(in); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	out, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !out.SignatureEnabled {
		t.Fatal("SignatureEnabled: got false, want true for GnuPG signing method")
	}
	if out.SignatureMethod != SignatureMethodGnuPG {
		t.Fatalf("SignatureMethod = %q, want %q", out.SignatureMethod, SignatureMethodGnuPG)
	}
}

func TestGetSettingsBackfillsLegacySignatureEnabledToPAdES(t *testing.T) {
	d := newBackupTestDB(t)
	if _, err := d.Conn.Exec(`
		INSERT INTO settings (id, export_theme, auto_repair, signature_enabled, signature_method)
		VALUES (1, 'modern', 1, 1, '')
	`); err != nil {
		t.Fatalf("force legacy settings row: %v", err)
	}

	out, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if out.SignatureMethod != SignatureMethodPAdES {
		t.Fatalf("SignatureMethod = %q, want %q", out.SignatureMethod, SignatureMethodPAdES)
	}
}
