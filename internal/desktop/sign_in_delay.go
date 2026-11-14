// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"sync"
	"time"
)

// Sign-in delay (owner decision of 2026-11-14, SEC-04 items 3 and 4): after
// repeated failed sign-ins, each further failure waits before it answers,
// up to signInDelayCap. Nothing locks an account out. An unknown username
// counts as a failure exactly as a wrong password does, so the delay does
// not tell them apart.
const (
	// signInFreeFailures is how many failures in a row answer at once, so
	// that a mistyped password costs nothing.
	signInFreeFailures = 2
	signInFirstDelay   = time.Second
	signInDelayCap     = 5 * time.Second
	// signInFailureMemory is how long after the last failure the count
	// starts again from zero.
	signInFailureMemory = 15 * time.Minute
)

// signInDelay is the wait after the failures-th failure in a row: none for
// the first signInFreeFailures, then signInFirstDelay doubling with each
// further failure, never more than signInDelayCap.
func signInDelay(failures int) time.Duration {
	extra := failures - signInFreeFailures
	if extra <= 0 {
		return 0
	}
	d := signInFirstDelay
	for i := 1; i < extra && d < signInDelayCap; i++ {
		d *= 2
	}
	return min(d, signInDelayCap)
}

// signInThrottle runs one sign-in at a time and delays failures. Its zero
// value is ready to use with the real clock.
type signInThrottle struct {
	// gate is held for a whole attempt, its delay included, so attempts
	// sent together queue behind each other's delays instead of
	// overlapping them.
	gate sync.Mutex

	failures    int
	lastFailure time.Time

	// now and sleep replace the clock in tests; nil means the real one.
	now   func() time.Time
	sleep func(time.Duration)
}

// attempt runs signIn. When invalid reports that its error is a failed
// credential check, it counts the failure and waits that failure's delay
// before returning; any success clears the count. Other errors, such as a
// disabled account (known only after the password matched), leave the
// count as it is.
func (t *signInThrottle) attempt(signIn func() error, invalid func(error) bool) error {
	t.gate.Lock()
	defer t.gate.Unlock()
	err := signIn()
	if err == nil {
		t.failures = 0
		return nil
	}
	if !invalid(err) {
		return err
	}
	now := t.clock()
	if t.failures > 0 && now.Sub(t.lastFailure) >= signInFailureMemory {
		t.failures = 0
	}
	t.failures++
	t.lastFailure = now
	if d := signInDelay(t.failures); d > 0 {
		t.wait(d)
	}
	return err
}

func (t *signInThrottle) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func (t *signInThrottle) wait(d time.Duration) {
	if t.sleep != nil {
		t.sleep(d)
		return
	}
	time.Sleep(d)
}
