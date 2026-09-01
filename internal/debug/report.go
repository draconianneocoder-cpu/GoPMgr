// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package debug provides structured, high-precision error reports for
// GoPMgr's self-healing diagnostics. Every recoverable error path SHOULD
// wrap the underlying error with debug.Wrap so a caller can recover the
// full report (timestamp, file:line, stack) via debug.Report instead of
// settling for an opaque string.
//
// Capture does that recovery and keeps the most recent reports in
// memory; RecentReports exposes them to the bug-report generator
// (App.GenerateBugReport, app_projects.go), which is the one place this
// package's structured detail reaches a user-facing surface today. Most
// debug.Wrap call sites still only reach the persistent log file, via
// Wrap's own log.Printf side effect.
package debug

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// ErrorReport is the canonical GoPMgr error envelope. JSON tags allow
// the Wails bridge to serialize it directly to the Svelte frontend.
type ErrorReport struct {
	Timestamp time.Time `json:"timestamp"`       // RFC3339Nano on the wire
	Context   string    `json:"context"`         // short tag, e.g. BACKUP_SNAPSHOT_FAILED
	Message   string    `json:"message"`         // human-readable
	File      string    `json:"file"`            // source file of the call site
	Line      int       `json:"line"`            // line number of the call site
	Stack     string    `json:"stack"`           // captured stack trace
	Cause     string    `json:"cause,omitempty"` // original error string, if any
}

// reportError is an error that wraps an ErrorReport. Returned by
// ErrorReport.ToError so callers can pass reports through standard
// `error` plumbing without losing the underlying data.
type reportError struct {
	r ErrorReport
}

func (e *reportError) Error() string { return e.r.Message }

// Report exposes the underlying ErrorReport for callers that recover from
// a returned error via errors.As.
func (e *reportError) Report() ErrorReport { return e.r }

// Wrap captures the caller's file:line, a stack trace, and a nanosecond
// timestamp around the given error. The `context` argument should be a
// short uppercase tag (BACKUP_SNAPSHOT_FAILED, CERT_BUNDLING_FAILED, ...) that
// the UI can match against to render a specific recovery hint.
//
// When err is non-nil, Wrap emits one line to the standard logger so the
// error reaches the persistent log file without any additional call sites.
//
// Passing a nil error returns a zero-value ErrorReport whose Message is
// empty; callers should not Wrap nil unless they specifically want a
// placeholder.
func Wrap(err error, context string) ErrorReport {
	_, file, line, _ := runtime.Caller(1)

	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)

	msg := context
	cause := ""
	if err != nil {
		msg = fmt.Sprintf("[%s] %v", context, err)
		cause = err.Error()
		log.Printf("debug: [%s] %v (at %s:%d)", context, err, file, line)
	}

	return ErrorReport{
		Timestamp: time.Now().UTC(),
		Context:   context,
		Message:   msg,
		File:      file,
		Line:      line,
		Stack:     string(buf[:n]),
		Cause:     cause,
	}
}

// ToError converts an ErrorReport back into a standard error value while
// preserving the underlying report. Call Report(err) to recover it.
func (r ErrorReport) ToError() error {
	return &reportError{r: r}
}

// Report attempts to extract the embedded ErrorReport from a standard
// error. Returns (zero, false) if err was not produced by Wrap.ToError().
func Report(err error) (ErrorReport, bool) {
	var re *reportError
	if errors.As(err, &re) {
		return re.r, true
	}
	return ErrorReport{}, false
}

// maxRecentReports bounds the in-memory ring buffer Capture fills, so a
// long session cannot grow it without limit.
const maxRecentReports = 5

var (
	recentMu      sync.Mutex
	recentReports = make([]ErrorReport, 0, maxRecentReports)
)

// Capture recovers the ErrorReport embedded in err (via Report) and, if
// present, appends it to a small in-memory ring buffer that
// RecentReports exposes. It always returns err unchanged, so a call site
// can capture and return in one step: `return debug.Capture(err)`.
func Capture(err error) error {
	report, ok := Report(err)
	if !ok {
		return err
	}
	recentMu.Lock()
	if len(recentReports) == maxRecentReports {
		copy(recentReports, recentReports[1:])
		recentReports[maxRecentReports-1] = report
	} else {
		recentReports = append(recentReports, report)
	}
	recentMu.Unlock()
	return err
}

// RecentReports returns a copy of the most recently Captured
// ErrorReports, oldest first. The bug-report generator uses this to
// include full diagnostic detail (stack, precise timestamp) that a plain
// error string loses.
func RecentReports() []ErrorReport {
	recentMu.Lock()
	defer recentMu.Unlock()
	out := make([]ErrorReport, len(recentReports))
	copy(out, recentReports)
	return out
}
