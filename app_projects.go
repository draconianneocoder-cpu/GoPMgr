// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gopmgr/internal/admin"
	"gopmgr/internal/applog"
	"gopmgr/internal/calendar"
	"gopmgr/internal/cli"
	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/fonts"
	"gopmgr/internal/sigma/service"
	"gopmgr/internal/update"
	"gopmgr/internal/users"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// ListProjects returns every project file (.gopmgr, or legacy .pmforge) under the current user's
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

// CreateProject creates a new .gopmgr project file under the user's
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

// RestoreProjectArchive imports a validated .pmba archive as a new project.
// It never overwrites an existing project and never imports bundled
// certificates automatically.
func (a *App) RestoreProjectArchive() (ProjectFile, error) {
	user := a.requireUser()
	if user == nil {
		return ProjectFile{}, errors.New("not signed in")
	}
	if a.ctx == nil {
		return ProjectFile{}, errors.New("no context (Wails not started)")
	}
	archivePath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Restore GoPMgr project backup",
		DefaultDirectory: user.DataDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "GoPMgr backup (*.pmba)", Pattern: "*.pmba"},
		},
	})
	if err != nil {
		return ProjectFile{}, err
	}
	if archivePath == "" {
		return ProjectFile{}, errors.New("restore cancelled")
	}
	return a.restoreProjectArchiveFrom(archivePath, user)
}

