// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"

	"gopmgr/internal/cli"
)

// TestHeadlessProjectMode covers headlessProjectMode's seven independent
// OR-clauses individually. Unlike a bare pass-through, dropping one of
// these clauses is a real bug class: it would silently route a CLI
// invocation that should run in headless maintenance mode into the GUI
// startup path instead (or vice versa), which is why this earns a test
// despite having no guard of its own.
func TestHeadlessProjectMode(t *testing.T) {
	cases := []struct {
		name string
		cfg  cli.Config
	}{
		{"CheckOnly", cli.Config{CheckOnly: true}},
		{"Repair", cli.Config{Repair: true}},
		{"Vacuum", cli.Config{Vacuum: true}},
		{"ExportAuditPath", cli.Config{ExportAuditPath: "/tmp/audit.json"}},
		{"ShowStats", cli.Config{ShowStats: true}},
		{"SchemaDump", cli.Config{SchemaDump: true}},
		{"ExportPath", cli.Config{ExportPath: "/tmp/export.csv"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !headlessProjectMode(&tc.cfg) {
				t.Fatalf("headlessProjectMode(%+v): want true, got false", tc.cfg)
			}
		})
	}

	if headlessProjectMode(&cli.Config{}) {
		t.Fatal("headlessProjectMode(zero value): want false, got true")
	}
}

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
	logPath := filepath.Join(dir, "gopmgr-x.log")
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
		"GoPMgr Diagnostic Report",
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
	logPath := filepath.Join(dir, "gopmgr-x.log")
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

// findMenuItem returns the first top-level item in m with the given label,
// or nil. Used to inspect buildAppMenu's output structurally instead of
// re-deriving it via runtime.GOOS, which is fixed for the whole test binary
// and can't exercise both platform branches in one run.
func findMenuItem(m *menu.Menu, label string) *menu.MenuItem {
	if m == nil {
		return nil
	}
	for _, item := range m.Items {
		if item.Label == label {
			return item
		}
	}
	return nil
}

// TestBuildAppMenu_Darwin pins the macOS branch: the native App menu is
// prepended (About/Hide/Quit come from there, so Quit must NOT also appear
// in the File menu), and Edit/Window are the OS-provided role menus rather
// than GoPMgr's own hand-built Window submenu.
func TestBuildAppMenu_Darwin(t *testing.T) {
	m := buildAppMenu(&App{}, "darwin")

	if len(m.Items) == 0 || m.Items[0].Role != menu.AppMenuRole {
		t.Fatalf("first top-level item = %+v, want the AppMenuRole item first", m.Items)
	}

	file := findMenuItem(m, "File")
	if file == nil || file.SubMenu == nil {
		t.Fatal("File submenu missing")
	}
	if findMenuItem(file.SubMenu, "Quit") != nil {
		t.Error("File submenu contains Quit on darwin; macOS gets Quit from the App menu, so this would show it twice")
	}

	var sawEditRole, sawWindowRole bool
	for _, item := range m.Items {
		switch item.Role {
		case menu.EditMenuRole:
			sawEditRole = true
		case menu.WindowMenuRole:
			sawWindowRole = true
		}
	}
	if !sawEditRole {
		t.Error("Edit role menu missing on darwin")
	}
	if !sawWindowRole {
		t.Error("Window role menu missing on darwin")
	}
	if findMenuItem(m, "Window") != nil {
		t.Error("a hand-built \"Window\" submenu is present on darwin; darwin should use the OS role menu instead")
	}
}

// TestBuildAppMenu_NonDarwin pins the everyone-else branch: no native App
// menu, so File must carry its own Quit item, and Window is GoPMgr's own
// submenu (Maximize/Minimize) rather than the macOS role menu.
func TestBuildAppMenu_NonDarwin(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			m := buildAppMenu(&App{}, goos)

			if len(m.Items) > 0 && m.Items[0].Role == menu.AppMenuRole {
				t.Error("AppMenuRole item present on non-darwin; that role is macOS-only")
			}

			file := findMenuItem(m, "File")
			if file == nil || file.SubMenu == nil {
				t.Fatal("File submenu missing")
			}
			quit := findMenuItem(file.SubMenu, "Quit")
			if quit == nil {
				t.Fatal("File submenu missing Quit on non-darwin; there is no App menu to provide it")
			}

			window := findMenuItem(m, "Window")
			if window == nil || window.SubMenu == nil {
				t.Fatal("hand-built Window submenu missing on non-darwin")
			}
			if findMenuItem(window.SubMenu, "Maximize / Restore") == nil {
				t.Error("Window submenu missing \"Maximize / Restore\"")
			}
			if findMenuItem(window.SubMenu, "Minimize") == nil {
				t.Error("Window submenu missing \"Minimize\"")
			}
			for _, item := range m.Items {
				if item.Role == menu.WindowMenuRole {
					t.Error("WindowMenuRole (macOS role menu) present on non-darwin; should use the hand-built submenu instead")
				}
			}
		})
	}
}

func TestBuildAppOptionsPreservesNativeWindowContract(t *testing.T) {
	app := &App{}
	opts := buildAppOptions(app)
	if opts.Title != "GoPMgr" || opts.Width != 1280 || opts.Height != 800 {
		t.Fatalf("unexpected main window options: %#v", opts)
	}
	if opts.MinWidth != 800 || opts.MinHeight != 600 {
		t.Fatalf("minimum window size = %dx%d, want 800x600", opts.MinWidth, opts.MinHeight)
	}
	if opts.Mac == nil {
		t.Fatal("macOS options are nil; Wails will disable the native zoom control")
	}
	if opts.Mac.DisableZoom {
		t.Fatal("native macOS zoom control is disabled")
	}
	if opts.Menu == nil || opts.AssetServer == nil || len(opts.Bind) != 1 {
		t.Fatal("centralized options dropped menu, assets, or App binding")
	}
}

func TestNativeCloseGuardConsumesOneShotPermit(t *testing.T) {
	app := &App{}
	if app.shouldPreventNativeClose() {
		t.Fatal("native close was prevented before the frontend guard was ready")
	}

	app.EnableNativeCloseGuard()
	if !app.shouldPreventNativeClose() {
		t.Fatal("native close was allowed without a frontend decision")
	}

	app.ctx = context.Background()
	quitCalls := 0
	if err := app.completeNativeClose(func(context.Context) {
		quitCalls++
		if app.shouldPreventNativeClose() {
			t.Error("native close permit was not established before quit")
		}
	}); err != nil {
		t.Fatalf("complete native close: %v", err)
	}
	if quitCalls != 1 {
		t.Fatalf("quit calls = %d, want 1", quitCalls)
	}
	if !app.shouldPreventNativeClose() {
		t.Fatal("native close remained allowed after one-shot permit was consumed")
	}
}

func TestCompleteNativeCloseRequiresRuntimeAndGuard(t *testing.T) {
	app := &App{}
	if err := app.completeNativeClose(func(context.Context) {}); err == nil {
		t.Fatal("complete native close without runtime succeeded")
	}

	app.ctx = context.Background()
	if err := app.completeNativeClose(func(context.Context) {}); err == nil {
		t.Fatal("complete native close without frontend guard succeeded")
	}
}
