// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// standbyStore returns superStore's machine (alice the super administrator,
// bob a subordinate, carol standard) plus dave, a second subordinate who
// has signed in, with alice's, bob's, and dave's DEKs.
func standbyStore(t *testing.T) (store *Store, aliceDEK, bobDEK, daveDEK []byte) {
	t.Helper()
	store, aliceDEK, bobDEK = superStore(t)
	daveDEK = addUser(t, store, "dave", nil)
	if err := store.PromoteAdmin("alice", aliceDEK, "dave"); err != nil {
		t.Fatalf("PromoteAdmin(dave): %v", err)
	}
	return store, aliceDEK, bobDEK, daveDEK
}

func mustSetStandby(t *testing.T, store *Store, actor string, dek []byte, standby string, days int) {
	t.Helper()
	if keyPassed, err := store.SetStandby(actor, dek, standby, days); err != nil || (standby != "" && !keyPassed) {
		t.Fatalf("SetStandby(%q, %d) = %v, %v; want it named with the key", standby, days, keyPassed, err)
	}
}

// superInactiveFor makes the super administrator's last sign-in and their
// assignment both d ago.
func superInactiveFor(t *testing.T, store *Store, d time.Duration) {
	t.Helper()
	mustSuper(t, store)
	stamp := time.Now().UTC().Add(-d).Format(time.RFC3339Nano)
	mustExec(t, store, `UPDATE users SET last_login = ? WHERE username = (SELECT username FROM super_admin WHERE id = 1)`, stamp)
	mustExec(t, store, `UPDATE super_admin SET assigned_at = ?`, stamp)
}

func mustSuccession(t *testing.T, store *Store, actor string) Succession {
	t.Helper()
	got, err := store.Succession(actor)
	if err != nil {
		t.Fatalf("Succession(%s): %v", actor, err)
	}
	return got
}

func latestEvent(t *testing.T, store *Store) AccountEvent {
	t.Helper()
	events, err := store.AccountEvents()
	if err != nil || len(events) == 0 {
		t.Fatalf("AccountEvents = %d, %v", len(events), err)
	}
	return events[0]
}

const day = 24 * time.Hour

func TestNamingAStandbyPassesTheKeyButNotTheRole(t *testing.T) {
	store, aliceDEK, bobDEK, _ := standbyStore(t)
	aliceKey := append([]byte(nil), openEscrow(t, store, "alice", aliceDEK).public...)

	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)

	if got := openEscrow(t, store, "bob", bobDEK); !bytes.Equal(got.public, aliceKey) {
		t.Fatal("the standby's grant does not open to the administrator key")
	}
	if e := latestEvent(t, store); e.Actor != "alice" || e.Action != AccountStandbyNamed || e.Username != "bob" ||
		e.Detail != "takes over after 30 days without a sign-in by the super administrator; administrator key passed" {
		t.Fatalf("latest event = %+v; want the standby named with the period and the key", e)
	}
	got := mustSuccession(t, store, "alice")
	want := Succession{Super: "alice", Standby: "bob", StandbyHoldsKey: true, TakeoverDays: 30}
	if got != want {
		t.Fatalf("Succession = %+v, want %+v", got, want)
	}
	// Until a takeover the standby is a subordinate.
	if _, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "checking"); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("the standby opening data = %v, want ErrNotSuper", err)
	}
	if err := store.SetAdmin("bob", "dave", false); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("the standby demoting an administrator = %v, want ErrNotSuper", err)
	}
	if _, err := store.SetStandby("bob", bobDEK, "dave", 30); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("the standby naming a standby = %v, want ErrNotSuper", err)
	}
}

