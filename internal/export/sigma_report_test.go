// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"os"
	"path/filepath"
	"testing"

	"pmforge/internal/sigma/domain"
)

func TestGenerateSigmaReportWritesPrivateExportArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	outputPath, err := GenerateSigmaReport(
		domain.Project{Title: "Permission Test", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusActive},
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("GenerateSigmaReport: %v", err)
	}

	exportDir := filepath.Join(home, "PMForge", "exports")
	info, err := os.Stat(exportDir)
	if err != nil {
		t.Fatalf("stat export dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("export dir mode = %o, want 700", mode)
	}

	info, err = os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat report: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("report mode = %o, want 600", mode)
	}
}

func TestGenerateSigmaReportTightensExistingExportDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	exportDir := filepath.Join(home, "PMForge", "exports")
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		t.Fatalf("mkdir export dir: %v", err)
	}
	if err := os.Chmod(exportDir, 0o755); err != nil {
		t.Fatalf("chmod broad export dir: %v", err)
	}

	if _, err := GenerateSigmaReport(
		domain.Project{Title: "Existing Directory Test", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusActive},
		nil,
		nil,
		nil,
		nil,
		nil,
	); err != nil {
		t.Fatalf("GenerateSigmaReport: %v", err)
	}

	info, err := os.Stat(exportDir)
	if err != nil {
		t.Fatalf("stat export dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("export dir mode = %o, want 700", mode)
	}
}
