// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

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
	"testing/fstest"

	"github.com/wailsapp/wails/v3/pkg/application"

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
	acceptNoRecoveryCodes(t, app)
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

func findMenuItem(m *application.Menu, label string) *application.MenuItem {
	if m == nil {
		return nil
	}
	for i := 0; ; i++ {
		item := m.ItemAt(i)
		if item == nil {
			return nil
		}
		if item.Label() == label {
			return item
		}
	}
}

func TestBuildAppMenu_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Wails v3 macOS role construction is native-platform-specific")
	}
	application.New(application.Options{Name: "GoPMgr"})
	m := buildAppMenu(&App{}, "darwin")
	if m.FindByRole(application.AppMenu) == nil || m.ItemAt(0) != m.FindByRole(application.AppMenu) {
		t.Fatal("standard application menu must come first")
	}
	if m.FindByRole(application.About) != nil {
		t.Fatal("native About selector bypasses owned dialog admission")
	}
	if item := m.ItemAt(0).GetSubmenu().ItemAt(0); item.Label() != "About GoPMgr" {
		t.Fatal("application menu lost its About action")
	}
	file := findMenuItem(m, "File")
	if file == nil || file.GetSubmenu() == nil {
		t.Fatal("File submenu missing")
	}
	if findMenuItem(file.GetSubmenu(), "Quit") != nil {
		t.Fatal("duplicate macOS File/Quit")
	}
	if m.FindByRole(application.EditMenu) == nil || m.FindByRole(application.WindowMenu) == nil {
		t.Fatal("native editing/window roles missing")
	}
}

func TestBuildAppMenu_NonDarwin(t *testing.T) {
	application.New(application.Options{Name: "GoPMgr"})
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			m := buildAppMenu(&App{}, goos)
			if m.FindByRole(application.AppMenu) != nil {
				t.Fatal("macOS role on non-macOS")
			}
			file := findMenuItem(m, "File")
			if file == nil || file.GetSubmenu() == nil || findMenuItem(file.GetSubmenu(), "Quit") == nil {
				t.Fatal("non-macOS Quit missing")
			}
			window := findMenuItem(m, "Window")
			if window == nil || window.GetSubmenu() == nil || findMenuItem(window.GetSubmenu(), "Maximize / Restore") == nil || findMenuItem(window.GetSubmenu(), "Minimize") == nil {
				t.Fatal("window controls missing")
			}
			if m.FindByRole(application.WindowMenu) != nil {
				t.Fatal("macOS Window role on non-macOS")
			}
		})
	}
}

func TestBuildAppOptionsPreservesNativeWindowContract(t *testing.T) {
	app := &App{}
	opts := buildAppOptions(app, fstest.MapFS{})
	t.Cleanup(func() {
		if err := app.native.life.FinishAfterRun(); err != nil {
			t.Error(err)
		}
	})
	window := mainWindowOptions()
	if window.Title != "GoPMgr" || window.Width != 1280 || window.Height != 800 || window.MinWidth != 800 || window.MinHeight != 600 {
		t.Fatalf("window contract changed: %#v", window)
	}
	if window.DisableResize || window.MaximiseButtonState != application.ButtonEnabled {
		t.Fatal("native resize/zoom disabled")
	}
	if opts.Assets.Handler == nil || opts.Assets.Middleware == nil || len(opts.Services) != 1 || opts.Services[0].Instance() != app {
		t.Fatal("assets, admission or existing App service missing")
	}
	if opts.ShouldQuit == nil || opts.OnShutdown == nil || !opts.Mac.ApplicationShouldTerminateAfterLastWindowClosed {
		t.Fatal("central native close/shutdown contract missing")
	}
}

func TestNativeCloseGuardKeepsTerminalPermitAcrossHooks(t *testing.T) {
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
	if app.shouldPreventNativeClose() {
		t.Fatal("second native hook revoked accepted terminal close")
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

// Late enables must not revoke a terminal decision observed by both hooks.
func TestNativeCloseGuardConcurrentEnableCannotRevokeTerminalPermit(t *testing.T) {
	app := &App{ctx: context.Background()}
	app.EnableNativeCloseGuard()
	if err := app.completeNativeClose(func(context.Context) {}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var revoked atomic.Int64
	for range 200 {
		wg.Add(2)
		go func() { defer wg.Done(); app.EnableNativeCloseGuard() }()
		go func() {
			defer wg.Done()
			if app.shouldPreventNativeClose() {
				revoked.Add(1)
			}
		}()
	}
	wg.Wait()
	if revoked.Load() != 0 {
		t.Fatalf("terminal permission revoked %d times", revoked.Load())
	}
}
