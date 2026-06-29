// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "database/sql"

const (
	SignatureMethodNone  = "none"
	SignatureMethodPAdES = "pades"
	SignatureMethodGnuPG = "gpg"
)

// UserSettings is the persisted state of PMForge's global preferences.
// It is keyed at id=1 (singleton row) so SaveSettings can use UPSERT.
//
// CertPath and SignatureEnabled were added when the Digital Signatures
// feature landed. SignatureMethod/GPGKeyID extend that boolean into an
// explicit signing-policy choice while preserving older databases.
type UserSettings struct {
	DefaultPassword  string `json:"default_password"`
	ExportTheme      string `json:"export_theme"` // "modern" | "classic" | "archival"
	AutoRepair       bool   `json:"auto_repair"`
	CertPath         string `json:"cert_path"`
	SignatureEnabled bool   `json:"signature_enabled"`
	SignatureMethod  string `json:"signature_method"`
	GPGKeyID         string `json:"gpg_key_id"`
	// DefaultFont is the document-export font family (a name from the
	// fonts catalog or a user-imported family). Empty means "use the
	// catalog default".
	DefaultFont string `json:"default_font"`
	// AgileEnabled persists the Software-Dev Pack toggle so the pack
	// state survives project close/reopen without a CLI flag.
	AgileEnabled bool `json:"agile_enabled"`
	// ComplianceMode enables fail-closed checks such as audit hash-chain
	// verification when the project file is opened.
	ComplianceMode bool `json:"compliance_mode"`
}

// DefaultUserSettings is the canonical project-settings reset target.
func DefaultUserSettings() UserSettings {
	return UserSettings{ExportTheme: "modern", AutoRepair: true, SignatureMethod: SignatureMethodNone}
}

// SaveSettings upserts the singleton settings row. The id is hard-coded
// to 1 (the CHECK constraint on the settings table enforces this).
func (db *Database) SaveSettings(s UserSettings) error {
	s = normalizeUserSettings(s)
	const q = `
		INSERT INTO settings
			(id, default_password, export_theme, auto_repair, cert_path, signature_enabled, signature_method, gpg_key_id, default_font, agile_enabled, compliance_mode)
		VALUES
			(1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			default_password  = excluded.default_password,
			export_theme      = excluded.export_theme,
			auto_repair       = excluded.auto_repair,
			cert_path         = excluded.cert_path,
			signature_enabled = excluded.signature_enabled,
			signature_method  = excluded.signature_method,
			gpg_key_id        = excluded.gpg_key_id,
			default_font      = excluded.default_font,
			agile_enabled     = excluded.agile_enabled,
			compliance_mode   = excluded.compliance_mode
	`
	_, err := db.Conn.Exec(q,
		s.DefaultPassword,
		s.ExportTheme,
		boolToInt(s.AutoRepair),
		s.CertPath,
		boolToInt(s.SignatureEnabled),
		s.SignatureMethod,
		s.GPGKeyID,
		s.DefaultFont,
		boolToInt(s.AgileEnabled),
		boolToInt(s.ComplianceMode),
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
		compliance   int
	)
	err := db.Conn.QueryRow(
		`SELECT default_password, export_theme, auto_repair, cert_path, signature_enabled, signature_method, gpg_key_id, default_font, agile_enabled, compliance_mode
		 FROM settings WHERE id = 1`,
	).Scan(&s.DefaultPassword, &s.ExportTheme, &autoRepair, &s.CertPath, &signatureOn, &s.SignatureMethod, &s.GPGKeyID, &s.DefaultFont, &agileEnabled, &compliance)

	if err == sql.ErrNoRows {
		return DefaultUserSettings(), nil
	}
	if err != nil {
		return UserSettings{}, err
	}
	s.AutoRepair = autoRepair != 0
	s.SignatureEnabled = signatureOn != 0
	s.AgileEnabled = agileEnabled != 0
	s.ComplianceMode = compliance != 0
	return normalizeUserSettings(s), nil
}

func normalizeUserSettings(s UserSettings) UserSettings {
	switch s.SignatureMethod {
	case SignatureMethodNone, SignatureMethodPAdES, SignatureMethodGnuPG:
	case "":
		if s.SignatureEnabled {
			s.SignatureMethod = SignatureMethodPAdES
		} else {
			s.SignatureMethod = SignatureMethodNone
		}
	default:
		s.SignatureMethod = SignatureMethodNone
	}
	s.SignatureEnabled = s.SignatureMethod != SignatureMethodNone
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
