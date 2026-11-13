// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/users"
)

// SEC-04: a project named "Phase #2" could not be created. The name became
// part of the folder path, and the encrypted database refuses a path with
// '#' or '?' (they would be read as connection options). The folder name is
// now made safe for that; the project keeps the name the user typed.
func TestCreateProjectAcceptsAHashInItsName(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("user1", "User One", "a-strong-password-123", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	acceptNoRecoveryCodes(t, app)

	pf, err := app.CreateProject("Phase #2", "")
	if err != nil {
		t.Fatalf("CreateProject(%q): %v", "Phase #2", err)
	}
	if pf.Name != "Phase #2" {
		t.Fatalf("project name = %q, want the name as typed", pf.Name)
	}
	if folder := filepath.Base(filepath.Dir(pf.Path)); strings.ContainsAny(folder, "#?") {
		t.Fatalf("project folder %q still contains a character the database refuses", folder)
	}
	if _, err := app.CloneProject(pf.Path); err != nil {
		t.Fatalf("CloneProject of a project named with '#': %v", err)
	}
	if list, err := app.ListProjects(); err != nil || len(list) != 2 {
		t.Fatalf("ListProjects: err=%v len=%d, want 2", err, len(list))
	}
}

// When the project database cannot be created, CreateProject used to leave
// the folder it had reserved behind, empty. Here the account's own data
// folder contains '#', so the encrypted database refuses every project path.
func TestCreateProjectRemovesTheFolderItCouldNotUse(t *testing.T) {
	store, err := users.Open(filepath.Join(t.TempDir(), "root#data"))
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	app := &App{store: store}
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if _, err := app.CreateAccount("user1", "User One", "a-strong-password-123", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	acceptNoRecoveryCodes(t, app)

	if _, err := app.CreateProject("Plan", ""); err == nil {
		t.Fatal("CreateProject succeeded under a data folder the database refuses")
	}
	projects := filepath.Join(app.currentSession().user.DataDir, "projects")
	entries, err := os.ReadDir(projects)
	if err != nil {
		t.Fatalf("read projects folder: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("a failed CreateProject left %d entries behind, first %q", len(entries), entries[0].Name())
	}
}
