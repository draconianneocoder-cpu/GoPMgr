// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// linkSources outside docs/ whose links the check also follows (plan 6.4).
var extraLinkSources = []string{"AGENTS.md", "README.md", "code-map/recent-decisions.md"}

var (
	fencePattern    = regexp.MustCompile("(?ms)^[ \t]*(```|~~~).*?^[ \t]*(```|~~~)[^\n]*$")
	codeSpanPattern = regexp.MustCompile("`[^`\n]*`")
	mdLinkPattern   = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	refLinkPattern  = regexp.MustCompile(`(?m)^[ \t]{0,3}\[[^\]]+\]:[ \t]*(\S+)`)
	pathMention     = regexp.MustCompile(`docs/[A-Za-z0-9_./-]+\.(?:md|txt|json|jsonl|csv|docx)`)
	headingPattern  = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t#]*$`)
	htmlIDPattern   = regexp.MustCompile(`(?:id|name)="([^"]+)"`)
	schemePattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// checkLinks runs contract 4: relative links and plain-text docs/ paths in
// scope resolve, are planned, or are recorded as dangling from a source that
// may not be edited.
func checkLinks(l *Ledger, r *repo) []string {
	byPath := map[string]Entry{}
	for _, e := range l.Entries {
		byPath[e.Path] = e
	}
	// A document may name its own former paths in plain text: the paths in
	// its previous_paths, and those of removed entries whose content it now
	// holds. Links to former paths, and mentions anywhere else, still fail.
	idPath := map[string]string{}
	for _, e := range l.Entries {
		idPath[e.ID] = e.Path
	}
	former := map[[2]string]bool{}
	for _, e := range l.Entries {
		for _, p := range e.PreviousPaths {
			former[[2]string{e.Path, p.Path}] = true
		}
		if e.Status == "removed" && idPath[e.ContentNowIn] != "" {
			former[[2]string{idPath[e.ContentNowIn], e.Path}] = true
		}
	}
	planned := map[string]bool{}
	for _, p := range l.Planned {
		planned[p.Path] = true
	}
	dangling := map[[2]string]bool{}
	used := map[[2]string]bool{}
	var errs []string
	for _, d := range l.RecordedDangling {
		dangling[[2]string{d.Source, d.Target}] = true
		e, ok := byPath[d.Source]
		if !ok || (e.Status != "frozen" && e.Status != "historical" && e.Governed == "") {
			errs = append(errs, fmt.Sprintf("%s: recorded_dangling is only for frozen, historical or governed sources; fix the reference instead", d.Source))
		}
	}

	var sources []string
	for f := range r.tracked {
		// LEDGER.md is generated from validated entries and lists removed
		// and planned paths by design; contract 5 checks it instead.
		if strings.HasPrefix(f, "docs/") && strings.HasSuffix(f, ".md") && byPath[f].Status != "frozen" && f != ledgerViewPath {
			sources = append(sources, f)
		}
	}
	for _, f := range extraLinkSources {
		if r.tracked[f] {
			sources = append(sources, f)
		}
	}
	sort.Strings(sources)

	anchors := map[string]map[string]bool{}
	anchorsOf := func(p string) map[string]bool {
		if a, ok := anchors[p]; ok {
			return a
		}
		b, err := r.read(p)
		a := map[string]bool{}
		if err == nil {
			a = headingAnchors(string(b))
		}
		anchors[p] = a
		return a
	}

	for _, src := range sources {
		b, err := r.read(src)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: cannot read: %v", src, err))
			continue
		}
		text := string(b)
		report := func(target, what string) {
			if dangling[[2]string{src, target}] {
				used[[2]string{src, target}] = true
				return
			}
			if planned[target] || (r.ignored != nil && r.ignored(target)) {
				return
			}
			errs = append(errs, fmt.Sprintf("%s: %s %q does not resolve", src, what, target))
		}

		prose := codeSpanPattern.ReplaceAllString(fencePattern.ReplaceAllString(text, ""), "")
		links := mdLinkPattern.FindAllStringSubmatch(prose, -1)
		links = append(links, refLinkPattern.FindAllStringSubmatch(prose, -1)...)
		for _, m := range links {
			link := strings.Trim(m[1], "<>")
			if schemePattern.MatchString(link) {
				continue
			}
			target, frag, _ := strings.Cut(link, "#")
			resolved := src
			if target != "" {
				resolved = path.Clean(path.Join(path.Dir(src), target))
				if !r.exists(resolved) {
					report(resolved, "link")
					continue
				}
			}
			if frag != "" && strings.HasSuffix(resolved, ".md") && !anchorsOf(resolved)[strings.ToLower(frag)] {
				report(resolved+"#"+frag, "anchor")
			}
		}

		// Fenced blocks hold examples, not references, so only prose counts.
		unfenced := fencePattern.ReplaceAllString(text, "")
		for _, loc := range pathMention.FindAllStringIndex(unfenced, -1) {
			if inURL(unfenced, loc[0]) {
				continue
			}
			target := strings.TrimRight(unfenced[loc[0]:loc[1]], ".")
			ownFormer := former[[2]string{src, target}] && provenanceLine(unfenced, loc[0], target)
			if !r.exists(target) && !ownFormer {
				report(target, "path")
			}
		}
	}
	for _, d := range l.RecordedDangling {
		if !used[[2]string{d.Source, d.Target}] {
			errs = append(errs, fmt.Sprintf("%s: recorded_dangling entry for %q is unused; remove it", d.Source, d.Target))
		}
	}
	return errs
}

