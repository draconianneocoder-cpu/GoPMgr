// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// plantedProject creates a plaintext project file, runs extra SQL against
// it, and closes it.
func plantedProject(t *testing.T, statements ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "planted.gopmgr")
	d, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpsertProject(Project{Name: "Planted"}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range statements {
		if _, err := d.Conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckForeignSchemaObjectsAcceptsGoPMgrsOwnSchema(t *testing.T) {
	t.Parallel()
	if err := CheckForeignSchemaObjects(plantedProject(t), nil); err != nil {
		t.Fatalf("a project GoPMgr created was refused: %v", err)
	}
}

func TestCheckForeignSchemaObjectsRefusesPlantedObjects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, sql, want string
	}{
		{"trigger", `CREATE TRIGGER planted AFTER INSERT ON project BEGIN DELETE FROM project; END`, "trigger planted"},
		{"view", `CREATE VIEW planted AS SELECT 1`, "view planted"},
		{"rtree", `CREATE VIRTUAL TABLE planted USING rtree(id, lo, hi)`, "virtual table planted"},
		{"fts3", `CREATE VIRTUAL TABLE planted USING fts3(body)`, "virtual table planted"},
		{"odd spelling", "create   virtual\ttable planted using fts3(body)", "virtual table planted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckForeignSchemaObjects(plantedProject(t, tc.sql), nil)
			var foreign *ForeignSchemaError
			if !errors.Is(err, ErrForeignSchemaObject) || !errors.As(err, &foreign) || !slices.Contains(foreign.Objects, tc.want) {
				t.Fatalf("CheckForeignSchemaObjects = %v, want ErrForeignSchemaObject listing %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "planted") {
				t.Fatalf("the error text shows a name from the file: %q", err)
			}
		})
	}
}

func TestCheckForeignSchemaObjectsWorksOnEncryptedFiles(t *testing.T) {
	t.Parallel()
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	path := filepath.Join(t.TempDir(), "encrypted.gopmgr")
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckForeignSchemaObjects(path, dek); err != nil {
		_ = d.Close()
		t.Fatalf("a clean encrypted project was refused: %v", err)
	}
	if _, err := d.Conn.Exec(`CREATE VIEW planted AS SELECT 1`); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := CheckForeignSchemaObjects(path, dek); !errors.Is(err, ErrForeignSchemaObject) {
		t.Fatalf("CheckForeignSchemaObjects = %v, want ErrForeignSchemaObject", err)
	}
}
