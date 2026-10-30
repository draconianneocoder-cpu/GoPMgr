// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The committed manifest must match a fresh generation, byte for byte.
func TestCorpusMatchesCommittedManifest(t *testing.T) {
	m, err := WriteCorpus(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want Manifest
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if d := Diff(m, want); len(d) > 0 {
		t.Fatalf("corpus drifted from fixtures/manifest.json (run df0-fixtures -write-manifest and review):\n%v", d)
	}
}

func TestPortfolioRoundTrip(t *testing.T) {
	for _, c := range PortfolioCases() {
		b, err := EncodePortfolio(c)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodePortfolio(b)
		if err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		if len(got.Projects) != len(c.Projects) {
			t.Fatalf("%s: %d projects, want %d", c.ID, len(got.Projects), len(c.Projects))
		}
		for i := range c.Projects {
			w, g := c.Projects[i], got.Projects[i]
			// NaN never equals itself; compare its bits.
			for _, pair := range [][2]float64{{w.BudgetedCost, g.BudgetedCost}, {w.CommittedCost, g.CommittedCost}, {w.ActualCost, g.ActualCost}} {
				if math.Float64bits(pair[0]) != math.Float64bits(pair[1]) && !(math.IsNaN(pair[0]) && math.IsNaN(pair[1])) {
					t.Fatalf("%s project %d: float %v became %v", c.ID, i, pair[0], pair[1])
				}
			}
			w.BudgetedCost, w.CommittedCost, w.ActualCost = 0, 0, 0
			g.BudgetedCost, g.CommittedCost, g.ActualCost = 0, 0, 0
			if !reflect.DeepEqual(w, g) {
				t.Fatalf("%s project %d: %+v became %+v", c.ID, i, w, g)
			}
		}
	}
}

func TestCaseIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	files := map[string]bool{}
	for _, c := range PortfolioCases() {
		if seen[c.ID] {
			t.Fatalf("duplicate id %s", c.ID)
		}
		seen[c.ID] = true
	}
	for _, c := range TabularCases() {
		if seen[c.ID] || files[c.File] {
			t.Fatalf("duplicate id or file %s %s", c.ID, c.File)
		}
		seen[c.ID], files[c.File] = true, true
		if c.Accept != AcceptError && c.Row == nil {
			t.Fatalf("%s: no truth", c.ID)
		}
	}
}