func TestStandbyRefusals(t *testing.T) {
	store, aliceDEK, _, daveDEK := standbyStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	_, attackerPub := attackerKeyPair(t)
	// gina: an administrator whose attested key was swapped.
	addUser(t, store, "gina", session)
	mustExec(t, store, `UPDATE users SET is_admin = 1 WHERE username = 'gina'`)
	mustExec(t, store, `UPDATE personal_keys SET public_key = ? WHERE username = 'gina'`, attackerPub)
	// erin: an administrator who has not signed in since enrollment.
	if _, err := store.CreateAccount("erin", "Erin", statusPassword, true); err != nil {
		t.Fatalf("create erin: %v", err)
	}
	// frank: a disabled administrator.
	addUser(t, store, "frank", nil)
	mustExec(t, store, `UPDATE users SET is_admin = 1, disabled = 1 WHERE username = 'frank'`)

	for standby, want := range map[string]error{
		"carol":  ErrTargetNotAdmin,
		"frank":  ErrTargetNotAdmin,
		"nobody": ErrTargetNotAdmin,
		"erin":   ErrNoPersonalKey,
		"gina":   ErrPersonalKeyNotAttested,
		"alice":  ErrTargetIsSuper,
	} {
		if _, err := store.SetStandby("alice", aliceDEK, standby, 30); !errors.Is(err, want) {
			t.Errorf("SetStandby(%s) = %v, want %v", standby, err, want)
		}
	}
	for _, days := range []int{-1, 0, MinTakeoverDays - 1, MaxTakeoverDays + 1} {
		if _, err := store.SetStandby("alice", aliceDEK, "bob", days); !errors.Is(err, ErrTakeoverPeriod) {
			t.Errorf("SetStandby(bob, %d days) = %v, want ErrTakeoverPeriod", days, err)
		}
	}
	if _, err := store.SetStandby("dave", daveDEK, "bob", 30); !errors.Is(err, ErrNotSuper) {
		t.Errorf("SetStandby by a subordinate = %v, want ErrNotSuper", err)
	}
	if got := mustSuccession(t, store, "alice"); got.Standby != "" || got.TakeoverDays != DefaultTakeoverDays {
		t.Fatalf("Succession = %+v after refusals; want no standby and the default period", got)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username <> 'alice'"); n != 0 {
		t.Fatalf("%d grants were given by refused standby changes", n)
	}
	requireEvent(t, store, "alice escrow_key_mismatch gina")

	for _, days := range []int{MinTakeoverDays, MaxTakeoverDays} {
		mustSetStandby(t, store, "alice", aliceDEK, "bob", days)
		if got := mustSuccession(t, store, "alice").TakeoverDays; got != days {
			t.Fatalf("takeover period = %d, want %d", got, days)
		}
	}
}

func TestReplacingOrRemovingTheStandbyTakesTheirKey(t *testing.T) {
	store, aliceDEK, _, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)

	mustSetStandby(t, store, "alice", aliceDEK, "dave", 30)
	if n := countRows(t, store, "escrow_grants", "admin_username = 'bob'"); n != 0 {
		t.Fatal("the replaced standby kept the key")
	}
	requireEvent(t, store, "alice super_standby_removed bob")
	if got := mustSuccession(t, store, "alice"); got.Standby != "dave" || !got.StandbyHoldsKey {
		t.Fatalf("Succession = %+v, want dave holding the key", got)
	}

	mustSetStandby(t, store, "alice", aliceDEK, "", 30)
	if n := countRows(t, store, "escrow_grants", "admin_username <> 'alice'"); n != 0 {
		t.Fatal("the removed standby kept the key")
	}
	if e := latestEvent(t, store); e.Action != AccountStandbyRemoved || e.Username != "dave" || e.Detail != "removed by the super administrator" {
		t.Fatalf("latest event = %+v; want dave's removal", e)
	}

	before := len(eventActions(t, store))
	mustSetStandby(t, store, "alice", aliceDEK, "", 30)
	if after := len(eventActions(t, store)); after != before {
		t.Fatal("a change that changed nothing was recorded")
	}
	mustSetStandby(t, store, "alice", aliceDEK, "", 45)
	if e := latestEvent(t, store); e.Action != AccountTakeoverPeriodChanged || e.Detail != "45 days" {
		t.Fatalf("latest event = %+v; want the new period recorded", e)
	}
	if got := mustSuccession(t, store, "alice"); got.Standby != "" || got.TakeoverDays != 45 {
		t.Fatalf("Succession = %+v, want no standby and 45 days", got)
	}
}

