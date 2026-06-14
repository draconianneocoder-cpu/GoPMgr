// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pmforge/internal/admin"
	"pmforge/internal/agile"
	"pmforge/internal/auth"
	"pmforge/internal/budget"
	"pmforge/internal/calendar"
	"pmforge/internal/charts"
	"pmforge/internal/charts/dag"
	"pmforge/internal/cli"
	"pmforge/internal/crypto"
	"pmforge/internal/db"
	"pmforge/internal/documents"
	"pmforge/internal/export"
	"pmforge/internal/fonts"
	"pmforge/internal/kernel"
	"pmforge/internal/pdfmeta"
	sigmacharts "pmforge/internal/sigma/charts"
	"pmforge/internal/sigma/domain"
	"pmforge/internal/sigma/service"
	"pmforge/internal/sigma/stats"
	"pmforge/internal/sigma/tollgate"
	"pmforge/internal/templates"
	"pmforge/internal/timeline"
	"pmforge/internal/update"
	"pmforge/internal/users"
)

//go:embed all:frontend/dist
var assets embed.FS

// App is the Wails-exposed object. Every exported method becomes
// callable from the Svelte frontend via window.go.main.App.<Method>.
//
// Concurrency model (see AGENT.md §6 for the full invariants):
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
}

// NewApp constructs an App at process start. It opens the system DB
// up front (so the Login screen can list known users) and leaves the
// per-user / per-project handles for later.
func NewApp() (*App, error) {
	root, err := users.DefaultRootDir()
	if err != nil {
		return nil, err
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
	if a.store != nil {
		_ = a.store.Close()
		a.store = nil
	}
}

// =========================================================
// Diagnostics / sanity
// =========================================================

func (a *App) Greet() string {
	return "PMForge " + cli.Version + " ready."
}

// =========================================================
// Accounts & session
// =========================================================

// ListUsers returns every account on the machine. Used by the login
// screen if you want a user-picker variant later.
func (a *App) ListUsers() ([]users.Account, error) {
	return a.store.List()
}

// CreateAccount provisions a new user and signs them in. Returns the
// account (without password material).
func (a *App) CreateAccount(username, displayName, password string) (users.Account, error) {
	acc, err := a.store.CreateAccount(username, displayName, password)
	if err != nil {
		return users.Account{}, err
	}
	// ADR-001: unlock (here: lazily create) the per-user DEK while we
	// hold the verified password — the only moment that is possible.
	dek, err := a.store.UnlockDEK(username, password)
	if err != nil {
		return users.Account{}, err
	}
	a.mu.Lock()
	a.user = &acc
	a.dek = dek
	a.mu.Unlock()
	return acc, nil
}

// Login authenticates and stores the user as the active session.
// Returns a generic error on bad credentials — the message is shaped
// by the frontend so usernames cannot be enumerated by error
// inspection.
func (a *App) Login(username, password string) (users.Account, error) {
	acc, err := a.store.Authenticate(username, password)
	if err != nil {
		// Collapse both "no such user" and "password mismatch" into
		// one error so the timing/message is identical.
		if errors.Is(err, users.ErrNoSuchUser) || errors.Is(err, auth.ErrMismatch) {
			return users.Account{}, errors.New("invalid credentials")
		}
		return users.Account{}, err
	}
	// ADR-001: unlock the per-user DEK with the verified password.
	// Lazy generation covers accounts that predate the key hierarchy.
	dek, err := a.store.UnlockDEK(username, password)
	if err != nil {
		return users.Account{}, err
	}
	a.mu.Lock()
	a.user = &acc
	a.dek = dek
	a.mu.Unlock()
	return acc, nil
}

// IssueRecoveryCodes generates 8 fresh recovery codes for the
// currently-signed-in user and returns the plaintext codes ONCE.
// The GUI MUST show them to the user immediately and warn that they
// will not be visible again — only their Argon2id hashes are
// persisted.
//
// Calling this rotates the user's existing unused codes.
func (a *App) IssueRecoveryCodes() ([]string, error) {
	u := a.requireUser()
	if u == nil {
		return nil, errors.New("not signed in")
	}
	// ADR-001: wrap the session DEK into every code so a recovery
	// reset can re-wrap the same DEK (encrypted projects survive).
	a.mu.RLock()
	dek := a.dek
	a.mu.RUnlock()
	return a.store.IssueRecoveryCodes(u.Username, dek)
}

// RemainingRecoveryCodes returns the count of unused recovery codes
// for the active user. The GUI nags at 0 or 1.
func (a *App) RemainingRecoveryCodes() (int, error) {
	u := a.requireUser()
	if u == nil {
		return 0, errors.New("not signed in")
	}
	return a.store.RemainingRecoveryCodes(u.Username)
}

// ResetWithRecoveryCode is the "forgot password" flow. It does NOT
// require an active session — the user lands on the login screen,
// clicks "use a recovery code", enters username + code + new
// password, and we verify + rotate atomically.
func (a *App) ResetWithRecoveryCode(username, code, newPassword string) error {
	return a.store.ResetWithRecoveryCode(username, code, newPassword)
}

// Logout clears the active session and closes any open project.
func (a *App) Logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
		a.dbPath = ""
		a.adminSvc = nil
	}
	a.user = nil
	// ADR-001: zero the session DEK before dropping it.
	for i := range a.dek {
		a.dek[i] = 0
	}
	a.dek = nil
	return nil
}

// CurrentUser returns the active session or nil. Used by the GUI on
// initial mount to skip the login screen if we already have a user.
func (a *App) CurrentUser() *users.Account {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.user
}

// =========================================================
// Projects
// =========================================================

// ProjectFile is the lightweight "card" the project picker renders.
type ProjectFile struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Modified string `json:"modified"`
}

var ErrProjectRequiresEncryptionMigration = errors.New("project requires encryption migration")

var ErrRecoveryCodesRequireReissue = errors.New("Reissue recovery codes before enabling database encryption. Old recovery codes cannot preserve encrypted projects during password reset.")

// ListProjects returns every .pmforge file under the current user's
// projects/ folder.
func (a *App) ListProjects() ([]ProjectFile, error) {
	user := a.requireUser()
	if user == nil {
		return nil, errors.New("not signed in")
	}
	dir := filepath.Join(user.DataDir, "projects")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProjectFile{}, nil
		}
		return nil, err
	}
	var out []ProjectFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".pmforge" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, ProjectFile{
			Path:     filepath.Join(dir, e.Name()),
			Name:     trimExt(e.Name()),
			Modified: info.ModTime().Format(time.RFC3339),
		})
	}
	return out, nil
}

// CreateProject creates a new .pmforge file under the user's
// projects/ folder, initialises the project row, and returns its
// ProjectFile representation.
func (a *App) CreateProject(name, description string) (ProjectFile, error) {
	user := a.requireUser()
	if user == nil {
		return ProjectFile{}, errors.New("not signed in")
	}
	safe := sanitizeFilename(name)
	if safe == "" {
		return ProjectFile{}, errors.New("invalid project name")
	}
	dir := filepath.Join(user.DataDir, "projects")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ProjectFile{}, err
	}
	a.mu.RLock()
	dek, err := a.requireDEKLocked()
	a.mu.RUnlock()
	if err != nil {
		return ProjectFile{}, err
	}

	// Choose a non-conflicting filename.
	path := filepath.Join(dir, safe+".pmforge")
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d.pmforge", safe, i))
	}

	d, err := db.InitEncryptedDB(path, dek)
	if err != nil {
		return ProjectFile{}, err
	}
	if _, err := d.UpsertProject(db.Project{
		Name:        name,
		Description: description,
		Status:      "planning",
		Phase:       "initiation",
		Owner:       user.Username,
	}); err != nil {
		_ = d.Close()
		return ProjectFile{}, err
	}
	_ = d.Close()

	return ProjectFile{
		Path:     path,
		Name:     name,
		Modified: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// OpenProject loads a .pmforge file as the current project.
func (a *App) OpenProject(path string) (db.Project, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	dek, err := a.requireDEKLocked()
	if err != nil {
		return db.Project{}, err
	}
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	d, err := db.InitEncryptedDB(path, dek)
	if err != nil {
		if encrypted, encErr := db.IsEncryptedFile(path); encErr == nil && !encrypted {
			return db.Project{}, ErrProjectRequiresEncryptionMigration
		}
		return db.Project{}, err
	}
	a.db = d
	a.dbPath = path
	a.adminSvc = admin.NewService(d)
	a.sigmaSvc = service.NewProjectService(d)
	proj, projErr := d.GetProject()
	// Apply the project's saved document font (no-op if unset). Done
	// while we still hold the lock since it only reads d + the user's
	// font dir; configureFonts must not re-acquire a.mu.
	a.configureFontsLocked(d)
	return proj, projErr
}

// IsProjectEncrypted reports whether a .pmforge file is already
// SQLCipher-encrypted. Used by the Settings migration flow before
// presenting the opt-in action.
func (a *App) IsProjectEncrypted(path string) (bool, error) {
	return db.IsEncryptedFile(path)
}

// EncryptProjectAtRest migrates a legacy plaintext .pmforge file to
// SQLCipher with the active user's session DEK. Active recovery codes
// must already carry DEK wraps; otherwise a future recovery reset
// would orphan encrypted projects.
func (a *App) EncryptProjectAtRest(path string) (string, error) {
	user := a.requireUser()
	if user == nil {
		return "", errors.New("not signed in")
	}
	needsReissue, err := a.store.HasLegacyRecoveryCodeWraps(user.Username)
	if err != nil {
		return "", err
	}
	if needsReissue {
		return "", ErrRecoveryCodesRequireReissue
	}

	a.mu.Lock()
	dek, err := a.requireDEKLocked()
	if err != nil {
		a.mu.Unlock()
		return "", err
	}
	if a.db != nil && samePath(a.dbPath, path) {
		_ = a.db.Close()
		a.db = nil
		a.dbPath = ""
		a.adminSvc = nil
		a.sigmaSvc = nil
	}
	a.mu.Unlock()

	return db.MigratePlaintextToEncrypted(path, dek)
}

// CloseProject closes the currently-open .pmforge.
func (a *App) CloseProject() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
		a.dbPath = ""
		a.adminSvc = nil
		a.sigmaSvc = nil
	}
	// Revert document rendering to the built-in font.
	documents.UseFont(nil, "")
	return nil
}

