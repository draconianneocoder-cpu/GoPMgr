// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSignInDelaySchedule(t *testing.T) {
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, 0},
		{1, 0},
		{2, 0},
		{3, time.Second},
		{4, 2 * time.Second},
		{5, 4 * time.Second},
		{6, 5 * time.Second},
		{50, 5 * time.Second},
	}
	for _, tc := range cases {
		if got := signInDelay(tc.failures); got != tc.want {
			t.Errorf("signInDelay(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}
}

// fakeSignInClock records the throttle's waits and moves its clock by them.
type fakeSignInClock struct {
	now   time.Time
	waits []time.Duration
}

func (c *fakeSignInClock) install(t *signInThrottle) {
	t.now = func() time.Time { return c.now }
	t.sleep = func(d time.Duration) {
		c.waits = append(c.waits, d)
		c.now = c.now.Add(d)
	}
}

var errTestInvalid = errors.New("invalid credentials")

func isTestInvalid(err error) bool { return errors.Is(err, errTestInvalid) }

func TestSignInThrottleDelaysRepeatedFailures(t *testing.T) {
	errDisabled := errors.New("disabled")
	type step struct {
		result  error
		advance time.Duration // clock moves before this attempt
	}
	cases := []struct {
		name      string
		steps     []step
		wantWaits []time.Duration
	}{
		{
			name:      "two failures answer at once",
			steps:     []step{{result: errTestInvalid}, {result: errTestInvalid}},
			wantWaits: nil,
		},
		{
			name: "further failures wait longer, up to the cap",
			steps: []step{
				{result: errTestInvalid}, {result: errTestInvalid}, {result: errTestInvalid},
				{result: errTestInvalid}, {result: errTestInvalid}, {result: errTestInvalid},
			},
			wantWaits: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second},
		},
		{
			name: "a success starts the count again",
			steps: []step{
				{result: errTestInvalid}, {result: errTestInvalid}, {result: nil},
				{result: errTestInvalid}, {result: errTestInvalid}, {result: errTestInvalid},
			},
			wantWaits: []time.Duration{time.Second},
		},
		{
			name: "other errors neither count nor clear",
			steps: []step{
				{result: errTestInvalid}, {result: errDisabled}, {result: errTestInvalid},
				{result: errTestInvalid},
			},
			wantWaits: []time.Duration{time.Second},
		},
		{
			name: "the count is forgotten after a quiet spell",
			steps: []step{
				{result: errTestInvalid}, {result: errTestInvalid}, {result: errTestInvalid},
				{result: errTestInvalid, advance: signInFailureMemory},
				{result: errTestInvalid}, {result: errTestInvalid},
			},
			wantWaits: []time.Duration{time.Second, time.Second},
		},
		{
			name: "but not just before it",
			steps: []step{
				{result: errTestInvalid}, {result: errTestInvalid}, {result: errTestInvalid},
				{result: errTestInvalid, advance: signInFailureMemory - 2*time.Second},
			},
			wantWaits: []time.Duration{time.Second, 2 * time.Second},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var throttle signInThrottle
			clock := &fakeSignInClock{now: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)}
			clock.install(&throttle)
			for i, s := range tc.steps {
				clock.now = clock.now.Add(s.advance)
				err := throttle.attempt(func() error { return s.result }, isTestInvalid)
				if !errors.Is(err, s.result) || (s.result == nil) != (err == nil) {
					t.Fatalf("attempt %d returned %v, want %v", i+1, err, s.result)
				}
			}
			if !reflect.DeepEqual(clock.waits, tc.wantWaits) {
				t.Fatalf("waits = %v, want %v", clock.waits, tc.wantWaits)
			}
		})
	}
}

// Attempts sent together queue: the second does not start its password
// check until the first has finished its delay.
func TestSignInThrottleRunsOneAttemptAtATime(t *testing.T) {
	throttle := signInThrottle{failures: signInFreeFailures, lastFailure: time.Now()}
	waiting := make(chan struct{})
	release := make(chan struct{})
	throttle.sleep = func(time.Duration) {
		close(waiting)
		<-release
	}
	first := make(chan error, 1)
	go func() {
		first <- throttle.attempt(func() error { return errTestInvalid }, isTestInvalid)
	}()
	select {
	case <-waiting:
	case <-time.After(10 * time.Second):
		t.Fatal("the first attempt never reached its delay")
	}

	started := make(chan struct{})
	second := make(chan error, 1)
	go func() {
		second <- throttle.attempt(func() error { close(started); return nil }, isTestInvalid)
	}()
	select {
	case <-started:
		t.Fatal("a second attempt ran while the first was in its delay")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-first; !errors.Is(err, errTestInvalid) {
		t.Fatalf("first attempt = %v, want invalid credentials", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second attempt = %v, want success", err)
	}
}

// Through Login, an unknown username counts toward the delay exactly as a
// wrong password does, and gets the same message.
func TestLoginDelaysUnknownNamesLikeWrongPasswords(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	clock := &fakeSignInClock{now: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)}
	clock.install(&app.signIns)

	attempts := []struct{ username, password string }{
		{"nobody", "passphrase-long"},
		{"alice", "wrong-password"},
		{"nobody", "passphrase-long"},
		{"no such/name", "x"},
	}
	for _, a := range attempts {
		if _, err := app.Login(a.username, a.password); err == nil || err.Error() != "invalid credentials" {
			t.Fatalf("Login(%q) = %v, want invalid credentials", a.username, err)
		}
	}
	if want := []time.Duration{time.Second, 2 * time.Second}; !reflect.DeepEqual(clock.waits, want) {
		t.Fatalf("waits = %v, want %v", clock.waits, want)
	}
	if _, err := app.Login("alice", "passphrase-long"); err != nil {
		t.Fatalf("Login alice: %v", err)
	}
	if app.signIns.failures != 0 {
		t.Fatalf("failures after a successful sign-in = %d, want 0", app.signIns.failures)
	}
}
