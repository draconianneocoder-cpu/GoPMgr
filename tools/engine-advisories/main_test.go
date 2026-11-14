// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// repoRoot is the repository root, from this package's directory.
const repoRoot = "../.."

// loopbackOnly is the HTTP client every test uses: it cannot reach anything
// but this machine, so no test depends on, or reaches, NVD.
func loopbackOnly() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
					return nil, fmt.Errorf("test refused a connection to %s", addr)
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
}

func TestTestsCannotReachTheNetwork(t *testing.T) {
	_, err := loopbackOnly().Get(nvdAPI)
	if err == nil || !strings.Contains(err.Error(), "test refused a connection") {
		t.Fatalf("the test client reached %s: %v", nvdAPI, err)
	}
}

// testClient talks to srv without waiting between requests, and records
// the waits it would have made.
func testClient(srv *httptest.Server, waits *[]time.Duration) *nvdClient {
	return &nvdClient{
		http:    loopbackOnly(),
		baseURL: srv.URL,
		delay:   7 * time.Second,
		backoff: []time.Duration{10 * time.Second, 20 * time.Second},
		sleep: func(_ context.Context, d time.Duration) error {
			if waits != nil {
				*waits = append(*waits, d)
			}
			return nil
		},
	}
}

func TestParseAndCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"3.54.1", "3.54.1", 0},
		{"3.54", "3.54.0", 0},
		{"3.54", "3.54.1", -1},
		{"3.54.1.1", "3.54.1", 1},
		{"3.9.0", "3.10.0", -1},
		{"4.20.0", "4.4.2", 1},
	} {
		a, err := parseVersion(tc.a)
		if err != nil {
			t.Fatal(err)
		}
		b, err := parseVersion(tc.b)
		if err != nil {
			t.Fatal(err)
		}
		if got := a.compare(b); got != tc.want {
			t.Errorf("compare(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	for _, bad := range []string{"", "3.54.1a", "3..1", "-1.0", "3.054", "3.54.1-beta", " 3.54"} {
		if _, err := parseVersion(bad); err == nil {
			t.Errorf("parseVersion(%q) accepted it", bad)
		}
	}
}

func TestSplitCPE(t *testing.T) {
	part, vendor, product, v, err := splitCPE(`cpe:2.3:a:sqlite:sqlite:3.54.1:*:*:*:*:*:*:*`)
	if err != nil || part != "a" || vendor != "sqlite" || product != "sqlite" || v != "3.54.1" {
		t.Fatalf("splitCPE = %q %q %q %q %v", part, vendor, product, v, err)
	}
	if _, vendor, _, _, err = splitCPE(`cpe:2.3:a:ex\:ample:p:*:*:*:*:*:*:*:*`); err != nil || vendor != `ex\:ample` {
		t.Fatalf("escaped colon: vendor %q, %v", vendor, err)
	}
	for _, bad := range []string{"cpe:2.3:a:sqlite", "cpe:/a:sqlite:sqlite:3.54.1", "x:2.3:a:b:c:d:e:f:g:h:i:j:k"} {
		if _, _, _, _, err := splitCPE(bad); err == nil {
			t.Errorf("splitCPE(%q) accepted it", bad)
		}
	}
}

var sqliteComponent = component{Name: "SQLite", CPEVendor: "sqlite", CPEProduct: "sqlite", Version: "3.54.1"}

func match(vulnerable bool, criteria string, bounds ...string) nvdCPEMatch {
	m := nvdCPEMatch{Vulnerable: &vulnerable, Criteria: criteria}
	for i := 0; i+1 < len(bounds); i += 2 {
		switch bounds[i] {
		case "si":
			m.VersionStartIncluding = bounds[i+1]
		case "se":
			m.VersionStartExcluding = bounds[i+1]
		case "ei":
			m.VersionEndIncluding = bounds[i+1]
		case "ee":
			m.VersionEndExcluding = bounds[i+1]
		}
	}
	return m
}

func cveWith(ms ...nvdCPEMatch) nvdCVE {
	return nvdCVE{ID: "CVE-2026-0001", Configurations: []nvdConfiguration{{Nodes: []nvdNode{{Operator: "OR", CPEMatch: ms}}}}}
}

const anySQLite = "cpe:2.3:a:sqlite:sqlite:*:*:*:*:*:*:*:*"

func TestApplies(t *testing.T) {
	for _, tc := range []struct {
		name string
		cve  nvdCVE
		want bool
	}{
		// The two shapes NVD returned for 3.54.1 on 2026-11-13.
		{"shell bug through our version", cveWith(match(true, anySQLite, "ei", "3.54.1")), true},
		{"sqlite only as the platform of a PHP bug", nvdCVE{Configurations: []nvdConfiguration{{Operator: "AND", Nodes: []nvdNode{
			{Operator: "OR", CPEMatch: []nvdCPEMatch{match(true, "cpe:2.3:a:php:php:*:*:*:*:*:*:*:*", "se", "8.0.0")}},
			{Operator: "OR", CPEMatch: []nvdCPEMatch{match(false, anySQLite, "si", "3.39.2")}},
		}}}}, false},
		{"vulnerable and AND-ed with a platform", nvdCVE{Configurations: []nvdConfiguration{{Operator: "AND", Nodes: []nvdNode{
			{Operator: "OR", CPEMatch: []nvdCPEMatch{match(true, anySQLite, "ee", "3.60")}},
			{Operator: "OR", CPEMatch: []nvdCPEMatch{match(false, "cpe:2.3:o:linux:linux_kernel:*:*:*:*:*:*:*:*")}},
		}}}}, true},
		{"fixed in our version", cveWith(match(true, anySQLite, "ee", "3.54.1")), false},
		{"fixed before our version", cveWith(match(true, anySQLite, "si", "3.30.0", "ee", "3.50.0")), false},
		{"starts after our version", cveWith(match(true, anySQLite, "se", "3.54.1")), false},
		{"starts at our version", cveWith(match(true, anySQLite, "si", "3.54.1")), true},
		{"exact other version", cveWith(match(true, "cpe:2.3:a:sqlite:sqlite:3.54.0:*:*:*:*:*:*:*")), false},
		{"exact our version with an update field", cveWith(match(true, "cpe:2.3:a:sqlite:sqlite:3.54.1:beta:*:*:*:*:*:*")), true},
		{"version not applicable", cveWith(match(true, "cpe:2.3:a:sqlite:sqlite:-:*:*:*:*:*:*:*")), true},
		{"four-part end bound below", cveWith(match(true, anySQLite, "ei", "3.54.0.9")), false},
		{"four-part end bound above", cveWith(match(true, anySQLite, "ee", "3.54.1.1")), true},
		{"vendor in another case", cveWith(match(true, "cpe:2.3:a:SQLite:SQLite:*:*:*:*:*:*:*:*")), true},
		// Fail closed.
		{"no configurations", nvdCVE{ID: "CVE-2026-0002"}, true},
		{"only another product", cveWith(match(true, "cpe:2.3:a:other:thing:*:*:*:*:*:*:*:*")), true},
		{"negated node", nvdCVE{Configurations: []nvdConfiguration{{Nodes: []nvdNode{{Negate: true, CPEMatch: []nvdCPEMatch{match(true, anySQLite, "ee", "3.0")}}}}}}, true},
		{"negated configuration", nvdCVE{Configurations: []nvdConfiguration{{Negate: true, Nodes: []nvdNode{{CPEMatch: []nvdCPEMatch{match(true, anySQLite, "ee", "3.0")}}}}}}, true},
		{"unreadable bound", cveWith(match(true, anySQLite, "ee", "3.60-rc1")), true},
		{"unreadable criteria version", cveWith(match(true, "cpe:2.3:a:sqlite:sqlite:3.54.x:*:*:*:*:*:*:*")), true},
		{"unreadable criteria", cveWith(match(false, "cpe:2.3:a:sqlite")), true},
		{"vulnerable missing from NVD's entry", cveWith(nvdCPEMatch{Criteria: anySQLite, VersionEndExcluding: "3.60"}), true},
	} {
		if got, why := applies(tc.cve, sqliteComponent); got != tc.want {
			t.Errorf("%s: applies = %v (%s), want %v", tc.name, got, why, tc.want)
		}
	}
}

// writeTree copies the reviewed record, the SBOM metadata and the engine
// files into a temporary root, so a test can change one of them.
func writeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	r, err := loadRecord(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	files := []string{recordPath, sbomPath}
	for f := range r.EngineFiles {
		files = append(files, f)
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(repoRoot, f))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, f, data)
	}
	return root
}