// configureFontsLocked wires the document renderers to use the saved
// default font for database d. Must be called with a.mu held (it reads
// a.user without re-locking). A nil/empty default reverts to the core
// font.
func (a *App) configureFontsLocked(d *db.Database) {
	if d == nil {
		documents.UseFont(nil, "")
		return
	}
	s, err := d.GetSettings()
	if err != nil || s.DefaultFont == "" {
		documents.UseFont(nil, "")
		return
	}
	userDir := ""
	if a.user != nil {
		userDir = filepath.Join(a.user.DataDir, "fonts")
	}
	documents.UseFont(fonts.NewManager(userDir), s.DefaultFont)
}

// GetProjectMeta returns the metadata of the currently-open project.
func (a *App) GetProjectMeta() (db.Project, error) {
	d := a.requireDB()
	if d == nil {
		return db.Project{}, errors.New("no project open")
	}
	return d.GetProject()
}

// UpdateProjectMeta upserts the project metadata.
func (a *App) UpdateProjectMeta(p db.Project) (db.Project, error) {
	d := a.requireDB()
	if d == nil {
		return db.Project{}, errors.New("no project open")
	}
	return d.UpsertProject(p)
}

// =========================================================
// Charts
// =========================================================

func (a *App) ListChartKinds() []charts.Definition { return charts.All() }

func (a *App) ListCharts(kind string) ([]db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListCharts(p.ID, kind)
}

func (a *App) GetChart(id string) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	return d.GetChart(id)
}

func (a *App) SaveChart(c db.Chart) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	if c.ProjectID == "" {
		p, err := d.GetProject()
		if err != nil {
			return db.Chart{}, err
		}
		c.ProjectID = p.ID
	}
	return d.SaveChart(c)
}

func (a *App) DeleteChart(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteChart(id)
}

// LayoutChart asks the chart engine to produce a frontend-ready
// layout. The Svelte renderer reads `engine` and dispatches.
//
// CPM charts are calendar-anchored when the open project has a start
// date: each node additionally carries StartDate/FinishDate computed
// against the project country's work calendar. Projects without a
// start date keep the plain day-offset layout.
func (a *App) LayoutChart(id string) (charts.LayoutResult, error) {
	d := a.requireDB()
	if d == nil {
		return charts.LayoutResult{}, errors.New("no project open")
	}
	c, err := d.GetChart(id)
	if err != nil {
		return charts.LayoutResult{}, err
	}

	var (
		projectStart time.Time
		isWorkday    kernel.WorkdayFunc
		capacities   map[string]float64
	)
	if proj, perr := d.GetProject(); perr == nil {
		if start, ok := parseProjectDate(proj.StartDate); ok {
			projectStart = start
			isWorkday = calendar.For(proj.CountryCode).IsWorkday
			capacities = stakeholderCapacities(d, proj.ID)
		}
	}

	res, err := charts.LayoutWithSchedule(charts.Kind(c.Kind), c.Data, projectStart, isWorkday, capacities)
	if err != nil && !errors.Is(err, charts.ErrEngineNotImplemented) {
		return charts.LayoutResult{}, err
	}
	res.Title = c.Title
	return res, nil
}

// =========================================================
// Schedule baselines (roadmap item 17)
// =========================================================

// SetScheduleBaseline snapshots the current scheduled state of a CPM
// chart under an optional name. The stored payload is the fully
// scheduled kernel task map (constraints armed, CPM run, dates
// anchored when the project has a start date).
func (a *App) SetScheduleBaseline(chartID, name string) (db.Baseline, error) {
	d := a.requireDB()
	if d == nil {
		return db.Baseline{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return db.Baseline{}, err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return db.Baseline{}, err
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return db.Baseline{}, err
	}
	if len(tasks) == 0 {
		return db.Baseline{}, errors.New("chart has no tasks to baseline")
	}
	scheduleProjectTasks(proj, tasks)
	blob, err := json.Marshal(tasks)
	if err != nil {
		return db.Baseline{}, err
	}
	return d.SaveBaseline(db.Baseline{
		ProjectID: proj.ID,
		ChartID:   chartID,
		Name:      name,
		Data:      string(blob),
	})
}

// ListScheduleBaselines returns a chart's baselines, newest first.
func (a *App) ListScheduleBaselines(chartID string) ([]db.Baseline, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	return d.ListBaselines(chartID)
}

// DeleteScheduleBaseline removes a baseline snapshot.
func (a *App) DeleteScheduleBaseline(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteBaseline(id)
}

// CompareScheduleBaseline diffs the chart's CURRENT schedule against
// a stored baseline (the newest one when baselineID is empty).
// Returns per-task variances keyed by task ID; an empty map when the
// chart has no baseline yet.
func (a *App) CompareScheduleBaseline(chartID, baselineID string) (map[string]kernel.ScheduleVariance, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}

	var (
		base db.Baseline
		err  error
	)
	if baselineID != "" {
		base, err = d.GetBaseline(baselineID)
		if err != nil {
			return nil, err
		}
	} else {
		list, lerr := d.ListBaselines(chartID)
		if lerr != nil {
			return nil, lerr
		}
		if len(list) == 0 {
			return map[string]kernel.ScheduleVariance{}, nil
		}
		base = list[0]
	}

	baseline := make(map[string]*kernel.Task)
	if err := json.Unmarshal([]byte(base.Data), &baseline); err != nil {
		return nil, fmt.Errorf("baseline %s is corrupt: %w", base.ID, err)
	}

	proj, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return nil, err
	}
	current, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return nil, err
	}
	scheduleProjectTasks(proj, current)

	return kernel.CompareSchedules(current, baseline), nil
}

// ComputeScheduleEVM derives earned-value metrics for a CPM chart at
// a status date ("" = today, else YYYY-MM-DD). EVM needs the project
// start date to place the status date on the schedule's working-day
// axis, so projects without one get a clear error instead of numbers
// that look right but mean nothing.
func (a *App) ComputeScheduleEVM(chartID, asOfDate string) (kernel.EVMetrics, error) {
	d := a.requireDB()
	if d == nil {
		return kernel.EVMetrics{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return kernel.EVMetrics{}, err
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		return kernel.EVMetrics{}, errors.New("earned value needs a project start date (Project Settings)")
	}

	c, err := d.GetChart(chartID)
	if err != nil {
		return kernel.EVMetrics{}, err
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return kernel.EVMetrics{}, err
	}
	if len(tasks) == 0 {
		return kernel.EVMetrics{}, errors.New("chart has no tasks")
	}
	scheduleProjectTasks(proj, tasks)

	asOf := time.Now().UTC()
	if asOfDate != "" {
		parsed, perr := time.Parse(kernel.DateLayout, asOfDate)
		if perr != nil {
			return kernel.EVMetrics{}, fmt.Errorf("status date %q: want YYYY-MM-DD", asOfDate)
		}
		asOf = parsed
	}
	cal := calendar.For(proj.CountryCode)
	asOfDay, ok := kernel.DayOffset(start, asOf, cal.IsWorkday)
	if !ok {
		return kernel.EVMetrics{}, errors.New("status date is unreachably far from the project start")
	}

	return kernel.ComputeEVM(tasks, asOfDay), nil
}

// LevelChartResources runs the kernel's serial resource-levelling
// pass on a CPM chart and PERSISTS the result: every task that
// levelling delayed beyond its precedence-earliest start gets a SNET
// constraint at its levelled start date. Nodes with a user-set
// constraint other than SNET are never touched (links and user intent
// win); previously levelled SNET pins are recomputed. Requires a
// project start date to express levelled offsets as dates.
//
// Returns the number of tasks pinned.
func (a *App) LevelChartResources(chartID string) (int, error) {
	d := a.requireDB()
	if d == nil {
		return 0, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return 0, err
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		return 0, errors.New("resource levelling needs a project start date (Project Settings)")
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return 0, err
	}

	var doc dagDoc
	if err := json.Unmarshal([]byte(c.Data), &doc); err != nil {
		return 0, err
	}

	cal := calendar.For(proj.CountryCode)

	// Baseline pass: precedence-only ES per task.
	plain, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return 0, err
	}
	if len(plain) == 0 {
		return 0, errors.New("chart has no tasks")
	}
	kernel.ApplyConstraintDates(plain, start, cal.IsWorkday)
	if !kernel.CalculateCPM(plain) {
		return 0, errors.New("chart contains a dependency cycle")
	}

	// Levelling pass on a fresh copy.
	levelled, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return 0, err
	}
	kernel.ApplyConstraintDates(levelled, start, cal.IsWorkday)
	if !kernel.LevelResources(levelled, stakeholderCapacities(d, proj.ID)) {
		return 0, errors.New("chart contains a dependency cycle")
	}
	kernel.AnchorSchedule(levelled, start, cal.IsWorkday)

	pinned := 0
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		lt, lok := levelled[n.ID]
		pt, pok := plain[n.ID]
		if !lok || !pok {
			continue
		}
		existing := strings.ToUpper(strings.TrimSpace(n.Constraint))
		if existing != "" && existing != string(kernel.StartNoEarlierThan) {
			continue // never override a user-set non-SNET constraint
		}
		if lt.ES > pt.ES+1e-9 {
			n.Constraint = string(kernel.StartNoEarlierThan)
			n.ConstraintDate = lt.StartDate
			pinned++
		} else if existing == string(kernel.StartNoEarlierThan) && n.ConstraintDate != "" {
			// A previous levelling pin that's no longer needed.
			n.Constraint = ""
			n.ConstraintDate = ""
		}
	}

	blob, err := json.Marshal(doc)
	if err != nil {
		return 0, err
	}
	c.Data = string(blob)
	if _, err := d.SaveChart(c); err != nil {
		return 0, err
	}
	return pinned, nil
}

