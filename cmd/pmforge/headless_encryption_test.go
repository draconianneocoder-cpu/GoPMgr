// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/cli"
	"pmforge/internal/db"
	"pmforge/internal/users"
)

func createHeadlessEncryptedProject(t *testing.T) (projectPath, password string) {
	t.Helper()

	password = "headless-password"
	root := t.TempDir()
	store, err := users.Open(root)
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	acc, err := store.CreateAccount("alice", "Alice", password)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	dek, err := store.UnlockDEK("alice", password)
	if err != nil {
		t.Fatalf("UnlockDEK: %v", err)
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
	t.Setenv("PMFORGE_HEADLESS_PASSWORD", password)

	d, err := openHeadlessDB(&cli.Config{
		CheckOnly:   true,
		ProjectPath: projectPath,
		Username:    "alice",
		PasswordEnv: "PMFORGE_HEADLESS_PASSWORD",
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
