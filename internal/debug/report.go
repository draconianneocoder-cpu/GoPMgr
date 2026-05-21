// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package debug provides structured, high-precision error reports for
// PMForge's self-healing diagnostics. Every recoverable error path SHOULD
// wrap the underlying error with debug.Wrap so the UI can surface a
// full report (timestamp, file:line, stack) instead of an opaque string.
package debug

import (
	"errors"
	"fmt"
	"runtime"
	"time"
)

// ErrorReport is the canonical PMForge error envelope. JSON tags allow
// the Wails bridge to serialize it directly to the Svelte frontend.
type ErrorReport struct {
	Timestamp time.Time `json:"timestamp"`         // RFC3339Nano on the wire
	Context   string    `json:"context"`           // short tag, e.g. SNAPSHOT_FAILED
	Message   string    `json:"message"`           // human-readable
	File      string    `json:"file"`              // source file of the call site
	Line      int       `json:"line"`              // line number of the call site
	Stack     string    `json:"stack"`             // captured stack trace
	Cause     string    `json:"cause,omitempty"`   // original error string, if any
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
// short uppercase tag (SNAPSHOT_FAILED, CERT_BUNDLING_FAILED, ...) that
// the UI can match against to render a specific recovery hint.
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
// preserving the underlying report. Use errors.As(err, &target) where
// target is *Report{} (defined below) to recover it.
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
