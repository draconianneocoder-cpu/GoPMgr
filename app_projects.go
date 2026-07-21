// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"pmforge/internal/admin"
	"pmforge/internal/applog"
	"pmforge/internal/calendar"
	"pmforge/internal/cli"
	"pmforge/internal/db"
	"pmforge/internal/documents"
	"pmforge/internal/fonts"
	"pmforge/internal/sigma/service"
	"pmforge/internal/users"
	"regexp"
	"runtime"
	"strings"
	"time"
)

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
	entries, err := enumerateProjects(dir)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectFile, 0, len(entries))
	for _, e := range entries {
		out = append(out, ProjectFile(e))
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

	// Each project gets its own uniquely-named, time-stamped subfolder.
	path, err := newProjectPath(dir, safe)
	if err != nil {
		return ProjectFile{}, err
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
	a.applyGlobalDefaults(d)
	_ = d.Close()

	return ProjectFile{
		Path:     path,
		Name:     name,
		Modified: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// projectPathFor validates that path points at a .pmforge file inside the
// signed-in user's own projects directory and returns the cleaned path plus
// the account. It rejects anything outside that directory so DeleteProject
// and CloneProject can never touch arbitrary files on disk.
func (a *App) projectPathFor(path string) (string, *users.Account, error) {
	user := a.requireUser()
	if user == nil {
		return "", nil, errors.New("not signed in")
	}
	clean := filepath.Clean(path)
	if filepath.Ext(clean) != ".pmforge" {
		return "", nil, errors.New("not a project file")
	}
	projectsDir := filepath.Clean(filepath.Join(user.DataDir, "projects"))
	parent := filepath.Dir(clean)
	// Allowed: legacy flat layout (<projects>/<name>.pmforge) where parent is
	// the projects dir, OR the current layout (<projects>/<id>/project.pmforge)
	// where the parent is an immediate subfolder of the projects dir. Anything
	// deeper or outside is rejected.
	if parent != projectsDir && filepath.Dir(parent) != projectsDir {
		return "", nil, errors.New("project is outside your projects folder")
	}
	return clean, user, nil
}

// DeleteProject permanently removes a project's .pmforge file and its
// WAL/SHM sidecars from the signed-in user's projects folder. If the project
// is the one currently open it is closed first so we never unlink an in-use
// database. The path must live inside the user's own projects directory.
func (a *App) DeleteProject(path string) error {
	clean, user, err := a.projectPathFor(path)
	if err != nil {
		return err
	}
	a.mu.RLock()
	openPath := a.dbPath
	a.mu.RUnlock()
	if openPath != "" && filepath.Clean(openPath) == clean {
		if err := a.CloseProject(); err != nil {
			return err
		}
	}
	if err := a.appendProjectDeleteAudit(clean, user.Username); err != nil {
		return err
	}
	projectsDir := filepath.Clean(filepath.Join(user.DataDir, "projects"))
	parent := filepath.Dir(clean)
	if parent != projectsDir {
		// Current layout: the project owns its subfolder; remove it whole
		// (DB + WAL/SHM sidecars). projectPathFor already proved `parent` is
		// an immediate child of the user's projects dir, so this is safe.
		return os.RemoveAll(parent)
	}
	// Legacy flat layout: remove just the file and its sidecars.
	for _, p := range []string{clean, clean + "-wal", clean + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (a *App) appendProjectDeleteAudit(path, actor string) error {
	a.mu.RLock()
	dek, err := a.requireDEKLocked()
	a.mu.RUnlock()
	if err != nil {
		return err
	}
	d, err := db.InitEncryptedDB(path, dek)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	project, err := d.GetProject()
	if err != nil {
		return err
	}
	before, err := json.Marshal(project)
	if err != nil {
		return err
	}
	if _, err := d.AppendAuditEvent(db.AuditEventInput{
		ProjectID:  project.ID,
		EventType:  "project.delete",
		EntityType: "project",
		EntityID:   project.ID,
		BeforeJSON: string(before),
		UserID:     actor,
	}); err != nil {
		return err
	}
	return nil
}

// CloneProject duplicates a project file under a new, non-conflicting name in
// the same folder and returns the new ProjectFile. Bytes are copied verbatim,
// so an encrypted project's clone stays encrypted under the same user DEK.
// WAL/SHM sidecars are copied when present so no committed data is lost if the
// source had uncheckpointed pages.
func (a *App) CloneProject(path string) (ProjectFile, error) {
	clean, user, err := a.projectPathFor(path)
	if err != nil {
		return ProjectFile{}, err
	}
	projectsDir := filepath.Join(user.DataDir, "projects")
	// Recover the source's display name from either layout.
	var srcName string
	if filepath.Dir(clean) == filepath.Clean(projectsDir) {
		srcName = trimExt(filepath.Base(clean)) // legacy flat
	} else {
		srcName = projectDisplayName(filepath.Base(filepath.Dir(clean))) // subfolder
	}
	cloneName := strings.TrimSpace(srcName) + " copy"
	dest, err := newProjectPath(projectsDir, sanitizeFilename(cloneName))
	if err != nil {
		return ProjectFile{}, err
	}
	// When the source is the currently-open project, raw file copy can race
	// against a WAL checkpoint and produce a clone missing committed data.
	// Use VACUUM INTO for an atomic, fully-checkpointed snapshot instead.
	a.mu.RLock()
	isOpen := a.db != nil && samePath(a.dbPath, clean)
	openDB := a.db
	a.mu.RUnlock()

	if isOpen {
		if err := openDB.CreateSnapshot(dest); err != nil {
			return ProjectFile{}, fmt.Errorf("clone snapshot: %w", err)
		}
		if err := os.Chmod(dest, 0o600); err != nil {
			_ = os.Remove(dest)
			return ProjectFile{}, err
		}
	} else {
		if err := copyFile(clean, dest); err != nil {
			return ProjectFile{}, err
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, statErr := os.Stat(clean + suffix); statErr == nil {
				_ = copyFile(clean+suffix, dest+suffix)
			}
		}
	}
	modified := time.Now().UTC().Format(time.RFC3339)
	if info, statErr := os.Stat(dest); statErr == nil {
		modified = info.ModTime().Format(time.RFC3339)
	}
	return ProjectFile{
		Path:     dest,
		Name:     cloneName,
		Modified: modified,
	}, nil
}

// copyFile copies src to dst, creating dst with private (0600) permissions.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src) // #nosec G304 -- src is a validated project path under the user's folder.
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- dst derived from the user's projects folder.
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

// newProjectPath creates a fresh, uniquely-named subfolder for a project
// under dir and returns the path of the project database inside it. The
// folder ID is "<YYYYMMDD-HHMMSS>-<safe-name>", so every project gets a
// unique, time-stamped folder - this avoids name collisions and keeps a
// project's files grouped together. `safe` must be a non-empty sanitized
// name (the caller validates it).
func newProjectPath(dir, safe string) (string, error) {
	id := time.Now().Format("20060102-150405") + "-" + safe
	folder := filepath.Join(dir, id)
	for i := 2; ; i++ {
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			break
		}
		folder = filepath.Join(dir, fmt.Sprintf("%s-%d", id, i))
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(folder, "project.pmforge"), nil
}

// projectFolderRe matches the "<YYYYMMDD-HHMMSS>-" prefix newProjectPath puts
// on a project folder, so the display name can be recovered from it.
var projectFolderRe = regexp.MustCompile(`^\d{8}-\d{6}-`)

// projectDisplayName recovers a human-readable name from a project folder
// name by stripping the timestamp prefix.
func projectDisplayName(folder string) string {
	return projectFolderRe.ReplaceAllString(folder, "")
}

// projectEntry is one discovered project file plus lightweight metadata.
type projectEntry struct {
	Path     string
	Name     string
	Modified string
}

// enumerateProjects lists every project in projectsDir, supporting BOTH the
// current layout (each project in its own "<id>/project.pmforge" subfolder)
// and the legacy flat layout ("<name>.pmforge" directly in projectsDir), so
// projects created before the subfolder change keep working.
func enumerateProjects(projectsDir string) ([]projectEntry, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []projectEntry
	for _, e := range entries {
		if e.IsDir() {
			pf := filepath.Join(projectsDir, e.Name(), "project.pmforge")
			info, serr := os.Stat(pf)
			if serr != nil {
				continue // not a project subfolder
			}
			out = append(out, projectEntry{
				Path:     pf,
				Name:     projectDisplayName(e.Name()),
				Modified: info.ModTime().Format(time.RFC3339),
			})
			continue
		}
		if filepath.Ext(e.Name()) != ".pmforge" {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		out = append(out, projectEntry{
			Path:     filepath.Join(projectsDir, e.Name()),
			Name:     trimExt(e.Name()),
			Modified: info.ModTime().Format(time.RFC3339),
		})
	}
	return out, nil
}

// ProjectSummary is a portfolio-level snapshot of one project, produced by
// ProjectsOverview without making the project the active one.
type ProjectSummary struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Status    string `json:"status"` // planning|active|on_hold|complete|cancelled ("" if unreadable)
	Phase     string `json:"phase"`  // initiation|planning|execution|monitoring|closing
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Modified  string `json:"modified"`
	Charts    int    `json:"charts"`
	Documents int    `json:"documents"`
	Readable  bool   `json:"readable"` // false if the file could not be opened/decrypted
}

// ProjectsOverview returns a portfolio snapshot of every project in the
// signed-in user's folder: each project's status / phase / dates plus chart
// and document counts. Each project database is opened with the session DEK
// and closed again, so the app's active project is left untouched. A project
// that cannot be opened is still listed (Readable=false) so nothing silently
// disappears from the overview.
func (a *App) ProjectsOverview() ([]ProjectSummary, error) {
	user := a.requireUser()
	if user == nil {
		return nil, errors.New("not signed in")
	}
	a.mu.RLock()
	dek, err := a.requireDEKLocked()
	a.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(user.DataDir, "projects")
	entries, err := enumerateProjects(dir)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectSummary, 0, len(entries))
	for _, e := range entries {
		sum := ProjectSummary{Path: e.Path, Name: e.Name, Modified: e.Modified}
		if d, derr := db.InitEncryptedDB(e.Path, dek); derr == nil {
			if p, perr := d.GetProject(); perr == nil {
				if strings.TrimSpace(p.Name) != "" {
					sum.Name = p.Name // prefer the real (typed) project name
				}
				sum.Status = p.Status
				sum.Phase = p.Phase
				sum.StartDate = p.StartDate
				sum.EndDate = p.EndDate
				if cs, cerr := d.ListCharts(p.ID, ""); cerr == nil {
					sum.Charts = len(cs)
				}
				if ds, dderr := d.ListDocuments(p.ID, ""); dderr == nil {
					sum.Documents = len(ds)
				}
				sum.Readable = true
			}
			_ = d.Close()
		}
		out = append(out, sum)
	}
	return out, nil
}

// AppSettings holds per-user, app-level preferences that apply across all
// projects (currently the default font/theme used when creating a project).
// Stored as JSON in the user's data folder, independent of any project DB.
type AppSettings struct {
	// DefaultFont is the export font seeded into newly created projects.
	DefaultFont string `json:"default_font"`
	// DefaultTheme is the export theme: modern|classic|archival ("" => modern).
	DefaultTheme string `json:"default_theme"`
	// AppTheme is the UI theme: light|dark ("" => dark).
	AppTheme string `json:"app_theme"`
	// AutoSaveSeconds is the editor auto-save interval in seconds; 0 disables auto-save.
	AutoSaveSeconds int `json:"auto_save_seconds"`
}

// defaultAppSettings is what a brand-new user gets before they save any
// preferences: auto-save on at 60s, theme/font left to their built-in
// defaults (dark / catalog font).
func defaultAppSettings() AppSettings {
	return AppSettings{AutoSaveSeconds: 60}
}

// AppInfo is the global-settings screen payload: editable app settings plus
// read-only environment info and the available font catalog.
type AppInfo struct {
	Version      string             `json:"version"`
	DataLocation string             `json:"data_location"`
	Username     string             `json:"username"`
	Settings     AppSettings        `json:"settings"`
	Fonts        []fonts.FamilyInfo `json:"fonts"`
	LogsDir      string             `json:"logs_dir"`
}

func (a *App) appSettingsPath() (string, error) {
	user := a.requireUser()
	if user == nil {
		return "", errors.New("not signed in")
	}
	return filepath.Join(user.DataDir, "app-settings.json"), nil
}

// loadGlobalAppSettings reads the per-user app settings, returning zero values
// (no error) when the file is missing or unreadable.
func (a *App) loadGlobalAppSettings() AppSettings {
	path, err := a.appSettingsPath()
	if err != nil {
		return defaultAppSettings()
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is under the user's own data folder.
	if err != nil {
		// No settings yet: hand back the defaults (auto-save on at 60s).
		return defaultAppSettings()
	}
	var s AppSettings
	_ = json.Unmarshal(data, &s)
	return s
}

// GetAppInfo returns the global application-settings screen payload.
func (a *App) GetAppInfo() (AppInfo, error) {
	user := a.requireUser()
	if user == nil {
		return AppInfo{}, errors.New("not signed in")
	}
	return AppInfo{
		Version:      cli.Version,
		DataLocation: user.DataDir,
		Username:     user.Username,
		Settings:     a.loadGlobalAppSettings(),
		Fonts:        a.ListFonts(),
		LogsDir:      a.logDir,
	}, nil
}

// SaveAppSettings persists the per-user, app-level preferences as JSON.
func (a *App) SaveAppSettings(s AppSettings) error {
	path, err := a.appSettingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ResetAppSettings removes the signed-in user's app-level preferences so the
// next load uses the built-in defaults.
func (a *App) ResetAppSettings() (AppSettings, error) {
	path, err := a.appSettingsPath()
	if err != nil {
		return AppSettings{}, err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return AppSettings{}, err
	}
	return defaultAppSettings(), nil
}

// OpenLogsFolder opens the PMForge log directory in the system file manager
// so the user can inspect or attach log files to a bug report manually.
func (a *App) OpenLogsFolder() error {
	if a.logDir == "" {
		return errors.New("log directory not available")
	}
	return applog.OpenFolder(a.logDir)
}

// GenerateBugReport writes a self-contained diagnostic bundle to the logs
// directory and returns its path. The bundle includes environment info and
// the tail of today's log file. It never contains credentials or key material.
func (a *App) GenerateBugReport() (string, error) {
	if a.logDir == "" {
		return "", errors.New("log directory not available")
	}
	if err := os.MkdirAll(a.logDir, 0o700); err != nil {
		return "", fmt.Errorf("ensure log dir: %w", err)
	}
	ts := time.Now().UTC()
	reportPath := filepath.Join(a.logDir, fmt.Sprintf("bug-report-%s.txt", ts.Format("20060102-150405")))

	var buf strings.Builder
	fmt.Fprintf(&buf, "PMForge Diagnostic Report\n")
	fmt.Fprintf(&buf, "Generated: %s\n\n", ts.Format(time.RFC3339Nano))
	fmt.Fprintf(&buf, "=== Environment ===\n")
	fmt.Fprintf(&buf, "PMForge version: %s\n", cli.Version)
	fmt.Fprintf(&buf, "OS:              %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&buf, "Go runtime:      %s\n", runtime.Version())
	fmt.Fprintf(&buf, "PID:             %d\n", os.Getpid())
	fmt.Fprintf(&buf, "Log directory:   %s\n", a.logDir)
	if a.logPath != "" {
		fmt.Fprintf(&buf, "Log file:        %s\n", a.logPath)
	}
	if user := a.requireUser(); user != nil {
		fmt.Fprintf(&buf, "Data directory:  %s\n", user.DataDir)
	}
	fmt.Fprintf(&buf, "\n=== Recent Log (last 200 lines) ===\n")
	if a.logPath != "" {
		tail, err := logTail(a.logPath, 200)
		if err != nil {
			fmt.Fprintf(&buf, "(could not read log: %v)\n", err)
		} else {
			buf.WriteString(tail)
		}
	} else {
		fmt.Fprintf(&buf, "(no log file — logging fell back to stderr at startup)\n")
	}

	if err := os.WriteFile(reportPath, []byte(buf.String()), 0o600); err != nil { // #nosec G306 -- 0o600: report is private to the user.
		return "", fmt.Errorf("write bug report: %w", err)
	}
	log.Printf("bug report written to: %s", reportPath)
	return reportPath, nil
}

// logTail returns up to maxLines lines from the end of the file at path.
func logTail(path string, maxLines int) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the PMForge log file, resolved at startup.
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}

// applyGlobalDefaults seeds a freshly created project's settings with the
// user's app-level default font/theme. Best-effort: any failure is ignored so
// project creation never fails because of a preference.
func (a *App) applyGlobalDefaults(d *db.Database) {
	g := a.loadGlobalAppSettings()
	if g.DefaultFont == "" && g.DefaultTheme == "" {
		return
	}
	s, err := d.GetSettings()
	if err != nil {
		return
	}
	if g.DefaultFont != "" {
		s.DefaultFont = g.DefaultFont
	}
	if g.DefaultTheme != "" {
		s.ExportTheme = g.DefaultTheme
	}
	_ = d.SaveSettings(s)
}

// OpenProject loads a .pmforge file as the current project.
func (a *App) OpenProject(path string) (db.Project, error) {
	// Confine to the signed-in user's own projects folder before doing any
	// work (and before taking the write lock, since projectPathFor read-locks
	// a.mu via requireUser). This is the same boundary DeleteProject/
	// CloneProject enforce; opening is no less sensitive — the path also feeds
	// the SQLCipher DSN.
	clean, _, err := a.projectPathFor(path)
	if err != nil {
		return db.Project{}, err
	}

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
	d, err := db.InitEncryptedDB(clean, dek)
	if err != nil {
		if encrypted, encErr := db.IsEncryptedFile(clean); encErr == nil && !encrypted {
			return db.Project{}, ErrProjectRequiresEncryptionMigration
		}
		return db.Project{}, err
	}
	proj, projErr := d.GetProject()
	if projErr != nil {
		_ = d.Close()
		return db.Project{}, projErr
	}
	if err := verifyProjectAuditForOpen(d, proj); err != nil {
		_ = d.Close()
		return db.Project{}, err
	}
	// Apply the project's saved document font (no-op if unset). Done
	// while we still hold the lock since it only reads d + the user's
	// font dir; configureFonts must not re-acquire a.mu.
	a.configureFontsLocked(d)
	a.db = d
	a.dbPath = clean
	a.adminSvc = admin.NewService(d)
	a.sigmaSvc = service.NewProjectService(d)
	return proj, nil
}

func verifyProjectAuditForOpen(d *db.Database, project db.Project) error {
	settings, err := d.GetSettings()
	if err != nil {
		return fmt.Errorf("load compliance settings: %w", err)
	}
	if !settings.ComplianceMode {
		return nil
	}
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		return fmt.Errorf("audit verification failed: %w", err)
	}
	if report.Valid {
		return nil
	}
	return fmt.Errorf(
		"audit verification failed at sequence %d for event %s: %s",
		report.FirstInvalidSequence,
		report.FirstInvalidEventID,
		report.FirstInvalidReason,
	)
}

// IsProjectEncrypted reports whether a .pmforge file is already
// SQLCipher-encrypted. Used by the Settings migration flow before
// presenting the opt-in action.
func (a *App) IsProjectEncrypted(path string) (bool, error) {
	clean, _, err := a.projectPathFor(path)
	if err != nil {
		return false, err
	}
	return db.IsEncryptedFile(clean)
}

// EncryptProjectAtRest migrates a legacy plaintext .pmforge file to
// SQLCipher with the active user's session DEK. Active recovery codes
// must already carry DEK wraps; otherwise a future recovery reset
// would orphan encrypted projects.
func (a *App) EncryptProjectAtRest(path string) (string, error) {
	// Confine to the user's own projects folder before any filesystem work:
	// MigratePlaintextToEncrypted renames the source to a .bak, writes a new
	// file at this path, and chmods it, so an unconfined path would be a
	// rename/overwrite primitive outside the user's sandbox.
	clean, user, err := a.projectPathFor(path)
	if err != nil {
		return "", err
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
	if a.db != nil && samePath(a.dbPath, clean) {
		_ = a.db.Close()
		a.db = nil
		a.dbPath = ""
		a.adminSvc = nil
		a.sigmaSvc = nil
	}
	a.mu.Unlock()

	return db.MigratePlaintextToEncrypted(clean, dek)
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
	if p.TimeZone == "" {
		p.TimeZone = calendar.DefaultTimeZone(p.CountryCode)
	}
	if !calendar.ValidTimeZone(p.CountryCode, p.TimeZone) {
		return db.Project{}, fmt.Errorf("time zone %q is not supported by the %s business-calendar policy", p.TimeZone, p.CountryCode)
	}
	return d.UpsertProject(p)
}