func writeFile(t *testing.T, root, name string, data []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func editFile(t *testing.T, root, name string, edit func(string) string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	changed := edit(string(data))
	if changed == string(data) {
		t.Fatalf("edit of %s changed nothing", name)
	}
	writeFile(t, root, name, []byte(changed))
}

// The offline half of SEC-06, run by `make test`: the vendored engine is
// the one whose advisories were reviewed.
func TestVendoredEngineIsTheReviewedOne(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"check", "-root", repoRoot}, &out, nil, time.Now); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFailsWhenTheEngineIsNotTheReviewedOne(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(t *testing.T, root string)
		want string
	}{
		{"amalgamation changed at the same version", func(t *testing.T, root string) {
			editFile(t, root, "third_party/go-sqlcipher/sqlite3.c", func(s string) string { return s + "\n/* local patch */\n" })
		}, "sqlite3.c changed since the review"},
		{"build flags changed", func(t *testing.T, root string) {
			editFile(t, root, "third_party/go-sqlcipher/flags.go", func(s string) string { return s + "\n// -DSQLITE_TRUSTED_SCHEMA=1\n" })
		}, "flags.go changed since the review"},
		{"version macro differs from the record", func(t *testing.T, root string) {
			editFile(t, root, recordPath, func(s string) string { return strings.Replace(s, `"version": "3.54.1"`, `"version": "3.54.2"`, 1) })
			editFile(t, root, sbomPath, func(s string) string { return strings.ReplaceAll(s, "3.54.1", "3.54.2") })
		}, "defines SQLITE_VERSION as 3.54.1, but the review is of SQLite 3.54.2"},
		{"SBOM names another version", func(t *testing.T, root string) {
			editFile(t, root, sbomPath, func(s string) string { return strings.Replace(s, `"version": "4.20.0"`, `"version": "4.21.0"`, 1) })
		}, "lists SQLCipher 4.21.0"},
		{"SBOM loses an entry", func(t *testing.T, root string) {
			editFile(t, root, sbomPath, func(s string) string {
				return strings.Replace(s, `"name": "SQLCipher"`, `"name": "SQLCipher engine"`, 1)
			})
		}, "no native entry named SQLCipher"},
		{"pinned file missing", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "third_party/go-sqlcipher/gopmgr_rng.c")); err != nil {
				t.Fatal(err)
			}
		}, "read third_party/go-sqlcipher/gopmgr_rng.c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t)
			tc.edit(t, root)
			err := run(context.Background(), []string{"check", "-root", root}, &bytes.Buffer{}, nil, time.Now)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("check = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestRecordValidation(t *testing.T) {
	good, err := loadRecord(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := good.validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(r *record)
		want string
	}{
		{"schema", func(r *record) { r.SchemaVersion = 2 }, "schema_version"},
		{"date", func(r *record) { r.ReviewedOn = "13 Nov 2026" }, "reviewed_on"},
		{"hash", func(r *record) { r.EngineFiles["third_party/go-sqlcipher/sqlite3.h"] = "ABC" }, "lowercase SHA-256"},
		{"version source not pinned", func(r *record) { r.Components[1].VersionSources[0].File = "other.c" }, "does not pin"},
		{"engine file dropped", func(r *record) { delete(r.EngineFiles, "third_party/go-sqlcipher/gopmgr_rng.c") }, "does not pin third_party/go-sqlcipher/gopmgr_rng.c"},
		{"extra engine file", func(r *record) { r.EngineFiles["third_party/go-sqlcipher/sqlite3.go"] = strings.Repeat("a", 64) }, "pins 5 files"},
		{"bad version", func(r *record) { r.Components[0].Version = "3.54.1-1" }, "not dotted"},
		{"duplicate component", func(r *record) { r.Components = append(r.Components, r.Components[0]) }, "listed twice"},
		{"status affected", func(r *record) { r.Dispositions[0].Status = "affected" }, "only disposition"},
		{"no reason", func(r *record) { r.Dispositions[0].Reason = " " }, "no reason"},
		{"no evidence", func(r *record) { r.Dispositions[0].Evidence = nil }, "no evidence"},
		{"unknown component", func(r *record) { r.Dispositions[0].Component = "SQLiteX" }, "unknown component"},
		{"not a CVE ID", func(r *record) { r.Dispositions[0].ID = "GHSA-xxxx" }, "not a CVE ID"},
		{"duplicate disposition", func(r *record) { r.Dispositions = append(r.Dispositions, r.Dispositions[0]) }, "two dispositions"},
	} {
		r, err := loadRecord(repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		tc.edit(&r)
		if err := r.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: validate = %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestRecordRejectsUnknownFields(t *testing.T) {
	root := writeTree(t)
	editFile(t, root, recordPath, func(s string) string {
		return strings.Replace(s, `"reviewed_on"`, `"reviewd_on": "x", "reviewed_on"`, 1)
	})
	if _, err := loadRecord(root); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("loadRecord = %v, want an unknown-field error", err)
	}
}

// fakeNVD serves pages for each virtualMatchString from pages, in order of
// startIndex, after failing the first fails requests with failStatus. A
// query without a version (the scan's control) gets a page listing known
// CVEs, unless unknownProduct is set.
type fakeNVD struct {
	mu             sync.Mutex
	pages          map[string][]nvdPage
	failStatus     int
	fails          int
	body           string
	versionBody    string // served to version queries only
	unknownProduct bool
	requests       []string
}

func (f *fakeNVD) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.URL.RawQuery)
	if f.fails > 0 {
		f.fails--
		w.WriteHeader(f.failStatus)
		return
	}
	if f.body != "" {
		_, _ = w.Write([]byte(f.body))
		return
	}
	cpe := r.URL.Query().Get("virtualMatchString")
	if strings.Count(cpe, ":") == 4 {
		total := 3
		if f.unknownProduct {
			total = 0
		}
		_ = json.NewEncoder(w).Encode(page(0, total))
		return
	}
	if f.versionBody != "" {
		_, _ = w.Write([]byte(f.versionBody))
		return
	}
	start := 0
	if s := r.URL.Query().Get("startIndex"); s != "" {
		_, _ = fmt.Sscan(s, &start)
	}
	for _, p := range f.pages[cpe] {
		if *p.StartIndex == start {
			_ = json.NewEncoder(w).Encode(p)
			return
		}
	}
	_ = json.NewEncoder(w).Encode(page(start, start))
}

