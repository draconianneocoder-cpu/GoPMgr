// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Paths of the ledger and its generated view, relative to the repository root.
const (
	ledgerPath     = "docs/ledger.json"
	ledgerViewPath = "docs/LEDGER.md"
	maxSummary     = 120
)

// Ledger is docs/ledger.json: one entry per document (or per evidence-data
// directory), with permanent IDs. See docs/design/docs-reorganization-plan.md
// section 6.
type Ledger struct {
	NextID           int        `json:"next_id"`
	NamingRuleFrom   int        `json:"naming_rule_from"`
	Entries          []Entry    `json:"entries"`
	Planned          []Planned  `json:"planned"`
	RecordedDangling []Dangling `json:"recorded_dangling"`
}

// Entry is one ledger row.
type Entry struct {
	ID             string     `json:"id"`
	Path           string     `json:"path"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	StatusEvidence string     `json:"status_evidence"`
	StatedStatus   string     `json:"stated_status,omitempty"`
	Governed       string     `json:"governed,omitempty"`
	Summary        string     `json:"summary"`
	FrozenSHA256   string     `json:"frozen_sha256,omitempty"`
	PreviousPaths  []PrevPath `json:"previous_paths,omitempty"`
	ContentNowIn   string     `json:"content_now_in,omitempty"`
	SupersededBy   string     `json:"superseded_by,omitempty"`
}

// PrevPath is a path an entry used to have, and the PR that stopped using it.
type PrevPath struct {
	Path  string `json:"path"`
	Until string `json:"until"`
}

// Planned is a promised document that does not exist yet.
type Planned struct {
	Path       string `json:"path"`
	PromisedBy string `json:"promised_by"`
}

// Dangling is a reference that may stay unresolved because its source
// cannot be edited.
type Dangling struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

var kinds = []string{"guide", "reference", "status", "roadmap", "adr", "design", "plan",
	"decision", "review", "strategy", "evidence", "evidence-data", "release", "agent", "index"}

var statuses = []string{"current", "proposed", "accepted", "superseded", "historical", "frozen", "removed"}

// allowedFolders maps a kind to the top-level docs folders it may live in;
// "" is docs/ itself and "*" is any folder (plan section 6.3).
var allowedFolders = map[string][]string{
	"adr":           {"design"},
	"design":        {"design"},
	"plan":          {"design", "engineering", "testing", ""},
	"strategy":      {"testing", "design"},
	"evidence":      {"testing", "engineering"},
	"evidence-data": {"testing", "engineering"},
	"decision":      {"review"},
	"review":        {"review", "interoperability", "engineering"},
	"reference":     {"", "design", "engineering"},
	"guide":         {""},
	"status":        {""},
	"roadmap":       {""},
	"release":       {"release-notes", "releases", ""},
	"agent":         {"agent"},
	"index":         {"*"},
}

var (
	idPattern       = regexp.MustCompile(`^DOC-(\d{4})$`)
	commitPattern   = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	prPattern       = regexp.MustCompile(`^PR [1-9][0-9]*$`)
	newNamePattern  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*\.[a-z0-9]+$`)
	newDirPattern   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	adrNamePattern  = regexp.MustCompile(`^ADR-\d{3}-[a-z0-9]+(-[a-z0-9]+)*\.md$`)
	datePattern     = regexp.MustCompile(`\d{4}-\d{2}(-\d{2})?`)
	dateSuffix      = regexp.MustCompile(`-\d{4}-\d{2}(-\d{2})?(\.[a-z0-9]+)?$`)
	genericPrefixes = []string{"roadmap-", "test-", "critical-review-", "adr-tbd-"}
)

func loadLedger(root string) (*Ledger, error) {
	data, err := os.ReadFile(path.Join(root, ledgerPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ledgerPath, err)
	}
	return parseLedger(data)
}

func parseLedger(data []byte) (*Ledger, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var l Ledger
	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ledgerPath, err)
	}
	return &l, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// isDirEntry reports whether an entry covers a directory ("dir/**").
func isDirEntry(p string) bool { return strings.HasSuffix(p, "/**") }

func dirOf(p string) string { return strings.TrimSuffix(p, "/**") }

