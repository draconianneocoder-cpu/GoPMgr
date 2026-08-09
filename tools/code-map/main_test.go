// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildPackageMapKeepsOnlyRepositoryPackages(t *testing.T) {
	root := t.TempDir()
	result, err := buildPackageMap(root, []listedPackage{
		{
			ImportPath: "gopmgr",
			Dir:        root,
			Imports:    []string{"fmt", "gopmgr/internal/db", "github.com/wailsapp/wails/v2"},
		},
		{
			ImportPath: "gopmgr/internal/db",
			Dir:        filepath.Join(root, "internal", "db"),
			Imports:    []string{"github.com/mutecomm/go-sqlcipher/v4"},
		},
		{
			ImportPath: "gopmgr/frontend/node_modules/flatted/golang/pkg/flatted",
			Dir:        filepath.Join(root, "frontend", "node_modules", "flatted", "golang", "pkg", "flatted"),
		},
		{
			ImportPath: "example.test/outside",
			Dir:        filepath.Join(root, "vendor", "outside"),
		},
	})
	if err != nil {
		t.Fatalf("buildPackageMap: %v", err)
	}

	if _, found := result.Packages["gopmgr/frontend/node_modules/flatted/golang/pkg/flatted"]; found {
		t.Fatal("node_modules package was included in the package map")
	}
	if got := result.Packages["gopmgr"].Dir; got != "." {
		t.Fatalf("root directory = %q, want .", got)
	}
	if got := result.Packages["gopmgr/internal/db"].Dir; got != "internal/db" {
		t.Fatalf("database directory = %q, want internal/db", got)
	}
	if got, want := result.Packages["gopmgr"].ImportsInternal, []string{"gopmgr/internal/db"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root internal imports = %v, want %v", got, want)
	}
	if got, want := result.Packages["gopmgr"].NotableExternalDeps, []string{"github.com/wailsapp/wails/v2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root external dependencies = %v, want %v", got, want)
	}
	if got, want := result.Packages["gopmgr/internal/db"].ImportedByInternal, []string{"gopmgr"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("database importers = %v, want %v", got, want)
	}
}

func TestUniqueSorted(t *testing.T) {
	got := uniqueSorted([]string{"b", "a", "b"})
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueSorted() = %v, want %v", got, want)
	}
}

func TestRunWritesAndChecksCurrentMap(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	output := filepath.Join(t.TempDir(), "package-dependencies.json")
	if err := run(root, output, false); err != nil {
		t.Fatalf("run(write): %v", err)
	}
	if err := run(root, output, true); err != nil {
		t.Fatalf("run(check): %v", err)
	}
	if err := os.WriteFile(output, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("write stale map: %v", err)
	}
	if err := run(root, output, true); err == nil || !strings.Contains(err.Error(), "is stale") {
		t.Fatalf("run(check stale) error = %v, want stale-map failure", err)
	}
}
