// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"

	"gopmgr/internal/cli"
	"gopmgr/internal/debug"
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

// TestGenerateBugReport_IncludesRecentStructuredReports proves the
// consumer half of the wiring: debug.RecentReports() content reaches the
// bug-report file. Asserts by a unique per-test marker rather than exact
// buffer contents/count — debug's ring buffer is process-global package
// state, so other tests in this binary may have already populated it
// (see internal/debug/report_test.go's resetRecentReports, which only
// package debug's own tests can use).
func TestGenerateBugReport_IncludesRecentStructuredReports(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "gopmgr-x.log")
	if err := os.WriteFile(logPath, []byte("line1\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	marker := "BUG_REPORT_MARKER_" + t.Name()
	debug.Capture(debug.Wrap(errors.New("synthetic failure"), marker).ToError())

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

	for _, want := range []string{"=== Recent Structured Diagnostics ===", marker, "synthetic failure"} {
		if !strings.Contains(content, want) {
			t.Errorf("report missing %q; content:\n%s", want, content)
		}
	}
}

// TestSecureArchive_CapturesReportForBugReport is the end-to-end proof
// for this feature: a real, project-scoped, Wails-bound App.SecureArchive
// failure — reached the same way the frontend reaches it
// (ProjectSettings.svelte's createBackup) — must leave a structured
// report behind for a later App.GenerateBugReport call to surface. Forces
// the ARCHIVE_SETTINGS_LOAD_FAILED branch (internal/admin/workflow.go:34)
// the same way internal/admin/admin_test.go's
// TestSecureArchive_PropagatesSettingsLoadError does: close the DB before
// calling SecureArchive.
func TestSecureArchive_CapturesReportForBugReport(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	logDir := t.TempDir()
	app.logDir = logDir
	app.logPath = filepath.Join(logDir, "gopmgr-x.log")
	if err := os.WriteFile(app.logPath, []byte("line1\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	file, err := app.CreateProject("Secure Archive Test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := app.OpenProject(file.Path); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if err := app.db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if _, err := app.SecureArchive(file.Path); err == nil || !strings.Contains(err.Error(), "ARCHIVE_SETTINGS_LOAD_FAILED") {
		t.Fatalf("SecureArchive() error = %v, want ARCHIVE_SETTINGS_LOAD_FAILED", err)
	}

	path, err := app.GenerateBugReport()
	if err != nil {
		t.Fatalf("GenerateBugReport: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	content := string(data)

	// "ARCHIVE_SETTINGS_LOAD_FAILED" alone isn't a strong enough anchor:
	// Wrap also log.Printf's it, so it could in principle reach the report
	// via the "Recent Log" tail section instead of the structured-reports
	// section this test exists to prove. "cause: sql: database is closed"
	// is only ever written by the fmt.Fprintf in GenerateBugReport's new
	// section (app_projects.go), so its presence pins the right path.
	if !strings.Contains(content, "=== Recent Structured Diagnostics ===") {
		t.Fatalf("bug report is missing the structured-diagnostics section; content:\n%s", content)
	}
	if !strings.Contains(content, "cause: sql: database is closed") {
		t.Errorf("bug report's structured-diagnostics section does not include the SecureArchive failure captured moments earlier; content:\n%s", content)
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

// TestNativeCloseGuard_ConcurrentAccessRaceFree proves — under `go test
// -race`, not by reading the mutex and assuming — that EnableNativeCloseGuard,
// completeNativeClose, and shouldPreventNativeClose have no unsynchronized
// access to nativeCloseGuardReady/nativeClosePermit. This matters because the
// App struct's own concurrency-model comment above documents these fields as
// "shared mutable state" accessed from a fresh goroutine per Wails frontend
// call, but until this test, that claim was only exercised sequentially
// (TestNativeCloseGuardConsumesOneShotPermit) — a clean `-race` run over code
// that never ran concurrently is not evidence it is race-free.
//
// Fault-seed check performed manually while writing this test (not left as
// an untested assumption): temporarily removing shouldPreventNativeClose's
// `a.mu.Lock()`/`defer a.mu.Unlock()` and re-running `go test -race -run
// TestNativeCloseGuard_ConcurrentAccessRaceFree` reliably reported a DATA
// RACE on nativeClosePermit between that function and EnableNativeCloseGuard;
// restoring the lock made the race disappear. This confirms the test
// actually exercises the concurrent path the race detector needs to see —
// see TEST_COVERAGE_LEDGER.md's entry for this file for the full result.
func TestNativeCloseGuard_ConcurrentAccessRaceFree(t *testing.T) {
	app := &App{ctx: context.Background()}

	const n = 200
	var wg sync.WaitGroup
	wg.Add(n * 3)
	for range n {
		go func() {
			defer wg.Done()
			app.EnableNativeCloseGuard()
		}()
		go func() {
			defer wg.Done()
			_ = app.shouldPreventNativeClose()
		}()
		go func() {
			defer wg.Done()
			_ = app.completeNativeClose(func(context.Context) {})
		}()
	}
	wg.Wait()
}

// TestNativeCloseGuard_ConcurrentEnableRevokesPermits_Observation records a
// real, demonstrated consequence of EnableNativeCloseGuard,
// completeNativeClose, and shouldPreventNativeClose being three separate
// critical sections rather than one atomic operation: a permit
// completeNativeClose has just granted (and is about to consume via its own
// quit callback) can be silently revoked by a concurrent EnableNativeCloseGuard
// call landing in the gap between completeNativeClose's unlock and its quit
// callback's shouldPreventNativeClose check. This is a genuine TOCTOU gap in
// the exposed Go API's contract, not a data race (the mutex fully serializes
// every access; nothing is corrupted) and not a bug reachable from the
// current frontend: frontend/src/lib/native-close.ts's NativeCloseController
// only calls EnableNativeCloseGuard once at startup, or sequentially in the
// catch block after a CompleteNativeClose call has already rejected — both of
// completeNativeClose's Go-side error returns happen before
// nativeClosePermit is ever set to true, so the frontend never has an
// in-flight granted permit for a concurrent EnableNativeCloseGuard call to
// revoke.
//
// This is deliberately NOT a regression guard for the gap itself: the split
// between "revoked" and "consumed by self" is scheduler-dependent (348/500
// and 360/500 across two observed runs), so asserting a specific ratio, or
// even that a gap exists at all, would pin non-deterministic behavior this
// test doesn't control. If a future change merges the three critical
// sections into one atomic operation and closes the gap, this test must
// keep passing — that would be a fix, not a regression. The only invariant
// asserted is that the callback path actually runs; the logged ratio is
// evidence for a human reader, not a pass/fail condition. Re-check the
// JS-side reachability argument above if native-close.ts's control flow
// changes or a second Go caller of EnableNativeCloseGuard is added — that
// argument, not this test, is what keeps the gap unreachable in practice.
func TestNativeCloseGuard_ConcurrentEnableRevokesPermits_Observation(t *testing.T) {
	app := &App{ctx: context.Background()}
	app.EnableNativeCloseGuard()

	const n = 500
	var wg sync.WaitGroup
	var granted, consumedBySelf int64

	wg.Add(n * 2)
	for range n {
		go func() {
			defer wg.Done()
			_ = app.completeNativeClose(func(context.Context) {
				atomic.AddInt64(&granted, 1)
				if !app.shouldPreventNativeClose() {
					atomic.AddInt64(&consumedBySelf, 1)
				}
			})
		}()
		// A concurrent re-arm, interleaved with the completeNativeClose calls
		// above rather than run in its own separate loop, so it lands in the
		// narrow unlock-to-consume window this test is checking for.
		go func() {
			defer wg.Done()
			app.EnableNativeCloseGuard()
		}()
	}
	wg.Wait()

	if granted == 0 {
		t.Fatal("no completeNativeClose call ever ran its quit callback")
	}
	// Observational only — see the function comment for why this ratio is
	// not, and must not become, a pass/fail assertion.
	t.Logf("granted=%d consumedBySelf=%d (scheduler-dependent; a gap here is the documented TOCTOU behavior, not a failure, and its absence would mean the gap is closed, also not a failure)", granted, consumedBySelf)
}
