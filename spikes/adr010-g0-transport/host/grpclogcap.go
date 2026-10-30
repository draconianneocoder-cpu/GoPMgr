// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/grpclog"
)

// grpcLogLine is one line grpc-go logged, with the time it arrived.
type grpcLogLine struct {
	at   time.Time
	text string
}

// grpcLogCapture keeps grpc-go's own log lines, up to grpcLogLimit, so a
// failed job's evidence can show why the server dropped a connection.
// grpc-go logs no credentials: the proof travels in call metadata, which
// it does not log.
type grpcLogCapture struct {
	mu    sync.Mutex
	lines []grpcLogLine
}

const grpcLogLimit = 20_000

var hostGRPCLog = &grpcLogCapture{}

// captureGRPCLogs routes grpc-go's logger into hostGRPCLog. It must run
// before any gRPC call, as grpclog.SetLoggerV2 requires.
func captureGRPCLogs() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(hostGRPCLog, io.Discard, io.Discard))
}

func (c *grpcLogCapture) Write(p []byte) (int, error) {
	text := strings.TrimSpace(string(p))
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.lines) < grpcLogLimit {
		c.lines = append(c.lines, grpcLogLine{at: time.Now(), text: text})
	}
	return len(p), nil
}

// failureLines returns up to limit lines logged between from and to that
// report a failure or a closed connection, each cut to 300 characters.
func (c *grpcLogCapture) failureLines(from, to time.Time, limit int) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, l := range c.lines {
		if l.at.Before(from) || l.at.After(to) || !looksLikeFailure(l.text) {
			continue
		}
		t := l.text
		if len(t) > 300 {
			t = t[:300] + "…"
		}
		out = append(out, t)
		if len(out) == limit {
			break
		}
	}
	return out
}

func looksLikeFailure(s string) bool {
	s = strings.ToLower(s)
	for _, w := range []string{"fail", "error", "eof", "closing", "refused", "broken", "reset"} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}
