// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"gopmgr/internal/admin"
	"gopmgr/internal/applog"
	"gopmgr/internal/calendar"
	"gopmgr/internal/cli"
	"gopmgr/internal/db"
	"gopmgr/internal/export"
	"gopmgr/internal/kernel"
	"gopmgr/internal/sigma/service"
	"gopmgr/internal/templates"
	"gopmgr/internal/update"
	"gopmgr/internal/users"
)

// App is the Wails-exposed object. Every exported method becomes
// callable through the generated Wails v3 desktop.App service bindings.
//
// Concurrency model (see DEVELOPER_HANDBOOK.md §16 for the full invariants):
//
//   - The Wails runtime dispatches each frontend call on a fresh
//     goroutine, so every field below is shared mutable state.
//   - `mu` is an RWMutex. Reads (most chart/document fetches) take
//     RLock; writes (Login, Logout, OpenProject, CloseProject,
//     RepairAndSwap) take Lock.
//   - `store` is set once by NewApp and never re-assigned; methods
//     may read it without holding the lock.
//   - `user`, `db`, `dbPath`, `adminSvc` change with session state
//     and MUST be accessed under the lock. Helpers `requireUser()`
//     and `requireDB()` do the RLock/copy/RUnlock dance.
type App struct {
	ctx          context.Context
	native       *wailsRuntime
	shutdownOnce sync.Once

	mu    sync.RWMutex
	store *users.Store   // immutable after NewApp — safe to read without lock
	user  *users.Account // nil unless logged in
	dek   []byte         // ADR-001: session DEK, unlocked at login; nil when logged out
	// principal is the account incarnation the session signed in to
	// (users.SignIn), zero when logged out. Not sent to the frontend.
	principal users.Principal
	// epoch advances every time the signed-in session changes (sign-in,
	// sign-out, shutdown). An operation that captured the session before
	// slow work publishes or uses the session's key only if it has not moved
	// (session_state.go).
	epoch uint64
	// pendingCodes holds recovery codes prepared in App Settings but not
	// yet confirmed as saved; cleared on sign-out and shutdown.
	pendingCodes *users.PendingRecoveryCodes
	// noCodesAcceptedFor names the user who, this session, accepted that
	// projects they create or encrypt cannot be recovered without recovery
	// codes (AcceptEncryptionWithoutRecoveryCodes). Cleared on sign-in,
	// sign-out, and shutdown.
	noCodesAcceptedFor string
	// access is an administrator's recorded access to another account's
	// data (ADR-004), with its own key and project copy; nil when none.
	access    *adminAccess
	db        *db.Database // nil unless a project is open
	dbPath    string       // absolute path of the open project file (.gopmgr, or legacy .pmforge)
	adminSvc  *admin.Service
	templates *templates.Engine       // immutable after NewApp; safe lock-free read
	sigmaSvc  *service.ProjectService // initialized when a project is open

	// openCheck caches CheckOpenProject's result for the database handle it
	// checked; any open, reopen, or swap installs a new handle, which makes
	// it stale without a reset. Guarded by mu.
	openCheck *openProjectCheck

	// Diagnostic logging — set in main() after applog.Init; never reassigned.
	logPath string // dated log file path, e.g. .../logs/gopmgr-2026-06-20.log
	logDir  string // parent of logPath, e.g. .../logs

	nativeCloseGuardReady bool
	nativeClosePermit     bool
}