// GenerateResourceHistogram builds (or refreshes) a Bar chart showing
// each resource's per-day demand for a CPM chart's schedule. The
// histogram is a snapshot: regenerate it after editing the schedule.
// The bar chart's config carries {"source_chart_id": ...} so repeated
// generation updates the same chart instead of accumulating copies.
func (a *App) GenerateResourceHistogram(chartID string) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return db.Chart{}, err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return db.Chart{}, err
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return db.Chart{}, err
	}
	if len(tasks) == 0 {
		return db.Chart{}, errors.New("chart has no tasks")
	}
	scheduleProjectTasks(proj, tasks)

	usage := kernel.ResourceUsage(tasks)
	if len(usage) == 0 {
		return db.Chart{}, errors.New("no resource assignments on this chart")
	}

	// Shared horizon across resources; day labels are real dates when
	// the project is anchored, plain offsets otherwise.
	horizon := 0
	for _, profile := range usage {
		if len(profile) > horizon {
			horizon = len(profile)
		}
	}
	categories := make([]string, horizon)
	if start, ok := parseProjectDate(proj.StartDate); ok {
		cal := calendar.For(proj.CountryCode)
		dayTasks := make(map[string]*kernel.Task, horizon)
		for i := 0; i < horizon; i++ {
			id := fmt.Sprintf("d%d", i)
			dayTasks[id] = &kernel.Task{ID: id, Duration: 1, ES: float64(i), EF: float64(i + 1)}
		}
		kernel.AnchorSchedule(dayTasks, start, cal.IsWorkday)
		for i := 0; i < horizon; i++ {
			categories[i] = dayTasks[fmt.Sprintf("d%d", i)].StartDate
		}
	} else {
		for i := 0; i < horizon; i++ {
			categories[i] = fmt.Sprintf("Day %d", i+1)
		}
	}

	resources := make([]string, 0, len(usage))
	for r := range usage {
		resources = append(resources, r)
	}
	sort.Strings(resources)

	type barSeries struct {
		Name   string    `json:"name"`
		Values []float64 `json:"values"`
	}
	barDoc := struct {
		Title      string      `json:"title"`
		XLabel     string      `json:"x_label"`
		YLabel     string      `json:"y_label"`
		Categories []string    `json:"categories"`
		Series     []barSeries `json:"series"`
	}{
		Title:      "Resource usage — " + c.Title,
		XLabel:     "Day",
		YLabel:     "Units",
		Categories: categories,
	}
	for _, r := range resources {
		values := make([]float64, horizon)
		copy(values, usage[r])
		barDoc.Series = append(barDoc.Series, barSeries{Name: r, Values: values})
	}

	blob, err := json.Marshal(barDoc)
	if err != nil {
		return db.Chart{}, err
	}

	// Reuse the previous histogram for this source chart if present.
	configMarker := fmt.Sprintf(`{"source_chart_id":%q}`, chartID)
	target := db.Chart{
		ProjectID: proj.ID,
		Kind:      string(charts.KindBar),
		Title:     "Resource Histogram — " + c.Title,
		Config:    configMarker,
	}
	if existing, lerr := d.ListCharts(proj.ID, string(charts.KindBar)); lerr == nil {
		for _, e := range existing {
			if e.Config == configMarker {
				target.ID = e.ID
				break
			}
		}
	}
	target.Data = string(blob)
	return d.SaveChart(target)
}

// dagDoc is the minimal layered-document shape LevelChartResources
// round-trips. It must list every persisted node field so the
// re-marshal does not drop data.
type dagDoc struct {
	Nodes []dag.LayeredNode `json:"nodes"`
	Edges []dag.LayeredEdge `json:"edges"`
}

// ImportMSPDIChart opens a file dialog for a Microsoft Project Data
// Interchange XML file and imports it as a new CPM chart in the open
// project. If the project has no start date yet and the file carries
// one, the project start date is adopted so the imported schedule
// anchors immediately.
func (a *App) ImportMSPDIChart() (db.Chart, error) {
	if a.ctx == nil {
		return db.Chart{}, errors.New("no context (Wails not started)")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Import MSPDI schedule",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "MSPDI XML (*.xml)", Pattern: "*.xml"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return db.Chart{}, err
	}
	if path == "" {
		return db.Chart{}, errors.New("import cancelled")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return db.Chart{}, err
	}
	return a.importMSPDIFromBytes(data)
}

// importMSPDIFromBytes is ImportMSPDIChart minus the file dialog so
// the conversion is unit-testable.
func (a *App) importMSPDIFromBytes(data []byte) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return db.Chart{}, err
	}

	imported, err := export.FromMSPDI(data)
	if err != nil {
		return db.Chart{}, err
	}

	var doc dagDoc
	for _, t := range imported.Tasks {
		doc.Nodes = append(doc.Nodes, dag.LayeredNode{
			ID:              t.UID,
			Label:           t.Name,
			Duration:        t.DurationDays,
			Milestone:       t.Milestone,
			PercentComplete: t.PercentComplete,
			Assignments:     t.Assignments,
		})
		for _, l := range t.Links {
			doc.Edges = append(doc.Edges, dag.LayeredEdge{
				From:  l.Pred,
				To:    t.UID,
				Label: dag.FormatLinkLabel(l.Type, l.Lag),
			})
		}
	}

	blob, err := json.Marshal(doc)
	if err != nil {
		return db.Chart{}, err
	}

	title := imported.Title
	if title == "" {
		title = "Imported Schedule"
	}

	// Adopt the file's start date when the project lacks one.
	if _, ok := parseProjectDate(proj.StartDate); !ok && imported.StartDate != "" {
		proj.StartDate = imported.StartDate
		if _, err := d.UpsertProject(proj); err != nil {
			return db.Chart{}, err
		}
	}

	return d.SaveChart(db.Chart{
		ProjectID: proj.ID,
		Kind:      string(charts.KindCPM),
		Title:     title,
		Data:      string(blob),
	})
}

// =========================================================
// Documents
// =========================================================

func (a *App) ListDocumentKinds() []documents.Definition { return documents.All() }

func (a *App) ListDocuments(kind string) ([]db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListDocuments(p.ID, kind)
}

func (a *App) GetDocument(id string) (db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return db.Document{}, errors.New("no project open")
	}
	return d.GetDocument(id)
}

// NewDocument creates a fresh document with default content for the
// requested kind.
func (a *App) NewDocument(kind, title string) (db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return db.Document{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Document{}, err
	}
	def, ok := documents.Get(documents.Kind(kind))
	if !ok {
		return db.Document{}, fmt.Errorf("unknown document kind %q", kind)
	}
	if title == "" {
		title = def.Name
	}
	return d.SaveDocument(db.Document{
		ProjectID: p.ID,
		Kind:      kind,
		Title:     title,
		Content:   documents.DefaultContent(documents.Kind(kind)),
		Version:   1,
		Status:    "draft",
	})
}

func (a *App) SaveDocument(doc db.Document) (db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return db.Document{}, errors.New("no project open")
	}
	return d.SaveDocument(doc)
}

func (a *App) DeleteDocument(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteDocument(id)
}

// ExportCombinedReport assembles multiple documents into one PDF.
// `sections` is an ordered list of {document_id, title, description}
// tuples — the report renders sections in that order. Returns the
// absolute path the PDF was written to (under the user's exports/).
func (a *App) ExportCombinedReport(reportTitle, subtitle string, sections []documents.ReportSection) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	if len(sections) == 0 {
		return "", errors.New("report has no sections")
	}

	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	// Resolve each section to a (doc kind + content) pair, and along
	// the way collect every chart_ref value so we can pre-fetch the
	// referenced charts in one pass.
	resolved := make([]documents.ResolvedSection, 0, len(sections))
	chartIDs := make(map[string]struct{})
	for _, s := range sections {
		doc, err := d.GetDocument(s.DocumentID)
		if err != nil {
			return "", fmt.Errorf("section %s: %w", s.DocumentID, err)
		}
		if s.Title == "" {
			s.Title = doc.Title
		}
		resolved = append(resolved, documents.ResolvedSection{
			Section: s,
			Kind:    documents.Kind(doc.Kind),
			Content: doc.Content,
			Version: doc.Version,
			Status:  doc.Status,
		})

		// Scan the document's content for chart_ref values. We
		// don't unmarshal the JSON twice — that work happens again
		// in renderSectionBody — but a cheap string-key lookup is
		// fine because chart_ref values are short opaque IDs.
		for _, id := range collectChartRefs(doc.Content, documents.EffectiveFields(documents.Kind(doc.Kind))) {
			chartIDs[id] = struct{}{}
		}
	}

	// Pre-fetch every referenced chart.
	resolvedCharts := make(map[string]documents.ResolvedChart, len(chartIDs))
	for id := range chartIDs {
		c, err := d.GetChart(id)
		if err != nil {
			// Skip silently; report.go's fallback handles missing charts.
			continue
		}
		resolvedCharts[id] = documents.ResolvedChart{
			Kind:  c.Kind,
			Title: c.Title,
			Data:  c.Data,
		}
	}

	bytes, err := documents.BuildCombinedReport(documents.ReportSpec{
		ReportTitle:    reportTitle,
		Subtitle:       subtitle,
		Author:         u.DisplayName,
		ProjectName:    proj.Name,
		Sections:       sections,
		ResolvedCharts: resolvedCharts,
	}, resolved)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.pdf", sanitizeFilename(reportTitle), stamp))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// ExportCombinedReportSigned is like ExportCombinedReport but applies a
