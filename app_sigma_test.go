// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

// TestSigmaGetProject_RejectsSigmaProjectFromAnotherFile is the App-layer
// regression test for the second residual risk the Sigma DMAIC FK-bug fix
// left open: SigmaGetProject took a bare Sigma project id and returned
// whichever row matched it in the currently-open database, with no check
// that the row belongs to the currently-open GoPMgr project. In today's
// real usage this id always comes from session.editingId, itself always
// sourced from a prior SigmaListProjects call (now scoped) or a fresh
// SigmaCreateProject response for the file that's actually open, so this
// was not reachable through the shipped UI as far as could be determined
// -- but a stale id surviving a project switch, a future UI bug, or a
// direct binding call from devtools could still trigger it, and
// SigmaProjectView.svelte's loadProject() calls SigmaGetProject before any
// sub-tab getter (Charter/Fishbone/SIPOC/VoC/...), so scoping it here is
// the effective chokepoint for the whole load sequence.
func TestSigmaGetProject_RejectsSigmaProjectFromAnotherFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	mustOpenProject(t, app, "Project A")
	sigmaA, err := app.SigmaCreateProject("A's DMAIC Project", "", "green_belt")
	if err != nil {
		t.Fatalf("SigmaCreateProject (A): %v", err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatalf("CloseProject (A): %v", err)
	}

	mustOpenProject(t, app, "Project B")
	sigmaB, err := app.SigmaCreateProject("B's DMAIC Project", "", "green_belt")
	if err != nil {
		t.Fatalf("SigmaCreateProject (B): %v", err)
	}

	if _, err := app.SigmaGetProject(sigmaA.ID); err == nil {
		t.Fatal("SigmaGetProject(A's sigma id) while B is open: want an error, got nil")
	}

	got, err := app.SigmaGetProject(sigmaB.ID)
	if err != nil {
		t.Fatalf("SigmaGetProject(B's sigma id) while B is open: %v", err)
	}
	if got.Title != "B's DMAIC Project" {
		t.Fatalf("SigmaGetProject(B) = %+v, want B's own project", got)
	}

	list, err := app.SigmaListProjects()
	if err != nil {
		t.Fatalf("SigmaListProjects (B open): %v", err)
	}
	if len(list) != 1 || list[0].Title != "B's DMAIC Project" {
		t.Fatalf("SigmaListProjects (B open) = %+v, want only B's Sigma project", list)
	}
}
