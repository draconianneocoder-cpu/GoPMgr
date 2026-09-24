// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProjectsAreIsolatedPerUser locks in the invariant that a signed-in
// user only ever enumerates their own projects. It guards against a
// regression where one user could see another user's projects in-app
// (ListProjects / ProjectsOverview read the per-user DataDir, and the
// session user/DEK is switched on login).
func TestProjectsAreIsolatedPerUser(t *testing.T) {
	app := newEncryptionProjectTestApp(t)

	// Alice creates a project.
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	if _, err := app.CreateProject("Alice Secret", "only alice"); err != nil {
		t.Fatalf("alice CreateProject: %v", err)
	}
	aliceList, err := app.ListProjects()
	if err != nil {
		t.Fatalf("alice ListProjects: %v", err)
	}
	if len(aliceList) != 1 {
		t.Fatalf("alice should see exactly her 1 project, got %d", len(aliceList))
	}

	// Switch to Bob, a brand-new user. Alice, the first account and so the
	// administrator, creates him; only administrators can add accounts.
	if _, err := app.CreateAccount("bob", "Bob", "bob-strong-password", false); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := app.Login("bob", "bob-strong-password"); err != nil {
		t.Fatalf("Login bob: %v", err)
	}

	// Bob must see none of Alice's projects, via either listing path.
	bobList, err := app.ListProjects()
	if err != nil {
		t.Fatalf("bob ListProjects: %v", err)
	}
	if len(bobList) != 0 {
		t.Fatalf("ISOLATION LEAK: bob saw %d projects via ListProjects: %#v", len(bobList), bobList)
	}
	bobOverview, err := app.ProjectsOverview()
	if err != nil {
		t.Fatalf("bob ProjectsOverview: %v", err)
	}
	if len(bobOverview) != 0 {
		t.Fatalf("ISOLATION LEAK: bob saw %d projects via ProjectsOverview: %#v", len(bobOverview), bobOverview)
	}
}

// TestRecreatedUsernameDoesNotInheritDeletedAccountsFolder covers an
// administrator deleting an account and later creating one with the same
// name, possibly for someone else. The deleted account's folder stays on
// disk; the new account must not be handed it.
func TestRecreatedUsernameDoesNotInheritDeletedAccountsFolder(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "bob-strong-password", false); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout alice: %v", err)
	}
	if _, err := app.Login("bob", "bob-strong-password"); err != nil {
		t.Fatalf("Login bob: %v", err)
	}
	project, err := app.CreateProject("Bob Private", "")
	if err != nil {
		t.Fatalf("bob CreateProject: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout bob: %v", err)
	}
	projectBytes, err := os.ReadFile(project.Path)
	if err != nil {
		t.Fatalf("read bob's project: %v", err)
	}

	if _, err := app.Login("alice", "alice-strong-password"); err != nil {
		t.Fatalf("Login alice: %v", err)
	}
	// Leave bob's folder behind without his account, as account deletion
	// did before Purge existed (and as a Purge that cannot finish does):
	// set the folder aside, purge, and put it back.
	bobDir := filepath.Dir(filepath.Dir(filepath.Dir(project.Path)))
	aside := bobDir + "-aside"
	if err := os.Rename(bobDir, aside); err != nil {
		t.Fatalf("set bob's folder aside: %v", err)
	}
	if err := app.AdminPurgeUser("bob", "bob"); err != nil {
		t.Fatalf("AdminPurgeUser bob: %v", err)
	}
	if err := os.Rename(aside, bobDir); err != nil {
		t.Fatalf("restore bob's folder: %v", err)
	}

	_, err = app.CreateAccount("bob", "Another Bob", "new-bob-password", false)
	if err == nil {
		// Show the leak itself, not just the missing error.
		if logoutErr := app.Logout(); logoutErr != nil {
			t.Fatalf("Logout alice: %v", logoutErr)
		}
		if _, loginErr := app.Login("bob", "new-bob-password"); loginErr != nil {
			t.Fatalf("recreated bob was allowed but cannot sign in: %v", loginErr)
		}
		list, listErr := app.ListProjects()
		t.Fatalf("recreated bob was allowed and inherited the old folder: sees %d project(s) %v (list err %v)", len(list), list, listErr)
	}
	if !strings.Contains(err.Error(), "left over") {
		t.Fatalf("CreateAccount over a deleted account's folder: err = %v, want the left-over folder message", err)
	}
	if after, err := os.ReadFile(project.Path); err != nil || !bytes.Equal(after, projectBytes) {
		t.Fatalf("deleted account's project changed by the refused creation (read err %v)", err)
	}
	if _, err := app.CreateAccount("bobby", "Bobby", "bobby-password", false); err != nil {
		t.Fatalf("CreateAccount with a different name: %v", err)
	}
}