// real PAdES B-B digital signature (with visual appearance page) using
// the supplied certificate.
func (a *App) ExportCombinedReportSigned(reportTitle, subtitle string, sections []documents.ReportSection, certPath, certPassword string) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	if len(sections) == 0 {
		return "", errors.New("report has no sections")
	}

	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	// Resolve sections + charts (same logic as unsigned version)
	resolved := make([]documents.ResolvedSection, 0, len(sections))
	chartIDs := make(map[string]struct{})
	for _, s := range sections {
		doc, err := d.GetDocument(s.DocumentID)
		if err != nil {
			return "", fmt.Errorf("section %s: %w", s.DocumentID, err)
		}
		if s.Title == "" {
			s.Title = doc.Title
		}
		resolved = append(resolved, documents.ResolvedSection{
			Section: s,
			Kind:    documents.Kind(doc.Kind),
			Content: doc.Content,
			Version: doc.Version,
			Status:  doc.Status,
		})
		for _, id := range collectChartRefs(doc.Content, documents.EffectiveFields(documents.Kind(doc.Kind))) {
			chartIDs[id] = struct{}{}
		}
	}

	resolvedCharts := make(map[string]documents.ResolvedChart, len(chartIDs))
	for id := range chartIDs {
		c, err := d.GetChart(id)
		if err != nil {
			continue
		}
		resolvedCharts[id] = documents.ResolvedChart{Kind: c.Kind, Title: c.Title, Data: c.Data}
	}

	bytes, err := documents.BuildCombinedReport(documents.ReportSpec{
		ReportTitle:       reportTitle,
		Subtitle:          subtitle,
		Author:            u.DisplayName,
		ProjectName:       proj.Name,
		Sections:          sections,
		ResolvedCharts:    resolvedCharts,
		AddSignatureBlock: true,
	}, resolved)
	if err != nil {
		return "", err
	}

	// Apply real PAdES B-B signature
	signer, err := crypto.LoadCertificate(certPath, certPassword)
	if err != nil {
		return "", fmt.Errorf("load certificate: %w", err)
	}

	signedBytes, err := pdfmeta.InjectPAdESSignature(bytes, signer.SignPDFCMS)
	if err != nil {
		return "", fmt.Errorf("pades embedding: %w", err)
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-signed.pdf", sanitizeFilename(reportTitle), stamp))
	if err := os.WriteFile(outPath, signedBytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// RepairAndSwap runs InformativeSelfHeal and, on success, calls
// SwapInSnapshot to atomically replace the live file. The handle on
// `a.db` is refreshed in place.
func (a *App) RepairAndSwap() (db.RepairResult, error) {
	a.mu.RLock()
	d := a.db
	path := a.dbPath
	var dek []byte
	if len(a.dek) == crypto.DEKSize {
		dek = make([]byte, len(a.dek))
		copy(dek, a.dek)
	}
	a.mu.RUnlock()
	if d == nil {
		return db.RepairResult{}, errors.New("no project open")
	}

	result, err := d.InformativeSelfHeal(path)
	if err != nil || !result.Success {
		return result, err
	}
	// If the result.Log mentions a snapshot, do the swap. We detect
	// this by checking for a .bak file rather than re-parsing the log.
	if _, statErr := os.Stat(path + ".bak"); statErr == nil {
		encrypted, err := db.IsEncryptedFile(path)
		if err != nil {
			result.Log = append(result.Log, "Swap failed: "+err.Error())
			return result, err
		}
		var fresh *db.Database
		if encrypted {
			if len(dek) != crypto.DEKSize {
				err := errors.New("database key is locked; sign in again")
				result.Log = append(result.Log, "Swap failed: "+err.Error())
				return result, err
			}
			fresh, err = d.SwapInEncryptedSnapshot(path, dek)
		} else {
			fresh, err = d.SwapInSnapshot(path)
		}
		if err != nil {
			result.Log = append(result.Log, "Swap failed: "+err.Error())
			return result, err
		}
		a.mu.Lock()
		a.db = fresh
		a.adminSvc = admin.NewService(fresh)
		a.mu.Unlock()
		result.Log = append(result.Log, "Snapshot swapped into place; live file is now the healed copy.")
	}
	return result, nil
}

// ExportDocumentDOCX renders the document to a Microsoft Word file
// under the user's exports/ folder and returns the absolute path
// written. Uses gomutex/godocx under the hood.
func (a *App) ExportDocumentDOCX(id string) (string, error) {
	return a.exportDocumentAs(id, ".docx", func(kind documents.Kind, content, projectName string) ([]byte, error) {
		return export.RenderDocumentDOCX(kind, content, projectName)
	})
}

// ExportDocumentODT renders the document to an OpenDocument Text
// file. Sibling to ExportDocumentDOCX; uses the hand-built ODT
// generator in internal/export/odt.go.
func (a *App) ExportDocumentODT(id string) (string, error) {
	return a.exportDocumentAs(id, ".odt", func(kind documents.Kind, content, projectName string) ([]byte, error) {
		return export.RenderDocumentODT(kind, content, projectName)
	})
}

// ExportScheduleReportDOCX generates a Microsoft Word report of the
// current project's CPM schedule (tasks with full ES/EF/LS/LF/Float/
// Critical data) and saves it to the user's exports folder.
func (a *App) ExportScheduleReportDOCX() (string, error) {
	return a.exportScheduleReportAs(export.FormatDOCX)
}

// ExportScheduleReportODT generates an OpenDocument Text report of the
// current project's CPM schedule and saves it to the user's exports folder.
func (a *App) ExportScheduleReportODT() (string, error) {
	return a.exportScheduleReportAs(export.FormatODT)
}

// ExportScheduleReportPDF generates a PDF report of the current project's
// CPM schedule (for completeness with the other formats).
func (a *App) ExportScheduleReportPDF() (string, error) {
	return a.exportScheduleReportAs(export.FormatPDF)
}

// exportDocumentAs is the shared body of every per-format export
// method on App: fetch the document, call the format-specific
// renderer, write to the user's exports/ folder.
func (a *App) exportDocumentAs(
	id, extension string,
	renderer func(documents.Kind, string, string) ([]byte, error),
) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	doc, err := d.GetDocument(id)
	if err != nil {
		return "", err
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	bytes, err := renderer(documents.Kind(doc.Kind), doc.Content, proj.Name)
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s%s",
		sanitizeFilename(doc.Title),
		time.Now().UTC().Format("20060102-150405"),
		extension,
	))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// exportScheduleReportAs is the shared implementation for exporting
// the current project's CPM schedule (Administrative Pack report) in
// DOCX or ODT.
func (a *App) exportScheduleReportAs(format export.ExportFormat) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}

	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	// Best-effort load of current schedule data.
	// V2 priority: active CPM chart (the one the user is actually maintaining).
	// Fallback: legacy V1 tasks table (for old projects).
	kernelTasks, err := loadCurrentProjectSchedule(d, proj.ID)
	if err != nil {
		// Non-fatal for export — we can still produce an empty report.
		kernelTasks = make(map[string]*kernel.Task)
	}

	if len(kernelTasks) > 0 {
		// Full scheduling pipeline: arm date constraints, run CPM,
		// anchor onto real dates (the latter two steps only when the
		// project has a parseable start date).
		scheduleProjectTasks(proj, kernelTasks)
	}

	payload := export.ReportPayload{Tasks: kernelTasks}

	// Earned-value summary at today's status date — only when the
	// project is anchored (offsets map to dates); renderers further
	// suppress the section when there is no cost data.
	if start, ok := parseProjectDate(proj.StartDate); ok && len(kernelTasks) > 0 {
		cal := calendar.For(proj.CountryCode)
		if day, dok := kernel.DayOffset(start, time.Now().UTC(), cal.IsWorkday); dok {
			m := kernel.ComputeEVM(kernelTasks, day)
			payload.EVM = &m
		}
	}

	opts := export.ExportOptions{
		Format: format,
		Title:  proj.Name,
	}

	raw, err := export.GenerateArchivalReport(payload, opts)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}

	ext := ".docx"
	if format == export.FormatODT {
		ext = ".odt"
	}

	outPath := filepath.Join(outDir, fmt.Sprintf("Schedule-Report-%s%s",
		time.Now().UTC().Format("20060102-150405"),
		ext,
	))

	if err := os.WriteFile(outPath, raw, 0o600); err != nil {
		return "", err
	}

	// Best-effort audit (the audit table exists; we log via a simple insert if the helper is available in future).
	// For now we rely on the command_log + file presence for traceability.

	return outPath, nil
}