func page(start, total int, cves ...nvdCVE) nvdPage {
	perPage := 2000
	p := nvdPage{Format: "NVD_CVE", Version: "2.0", StartIndex: &start, TotalResults: &total, ResultsPerPage: &perPage, Timestamp: "2026-11-13T12:00:00.000"}
	for _, c := range cves {
		p.Vulnerabilities = append(p.Vulnerabilities, struct {
			CVE nvdCVE `json:"cve"`
		}{c})
	}
	return p
}

func named(id, lastModified string, c nvdCVE) nvdCVE {
	c.ID, c.VulnStatus, c.LastModified = id, "Analyzed", lastModified
	return c
}

const (
	sqliteCPE    = "cpe:2.3:a:sqlite:sqlite:3.54.1:*:*:*:*:*:*:*"
	sqlcipherCPE = "cpe:2.3:a:zetetic:sqlcipher:4.20.0:*:*:*:*:*:*:*"
)

var shellBug = named("CVE-2026-119586", "2026-11-01T00:00:00.000", cveWith(match(true, anySQLite, "ei", "3.54.1")))

// scanWith scans the repository's record against f, 18 days after its
// review, so the tests hold whatever the record's date.
func scanWith(t *testing.T, f *fakeNVD, waits *[]time.Duration) (string, error) {
	t.Helper()
	return scanRootAt(t, repoRoot, f, waits, reviewDay(t, repoRoot).AddDate(0, 0, 18))
}

