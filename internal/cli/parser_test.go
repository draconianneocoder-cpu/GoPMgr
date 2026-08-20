// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
)

// parseArgs resets the stdlib's global flag.CommandLine (ParseFlags reads
// os.Args and calls flag.Parse(), which is package-level state in the
// standard library, not something this package owns) and runs ParseFlags
// against the given argv. t.Cleanup restores both so later tests in this
// package -- or a future test added after this one -- see a clean flag set.
func parseArgs(t *testing.T, args ...string) *Config {
	t.Helper()
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})
	os.Args = append([]string{"gopmgr"}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	return ParseFlags()
}

func TestVersion_NonEmpty(t *testing.T) {
	if Version == "" {
		t.Error("Version constant must not be empty")
	}
}

func TestPrintVersion_ContainsBanner(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	PrintVersion()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "GoPMgr") {
		t.Errorf("output %q does not contain 'GoPMgr'", out)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("output %q does not contain Version %q", out, Version)
	}
	if !strings.Contains(out, "GPL") {
		t.Errorf("output %q does not contain 'GPL'", out)
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := &Config{}
	// Zero value represents unset flags — verify the bool fields default false
	// and string fields default empty (i.e., the struct is coherent as a zero value).
	if cfg.ShowVersion {
		t.Error("ShowVersion should default false")
	}
	if cfg.DebugMode {
		t.Error("DebugMode should default false")
	}
	if cfg.ExportFormat != "" {
		t.Errorf("ExportFormat should default empty, got %q", cfg.ExportFormat)
	}
	if cfg.ProjectPath != "" {
		t.Errorf("ProjectPath should default empty, got %q", cfg.ProjectPath)
	}
	if cfg.Username != "" {
		t.Errorf("Username should default empty, got %q", cfg.Username)
	}
	if cfg.PasswordEnv != "" {
		t.Errorf("PasswordEnv should default empty, got %q", cfg.PasswordEnv)
	}
}

func TestParseFlags_NoArgsUsesDefaults(t *testing.T) {
	cfg := parseArgs(t)
	if cfg.ShowVersion || cfg.CheckOnly ||
		cfg.Repair || cfg.UpdateCheck || cfg.SchemaDump || cfg.ShowStats ||
		cfg.Vacuum || cfg.EncryptExport || cfg.DebugMode {
		t.Errorf("expected every bool flag false with no args, got %+v", cfg)
	}
	if cfg.ExportAuditPath != "" || cfg.Username != "" || cfg.PasswordEnv != "" ||
		cfg.ExportPath != "" || cfg.ProjectPath != "" {
		t.Errorf("expected every unset string flag empty with no args, got %+v", cfg)
	}
	// The one flag with a non-empty default.
	if cfg.ExportFormat != "pdf" {
		t.Errorf("ExportFormat default = %q, want %q", cfg.ExportFormat, "pdf")
	}
}

func TestParseFlags_BoolFlags(t *testing.T) {
	cfg := parseArgs(t,
		"-version", "-check", "-repair",
		"-update", "-schema-dump", "-stats", "-vacuum", "-encrypt", "-debug",
	)
	cases := map[string]bool{
		"ShowVersion":   cfg.ShowVersion,
		"CheckOnly":     cfg.CheckOnly,
		"Repair":        cfg.Repair,
		"UpdateCheck":   cfg.UpdateCheck,
		"SchemaDump":    cfg.SchemaDump,
		"ShowStats":     cfg.ShowStats,
		"Vacuum":        cfg.Vacuum,
		"EncryptExport": cfg.EncryptExport,
		"DebugMode":     cfg.DebugMode,
	}
	for name, got := range cases {
		if !got {
			t.Errorf("%s = false, want true when its flag is passed", name)
		}
	}
}

func TestParseFlags_StringFlags(t *testing.T) {
	cfg := parseArgs(t,
		"-export-audit", "/tmp/audit.csv",
		"-username", "alice",
		"-password-env", "GOPMGR_PW",
		"-format", "docx",
		"-export", "/tmp/out.docx",
	)
	if cfg.ExportAuditPath != "/tmp/audit.csv" {
		t.Errorf("ExportAuditPath = %q, want %q", cfg.ExportAuditPath, "/tmp/audit.csv")
	}
	if cfg.Username != "alice" {
		t.Errorf("Username = %q, want %q", cfg.Username, "alice")
	}
	if cfg.PasswordEnv != "GOPMGR_PW" {
		t.Errorf("PasswordEnv = %q, want %q", cfg.PasswordEnv, "GOPMGR_PW")
	}
	if cfg.ExportFormat != "docx" {
		t.Errorf("ExportFormat = %q, want %q", cfg.ExportFormat, "docx")
	}
	if cfg.ExportPath != "/tmp/out.docx" {
		t.Errorf("ExportPath = %q, want %q", cfg.ExportPath, "/tmp/out.docx")
	}
}

func TestParseFlags_PositionalProjectPath(t *testing.T) {
	cfg := parseArgs(t, "-stats", "/Users/alice/My Plan.gopmgr")
	if cfg.ProjectPath != "/Users/alice/My Plan.gopmgr" {
		t.Errorf("ProjectPath = %q, want %q", cfg.ProjectPath, "/Users/alice/My Plan.gopmgr")
	}
	if !cfg.ShowStats {
		t.Error("ShowStats should still be true alongside a positional arg")
	}
}

func TestParseFlags_NoPositionalArgLeavesProjectPathEmpty(t *testing.T) {
	cfg := parseArgs(t, "-check")
	if cfg.ProjectPath != "" {
		t.Errorf("ProjectPath = %q, want empty when no positional arg is given", cfg.ProjectPath)
	}
}

func TestParseFlags_OnlyFirstPositionalArgIsUsed(t *testing.T) {
	// Pins that ProjectPath is flag.Arg(0) -- the first positional arg --
	// not the last, when more than one is given. Passing two args here
	// means this test alone can't distinguish "uses the first" from "uses
	// the last one at index 0" under every possible mutation (e.g. it
	// stays green under a flag.NArg() > 0 -> > 1 mutation, since both
	// guards are satisfied by two args; TestParseFlags_NoPositionalArgLeavesProjectPathEmpty
	// and TestParseFlags_PositionalProjectPath catch that one instead).
	cfg := parseArgs(t, "first.gopmgr", "second.gopmgr")
	if cfg.ProjectPath != "first.gopmgr" {
		t.Errorf("ProjectPath = %q, want %q (flag.Arg(0), not the last arg)", cfg.ProjectPath, "first.gopmgr")
	}
}
