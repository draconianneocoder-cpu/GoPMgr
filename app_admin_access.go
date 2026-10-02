// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopmgr/internal/db"
	"gopmgr/internal/users"
)

// =========================================================
// Recorded administrator access (ADR-004 phase 2)
// =========================================================
//
// An administrator opens another account's data for viewing only. The
// target's key lives in its own handle, never in a.dek, and their projects
// are read from a copy held in its own database handle, never in a.db: no
// method that works on the administrator's own project (exports, reports,
// archives, attachments, edits) can reach the target's data. The copy sits
// in the target's folder under an unpredictable name and is removed, and
// the key zeroed, when access stops, at sign-out or sign-in, and at
// shutdown.

// adminViewDirPrefix starts the name of each folder holding a copy of a
// project an administrator is viewing.
const adminViewDirPrefix = "admin-view-"

// staleAdminViewCopyAge is how old a copy folder must be before opening
// access removes it as left over by a crash. Younger ones may be open in
// another GoPMgr process sharing the data folder.
const staleAdminViewCopyAge = 24 * time.Hour

// adminAccess is an administrator's open access to one account. Guarded by
// App.mu.
type adminAccess struct {
	target  string
	dataDir string
	dek     []byte
	view    *db.Database // the project copy being viewed, or nil
	copyDir string       // the folder holding that copy
}

// AdminOpenUserData records the signed-in administrator's access to
// username's data, with reason, and opens it for viewing. Any access
// already open is stopped first. Nothing is recorded when it is refused.
func (a *App) AdminOpenUserData(username, reason string) error {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return errors.New("administrator privileges required")
	}
	var targetDEK []byte
	err := a.withSessionDEK(func(dek []byte) error {
		var err error
		targetDEK, err = a.store.OpenUserForAdmin(caller.Username, dek, username, reason)
		return err
	})
	if err != nil {
		return adminAccessError(username, err)
	}
	dataDir, err := a.accountDataDir(username)
	if err != nil {
		zeroBytes(targetDEK)
		return err
	}
	a.mu.Lock()
	a.stopAdminAccessLocked()
	a.mu.Unlock()
	removeAdminViewCopies(dataDir, time.Now().Add(-staleAdminViewCopyAge))

	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopAdminAccessLocked()
	a.access = &adminAccess{target: username, dataDir: dataDir, dek: targetDEK}
	return nil
}

// AdminListUserProjects lists the projects of the account the
// administrator has open.
func (a *App) AdminListUserProjects() ([]ProjectFile, error) {
	dataDir, _, err := a.openAccess()
	if err != nil {
		return nil, err
	}
	entries, err := enumerateProjects(filepath.Join(dataDir, "projects"))
	if err != nil {
		return nil, err
	}
	out := make([]ProjectFile, 0, len(entries))
	for _, e := range entries {
		out = append(out, ProjectFile(e))
	}
	return out, nil
}