// NewApp constructs an App at process start. It opens the system DB
// up front (so the Login screen can list known users) and leaves the
// per-user / per-project handles for later.
func NewApp() (*App, error) {
	root, err := users.DefaultRootDir()
	if err != nil {
		return nil, err
	}
	// One-time relocation, now covering two prior moves: older macOS
	// installs kept their data under the iCloud-synced, TCC-protected
	// ~/Documents/PMForge (pre-2026-06), and every platform's most recent
	// pre-rename install used a "PMForge"-named root (pre-2026-08-04). Copy
	// whichever of those still exists into the new root before opening the
	// store so an existing user's accounts and projects survive both moves.
	// A failure here is non-fatal — we log it and fall through to a clean
	// new-location install rather than blocking startup.
	if migrated, mErr := users.MigrateLegacyRoot(root); mErr != nil {
		log.Printf("users: legacy data migration failed: %v (continuing with a fresh %s)", mErr, root)
	} else if migrated {
		log.Printf("users: migrated legacy PMForge-named data into %s", root)
	}
	store, err := users.Open(root)
	if err != nil {
		return nil, err
	}
	// templates.Engine wraps the zen-go decision engine driving the
	// Launchpad's seeding rules. Failing to initialise it is not
	// fatal — the GUI will fall back to "no auto-seed" — but we log
	// the error so misconfigured JDM doesn't pass silently.
	tmpl, err := templates.NewEngine()
	if err != nil {
		log.Printf("templates: engine init failed: %v (launchpad will skip seeding)", err)
		tmpl = nil
	}
	return &App{store: store, templates: tmpl}, nil
}

func (a *App) shutdown(_ context.Context) {
	a.shutdownOnce.Do(func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.clearSessionLocked()
		// The native host has already drained admitted calls. Keep the store
		// pointer immutable; closed stores also refuse non-native late callers.
		if a.store != nil {
			if err := a.store.Close(); err != nil {
				log.Printf("close account store: %v", err)
			}
		}
	})
}

// EnableNativeCloseGuard follows frontend listener registration. A late or
// duplicated enable must not revoke an already accepted terminal cycle.
func (a *App) EnableNativeCloseGuard() {
	a.mu.Lock()
	a.nativeCloseGuardReady = true
	a.mu.Unlock()
}

// CompleteNativeClose accepts the frontend's completed save/discard decision.
// The same permit covers window and application hooks. The host drains calls
// with the event loop alive before queuing the final native Quit.
func (a *App) CompleteNativeClose() error {
	if !a.native.ready() {
		return errors.New("native close: Wails runtime is not ready")
	}
	return a.completeNativeClose(func(context.Context) { a.native.beginQuit() })
}

func (a *App) completeNativeClose(quit func(context.Context)) error {
	a.mu.Lock()
	if a.nativeClosePermit {
		a.mu.Unlock()
		return nil
	}
	if a.ctx == nil {
		a.mu.Unlock()
		return errors.New("native close: Wails runtime is not ready")
	}
	if err := a.ctx.Err(); err != nil {
		a.mu.Unlock()
		return err
	}
	if !a.nativeCloseGuardReady {
		a.mu.Unlock()
		return errors.New("native close: frontend guard is not ready")
	}
	if a.native != nil {
		if err := a.native.life.ReserveQuit(); err != nil {
			a.mu.Unlock()
			return err
		}
	}
	a.nativeClosePermit = true
	ctx := a.ctx
	a.mu.Unlock()
	quit(ctx)
	return nil
}

func (a *App) shouldPreventNativeClose() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.nativeCloseGuardReady && !a.nativeClosePermit
}

func (a *App) beforeClose(_ context.Context) bool {
	if a.native == nil {
		return a.shouldPreventNativeClose()
	}
	if a.native.life.UIBusy() {
		return true
	}
	if a.shouldPreventNativeClose() {
		a.native.emit("app:before-close")
		return true
	}
	if a.native.life.Ready() {
		return false
	}
	// Also drain bootstrap closes. Establish the terminal cycle before a
	// concurrently arriving frontend enable can turn it into a new prompt.
	a.mu.Lock()
	a.nativeClosePermit = true
	if err := a.native.life.ReserveQuit(); err != nil {
		a.nativeClosePermit = false
		a.mu.Unlock()
		return true
	}
	a.mu.Unlock()
	a.native.beginQuit()
	return true
}

// =========================================================
// main: CLI dispatch + Wails launch
// =========================================================

