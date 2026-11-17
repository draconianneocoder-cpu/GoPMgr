// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a small repository with a valid ledger; cases mutate it.
type fixture struct {
	files  map[string]string
	ledger Ledger
}

const frozenText = "# Accepted record\n\nKept as received.\n"

func baseFixture() *fixture {
	return &fixture{
		files: map[string]string{
			"AGENTS.md":                        "# Agents\n\nSee [the guide](docs/guide.md#install).\n",
			"docs/guide.md":                    "# Guide\n\n## Install\n\nSee `docs/testing/run-2026-10-01.md`.\n",
			"docs/design/ADR-001-storage.md":   "# ADR-001\n\n**Status:** Accepted.\n",
			"docs/design/frozen-plan.md":       frozenText,
			"docs/testing/run-2026-10-01.md":   "# Run\n\nResults.\n",
			"docs/testing/data/raw.json":       "{}\n",
			"docs/testing/README.md":           "# Testing\n\n" + beginMarker + "\n" + endMarker + "\n",
			"docs/ledger.json":                 "", // written from ledger
			"docs/LEDGER.md":                   "", // generated
			"docs/design/reviewed-source.md":   "# Source\n",
			"docs/design/reviewed-source-2.md": "# Source two\n",
		},
		ledger: Ledger{
			NextID:         11,
			NamingRuleFrom: 9,
			Entries: []Entry{
				{ID: "DOC-0001", Path: "docs/guide.md", Kind: "guide", Status: "current", StatusEvidence: "PR 1", Summary: "Guide"},
				{ID: "DOC-0002", Path: "docs/design/ADR-001-storage.md", Kind: "adr", Status: "accepted", StatusEvidence: "docs/design/ADR-001-storage.md", StatedStatus: "Accepted.", Summary: "ADR-001"},
				{ID: "DOC-0003", Path: "docs/design/frozen-plan.md", Kind: "design", Status: "frozen", StatusEvidence: "PR 1", Summary: "Frozen plan", FrozenSHA256: sha256Hex([]byte(frozenText))},
				{ID: "DOC-0004", Path: "docs/testing/run-2026-10-01.md", Kind: "evidence", Status: "historical", StatusEvidence: "abc1234", Summary: "Run"},
				{ID: "DOC-0005", Path: "docs/testing/data/**", Kind: "evidence-data", Status: "historical", StatusEvidence: "PR 1", Summary: "Raw data"},
				{ID: "DOC-0006", Path: "docs/design/reviewed-source.md", Kind: "design", Status: "proposed", StatusEvidence: "PR 1", Summary: "Source"},
				{ID: "DOC-0007", Path: "docs/design/reviewed-source-2.md", Kind: "design", Status: "proposed", StatusEvidence: "PR 1", Summary: "Source two"},
				{ID: "DOC-0008", Path: "docs/testing/README.md", Kind: "index", Status: "current", StatusEvidence: "PR 1", Summary: "Testing index"},
				{ID: "DOC-0009", Path: "docs/ledger.json", Kind: "index", Status: "current", StatusEvidence: "PR 1", Summary: "Ledger"},
				{ID: "DOC-0010", Path: "docs/LEDGER.md", Kind: "index", Status: "current", StatusEvidence: "PR 1", Summary: "Ledger view"},
			},
			Planned: []Planned{{Path: "docs/testing/future.md", PromisedBy: "docs/guide.md"}},
		},
	}
}

