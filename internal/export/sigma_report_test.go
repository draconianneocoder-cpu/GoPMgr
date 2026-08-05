// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"os"
	"path/filepath"
	"testing"

	"gopmgr/internal/sigma/domain"
)

func TestGenerateSigmaReportWritesPrivateExportArtifacts(t *testing.T) {
	// The caller decides outDir (app_sigma.go passes the signed-in user's
	// own <DataDir>/exports); this test uses an arbitrary temp path to
	// prove GenerateSigmaReport honors whatever it's given rather than
	// deriving its own location.
	exportDir := filepath.Join(t.TempDir(), "someuser", "exports")

	outputPath, err := GenerateSigmaReport(
		domain.Project{Title: "Permission Test", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusActive},
		nil,
		nil,
		nil,
		nil,
		nil,
		exportDir,
	)
	if err != nil {
		t.Fatalf("GenerateSigmaReport: %v", err)
	}

	info, err := os.Stat(exportDir)
	if err != nil {
		t.Fatalf("stat export dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("export dir mode = %o, want 700", mode)
	}
	if filepath.Dir(outputPath) != exportDir {
		t.Fatalf("report written to %q, want inside %q", outputPath, exportDir)
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
	exportDir := filepath.Join(t.TempDir(), "someuser", "exports")
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
		exportDir,
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
