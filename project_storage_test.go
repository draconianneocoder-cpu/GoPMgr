// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectDisplayNameStripsTimestamp(t *testing.T) {
	if got := projectDisplayName("20260615-153000-My Plan"); got != "My Plan" {
		t.Fatalf("display name = %q, want %q", got, "My Plan")
	}
	if got := projectDisplayName("legacyish"); got != "legacyish" {
		t.Fatalf("non-prefixed name should pass through, got %q", got)
	}
}

// TestEnumerateProjectsSupportsBothLayouts proves the listing helper finds
// both legacy flat ".pmforge" files and the current "<id>/project.pmforge"
// subfolders, and ignores unrelated subfolders.
func TestEnumerateProjectsSupportsBothLayouts(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "Legacy Project.pmforge"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "20260615-153000-New Project")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "project.pmforge"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "not-a-project"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := enumerateProjects(dir)
	if err != nil {
		t.Fatalf("enumerateProjects: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d: %#v", len(got), got)
	}
	names := map[string]bool{}
	for _, e := range got {
		names[e.Name] = true
	}
	if !names["Legacy Project"] {
		t.Errorf("legacy flat project missing; names=%v", names)
	}
	if !names["New Project"] {
		t.Errorf("subfolder project name not de-prefixed; names=%v", names)
	}
}

// TestCreateProjectUsesUniqueSubfolder covers the full create/clone/delete
// lifecycle on the new per-project subfolder layout.
func TestCreateProjectUsesUniqueSubfolder(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("user1", "User One", "a-strong-password-123", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	pf, err := app.CreateProject("My Plan", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if filepath.Base(pf.Path) != "project.pmforge" {
		t.Fatalf("expected project.pmforge inside a subfolder, got %s", pf.Path)
	}
	folder := filepath.Base(filepath.Dir(pf.Path))
	if !projectFolderRe.MatchString(folder) {
		t.Fatalf("project folder %q lacks the timestamp prefix", folder)
	}
	if list, err := app.ListProjects(); err != nil || len(list) != 1 {
		t.Fatalf("after create: list err=%v len=%d", err, len(list))
	}

	// Clone -> a distinct subfolder, name + " copy".
	clone, err := app.CloneProject(pf.Path)
	if err != nil {
		t.Fatalf("CloneProject: %v", err)
	}
	if filepath.Dir(clone.Path) == filepath.Dir(pf.Path) {
		t.Fatalf("clone must live in a new subfolder; got %s", clone.Path)
	}
	if clone.Name != "My Plan copy" {
		t.Fatalf("clone name = %q, want %q", clone.Name, "My Plan copy")
	}
	if list, err := app.ListProjects(); err != nil || len(list) != 2 {
		t.Fatalf("after clone: list err=%v len=%d", err, len(list))
	}

	// Delete the original -> its whole subfolder is removed.
	if err := app.DeleteProject(pf.Path); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, statErr := os.Stat(filepath.Dir(pf.Path)); !os.IsNotExist(statErr) {
		t.Fatalf("deleted project subfolder still exists: %v", statErr)
	}
	if list, err := app.ListProjects(); err != nil || len(list) != 1 {
		t.Fatalf("after delete: list err=%v len=%d", err, len(list))
	}
}
