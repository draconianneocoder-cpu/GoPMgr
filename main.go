// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command pmforge is the entry point for the PMForge desktop
// application. V2 expands V1 in three ways:
//
//   - Local multi-user accounts (Argon2id) backed by a system DB at
//     ~/Documents/PMForge/system.db
//   - Per-user folders for project files and exports
//   - Unified charts/documents data model (19 + 25 kinds)
//
// CLI dispatch (--version, --check, --repair, ...) is unchanged from
// V1 and runs without launching the Wails GUI.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pmforge/internal/admin"
	"pmforge/internal/applog"
	"pmforge/internal/calendar"
	"pmforge/internal/cli"
	"pmforge/internal/db"
	"pmforge/internal/export"
	"pmforge/internal/kernel"
	"pmforge/internal/sigma/service"
	"pmforge/internal/templates"
	"pmforge/internal/update"
	"pmforge/internal/users"
)

//go:embed all:frontend/dist
var assets embed.FS

// App is the Wails-exposed object. Every exported method becomes
// callable from the Svelte frontend via window.go.main.App.<Method>.
//
// Concurrency model (see DEVELOPER_HANDBOOK.md §6 for the full invariants):
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
	ctx context.Context

	mu        sync.RWMutex
	store     *users.Store   // immutable after NewApp — safe to read without lock
	user      *users.Account // nil unless logged in
	dek       []byte         // ADR-001: session DEK, unlocked at login; nil when logged out
	db        *db.Database   // nil unless a project is open
	dbPath    string         // absolute path of the open .pmforge
	adminSvc  *admin.Service
	templates *templates.Engine       // immutable after NewApp; safe lock-free read
	sigmaSvc  *service.ProjectService // initialized when a project is open

	// Diagnostic logging — set in main() after applog.Init; never reassigned.
	logPath string // dated log file path, e.g. .../logs/pmforge-2026-06-20.log
	logDir  string // parent of logPath, e.g. .../logs
}

// NewApp constructs an App at process start. It opens the system DB
// up front (so the Login screen can list known users) and leaves the
// per-user / per-project handles for later.
func NewApp() (*App, error) {
	root, err := users.DefaultRootDir()
	if err != nil {
		return nil, err
	}
	// One-time relocation: older macOS installs kept their data under the
	// iCloud-synced, TCC-protected ~/Documents/PMForge. Copy it into the new
	// Application Support root before opening the store so an existing user's
	// accounts and projects survive the move. A failure here is non-fatal —
	// we log it and fall through to a clean new-location install rather than
	// blocking startup.
	if migrated, mErr := users.MigrateLegacyRoot(root); mErr != nil {
		log.Printf("users: legacy data migration failed: %v (continuing with a fresh %s)", mErr, root)
	} else if migrated {
		log.Printf("users: migrated legacy ~/Documents/PMForge data into %s", root)
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
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	// ADR-001: zero the session DEK on exit too, not only on Logout, so
	// quitting with a session open does not leave key bytes in the heap
	// (swap / core-dump hygiene).
	for i := range a.dek {
		a.dek[i] = 0
	}
	a.dek = nil
	// Close the store but keep the pointer: `store` is documented as
	// set-once and readable without the lock (DEVELOPER_HANDBOOK.md §6), so nilling it
	// here would be the one write that violates that invariant. A closed
	// store safely returns errors to any late caller (unreachable in
	// practice — Wails stops dispatching before shutdown runs).
	if a.store != nil {
		_ = a.store.Close()
	}
}

// =========================================================
// Diagnostics / sanity
// =========================================================

func (a *App) Greet() string {
	return "PMForge " + cli.Version + " ready."
}

// =========================================================
// main: CLI dispatch + Wails launch
// =========================================================

// buildAppMenu constructs the native application menu. The File submenu drives
// the project lifecycle and Help shows an About dialog. Menu items emit Wails
// runtime events that the frontend (App.svelte) listens for and turns into
// navigation, so the menu triggers the same flows as the in-app buttons. On
// macOS the standard App and Edit menus are included so Quit/Hide and
// copy/paste/select-all keep working when a custom menu is set.
func buildAppMenu(app *App) *menu.Menu {
	// emit returns a menu callback that fires a frontend event. app.ctx is nil
	// until OnStartup runs, but menu clicks only happen after the window is up,
	// so the guard is belt-and-suspenders.
	emit := func(event string) func(*menu.CallbackData) {
		return func(_ *menu.CallbackData) {
			if app.ctx != nil {
				wailsruntime.EventsEmit(app.ctx, event)
			}
		}
	}

	m := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		m.Append(menu.AppMenu()) // standard macOS app menu: About/Hide/Quit
	}

	fileMenu := m.AddSubmenu("File")
	fileMenu.AddText("Dashboard", keys.CmdOrCtrl("d"), emit("menu:dashboard"))
	fileMenu.AddText("New Project", keys.CmdOrCtrl("n"), emit("menu:new-project"))
	fileMenu.AddText("Open Project…", keys.CmdOrCtrl("o"), emit("menu:open-project"))
	fileMenu.AddSeparator()
	fileMenu.AddText("Application Settings…", keys.CmdOrCtrl(","), emit("menu:app-settings"))
	fileMenu.AddText("Project Settings…", nil, emit("menu:settings"))
	fileMenu.AddSeparator()
	fileMenu.AddText("Close Project", keys.CmdOrCtrl("w"), emit("menu:close-project"))
	if runtime.GOOS != "darwin" {
		// macOS gets Quit from the App menu; other platforms need it here.
		fileMenu.AddSeparator()
		fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			if app.ctx != nil {
				wailsruntime.Quit(app.ctx)
			}
		})
	}

	if runtime.GOOS == "darwin" {
		m.Append(menu.EditMenu())   // undo/redo/cut/copy/paste/select-all
		m.Append(menu.WindowMenu()) // Minimize (Cmd+M), Zoom, Bring All to Front
	} else {
		// Windows and Linux don't get the macOS role-based Window menu, so
		// wire Maximize explicitly so keyboard users aren't forced to reach
		// for the title-bar button.
		windowMenu := m.AddSubmenu("Window")
		windowMenu.AddText("Maximize / Restore", keys.Key("F11"), func(_ *menu.CallbackData) {
			if app.ctx != nil {
				wailsruntime.WindowToggleMaximise(app.ctx)
			}
		})
		windowMenu.AddText("Minimize", nil, func(_ *menu.CallbackData) {
			if app.ctx != nil {
				wailsruntime.WindowMinimise(app.ctx)
			}
		})
	}

	helpMenu := m.AddSubmenu("Help")
	helpMenu.AddText("User Guide", nil, emit("menu:help"))
	helpMenu.AddText("About PMForge", nil, func(_ *menu.CallbackData) {
		if app.ctx == nil {
			return
		}
		_, _ = wailsruntime.MessageDialog(app.ctx, wailsruntime.MessageDialogOptions{
			Type:  wailsruntime.InfoDialog,
			Title: "About PMForge",
			Message: fmt.Sprintf(
				"PMForge %s\n\nLocal-first project controls.\nCopyright (C) 2026 James L. Burns and The PMForge Contributors.\nLicensed under GPL-3.0-or-later.",
				cli.Version,
			),
		})
	})

	return m
}