// buildAppMenu keeps project navigation and native editing/window roles.
func buildAppMenu(app *App, goos string) *application.Menu {
	about := func(*application.Context) {
		app.native.showAbout(fmt.Sprintf(
			"GoPMgr %s\n\nLocal-first project controls.\nCopyright (C) 2026 James L. Burns and The GoPMgr Contributors.\nLicensed under GPL-3.0-or-later.", cli.Version))
	}
	emit := func(event string) func(*application.Context) {
		return func(*application.Context) { app.native.emit(event) }
	}
	m := application.NewMenu()
	if goos == "darwin" {
		m.AddRole(application.AppMenu)
		// The standard About selector bypasses Go callbacks. Route both menu
		// entries through the same owned, dismissible native dialog instead.
		m.FindByRole(application.About).SetRole(application.NoRole).OnClick(about)
	}
	file := m.AddSubmenu("File")
	file.Add("Dashboard").SetAccelerator("CmdOrCtrl+d").OnClick(emit("menu:dashboard"))
	file.Add("New Project").SetAccelerator("CmdOrCtrl+n").OnClick(emit("menu:new-project"))
	file.Add("Open Project…").SetAccelerator("CmdOrCtrl+o").OnClick(emit("menu:open-project"))
	file.AddSeparator()
	file.Add("Application Settings…").SetAccelerator("CmdOrCtrl+,").OnClick(emit("menu:app-settings"))
	file.Add("Project Settings…").OnClick(emit("menu:settings"))
	file.AddSeparator()
	file.Add("Close Project").SetAccelerator("CmdOrCtrl+w").OnClick(emit("menu:close-project"))
	if goos != "darwin" {
		file.AddSeparator()
		file.Add("Quit").SetAccelerator("CmdOrCtrl+q").OnClick(func(*application.Context) { app.native.requestQuit() })
	}
	if goos == "darwin" {
		m.AddRole(application.EditMenu)
		m.AddRole(application.WindowMenu)
	} else {
		window := m.AddSubmenu("Window")
		window.Add("Maximize / Restore").SetAccelerator("F11").OnClick(func(*application.Context) {
			app.native.withUI(func() { app.native.window.ToggleMaximise() })
		})
		window.Add("Minimize").OnClick(func(*application.Context) {
			app.native.withUI(func() { app.native.window.Minimise() })
		})
	}
	help := m.AddSubmenu("Help")
	help.Add("User Guide").OnClick(emit("menu:help"))
	help.Add("About GoPMgr").OnClick(about)
	return m
}

func mainWindowOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{Title: "GoPMgr", Width: 1280, Height: 800,
		MinWidth: 800, MinHeight: 600, MaximiseButtonState: application.ButtonEnabled}
}

func buildAppOptions(app *App, assets fs.FS) application.Options {
	app.native = newWailsRuntime()
	app.ctx = app.native.life.Context()
	return application.Options{
		Name:       "GoPMgr",
		Mac:        application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Assets:     application.AssetOptions{Handler: application.BundledAssetFileServer(assets), Middleware: app.native.life.Middleware},
		Services:   []application.Service{application.NewService(app)},
		ShouldQuit: func() bool { return !app.beforeClose(app.ctx) },
		OnShutdown: func() {
			// No wait on the native thread: full RPC response delivery was
			// joined before the host allowed this terminal Quit.
			if app.native.life.Ready() {
				app.shutdown(app.ctx)
			}
		},
	}
}

