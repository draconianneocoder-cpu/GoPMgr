// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildTestLog constructs a slice of n lines, each "line NNN", and writes
// them joined with newlines to path.
func buildTestLog(t *testing.T, path string, n int) []string {
	t.Helper()
	lines := make([]string, n)
	for i := range lines {
		lines[i] = strings.Repeat("x", 60)         // ~60 chars per line; realistic log width
		lines[i] = strings.TrimRight(lines[i], "") // keep linter happy
	}
	// Use distinguishable first / last markers so truncation tests are unambiguous.
	lines[0] = "FIRST_LINE_SENTINEL"
	lines[n-1] = "LAST_LINE_SENTINEL"
	data := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write test log: %v", err)
	}
	return lines
}

func TestLogTail_ReturnsAllLinesWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	buildTestLog(t, path, 10)

	got, err := logTail(path, 200)
	if err != nil {
		t.Fatalf("logTail: %v", err)
	}
	if !strings.Contains(got, "FIRST_LINE_SENTINEL") {
		t.Error("logTail omitted the first line when under the limit")
	}
	if !strings.Contains(got, "LAST_LINE_SENTINEL") {
		t.Error("logTail omitted the last line")
	}
}

func TestLogTail_TruncatesExcessLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	buildTestLog(t, path, 300) // 300 lines, limit 200

	got, err := logTail(path, 200)
	if err != nil {
		t.Fatalf("logTail: %v", err)
	}
	// The first line of the 300-line file must be absent — it was trimmed.
	if strings.Contains(got, "FIRST_LINE_SENTINEL") {
		t.Error("logTail included the first line even though the log exceeds maxLines")
	}
	// The last line must always be present.
	if !strings.Contains(got, "LAST_LINE_SENTINEL") {
		t.Error("logTail omitted the last line")
	}
}

func TestLogTail_MissingFile(t *testing.T) {
	_, err := logTail(filepath.Join(t.TempDir(), "nope.log"), 200)
	if err == nil {
		t.Fatal("logTail with a missing file should return an error")
	}
}

func TestGenerateBugReport_WritesReport(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "pmforge-x.log")
	// Write a small log file so the tail section is non-empty.
	if err := os.WriteFile(logPath, []byte("line1\nline2\nline3\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	app := &App{logDir: dir, logPath: logPath}
	path, err := app.GenerateBugReport()
	if err != nil {
		t.Fatalf("GenerateBugReport: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"PMForge Diagnostic Report",
		runtime.GOOS,
		"=== Recent Log",
		"line3",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("report missing %q; content:\n%s", want, content)
		}
	}
}

func TestGenerateBugReport_TailIsLast200Lines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "pmforge-x.log")
	buildTestLog(t, logPath, 300)

	app := &App{logDir: dir, logPath: logPath}
	path, err := app.GenerateBugReport()
	if err != nil {
		t.Fatalf("GenerateBugReport: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "FIRST_LINE_SENTINEL") {
		t.Error("bug report included log lines beyond the 200-line tail")
	}
	if !strings.Contains(content, "LAST_LINE_SENTINEL") {
		t.Error("bug report omitted the final log line")
	}
}

func TestGenerateBugReport_NoLogDir(t *testing.T) {
	app := &App{}
	_, err := app.GenerateBugReport()
	if err == nil {
		t.Fatal("GenerateBugReport with empty logDir should return an error")
	}
}
