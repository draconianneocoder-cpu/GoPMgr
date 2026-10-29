// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/cli"
	"gopmgr/internal/db"
	"gopmgr/internal/users"
)

func createHeadlessEncryptedProject(t *testing.T) (projectPath, password string) {
	t.Helper()

	password = "headless-password"
	root := t.TempDir()
	store, err := users.Open(root)
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	acc, err := store.CreateAccount("alice", "Alice", password, false)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	_, _, dek, err := store.SignInHeadless("alice", password)
	if err != nil {
		t.Fatalf("SignInHeadless: %v", err)
	}

	projectPath = filepath.Join(acc.DataDir, "projects", "headless.pmforge")
	d, err := db.InitEncryptedDB(projectPath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if _, err := d.UpsertProject(db.Project{Name: "Headless Secret", Owner: "alice"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close encrypted project: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close users store: %v", err)
	}
	return projectPath, password
}

func TestOpenHeadlessDBUnlocksEncryptedProjectWithPasswordEnv(t *testing.T) {
	projectPath, password := createHeadlessEncryptedProject(t)
	t.Setenv("GOPMGR_HEADLESS_PASSWORD", password)

	d, err := openHeadlessDB(&cli.Config{
		CheckOnly:   true,
		ProjectPath: projectPath,
		Username:    "alice",
		PasswordEnv: "GOPMGR_HEADLESS_PASSWORD",
	})
	if err != nil {
		t.Fatalf("openHeadlessDB: %v", err)
	}
	defer d.Close()

	project, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if project.Name != "Headless Secret" || project.Owner != "alice" {
		t.Fatalf("project = %#v, want encrypted headless project", project)
	}
}

func TestOpenHeadlessDBEncryptedProjectRequiresCredentials(t *testing.T) {
	projectPath, _ := createHeadlessEncryptedProject(t)

	_, err := openHeadlessDB(&cli.Config{
		CheckOnly:   true,
		ProjectPath: projectPath,
	})
	if err == nil || !strings.Contains(err.Error(), "--username") {
		t.Fatalf("openHeadlessDB error = %v, want username requirement", err)
	}
}

func TestHeadlessCheckDoesNotAddAMissingColumnAndVacuumDoes(t *testing.T) {
	projectPath, password := createHeadlessEncryptedProject(t)
	t.Setenv("GOPMGR_HEADLESS_PASSWORD", password)
	root := filepath.Dir(filepath.Dir(filepath.Dir(projectPath)))
	store, err := users.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, dek, err := store.SignInHeadless("alice", password)
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.InitEncryptedDB(projectPath, dek)
	if err != nil {
		t.Fatal(err)
	}
	omitAppSQLiteColumn(t, d, "project", "currency_code")
	freezeAppDeleteJournal(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	removeAppSQLiteSidecars(t, projectPath)
	before := stampAppDBFile(t, projectPath)

	checked, err := openHeadlessDB(&cli.Config{
		CheckOnly:   true,
		ProjectPath: projectPath,
		Username:    "alice",
		PasswordEnv: "GOPMGR_HEADLESS_PASSWORD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := checked.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stampAppDBFile(t, projectPath)) {
		t.Fatal("headless check changed the project file")
	}
	ro, err := db.OpenEncryptedReadOnly(projectPath, dek)
	if err != nil {
		t.Fatal(err)
	}
	var absent int
	if err := ro.Conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('project') WHERE name = 'currency_code'`).Scan(&absent); err != nil {
		t.Fatal(err)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if absent != 0 {
		t.Fatal("headless check added currency_code")
	}

	vacuumed, err := openHeadlessDB(&cli.Config{
		Vacuum:      true,
		ProjectPath: projectPath,
		Username:    "alice",
		PasswordEnv: "GOPMGR_HEADLESS_PASSWORD",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = vacuumed.Close() }()
	var present int
	if err := vacuumed.Conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('project') WHERE name = 'currency_code'`).Scan(&present); err != nil {
		t.Fatal(err)
	}
	if present != 1 {
		t.Fatal("writable headless open left currency_code missing")
	}
}