// stakeholderCapacities builds the resource-capacity map the kernel's
// overallocation detection and levelling consume: stakeholder name →
// availability in units (1.0 = full-time). Assignments naming
// resources that are not stakeholders fall back to the kernel's 1.0
// default. Best-effort: a lookup failure returns nil (default
// capacities) rather than blocking scheduling.
func stakeholderCapacities(d *db.Database, projectID string) map[string]float64 {
	list, err := d.ListStakeholders(projectID, "")
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make(map[string]float64, len(list))
	for _, s := range list {
		if s.Name != "" && s.Availability > 0 {
			out[s.Name] = s.Availability
		}
	}
	return out
}

// scheduleProjectTasks runs the full scheduling pipeline on a kernel
// task map: date constraints are armed against the project start date
// and country work calendar, CPM computes the schedule, and the
// offsets are anchored onto real dates. Projects without a parseable
// start date still get plain CPM (date constraints stay dormant and
// no calendar dates are emitted), preserving legacy behaviour.
func scheduleProjectTasks(proj db.Project, tasks map[string]*kernel.Task) {
	if len(tasks) == 0 {
		return
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		kernel.CalculateCPM(tasks)
		return
	}
	c := calendar.For(proj.CountryCode)
	kernel.ApplyConstraintDates(tasks, start, c.IsWorkday)
	kernel.CalculateCPM(tasks)
	kernel.AnchorSchedule(tasks, start, c.IsWorkday)
}

// parseProjectDate accepts the two date shapes stored in
// project.start_date: plain YYYY-MM-DD and full RFC3339.
func parseProjectDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// loadCurrentProjectSchedule returns the best available schedule data
// as a kernel.Task map (with CPM fields computed).
//
// V2 path: newest CPM chart for the project.
// V1 fallback: legacy tasks table.
func loadCurrentProjectSchedule(d *db.Database, projectID string) (map[string]*kernel.Task, error) {
	// 1. Try current V2 CPM chart (preferred)
	if chs, err := d.ListCharts(projectID, string(charts.KindCPM)); err == nil && len(chs) > 0 {
		// Most recently updated
		sort.Slice(chs, func(i, j int) bool { return chs[i].UpdatedAt.After(chs[j].UpdatedAt) })
		if tasks, err := cpmChartDataToKernelTasks(chs[0].Data); err == nil && len(tasks) > 0 {
			return tasks, nil
		}
	}

	// 2. Fallback to V1 tasks table
	return loadV1TasksAsKernel(d)
}

func cpmChartDataToKernelTasks(dataJSON string) (map[string]*kernel.Task, error) {
	if dataJSON == "" {
		return nil, nil
	}
	var doc struct {
		Nodes []struct {
			ID              string  `json:"id"`
			Label           string  `json:"label"`
			Duration        float64 `json:"duration"`
			Constraint      string  `json:"constraint"`
			ConstraintDate  string  `json:"constraint_date"`
			PercentComplete float64 `json:"percent_complete"`
			Milestone       bool    `json:"milestone"`
			ActualStart     string  `json:"actual_start"`
			ActualFinish    string  `json:"actual_finish"`
			BudgetedCost    float64 `json:"budgeted_cost"`
			ActualCost      float64 `json:"actual_cost"`
			Assignments     []struct {
				Resource string  `json:"resource"`
				Units    float64 `json:"units"`
			} `json:"assignments"`
		} `json:"nodes"`
		Edges []struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Label string `json:"label"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &doc); err != nil {
		return nil, err
	}

	tasks := make(map[string]*kernel.Task, len(doc.Nodes))
	for _, n := range doc.Nodes {
		t := &kernel.Task{
			ID:              n.ID,
			Title:           n.Label,
			Duration:        n.Duration,
			Constraint:      kernel.ConstraintType(strings.ToUpper(strings.TrimSpace(n.Constraint))),
			ConstraintDate:  n.ConstraintDate,
			PercentComplete: n.PercentComplete,
			Milestone:       n.Milestone,
			ActualStart:     n.ActualStart,
			ActualFinish:    n.ActualFinish,
			BudgetedCost:    n.BudgetedCost,
			ActualCost:      n.ActualCost,
		}
		for _, a := range n.Assignments {
			t.Assignments = append(t.Assignments, kernel.Assignment{
				Resource: a.Resource,
				Units:    a.Units,
			})
		}
		tasks[n.ID] = t
	}
	for _, e := range doc.Edges {
		if t, ok := tasks[e.To]; ok {
			typ, lag := dag.ParseLinkLabel(e.Label)
			t.Links = append(t.Links, kernel.Link{Pred: e.From, Type: typ, Lag: lag})
		}
	}
	return tasks, nil
}

func loadV1TasksAsKernel(d *db.Database) (map[string]*kernel.Task, error) {
	rows, err := d.Conn.Query(`SELECT id, title, duration, precedents FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make(map[string]*kernel.Task)
	for rows.Next() {
		var id, title, precJSON string
		var duration float64
		if err := rows.Scan(&id, &title, &duration, &precJSON); err != nil {
			continue
		}
		var precedents []string
		_ = json.Unmarshal([]byte(precJSON), &precedents)

		tasks[id] = &kernel.Task{
			ID:         id,
			Title:      title,
			Duration:   duration,
			Precedents: precedents,
		}
	}
	return tasks, nil
}

// ExportDocumentPDF renders the document to PDF under the user's
// exports/ folder and returns the absolute path written.
func (a *App) ExportDocumentPDF(id string) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	doc, err := d.GetDocument(id)
	if err != nil {
		return "", err
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	bytes, err := documents.Render(documents.Kind(doc.Kind), doc.Content, proj.Name)
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.pdf",
		sanitizeFilename(doc.Title), time.Now().UTC().Format("20060102-150405")))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// ExportDocumentPDFSigned is like ExportDocumentPDF but applies a real
// PAdES B-B digital signature using the provided certificate.
func (a *App) ExportDocumentPDFSigned(id, certPath, certPassword string) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	doc, err := d.GetDocument(id)
	if err != nil {
		return "", err
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	bytes, err := documents.RenderSigned(documents.Kind(doc.Kind), doc.Content, proj.Name, certPath, certPassword)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-signed.pdf",
		sanitizeFilename(doc.Title), time.Now().UTC().Format("20060102-150405")))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// =========================================================
// V1 settings (kept for compat with the V1 Settings panel)
// =========================================================

func (a *App) GetSettings() (db.UserSettings, error) {
	d := a.requireDB()
	if d == nil {
		return db.UserSettings{}, errors.New("no project open")
	}
	return d.GetSettings()
}

func (a *App) SaveSettings(s db.UserSettings) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.SaveSettings(s)
}

func (a *App) SecureArchive(projectPath string) (string, error) {
	a.mu.RLock()
	svc := a.adminSvc
	a.mu.RUnlock()
	if svc == nil {
		return "", errors.New("no project open")
	}
	return svc.SecureArchive(projectPath)
}

// =========================================================
// Agile Pack (V2.x — Kanban / Sprints / DORA)
// =========================================================
//
// All methods below build an agile.Store on demand, scoped to the
// currently-open project. Callers MUST have a project open;
// otherwise an "agile: no project" error is returned.

func (a *App) agileStore() (*agile.Store, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("agile: no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return agile.NewStore(d.Conn, p.ID), nil
}

// AgileEnabled reports whether the Software-Dev Pack is active for the
// open project. The value is read from settings on each project open and
// cached in agile.PackEnabled for cheap in-process checks.
func (a *App) AgileEnabled() (bool, error) {
	d := a.requireDB()
	if d == nil {
		return agile.PackEnabled, nil
	}
	s, err := d.GetSettings()
	if err != nil {
		return agile.PackEnabled, fmt.Errorf("AgileEnabled: %w", err)
	}
	agile.PackEnabled = s.AgileEnabled
	return s.AgileEnabled, nil
}

// SetAgileEnabled persists the Software-Dev Pack toggle to the project
// settings and updates the in-process cache.
func (a *App) SetAgileEnabled(enabled bool) error {
	agile.PackEnabled = enabled
	d := a.requireDB()
	if d == nil {
		return nil
	}
	s, err := d.GetSettings()
	if err != nil {
		return fmt.Errorf("SetAgileEnabled: %w", err)
	}
	s.AgileEnabled = enabled
	return d.SaveSettings(s)
}

// EnsureDefaultBoard returns (and creates if missing) the default
// Kanban board for the open project, along with its seeded columns.
func (a *App) EnsureDefaultBoard() (agile.Board, []agile.Column, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.Board{}, nil, err
	}
	b, err := s.EnsureDefaultBoard()
	if err != nil {
		return agile.Board{}, nil, err
	}
	cols, err := s.ListColumns(b.ID)
	if err != nil {
		return agile.Board{}, nil, err
	}
	return b, cols, nil
}

// SaveColumn upserts a column (rename, change WIP, reorder).
func (a *App) SaveColumn(c agile.Column) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.SaveColumn(c)
}

// DeleteColumn removes a column. The frontend warns about
// re-homing work items before calling this.
func (a *App) DeleteColumn(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteColumn(id)
}

// SaveWorkItem inserts or updates a work item.
func (a *App) SaveWorkItem(wi agile.WorkItem) (agile.WorkItem, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.WorkItem{}, err
	}
	return s.SaveWorkItem(wi)
}

