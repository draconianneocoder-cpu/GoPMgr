// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// PreviewRows is the shipped ImportTabular row cap.
const PreviewRows = 10_000

// Entry is one manifest record.
type Entry struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"` // "portfolio" or "tabular"
	File      string   `json:"file"`
	Format    string   `json:"format,omitempty"`
	Class     Class    `json:"class"`
	Accept    Accept   `json:"accept,omitempty"`
	Note      string   `json:"note,omitempty"`
	Large     bool     `json:"large,omitempty"`
	Bytes     int64    `json:"bytes"`
	SHA256    string   `json:"sha256"`
	TotalRows int      `json:"total_rows"`
	Columns   []string `json:"columns,omitempty"`
	// ExpectedSHA256 digests the canonical expected rows (at most
	// PreviewRows of them, one JSON array per line), so the truth itself is
	// pinned without storing it.
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
}

// Manifest is fixtures/manifest.json.
type Manifest struct {
	Generator string  `json:"generator"`
	Entries   []Entry `json:"entries"`
}

// portfolioDoc is the engine-neutral portfolio input file. Floats are
// strings so NaN and ±Inf are exact; minor units are strings so no JSON
// reader rounds a value beyond 2^53.
type portfolioDoc struct {
	ID       string            `json:"id"`
	Projects []portfolioRecord `json:"projects"`
}

type portfolioRecord struct {
	ProjectID               string `json:"project_id"`
	Name                    string `json:"name"`
	BudgetedCost            string `json:"budgeted_cost"`
	CommittedCost           string `json:"committed_cost"`
	ActualCost              string `json:"actual_cost"`
	EarnedValue             string `json:"earned_value"`
	PlannedValue            string `json:"planned_value"`
	BudgetedCostMinorUnits  string `json:"budgeted_cost_minor_units"`
	CommittedCostMinorUnits string `json:"committed_cost_minor_units"`
	ActualCostMinorUnits    string `json:"actual_cost_minor_units"`
	EarnedValueMinorUnits   string `json:"earned_value_minor_units"`
	PlannedValueMinorUnits  string `json:"planned_value_minor_units"`
	PercentComplete         string `json:"percent_complete"`
	EVMAvailable            bool   `json:"evm_available"`
}

func f64(v float64) string { return floatText(v) }
func i64(v int64) string   { return strconv.FormatInt(v, 10) }