// The standby's sign-in keeps their grant, where any other subordinate's
// is removed, and removes one that no longer opens.
func TestStandbyKeepsTheKeyAtSignIn(t *testing.T) {
	store, aliceDEK, bobDEK, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)

	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
	openEscrow(t, store, "bob", bobDEK)

	mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'bob'`)
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
	if got := mustSuccession(t, store, "alice"); got.Standby != "bob" || got.StandbyHoldsKey {
		t.Fatalf("Succession = %+v; want bob still standby, without the broken key", got)
	}
	requireEvent(t, store, "bob escrow_key_mismatch bob")
}

func TestTakeoverRefusalBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	base := successionState{super: "alice", days: 30, lastActive: now.Add(-30 * day)}
	withStandby := base
	withStandby.standby = "bob"
	for _, tc := range []struct {
		name  string
		st    successionState
		actor string
		at    time.Time
		want  error
	}{
		{"exactly the period", base, "bob", now, nil},
		{"one second short", base, "bob", now.Add(-time.Second), ErrTakeoverNotDue},
		{"the super administrator", base, "alice", now, ErrTargetIsSuper},
		{"no super administrator", successionState{days: 30}, "bob", now, ErrTargetIsSuper},
		{"the standby", withStandby, "bob", now, nil},
		{"not the standby", withStandby, "dave", now, ErrNotStandby},
		{"not the standby, not due", withStandby, "dave", now.Add(-day), ErrNotStandby},
		{"last active in the future", successionState{super: "alice", days: 30, lastActive: now.Add(day)}, "bob", now, ErrTakeoverNotDue},
		{"nothing on record", successionState{super: "alice", days: 30}, "bob", now, nil},
	} {
		if got := tc.st.takeoverRefusal(tc.actor, tc.at); !errors.Is(got, tc.want) || (tc.want == nil && got != nil) {
			t.Errorf("%s: takeoverRefusal = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The period counts from the later of the last sign-in and the time the
// role was given, so a hand-over or an upgrade starts it again.
func TestTakeoverCountsFromTheLaterOfSignInAndAssignment(t *testing.T) {
	store, _, bobDEK, _ := standbyStore(t)
	mustSuper(t, store)
	stamp := func(d time.Duration) string { return time.Now().UTC().Add(-d).Format(time.RFC3339Nano) }
	for _, tc := range []struct {
		name                  string
		lastLogin, assignedAt string
		due                   bool
	}{
		{"both old", stamp(40 * day), stamp(40 * day), true},
		{"signed in recently", stamp(10 * day), stamp(40 * day), false},
		{"given the role recently", stamp(40 * day), stamp(10 * day), false},
		{"never signed in, given the role long ago", "", stamp(40 * day), true},
		{"never signed in, given the role recently", "", stamp(10 * day), false},
		{"unreadable sign-in", "garbage", stamp(40 * day), true},
	} {
		mustExec(t, store, `UPDATE users SET last_login = ? WHERE username = 'alice'`, tc.lastLogin)
		mustExec(t, store, `UPDATE super_admin SET assigned_at = ?`, tc.assignedAt)
		if got := mustSuccession(t, store, "bob").CanTakeOver; got != tc.due {
			t.Errorf("%s: CanTakeOver = %v, want %v", tc.name, got, tc.due)
		}
	}
	clear(bobDEK)
}

func TestStandbyTakesOverWithTheKey(t *testing.T) {
	store, aliceDEK, bobDEK, daveDEK := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	superInactiveFor(t, store, 29*day)
	if _, err := store.TakeOverSuper("bob", bobDEK); !errors.Is(err, ErrTakeoverNotDue) {
		t.Fatalf("takeover before the period = %v, want ErrTakeoverNotDue", err)
	}
	superInactiveFor(t, store, 31*day)
	if got := mustSuccession(t, store, "bob"); !got.CanTakeOver || got.SuperInactiveDays != 31 {
		t.Fatalf("Succession(bob) = %+v; want the takeover offered after 31 days", got)
	}
	if got := mustSuccession(t, store, "dave"); got.CanTakeOver {
		t.Fatal("a subordinate who is not the standby was offered the takeover")
	}
	if _, err := store.TakeOverSuper("dave", daveDEK); !errors.Is(err, ErrNotStandby) {
		t.Fatalf("takeover by another subordinate = %v, want ErrNotStandby", err)
	}

	keyHeld, err := store.TakeOverSuper("bob", bobDEK)
	if err != nil || !keyHeld {
		t.Fatalf("TakeOverSuper(bob) = %v, %v; want the role with the key", keyHeld, err)
	}
	if got := mustSuccession(t, store, "bob"); got.Super != "bob" || got.Standby != "" || got.CanTakeOver {
		t.Fatalf("Succession = %+v; want bob super with no standby", got)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
		t.Fatal("the former super administrator kept the key")
	}
	if e := latestEvent(t, store); e.Actor != "bob" || e.Action != AccountSuperTakenOver || e.Username != "alice" ||
		e.Detail != "no sign-in for at least 31 days; administrator key held" {
		t.Fatalf("latest event = %+v; want bob's takeover from alice recorded", e)
	}
	dek, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "checking")
	if err != nil {
		t.Fatalf("the new super administrator opening data: %v", err)
	}
	clear(dek)
	if _, err := store.OpenUserForAdmin("alice", aliceDEK, "carol", "checking"); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("the former super administrator opening data = %v, want ErrNotSuper", err)
	}
	// The role was just given, so nobody can take it straight back.
	if _, err := store.TakeOverSuper("dave", daveDEK); !errors.Is(err, ErrTakeoverNotDue) {
		t.Fatalf("a second takeover = %v, want ErrTakeoverNotDue", err)
	}
	// The returning super administrator gets it back by hand-over.
	if keyPassed, err := store.HandOverSuper("bob", bobDEK, "alice"); err != nil || !keyPassed {
		t.Fatalf("handing back = %v, %v", keyPassed, err)
	}
	openEscrow(t, store, "alice", aliceDEK)
}

func TestAnyAdministratorTakesOverWhenNoStandbyIsNamed(t *testing.T) {
	store, aliceDEK, bobDEK, _ := standbyStore(t)
	carolDEK, err := store.UnlockDEK("carol", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(carol): %v", err)
	}
	defer clear(carolDEK)
	superInactiveFor(t, store, 31*day)

	if _, err := store.TakeOverSuper("alice", aliceDEK); !errors.Is(err, ErrTargetIsSuper) {
		t.Fatalf("the super administrator taking over = %v, want ErrTargetIsSuper", err)
	}
	if _, err := store.TakeOverSuper("carol", carolDEK); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("a standard account taking over = %v, want ErrNotAdmin", err)
	}
	keyHeld, err := store.TakeOverSuper("bob", bobDEK)
	if err != nil || keyHeld {
		t.Fatalf("TakeOverSuper(bob) = %v, %v; want the role without the key", keyHeld, err)
	}
	if super := mustSuper(t, store); super != "bob" {
		t.Fatalf("super administrator = %q, want bob", super)
	}
	if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
		t.Fatalf("%d grants remain; nobody holds the key after a takeover with no standby", n)
	}
	if e := latestEvent(t, store); e.Detail != "no sign-in for at least 31 days; the new super administrator does not hold the administrator key" {
		t.Fatalf("latest event = %+v; want the missing key recorded", e)
	}
	if _, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "checking"); !errors.Is(err, ErrNoEscrowGrant) {
		t.Fatalf("opening data without the key = %v, want ErrNoEscrowGrant", err)
	}
}

// The dormant-administrator case the owner asked to close: on an upgraded
// install the earliest administrator is made super and never signs in.
func TestAnUpgradedInstallsDormantSuperCanBeTakenOver(t *testing.T) {
	store := openTestStore(t)
	for _, name := range []string{"carol", "bob"} {
		if _, err := store.CreateAccount(name, name, statusPassword, true); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	mustExec(t, store, `UPDATE users SET created_at = '2026-01-01T00:00:00Z' WHERE username = 'carol'`)
	bobDEK, err := store.UnlockDEK("bob", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(bob): %v", err)
	}
	defer clear(bobDEK)
	if super := mustSuper(t, store); super != "carol" {
		t.Fatalf("super administrator = %q, want carol", super)
	}
	if _, err := store.TakeOverSuper("bob", bobDEK); !errors.Is(err, ErrTakeoverNotDue) {
		t.Fatalf("takeover right after the upgrade = %v, want ErrTakeoverNotDue", err)
	}
	mustExec(t, store, `UPDATE super_admin SET assigned_at = ?`, time.Now().UTC().Add(-31*day).Format(time.RFC3339Nano))
	if _, err := store.TakeOverSuper("bob", bobDEK); err != nil {
		t.Fatalf("takeover from a dormant super administrator: %v", err)
	}
	if super := mustSuper(t, store); super != "bob" {
		t.Fatalf("super administrator = %q, want bob", super)
	}
	// No administrator key ever existed on this install; the new super
	// administrator's sign-in creates it, so the role ends up usable.
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
	openEscrow(t, store, "bob", bobDEK)
}

// Saving the same standby again while the super administrator's own key
// does not open reports and records what the standby still holds.
func TestNamingTheSameStandbyAgainReportsTheKeyTheyHold(t *testing.T) {
	store, aliceDEK, _, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`)

	keyPassed, err := store.SetStandby("alice", aliceDEK, "bob", 40)
	if err != nil || !keyPassed {
		t.Fatalf("SetStandby(bob) again = %v, %v; bob still holds the key", keyPassed, err)
	}
	if e := latestEvent(t, store); e.Detail != "takes over after 40 days without a sign-in by the super administrator; administrator key passed" {
		t.Fatalf("latest event = %+v; want it to say bob holds the key", e)
	}

	// dave has no key and alice's does not open: named without it.
	keyPassed, err = store.SetStandby("alice", aliceDEK, "dave", 40)
	if err != nil || keyPassed {
		t.Fatalf("SetStandby(dave) = %v, %v; want named without the key", keyPassed, err)
	}
	if e := latestEvent(t, store); e.Detail != "takes over after 40 days without a sign-in by the super administrator; the administrator key could not be passed" {
		t.Fatalf("latest event = %+v; want the missing key recorded", e)
	}
	if got := mustSuccession(t, store, "alice"); got.Standby != "dave" || got.StandbyHoldsKey {
		t.Fatalf("Succession = %+v; want dave without the key", got)
	}
}

