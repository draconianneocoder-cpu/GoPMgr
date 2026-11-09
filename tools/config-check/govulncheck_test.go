// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func scannerWorkflowRun(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate workflow fixture")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../.github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := parseConfig(configSpec{path: "ci.yml", format: formatYAML}, data)
	if err != nil {
		t.Fatal(err)
	}
	jobs, ok := stringMap(config["jobs"])
	if !ok {
		t.Fatal("missing jobs")
	}
	job, ok := stringMap(jobs["vuln"])
	if !ok {
		t.Fatal("missing vulnerability job")
	}
	steps, ok := job["steps"].([]any)
	if !ok {
		t.Fatal("missing vulnerability steps")
	}
	for _, raw := range steps {
		step, ok := stringMap(raw)
		if ok && step["name"] == name {
			run, ok := step["run"].(string)
			if !ok || run == "" {
				t.Fatal("missing executable workflow command")
			}
			return run
		}
	}
	t.Fatalf("workflow step %q is missing", name)
	return ""
}

func TestGovulncheckWorkflowCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX workflow shell qualification")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, tool, wantArgs string }{
		{"Install govulncheck", "go", "install\ngolang.org/x/vuln/cmd/govulncheck@v1.9.0\n"},
		{"Scan default build (govulncheck ./...)", "govulncheck", "./...\n"},
	} {
		t.Run(tt.tool, func(t *testing.T) {
			run := scannerWorkflowRun(t, tt.name)
			for _, status := range []string{"0", "73"} {
				t.Run(status, func(t *testing.T) {
					root := t.TempDir()
					log := filepath.Join(root, "arguments")
					stub := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARG_LOG\"\nprintf 'scanner-boundary-marker\\n' >&2\nexit \"$TOOL_EXIT\"\n"
					if err := os.WriteFile(filepath.Join(root, tt.tool), []byte(stub), 0700); err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					t.Cleanup(cancel)
					command := exec.CommandContext(ctx, bash, "--noprofile", "--norc", "-e", "-c", run)
					command.Dir = root
					// Unspecified Actions shells use bash -e, without pipefail.
					var env []string
					for _, value := range os.Environ() {
						if !strings.HasPrefix(value, "SHELLOPTS=") && !strings.HasPrefix(value, "BASHOPTS=") {
							env = append(env, value)
						}
					}
					command.Env = append(env, "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"), "ARG_LOG="+log, "TOOL_EXIT="+status, "BASH_ENV=/dev/null")
					output, err := command.CombinedOutput()
					if status == "0" && err != nil {
						t.Fatalf("healthy step: %v: %s", err, output)
					}
					if status != "0" && err == nil {
						t.Fatalf("workflow swallowed %s failure: %s", tt.tool, output)
					}
					if !strings.Contains(string(output), "scanner-boundary-marker") {
						t.Fatalf("tool boundary not reached: %s", output)
					}
					args, readErr := os.ReadFile(log)
					if readErr != nil {
						t.Fatal(readErr)
					}
					if string(args) != tt.wantArgs {
						t.Fatalf("workflow tool arguments = %q, want %q", args, tt.wantArgs)
					}
				})
			}
		})
	}
}
