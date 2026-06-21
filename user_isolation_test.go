// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

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

	// Switch to Bob, a brand-new user.
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "bob-strong-password", false); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
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
