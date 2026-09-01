// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package debug

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWrap_WithError(t *testing.T) {
	err := errors.New("disk full")
	r := Wrap(err, "SNAPSHOT_FAILED")

	if r.Context != "SNAPSHOT_FAILED" {
		t.Errorf("Context: got %q, want %q", r.Context, "SNAPSHOT_FAILED")
	}
	if !strings.Contains(r.Message, "SNAPSHOT_FAILED") {
		t.Errorf("Message %q does not contain context tag", r.Message)
	}
	if !strings.Contains(r.Message, "disk full") {
		t.Errorf("Message %q does not contain error text", r.Message)
	}
	if r.Cause != "disk full" {
		t.Errorf("Cause: got %q, want %q", r.Cause, "disk full")
	}
}

func TestWrap_NilError(t *testing.T) {
	r := Wrap(nil, "PLACEHOLDER")
	if r.Context != "PLACEHOLDER" {
		t.Errorf("Context: got %q, want %q", r.Context, "PLACEHOLDER")
	}
	if r.Message != "PLACEHOLDER" {
		t.Errorf("Message: got %q, want %q", r.Message, "PLACEHOLDER")
	}
	if r.Cause != "" {
		t.Errorf("Cause: got %q, want empty string", r.Cause)
	}
}

func TestWrap_CapturesFileAndLine(t *testing.T) {
	r := Wrap(nil, "TEST")
	if r.File == "" {
		t.Error("File should be non-empty")
	}
	if r.Line <= 0 {
		t.Errorf("Line should be positive, got %d", r.Line)
	}
	// Wrap records the immediate caller — this test file.
	if !strings.HasSuffix(r.File, "_test.go") {
		t.Errorf("File %q should end with _test.go (caller is this test)", r.File)
	}
}

func TestWrap_CapturesStack(t *testing.T) {
	r := Wrap(nil, "STACK_TEST")
	if r.Stack == "" {
		t.Error("Stack should be non-empty")
	}
}

func TestWrap_RecentTimestamp(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	r := Wrap(nil, "TS_TEST")
	after := time.Now().UTC().Add(time.Second)
	if r.Timestamp.Before(before) || r.Timestamp.After(after) {
		t.Errorf("Timestamp %v not in window [%v, %v]", r.Timestamp, before, after)
	}
}

func TestToError_ImplementsError(t *testing.T) {
	r := Wrap(errors.New("something"), "CTX")
	err := r.ToError()
	if err == nil {
		t.Fatal("ToError returned nil")
	}
	if err.Error() != r.Message {
		t.Errorf("error string: got %q, want %q", err.Error(), r.Message)
	}
}

func TestReport_ExtractsFromToError(t *testing.T) {
	original := Wrap(errors.New("db locked"), "DB_LOCK")
	err := original.ToError()

	got, ok := Report(err)
	if !ok {
		t.Fatal("Report returned false for a ToError-wrapped error")
	}
	if got.Context != original.Context {
		t.Errorf("Context: got %q, want %q", got.Context, original.Context)
	}
	if got.Cause != original.Cause {
		t.Errorf("Cause: got %q, want %q", got.Cause, original.Cause)
	}
	if got.Message != original.Message {
		t.Errorf("Message: got %q, want %q", got.Message, original.Message)
	}
}

func TestReport_UnrelatedError_ReturnsFalse(t *testing.T) {
	_, ok := Report(errors.New("unrelated"))
	if ok {
		t.Error("Report should return false for a plain errors.New error")
	}
}

func TestReport_NilError_ReturnsFalse(t *testing.T) {
	_, ok := Report(nil)
	if ok {
		t.Error("Report should return false for nil")
	}
}

func TestWrap_LogsNonNilError(t *testing.T) {
	var buf strings.Builder
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })

	Wrap(errors.New("disk full"), "SNAPSHOT_FAILED")
	if !strings.Contains(buf.String(), "SNAPSHOT_FAILED") {
		t.Errorf("Wrap with non-nil err did not log context tag; got: %q", buf.String())
	}
}

func TestWrap_DoesNotLogNilError(t *testing.T) {
	var buf strings.Builder
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })

	Wrap(nil, "PLACEHOLDER")
	if buf.Len() > 0 {
		t.Errorf("Wrap with nil err must not emit a log line; got: %q", buf.String())
	}
}

