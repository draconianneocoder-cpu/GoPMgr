// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

// TestLoginRefusesWhileProjectOpen guards against a real failure mode
// spotted while investigating App.RepairAndSwap: unlike Logout and
// shutdown, which always clear a.db and a.dek together, Login had no
// check against a.db already being set. A caller invoking Login while a
// project was open would leave that project's live database connection
// under the newly-authenticated user's DEK — mixing one user's project
// with another user's encryption key. Not reachable through the shipped
// frontend today (see docs/beta-release-backlog.md), but this pins the
// defense-in-depth guard directly, independent of frontend behavior.
func TestLoginRefusesWhileProjectOpen(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "hunter2hunter2hunter2", true); err != nil {
		t.Fatalf("CreateAccount(bob): %v", err)
	}
	mustOpenProject(t, app, "Alice's Project")

	aliceUser := app.requireUser()
	if aliceUser == nil || aliceUser.Username != "alice" {
		t.Fatalf("fixture setup: want alice signed in, got %+v", aliceUser)
	}
	aliceDB := app.requireDB()
	if aliceDB == nil {
		t.Fatal("fixture setup: want a project open before the refused Login attempt")
	}

	// Assert the specific refusal, not just any error — a typo'd
	// password or an unexpected CreateAccount failure would also make
	// Login return a non-nil error ("invalid credentials") without ever
	// reaching the guard this test exists to pin.
	_, err := app.Login("bob", "hunter2hunter2hunter2")
	if err == nil {
		t.Fatal("Login: want refusal while a project is open, got nil error")
	}
	if !strings.Contains(err.Error(), "already open") {
		t.Fatalf("Login: want the \"a project is already open\" refusal, got %v", err)
	}

	// The original session must be completely untouched by the refused
	// attempt — not just "still some user signed in", but the exact
	// same user and the exact same live database connection, proving
	// Login didn't partially apply before refusing.
	if got := app.requireUser(); got == nil || got.Username != "alice" {
		t.Fatalf("Login refusal must not change the active user: want alice, got %+v", got)
	}
	if got := app.requireDB(); got != aliceDB {
		t.Fatalf("Login refusal must not touch the open database: want the same *db.Database, got a different one (or nil)")
	}
	if _, err := app.ListStakeholders(""); err != nil {
		t.Fatalf("project must remain fully usable after the refused Login: %v", err)
	}
}