func main() {
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

	// CLI mode that operates on a single .pmforge file directly
	// (--check / --repair / --vacuum / --export-audit). Plaintext files
	// open directly; encrypted files require --username plus
	// --password-env so the user's DEK can be unlocked from system.db.
	if cfg.ProjectPath != "" && headlessProjectMode(cfg) {
		runHeadless(cfg)
		return
	}

	// GUI mode.
	//
	// Initialise diagnostic logging before anything else in this path. A
	// Wails binary launched from Finder/Explorer/.desktop has its stderr
	// routed to a null sink, so a bare log.Fatalf here would make the app
	// die with no window and no trace. applog tees the log to stderr AND a
	// dated file under the PMForge data tree, and applog.Fatal additionally
	// shows a native error dialog so a startup failure is never silent.
	root, rootErr := users.DefaultRootDir()
	if rootErr != nil {
		// Non-fatal: applog falls back to a home/temp logs directory.
		root = ""
	}
	logPath, closeLog := applog.Init(root)
	defer closeLog()
	log.Printf("PMForge %s starting (pid=%d, %s/%s, %s)",
		cli.Version, os.Getpid(), runtime.GOOS, runtime.GOARCH, runtime.Version())

	app, err := NewApp()
	if err != nil {
		applog.Fatal("PMForge could not start",
			"PMForge failed to initialise its local data store.", logPath, err)
	}
	app.logPath = logPath
	app.logDir = applog.LogDir(root)

	err = wails.Run(&options.App{
		Title:     "PMForge",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu: buildAppMenu(app),
		OnStartup: func(ctx context.Context) {
			app.ctx = ctx
		},
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})
	if err != nil {
		applog.Fatal("PMForge could not start",
			"PMForge failed to start its application window.", logPath, err)
	}
	log.Print("PMForge exited cleanly")
}

func headlessProjectMode(cfg *cli.Config) bool {
	return cfg.CheckOnly || cfg.Repair || cfg.Vacuum || cfg.ExportAuditPath != "" ||
		cfg.ShowStats || cfg.SchemaDump || cfg.ExportPath != ""
}

