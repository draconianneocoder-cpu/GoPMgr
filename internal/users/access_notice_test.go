// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"errors"
	"path/filepath"
	"testing"
)

// recordAccess has alice open username's data with reason and returns the
// access's event id.
func recordAccess(t *testing.T, store *Store, aliceDEK []byte, username, reason string) int64 {
	t.Helper()
	dek, err := store.OpenUserForAdmin("alice", aliceDEK, username, reason)
	if err != nil {
		t.Fatalf("OpenUserForAdmin(%s): %v", username, err)
	}
	clear(dek)
	history, err := store.AccessHistory(username)
	if err != nil || len(history) == 0 || history[0].Detail != reason {
		t.Fatalf("AccessHistory(%s) = %+v, %v; want the access just recorded first", username, history, err)
	}
	return history[0].ID
}

func unreadReasons(t *testing.T, store *Store, username string) []string {
	t.Helper()
	notices, err := store.UnreadAccessNotices(username)
	if err != nil {
		t.Fatalf("UnreadAccessNotices(%s): %v", username, err)
	}
	var out []string
	for _, n := range notices {
		out = append(out, n.Detail)
	}
	return out
}

func noticeReadEvents(t *testing.T, store *Store) []AccountEvent {
	t.Helper()
	events, err := store.AccountEvents()
	if err != nil {
		t.Fatalf("AccountEvents: %v", err)
	}
	var out []AccountEvent
	for _, e := range events {
		if e.Action == AccountAccessNoticeRead {
			out = append(out, e)
		}
	}
	return out
}

func TestAccessNoticesStayUntilAcknowledged(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	recordAccess(t, store, aliceDEK, "bob", "ticket 1")
	newest := recordAccess(t, store, aliceDEK, "bob", "ticket 2")

	if got := unreadReasons(t, store, "bob"); len(got) != 2 || got[0] != "ticket 2" || got[1] != "ticket 1" {
		t.Fatalf("unread = %q; want both accesses, newest first", got)
	}
	if err := store.AcknowledgeAccessNotices("bob", newest); err != nil {
		t.Fatalf("AcknowledgeAccessNotices: %v", err)
	}
	if got := unreadReasons(t, store, "bob"); len(got) != 0 {
		t.Fatalf("unread after acknowledging = %q; want none", got)
	}
	read := noticeReadEvents(t, store)
	if len(read) != 1 || read[0].Actor != "bob" || read[0].Username != "bob" || read[0].Detail != "2 accesses" {
		t.Fatalf("acknowledgement events = %+v; want one by bob for 2 accesses", read)
	}
	if history, _ := store.AccessHistory("bob"); len(history) != 2 {
		t.Fatalf("history after acknowledging = %+v; want both accesses kept", history)
	}
}

// An access recorded while the notice was on screen was not shown, so
// acknowledging what was shown leaves it unread.
func TestAccessRecordedAfterTheNoticeWasShownStaysUnread(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	shown := recordAccess(t, store, aliceDEK, "bob", "ticket 1")
	recordAccess(t, store, aliceDEK, "bob", "ticket 2")

	if err := store.AcknowledgeAccessNotices("bob", shown); err != nil {
		t.Fatalf("AcknowledgeAccessNotices: %v", err)
	}
	if got := unreadReasons(t, store, "bob"); len(got) != 1 || got[0] != "ticket 2" {
		t.Fatalf("unread = %q; want only the access recorded after the notice was shown", got)
	}
	if read := noticeReadEvents(t, store); len(read) != 1 || read[0].Detail != "1 access" {
		t.Fatalf("acknowledgement events = %+v; want one for 1 access", read)
	}
}

func TestAcknowledgingOnlyMovesForwardAndOnlyForYourOwnAccesses(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	addUser(t, store, "carol", nil)
	older := recordAccess(t, store, aliceDEK, "bob", "ticket 1")
	newer := recordAccess(t, store, aliceDEK, "bob", "ticket 2")
	carols := recordAccess(t, store, aliceDEK, "carol", "ticket 3")

	for name, id := range map[string]int64{"another account's access": carols, "an access never recorded": newer + 1000} {
		if err := store.AcknowledgeAccessNotices("bob", id); !errors.Is(err, ErrAcknowledgeUnknownAccess) {
			t.Errorf("%s: AcknowledgeAccessNotices = %v, want ErrAcknowledgeUnknownAccess", name, err)
		}
	}
	if err := store.AcknowledgeAccessNotices("bob", newer); err != nil {
		t.Fatalf("AcknowledgeAccessNotices(newer): %v", err)
	}
	// Going back, or acknowledging again, changes and records nothing.
	for _, id := range []int64{older, newer} {
		if err := store.AcknowledgeAccessNotices("bob", id); err != nil {
			t.Fatalf("AcknowledgeAccessNotices(%d) again: %v", id, err)
		}
	}
	if got := unreadReasons(t, store, "bob"); len(got) != 0 {
		t.Fatalf("bob's unread = %q; want none", got)
	}
	if read := noticeReadEvents(t, store); len(read) != 1 {
		t.Fatalf("acknowledgement events = %+v; want one, not one per call", read)
	}
	if got := unreadReasons(t, store, "carol"); len(got) != 1 || got[0] != "ticket 3" {
		t.Fatalf("carol's unread = %q; bob's acknowledgement must not touch it", got)
	}
	if history, _ := store.AccessHistory("carol"); len(history) != 1 || history[0].Detail != "ticket 3" {
		t.Fatalf("carol's history = %+v; want only her own access", history)
	}
}

func TestAccessNoticesSurviveReopeningAndGoWithTheAccount(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	aliceDEK := firstAdmin(t, store)
	addUser(t, store, "bob", nil)
	id := recordAccess(t, store, aliceDEK, "bob", "ticket 1")
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store, err = Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if got := unreadReasons(t, store, "bob"); len(got) != 1 {
		t.Fatalf("unread after reopening = %q; want the access", got)
	}
	if err := store.AcknowledgeAccessNotices("bob", id); err != nil {
		t.Fatalf("AcknowledgeAccessNotices: %v", err)
	}
	if err := store.PurgeAccount("alice", "bob"); err != nil {
		t.Fatalf("PurgeAccount(bob): %v", err)
	}
	if n := countRows(t, store, "access_notices", "username = 'bob'"); n != 0 {
		t.Fatal("a deleted account's acknowledgement mark was kept")
	}
}