// EncodePortfolio writes a portfolio case as its engine-neutral JSON file.
func EncodePortfolio(c PortfolioCase) ([]byte, error) {
	doc := portfolioDoc{ID: c.ID, Projects: make([]portfolioRecord, len(c.Projects))}
	for i, p := range c.Projects {
		doc.Projects[i] = portfolioRecord{
			ProjectID: p.ProjectID, Name: p.Name,
			BudgetedCost: f64(p.BudgetedCost), CommittedCost: f64(p.CommittedCost), ActualCost: f64(p.ActualCost),
			EarnedValue: f64(p.EarnedValue), PlannedValue: f64(p.PlannedValue),
			BudgetedCostMinorUnits: i64(p.BudgetedCostMinorUnits), CommittedCostMinorUnits: i64(p.CommittedCostMinorUnits),
			ActualCostMinorUnits: i64(p.ActualCostMinorUnits), EarnedValueMinorUnits: i64(p.EarnedValueMinorUnits),
			PlannedValueMinorUnits: i64(p.PlannedValueMinorUnits),
			PercentComplete:        f64(p.PercentComplete), EVMAvailable: p.EVMAvailable,
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// DecodePortfolio reads a file written by EncodePortfolio.
func DecodePortfolio(b []byte) (PortfolioCase, error) {
	var doc portfolioDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return PortfolioCase{}, err
	}
	c := PortfolioCase{ID: doc.ID, Projects: make([]Project, len(doc.Projects))}
	pf := func(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
	pi := func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
	for i, r := range doc.Projects {
		var p Project
		var err error
		p.ProjectID, p.Name, p.EVMAvailable = r.ProjectID, r.Name, r.EVMAvailable
		for _, f := range []struct {
			dst *float64
			src string
		}{{&p.BudgetedCost, r.BudgetedCost}, {&p.CommittedCost, r.CommittedCost}, {&p.ActualCost, r.ActualCost}, {&p.EarnedValue, r.EarnedValue}, {&p.PlannedValue, r.PlannedValue}, {&p.PercentComplete, r.PercentComplete}} {
			if *f.dst, err = pf(f.src); err != nil {
				return PortfolioCase{}, fmt.Errorf("project %d: %w", i, err)
			}
		}
		for _, f := range []struct {
			dst *int64
			src string
		}{{&p.BudgetedCostMinorUnits, r.BudgetedCostMinorUnits}, {&p.CommittedCostMinorUnits, r.CommittedCostMinorUnits}, {&p.ActualCostMinorUnits, r.ActualCostMinorUnits}, {&p.EarnedValueMinorUnits, r.EarnedValueMinorUnits}, {&p.PlannedValueMinorUnits, r.PlannedValueMinorUnits}} {
			if *f.dst, err = pi(f.src); err != nil {
				return PortfolioCase{}, fmt.Errorf("project %d: %w", i, err)
			}
		}
		c.Projects[i] = p
	}
	return c, nil
}

func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// ExpectedDigest hashes a case's canonical expected rows.
func ExpectedDigest(c TabularCase) (string, error) {
	if c.Accept == AcceptError || c.Row == nil {
		return "", nil
	}
	h := sha256.New()
	for i := 0; i < min(c.TotalRows, PreviewRows); i++ {
		b, err := json.Marshal(c.Row(i))
		if err != nil {
			return "", err
		}
		h.Write(append(b, '\n'))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// WriteCorpus writes every case under dir (portfolio/ and tabular/) and
// returns the manifest. Large cases are written only when large is true but
// are always listed.
func WriteCorpus(dir string, large bool) (Manifest, error) {
	m := Manifest{Generator: "go run ./cmd/df0-fixtures"}
	if err := os.MkdirAll(filepath.Join(dir, "portfolio"), 0o700); err != nil {
		return m, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "tabular"), 0o700); err != nil {
		return m, err
	}
	for _, c := range PortfolioCases() {
		b, err := EncodePortfolio(c)
		if err != nil {
			return m, err
		}
		rel := filepath.Join("portfolio", c.ID+".json")
		if err := os.WriteFile(filepath.Join(dir, rel), b, 0o600); err != nil {
			return m, err
		}
		sum := sha256.Sum256(b)
		m.Entries = append(m.Entries, Entry{ID: c.ID, Kind: "portfolio", File: filepath.ToSlash(rel), Class: c.Class, Note: c.Note,
			Bytes: int64(len(b)), SHA256: hex.EncodeToString(sum[:]), TotalRows: len(c.Projects)})
	}
	for _, c := range TabularCases() {
		rel := filepath.Join("tabular", c.File)
		e := Entry{ID: c.ID, Kind: "tabular", File: filepath.ToSlash(rel), Format: c.Format, Class: c.Class, Accept: c.Accept,
			Note: c.Note, Large: c.Large, TotalRows: c.TotalRows, Columns: c.Columns}
		var err error
		if e.ExpectedSHA256, err = ExpectedDigest(c); err != nil {
			return m, fmt.Errorf("%s expected rows: %w", c.ID, err)
		}
		if !c.Large || large {
			if err := c.Write(filepath.Join(dir, rel)); err != nil {
				return m, err
			}
			if e.SHA256, e.Bytes, err = fileDigest(filepath.Join(dir, rel)); err != nil {
				return m, err
			}
		}
		m.Entries = append(m.Entries, e)
	}
	sort.SliceStable(m.Entries, func(i, j int) bool { return m.Entries[i].Kind < m.Entries[j].Kind })
	return m, nil
}

// Diff reports how have differs from want, ignoring large entries whose file
// was not written in have.
func Diff(have, want Manifest) []string {
	idx := map[string]Entry{}
	for _, e := range want.Entries {
		idx[e.ID] = e
	}
	var out []string
	seen := map[string]bool{}
	for _, h := range have.Entries {
		seen[h.ID] = true
		w, ok := idx[h.ID]
		if !ok {
			out = append(out, h.ID+": not in the committed manifest")
			continue
		}
		if h.SHA256 == "" && h.Large {
			h.SHA256, h.Bytes = w.SHA256, w.Bytes
		}
		hb, _ := json.Marshal(h)
		wb, _ := json.Marshal(w)
		if string(hb) != string(wb) {
			out = append(out, h.ID+": differs from the committed manifest")
		}
	}
	for _, w := range want.Entries {
		if !seen[w.ID] {
			out = append(out, w.ID+": missing from the generated corpus")
		}
	}
	return out
}