func openHeadlessDB(cfg *cli.Config) (*db.Database, error) {
	encrypted, err := db.IsEncryptedFile(cfg.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("inspect project encryption: %w", err)
	}
	if !encrypted {
		return db.InitDB(cfg.ProjectPath)
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return nil, errors.New("--username is required for encrypted project maintenance")
	}
	if strings.TrimSpace(cfg.PasswordEnv) == "" {
		return nil, errors.New("--password-env is required for encrypted project maintenance")
	}
	password, ok := os.LookupEnv(cfg.PasswordEnv)
	if !ok || password == "" {
		return nil, fmt.Errorf("password environment variable %q is not set", cfg.PasswordEnv)
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
	if _, err := store.Authenticate(cfg.Username, password); err != nil {
		return nil, fmt.Errorf("authenticate headless user: %w", err)
	}
	dek, err := store.UnlockDEK(cfg.Username, password)
	if err != nil {
		return nil, fmt.Errorf("unlock database key: %w", err)
	}
	return db.InitEncryptedDB(cfg.ProjectPath, dek)
}

func inferHeadlessRootDir(projectPath, username string) (string, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", err
	}
	projectsDir := filepath.Dir(absPath)
	// Current layout nests each project in its own subfolder:
	// <root>/<username>/projects/<id>/project.pmforge. Step up out of the
	// per-project subfolder so projectsDir points at ".../projects".
	if filepath.Base(projectsDir) != "projects" {
		projectsDir = filepath.Dir(projectsDir)
	}
	userDir := filepath.Dir(projectsDir)
	if filepath.Base(projectsDir) != "projects" || filepath.Base(userDir) != username {
		return "", fmt.Errorf("encrypted headless project must be under <pmforge-root>/%s/projects", username)
	}
	return filepath.Dir(userDir), nil
}

func runHeadless(cfg *cli.Config) {
	d, err := openHeadlessDB(cfg)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer func() { _ = d.Close() }()

	switch {
	case cfg.CheckOnly:
		ok, err := d.CheckIntegrity()
		if err != nil {
			log.Fatalf("integrity check: %v", err)
		}
		if ok {
			fmt.Println("ok")
			return
		}
		fmt.Println("CORRUPT")
		os.Exit(1)
	case cfg.Repair:
		result, err := d.InformativeSelfHeal(cfg.ProjectPath)
		for _, line := range result.Log {
			fmt.Println(line)
		}
		if err != nil {
			log.Fatalf("repair: %v", err)
		}
	case cfg.Vacuum:
		if err := d.Vacuum(); err != nil {
			log.Fatalf("vacuum: %v", err)
		}
	case cfg.ExportAuditPath != "":
		if err := d.ExportAuditCSV(cfg.ExportAuditPath); err != nil {
			log.Fatalf("export audit: %v", err)
		}
		fmt.Printf("audit log written to %s\n", cfg.ExportAuditPath)
	case cfg.ShowStats:
		if err := printHeadlessStats(d); err != nil {
			log.Fatalf("stats: %v", err)
		}
	case cfg.SchemaDump:
		schema, err := d.DumpSchema()
		if err != nil {
			log.Fatalf("schema dump: %v", err)
		}
		fmt.Print(schema)
	case cfg.ExportPath != "":
		if err := runHeadlessExport(cfg, d); err != nil {
			log.Fatalf("export: %v", err)
		}
	}
}

// printHeadlessStats writes a compact project summary to stdout for the
// `--stats` flag.
func printHeadlessStats(d *db.Database) error {
	proj, err := d.GetProject()
	if err != nil {
		return err
	}
	charts, err := d.ListCharts(proj.ID, "")
	if err != nil {
		return err
	}
	docs, err := d.ListDocuments(proj.ID, "")
	if err != nil {
		return err
	}
	stakeholders, err := d.ListStakeholders(proj.ID, "")
	if err != nil {
		return err
	}
	auditEvents, err := d.ListAuditEvents(proj.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Project:      %s\n", proj.Name)
	fmt.Printf("ID:           %s\n", proj.ID)
	fmt.Printf("Status:       %s\n", proj.Status)
	fmt.Printf("Phase:        %s\n", proj.Phase)
	fmt.Printf("Methodology:  %s\n", proj.Methodology)
	fmt.Printf("Charts:       %d\n", len(charts))
	fmt.Printf("Documents:    %d\n", len(docs))
	fmt.Printf("Stakeholders: %d\n", len(stakeholders))
	fmt.Printf("Audit events: %d\n", len(auditEvents))
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
		tasks = make(map[string]*kernel.Task)
	}
	if len(tasks) > 0 {
		scheduleProjectTasks(proj, tasks)
	}
	payload := export.ReportPayload{Tasks: tasks}
	if start, ok := parseProjectDate(proj.StartDate); ok && len(tasks) > 0 {
		cal := calendar.For(proj.CountryCode)
		if day, dok := kernel.DayOffset(start, time.Now().UTC(), cal.IsWorkday); dok {
			m := kernel.ComputeEVM(tasks, day)
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

// headlessExportPassword resolves the export-encryption password from the
// environment variable named by --password-env.
func headlessExportPassword(cfg *cli.Config) (string, error) {
	if strings.TrimSpace(cfg.PasswordEnv) == "" {
		return "", errors.New("--encrypt requires --password-env naming the environment variable that holds the export password")
	}
	pw, ok := os.LookupEnv(cfg.PasswordEnv)
	if !ok || pw == "" {
		return "", fmt.Errorf("password environment variable %q is not set", cfg.PasswordEnv)
	}
	return pw, nil
}