// provenanceLine reports whether the mention at i sits on a line that
// records where a merged record came from ("formerly `<path>`" or
// "Formerly `<path>`"), the only place a document may name a former path.
func provenanceLine(text string, i int, target string) bool {
	start := strings.LastIndex(text[:i], "\n") + 1
	end := strings.Index(text[i:], "\n")
	if end < 0 {
		end = len(text) - i
	}
	return strings.Contains(strings.ToLower(text[start:i+end]), "formerly `"+strings.ToLower(target)+"`")
}

// inURL reports whether the match at i is part of a URL or a longer path,
// such as website/content/docs/...: it looks back to the start of the token.
func inURL(text string, i int) bool {
	start := i
	for start > 0 && !strings.ContainsRune(" \t\n(<\"'`[|*", rune(text[start-1])) {
		start--
	}
	return start != i
}

// headingAnchors returns the anchors GitHub generates for a Markdown file's
// headings, with -1, -2 for repeats, plus explicit HTML id and name values.
func headingAnchors(text string) map[string]bool {
	out := map[string]bool{}
	text = fencePattern.ReplaceAllString(text, "")
	count := map[string]int{}
	for _, m := range headingPattern.FindAllStringSubmatch(text, -1) {
		slug := slugify(m[1])
		if n := count[slug]; n > 0 {
			out[fmt.Sprintf("%s-%d", slug, n)] = true
		} else {
			out[slug] = true
		}
		count[slug]++
	}
	for _, m := range htmlIDPattern.FindAllStringSubmatch(text, -1) {
		out[strings.ToLower(m[1])] = true
	}
	return out
}

var (
	slugLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	slugTagPattern  = regexp.MustCompile(`<[^>]+>`)
)

func slugify(heading string) string {
	h := slugLinkPattern.ReplaceAllString(heading, "$1")
	h = slugTagPattern.ReplaceAllString(h, "")
	var b strings.Builder
	for _, c := range strings.ToLower(h) {
		switch {
		case c == ' ':
			b.WriteRune('-')
		case c == '-' || c == '_' || unicode.IsLetter(c) || unicode.IsNumber(c):
			b.WriteRune(c)
		}
	}
	return b.String()
}