func scanRootAt(t *testing.T, root string, f *fakeNVD, waits *[]time.Duration, at time.Time) (string, error) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	var out bytes.Buffer
	now := func() time.Time { return at }
	err := run(context.Background(), []string{"scan", "-root", root}, &out, testClient(srv, waits), now)
	return out.String(), err
}

// reviewDay is the record's reviewed_on in root.
func reviewDay(t *testing.T, root string) time.Time {
	t.Helper()
	r, err := loadRecord(root)
	if err != nil {
		t.Fatal(err)
	}
	day, err := r.reviewDate()
	if err != nil {
		t.Fatal(err)
	}
	return day
}

func TestScanPassesWhenEveryApplicableAdvisoryHasADisposition(t *testing.T) {
	var waits []time.Duration
	f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 1, shellBug)}, sqlcipherCPE: {page(0, 0)}}}
	out, err := scanWith(t, f, &waits)
	if err != nil {
		t.Fatalf("scan: %v\n%s", err, out)
	}
	reviewed := reviewDay(t, repoRoot).Format(time.DateOnly)
	for _, want := range []string{"Review of " + reviewed + ", 18 days ago (UTC dates); the scan allows 90.", "CVE-2026-119586 (Analyzed): applies", "No advisory applies without a disposition."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if len(f.requests) != 4 || fmt.Sprint(waits) != fmt.Sprint([]time.Duration{7 * time.Second, 7 * time.Second, 7 * time.Second}) {
		t.Errorf("requests %d, waits %v; want 4 requests (a control and a version query per component) spaced by 7s", len(f.requests), waits)
	}
}