// Main runs GoPMgr: the CLI and headless project modes, or the Wails GUI
// serving assets (the built frontend, which the root main package embeds).
// It is a package function, not an App method, so Wails does not bind it.
func Main(assets fs.FS) {
	cfg := cli.ParseFlags()
	export.SetVersion(cli.Version)

	switch {
	case cfg.ShowVersion:
		cli.PrintVersion()
		return
	case cfg.UpdateCheck:
		update.Check()
		return
	}

	// CLI mode that operates on a single project file directly (.gopmgr,
	// or legacy .pmforge — this path doesn't check the extension)
	// (--check / --repair / --vacuum / --export-audit). Plaintext files
	// open directly; encrypted files require --username plus
	// --password-env so the user's DEK can be unlocked from system.db.
	if cfg.ProjectPath != "" && headlessProjectMode(cfg) {
		os.Exit(runHeadless(cfg))
	}

	// GUI mode.
	//
	// Initialise diagnostic logging before anything else in this path. A
	// Wails binary launched from Finder/Explorer/.desktop has its stderr
	// routed to a null sink, so a bare log.Fatalf here would make the app
	// die with no window and no trace. applog tees the log to stderr AND a
	// dated file under the GoPMgr data tree, and applog.Fatal additionally
	// shows a native error dialog so a startup failure is never silent.
	root, rootErr := users.DefaultRootDir()
	if rootErr != nil {
		// Non-fatal: applog falls back to a home/temp logs directory.
		root = ""
	}
	logPath, closeLog := applog.Init(root)
	defer closeLog()
	log.Printf("GoPMgr %s starting (pid=%d, %s/%s, %s)",
		cli.Version, os.Getpid(), runtime.GOOS, runtime.GOARCH, runtime.Version())

	app, err := NewApp()
	if err != nil {
		applog.Fatal("GoPMgr could not start",
			"GoPMgr failed to initialise its local data store.", logPath, err)
	}
	app.logPath = logPath
	app.logDir = applog.LogDir(root)

	gui := application.New(buildAppOptions(app, assets))
	window := gui.Window.NewWithOptions(mainWindowOptions())
	app.native.app, app.native.window = gui, window
	gui.Menu.Set(buildAppMenu(app, runtime.GOOS))
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if app.beforeClose(app.ctx) {
			event.Cancel()
		}
	})
	err = gui.Run()
	if finishErr := app.native.life.FinishAfterRun(); finishErr != nil {
		// A platform-loop failure after dispatch is not an empty startup
		// failure. Do not clear keys/close stores beneath active execution.
		applog.Fatal("GoPMgr could not stop safely", "The native loop stopped before desktop work drained.", logPath, errors.Join(err, finishErr))
	}
	app.shutdown(app.ctx)
	if err != nil {
		applog.Fatal("GoPMgr could not start",
			"GoPMgr failed to start its application window.", logPath, err)
	}
	log.Print("GoPMgr exited cleanly")
}

func headlessProjectMode(cfg *cli.Config) bool {
	return cfg.CheckOnly || cfg.Repair || cfg.Vacuum || cfg.ExportAuditPath != "" ||
		cfg.ShowStats || cfg.SchemaDump || cfg.ExportPath != ""
}

// runHeadlessRepair prints --repair's log and, when it wrote a rebuilt
// copy, how to use it: the CLI never replaces the project file, and only
// the app checks a copy before swapping it in. damaged reports that it
// found damage and wrote a copy, so the project file still needs repair.
func runHeadlessRepair(d *db.Database, projectPath string, w io.Writer) (damaged bool, err error) {
	result, err := d.InformativeSelfHeal(projectPath)
	lines := append([]string{}, result.Log...)
	if err == nil && result.Snapshot != "" {
		lines = append(lines,
			"The project file was not changed.",
			"To repair the project, open it in GoPMgr and choose Check and repair in Project Settings › Data Protection.",
			"GoPMgr checks a rebuilt copy before using it and keeps the damaged file as "+projectPath+".corrupt.",
		)
	}
	if _, werr := io.WriteString(w, strings.Join(lines, "\n")+"\n"); werr != nil {
		return false, fmt.Errorf("print repair log: %w", werr)
	}
	if err != nil {
		return false, err
	}
	return result.Snapshot != "", nil
}

func openHeadlessDB(cfg *cli.Config) (*db.Database, error) {
	encrypted, err := db.IsEncryptedFile(cfg.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("inspect project encryption: %w", err)
	}
	if !encrypted {
		if cfg.Repair || cfg.Vacuum {
			return db.InitDB(cfg.ProjectPath)
		}
		return db.OpenReadOnly(cfg.ProjectPath)
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return nil, errors.New("--username is required for encrypted project maintenance")
	}
	if strings.TrimSpace(cfg.PasswordEnv) == "" {
		return nil, errors.New("--password-env is required for encrypted project maintenance")
	}
	password, ok := os.LookupEnv(cfg.PasswordEnv)
	if !ok || password == "" {
		return nil, errPasswordEnvUnset
	}
	rootDir, err := inferHeadlessRootDir(cfg.ProjectPath, cfg.Username)
	if err != nil {
		return nil, err
	}
	store, err := users.Open(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open system database: %w", err)
	}
	defer func() { _ = store.Close() }()
	_, _, dek, err := store.SignInHeadless(cfg.Username, password)
	if err != nil {
		return nil, fmt.Errorf("authenticate headless user: %w", err)
	}
	defer zeroBytes(dek)
	// Repair and vacuum write. Every other headless command only reads,
	// so it must not migrate the file or switch its journal mode.
	if cfg.Repair || cfg.Vacuum {
		return db.InitEncryptedDB(cfg.ProjectPath, dek)
	}
	return db.OpenEncryptedReadOnly(cfg.ProjectPath, dek)
}

