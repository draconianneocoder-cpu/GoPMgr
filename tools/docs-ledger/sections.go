// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Section is a record merged into a combined file, or appended to a living
// document, by the reorganization (plan section 5.2). Its text must stay as
// written: only its heading level and its link targets may differ from the
// source it came from.
type Section struct {
	Title    string `json:"title"`              // the source's own title, its first heading
	From     string `json:"from"`               // the source's former path
	Commit   string `json:"commit"`             // the commit the source was taken from
	Date     string `json:"date"`               // the date the record states, or "undated"
	Appendix bool   `json:"appendix,omitempty"` // appended to a living document, not combined
	SHA256   string `json:"sha256"`             // of the normalized record (normalizeRecord)
}

const appendixPrefix = "Superseded record, not maintained: "

// mergeNote is the fixed note every combined file carries after its
// one-sentence description (plan section 5.2).
const mergeNote = "Each section below is a record kept as written, with its headings one\nlevel lower and its links updated; nothing else changed."

var (
	commentHeader = regexp.MustCompile(`(?s)\A<!--.*?-->\s*`)
	refDefPattern = regexp.MustCompile(`(?m)^([ \t]{0,3}\[[^\]]+\]:[ \t]*)\S+`)
	inlineTarget  = regexp.MustCompile(`\]\([^)\s]+((?:\s+"[^"]*")?)\)`)
	holdsLine     = regexp.MustCompile("^- \\[([^\\]]+)\\]\\(#([^)]+)\\): formerly `([^`]+)`, (undated|\\d{4}-\\d{2}-\\d{2}), from commit `([0-9a-f]{7,40})`\\.$")
	formerlyLine  = regexp.MustCompile("^Formerly `([^`]+)`, (undated|\\d{4}-\\d{2}-\\d{2}), from commit `([0-9a-f]{7,40})`\\.$")
)

// findSection returns a record's text in a file: from its heading, which is
// "## <title>" or "## Superseded record, not maintained: <title>", up to the
// next level-2 heading outside a code fence.
func findSection(text, title string) (string, bool, bool) {
	lines := strings.SplitAfter(text, "\n")
	start, appendix := -1, false
	fence := false
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		if t := strings.TrimSpace(trimmed); strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if fence || !strings.HasPrefix(trimmed, "## ") {
			continue
		}
		if start >= 0 {
			return strings.Join(lines[start:i], ""), appendix, true
		}
		switch strings.TrimPrefix(trimmed, "## ") {
		case title:
			start = i
		case appendixPrefix + title:
			start, appendix = i, true
		}
	}
	if start < 0 {
		return "", false, false
	}
	return strings.Join(lines[start:], ""), appendix, true
}

// normalizeRecord turns a merged section back into the form its source had,
// apart from link targets: its heading becomes "# <title>", an appendix's
// "Formerly" line goes, every other heading moves up one level, and link
// targets and docs/ paths are masked.
func normalizeRecord(section, title string, appendix bool) string {
	lines := strings.Split(strings.TrimRight(section, "\n"), "\n")
	lines[0] = "# " + title
	if appendix && len(lines) > 2 && lines[1] == "" && formerlyLine.MatchString(lines[2]) {
		lines = append(lines[:1], lines[3:]...)
	}
	fence := false
	for i := 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if !fence && strings.HasPrefix(lines[i], "##") {
			lines[i] = lines[i][1:]
		}
	}
	return maskTargets(strings.Join(lines, "\n")) + "\n"
}

// normalizeSource is the same form for the original file: its comment header
// goes and link targets and docs/ paths are masked.
func normalizeSource(text string) string {
	text = commentHeader.ReplaceAllString(text, "")
	return maskTargets(strings.TrimRight(text, "\n")) + "\n"
}

func maskTargets(s string) string {
	s = inlineTarget.ReplaceAllString(s, "](<target>$1)")
	s = refDefPattern.ReplaceAllString(s, "$1<target>")
	return pathMention.ReplaceAllString(s, "<path>")
}

