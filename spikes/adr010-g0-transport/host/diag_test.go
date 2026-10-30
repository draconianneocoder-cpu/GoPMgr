// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"reflect"
	"testing"
	"time"
)

func TestUncaughtExceptions(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   int
	}{
		{"clean", "g0-timing: main=1\ng0-stats: \n", 0},
		{"one", "Exception in thread \"Thread-0\" java.nio.channels.ShutdownChannelGroupException\n\tat x\n", 1},
		{"two", "Exception in thread \"a\" X\nException in thread \"main\" Y\n", 2},
		{"logged, not uncaught", "WARNING: some Exception in handler\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := uncaughtExceptions(c.stderr); got != c.want {
				t.Fatalf("uncaughtExceptions = %d, want %d", got, c.want)
			}
		})
	}
}

func TestParseWorkerDiagNotes(t *testing.T) {
	stderr := "g0-timing: main=5,stream_error=10\r\n" +
		"g0-stats: bridge_write_failed=1\r\n" +
		"g0-native-at-exit: \r\n" +
		"g0-note: bridge write failed: java.io.IOException: The pipe is being closed\r\n" +
		"g0-note: second\n"
	d := parseWorkerDiag(stderr)
	want := []string{"bridge write failed: java.io.IOException: The pipe is being closed", "second"}
	if !reflect.DeepEqual(d.notes, want) {
		t.Fatalf("notes = %q, want %q", d.notes, want)
	}
	if d.stats["bridge_write_failed"] != 1 {
		t.Fatalf("stats = %v", d.stats)
	}
}

func TestGRPCLogFailureLines(t *testing.T) {
	c := &grpcLogCapture{}
	t0 := time.Now()
	c.lines = []grpcLogLine{
		{t0.Add(-time.Second), "transport: closing: EOF (before the window)"},
		{t0.Add(1 * time.Millisecond), "[core] Server created"},
		{t0.Add(2 * time.Millisecond), "transport: http2Server.HandleStreams failed to receive the preface from client: EOF"},
		{t0.Add(3 * time.Millisecond), "[transport] Closing: connection reset"},
		{t0.Add(time.Hour), "Error after the window"},
	}
	got := c.failureLines(t0, t0.Add(time.Minute), 5)
	want := []string{
		"transport: http2Server.HandleStreams failed to receive the preface from client: EOF",
		"[transport] Closing: connection reset",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("failureLines = %q, want %q", got, want)
	}
	if got := c.failureLines(t0, t0.Add(time.Minute), 1); len(got) != 1 {
		t.Fatalf("limit not applied: %q", got)
	}
}

func TestGRPCLogCaptureIsBounded(t *testing.T) {
	c := &grpcLogCapture{}
	for i := 0; i < grpcLogLimit+10; i++ {
		if _, err := c.Write([]byte("line\n")); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.lines) != grpcLogLimit {
		t.Fatalf("kept %d lines, want %d", len(c.lines), grpcLogLimit)
	}
}

// A check that could not run must leave the scenario BLOCKED, never PASS,
// and a real finding must still FAIL.
func TestCommonChecksUnrunChecksBlock(t *testing.T) {
	cases := []struct {
		name    string
		metrics map[string]any
		want    string
	}{
		{"clean", map[string]any{"credential_leaked": false, "tcp_sockets": 0}, statusPass},
		{"leak found", map[string]any{"credential_leaked": true, "tcp_sockets": 0}, statusFail},
		{"tcp found", map[string]any{"credential_leaked": false, "tcp_sockets": 2}, statusFail},
		{"leak check did not run", map[string]any{"credential_leak_check_error": "fork/exec /bin/ps: operation not permitted", "tcp_sockets": 0}, statusBlocked},
		{"tcp check did not run", map[string]any{"credential_leaked": false, "tcp_socket_check_error": "lsof: exit status 2"}, statusBlocked},
		{"fail outranks blocked", map[string]any{"credential_leaked": true, "tcp_socket_check_error": "lsof: exit status 2"}, statusFail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := newEvidence("handshake", "test", nil)
			commonChecks(&jobRun{metrics: c.metrics}, ev)
			if ev.Status != c.want {
				t.Fatalf("status = %s, want %s (notes %q)", ev.Status, c.want, ev.Notes)
			}
		})
	}
}