// Losing the administrator role in any way ends being the standby.
func TestTheStandbyIsClearedWhenTheirRoleChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(store *Store, aliceDEK []byte) error
		detail string
		key    bool // whether bob keeps a grant
	}{
		{"demoted", func(s *Store, _ []byte) error { return s.SetAdmin("alice", "bob", false) }, "no longer an administrator", false},
		{"disabled", func(s *Store, _ []byte) error { return s.SetDisabled("alice", "bob", true) }, "disabled", false},
		{"deleted", func(s *Store, _ []byte) error { return s.PurgeAccount("alice", "bob") }, "deleted", false},
		{"made super administrator", func(s *Store, dek []byte) error { _, err := s.HandOverSuper("alice", dek, "bob"); return err }, "became the super administrator", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, aliceDEK, _, _ := standbyStore(t)
			mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
			if err := tc.change(store, aliceDEK); err != nil {
				t.Fatalf("change: %v", err)
			}
			var standby any
			if err := store.conn.QueryRow(`SELECT standby FROM super_succession`).Scan(&standby); err != nil || standby != nil {
				t.Fatalf("standby column = %v, %v; want it cleared", standby, err)
			}
			if n := countRows(t, store, "escrow_grants", "admin_username = 'bob'"); (n == 1) != tc.key {
				t.Fatalf("bob holds %d grants; want key=%v", n, tc.key)
			}
			requireEvent(t, store, "alice super_standby_removed bob")
			events, _ := store.AccountEvents()
			for _, e := range events {
				if e.Action == AccountStandbyRemoved && e.Detail != tc.detail {
					t.Fatalf("removal recorded as %q, want %q", e.Detail, tc.detail)
				}
			}
		})
	}
}