// checkSections runs contract 3b offline: every recorded section is still
// in its file, unchanged (by hash of its normalized form), and named by its
// provenance line; a combined file has nothing at its top but its title, a
// one-sentence description, the fixed note and the list of what it holds.
func checkSections(l *Ledger, r *repo) []string {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	for _, e := range l.Entries {
		if len(e.Sections) == 0 {
			continue
		}
		b, err := r.read(e.Path)
		if err != nil {
			continue // contract 1 reports a missing file
		}
		text := string(b)
		combined := false
		for _, s := range e.Sections {
			sec, appendix, ok := findSection(text, s.Title)
			if !ok {
				add("%s (%s): section %q is missing", e.Path, e.ID, s.Title)
				continue
			}
			combined = combined || !appendix
			if appendix != s.Appendix {
				add("%s (%s): section %q is recorded as appendix=%t but its heading says otherwise", e.Path, e.ID, s.Title, s.Appendix)
			}
			if got := sha256Hex([]byte(normalizeRecord(sec, s.Title, appendix))); got != s.SHA256 {
				add("%s (%s): section %q changed (sha256 %s, ledger %s); merged records stay as written", e.Path, e.ID, s.Title, got, s.SHA256)
			}
			if appendix {
				lines := strings.Split(sec, "\n")
				if m := formerlyMatch(lines); m == nil || m[1] != s.From || m[2] != s.Date || m[3] != s.Commit {
					add("%s (%s): appendix %q must open with its Formerly line naming %s", e.Path, e.ID, s.Title, s.From)
				}
			}
		}
		heads := levelTwoHeadings(text)
		if combined {
			errs = append(errs, checkCombinedHeader(e, text)...)
			var want []string
			for _, s := range e.Sections {
				want = append(want, s.Title)
			}
			if i := indexOf(heads, "What this file holds"); i < 0 || strings.Join(heads[i+1:], "\n") != strings.Join(want, "\n") {
				add("%s (%s): after \"What this file holds\", the sections must be exactly the recorded ones, in order: %s", e.Path, e.ID, strings.Join(want, "; "))
			}
		} else if len(heads) == 0 || heads[len(heads)-1] != appendixPrefix+e.Sections[len(e.Sections)-1].Title {
			add("%s (%s): an appendix must be the document's last section", e.Path, e.ID)
		}
	}
	return errs
}

// levelTwoHeadings lists a file's "## " headings outside code fences.
func levelTwoHeadings(text string) []string {
	var out []string
	fence := false
	for _, line := range strings.Split(text, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if !fence && strings.HasPrefix(line, "## ") {
			out = append(out, strings.TrimPrefix(line, "## "))
		}
	}
	return out
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// checkCombinedHeader enforces the top of a combined file (plan 5.2).
func checkCombinedHeader(e Entry, text string) []string {
	where := fmt.Sprintf("%s (%s)", e.Path, e.ID)
	body := commentHeader.ReplaceAllString(text, "")
	parts := strings.SplitN(body, "\n## ", 3)
	if len(parts) < 3 {
		return []string{where + ": a combined file needs its title, description, note, what-it-holds list and sections"}
	}
	head := strings.Split(strings.TrimRight(parts[0], "\n"), "\n\n")
	if len(head) != 3 || !strings.HasPrefix(head[0], "# ") || strings.Contains(head[0], "\n") ||
		strings.Contains(head[1], "\n") || head[2] != mergeNote {
		return []string{where + ": the top of a combined file is its title, a one-sentence description and the fixed note, nothing else"}
	}
	holds := strings.Split(strings.TrimRight(parts[1], "\n"), "\n")
	if holds[0] != "What this file holds" || len(holds) != len(e.Sections)+2 || holds[1] != "" {
		return []string{where + ": \"What this file holds\" must list exactly one line per section"}
	}
	var errs []string
	for i, s := range e.Sections {
		m := holdsLine.FindStringSubmatch(holds[i+2])
		if m == nil || m[1] != s.Title || m[2] != slugify(s.Title) || m[3] != s.From || m[4] != s.Date || m[5] != s.Commit {
			errs = append(errs, fmt.Sprintf("%s: line %d of \"What this file holds\" must name %q, formerly %s, %s, from commit %s", where, i+1, s.Title, s.From, s.Date, s.Commit))
		}
	}
	return errs
}

func formerlyMatch(lines []string) []string {
	if len(lines) < 3 {
		return nil
	}
	return formerlyLine.FindStringSubmatch(lines[2])
}

// sourceErrors compares each recorded section with its source at its commit
// (the history check; it needs that commit).
func sourceErrors(l *Ledger, r *repo, show func(rev, path string) (string, error)) []string {
	var errs []string
	for _, e := range l.Entries {
		b, err := r.read(e.Path)
		if err != nil {
			continue
		}
		for _, s := range e.Sections {
			sec, appendix, ok := findSection(string(b), s.Title)
			if !ok {
				continue // checkSections reports it
			}
			src, err := show(s.Commit, s.From)
			if err != nil {
				errs = append(errs, fmt.Sprintf("history: cannot read %s at %s for %s: %v; fetch more history and run again", s.From, s.Commit, e.Path, err))
				continue
			}
			if normalizeRecord(sec, s.Title, appendix) != normalizeSource(src) {
				errs = append(errs, fmt.Sprintf("history: section %q of %s differs from %s at %s beyond its heading levels and link targets", s.Title, e.Path, s.From, s.Commit))
				continue
			}
			errs = append(errs, retargetErrors(l, e.Path, s, sec, src)...)
		}
	}
	return errs
}

var (
	targetPattern  = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)|(?m)^[ \t]{0,3}\[[^\]]+\]:[ \t]*(\S+)`)
	mentionPattern = regexp.MustCompile(`docs/[A-Za-z0-9_./-]+\.(?:md|txt|json|jsonl|csv|docx)(#[A-Za-z0-9_-]+)?`)
)

