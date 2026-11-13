// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// nvdAPI is NVD's CVE API 2.0. Its documentation was not reachable when
// this tool was written (2026-11-13); the behaviour relied on here was
// observed instead: virtualMatchString with a full CPE returns CVEs whose
// configurations cover that version, including through version ranges, and
// narrows by version (zetetic:sqlcipher returned 2 CVEs without a version
// and 0 at 4.20.0). docs/engineering/engine-advisories.md keeps the control
// queries that re-confirm it.
const nvdAPI = "https://services.nvd.nist.gov/rest/json/cves/2.0"

const (
	maxResponseBytes = 32 << 20
	maxPages         = 20
)

// nvdClient queries NVD one request at a time. Without an API key NVD
// allows about five requests in 30 seconds (not confirmed from its
// documentation), so requests are spaced by delay.
type nvdClient struct {
	http    *http.Client
	baseURL string
	delay   time.Duration
	// backoff is the wait before each retry; its length is the number of
	// retries after a 5xx response or a failed connection.
	backoff []time.Duration
	sleep   func(context.Context, time.Duration) error
	sent    bool
}

func newNVDClient() *nvdClient {
	return &nvdClient{
		http:    &http.Client{Timeout: 90 * time.Second},
		baseURL: nvdAPI,
		delay:   7 * time.Second,
		backoff: []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second},
		sleep:   sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// errNoVerdict marks a failure to get NVD's answer: the scan decided
// nothing, which is not the same as finding an advisory.
var errNoVerdict = errors.New("no verdict: NVD did not answer")

type nvdPage struct {
	ResultsPerPage  int    `json:"resultsPerPage"`
	StartIndex      int    `json:"startIndex"`
	TotalResults    int    `json:"totalResults"`
	Timestamp       string `json:"timestamp"`
	Vulnerabilities []struct {
		CVE nvdCVE `json:"cve"`
	} `json:"vulnerabilities"`
}

// query is one component's result: every CVE NVD returned for the CPE, and
// the requests that produced them.
type query struct {
	URLs      []string
	Timestamp string
	CVEs      []nvdCVE
}

// cvesFor returns every CVE NVD matches to cpe, reading all pages.
func (n *nvdClient) cvesFor(ctx context.Context, cpe string) (query, error) {
	var q query
	seen := map[string]bool{}
	start := 0
	for page := 0; ; page++ {
		if page == maxPages {
			return q, fmt.Errorf("%w: more than %d pages for %s", errNoVerdict, maxPages, cpe)
		}
		params := url.Values{"virtualMatchString": {cpe}}
		if start > 0 {
			params.Set("startIndex", strconv.Itoa(start))
		}
		u := n.baseURL + "?" + params.Encode()
		q.URLs = append(q.URLs, u)
		p, err := n.get(ctx, u)
		if err != nil {
			return q, err
		}
		if p.StartIndex != start || p.TotalResults < 0 {
			return q, fmt.Errorf("%w: page for %s starts at %d, asked for %d", errNoVerdict, cpe, p.StartIndex, start)
		}
		if q.Timestamp == "" {
			q.Timestamp = p.Timestamp
		}
		for _, v := range p.Vulnerabilities {
			if v.CVE.ID == "" {
				return q, fmt.Errorf("%w: a result for %s has no CVE ID", errNoVerdict, cpe)
			}
			if !seen[v.CVE.ID] {
				seen[v.CVE.ID] = true
				q.CVEs = append(q.CVEs, v.CVE)
			}
		}
		start += len(p.Vulnerabilities)
		if start >= p.TotalResults {
			return q, nil
		}
		if len(p.Vulnerabilities) == 0 {
			return q, fmt.Errorf("%w: empty page at %d of %d for %s", errNoVerdict, start, p.TotalResults, cpe)
		}
	}
}

// get fetches one page, waiting delay after any earlier request and
// retrying a 5xx response or a failed connection. Any other status (NVD
// may answer a rate-limited client with 403) fails at once.
func (n *nvdClient) get(ctx context.Context, u string) (nvdPage, error) {
	var lastErr error
	for attempt := 0; attempt <= len(n.backoff); attempt++ {
		wait := n.delay
		if attempt > 0 {
			wait = n.backoff[attempt-1]
		}
		if n.sent {
			if err := n.sleep(ctx, wait); err != nil {
				return nvdPage{}, err
			}
		}
		n.sent = true
		p, retry, err := n.fetch(ctx, u)
		if err == nil {
			return p, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return nvdPage{}, fmt.Errorf("%w: %v", errNoVerdict, lastErr)
}

func (n *nvdClient) fetch(ctx context.Context, u string) (page nvdPage, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return page, false, err
	}
	resp, err := n.http.Do(req)
	if err != nil {
		return page, ctx.Err() == nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return page, resp.StatusCode >= 500, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return page, true, fmt.Errorf("GET %s: %w", u, err)
	}
	if len(body) > maxResponseBytes {
		return page, false, fmt.Errorf("GET %s: response larger than %d bytes", u, maxResponseBytes)
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return page, false, fmt.Errorf("GET %s: %w", u, err)
	}
	return page, false, nil
}
