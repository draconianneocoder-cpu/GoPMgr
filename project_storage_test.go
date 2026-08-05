// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
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
// projects across BOTH axes it must stay backward compatible on: the flat
// vs. subfolder layout (pre- vs. post- the per-project-subfolder change),
// and the ".pmforge" vs. ".gopmgr" extension (pre- vs. post- the 2026-08-04
// PMForge -> GoPMgr rename) — all four combinations, since a real disk can
// contain projects from any point in the app's history. Also verifies
// unrelated subfolders are ignored.
func TestEnumerateProjectsSupportsBothLayouts(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "Legacy Flat Project.pmforge"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Renamed Flat Project.gopmgr"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacySub := filepath.Join(dir, "20260615-153000-Legacy Subfolder Project")
	if err := os.MkdirAll(legacySub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacySub, "project.pmforge"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	newSub := filepath.Join(dir, "20260805-090000-New Subfolder Project")
	if err := os.MkdirAll(newSub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newSub, "project.gopmgr"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "not-a-project"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := enumerateProjects(dir)
	if err != nil {
		t.Fatalf("enumerateProjects: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 projects, got %d: %#v", len(got), got)
	}
	names := map[string]bool{}
	for _, e := range got {
		names[e.Name] = true
	}
	for _, want := range []string{"Legacy Flat Project", "Renamed Flat Project", "Legacy Subfolder Project", "New Subfolder Project"} {
		if !names[want] {
			t.Errorf("%q missing; names=%v", want, names)
		}
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
	if filepath.Base(pf.Path) != "project.gopmgr" {
		t.Fatalf("expected project.gopmgr inside a subfolder, got %s", pf.Path)
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
