// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/db"
)

// adminAccessApp returns an app signed in as alice, the first
// administrator, and the path of a project bob created.
func adminAccessApp(t *testing.T) (*App, string) {
	t.Helper()
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", escrowTestPassword, false); err != nil {
		t.Fatalf("CreateAccount(bob): %v", err)
	}
	switchUser(t, app, "bob")
	if err := app.AcceptEncryptionWithoutRecoveryCodes(); err != nil {
		t.Fatalf("AcceptEncryptionWithoutRecoveryCodes: %v", err)
	}
	project, err := app.CreateProject("Plan", "bob's plan")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	switchUser(t, app, "alice")
	return app, project.Path
}

func fileDigest(t *testing.T, path string) ([32]byte, os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return sha256.Sum256(data), info.Mode()
}

func sessionDEK(app *App) []byte {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return append([]byte(nil), app.dek...)
}

func TestAdminAccessViewsACopyAndLeavesTheOwnersFileAlone(t *testing.T) {
	app, path := adminAccessApp(t)
	beforeDigest, beforeMode := fileDigest(t, path)
	aliceDEK := sessionDEK(app)

	if err := app.AdminOpenUserData("bob", "support ticket 42"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	projects, err := app.AdminListUserProjects()
	if err != nil || len(projects) != 1 || projects[0].Path != path {
		t.Fatalf("AdminListUserProjects = %+v, %v; want bob's one project", projects, err)
	}
	meta, err := app.AdminViewUserProject(path)
	if err != nil {
		t.Fatalf("AdminViewUserProject: %v", err)
	}
	if meta.Project.Name != "Plan" {
		t.Fatalf("viewed project name = %q, want Plan", meta.Project.Name)
	}

	app.mu.RLock()
	access, ownDB := app.access, app.db
	app.mu.RUnlock()
	if ownDB != nil {
		t.Fatal("the viewed copy was put in a.db, where every project method can reach it")
	}
	if !bytes.Equal(sessionDEK(app), aliceDEK) {
		t.Fatal("the session key changed during administrator access")
	}
	if !strings.HasPrefix(filepath.Base(access.copyDir), adminViewDirPrefix) || filepath.Dir(access.copyDir) == filepath.Dir(filepath.Dir(path)) {
		t.Fatalf("copy folder %q, want an %s folder outside bob's projects", access.copyDir, adminViewDirPrefix)
	}
	if _, err := access.view.Conn.Exec(`UPDATE project SET name = 'changed'`); err == nil || !strings.Contains(err.Error(), "readonly") {
		t.Fatalf("writing to the viewed copy: err = %v, want SQLite's read-only refusal", err)
	}

	if err := app.AdminStopUserData(); err != nil {
		t.Fatalf("AdminStopUserData: %v", err)
	}
	if _, err := os.Stat(access.copyDir); !os.IsNotExist(err) {
		t.Fatalf("the copy folder is still there after stopping (%v)", err)
	}
	if afterDigest, afterMode := fileDigest(t, path); afterDigest != beforeDigest || afterMode != beforeMode {
		t.Fatal("bob's project file changed during administrator access")
	}

	events, err := app.store.AccountEvents()
	if err != nil {
		t.Fatalf("AccountEvents: %v", err)
	}
	if events[0].Action != "admin_access" || events[0].Actor != "alice" || events[0].Username != "bob" || events[0].Detail != "support ticket 42" {
		t.Fatalf("latest event = %+v; want alice's recorded access of bob with the reason", events[0])
	}
}

// The target's key is zeroed and the copy removed whichever way access ends.
func TestAdminAccessIsClearedOnEveryExit(t *testing.T) {
	for name, exit := range map[string]func(t *testing.T, app *App){
		"stop":     func(t *testing.T, app *App) { _ = app.AdminStopUserData() },
		"sign-out": func(t *testing.T, app *App) { _ = app.Logout() },
		"sign-in": func(t *testing.T, app *App) {
			if _, err := app.Login("alice", escrowTestPassword); err != nil {
				t.Fatalf("Login: %v", err)
			}
		},
		"shutdown": func(t *testing.T, app *App) { app.shutdown(t.Context()) },
	} {
		t.Run(name, func(t *testing.T) {
			app, path := adminAccessApp(t)
			if err := app.AdminOpenUserData("bob", "checking"); err != nil {
				t.Fatalf("AdminOpenUserData: %v", err)
			}
			if _, err := app.AdminViewUserProject(path); err != nil {
				t.Fatalf("AdminViewUserProject: %v", err)
			}
			app.mu.RLock()
			key, copyDir := app.access.dek, app.access.copyDir
			app.mu.RUnlock()

			exit(t, app)

			app.mu.RLock()
			access := app.access
			app.mu.RUnlock()
			if access != nil {
				t.Fatal("administrator access is still open")
			}
			if !bytes.Equal(key, make([]byte, len(key))) {
				t.Fatal("the target's key was not zeroed")
			}
			if _, err := os.Stat(copyDir); !os.IsNotExist(err) {
				t.Fatalf("the copy folder is still there (%v)", err)
			}
		})
	}
}

func TestAdminViewIsConfinedToTheTargetsProjects(t *testing.T) {
	app, bobPath := adminAccessApp(t)
	if err := app.AcceptEncryptionWithoutRecoveryCodes(); err != nil {
		t.Fatalf("AcceptEncryptionWithoutRecoveryCodes: %v", err)
	}
	own, err := app.CreateProject("Mine", "")
	if err != nil {
		t.Fatalf("CreateProject(alice): %v", err)
	}
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	bobProjects := filepath.Dir(filepath.Dir(bobPath))
	link := filepath.Join(bobProjects, "linked", "project.gopmgr")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(own.Path, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Copies of bob's own project, which his key opens, in places outside
	// what projectPathFor allows: only confinement refuses these.
	bobDir := filepath.Dir(bobProjects)
	outside := filepath.Join(bobDir, "elsewhere", "project.gopmgr")
	tooDeep := filepath.Join(filepath.Dir(bobPath), "nested", "project.gopmgr")
	bobLink := filepath.Join(bobProjects, "bob-linked", "project.gopmgr")
	for _, dst := range []string{outside, tooDeep} {
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := copyFile(bobPath, dst); err != nil {
			t.Fatalf("copy bob's project: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(bobLink), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(outside, bobLink); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := app.AdminViewUserProject(bobPath); err != nil {
		t.Fatalf("viewing bob's real project: %v", err)
	}

	for name, path := range map[string]string{
		"bob's project outside projects/":  outside,
		"bob's project nested too deep":    tooDeep,
		"a symlink to bob's project":       bobLink,
		"the administrator's own project":  own.Path,
		"a symlink to another project":     link,
		"a path climbing out of projects/": filepath.Join(bobProjects, "..", "..", filepath.Base(filepath.Dir(own.Path)), "project.gopmgr"),
		"a file that is not a project":     filepath.Join(filepath.Dir(bobPath), "notes.txt"),
	} {
		if _, err := app.AdminViewUserProject(path); err == nil {
			t.Errorf("%s: viewed, want refused", name)
		}
	}
}

func TestAdminAccessStopsWhenTheAdministratorIsDemoted(t *testing.T) {
	app, path := adminAccessApp(t)
	if _, err := app.CreateAccount("carol", "Carol", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(carol): %v", err)
	}
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	app.mu.RLock()
	key := app.access.dek
	app.mu.RUnlock()
	// Another GoPMgr process demotes alice.
	if err := app.store.SetAdmin("carol", "alice", false); err != nil {
		t.Fatalf("demote alice: %v", err)
	}

	if _, err := app.AdminViewUserProject(path); err == nil || err.Error() != "administrator privileges required" {
		t.Fatalf("viewing after demotion: err = %v, want administrator privileges required", err)
	}
	if !bytes.Equal(key, make([]byte, len(key))) {
		t.Fatal("the target's key was kept after demotion")
	}
}

// Copy folders older than a day are left over from a crash and removed;
// newer ones may be open in another GoPMgr process and are kept.
func TestAdminAccessRemovesOnlyStaleLeftoverCopies(t *testing.T) {
	app, path := adminAccessApp(t)
	bobDir := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	stale := filepath.Join(bobDir, adminViewDirPrefix+"stale")
	fresh := filepath.Join(bobDir, adminViewDirPrefix+"in-use")
	for _, dir := range []string{stale, fresh} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("a day-old leftover copy folder was kept (%v)", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("a recent copy folder, possibly in use, was removed: %v", err)
	}
}

// During access the administrator's own key still opens their own catalog
// and deletion log.
func TestAdminAccessLeavesTheAdministratorsOwnDataOpen(t *testing.T) {
	app, _ := adminAccessApp(t)
	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	if _, err := app.ListCatalogVendors("", false); err != nil {
		t.Fatalf("ListCatalogVendors during access: %v", err)
	}
	if _, err := app.ListProjectDeletions(); err != nil {
		t.Fatalf("ListProjectDeletions during access: %v", err)
	}
}

// A project its owner has open from another process has recent changes in
// its -wal file; the copy includes them.
func TestAdminViewIncludesChangesStillInTheWriteAheadLog(t *testing.T) {
	app, path := adminAccessApp(t)
	bobDEK, err := app.store.UnlockDEK("bob", escrowTestPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(bob): %v", err)
	}
	defer zeroBytes(bobDEK)
	owner, err := db.InitEncryptedDB(path, bobDEK)
	if err != nil {
		t.Fatalf("bob opens his project: %v", err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	if _, err := owner.Conn.Exec(`UPDATE project SET name = 'Plan, revised'`); err != nil {
		t.Fatalf("bob edits his project: %v", err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("expected the edit in a -wal file (%v)", err)
	}

	if err := app.AdminOpenUserData("bob", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	meta, err := app.AdminViewUserProject(path)
	if err != nil {
		t.Fatalf("AdminViewUserProject: %v", err)
	}
	if meta.Project.Name != "Plan, revised" {
		t.Fatalf("viewed name = %q; want the edit still in bob's -wal file", meta.Project.Name)
	}
}

func TestAdminAccessRefusalsInPlainWords(t *testing.T) {
	app, _ := adminAccessApp(t)
	if _, err := app.store.CreateAccount("dave", "Dave", escrowTestPassword, false); err != nil {
		t.Fatalf("create dave: %v", err)
	}
	for reason, want := range map[string]string{
		"   ":                    "enter a reason for opening this account's data",
		strings.Repeat("x", 501): "keep the reason to 500 characters or fewer",
	} {
		if err := app.AdminOpenUserData("bob", reason); err == nil || err.Error() != want {
			t.Errorf("reason %.10q: err = %v, want %q", reason, err, want)
		}
	}
	want := "dave has not signed in since administrator access began, so their data can't be opened yet"
	if err := app.AdminOpenUserData("dave", "checking"); err == nil || err.Error() != want {
		t.Errorf("unenrolled account: err = %v, want %q", err, want)
	}
	if _, err := app.AdminListUserProjects(); err == nil {
		t.Error("listing projects with no access open succeeded")
	}
}
