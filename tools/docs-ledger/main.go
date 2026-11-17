// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// docs-ledger keeps docs/ledger.json, the documentation ledger, true to the
// repository, and generates docs/LEDGER.md and the folder README lists from
// it. The rules it enforces are in docs/design/docs-reorganization-plan.md,
// section 6.
//
//	go run ./tools/docs-ledger            check (contracts 1 to 7)
//	go run ./tools/docs-ledger -write     regenerate LEDGER.md and README lists
//	go run ./tools/docs-ledger -history origin/main
//	                                      also compare with the merge base's ledger
//	go run ./tools/docs-ledger -find DOC-0001
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("docs-ledger", flag.ContinueOnError)
	fs.SetOutput(stderr)
	write := fs.Bool("write", false, "regenerate docs/LEDGER.md and the folder README lists")
	history := fs.String("history", "", "also compare with the ledger at the merge base of this ref and HEAD")
	find := fs.String("find", "", "print the current and former paths of a document ID")
	sectionHash := fs.String("section-hash", "", "print the sha256 to record for the section titled -title in this file")
	title := fs.String("title", "", "with -section-hash: the section's title")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, err := gitOutput(".", "rev-parse", "--show-toplevel")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "docs-ledger: not in a git repository: %v\n", err)
		return 2
	}
	root = strings.TrimSpace(root)
	l, err := loadLedger(root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "docs-ledger: %v\n", err)
		return 1
	}
	if *find != "" {
		return findID(l, *find, stdout, stderr)
	}
	r, err := openRepo(root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "docs-ledger: %v\n", err)
		return 2
	}
	if *sectionHash != "" {
		b, err := r.read(*sectionHash)
		sec, appendix, ok := findSection(string(b), *title)
		if err != nil || !ok {
			_, _ = fmt.Fprintf(stderr, "docs-ledger: no section %q in %s\n", *title, *sectionHash)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, sha256Hex([]byte(normalizeRecord(sec, *title, appendix))))
		return 0
	}
	if *write {
		if err := writeGenerated(l, r); err != nil {
			_, _ = fmt.Fprintf(stderr, "docs-ledger: %v\n", err)
			return 1
		}
	}
	errs := check(l, r)
	if *history != "" {
		herrs, note := checkHistory(root, *history, l)
		herrs = append(herrs, sourceErrors(l, r, func(rev, p string) (string, error) {
			return gitOutput(root, "show", rev+":"+p)
		})...)
		errs = append(errs, herrs...)
		if note != "" {
			_, _ = fmt.Fprintln(stdout, "docs-ledger: "+note)
		}
	}
	if len(errs) > 0 {
		_, _ = fmt.Fprintf(stderr, "docs-ledger: %d problem(s):\n", len(errs))
		for _, e := range errs {
			_, _ = fmt.Fprintln(stderr, "  - "+e)
		}
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "docs-ledger: %d entries; every file under docs/ is listed, and every rule holds.\n", len(l.Entries))
	return 0
}