// GetWorkItem fetches one by ID.
func (a *App) GetWorkItem(id string) (agile.WorkItem, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.WorkItem{}, err
	}
	return s.GetWorkItem(id)
}

// ListWorkItems returns the project's work items, optionally
// filtered by sprintID, state (column ID), and assignee. Pass
// empty strings to disable a filter.
func (a *App) ListWorkItems(sprintID, state, assignee string) ([]agile.WorkItem, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	return s.ListWorkItems(sprintID, state, assignee)
}

// DeleteWorkItem removes a work item.
func (a *App) DeleteWorkItem(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteWorkItem(id)
}

// MoveWorkItem is the Kanban drag-and-drop hook: change a work
// item's state (= destination column ID) and its order within that
// column atomically.
func (a *App) MoveWorkItem(id, newState string, newOrder int) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.MoveWorkItem(id, newState, newOrder)
}

// WIPCounts returns the current count of work items per column,
// for the WIP-breach indicators on the Kanban board.
func (a *App) WIPCounts() (map[string]int, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	return s.WIPCountByColumn()
}

// SaveSprint upserts a sprint.
func (a *App) SaveSprint(sp agile.Sprint) (agile.Sprint, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.Sprint{}, err
	}
	return s.SaveSprint(sp)
}

// ListSprints returns every sprint for the open project.
func (a *App) ListSprints() ([]agile.Sprint, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	return s.ListSprints()
}

// DeleteSprint removes a sprint and unlinks its work items
// (transactionally).
func (a *App) DeleteSprint(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteSprint(id)
}

// SaveDeployment upserts a deployment record (feeds DORA metrics).
func (a *App) SaveDeployment(d agile.Deployment) (agile.Deployment, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.Deployment{}, err
	}
	return s.SaveDeployment(d)
}

// ListDeployments returns deployments newer than `sinceISO` (RFC3339
// timestamp). Pass "" for all deployments.
func (a *App) ListDeployments(sinceISO string) ([]agile.Deployment, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	var since time.Time
	if sinceISO != "" {
		if t, err := time.Parse(time.RFC3339, sinceISO); err == nil {
			since = t
		}
	}
	return s.ListDeployments(since)
}

// DeleteDeployment removes a deployment record.
func (a *App) DeleteDeployment(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteDeployment(id)
}

// =========================================================
// V2.x Foundation Slice — Launchpad / Stakeholders /
// Timeline / Budget / iCal (rickar+zen-go integrations)
// =========================================================

// LaunchpadEvaluate returns the list of seed actions (chart kinds +
// document kinds + agile flags) for a given (industry, methodology)
// pair. The actual decision lives in internal/templates'
// launchpad_seeds.json and is evaluated by zen-go.
func (a *App) LaunchpadEvaluate(industry, methodology string) ([]string, error) {
	if a.templates == nil {
		return []string{"charter"}, nil
	}
	resp, err := a.templates.Evaluate(a.ctx, templates.SeedRequest{
		Industry:    industry,
		Methodology: methodology,
	})
	if err != nil {
		return nil, err
	}
	return resp.Seeds, nil
}

// CreateProjectFromLaunchpad creates a new .pmforge file just like
// CreateProject, then applies the seed actions returned by the
// templates engine. The receipts slice records what was created so
// the GUI can show the user a "we set up the following for you" toast.
func (a *App) CreateProjectFromLaunchpad(
	name, description, industry, subCategory, methodology, countryCode string,
	seeds []string,
) (db.Project, []templates.SeedReceipt, string, error) {
	user := a.requireUser()
	if user == nil {
		return db.Project{}, nil, "", errors.New("not signed in")
	}
	safe := sanitizeFilename(name)
	if safe == "" {
		return db.Project{}, nil, "", errors.New("invalid project name")
	}
	dir := filepath.Join(user.DataDir, "projects")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return db.Project{}, nil, "", err
	}
	a.mu.RLock()
	dek, err := a.requireDEKLocked()
	a.mu.RUnlock()
	if err != nil {
		return db.Project{}, nil, "", err
	}

	// Choose a non-conflicting filename, same logic as CreateProject.
	path := filepath.Join(dir, safe+".pmforge")
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d.pmforge", safe, i))
	}

	d, err := db.InitEncryptedDB(path, dek)
	if err != nil {
		return db.Project{}, nil, "", err
	}
	// We close the local handle at the end and rely on OpenProject
	// to install the project as the app's active one, so the flow
	// matches what the user expects after CreateProject.
	proj, err := d.UpsertProject(db.Project{
		Name:        name,
		Description: description,
		Status:      "planning",
		Phase:       "initiation",
		Owner:       user.Username,
		Industry:    industry,
		SubCategory: subCategory,
		Methodology: methodology,
		CountryCode: countryCodeOrDefault(countryCode),
	})
	if err != nil {
		_ = d.Close()
		return db.Project{}, nil, "", err
	}

	// Apply seeds via the dedicated seeder.
	seeder := templates.NewSeeder(d, proj.ID)
	receipts, seedErr := seeder.Apply(seeds)
	_ = d.Close()

	// Even on seedErr we keep the project — the user can fix it
	// from the dashboard. Bubble the error up so the GUI shows a
	// notice.
	if seedErr != nil {
		return proj, receipts, path, fmt.Errorf("project created but seeding partial: %w", seedErr)
	}
	return proj, receipts, path, nil
}