func inferHeadlessRootDir(projectPath, username string) (string, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", err
	}
	projectsDir := filepath.Dir(absPath)
	// Current layout nests each project in its own subfolder:
	// <root>/<username>/projects/<id>/project.gopmgr (or, for a project
	// created before the 2026-08-04 rename, project.pmforge). Step up out of the
	// per-project subfolder so projectsDir points at ".../projects".
	if filepath.Base(projectsDir) != "projects" {
		projectsDir = filepath.Dir(projectsDir)
	}
	userDir := filepath.Dir(projectsDir)
	if filepath.Base(projectsDir) != "projects" || filepath.Base(userDir) != username {
		return "", fmt.Errorf("encrypted headless project must be under <gopmgr-root>/%s/projects", username)
	}
	return filepath.Dir(userDir), nil
}

// runHeadless runs one command-line operation on a project file and returns
// the process exit status. It returns instead of exiting so the deferred
// Close always runs: os.Exit and log.Fatal skip deferred calls.
func runHeadless(cfg *cli.Config) int {
	d, err := openHeadlessDB(cfg)
	if err != nil {
		log.Printf("init db: %v", err)
		return 1
	}
	defer func() { _ = d.Close() }()

	switch {
	case cfg.CheckOnly:
		ok, err := d.CheckIntegrity()
		if err != nil {
			log.Printf("integrity check: %v", err)
			return 1
		}
		if ok {
			fmt.Println("ok")
			return 0
		}
		fmt.Println("CORRUPT")
		return 1
	case cfg.Repair:
		damaged, err := runHeadlessRepair(d, cfg.ProjectPath, os.Stdout)
		if err != nil {
			log.Printf("repair: %v", err)
			return 1
		}
		if damaged {
			// The project file is still damaged; exit 1 like --check so
			// scripts notice.
			return 1
		}
	case cfg.Vacuum:
		if err := d.Vacuum(); err != nil {
			log.Printf("vacuum: %v", err)
			return 1
		}
	case cfg.ExportAuditPath != "":
		if err := d.ExportAuditCSV(cfg.ExportAuditPath); err != nil {
			log.Printf("export audit: %v", err)
			return 1
		}
		fmt.Printf("audit log written to %s\n", cfg.ExportAuditPath)
	case cfg.ShowStats:
		if err := printHeadlessStats(d); err != nil {
			log.Printf("stats: %v", err)
			return 1
		}
	case cfg.SchemaDump:
		schema, err := d.DumpSchema()
		if err != nil {
			log.Printf("schema dump: %v", err)
			return 1
		}
		fmt.Print(schema)
	case cfg.ExportPath != "":
		if err := runHeadlessExport(cfg, d); err != nil {
			log.Printf("export: %v", err)
			return 1
		}
	}
	return 0
}