// The standby column is plaintext like the role row: one naming someone who
// cannot be the standby is cleared when read.
func TestForgedStandbyRowsAreNotTrusted(t *testing.T) {
	store, aliceDEK, _, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)

	mustExec(t, store, `UPDATE super_succession SET standby = 'carol'`)
	if got := mustSuccession(t, store, "alice"); got.Standby != "" {
		t.Fatalf("standby = %q; a standard account must not be trusted as standby", got.Standby)
	}
	requireEvent(t, store, "alice super_standby_removed carol")

	mustExec(t, store, `UPDATE super_succession SET standby = 'alice'`)
	if got := mustSuccession(t, store, "alice"); got.Standby != "" {
		t.Fatalf("standby = %q; the super administrator cannot be their own standby", got.Standby)
	}
	openEscrow(t, store, "alice", aliceDEK) // the super administrator's key stays

	// A period edited out of range is read clamped to the allowed range.
	// This is input sanity only: whoever can edit the period can as easily
	// backdate the sign-in times.
	mustExec(t, store, `UPDATE super_succession SET takeover_days = 0`)
	superInactiveFor(t, store, 3*day)
	if got := mustSuccession(t, store, "bob"); got.TakeoverDays != MinTakeoverDays || got.CanTakeOver {
		t.Fatalf("Succession = %+v; want the period clamped to %d days and no takeover", got, MinTakeoverDays)
	}
}

