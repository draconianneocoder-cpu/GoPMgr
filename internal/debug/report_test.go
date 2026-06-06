// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package debug

import (
	"errors"
	"strings"
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