func TestScanFailsOnAnApplicableAdvisoryWithoutADisposition(t *testing.T) {
	other := named("CVE-2026-0042", "2026-11-01T00:00:00.000", cveWith(match(true, anySQLite, "ee", "3.55.0")))
	f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 2, shellBug, other)}, sqlcipherCPE: {page(0, 0)}}}
	out, err := scanWith(t, f, nil)
	if err == nil || !strings.Contains(err.Error(), "SQLite CVE-2026-0042") {
		t.Fatalf("scan = %v, want a failure naming CVE-2026-0042\n%s", err, out)
	}
	if errors.Is(err, errNoVerdict) {
		t.Fatalf("a found advisory was reported as no verdict: %v", err)
	}
}

func TestScanFailsOnAnAdvisoryNVDHasNotAnalysed(t *testing.T) {
	waiting := nvdCVE{ID: "CVE-2026-0043", VulnStatus: "Awaiting Analysis"}
	f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 2, shellBug, waiting)}, sqlcipherCPE: {page(0, 0)}}}
	if out, err := scanWith(t, f, nil); err == nil || !strings.Contains(err.Error(), "CVE-2026-0043") {
		t.Fatalf("scan = %v, want a failure naming CVE-2026-0043\n%s", err, out)
	}
}

func TestScanIgnoresAnEntryWhereTheEngineIsOnlyAPlatform(t *testing.T) {
	php := named("CVE-2022-31631", "2025-02-12T00:00:00.000", nvdCVE{Configurations: []nvdConfiguration{{Operator: "AND", Nodes: []nvdNode{
		{Operator: "OR", CPEMatch: []nvdCPEMatch{match(true, "cpe:2.3:a:php:php:*:*:*:*:*:*:*:*", "ee", "8.0.27")}},
		{Operator: "OR", CPEMatch: []nvdCPEMatch{match(false, anySQLite, "si", "3.39.2")}},
	}}}})
	f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 2, php, shellBug)}, sqlcipherCPE: {page(0, 0)}}}
	out, err := scanWith(t, f, nil)
	if err != nil || !strings.Contains(out, "CVE-2022-31631 (Analyzed): does not apply") {
		t.Fatalf("scan = %v\n%s", err, out)
	}
}

