// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestReserveProjectFolderRetriesCollisionsAndKeepsPrivateMode(t *testing.T) {
	dir := t.TempDir()
	const id = "20260815-120000-project"
	if err := os.Mkdir(filepath.Join(dir, id), 0o700); err != nil {
		t.Fatalf("create pre-existing project folder: %v", err)
	}
	sentinelPath := filepath.Join(dir, id, "keep")
	if err := os.WriteFile(sentinelPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write existing project sentinel: %v", err)
	}

	path, err := reserveProjectFolder(dir, id)
	if err != nil {
		t.Fatalf("reserveProjectFolder: %v", err)
	}
	if got, want := filepath.Base(filepath.Dir(path)), id+"-2"; got != want {
		t.Fatalf("reserved folder = %q, want %q", got, want)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat reserved folder: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("reserved folder mode = %o, want %o", got, want)
	}
	if got, err := os.ReadFile(sentinelPath); err != nil || string(got) != "original" {
		t.Fatalf("existing project sentinel = %q, err %v, want original", got, err)
	}
}

func TestReserveProjectFolderAllocatesUniquePathsUnderContention(t *testing.T) {
	const workers = 32
	const id = "20260815-120000-project"
	dir := t.TempDir()
	paths := make(chan string, workers)
	errs := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			<-start
			path, err := reserveProjectFolder(dir, id)
			if err != nil {
				errs <- err
				return
			}
			paths <- path
		})
	}
	close(start)
	wg.Wait()
	close(paths)
	close(errs)

	for err := range errs {
		t.Fatalf("reserveProjectFolder: %v", err)
	}
	seen := make(map[string]struct{}, workers)
	for path := range paths {
		if filepath.Base(path) != "project"+projectFileExtension {
			t.Fatalf("project path = %q, want project filename", path)
		}
		if _, exists := seen[path]; exists {
			t.Fatalf("duplicate project path allocated: %q", path)
		}
		if filepath.Dir(filepath.Dir(path)) != dir {
			t.Fatalf("project path = %q, want direct child of %q", path, dir)
		}
		seen[path] = struct{}{}
	}
	if got := len(seen); got != workers {
		t.Fatalf("allocated %d project paths, want %d", got, workers)
	}
}

func TestReserveProjectFolderReturnsParentCreationError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("block"), 0o600); err != nil {
		t.Fatalf("create blocking parent file: %v", err)
	}

	path, err := reserveProjectFolder(parentFile, "20260815-120000-project")
	if path != "" {
		t.Fatalf("path = %q, want empty on parent creation failure", path)
	}
	if err == nil {
		t.Fatal("reserveProjectFolder succeeded through a non-directory parent")
	}
	if errors.Is(err, fs.ErrExist) {
		t.Fatalf("reserveProjectFolder error = %v, want non-collision error", err)
	}
}

func TestReserveProjectFolderReturnsCandidateCreationError(t *testing.T) {
	dir := t.TempDir()
	called := false
	path, err := reserveProjectFolderWithMkdir(dir, "20260815-120000-project", func(_ string, _ fs.FileMode) error {
		called = true
		return fs.ErrPermission
	})
	if !called {
		t.Fatal("candidate directory creator was not called")
	}
	if path != "" {
		t.Fatalf("path = %q, want empty on candidate creation failure", path)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("reserveProjectFolder error = %v, want error wrapping fs.ErrPermission", err)
	}
}

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
