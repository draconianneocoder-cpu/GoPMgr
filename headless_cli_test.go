// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopmgr/internal/cli"
	"gopmgr/internal/crypto"
	"gopmgr/internal/db"
	"gopmgr/internal/export"
	"gopmgr/internal/money"
)

// seedHeadlessProject creates a plaintext project DB with one project row
// and returns an open handle.
func seedHeadlessProject(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "project.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(db.Project{Name: "Headless Demo", Status: "planning", Phase: "planning"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestParseHeadlessFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    export.ExportFormat
		wantErr bool
	}{
		{"", export.FormatPDF, false},
		{"pdf", export.FormatPDF, false},
		{"DOCX", export.FormatDOCX, false},
		{"odt", export.FormatODT, false},
		{"xlsx", export.FormatXLSX, false},
		{"csv", export.FormatCSV, false},
		{"html", export.FormatHTML, false},
		{"mspdi", export.FormatMSPDI, false},
		{"xml", export.FormatMSPDI, false},
		{"docx ", export.FormatDOCX, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := parseHeadlessFormat(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseHeadlessFormat(%q) = %q, want error", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("parseHeadlessFormat(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestRunHeadlessExportWritesFile(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.csv")
	cfg := &cli.Config{ExportPath: out, ExportFormat: "csv"}
	if err := runHeadlessExport(cfg, d); err != nil {
		t.Fatalf("runHeadlessExport: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("export file is empty")
	}
}

// TestRunHeadlessExportPropagatesEVMOverflow proves runHeadlessExport
// checks and propagates kernel.ComputeEVM's overflow error rather than
// silently writing an export file with missing or wrong EVM data, for the
// CLI headless path (a separate call site from the Wails-bound schedule
// and combined-report exports covered in report_evm_test.go).
func TestRunHeadlessExportPropagatesEVMOverflow(t *testing.T) {
	d := seedHeadlessProject(t)
	proj, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	proj.StartDate = "2026-06-01"
	proj.CountryCode = "US"
	if _, err := d.UpsertProject(proj); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := d.SaveChart(db.Chart{
		ProjectID: proj.ID,
		Kind:      "cpm",
		Title:     "Overflow Schedule",
		Data: `{"nodes":[
			{"id":"A","duration":1,"budgeted_cost_minor_units":9223372036854775807},
			{"id":"B","duration":1,"budgeted_cost_minor_units":1}
		],"edges":[]}`,
	}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	out := filepath.Join(t.TempDir(), "schedule.csv")
	cfg := &cli.Config{ExportPath: out, ExportFormat: "csv"}
	if err := runHeadlessExport(cfg, d); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("runHeadlessExport error = %v, want ErrOverflow", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("runHeadlessExport wrote an export file despite the EVM overflow error")
	}
}

func TestRunHeadlessExportEncrypts(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.csv.enc")
	const pw = "correct horse battery staple"
	t.Setenv("PMF_TEST_PW", pw)
	cfg := &cli.Config{
		ExportPath:    out,
		ExportFormat:  "csv",
		EncryptExport: true,
		PasswordEnv:   "PMF_TEST_PW",
	}
	if err := runHeadlessExport(cfg, d); err != nil {
		t.Fatalf("runHeadlessExport encrypted: %v", err)
	}
	blob, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read encrypted export: %v", err)
	}
	plain, err := crypto.DecryptBuffer(blob, pw)
	if err != nil {
		t.Fatalf("decrypt export: %v", err)
	}
	if len(plain) == 0 {
		t.Fatal("decrypted export is empty")
	}
}

func TestRunHeadlessExportEncryptRequiresPassword(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.csv.enc")
	cfg := &cli.Config{ExportPath: out, ExportFormat: "csv", EncryptExport: true}
	if err := runHeadlessExport(cfg, d); err == nil {
		t.Fatal("runHeadlessExport with --encrypt but no --password-env should error")
	}
}

func TestPrintHeadlessStatsSucceeds(t *testing.T) {
	d := seedHeadlessProject(t)
	if err := printHeadlessStats(d); err != nil {
		t.Fatalf("printHeadlessStats: %v", err)
	}
}

// TestRunHeadlessExportFormats runs the export through every format the CLI
// accepts and asserts each renderer writes a non-empty file, exercising the
// full render path (the 2026-07-05/07-11 reviews flagged the renderers as
// not dynamically covered).
func TestRunHeadlessExportFormats(t *testing.T) {
	for _, format := range []string{"pdf", "docx", "odt", "xlsx", "csv", "html", "mspdi"} {
		t.Run(format, func(t *testing.T) {
			d := seedHeadlessProject(t)
			out := filepath.Join(t.TempDir(), "schedule."+format)
			cfg := &cli.Config{ExportPath: out, ExportFormat: format}
			if err := runHeadlessExport(cfg, d); err != nil {
				t.Fatalf("runHeadlessExport(%s): %v", format, err)
			}
			info, err := os.Stat(out)
			if err != nil {
				t.Fatalf("stat %s export: %v", format, err)
			}
			if info.Size() == 0 {
				t.Fatalf("%s export is empty", format)
			}
		})
	}
}

// TestRunHeadlessExportRejectsBadFormat confirms an unsupported --format is
// rejected before anything is written.
func TestRunHeadlessExportRejectsBadFormat(t *testing.T) {
	d := seedHeadlessProject(t)
	out := filepath.Join(t.TempDir(), "schedule.bogus")
	cfg := &cli.Config{ExportPath: out, ExportFormat: "bogus"}
	if err := runHeadlessExport(cfg, d); err == nil {
		t.Fatal("runHeadlessExport accepted an unsupported format")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a file was written for a rejected format: %v", err)
	}
}

// TestHeadlessExportPassword covers every branch of the env-var password
// resolver: a missing --password-env, a named-but-unset variable, and the
// happy path. The password must come from the environment, never a flag.
func TestHeadlessExportPassword(t *testing.T) {
	t.Run("missing password-env", func(t *testing.T) {
		if _, err := headlessExportPassword(&cli.Config{PasswordEnv: "   "}); err == nil {
			t.Fatal("blank --password-env should error")
		}
	})
	t.Run("named variable unset", func(t *testing.T) {
		if _, err := headlessExportPassword(&cli.Config{PasswordEnv: "PMF_DEFINITELY_UNSET"}); err == nil {
			t.Fatal("unset password variable should error")
		}
	})
	t.Run("empty variable", func(t *testing.T) {
		t.Setenv("PMF_EMPTY_PW", "")
		if _, err := headlessExportPassword(&cli.Config{PasswordEnv: "PMF_EMPTY_PW"}); err == nil {
			t.Fatal("empty password variable should error")
		}
	})
	t.Run("resolves from environment", func(t *testing.T) {
		const pw = "correct horse battery staple"
		t.Setenv("PMF_SET_PW", pw)
		got, err := headlessExportPassword(&cli.Config{PasswordEnv: "PMF_SET_PW"})
		if err != nil {
			t.Fatalf("headlessExportPassword: %v", err)
		}
		if got != pw {
			t.Fatalf("password = %q, want %q", got, pw)
		}
	})
}

// TestOpenHeadlessDBUnencrypted confirms a plaintext project opens directly,
// with no --username/--password-env required. (The encrypted happy path and
// the missing-username case are covered in headless_encryption_test.go; this
// adds the plaintext branch and the remaining credential-failure modes.)
func TestOpenHeadlessDBUnencrypted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := db.InitDB(path)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	_ = d.Close()

	got, err := openHeadlessDB(&cli.Config{ProjectPath: path})
	if err != nil {
		t.Fatalf("openHeadlessDB(unencrypted): %v", err)
	}
	_ = got.Close()
}

// TestOpenHeadlessDBEncryptedCredentialFailures locks in that an encrypted
// project fails closed when the password cannot be resolved or is wrong:
// a missing --password-env, a named-but-unset variable, and the wrong
// password. Filesystem access alone must not decrypt a project.
func TestOpenHeadlessDBEncryptedCredentialFailures(t *testing.T) {
	path, _ := createHeadlessEncryptedProject(t) // helper uses username "alice"

	t.Run("missing password-env", func(t *testing.T) {
		if _, err := openHeadlessDB(&cli.Config{ProjectPath: path, Username: "alice"}); err == nil {
			t.Fatal("opened encrypted project without --password-env")
		}
	})
	t.Run("password variable unset", func(t *testing.T) {
		if _, err := openHeadlessDB(&cli.Config{ProjectPath: path, Username: "alice", PasswordEnv: "PMF_HL_UNSET"}); err == nil {
			t.Fatal("opened encrypted project with an unset password variable")
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		t.Setenv("PMF_HL_BAD", "not the password")
		if _, err := openHeadlessDB(&cli.Config{ProjectPath: path, Username: "alice", PasswordEnv: "PMF_HL_BAD"}); err == nil {
			t.Fatal("opened encrypted project with the wrong password")
		}
	})
}