func TestScanReadsEveryPage(t *testing.T) {
	late := named("CVE-2026-0044", "2026-11-01T00:00:00.000", cveWith(match(true, anySQLite)))
	f := &fakeNVD{pages: map[string][]nvdPage{
		sqliteCPE:    {page(0, 2, shellBug), page(1, 2, late)},
		sqlcipherCPE: {page(0, 0)},
	}}
	out, err := scanWith(t, f, nil)
	if err == nil || !strings.Contains(err.Error(), "CVE-2026-0044") {
		t.Fatalf("scan = %v, want the CVE on the second page\n%s", err, out)
	}
	if !strings.Contains(strings.Join(f.requests, "\n"), "startIndex=1") {
		t.Fatalf("second page never requested: %v", f.requests)
	}
}

func TestScanWarnsWithoutFailing(t *testing.T) {
	changed := shellBug
	changed.LastModified = "2026-11-20T08:00:00.000"
	f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 1, changed)}, sqlcipherCPE: {page(0, 0)}}}
	out, err := scanWith(t, f, nil)
	if err != nil || !strings.Contains(out, "warning: CVE-2026-119586 changed at NVD on 2026-11-20") {
		t.Fatalf("scan = %v\n%s", err, out)
	}
	f = &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 0)}, sqlcipherCPE: {page(0, 0)}}}
	out, err = scanWith(t, f, nil)
	if err != nil || !strings.Contains(out, "warning: NVD no longer returns CVE-2026-119586") {
		t.Fatalf("scan = %v\n%s", err, out)
	}
}

func TestScanRetriesAServerErrorThenSucceeds(t *testing.T) {
	var waits []time.Duration
	f := &fakeNVD{failStatus: http.StatusServiceUnavailable, fails: 2, pages: map[string][]nvdPage{sqliteCPE: {page(0, 1, shellBug)}, sqlcipherCPE: {page(0, 0)}}}
	if out, err := scanWith(t, f, &waits); err != nil {
		t.Fatalf("scan: %v\n%s", err, out)
	}
	want := []time.Duration{10 * time.Second, 20 * time.Second, 7 * time.Second, 7 * time.Second, 7 * time.Second}
	if fmt.Sprint(waits) != fmt.Sprint(want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
}

func TestScanGivesNoVerdictWhenNVDDoesNotAnswer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		f        *fakeNVD
		requests int
	}{
		{"server errors past every retry", &fakeNVD{failStatus: http.StatusServiceUnavailable, fails: 100}, 3},
		{"refused (NVD's rate limit), not retried", &fakeNVD{failStatus: http.StatusForbidden, fails: 100}, 1},
		{"not JSON", &fakeNVD{body: "<html>maintenance</html>"}, 1},
		// JSON that is not a CVE API page must not read as "no CVEs".
		{"empty object", &fakeNVD{body: "{}"}, 1},
		{"null", &fakeNVD{body: "null"}, 1},
		{"error message", &fakeNVD{body: `{"message":"error"}`}, 1},
		{"page without totalResults", &fakeNVD{body: `{"format":"NVD_CVE","version":"2.0","resultsPerPage":1,"startIndex":0,"timestamp":"2026-11-13T12:00:00.000","vulnerabilities":[{"cve":{"id":"CVE-2026-0001"}}]}`}, 1},
		{"page without timestamp", &fakeNVD{body: `{"format":"NVD_CVE","version":"2.0","resultsPerPage":0,"startIndex":0,"totalResults":0,"vulnerabilities":[]}`}, 1},
		{"more results than the page holds", &fakeNVD{body: `{"format":"NVD_CVE","version":"2.0","resultsPerPage":0,"startIndex":0,"totalResults":5,"timestamp":"t","vulnerabilities":[{"cve":{"id":"CVE-2026-0001"}}]}`}, 1},
		{"empty page short of the total", &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 5)}}}, 2},
		{"product unknown to NVD", &fakeNVD{unknownProduct: true}, 1},
		{"version query answers in another format", &fakeNVD{versionBody: `{"format":"NVD_CPE","version":"2.0","resultsPerPage":0,"startIndex":0,"totalResults":0,"timestamp":"t","vulnerabilities":[]}`}, 2},
		// The control query answers properly; the version query does not.
		{"version query answers {}", &fakeNVD{versionBody: "{}"}, 2},
		{"version query lacks totalResults", &fakeNVD{versionBody: `{"format":"NVD_CVE","version":"2.0","resultsPerPage":1,"startIndex":0,"timestamp":"t","vulnerabilities":[{"cve":{"id":"CVE-2026-0001"}}]}`}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := scanWith(t, tc.f, nil)
			if !errors.Is(err, errNoVerdict) {
				t.Fatalf("scan = %v, want no verdict\n%s", err, out)
			}
			if strings.Contains(out, "No advisory applies") {
				t.Fatalf("a failed query reported a clean result:\n%s", out)
			}
			if len(tc.f.requests) != tc.requests {
				t.Fatalf("requests = %d, want %d", len(tc.f.requests), tc.requests)
			}
		})
	}
}

