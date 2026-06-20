// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package applog

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restoreLogger snapshots and restores the global logger so an Init call
// inside a test does not leak its output/flags into other tests.
func restoreLogger(t *testing.T) {
	t.Helper()
	w := log.Writer()
	flags := log.Flags()
	prefix := log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(w)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
}

func TestInitWritesDatedLogFile(t *testing.T) {
	restoreLogger(t)

	dir := t.TempDir()
	logPath, cleanup := Init(dir)
	if logPath == "" {
		t.Fatal("Init returned an empty log path for a writable directory")
	}

	wantDir := filepath.Join(dir, "logs")
	if got := filepath.Dir(logPath); got != wantDir {
		t.Fatalf("log directory = %q, want %q", got, wantDir)
	}
	base := filepath.Base(logPath)
	if !strings.HasPrefix(base, "pmforge-") || !strings.HasSuffix(base, ".log") {
		t.Fatalf("log filename %q does not match pmforge-<date>.log", base)
	}

	log.Print("canary-marker-12345")
	cleanup()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "canary-marker-12345") {
		t.Fatalf("log file did not capture the message; contents:\n%s", data)
	}
}

func TestInitAppendsAcrossCalls(t *testing.T) {
	restoreLogger(t)

	dir := t.TempDir()
	path1, cleanup1 := Init(dir)
	log.Print("first-line-marker")
	cleanup1()

	path2, cleanup2 := Init(dir)
	log.Print("second-line-marker")
	cleanup2()

	if path1 != path2 {
		t.Fatalf("expected the same dated file across calls, got %q and %q", path1, path2)
	}
	data, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "first-line-marker") || !strings.Contains(got, "second-line-marker") {
		t.Fatalf("append mode lost a line; contents:\n%s", got)
	}
}

func TestLogDir(t *testing.T) {
	preferred := filepath.Join(t.TempDir(), "PMForge")
	want := filepath.Join(preferred, "logs")
	if got := LogDir(preferred); got != want {
		t.Fatalf("LogDir(%q) = %q, want %q", preferred, got, want)
	}
	if got := LogDir(""); got == "" {
		t.Fatal("LogDir(\"\") returned empty; expected a home/temp fallback")
	}
}

func TestResolveLogDir(t *testing.T) {
	preferred := filepath.Join("data", "PMForge")
	if got, want := resolveLogDir(preferred), filepath.Join(preferred, "logs"); got != want {
		t.Fatalf("resolveLogDir(%q) = %q, want %q", preferred, got, want)
	}

	// Empty and whitespace-only inputs must still yield a usable fallback.
	if got := resolveLogDir(""); got == "" {
		t.Fatal("resolveLogDir(\"\") returned empty; expected a home/temp fallback")
	}
	if got := resolveLogDir("   "); got == "" {
		t.Fatal("resolveLogDir(\"   \") returned empty; whitespace should be treated as unset")
	}
}

func TestFormatFatalIncludesContext(t *testing.T) {
	s := formatFatal("TITLE-TAG", "user-facing message", "/tmp/pmforge.log", errors.New("boom-cause"))
	for _, want := range []string{"TITLE-TAG", "user-facing message", "boom-cause", "/tmp/pmforge.log", "stack:"} {
		if !strings.Contains(s, want) {
			t.Errorf("formatFatal output missing %q; full output:\n%s", want, s)
		}
	}
}

func TestDialogMessage(t *testing.T) {
	m := dialogMessage("hello", "/tmp/pmforge.log", errors.New("boom-cause"))
	for _, want := range []string{"hello", "boom-cause", "/tmp/pmforge.log"} {
		if !strings.Contains(m, want) {
			t.Errorf("dialogMessage missing %q; got:\n%s", want, m)
		}
	}

	if got := dialogMessage("", "", nil); strings.TrimSpace(got) == "" {
		t.Fatal("dialogMessage with no detail returned empty; expected a default sentence")
	}
}
