// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command s0fs records the filesystem behavior ADR-010 publication relies
// on, at each candidate location (S0, DB-021, INT-03). It works only in
// throwaway folders it creates and removes, and never reads, moves or
// deletes existing data. Run it on test machines only.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type result struct {
	ID         string         `json:"id"`
	Location   string         `json:"location"`
	Path       string         `json:"path"`
	Status     string         `json:"status"`
	Checks     map[string]any `json:"checks"`
	Notes      []string       `json:"notes,omitempty"`
	Platform   string         `json:"platform"`
	SourceHead string         `json:"source_head"`
	StartedAt  string         `json:"started_at"`
}

// defaultRoot mirrors users.DefaultRootDir (internal/users/store.go:725),
// repeated here so this probe stays free of GoPMgr's CGO dependencies.
func defaultRoot() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "GoPMgr")
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "GoPMgr")
	}
	return filepath.Join(home, "Documents", "GoPMgr")
}

// nearestExisting returns path or its closest existing ancestor, so the
// probe never creates the GoPMgr data root itself.
func nearestExisting(path string) string {
	for p := path; ; p = filepath.Dir(p) {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
		if filepath.Dir(p) == p {
			return p
		}
	}
}

func probe(location, base string) *result {
	r := &result{ID: runtime.GOOS + "/" + location, Location: location, Path: base, Status: "PASS",
		Checks: map[string]any{}, Platform: runtime.GOOS + "/" + runtime.GOARCH, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	dir, err := os.MkdirTemp(base, ".gopmgr-s0fs-")
	if err != nil {
		r.Status = "BLOCKED"
		r.Notes = append(r.Notes, "cannot create a throwaway folder here: "+err.Error())
		return r
	}
	defer os.RemoveAll(dir)
	r.Checks["filesystem"] = fsType(dir)
	r.Checks["sync_hint"] = syncHint(base)

	// No-replace rename must refuse an existing file and an existing
	// empty folder, and still rename to a free name.
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	must(os.WriteFile(a, []byte("a"), 0o600))
	must(os.WriteFile(b, []byte("b"), 0o600))
	err = renameNoReplace(a, b)
	r.Checks["no_replace_file_refused"] = err != nil
	r.Checks["no_replace_file_error"] = errText(err)
	got, _ := os.ReadFile(b)
	if string(got) != "b" {
		r.fail("a no-replace rename overwrote an existing file")
	}
	if err := renameNoReplace(a, c); err != nil {
		r.fail("a no-replace rename to a free name failed: " + err.Error())
	}
	d1, d2 := filepath.Join(dir, "d1"), filepath.Join(dir, "d2")
	must(os.Mkdir(d1, 0o700))
	must(os.WriteFile(filepath.Join(d1, "project.gopmgr"), []byte("x"), 0o600))
	must(os.Mkdir(d2, 0o700))
	err = renameNoReplace(d1, d2)
	r.Checks["no_replace_dir_refused"] = err != nil
	r.Checks["no_replace_dir_error"] = errText(err)
	if err == nil {
		r.fail("a no-replace rename replaced an existing empty folder")
	}
	// The hazard: does the plain rename call replace an empty folder?
	replaced, detail := plainRenameReplacesEmptyDir(d1, d2)
	r.Checks["plain_rename_replaces_empty_dir"] = replaced
	r.Checks["plain_rename_detail"] = detail

	// Case sensitivity.
	must(os.WriteFile(filepath.Join(dir, "CaseProbe.txt"), nil, 0o600))
	_, err = os.Stat(filepath.Join(dir, "caseprobe.txt"))
	r.Checks["case_insensitive"] = err == nil

	// Exclusive locks from two handles to one file.
	excl, detail := lockConflicts(filepath.Join(dir, "lock"))
	r.Checks["exclusive_lock_conflicts"] = excl
	r.Checks["lock_detail"] = detail

	// Directory fsync, which publication uses where supported.
	r.Checks["dir_fsync"] = errText(syncDir(dir))

	// A deep path, as a long project folder name could produce.
	long := filepath.Join(dir, strings.Repeat("p", 60), strings.Repeat("q", 60), strings.Repeat("r", 60), strings.Repeat("s", 60))
	err = os.MkdirAll(long, 0o700)
	if err == nil {
		err = os.WriteFile(filepath.Join(long, "project.gopmgr"), nil, 0o600)
	}
	r.Checks["path_len_tested"] = len(long) + len("/project.gopmgr")
	r.Checks["long_path"] = errText(err)
	return r
}

func (r *result) fail(note string) {
	r.Status = "FAIL"
	r.Notes = append(r.Notes, note)
}

func errText(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

var errUnsupported = errors.New("not supported on this platform")

func main() {
	out := flag.String("out", "s0-fs.jsonl", "evidence file to append to")
	head := flag.String("head", "unknown", "source commit the run was built from")
	flag.Parse()
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "s0fs:", err)
		os.Exit(1)
	}
	defer f.Close()
	home, _ := os.UserHomeDir()
	root := defaultRoot()
	locations := []struct{ name, path string }{
		{"temp", os.TempDir()},
		{"home", home},
		{"default-data-root", nearestExisting(root)},
	}
	for _, loc := range locations {
		r := probe(loc.name, loc.path)
		r.SourceHead = *head
		if loc.name == "default-data-root" && loc.path != root {
			r.Notes = append(r.Notes, "default data root "+root+" does not exist yet; probed its nearest existing parent")
		}
		line, _ := json.Marshal(r)
		_, _ = f.Write(append(line, '\n'))
		b, _ := json.Marshal(r.Checks)
		fmt.Printf("%-7s %-30s %s\n", r.Status, r.ID, b)
	}
}