func TestScanGivesNoVerdictWhenCancelled(t *testing.T) {
	srv := httptest.NewServer(&fakeNVD{})
	t.Cleanup(srv.Close)
	c := testClient(srv, nil)
	c.sleep = func(ctx context.Context, d time.Duration) error { return context.Canceled }
	day := reviewDay(t, repoRoot)
	err := run(context.Background(), []string{"scan", "-root", repoRoot}, &bytes.Buffer{}, c, func() time.Time { return day })
	if !errors.Is(err, errNoVerdict) || errors.Is(err, errReviewAge) {
		t.Fatalf("scan = %v, want no verdict", err)
	}
}

func TestScanWarnsOnADispositionThatNoLongerApplies(t *testing.T) {
	fixed := shellBug
	fixed.Configurations = cveWith(match(true, anySQLite, "ee", "3.54.1")).Configurations
	f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 1, fixed)}, sqlcipherCPE: {page(0, 0)}}}
	out, err := scanWith(t, f, nil)
	if err != nil || !strings.Contains(out, "warning: CVE-2026-119586 no longer applies to SQLite 3.54.1") {
		t.Fatalf("scan = %v\n%s", err, out)
	}
}

func TestScanWarnsOnAChangeOnTheReviewDayOrAnUnreadableDate(t *testing.T) {
	for _, tc := range []struct{ modified, want string }{
		{"2026-11-13T18:00:00.000", "changed at NVD on 2026-11-13T18:00:00.000, on or after the review"},
		{"13/11/2026", `unreadable lastModified "13/11/2026"`},
	} {
		c := shellBug
		c.LastModified = tc.modified
		f := &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 1, c)}, sqlcipherCPE: {page(0, 0)}}}
		out, err := scanWith(t, f, nil)
		if err != nil || !strings.Contains(out, tc.want) {
			t.Fatalf("%s: scan = %v\n%s", tc.modified, err, out)
		}
	}
}