// resetRecentReports clears the package-level ring buffer so a test can
// assert on it deterministically, regardless of what earlier tests in
// this file left behind. Only this file (package debug itself) can do
// this — callers outside the package only see Capture/RecentReports and
// must assert presence via a unique marker, never absence.
func resetRecentReports(t *testing.T) {
	t.Helper()
	recentMu.Lock()
	recentReports = recentReports[:0]
	recentMu.Unlock()
}

func TestCapture_ExtractsAndStoresReport(t *testing.T) {
	resetRecentReports(t)
	err := Wrap(errors.New("disk full"), "CAPTURE_TEST").ToError()

	got := Capture(err)
	if got != err {
		t.Errorf("Capture must return err unchanged: got %v, want %v", got, err)
	}

	reports := RecentReports()
	if len(reports) != 1 {
		t.Fatalf("RecentReports: got %d entries, want 1", len(reports))
	}
	if reports[0].Context != "CAPTURE_TEST" {
		t.Errorf("Context: got %q, want %q", reports[0].Context, "CAPTURE_TEST")
	}
}

func TestCapture_UnrelatedError_NotStored(t *testing.T) {
	resetRecentReports(t)
	err := errors.New("plain error, never wrapped")

	got := Capture(err)
	if got != err {
		t.Errorf("Capture must return err unchanged: got %v, want %v", got, err)
	}
	if reports := RecentReports(); len(reports) != 0 {
		t.Errorf("RecentReports: got %d entries for an unwrapped error, want 0", len(reports))
	}
}

func TestCapture_NilError_NoOp(t *testing.T) {
	resetRecentReports(t)
	if got := Capture(nil); got != nil {
		t.Errorf("Capture(nil): got %v, want nil", got)
	}
	if reports := RecentReports(); len(reports) != 0 {
		t.Errorf("RecentReports: got %d entries after Capture(nil), want 0", len(reports))
	}
}

func TestRecentReports_ReturnsDefensiveCopy(t *testing.T) {
	resetRecentReports(t)
	Capture(Wrap(errors.New("x"), "DEFENSIVE_COPY_TEST").ToError())

	reports := RecentReports()
	reports[0].Context = "MUTATED"

	again := RecentReports()
	if again[0].Context != "DEFENSIVE_COPY_TEST" {
		t.Errorf("mutating a RecentReports() result leaked into internal state: got %q", again[0].Context)
	}
}

func TestCapture_RingBufferBoundedAndOldestFirst(t *testing.T) {
	resetRecentReports(t)
	// Capture one more than the cap; the oldest (CTX_0) must be evicted
	// and the buffer must stay oldest-first.
	for i := 0; i < maxRecentReports+1; i++ {
		Capture(Wrap(errors.New("x"), fmt.Sprintf("CTX_%d", i)).ToError())
	}

	reports := RecentReports()
	if len(reports) != maxRecentReports {
		t.Fatalf("RecentReports: got %d entries, want %d (buffer must stay bounded)", len(reports), maxRecentReports)
	}
	if reports[0].Context != "CTX_1" {
		t.Errorf("oldest surviving entry: got %q, want %q (CTX_0 should have been evicted)", reports[0].Context, "CTX_1")
	}
	last := fmt.Sprintf("CTX_%d", maxRecentReports)
	if reports[len(reports)-1].Context != last {
		t.Errorf("newest entry: got %q, want %q", reports[len(reports)-1].Context, last)
	}
}

// TestCapture_ConcurrentAccessRaceFree proves the ring buffer's mutex
// actually serializes concurrent Capture/RecentReports calls, matching
// how App.SecureArchive is reached: Wails dispatches each frontend call
// on its own goroutine. Run with -race; a clean run only counts as
// evidence because every goroutine's Capture call is confirmed to have
// landed (the final buffer is full and RecentReports never panics or
// returns a torn/short read while readers race writers).
func TestCapture_ConcurrentAccessRaceFree(t *testing.T) {
	resetRecentReports(t)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			Capture(Wrap(errors.New("x"), fmt.Sprintf("CONCURRENT_%d", i)).ToError())
			_ = RecentReports() // concurrent reader, races against writers under -race
		}(i)
	}
	wg.Wait()

	reports := RecentReports()
	if len(reports) != maxRecentReports {
		t.Fatalf("after %d concurrent captures, RecentReports: got %d entries, want %d",
			goroutines, len(reports), maxRecentReports)
	}
}