// AdminViewUserProject opens a copy of one of the target's projects for
// viewing, replacing any project already being viewed. path is checked
// against the target's projects folder as projectPathFor checks the
// administrator's own.
func (a *App) AdminViewUserProject(path string) (ProjectMetaWire, error) {
	dataDir, dek, err := a.openAccess()
	if err != nil {
		return ProjectMetaWire{}, err
	}
	defer zeroBytes(dek)
	clean, err := confineProjectPath(filepath.Join(dataDir, "projects"), path)
	if err != nil {
		return ProjectMetaWire{}, err
	}

	copyDir, err := os.MkdirTemp(dataDir, adminViewDirPrefix)
	if err != nil {
		return ProjectMetaWire{}, fmt.Errorf("make a folder for the copy: %w", err)
	}
	view, meta, err := openProjectCopy(clean, filepath.Join(copyDir, filepath.Base(clean)), dek)
	if err != nil {
		_ = os.RemoveAll(copyDir)
		return ProjectMetaWire{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.access == nil || a.access.dataDir != dataDir {
		// Access stopped or moved to another account meanwhile.
		_ = view.Close()
		_ = os.RemoveAll(copyDir)
		return ProjectMetaWire{}, errors.New("administrator access has stopped")
	}
	a.closeAdminViewLocked()
	a.access.view = view
	a.access.copyDir = copyDir
	return meta, nil
}

// AdminStopUserData stops the administrator's access: the project copy is
// closed and removed and the key zeroed.
func (a *App) AdminStopUserData() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopAdminAccessLocked()
	return nil
}

// openAccess re-checks in system.db that the signed-in user is still an
// enabled administrator and returns the open access's folder and a copy of
// its key, which the caller must zero.
func (a *App) openAccess() (dataDir string, dek []byte, err error) {
	caller := a.requireUser()
	if caller == nil || !caller.IsAdmin {
		return "", nil, errors.New("administrator privileges required")
	}
	if err := a.store.RequireEnabledAdmin(caller.Username); err != nil {
		a.mu.Lock()
		a.stopAdminAccessLocked()
		a.mu.Unlock()
		return "", nil, errors.New("administrator privileges required")
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.access == nil {
		return "", nil, errors.New("no account is open for administrator access")
	}
	return a.access.dataDir, append([]byte(nil), a.access.dek...), nil
}

// openProjectCopy copies the project at src (with any -wal and -shm files)
// to dst and opens the copy read-only with dek. The owner's file is only
// read.
func openProjectCopy(src, dst string, dek []byte) (*db.Database, ProjectMetaWire, error) {
	if err := copyFile(src, dst); err != nil {
		return nil, ProjectMetaWire{}, fmt.Errorf("copy the project: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Lstat(src + suffix)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, ProjectMetaWire{}, errors.New("the project's files could not be copied")
		}
		if err := copyFile(src+suffix, dst+suffix); err != nil {
			return nil, ProjectMetaWire{}, fmt.Errorf("copy the project: %w", err)
		}
	}
	view, err := db.OpenEncryptedCopyReadOnly(dst, dek)
	if errors.Is(err, db.ErrCopyDamaged) {
		return nil, ProjectMetaWire{}, errors.New("the copy of this project could not be read cleanly, perhaps because its owner was changing it; try again")
	}
	if err != nil {
		return nil, ProjectMetaWire{}, fmt.Errorf("open the project copy: %w", err)
	}
	proj, err := view.GetProject()
	if err != nil {
		_ = view.Close()
		return nil, ProjectMetaWire{}, err
	}
	return view, projectMetaWire(proj), nil
}

// stopAdminAccessLocked ends any administrator access. Must hold a.mu.
func (a *App) stopAdminAccessLocked() {
	if a.access == nil {
		return
	}
	a.closeAdminViewLocked()
	zeroBytes(a.access.dek)
	a.access = nil
}

// closeAdminViewLocked closes and removes the project copy being viewed.
// Must hold a.mu.
func (a *App) closeAdminViewLocked() {
	if a.access == nil || a.access.view == nil {
		return
	}
	_ = a.access.view.Close()
	if err := os.RemoveAll(a.access.copyDir); err != nil {
		log.Printf("remove administrator view copy %s: %v", a.access.copyDir, err)
	}
	a.access.view = nil
	a.access.copyDir = ""
}

// removeAdminViewCopies removes copy folders in dataDir last changed
// before cutoff: left by an access that ended without cleanup (a crash, a
// forced quit). Newer ones are kept, since another GoPMgr process may be
// viewing them.
func removeAdminViewCopies(dataDir string, cutoff time.Time) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), adminViewDirPrefix) || e.Type() != fs.ModeDir {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dataDir, e.Name())); err != nil {
			log.Printf("remove leftover administrator view copy: %v", err)
		}
	}
}

// accountDataDir returns username's data folder.
func (a *App) accountDataDir(username string) (string, error) {
	accounts, err := a.store.List()
	if err != nil {
		return "", err
	}
	for _, acc := range accounts {
		if acc.Username == username {
			return acc.DataDir, nil
		}
	}
	return "", errors.New("no such account")
}

// adminAccessError words a refused access for the Admin panel.
func adminAccessError(username string, err error) error {
	switch {
	case errors.Is(err, users.ErrAccessReasonRequired):
		return errors.New("enter a reason for opening this account's data")
	case errors.Is(err, users.ErrAccessReasonTooLong):
		return fmt.Errorf("keep the reason to %d characters or fewer", users.MaxAccessReasonLength)
	case errors.Is(err, users.ErrSelfAccess):
		return errors.New("you open your own data by signing in to your own account")
	case errors.Is(err, users.ErrNotAdmin):
		return errors.New("administrator privileges required")
	case errors.Is(err, users.ErrNoSuchUser):
		return errors.New("no such account")
	case errors.Is(err, users.ErrNotEnrolled):
		return fmt.Errorf("%s has not signed in since administrator access began, so their data can't be opened yet", username)
	case errors.Is(err, users.ErrNoEscrowGrant):
		return errors.New("you don't hold the administrator key yet; another administrator can give it to you")
	case errors.Is(err, users.ErrEscrowMismatch):
		return errors.New("a key check failed, so the data was not opened; the account history records it")
	}
	return err
}