// UpdateProjectIndustry persists changes to industry / sub-category /
// methodology / country code on an already-open project. Used by the
// project Settings view if the user reclassifies later.
func (a *App) UpdateProjectIndustry(industry, subCategory, methodology, countryCode string) (db.Project, error) {
	d := a.requireDB()
	if d == nil {
		return db.Project{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Project{}, err
	}
	p.Industry = industry
	p.SubCategory = subCategory
	p.Methodology = methodology
	p.CountryCode = countryCodeOrDefault(countryCode)
	return d.UpsertProject(p)
}

// ----- Stakeholders -----

// ListStakeholders returns every stakeholder for the open project,
// optionally filtered by category ("team" / "vendor" / "sponsor" /
// "external").
func (a *App) ListStakeholders(category string) ([]db.Stakeholder, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListStakeholders(p.ID, category)
}

// SaveStakeholder upserts a stakeholder. Empty ID creates one.
func (a *App) SaveStakeholder(s db.Stakeholder) (db.Stakeholder, error) {
	d := a.requireDB()
	if d == nil {
		return db.Stakeholder{}, errors.New("no project open")
	}
	if s.ProjectID == "" {
		p, err := d.GetProject()
		if err != nil {
			return db.Stakeholder{}, err
		}
		s.ProjectID = p.ID
	}
	return d.SaveStakeholder(s)
}

// DeleteStakeholder removes a stakeholder.
func (a *App) DeleteStakeholder(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteStakeholder(id)
}

// ----- Timeline + Budget -----

var errTimelineSourceMismatch = errors.New("timeline: source id does not match open project")

// BuildTimeline returns the project's chronological event stream
// (project start/end, sprint start/end, deployments).
func (a *App) BuildTimeline() ([]timeline.Entry, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	return buildTimelineFromDB(d)
}

// MoveTimelineEntry updates an editable timeline boundary and returns
// the rebuilt event stream. Only project and sprint date boundaries are
// editable from the Timeline view; deployments remain immutable DORA
// history and must be edited through the deployment log.
func (a *App) MoveTimelineEntry(kind, sourceID, dateISO string) ([]timeline.Entry, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	date, err := normaliseTimelineMoveDate(dateISO)
	if err != nil {
		return nil, err
	}

	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	store := agile.NewStore(d.Conn, p.ID)

	switch timeline.EntryKind(kind) {
	case timeline.KindProjectStart:
		if sourceID != "" && sourceID != p.ID {
			return nil, errTimelineSourceMismatch
		}
		p.StartDate = date
		if err := validateTimelineDateRange(p.StartDate, p.EndDate, "project"); err != nil {
			return nil, err
		}
		if _, err := d.UpsertProject(p); err != nil {
			return nil, err
		}
	case timeline.KindProjectEnd:
		if sourceID != "" && sourceID != p.ID {
			return nil, errTimelineSourceMismatch
		}
		p.EndDate = date
		if err := validateTimelineDateRange(p.StartDate, p.EndDate, "project"); err != nil {
			return nil, err
		}
		if _, err := d.UpsertProject(p); err != nil {
			return nil, err
		}
	case timeline.KindSprintStart, timeline.KindSprintEnd:
		if sourceID == "" {
			return nil, errors.New("timeline: sprint move requires a source id")
		}
		sp, err := store.GetSprint(sourceID)
		if err != nil {
			return nil, err
		}
		if timeline.EntryKind(kind) == timeline.KindSprintStart {
			sp.StartDate = date
		} else {
			sp.EndDate = date
		}
		if err := validateTimelineDateRange(sp.StartDate, sp.EndDate, "sprint"); err != nil {
			return nil, err
		}
		if _, err := store.SaveSprint(sp); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("timeline: %s entries are read-only", kind)
	}

	return buildTimelineFromDB(d)
}

func buildTimelineFromDB(d *db.Database) ([]timeline.Entry, error) {
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	store := agile.NewStore(d.Conn, p.ID)
	sprints, err := store.ListSprints()
	if err != nil {
		return nil, err
	}
	deploys, err := store.ListDeployments(time.Time{})
	if err != nil {
		return nil, err
	}
	return timeline.Build(p, sprints, deploys), nil
}

func normaliseTimelineMoveDate(dateISO string) (string, error) {
	t, err := time.Parse("2006-01-02", dateISO)
	if err != nil {
		return "", fmt.Errorf("timeline: date must be YYYY-MM-DD: %w", err)
	}
	return t.Format("2006-01-02"), nil
}

func validateTimelineDateRange(start, end, label string) error {
	if start == "" || end == "" {
		return nil
	}
	startDate, err := time.Parse("2006-01-02", start)
	if err != nil {
		return fmt.Errorf("timeline: %s start date is invalid: %w", label, err)
	}
	endDate, err := time.Parse("2006-01-02", end)
	if err != nil {
		return fmt.Errorf("timeline: %s end date is invalid: %w", label, err)
	}
	if endDate.Before(startDate) {
		return fmt.Errorf("timeline: %s end date cannot be before start date", label)
	}
	return nil
}

// ListHolidays returns the holidays between `fromISO` and `toISO`
// for the open project's country.
func (a *App) ListHolidays(fromISO, toISO string) ([]calendar.HolidayEvent, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	from, err := time.Parse("2006-01-02", fromISO)
	if err != nil {
		return nil, fmt.Errorf("from date: %w", err)
	}
	to, err := time.Parse("2006-01-02", toISO)
	if err != nil {
		return nil, fmt.Errorf("to date: %w", err)
	}
	c := calendar.For(p.CountryCode)
	return c.HolidaysIn(from, to), nil
}

// ComputeBudget rolls up the project's cost picture from stakeholder
// rates × work-item points + vendor contract values.
func (a *App) ComputeBudget() (budget.Summary, error) {
	d := a.requireDB()
	if d == nil {
		return budget.Summary{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return budget.Summary{}, err
	}
	stakeholders, err := d.ListStakeholders(p.ID, "")
	if err != nil {
		return budget.Summary{}, err
	}
	store := agile.NewStore(d.Conn, p.ID)
	workItems, err := store.ListWorkItems("", "", "")
	if err != nil {
		return budget.Summary{}, err
	}
	return budget.Compute(p, stakeholders, workItems), nil
}

// ----- iCal export -----

// CheckLatestVersion runs the signed-manifest update check. The
// frontend calls this from the Settings panel; the result is a
// Status struct describing whether an update is available.
func (a *App) CheckLatestVersion() (update.Status, error) {
	return update.CheckLatest(a.ctx)
}

// ChooseCertFile opens a native file-picker for X.509 certificate
// bundles (.p12 / .pfx). Returns the absolute path the user picked
// or empty string on cancel. Used by SignatureSettings.svelte.
func (a *App) ChooseCertFile() (string, error) {
	if a.ctx == nil {
		return "", errors.New("no context (Wails not started)")
	}
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select signing certificate",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "PKCS#12 bundles (*.p12, *.pfx)",
				Pattern:     "*.p12;*.pfx",
			},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}

// =========================================================
// Fonts
// =========================================================

// fontManager returns a *fonts.Manager scoped to the signed-in user's
// font directory (<DataDir>/fonts). If no user is signed in, the
// manager still serves the bundled catalog but cannot import or list
// user fonts.
func (a *App) fontManager() *fonts.Manager {
	userDir := ""
	if u := a.requireUser(); u != nil {
		userDir = filepath.Join(u.DataDir, "fonts")
	}
	return fonts.NewManager(userDir)
}

// ListFonts returns every font family available for document export:
// the bundled families whose .ttf files are present in the build, plus
// any fonts the user has imported. Each entry reports its origin
// (bundled / user), category, license, and available styles.
func (a *App) ListFonts() []fonts.FamilyInfo {
	return a.fontManager().Available()
}

// ImportFont opens a native file picker for a TrueType (.ttf) font,
// validates it, and copies it into the user's font directory so it
// becomes available for document export. Returns the imported family's
// info. OpenType/CFF (.otf), WOFF, and TrueType Collections are
// rejected with a clear error because the PDF engine embeds TrueType
// outlines only.
func (a *App) ImportFont() (fonts.FamilyInfo, error) {
	if a.ctx == nil {
		return fonts.FamilyInfo{}, errors.New("no context (Wails not started)")
	}
	if a.requireUser() == nil {
		return fonts.FamilyInfo{}, errors.New("not signed in")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select a TrueType font (.ttf)",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "TrueType fonts (*.ttf)", Pattern: "*.ttf"},
		},
	})
	if err != nil {
		return fonts.FamilyInfo{}, err
	}
	if path == "" {
		return fonts.FamilyInfo{}, errors.New("no file selected")
	}
	return a.fontManager().ImportFont(path)
}

// GetDefaultFont returns the document-export font family the user has
// chosen, or the catalog default when unset.
func (a *App) GetDefaultFont() (string, error) {
	d := a.requireDB()
	if d == nil {
		return fonts.DefaultFamily, nil
	}
	s, err := d.GetSettings()
	if err != nil {
		return fonts.DefaultFamily, err
	}
	if s.DefaultFont == "" {
		return fonts.DefaultFamily, nil
	}
	return s.DefaultFont, nil
}

// SetDefaultFont persists the document-export font family. The chosen
// family must be available (bundled-and-fetched or user-imported).
func (a *App) SetDefaultFont(family string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	available := false
	for _, f := range a.fontManager().Available() {
		if f.Name == family {
			available = true
			break
		}
	}
	if !available {
		return fmt.Errorf("font %q is not available", family)
	}
	s, err := d.GetSettings()
	if err != nil {
		return err
	}
	s.DefaultFont = family
	if err := d.SaveSettings(s); err != nil {
		return err
	}
	// Apply immediately so the next export uses the new font.
	documents.UseFont(a.fontManager(), family)
	return nil
}

// ExportProjectICS writes a .ics file with the project's timeline +
// (optionally) the country's holidays to the user's exports/ folder.
// Returns the absolute path. The frontend should open the file in
// the user's default calendar app.
func (a *App) ExportProjectICS(includeHolidays bool) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return "", err
	}
	store := agile.NewStore(d.Conn, p.ID)
	sprints, err := store.ListSprints()
	if err != nil {
		return "", err
	}
	deploys, err := store.ListDeployments(time.Time{})
	if err != nil {
		return "", err
	}
	entries := timeline.Build(p, sprints, deploys)

	events := make([]export.ICalEvent, 0, len(entries))
	for _, e := range entries {
		events = append(events, export.ICalEvent{
			UID:         e.SourceID + "-" + string(e.Kind),
			Summary:     e.Title,
			Description: e.Description,
			Start:       e.Date,
			End:         e.EndDate,
			Category:    string(e.Kind),
		})
	}

	spec := export.ICalSpec{
		CalendarName: p.Name,
		ProjectID:    p.ID,
		Events:       events,
	}
	if includeHolidays {
		cal := calendar.For(p.CountryCode)
		// Span the calendar over the project's window, or a default
		// of one year backward + one year forward when dates are
		// blank.
		from := time.Now().AddDate(-1, 0, 0)
		to := time.Now().AddDate(1, 0, 0)
		if t, ok := parseISODate(p.StartDate); ok {
			from = t
		}
		if t, ok := parseISODate(p.EndDate); ok {
			to = t
		}
		spec = export.AppendHolidayEvents(spec, cal, from, to)
	}

	bytes := export.ICalRender(spec)

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.ics", sanitizeFilename(p.Name), stamp))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// helpers ---------------------------------------------------------

func countryCodeOrDefault(c string) string {
	if c == "" {
		return "US"
	}
	return c
}

func parseISODate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// =========================================================
// (back to existing Agile methods)
// =========================================================

// ComputeDORA runs the four DORA metrics over the last `windowDays`
// of deployments. windowDays <= 0 defaults to 30.
func (a *App) ComputeDORA(windowDays int) (agile.DORAResult, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.DORAResult{}, err
	}
	since := time.Now().AddDate(0, 0, -windowDays)
	deploys, err := s.ListDeployments(since)
	if err != nil {
		return agile.DORAResult{}, err
	}
	return agile.ComputeDORA(deploys, windowDays, time.Now().UTC()), nil
}

// =========================================================
// helpers
// =========================================================

// requireUser returns the active session pointer under a read lock.
// The returned pointer is safe to dereference for the caller's
// lifetime — *users.Account is not freed by Logout (Go GC), and the
// fields the GUI reads are immutable after Login.
func (a *App) requireUser() *users.Account {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.user
}

// requireDEKLocked returns a copy of the active user's unlocked DEK.
// Caller must hold a.mu for reading or writing.
func (a *App) requireDEKLocked() ([]byte, error) {
	if len(a.dek) != crypto.DEKSize {
		return nil, errors.New("database key is locked; sign in again")
	}
	dek := make([]byte, len(a.dek))
	copy(dek, a.dek)
	return dek, nil
}

// requireDB returns the open *db.Database under a read lock. A
// concurrent Logout/CloseProject may Close the returned handle
// before the caller's query runs; the caller receives "sql:
// database is closed" rather than a crash. Acceptable for a
// single-user desktop app; see AGENT.md §6.
func (a *App) requireDB() *db.Database {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.db
}

// requireDBAndPath is the read-lock helper for callers that need
// both the db handle and its file path. Holding RLock once for both
// fields keeps them consistent across concurrent Logout.
func (a *App) requireDBAndPath() (*db.Database, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.db, a.dbPath
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return absA == absB
	}
	return a == b
}

// =========================================================
// Process Excellence Suite (Six Sigma) — MVP 1
// =========================================================