// covers reports whether entry path p covers tracked file f.
func covers(p, f string) bool {
	if isDirEntry(p) {
		return strings.HasPrefix(f, dirOf(p)+"/")
	}
	return p == f
}

func idNumber(id string) (int, bool) {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil && n > 0
}

// topFolder is the first folder under docs/, or "" for docs/ itself.
func topFolder(p string) string {
	rest := strings.TrimPrefix(dirOf(p), "docs/")
	if !strings.Contains(rest, "/") {
		if isDirEntry(p) {
			return rest
		}
		return ""
	}
	return rest[:strings.Index(rest, "/")]
}

// repo is the part of the repository the checks read.
type repo struct {
	root    string
	tracked map[string]bool // every tracked file
	dirs    map[string]bool // every directory holding a tracked file
	read    func(string) ([]byte, error)
	// ignored reports whether .gitignore excludes a path: such a path is
	// local-only by policy, so a reference to it is not a broken link.
	ignored func(string) bool
}

func (r *repo) exists(p string) bool { return r.tracked[p] || r.dirs[strings.TrimSuffix(p, "/")] }

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// validate runs contracts 1, 2, 3, 6 and 7 (plan section 6.4). Links
// (contract 4) and generated output (contract 5) are checked separately.
func validate(l *Ledger, r *repo) []string {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	// Contract 6: IDs unique, well formed, and contiguous below next_id.
	seen := map[int]string{}
	for _, e := range l.Entries {
		n, ok := idNumber(e.ID)
		switch {
		case !ok:
			add("%s: id %q is not DOC- plus four digits", e.Path, e.ID)
		case seen[n] != "":
			add("%s: id %s is also used by %s", e.Path, e.ID, seen[n])
		case n >= l.NextID:
			add("%s: id %s is not below next_id %d", e.Path, e.ID, l.NextID)
		default:
			seen[n] = e.Path
		}
	}
	for n := 1; n < l.NextID; n++ {
		if seen[n] == "" {
			add("DOC-%04d has no entry: IDs are never deleted; mark the entry removed instead", n)
		}
	}

	// Contract 1: every tracked docs file is covered by exactly one live entry.
	live := map[string]Entry{}
	for _, e := range l.Entries {
		if e.Status == "removed" {
			if (isDirEntry(e.Path) && r.dirs[dirOf(e.Path)]) || (!isDirEntry(e.Path) && r.tracked[e.Path]) {
				add("%s (%s): marked removed, but the file still exists", e.Path, e.ID)
			}
			continue
		}
		if prev, dup := live[e.Path]; dup {
			add("%s: listed twice (%s and %s)", e.Path, prev.ID, e.ID)
		}
		live[e.Path] = e
		if isDirEntry(e.Path) {
			if !r.dirs[dirOf(e.Path)] {
				add("%s (%s): no tracked files under this directory", e.Path, e.ID)
			}
		} else if !r.tracked[e.Path] {
			add("%s (%s): listed, but not a tracked file", e.Path, e.ID)
		}
	}
	var files []string
	for f := range r.tracked {
		if strings.HasPrefix(f, "docs/") {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	for _, f := range files {
		var by []string
		for p := range live {
			if covers(p, f) {
				by = append(by, p)
			}
		}
		switch len(by) {
		case 0:
			add("%s: tracked under docs/ but has no ledger entry", f)
		case 1:
		default:
			sort.Strings(by)
			add("%s: covered by more than one entry: %s", f, strings.Join(by, ", "))
		}
	}
	for _, p := range l.Planned {
		if r.exists(p.Path) {
			add("%s: listed as planned, but it exists; give it a ledger entry instead", p.Path)
		}
		if !r.tracked[p.PromisedBy] {
			add("%s: planned, but promised_by %q is not a tracked file", p.Path, p.PromisedBy)
		}
	}

	// Contracts 2, 3 and 7, per entry.
	for _, e := range l.Entries {
		where := fmt.Sprintf("%s (%s)", e.Path, e.ID)
		if !contains(kinds, e.Kind) {
			add("%s: unknown kind %q", where, e.Kind)
		}
		if !contains(statuses, e.Status) {
			add("%s: unknown status %q", where, e.Status)
		}
		if s := strings.TrimSpace(e.Summary); s == "" || len([]rune(s)) > maxSummary {
			add("%s: summary must be 1 to %d characters", where, maxSummary)
		}
		switch ev := e.StatusEvidence; {
		case ev == "":
			add("%s: status_evidence is missing", where)
		case prPattern.MatchString(ev), commitPattern.MatchString(ev), r.tracked[ev]:
		default:
			add("%s: status_evidence %q is not \"PR N\", a commit hash or a tracked file", where, ev)
		}
		if e.Status == "frozen" {
			b, err := r.read(e.Path)
			switch {
			case e.FrozenSHA256 == "":
				add("%s: frozen without frozen_sha256", where)
			case err != nil:
				add("%s: cannot read frozen file: %v", where, err)
			case sha256Hex(b) != e.FrozenSHA256:
				add("%s: frozen file changed (sha256 %s, ledger %s)", where, sha256Hex(b), e.FrozenSHA256)
			}
		} else if e.FrozenSHA256 != "" {
			add("%s: frozen_sha256 is set but status is %s", where, e.Status)
		}
		switch {
		case e.Status == "removed" && e.ContentNowIn == "":
			add("%s: removed entries must say where the content went (content_now_in: an ID, or \"none\")", where)
		case e.Status != "removed" && e.ContentNowIn != "":
			add("%s: content_now_in is only for removed entries", where)
		case e.ContentNowIn == e.ID:
			add("%s: content_now_in names the entry itself", where)
		case e.ContentNowIn != "" && e.ContentNowIn != "none" && !liveID(l, e.ContentNowIn, e.ID):
			add("%s: content_now_in %q is not the ID of a document that still exists", where, e.ContentNowIn)
		}
		switch {
		case e.Status == "superseded" && e.SupersededBy == "":
			add("%s: superseded entries must name their replacement (superseded_by: an ID)", where)
		case e.Status != "superseded" && e.SupersededBy != "":
			add("%s: superseded_by is only for superseded entries", where)
		case e.SupersededBy == e.ID:
			add("%s: superseded_by names the entry itself", where)
		case e.SupersededBy != "" && !liveID(l, e.SupersededBy, e.ID):
			add("%s: superseded_by %q is not the ID of a document that still exists", where, e.SupersededBy)
		}
		if e.Status == "removed" || e.Status == "frozen" {
			continue
		}
		if path.Base(e.Path) == "README.md" && strings.Count(e.Path, "/") <= 2 && e.Kind != "index" {
			add("%s: a folder README is an index (kind index), so its list is generated and checked", where)
		}
		if folders, ok := allowedFolders[e.Kind]; ok && !contains(folders, "*") && !contains(folders, topFolder(e.Path)) {
			add("%s: a %s does not belong in %q (allowed: %s)", where, e.Kind, "docs/"+topFolder(e.Path), strings.Join(folders, ", "))
		}
		if n, ok := idNumber(e.ID); ok && l.NamingRuleFrom > 0 && n >= l.NamingRuleFrom {
			if msg := nameProblem(path.Base(dirOf(e.Path)), isDirEntry(e.Path)); msg != "" {
				add("%s: %s", where, msg)
			}
		}
	}
	return errs
}

// liveID reports whether id names an entry, other than self, that is not
// removed.
func liveID(l *Ledger, id, self string) bool {
	for _, e := range l.Entries {
		if e.ID == id && e.Status != "removed" {
			return true
		}
	}
	return false
}

// nameProblem applies the naming rule for new files (plan section 6.3).
func nameProblem(base string, dir bool) string {
	if base == "README.md" || base == "LEDGER.md" || adrNamePattern.MatchString(base) {
		return ""
	}
	pattern := newNamePattern
	if dir {
		pattern = newDirPattern
	}
	if !pattern.MatchString(base) {
		return "name must be lowercase letters, digits and hyphens"
	}
	lower := strings.ToLower(base)
	for _, p := range genericPrefixes {
		if strings.HasPrefix(lower, p) {
			return fmt.Sprintf("name starts with the generic prefix %q; start with the subject", p)
		}
	}
	if datePattern.MatchString(base) && !dateSuffix.MatchString(base) {
		return "a date in a name must be its suffix: -YYYY-MM or -YYYY-MM-DD"
	}
	return ""
}