func (a *App) restoreProjectArchiveFrom(archivePath string, user *users.Account) (result ProjectFile, err error) {
	projectsDir := filepath.Join(user.DataDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		return result, err
	}
	destPath, err := newProjectPath(projectsDir, "restored-project")
	if err != nil {
		return result, err
	}
	destDir := filepath.Dir(destPath)
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(destDir)
		}
	}()
	manifest, err := db.RestoreArchivalBundle(archivePath, destPath)
	if err != nil {
		return result, err
	}

	a.mu.RLock()
	dek, dekErr := a.requireDEKLocked()
	a.mu.RUnlock()
	if dekErr != nil {
		return result, dekErr
	}
	var restored *db.Database
	if encrypted, inspectErr := db.IsEncryptedFile(destPath); inspectErr != nil {
		return result, inspectErr
	} else if encrypted {
		restored, err = db.InitEncryptedDB(destPath, dek)
	} else {
		restored, err = db.InitDB(destPath)
	}
	if err != nil {
		return result, fmt.Errorf("open restored project for this account: %w", err)
	}
	defer func() {
		if closeErr := restored.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if ok, integrityErr := restored.CheckIntegrity(); integrityErr != nil || !ok {
		if integrityErr != nil {
			return result, fmt.Errorf("check restored project integrity: %w", integrityErr)
		}
		return result, errors.New("restored project failed its database integrity check")
	}
	project, err := restored.GetProject()
	if err != nil {
		return result, fmt.Errorf("read restored project metadata: %w", err)
	}
	if manifest.DatabaseID != "" && manifest.DatabaseID != project.ID {
		return result, errors.New("backup manifest database ID does not match the restored project")
	}
	project.Name = strings.TrimSpace(project.Name) + " (restored)"
	project, err = restored.UpsertProject(project)
	if err != nil {
		return result, fmt.Errorf("rename restored project: %w", err)
	}
	published = true
	return ProjectFile{
		Path:     destPath,
		Name:     project.Name,
		Modified: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// projectFileExtension is the current project-file extension. Renamed
// 2026-08-04 from ".pmforge" to ".gopmgr"; newProjectPath below only ever
// writes this extension now, but isProjectExtension (and therefore every
// reader of existing files: projectPathFor, enumerateProjects) still
// accepts the old ".pmforge" extension too, because project files already
// exist on disk under it and nothing migrates them.
const projectFileExtension = ".gopmgr"

// legacyProjectFileExtension is the pre-2026-08-04 project-file extension.
// Read-only: still recognised so existing projects keep opening, but never
// written by this build.
const legacyProjectFileExtension = ".pmforge"

// isProjectExtension reports whether ext (as returned by filepath.Ext) is a
// project-file extension this build recognises for reading — current or
// legacy. It intentionally does NOT distinguish which one for callers that
// only need "is this a project file at all" (projectPathFor's confinement
// check, enumerateProjects' directory scan); callers that need to tell them
// apart use ext directly.
func isProjectExtension(ext string) bool {
	return ext == projectFileExtension || ext == legacyProjectFileExtension
}

// projectPathFor validates that path points at a project file (current
// .gopmgr or legacy .pmforge extension) inside the signed-in user's own
// projects directory and returns the cleaned path plus the account. It
// rejects anything outside that directory so DeleteProject and CloneProject
// can never touch arbitrary files on disk. Recognising both extensions
// widens WHICH files pass the extension check; it does not touch the
// containment check below, which is what actually stops path traversal.
func (a *App) projectPathFor(path string) (string, *users.Account, error) {
	user := a.requireUser()
	if user == nil {
		return "", nil, errors.New("not signed in")
	}
	clean := filepath.Clean(path)
	if !isProjectExtension(filepath.Ext(clean)) {
		return "", nil, errors.New("not a project file")
	}
	projectsDir := filepath.Clean(filepath.Join(user.DataDir, "projects"))
	parent := filepath.Dir(clean)
	// Allowed: legacy flat layout (<projects>/<name>.<ext>) where parent is
	// the projects dir, OR the current layout (<projects>/<id>/project.<ext>)
	// where the parent is an immediate subfolder of the projects dir. Anything
	// deeper or outside is rejected.
	if parent != projectsDir && filepath.Dir(parent) != projectsDir {
		return "", nil, errors.New("project is outside your projects folder")
	}
	return clean, user, nil
}

// DeleteProject permanently removes a project file (.gopmgr, or legacy .pmforge) and its
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
// folder ID is "<YYYYMMDD-HHMMSS>-<safe-name>". Each candidate is reserved
// atomically, so concurrent callers receive distinct folders. `safe` must be
// a non-empty sanitized name (the caller validates it).
func newProjectPath(dir, safe string) (string, error) {
	return reserveProjectFolder(dir, time.Now().Format("20060102-150405")+"-"+safe)
}

// reserveProjectFolder atomically creates one project folder under dir. The
// caller supplies an already-sanitized ID so tests can exercise collisions
// without depending on the wall clock.
func reserveProjectFolder(dir, id string) (string, error) {
	return reserveProjectFolderWithMkdir(dir, id, os.Mkdir)
}

// reserveProjectFolderWithMkdir keeps the collision policy testable without
// relying on platform-specific permission failures.
func reserveProjectFolderWithMkdir(dir, id string, mkdir func(string, fs.FileMode) error) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create projects directory: %w", err)
	}
	for i := 1; ; i++ {
		name := id
		if i > 1 {
			name = fmt.Sprintf("%s-%d", id, i)
		}
		folder := filepath.Join(dir, name)
		if err := mkdir(folder, 0o700); err == nil {
			return filepath.Join(folder, "project"+projectFileExtension), nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("create project directory: %w", err)
		}
	}
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

// isProjectFileName reports whether name is exactly a subfolder-layout
// project file — "project.gopmgr" or its pre-2026-08-04 name
// "project.pmforge" — as opposed to isProjectExtension, which only checks
// the suffix and is used for the flat-layout case where the file is named
// after the project itself.
func isProjectFileName(name string) bool {
	return name == "project"+projectFileExtension || name == "project"+legacyProjectFileExtension
}

// maxProjectDiscoveryDepth bounds how many directory levels below
// projectsDir enumerateProjects will descend while looking for a
// subfolder-layout project file. 3 covers the existing "<id>/project.<ext>"
// layout (depth 2) plus one extra level of wrapping (depth 3) — e.g. a
// project folder a user dragged into a manually-created subfolder, or one
// restored a level deeper by a backup/archive tool — without turning
// discovery into an unbounded walk of the user's projects/ directory.
const maxProjectDiscoveryDepth = 3

// enumerateProjects lists every project under projectsDir, supporting BOTH
// the current layout (each project in its own "<id>/project.<ext>"
// subfolder) and the legacy flat layout ("<name>.<ext>" directly in
// projectsDir), so projects created before the subfolder change keep
// working — and, as of 2026-08-04, both the current ".gopmgr" and legacy
// ".pmforge" extension within each layout, so projects created before that
// rename keep working too.
//
// The subfolder layout is discovered up to maxProjectDiscoveryDepth levels
// deep (not just directly under projectsDir), so a project nested inside an
// extra wrapping folder is still found instead of silently disappearing
// from the picker. The walk never follows symlinks — a symlinked directory
// is reported by the underlying directory read as a non-directory entry and
// is skipped without being descended into or listed, and a symlinked file
// is skipped explicitly below — because a symlink inside a user's own
// projects/ folder can point anywhere on disk, including another account's
// data, and following or listing through one would weaken the per-user
// isolation boundary this function exists inside of (callers only ever pass
// projectsDir = user.DataDir + "/projects" for the signed-in user).
func enumerateProjects(projectsDir string) ([]projectEntry, error) {
	if _, err := os.Lstat(projectsDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []projectEntry
	walkErr := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// One unreadable entry (permission error, concurrent delete
			// mid-walk) must not hide every other project; skip just this
			// subtree/file rather than aborting the whole listing.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == projectsDir {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		rel, relErr := filepath.Rel(projectsDir, path)
		if relErr != nil {
			return nil // unreachable: WalkDir only visits paths under projectsDir
		}
		depth := strings.Count(rel, string(filepath.Separator)) + 1

		if d.IsDir() {
			if depth >= maxProjectDiscoveryDepth {
				return fs.SkipDir
			}
			return nil
		}

		if depth == 1 {
			// Flat-layout project file directly in projectsDir.
			if !isProjectExtension(filepath.Ext(d.Name())) {
				return nil
			}
			info, ierr := d.Info()
			if ierr != nil {
				return nil
			}
			out = append(out, projectEntry{
				Path:     path,
				Name:     trimExt(d.Name()),
				Modified: info.ModTime().Format(time.RFC3339),
			})
			return nil
		}

		// Subfolder-layout project file, one or two levels deep. The
		// display name comes from the immediate parent folder regardless
		// of how many wrapping folders sit above it. If a subfolder somehow
		// contains BOTH "project.gopmgr" and "project.pmforge" — never
		// produced by this app, which only ever writes one extension per
		// project and never both — both are listed as separate entries
		// rather than deduped to one, unlike the old single-ReadDir version.
		// Accepted: reaching that state requires manual disk tampering, not
		// normal use.
		if isProjectFileName(d.Name()) {
			info, ierr := d.Info()
			if ierr != nil {
				return nil
			}
			out = append(out, projectEntry{
				Path:     path,
				Name:     projectDisplayName(filepath.Base(filepath.Dir(path))),
				Modified: info.ModTime().Format(time.RFC3339),
			})
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
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
	// AutoCheckUpdates opts into a single, non-blocking update.CheckLatest
	// call on sign-in (App.svelte's session-start effect), rather than
	// only the on-demand "Check for updates" button. Defaults to false
	// (Go zero value): this app's established posture is opt-in for
	// anything that reaches the network (signing, timestamping,
	// compliance mode), and a `true` default here would also disagree
	// with an older settings file written before this field existed,
	// which unmarshals the missing key as false regardless of what a
	// fresh default might claim. Checking never installs anything by
	// itself — DownloadAndInstallUpdate is always a separate, explicit
	// user action.
	AutoCheckUpdates bool `json:"auto_check_updates"`
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
		Version:      update.CurrentVersion,
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

// OpenLogsFolder opens the GoPMgr log directory in the system file manager
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
	fmt.Fprintf(&buf, "GoPMgr Diagnostic Report\n")
	fmt.Fprintf(&buf, "Generated: %s\n\n", ts.Format(time.RFC3339Nano))
	fmt.Fprintf(&buf, "=== Environment ===\n")
	fmt.Fprintf(&buf, "GoPMgr version: %s\n", cli.Version)
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
	data, err := os.ReadFile(path) // #nosec G304 -- path is the GoPMgr log file, resolved at startup.
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

// OpenProject loads a project file (.gopmgr, or legacy .pmforge) as the current project.
func (a *App) OpenProject(path string) (ProjectMetaWire, error) {
	// Confine to the signed-in user's own projects folder before doing any
	// work (and before taking the write lock, since projectPathFor read-locks
	// a.mu via requireUser). This is the same boundary DeleteProject/
	// CloneProject enforce; opening is no less sensitive — the path also feeds
	// the SQLCipher DSN.
	clean, _, err := a.projectPathFor(path)
	if err != nil {
		return ProjectMetaWire{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	dek, err := a.requireDEKLocked()
	if err != nil {
		return ProjectMetaWire{}, err
	}
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	d, err := db.InitEncryptedDB(clean, dek)
	if err != nil {
		if encrypted, encErr := db.IsEncryptedFile(clean); encErr == nil && !encrypted {
			return ProjectMetaWire{}, ErrProjectRequiresEncryptionMigration
		}
		return ProjectMetaWire{}, err
	}
	proj, projErr := d.GetProject()
	if projErr != nil {
		_ = d.Close()
		return ProjectMetaWire{}, projErr
	}
	if err := verifyProjectAuditForOpen(d, proj); err != nil {
		_ = d.Close()
		return ProjectMetaWire{}, err
	}
	// Apply the project's saved document font (no-op if unset). Done
	// while we still hold the lock since it only reads d + the user's
	// font dir; configureFonts must not re-acquire a.mu.
	a.configureFontsLocked(d)
	a.db = d
	a.dbPath = clean
	a.adminSvc = admin.NewService(d)
	a.sigmaSvc = service.NewProjectService(d)
	return projectMetaWire(proj), nil
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

// IsProjectEncrypted reports whether a project file is already
// SQLCipher-encrypted. Used by the Settings migration flow before
// presenting the opt-in action.
func (a *App) IsProjectEncrypted(path string) (bool, error) {
	clean, _, err := a.projectPathFor(path)
	if err != nil {
		return false, err
	}
	return db.IsEncryptedFile(clean)
}

// EncryptProjectAtRest migrates a legacy plaintext project file to
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

// CloseProject closes the currently-open project file.
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
func (a *App) GetProjectMeta() (ProjectMetaWire, error) {
	d := a.requireDB()
	if d == nil {
		return ProjectMetaWire{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return ProjectMetaWire{}, err
	}
	return projectMetaWire(p), nil
}

// UpdateProjectMeta upserts the project metadata.
func (a *App) UpdateProjectMeta(wire ProjectMetaWire) (ProjectMetaWire, error) {
	d := a.requireDB()
	if d == nil {
		return ProjectMetaWire{}, errors.New("no project open")
	}
	p, err := projectFromMetaWire(wire)
	if err != nil {
		return ProjectMetaWire{}, err
	}
	if p.TimeZone == "" {
		p.TimeZone = calendar.DefaultTimeZone(p.CountryCode)
	}
	if !calendar.ValidTimeZone(p.CountryCode, p.TimeZone) {
		return ProjectMetaWire{}, fmt.Errorf("time zone %q is not supported by the %s business-calendar policy", p.TimeZone, p.CountryCode)
	}
	saved, err := d.UpsertProject(p)
	if err != nil {
		return ProjectMetaWire{}, err
	}
	return projectMetaWire(saved), nil
}