// printHeadlessStats writes a compact project summary to stdout for the
// `--stats` flag.
func printHeadlessStats(d *db.Database) error {
	proj, err := d.GetProject()
	if err != nil {
		return err
	}
	charts, err := d.CountProjectRecords("charts", proj.ID)
	if err != nil {
		return err
	}
	docs, err := d.CountProjectRecords("documents", proj.ID)
	if err != nil {
		return err
	}
	stakeholders, err := d.CountProjectRecords("stakeholders", proj.ID)
	if err != nil {
		return err
	}
	auditEvents, err := d.CountProjectRecords("audit_events", proj.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Project:      %s\n", proj.Name)
	fmt.Printf("ID:           %s\n", proj.ID)
	fmt.Printf("Status:       %s\n", proj.Status)
	fmt.Printf("Phase:        %s\n", proj.Phase)
	fmt.Printf("Methodology:  %s\n", proj.Methodology)
	fmt.Printf("Charts:       %d\n", charts)
	fmt.Printf("Documents:    %d\n", docs)
	fmt.Printf("Stakeholders: %d\n", stakeholders)
	fmt.Printf("Audit events: %d\n", auditEvents)
	return nil
}

// runHeadlessExport renders the project's schedule report in the requested
// format and writes it to cfg.ExportPath for the `--export` flag. With
// --encrypt, the bytes are AES-GCM encrypted with the password named by
// --password-env (the same wrapping the GUI export uses).
func runHeadlessExport(cfg *cli.Config, d *db.Database) error {
	format, err := parseHeadlessFormat(cfg.ExportFormat)
	if err != nil {
		return err
	}
	proj, err := d.GetProject()
	if err != nil {
		return err
	}
	tasks, err := loadCurrentProjectSchedule(d, proj.ID)
	if err != nil {
		return fmt.Errorf("load schedule for export: %w", err)
	}
	if err := scheduleProjectTasks(proj, tasks); err != nil {
		return fmt.Errorf("schedule for export: %w", err)
	}
	payload := export.ReportPayload{Tasks: tasks, Currency: proj.CurrencyCode}
	if start, ok := parseProjectDate(proj.StartDate); ok && len(tasks) > 0 {
		cal := calendar.For(proj.CountryCode)
		if day, dok := kernel.DayOffset(start, time.Now().UTC(), cal.IsWorkday); dok {
			m, err := kernel.ComputeEVM(tasks, day)
			if err != nil {
				return fmt.Errorf("compute schedule EVM: %w", err)
			}
			payload.EVM = &m
		}
	}
	opts := export.ExportOptions{Format: format, Title: proj.Name}
	if cfg.EncryptExport {
		pw, err := headlessExportPassword(cfg)
		if err != nil {
			return err
		}
		opts.Encrypted = true
		opts.Password = pw
	}
	raw, err := export.GenerateArchivalReport(payload, opts)
	if err != nil {
		return err
	}
	if err := d.CheckExportDestination(cfg.ExportPath); err != nil {
		return err
	}
	if err := os.WriteFile(cfg.ExportPath, raw, 0o600); err != nil { // #nosec G306 -- exports are owner-private.
		return err
	}
	fmt.Printf("export written to %s\n", cfg.ExportPath)
	return nil
}

// parseHeadlessFormat maps the --format string to an export.ExportFormat.
// An empty value defaults to PDF, matching the flag default.
func parseHeadlessFormat(s string) (export.ExportFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "pdf":
		return export.FormatPDF, nil
	case "docx":
		return export.FormatDOCX, nil
	case "odt":
		return export.FormatODT, nil
	case "xlsx":
		return export.FormatXLSX, nil
	case "csv":
		return export.FormatCSV, nil
	case "html":
		return export.FormatHTML, nil
	case "mspdi", "xml":
		return export.FormatMSPDI, nil
	default:
		return "", fmt.Errorf("unsupported export format %q (want pdf, docx, odt, xlsx, csv, html, or mspdi)", s)
	}
}

// errPasswordEnvUnset deliberately leaves out the variable's name. The name
// is not secret, but it comes from the same configuration field the
// password is read through, and keeping that field out of every message
// means no later edit can log the password by mistake.
var errPasswordEnvUnset = errors.New("the environment variable named by --password-env is not set or is empty")

// headlessExportPassword resolves the export-encryption password from the
// environment variable named by --password-env.
func headlessExportPassword(cfg *cli.Config) (string, error) {
	if strings.TrimSpace(cfg.PasswordEnv) == "" {
		return "", errors.New("--encrypt requires --password-env naming the environment variable that holds the export password")
	}
	pw, ok := os.LookupEnv(cfg.PasswordEnv)
	if !ok || pw == "" {
		return "", errPasswordEnvUnset
	}
	return pw, nil
}
