// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"path/filepath"
	"testing"

	"gopmgr/internal/db"
)

// archiveWithSettings bundles a plaintext project holding the given
// settings into a .pmba archive and returns the archive's path.
func archiveWithSettings(t *testing.T, settings db.UserSettings) string {
	t.Helper()
	dir := t.TempDir()
	d, err := db.InitDB(filepath.Join(dir, "source.gopmgr"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if _, err := d.UpsertProject(db.Project{Name: "Backed up"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "backup.pmba")
	if err := d.CreateArchivalBundle(archive, nil); err != nil {
		t.Fatal(err)
	}
	return archive
}

// restoredSettings restores archive for a new account and returns the
// restored project's settings and its count of signing-cleared audit rows.
func restoredSettings(t *testing.T, archive string) (db.UserSettings, int) {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("ada", "Ada", "ada-strong-password", false); err != nil {
		t.Fatal(err)
	}
	acceptNoRecoveryCodes(t, app)
	restored, err := app.restoreProjectArchiveFrom(archive, app.currentSession())
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	d, err := db.InitDB(restored.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	settings, err := d.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	var cleared int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'RESTORE_SIGNING_CLEARED'`).Scan(&cleared); err != nil {
		t.Fatal(err)
	}
	return settings, cleared
}

// A backup's signing settings name files, keys and a server chosen by
// whoever made it, so a restored project must not keep them.
func TestRestoreClearsTheBackupsSigningSettings(t *testing.T) {
	kept := db.DefaultUserSettings()
	kept.ExportTheme = "classic"
	kept.DefaultFont = "Noto Sans"
	kept.AgileEnabled = true

	// Each case sets one signing setting alone, so a clear that misses a
	// field fails the case for that field.
	cases := map[string]func(*db.UserSettings){
		"certificate path": func(s *db.UserSettings) { s.CertPath = "/home/other/.ssh/id_ed25519" },
		"PAdES signing": func(s *db.UserSettings) {
			s.SignatureEnabled = true
			s.SignatureMethod = db.SignatureMethodPAdES
		},
		"GnuPG signing": func(s *db.UserSettings) { s.SignatureMethod = db.SignatureMethodGnuPG },
		"GnuPG key":     func(s *db.UserSettings) { s.GPGKeyID = "0xDEADBEEF" },
		// Timestamping is only stored together with PAdES signing.
		"PAdES signing with timestamping": func(s *db.UserSettings) {
			s.SignatureMethod = db.SignatureMethodPAdES
			s.TimestampEnabled = true
		},
		"timestamp endpoint":         func(s *db.UserSettings) { s.TSAEndpoint = "https://tsa.example.test/" },
		"timestamp policy":           func(s *db.UserSettings) { s.TSAPolicyOID = "1.2.3.4" },
		"timestamp root certificate": func(s *db.UserSettings) { s.TSARootCertPath = "/home/other/root.pem" },
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			stored := kept
			set(&stored)
			got, cleared := restoredSettings(t, archiveWithSettings(t, stored))
			if got != kept {
				t.Fatalf("restored settings = %+v, want the signing settings cleared and the rest kept: %+v", got, kept)
			}
			if cleared != 1 {
				t.Fatalf("signing-cleared audit rows = %d, want 1", cleared)
			}
		})
	}
}

// A backup with no signing set up restores unchanged and records nothing.
func TestRestoreKeepsSettingsThatAreNotAboutSigning(t *testing.T) {
	stored := db.DefaultUserSettings()
	stored.ExportTheme = "archival"
	stored.AutoRepair = false
	stored.ComplianceMode = true
	got, cleared := restoredSettings(t, archiveWithSettings(t, stored))
	if got != stored {
		t.Fatalf("restored settings = %+v, want %+v", got, stored)
	}
	if cleared != 0 {
		t.Fatalf("signing-cleared audit rows = %d, want 0", cleared)
	}
}