func (a *App) SigmaCreateProject(title, description string, beltLevel string) (domain.Project, error) {
	d := a.requireDB()
	if d == nil {
		return domain.Project{}, fmt.Errorf("sigma: no project open")
	}
	input := domain.Project{
		Title:       title,
		Description: description,
		BeltLevel:   domain.BeltLevel(beltLevel),
	}
	p, err := a.sigmaSvc.CreateProject(input)
	if err != nil {
		return domain.Project{}, err
	}
	return *p, nil
}

func (a *App) SigmaListProjects() ([]domain.Project, error) {
	d := a.requireDB()
	if d == nil {
		return nil, fmt.Errorf("sigma: no project open")
	}
	return a.sigmaSvc.ListProjects()
}

func (a *App) SigmaGetProject(id string) (domain.Project, error) {
	d := a.requireDB()
	if d == nil {
		return domain.Project{}, fmt.Errorf("sigma: no project open")
	}
	p, err := a.sigmaSvc.GetProject(id)
	if err != nil {
		return domain.Project{}, err
	}
	return *p, nil
}

func (a *App) SigmaSaveCharter(c domain.Charter) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return a.sigmaSvc.SaveCharter(c)
}

func (a *App) SigmaGetCharter(projectID string) (domain.Charter, error) {
	d := a.requireDB()
	if d == nil {
		return domain.Charter{}, fmt.Errorf("sigma: no project open")
	}
	c, err := a.sigmaSvc.GetCharter(projectID)
	if err != nil {
		return domain.Charter{}, err
	}
	return *c, nil
}

func (a *App) SigmaAdvancePhase(projectID, phase string) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no project open")
	}
	// Check readiness of the CURRENT phase before allowing advance
	// We need to know the current phase to check it.
	// For MVP, we check Define readiness if moving FROM Define.
	// In a real app, we'd pass currentPhase or fetch it.
	// Let's fetch the project to get current phase.
	p, err := a.sigmaSvc.GetProject(projectID)
	if err != nil {
		return err
	}

	// Only gate the Define phase for MVP 1
	if p.Phase == domain.PhaseDefine && phase != string(domain.PhaseDefine) {
		charter, _ := a.sigmaSvc.GetCharter(projectID)
		sipoc, _ := a.sigmaSvc.GetSIPOC(projectID)
		voc, _ := a.sigmaSvc.GetVoC(projectID)
		res := tollgate.CheckDefineReadiness(*charter, sipoc, voc)
		if !res.CanAdvance {
			return fmt.Errorf("sigma: Define phase readiness is %.0f%% (need 80%%). Missing: %s", res.Score, res.MissingList)
		}
	}

	return a.sigmaSvc.AdvancePhase(projectID, domain.Phase(phase))
}

// SigmaCalculateDescriptive returns mean, median, std dev, min, max for a dataset.
func (a *App) SigmaCalculateDescriptive(values []float64) (stats.DescriptiveResult, error) {
	return stats.CalculateDescriptive(values)
}

// SigmaCalculateCapability returns Cp, Cpk, Pp, Ppk, Sigma Level, DPMO.
func (a *App) SigmaCalculateCapability(values []float64, usl, lsl float64) (stats.CapabilityResult, error) {
	return stats.CalculateCapability(values, usl, lsl)
}

// SigmaCalculatePareto returns sorted categories with cumulative percentages.
func (a *App) SigmaCalculatePareto(categories []string, counts []int) ([]sigmacharts.ParetoItem, error) {
	return sigmacharts.CalculatePareto(categories, counts)
}

// SigmaCheckReadiness evaluates the current phase tollgate requirements.
func (a *App) SigmaCheckReadiness(projectID, phase string) (tollgate.Result, error) {
	d := a.requireDB()
	if d == nil {
		return tollgate.Result{}, fmt.Errorf("sigma: no projectopen")
	}
	charter, err := a.sigmaSvc.GetCharter(projectID)
	if err != nil {
		return tollgate.Result{}, err
	}
	sipoc, _ := a.sigmaSvc.GetSIPOC(projectID)
	voc, _ := a.sigmaSvc.GetVoC(projectID)
	fb, _ := a.sigmaSvc.GetFishbone(projectID)
	solutions, _ := a.sigmaSvc.GetSolutions(projectID)
	controlPlan, _ := a.sigmaSvc.GetControlPlan(projectID)
	return tollgate.CheckPhase(domain.Phase(phase), *charter, sipoc, voc, fb, solutions, controlPlan), nil
}

// SigmaSaveFishbone persists the Fishbone diagram data.
func (a *App) SigmaSaveFishbone(projectID string, fb domain.FishboneData) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return a.sigmaSvc.SaveFishbone(fb, projectID)
}

// SigmaGetFishbone retrieves the Fishbone diagram data.
func (a *App) SigmaGetFishbone(projectID string) (domain.FishboneData, error) {
	d := a.requireDB()
	if d == nil {
		return domain.FishboneData{}, fmt.Errorf("sigma: no projectopen")
	}
	fb, err := a.sigmaSvc.GetFishbone(projectID)
	if err != nil {
		return domain.FishboneData{}, err
	}
	return *fb, nil
}

// SigmaSaveSolutions persists the Solution Selection Matrix data.
func (a *App) SigmaSaveSolutions(projectID string, solutions []domain.Solution) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.SaveSolutions(projectID, solutions)
}

// SigmaGetSolutions retrieves the Solution Selection Matrix data.
func (a *App) SigmaGetSolutions(projectID string) ([]domain.Solution, error) {
	d := a.requireDB()
	if d == nil {
		return nil, fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.GetSolutions(projectID)
}

// SigmaSaveControlPlan persists the Control Plan data.
func (a *App) SigmaSaveControlPlan(projectID string, items []domain.ControlPlanItem) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.SaveControlPlan(projectID, items)
}

// SigmaGetControlPlan retrieves the Control Plan data.
func (a *App) SigmaGetControlPlan(projectID string) ([]domain.ControlPlanItem, error) {
	d := a.requireDB()
	if d == nil {
		return nil, fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.GetControlPlan(projectID)
}

// SigmaSaveSIPOC persists the SIPOC diagram data.
func (a *App) SigmaSaveSIPOC(projectID string, data domain.SIPOCData) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.SaveSIPOC(projectID, data)
}

// SigmaGetSIPOC retrieves the SIPOC diagram data.
func (a *App) SigmaGetSIPOC(projectID string) (domain.SIPOCData, error) {
	d := a.requireDB()
	if d == nil {
		return domain.SIPOCData{}, fmt.Errorf("sigma: no projectopen")
	}
	sipoc, err := a.sigmaSvc.GetSIPOC(projectID)
	if err != nil {
		return domain.SIPOCData{}, err
	}
	return *sipoc, nil
}

// SigmaSaveVoC persists the Voice of Customer data.
func (a *App) SigmaSaveVoC(projectID string, data domain.VoCData) error {
	d := a.requireDB()
	if d == nil {
		return fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.SaveVoC(projectID, data)
}

// SigmaGetVoC retrieves the Voice of Customer data.
func (a *App) SigmaGetVoC(projectID string) (domain.VoCData, error) {
	d := a.requireDB()
	if d == nil {
		return domain.VoCData{}, fmt.Errorf("sigma: no projectopen")
	}
	voc, err := a.sigmaSvc.GetVoC(projectID)
	if err != nil {
		return domain.VoCData{}, err
	}
	return *voc, nil
}

// SigmaGetToolStatus returns the completion status of tools for the given phase.
func (a *App) SigmaGetToolStatus(projectID, phase string) (service.PhaseTools, error) {
	d := a.requireDB()
	if d == nil {
		return service.PhaseTools{}, fmt.Errorf("sigma: no projectopen")
	}
	return a.sigmaSvc.GetToolStatus(projectID, phase), nil
}

// SigmaExportProjectReport generates a PDF report of all phase deliverables.
func (a *App) SigmaExportProjectReport(projectID string) (string, error) {
	d := a.requireDB()
	if d == nil {
		return "", fmt.Errorf("sigma: no projectopen")
	}

	project, charter, sipoc, fishbone, solutions, controlPlan, err := a.sigmaSvc.GetProjectReportData(projectID)
	if err != nil {
		return "", err
	}

	return export.GenerateSigmaReport(project, charter, sipoc, fishbone, solutions, controlPlan)
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}

// collectChartRefs scans a document's JSON content for FieldChartRef
// values, returning the chart IDs referenced. Used by
// ExportCombinedReport to pre-fetch every chart needed by the
// included documents in a single pass.
func collectChartRefs(contentJSON string, fields []documents.Field) []string {
	if contentJSON == "" || len(fields) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(contentJSON), &m); err != nil {
		return nil
	}
	var out []string
	for _, f := range fields {
		if f.Type != documents.FieldChartRef {
			continue
		}
		if id, ok := m[f.Key].(string); ok && id != "" {
			out = append(out, id)
		}
	}
	return out
}

// sanitizeFilename strips path separators and disallowed characters
// from a user-supplied project name so it is safe to use as a file
// name on every platform.
func sanitizeFilename(s string) string {
	var b []rune
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b = append(b, '_')
		default:
			if r >= 32 {
				b = append(b, r)
			}
		}
	}
	out := string(b)
	if out == "" {
		return ""
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

// =========================================================
// main: CLI dispatch + Wails launch
// =========================================================

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
	app, err := NewApp()
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "PMForge",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.ctx = ctx
		},
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}

func headlessProjectMode(cfg *cli.Config) bool {
	return cfg.CheckOnly || cfg.Repair || cfg.Vacuum || cfg.ExportAuditPath != ""
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
	defer store.Close()
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
	defer d.Close()

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
	}
}
