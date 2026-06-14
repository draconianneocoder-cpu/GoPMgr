// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli parses GNU-style command-line flags for PMForge.
//
// Every flag corresponds to a workflow described in the V1 blueprint.
// New flags MUST be added to Config so headless modes remain scriptable.
package cli

import (
	"flag"
	"fmt"
	"os"
)

// Version is the canonical PMForge version string.
//
// IMPORTANT: scripts/check-release.sh greps this constant and the
// `productVersion` key in wails.json. Keep them in sync.
const Version = "1.1.0-V1-Expansion"

// Config holds every flag the CLI accepts. Keep field order grouped by
// concern (info / packs / maintenance / export / debug / positional).
type Config struct {
	// Info flags
	ShowVersion bool

	// Pack toggles
	AdminPack       bool
	SoftwareDevPack bool

	// Maintenance
	CheckOnly       bool
	Repair          bool
	UpdateCheck     bool
	ExportAuditPath string
	SchemaDump      bool
	ShowStats       bool
	Vacuum          bool
	Username        string
	PasswordEnv     string

	// Export / interchange (added per V1 Extended scope)
	ExportFormat  string // pdf | odt | docx | xlsx | mspdi
	ExportPath    string
	EncryptExport bool

	// Debugging
	DebugMode bool

	// Positional argument: project file path (.pmforge)
	ProjectPath string
}

// ParseFlags reads os.Args and returns a populated Config.
// Callers are expected to PrintVersion() and exit when ShowVersion is set.
func ParseFlags() *Config {
	cfg := &Config{}

	// Info
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Display version information")

	// Packs
	flag.BoolVar(&cfg.AdminPack, "admin-pack", false, "Enable Admin Pack")
	flag.BoolVar(&cfg.SoftwareDevPack, "software-dev-pack", false, "Enable Software Dev Pack")

	// Maintenance
	flag.BoolVar(&cfg.CheckOnly, "check", false, "Run integrity check and exit")
	flag.BoolVar(&cfg.Repair, "repair", false, "Run self-healing repair workflow")
	flag.BoolVar(&cfg.UpdateCheck, "update", false, "Check for updates")
	flag.StringVar(&cfg.ExportAuditPath, "export-audit", "", "Export audit log to CSV at the given path")
	flag.BoolVar(&cfg.SchemaDump, "schema-dump", false, "Dump SQL schema to stdout")
	flag.BoolVar(&cfg.ShowStats, "stats", false, "Show project statistics")
	flag.BoolVar(&cfg.Vacuum, "vacuum", false, "Optimize database (VACUUM)")
	flag.StringVar(&cfg.Username, "username", "", "Username for encrypted headless maintenance")
	flag.StringVar(&cfg.PasswordEnv, "password-env", "", "Environment variable containing the password for encrypted headless maintenance")

	// Export
	flag.StringVar(&cfg.ExportFormat, "format", "pdf", "Format for headless export (pdf, odt, docx, xlsx, mspdi)")
	flag.StringVar(&cfg.ExportPath, "export", "", "Destination path for headless export")
	flag.BoolVar(&cfg.EncryptExport, "encrypt", false, "Encrypt the exported file with the user's default password")

	// Debug
	flag.BoolVar(&cfg.DebugMode, "debug", false, "Enable verbose error reporting and logging")

	flag.Parse()

	if flag.NArg() > 0 {
		cfg.ProjectPath = flag.Arg(0)
	}

	return cfg
}

// PrintVersion writes the canonical GPL-style version banner to stdout.
func PrintVersion() {
	fmt.Fprintf(os.Stdout,
		"PMForge %s\nCopyright (C) 2026 The PMForge Contributors\nLicense GPLv3+: GNU GPL version 3 or later <https://gnu.org/licenses/gpl.html>\nThis is free software: you are free to change and redistribute it.\nThere is NO WARRANTY, to the extent permitted by law.\n",
		Version,
	)
}
