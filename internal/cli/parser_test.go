// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

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

	if !strings.Contains(out, "PMForge") {
		t.Errorf("output %q does not contain 'PMForge'", out)
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
}