// ADR-005's security rule has one exception: the standby holds a usable
// grant, so a role row forged to name them does open data. It is still
// recorded, and the user is still told.
func TestForgedSuperRowNamingTheStandbyOpensDataAndIsRecorded(t *testing.T) {
	store, aliceDEK, bobDEK, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	mustExec(t, store, `UPDATE super_admin SET username = 'bob'`)

	dek, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "forged")
	if err != nil {
		t.Fatalf("OpenUserForAdmin: %v", err)
	}
	clear(dek)
	requireEvent(t, store, "bob admin_access carol")
}

func TestTwoAdministratorsTakingOverAtOnce(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	stores := make([]*Store, 2)
	for i := range stores {
		store, err := Open(root)
		if err != nil {
			t.Fatalf("Open %d: %v", i, err)
		}
		t.Cleanup(func() { _ = store.Close() })
		stores[i] = store
	}
	aliceDEK := firstAdmin(t, stores[0])
	deks := map[string][]byte{}
	for _, name := range []string{"bob", "dave"} {
		deks[name] = addUser(t, stores[0], name, nil)
		if err := stores[0].PromoteAdmin("alice", aliceDEK, name); err != nil {
			t.Fatalf("PromoteAdmin(%s): %v", name, err)
		}
	}
	superInactiveFor(t, stores[0], 31*day)

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, name := range []string{"bob", "dave"} {
		wg.Go(func() {
			<-start
			_, errs[i] = stores[i].TakeOverSuper(name, deks[name])
		})
	}
	close(start)
	wg.Wait()

	won := 0
	for i, err := range errs {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, ErrTakeoverNotDue):
			t.Fatalf("process %d: %v; the loser must find the takeover no longer due", i, err)
		}
	}
	if won != 1 {
		t.Fatalf("%d takeovers succeeded, want exactly 1 (errors %v)", won, errs)
	}
	if n := countRows(t, stores[0], "account_events", "action = 'super_admin_taken_over'"); n != 1 {
		t.Fatalf("%d takeovers recorded, want 1", n)
	}
}