// target is where a link or docs/ path points: a URL kept literally, or a
// repository path (self for an anchor in the same file) and its anchor.
type target struct {
	url, path, frag string
}

// targets lists a text's link targets, then its docs/ paths, in order.
func targets(text, dir, self string) []target {
	var out []target
	for _, m := range targetPattern.FindAllStringSubmatch(text, -1) {
		t := m[1] + m[2]
		if schemePattern.MatchString(t) {
			out = append(out, target{url: t})
			continue
		}
		p, frag, _ := strings.Cut(t, "#")
		if p == "" {
			out = append(out, target{path: self, frag: frag})
			continue
		}
		out = append(out, target{path: path.Clean(path.Join(dir, p)), frag: frag})
	}
	for _, m := range mentionPattern.FindAllString(text, -1) {
		p, frag, _ := strings.Cut(strings.TrimRight(m, "."), "#")
		out = append(out, target{path: p, frag: frag})
	}
	return out
}

// retargetErrors checks that every link target and docs/ path in a merged
// record reaches what its source's did. A URL must be unchanged. A target
// whose file did not move keeps its anchor. A target whose file moved must
// now point to the file that holds its content (its previous_paths, or a
// removed entry pointing to it): to the same anchor, or, if it had none and
// was merged into a section, to that section's heading.
func retargetErrors(l *Ledger, file string, s Section, sec, src string) []string {
	holds := map[string]string{}     // former path -> current path
	sectionOf := map[string]string{} // former path -> anchor of the section holding it
	idPath := map[string]string{}
	for _, e := range l.Entries {
		idPath[e.ID] = e.Path
	}
	for _, e := range l.Entries {
		for _, p := range e.PreviousPaths {
			holds[p.Path] = e.Path
		}
		if e.Status == "removed" && idPath[e.ContentNowIn] != "" {
			holds[e.Path] = idPath[e.ContentNowIn]
		}
		for _, sc := range e.Sections {
			heading := sc.Title
			if sc.Appendix {
				heading = appendixPrefix + sc.Title
			}
			sectionOf[sc.From] = slugify(heading)
		}
	}
	got, want := targets(sec, path.Dir(file), file), targets(src, path.Dir(s.From), s.From)
	var errs []string
	bad := func(o, n target) {
		show := func(t target) string {
			if t.url != "" {
				return t.url
			}
			if t.frag != "" {
				return t.path + "#" + t.frag
			}
			return t.path
		}
		errs = append(errs, fmt.Sprintf("history: section %q of %s points to %s where its source pointed to %s; a merged record's links may only follow their target's move", s.Title, file, show(n), show(o)))
	}
	for i := range got {
		if i >= len(want) {
			break
		}
		o, n := want[i], got[i]
		switch {
		case o.url != "" || n.url != "":
			if o.url != n.url {
				bad(o, n)
			}
		case o.path == n.path:
			if o.frag != n.frag {
				bad(o, n)
			}
		case holds[o.path] == n.path:
			wantFrag := o.frag
			if wantFrag == "" {
				wantFrag = sectionOf[o.path]
			}
			if n.frag != wantFrag {
				bad(o, n)
			}
		default:
			bad(o, n)
		}
	}
	return errs
}
