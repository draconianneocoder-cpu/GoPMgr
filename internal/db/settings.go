// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "database/sql"

// UserSettings is the persisted state of PMForge's global preferences.
// It is keyed at id=1 (singleton row) so SaveSettings can use UPSERT.
//
// CertPath and SignatureEnabled were added when the Digital Signatures
// feature landed; older databases will read defaults because Migrate()
// is additive.
type UserSettings struct {
	DefaultPassword  string `json:"default_password"`
	ExportTheme      string `json:"export_theme"` // "modern" | "classic" | "archival"
	AutoRepair       bool   `json:"auto_repair"`
	CertPath         string `json:"cert_path"`
	SignatureEnabled bool   `json:"signature_enabled"`
	// DefaultFont is the document-export font family (a name from the
	// fonts catalog or a user-imported family). Empty means "use the
	// catalog default".
	DefaultFont string `json:"default_font"`
	// AgileEnabled persists the Software-Dev Pack toggle so the pack
	// state survives project close/reopen without a CLI flag.
	AgileEnabled bool `json:"agile_enabled"`
}

// SaveSettings upserts the singleton settings row. The id is hard-coded
// to 1 (the CHECK constraint on the settings table enforces this).
func (db *Database) SaveSettings(s UserSettings) error {
	const q = `
		INSERT INTO settings
			(id, default_password, export_theme, auto_repair, cert_path, signature_enabled, default_font, agile_enabled)
		VALUES
			(1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			default_password  = excluded.default_password,
			export_theme      = excluded.export_theme,
			auto_repair       = excluded.auto_repair,
			cert_path         = excluded.cert_path,
			signature_enabled = excluded.signature_enabled,
			default_font      = excluded.default_font,
			agile_enabled     = excluded.agile_enabled
	`
	_, err := db.Conn.Exec(q,
		s.DefaultPassword,
		s.ExportTheme,
		boolToInt(s.AutoRepair),
		s.CertPath,
		boolToInt(s.SignatureEnabled),
		s.DefaultFont,
		boolToInt(s.AgileEnabled),
	)
	return err
}

// GetSettings returns the persisted settings, or a sensible default
// (theme=modern, auto-repair=on) when no row has been written yet.
func (db *Database) GetSettings() (UserSettings, error) {
	var (
		s            UserSettings
		autoRepair   int
		signatureOn  int
		agileEnabled int
	)
	err := db.Conn.QueryRow(
		`SELECT default_password, export_theme, auto_repair, cert_path, signature_enabled, default_font, agile_enabled
		 FROM settings WHERE id = 1`,
	).Scan(&s.DefaultPassword, &s.ExportTheme, &autoRepair, &s.CertPath, &signatureOn, &s.DefaultFont, &agileEnabled)

	if err == sql.ErrNoRows {
		return UserSettings{ExportTheme: "modern", AutoRepair: true}, nil
	}
	if err != nil {
		return UserSettings{}, err
	}
	s.AutoRepair = autoRepair != 0
	s.SignatureEnabled = signatureOn != 0
	s.AgileEnabled = agileEnabled != 0
	return s, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
