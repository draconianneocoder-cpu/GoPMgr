// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/cli"
	"gopmgr/internal/sqlitedriver"
	"gopmgr/internal/users"
)

func TestStartupFailureMessage(t *testing.T) {
	t.Parallel()
	const generic = "GoPMgr failed to initialise its local data store."
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "data from a newer build",
			err:  fmt.Errorf("users: migrate: %w", users.ErrSystemSchemaNewer),
			want: "This GoPMgr data was last opened by a newer version of GoPMgr. " +
				"Install that version or a later one to continue. " +
				"Your accounts and projects were not changed.",
		},
		{name: "any other failure", err: errors.New("disk full"), want: generic},
		{name: "another users error", err: users.ErrAccountIdentity, want: generic},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := startupFailureMessage(tc.err); got != tc.want {
				t.Fatalf("startupFailureMessage = %q, want %q", got, tc.want)
			}
		})
	}
}

// The headless command prints the error as it stands, so the text must tell
// the user what to do without the dialog's help.
func TestOpenHeadlessDBRefusesANewerSystemDatabase(t *testing.T) {
	projectPath, password := createHeadlessEncryptedProject(t)
	t.Setenv("GOPMGR_HEADLESS_PASSWORD", password)

	// <root>/alice/projects/headless.pmforge
	root := filepath.Dir(filepath.Dir(filepath.Dir(projectPath)))
	conn, err := sql.Open(sqlitedriver.Name, filepath.Join(root, "system.db"))
	if err != nil {
		t.Fatalf("open system.db: %v", err)
	}
	if _, err := conn.Exec(`PRAGMA user_version = 2147483647`); err != nil {
		t.Fatalf("mark system.db as newer: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close system.db: %v", err)
	}

	d, err := openHeadlessDB(&cli.Config{
		CheckOnly:   true,
		ProjectPath: projectPath,
		Username:    "alice",
		PasswordEnv: "GOPMGR_HEADLESS_PASSWORD",
	})
	if err == nil {
		_ = d.Close()
		t.Fatal("openHeadlessDB opened a project under a newer system.db")
	}
	if !errors.Is(err, users.ErrSystemSchemaNewer) {
		t.Fatalf("openHeadlessDB = %v, want users.ErrSystemSchemaNewer", err)
	}
	const want = "open system database: users: migrate: users: this data was last opened by a newer GoPMgr; install that version or a later one"
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("command-line text = %q, want it to start %q", err.Error(), want)
	}
}