func (f *fixture) entry(p string) *Entry {
	for i := range f.ledger.Entries {
		if f.ledger.Entries[i].Path == p {
			return &f.ledger.Entries[i]
		}
	}
	panic("no entry " + p)
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// materialize writes the fixture to a new git repository, generating
// LEDGER.md and README lists from the ledger, and returns its root.
func (f *fixture) materialize(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	data, err := json.MarshalIndent(f.ledger, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	f.files["docs/ledger.json"] = string(data)
	for p, text := range f.files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", "-A")
	r, err := openRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGenerated(&f.ledger, r); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "fixture")
	return root
}

func checkRoot(t *testing.T, root string) []string {
	t.Helper()
	l, err := loadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := openRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	return check(l, r)
}

// TestCheckContracts is the owner test for contracts 1 to 7 (plan 6.4).
// Each case names the regression it catches; want is the error it must
// produce, written by hand ("" means the tree must pass).
func TestCheckContracts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(f *fixture)
		after  func(t *testing.T, root string) // edits after generation
		want   string
	}{
		{name: "valid tree passes", want: ""},
		{
			name:   "unlisted file: a new document skips the ledger",
			mutate: func(f *fixture) { f.files["docs/testing/new-2026-11-01.md"] = "# New\n" },
			want:   "docs/testing/new-2026-11-01.md: tracked under docs/ but has no ledger entry",
		},
		{
			name:   "listed file missing: a move leaves a stale entry",
			mutate: func(f *fixture) { delete(f.files, "docs/design/reviewed-source.md") },
			want:   "docs/design/reviewed-source.md (DOC-0006): listed, but not a tracked file",
		},
		{
			name:   "planned path exists: a planned file lands without a real entry",
			mutate: func(f *fixture) { f.files["docs/testing/future.md"] = "# Future\n" },
			want:   "docs/testing/future.md: listed as planned, but it exists",
		},
		{
			name: "two entries for one file: ambiguous status",
			mutate: func(f *fixture) {
				f.entry("docs/design/reviewed-source-2.md").Path = "docs/design/reviewed-source.md"
				delete(f.files, "docs/design/reviewed-source-2.md")
			},
			want: "docs/design/reviewed-source.md: listed twice (DOC-0006 and DOC-0007)",
		},
		{
			name:   "unknown kind: vocabulary drift",
			mutate: func(f *fixture) { f.entry("docs/guide.md").Kind = "howto" },
			want:   `docs/guide.md (DOC-0001): unknown kind "howto"`,
		},
		{
			name:   "unknown status: vocabulary drift",
			mutate: func(f *fixture) { f.entry("docs/guide.md").Status = "draft" },
			want:   `docs/guide.md (DOC-0001): unknown status "draft"`,
		},
		{
			name: "one changed byte in a frozen file: an accepted record silently edited",
			mutate: func(f *fixture) {
				f.files["docs/design/frozen-plan.md"] = strings.Replace(frozenText, "Kept", "kept", 1)
			},
			want: "docs/design/frozen-plan.md (DOC-0003): frozen file changed",
		},
		{
			name:   "broken link in a current file: a move forgets a referrer",
			mutate: func(f *fixture) { f.files["docs/guide.md"] += "\nSee [old](design/gone.md).\n" },
			want:   `docs/guide.md: link "docs/design/gone.md" does not resolve`,
		},
		{
			name:   "broken anchor: a heading was renamed",
			mutate: func(f *fixture) { f.files["AGENTS.md"] = "# Agents\n\nSee [the guide](docs/guide.md#setup).\n" },
			want:   `AGENTS.md: anchor "docs/guide.md#setup" does not resolve`,
		},
		{
			name:   "broken plain-text path: a stale mention",
			mutate: func(f *fixture) { f.files["docs/guide.md"] += "\nSee `docs/testing/gone.md`.\n" },
			want:   `docs/guide.md: path "docs/testing/gone.md" does not resolve`,
		},
		{
			name:   "broken link in a historical file without a recorded_dangling entry",
			mutate: func(f *fixture) { f.files["docs/testing/run-2026-10-01.md"] += "\nSee `docs/review/old.md`.\n" },
			want:   `docs/testing/run-2026-10-01.md: path "docs/review/old.md" does not resolve`,
		},
		{
			name: "the same broken link with a recorded_dangling entry passes",
			mutate: func(f *fixture) {
				f.files["docs/testing/run-2026-10-01.md"] += "\nSee `docs/review/old.md`.\n"
				f.ledger.RecordedDangling = []Dangling{{Source: "docs/testing/run-2026-10-01.md", Target: "docs/review/old.md", Reason: "renamed"}}
			},
			want: "",
		},
		{
			name: "recorded_dangling from a current source: the exception misused for a fixable file",
			mutate: func(f *fixture) {
				f.ledger.RecordedDangling = []Dangling{{Source: "docs/guide.md", Target: "docs/review/old.md", Reason: "lazy"}}
			},
			want: "docs/guide.md: recorded_dangling is only for frozen, historical or governed sources",
		},
		{
			name: "stale README section: the index no longer matches the ledger",
			after: func(t *testing.T, root string) {
				replaceInFile(t, root, "docs/testing/README.md", "| Run |", "| Old summary |")
			},
			want: "docs/testing/README.md: generated list does not match the ledger",
		},
		{
			name:  "stale LEDGER.md: the view no longer matches the ledger",
			after: func(t *testing.T, root string) { appendFile(t, root, "docs/LEDGER.md", "\nhand edit\n") },
			want:  "docs/LEDGER.md does not match the ledger",
		},
		{
			name:   "missing status_evidence: a status with nothing to point at",
			mutate: func(f *fixture) { f.entry("docs/guide.md").StatusEvidence = "" },
			want:   "docs/guide.md (DOC-0001): status_evidence is missing",
		},
		{
			name:   "malformed status_evidence",
			mutate: func(f *fixture) { f.entry("docs/guide.md").StatusEvidence = "trust me" },
			want:   `docs/guide.md (DOC-0001): status_evidence "trust me" is not`,
		},
		{
			name:   "status_evidence naming an untracked file",
			mutate: func(f *fixture) { f.entry("docs/guide.md").StatusEvidence = "docs/review/missing.md" },
			want:   `docs/guide.md (DOC-0001): status_evidence "docs/review/missing.md" is not`,
		},
		{
			name: "deleted file whose entry is also deleted: a document silently disappears",
			mutate: func(f *fixture) {
				delete(f.files, "docs/design/reviewed-source-2.md")
				f.ledger.Entries = append(f.ledger.Entries[:6], f.ledger.Entries[7:]...)
			},
			want: "DOC-0007 has no entry: IDs are never deleted",
		},
		{
			name: "removed entry whose file still exists",
			mutate: func(f *fixture) {
				e := f.entry("docs/design/reviewed-source-2.md")
				e.Status, e.ContentNowIn = "removed", "DOC-0006"
			},
			want: "docs/design/reviewed-source-2.md (DOC-0007): marked removed, but the file still exists",
		},
		{
			name: "removed entry done right passes",
			mutate: func(f *fixture) {
				delete(f.files, "docs/design/reviewed-source-2.md")
				e := f.entry("docs/design/reviewed-source-2.md")
				e.Status, e.ContentNowIn, e.StatusEvidence = "removed", "DOC-0006", "PR 2"
			},
			want: "",
		},
		{
			name:   "two entries with one ID: references by ID become unreliable",
			mutate: func(f *fixture) { f.entry("docs/design/reviewed-source-2.md").ID = "DOC-0006" },
			want:   "docs/design/reviewed-source-2.md: id DOC-0006 is also used by docs/design/reviewed-source.md",
		},
		{
			name:   "ID at or above next_id",
			mutate: func(f *fixture) { f.ledger.NextID = 10 },
			want:   "docs/LEDGER.md: id DOC-0010 is not below next_id 10",
		},
		{
			name:   "evidence file under design/: placement drift",
			mutate: func(f *fixture) { f.entry("docs/design/reviewed-source.md").Kind = "evidence" },
			want:   `docs/design/reviewed-source.md (DOC-0006): a evidence does not belong in "docs/design"`,
		},
		{
			name: "new file named TEST-x.md: naming drift",
			mutate: func(f *fixture) {
				f.files["docs/design/TEST-plan.md"] = "# Test plan\n"
				f.ledger.Entries = append(f.ledger.Entries, Entry{ID: "DOC-0011", Path: "docs/design/TEST-plan.md", Kind: "strategy", Status: "proposed", StatusEvidence: "PR 2", Summary: "Tests"})
				f.ledger.NextID = 12
			},
			want: "docs/design/TEST-plan.md (DOC-0011): name must be lowercase letters, digits and hyphens",
		},
		{
			name: "new ADR-016-title.md and README.md are accepted names",
			mutate: func(f *fixture) {
				f.files["docs/design/ADR-016-new-store.md"] = "# ADR-016\n"
				f.files["docs/design/README.md"] = "# Design\n\n" + beginMarker + "\n" + endMarker + "\n"
				f.ledger.Entries = append(f.ledger.Entries,
					Entry{ID: "DOC-0011", Path: "docs/design/ADR-016-new-store.md", Kind: "adr", Status: "proposed", StatusEvidence: "PR 2", Summary: "ADR-016"},
					Entry{ID: "DOC-0012", Path: "docs/design/README.md", Kind: "index", Status: "current", StatusEvidence: "PR 2", Summary: "Design index"})
				f.ledger.NextID = 13
			},
			want: "",
		},
		{
			name: "a date that is not the suffix",
			mutate: func(f *fixture) {
				f.files["docs/testing/2026-11-02-run.md"] = "# Run\n"
				f.ledger.Entries = append(f.ledger.Entries, Entry{ID: "DOC-0011", Path: "docs/testing/2026-11-02-run.md", Kind: "evidence", Status: "historical", StatusEvidence: "PR 2", Summary: "Run"})
				f.ledger.NextID = 12
			},
			want: "a date in a name must be its suffix",
		},
		{
			name:   "malformed ID",
			mutate: func(f *fixture) { f.entry("docs/guide.md").ID = "DOC-1" },
			want:   `docs/guide.md: id "DOC-1" is not DOC- plus four digits`,
		},
		{
			name:   "frozen entry without a hash: the record is not actually protected",
			mutate: func(f *fixture) { f.entry("docs/design/frozen-plan.md").FrozenSHA256 = "" },
			want:   "docs/design/frozen-plan.md (DOC-0003): frozen without frozen_sha256",
		},
		{
			name:   "hash on an entry that is not frozen",
			mutate: func(f *fixture) { f.entry("docs/guide.md").FrozenSHA256 = "ab" },
			want:   "docs/guide.md (DOC-0001): frozen_sha256 is set but status is current",
		},
		{
			name: "removed entry without content_now_in: where the content went is lost",
			mutate: func(f *fixture) {
				delete(f.files, "docs/design/reviewed-source-2.md")
				f.entry("docs/design/reviewed-source-2.md").Status = "removed"
			},
			want: "docs/design/reviewed-source-2.md (DOC-0007): removed entries must say where the content went",
		},
		{
			name: "content_now_in naming no live document",
			mutate: func(f *fixture) {
				delete(f.files, "docs/design/reviewed-source-2.md")
				e := f.entry("docs/design/reviewed-source-2.md")
				e.Status, e.ContentNowIn = "removed", "banana"
			},
			want: `docs/design/reviewed-source-2.md (DOC-0007): content_now_in "banana" is not the ID of a document that still exists`,
		},
		{
			name:   "content_now_in on a live entry",
			mutate: func(f *fixture) { f.entry("docs/guide.md").ContentNowIn = "DOC-0006" },
			want:   "docs/guide.md (DOC-0001): content_now_in is only for removed entries",
		},
		{
			name:   "superseded without naming the replacement",
			mutate: func(f *fixture) { f.entry("docs/design/reviewed-source-2.md").Status = "superseded" },
			want:   "docs/design/reviewed-source-2.md (DOC-0007): superseded entries must name their replacement",
		},
		{
			name: "superseded_by naming no live document",
			mutate: func(f *fixture) {
				e := f.entry("docs/design/reviewed-source-2.md")
				e.Status, e.SupersededBy = "superseded", "DOC-0099"
			},
			want: `docs/design/reviewed-source-2.md (DOC-0007): superseded_by "DOC-0099" is not the ID of a document that still exists`,
		},
		{
			name: "superseded with its replacement passes",
			mutate: func(f *fixture) {
				e := f.entry("docs/design/reviewed-source-2.md")
				e.Status, e.SupersededBy = "superseded", "DOC-0006"
			},
			want: "",
		},
		{
			name:   "planned file promised by an untracked document",
			mutate: func(f *fixture) { f.ledger.Planned[0].PromisedBy = "docs/nowhere.md" },
			want:   `docs/testing/future.md: planned, but promised_by "docs/nowhere.md" is not a tracked file`,
		},
		{
			name:   "data directory entry with no files left",
			mutate: func(f *fixture) { delete(f.files, "docs/testing/data/raw.json") },
			want:   "docs/testing/data/** (DOC-0005): no tracked files under this directory",
		},
		{
			name: "a file covered by a directory entry and a file entry",
			mutate: func(f *fixture) {
				f.ledger.Entries = append(f.ledger.Entries, Entry{ID: "DOC-0011", Path: "docs/testing/data/raw.json", Kind: "evidence-data", Status: "historical", StatusEvidence: "PR 2", Summary: "Raw"})
				f.ledger.NextID = 12
			},
			want: "docs/testing/data/raw.json: covered by more than one entry",
		},
		{
			name: "a new file with a generic prefix",
			mutate: func(f *fixture) {
				f.files["docs/design/roadmap-security.md"] = "# Roadmap\n"
				f.ledger.Entries = append(f.ledger.Entries, Entry{ID: "DOC-0011", Path: "docs/design/roadmap-security.md", Kind: "plan", Status: "proposed", StatusEvidence: "PR 2", Summary: "Roadmap"})
				f.ledger.NextID = 12
			},
			want: `docs/design/roadmap-security.md (DOC-0011): name starts with the generic prefix "roadmap-"`,
		},
		{
			name: "references the check must not follow: examples, URLs, longer paths, local-only paths, planned links",
			mutate: func(f *fixture) {
				f.files[".gitignore"] = "/docs/private/\n"
				f.files["docs/guide.md"] += "\n```\ndocs/testing/example-only.md\n```\n" +
					"\nSee website/content/docs/page.md and https://example.org/docs/remote.md and `docs/private/plan.md`.\n" +
					"\n[Remote](https://example.org/nowhere.md) and [future](testing/future.md).\n"
			},
			want: "",
		},
		{
			name: "repeated headings get -1 anchors",
			mutate: func(f *fixture) {
				f.files["docs/guide.md"] += "\n## Install\n"
				f.files["AGENTS.md"] = "# Agents\n\n[Again](docs/guide.md#install-1).\n"
			},
			want: "",
		},
		{
			name: "an anchor past the last repeat",
			mutate: func(f *fixture) {
				f.files["docs/guide.md"] += "\n## Install\n"
				f.files["AGENTS.md"] = "# Agents\n\n[Again](docs/guide.md#install-2).\n"
			},
			want: `AGENTS.md: anchor "docs/guide.md#install-2" does not resolve`,
		},
		{
			name:   "a bold plain-text path",
			mutate: func(f *fixture) { f.files["docs/guide.md"] += "\nSee **docs/testing/nope.md**.\n" },
			want:   `docs/guide.md: path "docs/testing/nope.md" does not resolve`,
		},
		{
			name:   "a reference-style link",
			mutate: func(f *fixture) { f.files["docs/guide.md"] += "\nSee [the design][d].\n\n[d]: design/nope.md\n" },
			want:   `docs/guide.md: link "docs/design/nope.md" does not resolve`,
		},
		{
			name: "an unused recorded_dangling entry: an exception outlives its reason",
			mutate: func(f *fixture) {
				f.ledger.RecordedDangling = []Dangling{{Source: "docs/testing/run-2026-10-01.md", Target: "docs/review/old.md", Reason: "renamed"}}
			},
			want: `docs/testing/run-2026-10-01.md: recorded_dangling entry for "docs/review/old.md" is unused`,
		},
		{
			name:   "a folder README that is not an index: its list would escape the check",
			mutate: func(f *fixture) { f.entry("docs/testing/README.md").Kind = "strategy" },
			want:   "docs/testing/README.md (DOC-0008): a folder README is an index (kind index)",
		},
		{
			name: "superseded_by naming the entry itself",
			mutate: func(f *fixture) {
				e := f.entry("docs/design/reviewed-source-2.md")
				e.Status, e.SupersededBy = "superseded", "DOC-0007"
			},
			want: "docs/design/reviewed-source-2.md (DOC-0007): superseded_by names the entry itself",
		},
		{
			name:  "an index README whose markers were damaged",
			after: func(t *testing.T, root string) { replaceInFile(t, root, "docs/testing/README.md", endMarker, "") },
			want:  "docs/testing/README.md: needs one " + beginMarker + " followed by one " + endMarker,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := baseFixture()
			if tc.mutate != nil {
				tc.mutate(f)
			}
			root := f.materialize(t)
			if tc.after != nil {
				tc.after(t, root)
			}
			errs := checkRoot(t, root)
			if tc.want == "" {
				if len(errs) > 0 {
					t.Fatalf("want a pass, got:\n%s", strings.Join(errs, "\n"))
				}
				return
			}
			for _, e := range errs {
				if strings.Contains(e, tc.want) {
					return
				}
			}
			t.Fatalf("want an error containing %q, got:\n%s", tc.want, strings.Join(errs, "\n"))
		})
	}
}