func TestScanRunsCheckFirst(t *testing.T) {
	root := writeTree(t)
	editFile(t, root, "third_party/go-sqlcipher/sqlite3.h", func(s string) string { return s + "\n" })
	f := &fakeNVD{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	err := run(context.Background(), []string{"scan", "-root", root}, &bytes.Buffer{}, testClient(srv, nil), time.Now)
	if err == nil || !strings.Contains(err.Error(), "sqlite3.h changed since the review") {
		t.Fatalf("scan = %v, want the check failure", err)
	}
	if len(f.requests) != 0 {
		t.Fatalf("scan queried NVD %d times for an engine it had not checked", len(f.requests))
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := run(context.Background(), []string{"audit", "-root", repoRoot}, &bytes.Buffer{}, nil, time.Now); err == nil {
		t.Fatal("an unknown command succeeded")
	}
	if err := run(context.Background(), nil, &bytes.Buffer{}, nil, time.Now); err == nil {
		t.Fatal("no command succeeded")
	}
}

func TestReviewAgeCountsUTCCalendarDays(t *testing.T) {
	reviewed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	east := time.FixedZone("UTC+14", 14*60*60)
	cases := []struct {
		name string
		now  time.Time
		want int
	}{
		{"the review day", reviewed, 0},
		{"its last second", reviewed.Add(24*time.Hour - time.Second), 0},
		{"an hour before it", reviewed.Add(-time.Hour), -1},
		{"day 90, last second", reviewed.AddDate(0, 0, 91).Add(-time.Second), 90},
		{"day 91, first second", reviewed.AddDate(0, 0, 91), 91},
		{"a zone east of UTC still on the review day there", time.Date(2026, 1, 2, 9, 0, 0, 0, east), 0},
	}
	for _, tc := range cases {
		if got := reviewAgeDays(reviewed, tc.now); got != tc.want {
			t.Errorf("%s: reviewAgeDays = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The release gate's half of DEC-ADVISORY-SCAN: a review more than 90 UTC
// days old, or dated after today, fails the scan, which still queries NVD
// and reports what it found; its error joins any other.
func TestScanFailsOnAReviewOutOfDate(t *testing.T) {
	const reviewedOn = "2026-01-01"
	reviewed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	other := named("CVE-2026-0042", "2025-11-01T00:00:00.000", cveWith(match(true, anySQLite, "ee", "3.55.0")))
	clean := func() *fakeNVD {
		return &fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 1, shellBug)}, sqlcipherCPE: {page(0, 0)}}}
	}
	cases := []struct {
		name          string
		at            time.Time
		nvd           *fakeNVD
		wantAge       bool
		wantOther     error  // errNoVerdict, or nil
		wantUndispose string // an advisory the error also names
	}{
		{"reviewed today", reviewed, clean(), false, nil, ""},
		{"day 90, last second", reviewed.AddDate(0, 0, 91).Add(-time.Second), clean(), false, nil, ""},
		{"day 91", reviewed.AddDate(0, 0, 91), clean(), true, nil, ""},
		{"an hour before the review day", reviewed.Add(-time.Hour), clean(), true, nil, ""},
		{"day 91 and an advisory without a disposition", reviewed.AddDate(0, 0, 91),
			&fakeNVD{pages: map[string][]nvdPage{sqliteCPE: {page(0, 2, shellBug, other)}, sqlcipherCPE: {page(0, 0)}}}, true, nil, "SQLite CVE-2026-0042"},
		{"day 91 and no answer from NVD", reviewed.AddDate(0, 0, 91), &fakeNVD{failStatus: http.StatusForbidden, fails: 1}, true, errNoVerdict, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t)
			editFile(t, root, recordPath, func(s string) string {
				return regexp.MustCompile(`"reviewed_on": "[0-9-]+"`).ReplaceAllString(s, `"reviewed_on": "`+reviewedOn+`"`)
			})
			out, err := scanRootAt(t, root, tc.nvd, nil, tc.at)
			if got := errors.Is(err, errReviewAge); got != tc.wantAge {
				t.Fatalf("scan = %v; out of date: %v, want %v\n%s", err, got, tc.wantAge, out)
			}
			if tc.wantOther != nil && !errors.Is(err, tc.wantOther) {
				t.Fatalf("scan = %v, want it to include %v", err, tc.wantOther)
			}
			if tc.wantUndispose != "" && (err == nil || !strings.Contains(err.Error(), tc.wantUndispose)) {
				t.Fatalf("scan = %v, want it to name %s", err, tc.wantUndispose)
			}
			if !tc.wantAge && tc.wantOther == nil && tc.wantUndispose == "" && err != nil {
				t.Fatalf("scan = %v, want a pass\n%s", err, out)
			}
			passed := strings.Contains(out, "No advisory applies without a disposition.")
			if passed != (err == nil) {
				t.Fatalf("success line printed: %v, but scan = %v\n%s", passed, err, out)
			}
			if tc.nvd.pages != nil && !strings.Contains(out, "CVE-2026-119586") {
				t.Fatalf("an out-of-date review must still report NVD's results:\n%s", out)
			}
		})
	}
}

// The other half: the offline check, run by make test and CI, never fails
// because the review has aged, so main does not go red on its own.
func TestCheckIgnoresTheReviewsAge(t *testing.T) {
	root := writeTree(t)
	editFile(t, root, recordPath, func(s string) string {
		return regexp.MustCompile(`"reviewed_on": "[0-9-]+"`).ReplaceAllString(s, `"reviewed_on": "2000-01-01"`)
	})
	var out bytes.Buffer
	if err := run(context.Background(), []string{"check", "-root", root}, &out, nil, time.Now); err != nil {
		t.Fatalf("check with a review from 2000 = %v, want a pass", err)
	}
}
