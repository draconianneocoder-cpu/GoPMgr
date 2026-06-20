// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package applog wires PMForge's process-level diagnostic logging and
// fatal-startup handling.
//
// Motivation: a Wails GUI binary launched from Finder/Explorer/.desktop
// has its stdout/stderr routed to a null sink, so a plain log.Fatalf at
// startup makes the application die with no window, no dialog, and no
// trace - the exact "it never starts" failure mode. applog fixes both
// halves of that problem:
//
//   - Init tees the standard logger to BOTH stderr (so `wails dev` and
//     terminal launches still show output) AND a dated file under the
//     PMForge data tree, so maintainers always have a log to read.
//   - Fatal records the error (with a stack trace) to that log, shows a
//     native OS error dialog so a GUI launch can never fail silently,
//     then exits non-zero.
//
// The package is intentionally stdlib-only (native dialogs are invoked
// through the OS's own tooling via os/exec) so it adds no dependency and
// is safe to call before any heavier subsystem is initialised.
package applog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600
)

// Init configures the standard logger to write to stderr and to a dated
// log file under <preferredDir>/logs (e.g. pmforge-2026-06-15.log).
//
// It never fails: if the directory or file cannot be created, logging
// falls back to stderr only, a warning is logged, and the returned
// logPath is empty. Pass preferredDir == "" to use a home/temp fallback.
//
// The returned cleanup closes the file; callers should `defer` it. Note
// that Fatal calls os.Exit and therefore bypasses deferred cleanups, but
// log writes are flushed to the OS on each call, so no data is lost.
func Init(preferredDir string) (logPath string, cleanup func()) {
	// file:line on every line is invaluable for post-mortem debugging.
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	logDir := resolveLogDir(preferredDir)
	if logDir == "" {
		log.SetOutput(os.Stderr)
		log.Print("applog: no writable log directory found; logging to stderr only")
		return "", func() {}
	}

	if err := os.MkdirAll(logDir, dirPerm); err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("applog: cannot create log dir %q: %v (logging to stderr only)", logDir, err)
		return "", func() {}
	}

	path := filepath.Join(logDir, fmt.Sprintf("pmforge-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm) // #nosec G304 -- path is composed internally from the PMForge data root, not user input.
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("applog: cannot open log file %q: %v (logging to stderr only)", path, err)
		return "", func() {}
	}
	// Tighten perms in case the process umask loosened them at create.
	_ = f.Chmod(filePerm)

	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return path, func() { _ = f.Close() }
}

// LogDir returns the directory where PMForge writes diagnostic logs for the
// given data root. Uses the same resolution as Init: <preferredDir>/logs,
// with home/temp fallbacks when preferredDir is empty or whitespace. The
// directory is not guaranteed to exist; callers should check or create it.
func LogDir(preferredDir string) string {
	return resolveLogDir(preferredDir)
}

// resolveLogDir picks the logs directory. The caller normally passes the
// resolved PMForge data root so logs sit beside system.db; the home/temp
// fallbacks only matter if that resolution failed upstream.
func resolveLogDir(preferredDir string) string {
	if strings.TrimSpace(preferredDir) != "" {
		return filepath.Join(preferredDir, "logs")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "Documents", "PMForge", "logs")
	}
	if tmp := os.TempDir(); tmp != "" {
		return filepath.Join(tmp, "PMForge", "logs")
	}
	return ""
}

// Fatal records a fatal startup error (with a stack trace) to the
// configured logger, shows a native error dialog so a GUI launch never
// dies silently, and exits the process with status 1.
//
// title is a short dialog/headline tag; userMessage is a plain-language
// sentence for the end user; logPath (may be empty) is surfaced so the
// user or a maintainer knows where the full details were written.
func Fatal(title, userMessage, logPath string, err error) {
	log.Print(formatFatal(title, userMessage, logPath, err))
	ShowError(title, dialogMessage(userMessage, logPath, err))
	os.Exit(1)
}

// formatFatal builds the multi-line record written to the log file. Kept
// separate from Fatal (which calls os.Exit) so it is unit-testable.
func formatFatal(title, userMessage, logPath string, err error) string {
	var b strings.Builder
	fmt.Fprintf(&b, "FATAL STARTUP: %s\n", title)
	if strings.TrimSpace(userMessage) != "" {
		fmt.Fprintf(&b, "  message: %s\n", userMessage)
	}
	if err != nil {
		fmt.Fprintf(&b, "  error:   %v\n", err)
	}
	if strings.TrimSpace(logPath) != "" {
		fmt.Fprintf(&b, "  log:     %s\n", logPath)
	}
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	fmt.Fprintf(&b, "  stack:\n%s", buf[:n])
	return b.String()
}

// dialogMessage builds the concise body shown in the native error dialog.
func dialogMessage(userMessage, logPath string, err error) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(userMessage) != "" {
		parts = append(parts, userMessage)
	}
	if err != nil {
		parts = append(parts, err.Error())
	}
	if strings.TrimSpace(logPath) != "" {
		parts = append(parts, "A detailed log was saved to:\n"+logPath)
	}
	if len(parts) == 0 {
		return "PMForge encountered a fatal startup error."
	}
	return strings.Join(parts, "\n\n")
}
