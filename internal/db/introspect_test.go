// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpSchemaListsTables(t *testing.T) {
	d, err := InitDB(filepath.Join(t.TempDir(), "schema.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = d.Close() }()

	schema, err := d.DumpSchema()
	if err != nil {
		t.Fatalf("DumpSchema: %v", err)
	}
	if !strings.Contains(schema, "CREATE TABLE") {
		t.Fatalf("schema has no CREATE TABLE statements:\n%s", schema)
	}
	if !strings.Contains(schema, "projects") {
		t.Fatalf("schema missing the projects table:\n%s", schema)
	}
	if strings.Contains(schema, "sqlite_") {
		t.Fatalf("schema should not expose internal sqlite_ objects:\n%s", schema)
	}
}
