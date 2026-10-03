// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

// alice opens bob's data; bob then sees it, and only his own accesses,
// until he acknowledges.
func TestAppTellsUsersAboutAccessToTheirOwnDataOnly(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	for _, name := range []string{"bob", "carol"} {
		if _, err := app.CreateAccount(name, name, escrowTestPassword, false); err != nil {
			t.Fatalf("CreateAccount(%s): %v", name, err)
		}
	}
	for _, name := range []string{"bob", "carol"} {
		if err := app.AdminOpenUserData(name, "checking "+name); err != nil {
			t.Fatalf("AdminOpenUserData(%s): %v", name, err)
		}
	}

	switchUser(t, app, "bob")
	notices, err := app.MyDataAccessNotices()
	if err != nil || len(notices) != 1 || notices[0].Actor != "alice" || notices[0].Detail != "checking bob" {
		t.Fatalf("bob's notices = %+v, %v; want alice's one access, with its reason", notices, err)
	}
	if err := app.AcknowledgeDataAccessNotices(notices[0].ID); err != nil {
		t.Fatalf("AcknowledgeDataAccessNotices: %v", err)
	}
	if again, err := app.MyDataAccessNotices(); err != nil || len(again) != 0 {
		t.Fatalf("bob's notices after acknowledging = %+v, %v; want none", again, err)
	}
	if history, err := app.MyDataAccessHistory(); err != nil || len(history) != 1 || history[0].Detail != "checking bob" {
		t.Fatalf("bob's history = %+v, %v; want his one access", history, err)
	}

	// bob cannot acknowledge carol's access, and his acknowledging left
	// hers unread.
	switchUser(t, app, "carol")
	notices, err = app.MyDataAccessNotices()
	if err != nil || len(notices) != 1 || notices[0].Detail != "checking carol" {
		t.Fatalf("carol's notices = %+v, %v; want her one access", notices, err)
	}
	carolsID := notices[0].ID
	switchUser(t, app, "bob")
	want := "that access to your data was not found; reload the notice"
	if err := app.AcknowledgeDataAccessNotices(carolsID); err == nil || err.Error() != want {
		t.Fatalf("bob acknowledging carol's access = %v, want %q", err, want)
	}
	switchUser(t, app, "carol")
	if notices, _ := app.MyDataAccessNotices(); len(notices) != 1 {
		t.Fatalf("carol's notices = %+v; want hers still unread", notices)
	}
}

func TestAccessNoticeMethodsNeedASignedInUser(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.MyDataAccessNotices(); err == nil {
		t.Error("MyDataAccessNotices without a session succeeded")
	}
	if err := app.AcknowledgeDataAccessNotices(1); err == nil {
		t.Error("AcknowledgeDataAccessNotices without a session succeeded")
	}
	if _, err := app.MyDataAccessHistory(); err == nil {
		t.Error("MyDataAccessHistory without a session succeeded")
	}
}