// check runs every contract that needs no history.
func check(l *Ledger, r *repo) []string {
	errs := validate(l, r)
	errs = append(errs, checkLinks(l, r)...)
	errs = append(errs, checkGenerated(l, r)...)
	errs = append(errs, checkSections(l, r)...)
	return errs
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

func openRepo(root string) (*repo, error) {
	out, err := gitOutput(root, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	r := &repo{root: root, tracked: map[string]bool{}, dirs: map[string]bool{},
		read: func(p string) ([]byte, error) { return os.ReadFile(path.Join(root, p)) },
		ignored: func(p string) bool {
			// --no-index: judge the path by the ignore rules alone.
			return exec.Command("git", "-C", root, "check-ignore", "-q", "--no-index", p).Run() == nil
		}}
	for _, f := range strings.Split(out, "\x00") {
		if f == "" {
			continue
		}
		r.tracked[f] = true
		for d := path.Dir(f); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	return r, nil
}

// readmes lists the folder READMEs the ledger generates lists for: every
// README.md entry of kind index.
func readmes(l *Ledger) []string {
	var out []string
	for _, e := range l.Entries {
		if e.Kind == "index" && e.Status != "removed" && path.Base(e.Path) == "README.md" {
			out = append(out, e.Path)
		}
	}
	sort.Strings(out)
	return out
}

// checkGenerated runs contract 5.
func checkGenerated(l *Ledger, r *repo) []string {
	var errs []string
	if b, err := r.read(ledgerViewPath); err != nil || string(b) != renderLedgerView(l) {
		errs = append(errs, ledgerViewPath+" does not match the ledger; run make docs-ledger")
	}
	for _, f := range readmes(l) {
		b, err := r.read(f)
		if err != nil {
			continue // contract 1 reports a missing file
		}
		want, ok := spliceList(string(b), renderFolderList(l, path.Dir(f)))
		switch {
		case !ok:
			errs = append(errs, f+": needs one "+beginMarker+" followed by one "+endMarker)
		case string(b) != want:
			errs = append(errs, f+": generated list does not match the ledger; run make docs-ledger")
		}
	}
	return errs
}

func writeGenerated(l *Ledger, r *repo) error {
	if err := os.WriteFile(path.Join(r.root, ledgerViewPath), []byte(renderLedgerView(l)), 0o644); err != nil {
		return err
	}
	for _, f := range readmes(l) {
		b, err := r.read(f)
		if err != nil {
			return err
		}
		out, ok := spliceList(string(b), renderFolderList(l, path.Dir(f)))
		if !ok {
			return fmt.Errorf("%s: needs one %s followed by one %s", f, beginMarker, endMarker)
		}
		if err := os.WriteFile(path.Join(r.root, f), []byte(out), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func findID(l *Ledger, id string, stdout, stderr io.Writer) int {
	for _, e := range l.Entries {
		if e.ID != id {
			continue
		}
		_, _ = fmt.Fprintf(stdout, "%s  %s  (%s, %s)\n", e.ID, e.Path, e.Kind, e.Status)
		if e.ContentNowIn != "" {
			_, _ = fmt.Fprintf(stdout, "  content now in: %s\n", e.ContentNowIn)
		}
		for _, p := range e.PreviousPaths {
			_, _ = fmt.Fprintf(stdout, "  formerly %s (until %s)\n", p.Path, p.Until)
		}
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "docs-ledger: no entry has id %s\n", id)
	return 1
}

// errNoMergeBase is returned when history is too shallow to compare.
var errNoMergeBase = errors.New("no merge base")

// checkHistory compares the ledger with the one at the merge base of base
// and HEAD (plan section 6.4). It never passes by skipping: a missing merge
// base is an error. A base without a ledger is the bootstrap case.
func checkHistory(root, base string, l *Ledger) ([]string, string) {
	mb, err := gitOutput(root, "merge-base", base, "HEAD")
	if err != nil {
		return []string{fmt.Sprintf("history: %v: %v; fetch more history (for example git fetch --unshallow) and run again", errNoMergeBase, err)}, ""
	}
	mb = strings.TrimSpace(mb)
	old, err := gitOutput(root, "show", mb+":"+ledgerPath)
	if err != nil {
		if _, lsErr := gitOutput(root, "cat-file", "-e", mb+":"+ledgerPath); lsErr != nil {
			return nil, "bootstrap: no base ledger at " + mb[:min(12, len(mb))] + "; the new ledger is checked on its own merits"
		}
		return []string{fmt.Sprintf("history: read base ledger: %v", err)}, ""
	}
	prev, err := parseLedger([]byte(old))
	if err != nil {
		return []string{"history: base " + err.Error()}, ""
	}
	return compareHistory(prev, l), ""
}

// compareHistory reports changes that would make IDs or old paths unreliable.
func compareHistory(prev, cur *Ledger) []string {
	var errs []string
	if cur.NextID < prev.NextID {
		errs = append(errs, fmt.Sprintf("history: next_id fell from %d to %d", prev.NextID, cur.NextID))
	}
	if prev.NamingRuleFrom > 0 && (cur.NamingRuleFrom == 0 || cur.NamingRuleFrom > prev.NamingRuleFrom) {
		errs = append(errs, fmt.Sprintf("history: naming_rule_from rose from %d to %d, which exempts new files from the naming rule", prev.NamingRuleFrom, cur.NamingRuleFrom))
	}
	byID := map[string]Entry{}
	for _, e := range cur.Entries {
		byID[e.ID] = e
	}
	byPath := map[string]Entry{}
	for _, e := range cur.Entries {
		byPath[e.Path] = e
	}
	for _, p := range prev.Entries {
		c, ok := byID[p.ID]
		if !ok {
			errs = append(errs, fmt.Sprintf("history: %s (%s) is gone; IDs are never deleted", p.ID, p.Path))
			continue
		}
		for _, old := range p.PreviousPaths {
			kept := false
			for _, pp := range c.PreviousPaths {
				kept = kept || pp.Path == old.Path
			}
			if !kept {
				errs = append(errs, fmt.Sprintf("history: %s dropped its former path %s from previous_paths", p.ID, old.Path))
			}
		}
		for _, ps := range p.Sections {
			kept := false
			for _, cs := range c.Sections {
				kept = kept || cs == ps
			}
			if !kept {
				errs = append(errs, fmt.Sprintf("history: %s dropped or changed its recorded section %q (from %s); merged records stay recorded as written", p.ID, ps.Title, ps.From))
			}
		}
		if c.Path != p.Path {
			kept := false
			for _, pp := range c.PreviousPaths {
				kept = kept || pp.Path == p.Path
			}
			if !kept {
				errs = append(errs, fmt.Sprintf("history: %s moved from %s to %s without adding the old path to previous_paths", p.ID, p.Path, c.Path))
			}
		}
		if other, ok := byPath[p.Path]; ok && other.ID != p.ID && p.Status != "removed" {
			errs = append(errs, fmt.Sprintf("history: %s was %s and is now %s; IDs are never renumbered", p.Path, p.ID, other.ID))
		}
	}
	return errs
}
