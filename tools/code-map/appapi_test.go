// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeAppFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(appPackageDir))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGenerateAppAPIListsOnlyExportedAppMethods(t *testing.T) {
	root := writeAppFixture(t, map[string]string{
		"app.go": `package desktop

import (
	"gopmgr/internal/db"
	kern "gopmgr/internal/kernel"
	"strings"
)

type App struct{}
type Other struct{}

func (a *App) Open(path string, force bool) (*db.Database, error) {
	_ = strings.TrimSpace(path)
	return db.Open(path)
}

// A local variable named like an import is not a use of that package.
func (a App) Rollup(db int) int { _ = kern.Sum; return db.Value }

func (a *App) helper() { _ = db.Open }
func (o *Other) Exported() { _ = db.Open }
func Free() { _ = db.Open }
`,
		"app_test.go": `package desktop

func (a *App) TestOnly() {}
`,
	})
	data, err := generateAppAPI(root)
	if err != nil {
		t.Fatal(err)
	}
	var got appAPI
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := []appMethod{
		{Method: "Open", File: "internal/desktop/app.go", Line: 12, Params: "path string, force bool", Returns: "*db.Database, error", UsesInternal: []string{"gopmgr/internal/db"}},
		{Method: "Rollup", File: "internal/desktop/app.go", Line: 18, Params: "db int", Returns: "int", UsesInternal: []string{"gopmgr/internal/kernel"}},
	}
	if !reflect.DeepEqual(got.Methods, want) {
		t.Fatalf("methods = %+v\nwant      %+v", got.Methods, want)
	}
}

func TestGenerateAppAPIRefusesAnEmptyPackage(t *testing.T) {
	root := writeAppFixture(t, map[string]string{"app.go": "package desktop\n\ntype App struct{}\n"})
	if _, err := generateAppAPI(root); err == nil || !strings.Contains(err.Error(), "no exported App methods") {
		t.Fatalf("err = %v, want a refusal", err)
	}
}
