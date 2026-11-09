// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeSource(t *testing.T, root, name, source string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunRequiresRootsAndDistinguishesFindingsFromErrors(t *testing.T) {
	root := t.TempDir()
	leak := writeSource(t, root, "leak.go", "package p\nfunc f() {\n os.Open(\"fixture\")\n}\n") // Synthetic Go source, not a live resource.
	for _, tc := range []struct {
		name       string
		args       []string
		status     int
		diagnostic string
	}{
		{"no roots", nil, 2, "root"},
		{"empty directory", []string{t.TempDir()}, 0, ""},
		{"missing root", []string{filepath.Join(root, "missing")}, 2, "missing"},
		{"finding", []string{leak}, 1, "leak.go:3:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			got := run(tc.args, &out, &errOut)
			if got != tc.status {
				t.Fatalf("status=%d; want %d; %s %s", got, tc.status, &out, &errOut)
			}
			if tc.diagnostic != "" && !strings.Contains(out.String()+errOut.String(), tc.diagnostic) {
				t.Fatalf("missing diagnostic: %s %s", &out, &errOut)
			}
			if got == 0 && (out.Len() != 0 || errOut.Len() != 0) {
				t.Fatal("clean scan emitted findings/errors")
			}
		})
	}
}

func TestRunUsesExplicitRootsAndPrunesVendorComponents(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "internal")
	writeSource(t, root, "unrelated/leak.go", "os.Open(\"outside\")\n")          // Synthetic source.
	writeSource(t, root, "internal/vendor/lib/leak.go", "os.Open(\"vendor\")\n") // Synthetic source.
	clean := writeSource(t, root, "main.go", "package main\nfunc main() {}\n")
	var out, errOut bytes.Buffer
	if got := run([]string{clean, inside}, &out, &errOut); got != 0 {
		t.Fatalf("off-scope/vendor source scanned: %d, %s %s", got, &out, &errOut)
	}
	writeSource(t, root, "internal/myvendor/leak.go", "os.Open(\"owned\")\n") // Not a vendor component.
	if got := run([]string{inside}, &out, &errOut); got != 1 || !strings.Contains(out.String(), "myvendor") {
		t.Fatalf("owned source omitted: %d, %s %s", got, &out, &errOut)
	}
}

func TestRunPreservesTenLineAndCommentHeuristic(t *testing.T) {
	for _, tc := range []struct {
		gap    int
		status int
	}{{8, 0}, {9, 1}} {
		root := t.TempDir()
		body := "package p\nfunc f() {\n f,_:=os.Open(\"fixture\")\n" + strings.Repeat("\n", tc.gap) + "f.Close()\n}\n" // Synthetic source.
		path := writeSource(t, root, "fixture.go", body)
		var out, errOut bytes.Buffer
		if got := run([]string{path}, &out, &errOut); got != tc.status {
			t.Fatalf("gap %d: %d, %s %s", tc.gap, got, &out, &errOut)
		}
	}
	path := writeSource(t, t.TempDir(), "comment.go", "os.Open(\"fixture\") // existing heuristic ignores commented lines\n")
	var out, errOut bytes.Buffer
	if got := run([]string{path}, &out, &errOut); got != 0 {
		t.Fatalf("comment behavior changed: %d, %s %s", got, &out, &errOut)
	}
}

func TestRunRejectsSymlinksAndIncompleteReads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native symlink and Unix permission fixtures")
	}
	root := t.TempDir()
	outside := writeSource(t, t.TempDir(), "outside.go", "package p\n")
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, root} {
		var out, errOut bytes.Buffer
		if got := run([]string{path}, &out, &errOut); got != 2 || !strings.Contains(errOut.String(), "symlink") {
			t.Fatalf("symlink not refused: %d, %s %s", got, &out, &errOut)
		}
	}
	long := writeSource(t, t.TempDir(), "long.go", strings.Repeat("x", (1<<20)+1)+"\n")
	var out, errOut bytes.Buffer
	if got := run([]string{long}, &out, &errOut); got != 2 {
		t.Fatalf("incomplete scan called clean: %d, %s %s", got, &out, &errOut)
	}
	if os.Geteuid() == 0 {
		t.Skip("permission-denial cases require an unprivileged identity")
	}
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "read", true: "traversal"}[directory], func(t *testing.T) {
			path := writeSource(t, t.TempDir(), "denied/file.go", "package p\n")
			mode := os.FileMode(0600)
			if directory {
				path = filepath.Dir(path)
				mode = 0700
			}
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(path, mode); err != nil {
					t.Error(err)
				}
			})
			var out, errOut bytes.Buffer
			if got := run([]string{path}, &out, &errOut); got != 2 || errOut.Len() == 0 {
				t.Fatalf("unreadable input called clean: %d, %s %s", got, &out, &errOut)
			}
		})
	}
}

func TestCLIReportsExitStatus(t *testing.T) {
	if os.Getenv("GOPMGR_OPEN_SCAN_CHILD") == "1" {
		os.Args = []string{"os-open-scan"}
		if path := os.Getenv("GOPMGR_OPEN_SCAN_ROOT"); path != "" {
			os.Args = append(os.Args, path)
		}
		main()
		return
	}
	root := t.TempDir()
	leak := writeSource(t, root, "leak.go", "os.Open(\"fixture\")\n") // Synthetic source.
	for _, tc := range []struct {
		root       string
		status     int
		diagnostic string
	}{
		{t.TempDir(), 0, ""}, {leak, 1, "leak.go:1:"}, {filepath.Join(root, "missing"), 2, "missing"}, {"", 2, "root"},
	} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCLIReportsExitStatus$")
		cmd.Env = append(os.Environ(), "GOPMGR_OPEN_SCAN_CHILD=1", "GOPMGR_OPEN_SCAN_ROOT="+tc.root)
		output, err := cmd.CombinedOutput()
		status := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				status = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if status != tc.status || (tc.diagnostic != "" && !strings.Contains(string(output), tc.diagnostic)) {
			t.Fatalf("CLI status=%d want=%d output=%s", status, tc.status, output)
		}
	}
}