func appendFile(t *testing.T, root, p, text string) {
	t.Helper()
	full := filepath.Join(root, p)
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, append(b, text...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func replaceInFile(t *testing.T, root, p, old, new string) {
	t.Helper()
	full := filepath.Join(root, p)
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(old)) {
		t.Fatalf("%s does not contain %q", p, old)
	}
	if err := os.WriteFile(full, bytes.Replace(b, []byte(old), []byte(new), 1), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHistory is the owner test for the history check (plan 6.4): it
// compares with the ledger at the merge base, fails on shallow history, and
// accepts a first ledger only on its own merits.
func TestHistory(t *testing.T) {
	commitLedger := func(t *testing.T, root string, l *Ledger) {
		t.Helper()
		data, _ := json.MarshalIndent(l, "", "  ")
		if err := os.WriteFile(filepath.Join(root, ledgerPath), data, 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "commit", "-q", "--allow-empty", "-am", "change ledger")
	}
	cases := []struct {
		name   string
		base   func(f *fixture) // the ledger at the merge base
		change func(l *Ledger)
		want   string
	}{
		{name: "unchanged ledger passes", change: func(*Ledger) {}, want: ""},
		{
			name: "a former path dropped from previous_paths",
			base: func(f *fixture) {
				f.entry("docs/guide.md").PreviousPaths = []PrevPath{{Path: "docs/old-guide.md", Until: "PR 1"}}
			},
			change: func(l *Ledger) { l.Entries[0].PreviousPaths = nil },
			want:   "history: DOC-0001 dropped its former path docs/old-guide.md from previous_paths",
		},
		{
			name:   "naming_rule_from raised, exempting new files",
			change: func(l *Ledger) { l.NamingRuleFrom = 20 },
			want:   "history: naming_rule_from rose from 9 to 20",
		},
		{
			name:   "naming_rule_from removed",
			change: func(l *Ledger) { l.NamingRuleFrom = 0 },
			want:   "history: naming_rule_from rose from 9 to 0",
		},
		{
			name:   "an ID gone",
			change: func(l *Ledger) { l.Entries = l.Entries[:9] },
			want:   "history: DOC-0010 (docs/LEDGER.md) is gone",
		},
		{
			name:   "a path change without previous_paths: an old link can no longer be traced",
			change: func(l *Ledger) { l.Entries[0].Path = "docs/guide-new.md" },
			want:   "history: DOC-0001 moved from docs/guide.md to docs/guide-new.md without adding the old path to previous_paths",
		},
		{
			name: "a path change recorded in previous_paths passes",
			change: func(l *Ledger) {
				l.Entries[0].Path = "docs/guide-new.md"
				l.Entries[0].PreviousPaths = []PrevPath{{Path: "docs/guide.md", Until: "PR 2"}}
			},
			want: "",
		},
		{
			name: "an ID renumbered",
			change: func(l *Ledger) {
				l.Entries[0].ID = "DOC-0011"
				l.NextID = 12
				l.Entries = append(l.Entries, Entry{ID: "DOC-0001", Path: "docs/x.md"})
			},
			want: "history: docs/guide.md was DOC-0001 and is now DOC-0011",
		},
		{
			name:   "next_id lowered",
			change: func(l *Ledger) { l.NextID = 5 },
			want:   "history: next_id fell from 11 to 5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := baseFixture()
			if tc.base != nil {
				tc.base(f)
			}
			root := f.materialize(t)
			git(t, root, "branch", "-q", "base")
			l := f.ledger
			l.Entries = append([]Entry(nil), f.ledger.Entries...)
			tc.change(&l)
			commitLedger(t, root, &l)
			errs, _ := checkHistory(root, "base", &l)
			if tc.want == "" {
				if len(errs) > 0 {
					t.Fatalf("want a pass, got %v", errs)
				}
				return
			}
			if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, errs)
			}
		})
	}

	t.Run("shallow clone with no merge base fails, never skips", func(t *testing.T) {
		f := baseFixture()
		src := f.materialize(t)
		git(t, src, "branch", "-q", "base")
		git(t, src, "commit", "-q", "--allow-empty", "-m", "second")
		clone := filepath.Join(t.TempDir(), "clone")
		git(t, filepath.Dir(clone), "clone", "-q", "--depth=1", "--no-single-branch", "file://"+src, clone)
		git(t, clone, "branch", "-q", "base", "origin/base")
		errs, note := checkHistory(clone, "base", &f.ledger)
		if len(errs) == 0 || !strings.Contains(errs[0], "no merge base") || note != "" {
			t.Fatalf("want a no-merge-base failure, got errs=%v note=%q", errs, note)
		}
	})

	t.Run("no base ledger: bootstrap, judged on its own merits", func(t *testing.T) {
		// History: the fixture, then a commit without the ledger (the merge
		// base), then the ledger restored on HEAD.
		f := baseFixture()
		root := f.materialize(t)
		git(t, root, "rm", "-q", ledgerPath)
		git(t, root, "commit", "-q", "-m", "before the ledger")
		git(t, root, "branch", "-q", "base")
		git(t, root, "revert", "--no-edit", "HEAD")
		t.Chdir(root)
		var out, errb bytes.Buffer
		if code := run([]string{"-history", "base"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "bootstrap: no base ledger") {
			t.Fatalf("valid first ledger: exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
		}
		// The bootstrap note never excuses the other contracts.
		replaceInFile(t, root, ledgerPath, `"kind": "guide"`, `"kind": "howto"`)
		out.Reset()
		errb.Reset()
		if code := run([]string{"-history", "base"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), `unknown kind "howto"`) {
			t.Fatalf("invalid first ledger: exit %d, stderr %q", code, errb.String())
		}
	})
}

// TestRunExitStatus checks the command's exit status and messages through
// its real entry point.
func TestRunExitStatus(t *testing.T) {
	f := baseFixture()
	root := f.materialize(t)
	t.Chdir(root)
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 0 {
		t.Fatalf("valid tree: exit %d, stderr %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"-find", "DOC-0004"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "docs/testing/run-2026-10-01.md") {
		t.Fatalf("find: exit %d, out %q", code, out.String())
	}
	if code := run([]string{"-find", "DOC-0999"}, &out, &errb); code != 1 {
		t.Fatalf("find unknown: exit %d", code)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/testing/extra.md"), []byte("# Extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "-A")
	errb.Reset()
	if code := run(nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "docs/testing/extra.md: tracked under docs/ but has no ledger entry") {
		t.Fatalf("broken tree: exit %d, stderr %s", code, errb.String())
	}
}
