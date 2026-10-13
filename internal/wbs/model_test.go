// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package wbs

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

const minimal = `{"schema_version":2,"revision":"0","root":{"id":"r"}}`

func fixture() Aggregate {
	return Aggregate{SchemaVersion: SchemaVersion, Revision: "9007199254740993", Root: &Node{
		ID: "r", Title: "Project", Number: "1", Children: []*Node{
			{ID: "b", Title: "β", Number: "1.1", Note: "<script>plain data</script>", Owner: " Person ", Effort: -2.5,
				Classification: "uninterpreted/custom", Dictionary: &Dictionary{Description: "Definition", ScopeIn: "Included", ScopeOut: "Excluded", AcceptanceCriteria: "Reviewed"}},
			{ID: "a", Title: "Second", Number: "1.1"},
		}}, ActivityLinks: []ActivityLink{{"b", "schedule", "task"}}, CostLinks: []CostLink{{"a", "cost"}}}
}

func encode(t *testing.T, a Aggregate) []byte {
	t.Helper()
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestParseRoundTripPreservesScope(t *testing.T) {
	a := fixture()
	raw := encode(t, a)
	before := string(raw)
	got, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, a) {
		t.Fatalf("scope changed: %#v", got)
	}
	if string(raw) != before {
		t.Fatal("input modified")
	}
	if err := Validate(a); err != nil {
		t.Fatal(err)
	}
	if string(encode(t, a)) != before {
		t.Fatal("validation modified source")
	}
	// Display numbers may be repeated; child identity and order still survive.
	if got.Root.Children[0].ID != "b" || got.Root.Children[1].ID != "a" {
		t.Fatal("order or identity lost")
	}
}

func TestParseRejectsAmbiguousAndMalformedPayloads(t *testing.T) {
	cases := map[string]string{
		"empty": "", "array": "[]", "null": "null", "truncated": `{"root":`,
		"trailing object": minimal + `{}`, "trailing garbage": minimal + `x`,
		"missing schema":          `{"revision":"0","root":{"id":"r"}}`,
		"missing revision":        `{"schema_version":2,"root":{"id":"r"}}`,
		"missing root":            `{"schema_version":2,"revision":"0"}`,
		"missing node id":         `{"schema_version":2,"revision":"0","root":{}}`,
		"numeric revision":        strings.Replace(minimal, `"revision":"0"`, `"revision":0`, 1),
		"duplicate root":          strings.Replace(minimal, `"root":`, `"root":{},"root":`, 1),
		"escaped duplicate":       strings.Replace(minimal, `"id":"r"`, `"id":"a","\u0069d":"r"`, 1),
		"case alias":              strings.Replace(minimal, `"id":`, `"ID":`, 1),
		"unknown envelope":        strings.Replace(minimal, `"root":`, `"secret":true,"root":`, 1),
		"unknown node":            strings.Replace(minimal, `"id":"r"`, `"id":"r","future":1`, 1),
		"wrong shape":             strings.Replace(minimal, `{"id":"r"}`, `"r"`, 1),
		"wrong dictionary":        strings.Replace(minimal, `"id":"r"`, `"id":"r","dictionary":[]`, 1),
		"unknown dictionary":      strings.Replace(minimal, `"id":"r"`, `"id":"r","dictionary":{"future":"x"}`, 1),
		"invalid UTF8":            strings.Replace(minimal, `"r"`, "\"\xff\"", 1),
		"unpaired high surrogate": strings.Replace(minimal, `"r"`, `"\ud800"`, 1),
		"unpaired low surrogate":  strings.Replace(minimal, `"r"`, `"\udc00"`, 1),
		"blank id":                strings.Replace(minimal, `"r"`, `" "`, 1),
		"missing activity field":  strings.Replace(minimal, `"root":`, `"activity_links":[{"node_id":"r","activity_id":"t"}],"root":`, 1),
		"missing cost field":      strings.Replace(minimal, `"root":`, `"cost_links":[{"node_id":"r"}],"root":`, 1),
		"deep unknown":            `{"future":` + strings.Repeat(`[`, 1000) + `0` + strings.Repeat(`]`, 1000) + `}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Parse([]byte(raw))
			if err == nil || !reflect.DeepEqual(got, Aggregate{}) {
				t.Fatalf("accepted or returned partial model: %v", err)
			}
		})
	}
}

func TestParseRejectsExplicitNulls(t *testing.T) {
	for _, member := range []string{"title", "number", "note", "owner", "effort", "classification", "dictionary", "children"} {
		t.Run(member, func(t *testing.T) {
			raw := strings.Replace(minimal, `"id":"r"`, `"id":"r","`+member+`":null`, 1)
			if _, err := Parse([]byte(raw)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("null accepted: %v", err)
			}
		})
	}
	for _, raw := range []string{
		`{"schema_version":null,"revision":"0","root":{"id":"r"}}`,
		`{"schema_version":2,"revision":null,"root":{"id":"r"}}`,
		`{"schema_version":2,"revision":"0","root":null}`,
		strings.Replace(minimal, `"root":`, `"activity_links":null,"root":`, 1),
		strings.Replace(minimal, `"root":`, `"cost_links":null,"root":`, 1),
		strings.Replace(minimal, `"id":"r"`, `"id":"r","children":[null]`, 1),
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("null accepted: %s", raw)
		}
	}
}

func TestParseOptionalEmptyFieldsAndUnicode(t *testing.T) {
	for _, raw := range []string{minimal,
		strings.Replace(minimal, `"id":"r"`, `"id":"r","title":"","children":[],"dictionary":{}`, 1),
		strings.Replace(minimal, `"r"`, `"\ud83d\ude00"`, 1),
	} {
		a, err := Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		b, err := Parse(encode(t, a))
		if err != nil {
			t.Fatal(err)
		}
		if string(encode(t, a)) != string(encode(t, b)) {
			t.Fatal("semantic round trip changed")
		}
	}
}

func TestSchemaVersionFailsClosed(t *testing.T) {
	for _, version := range []int{0, 1, 3, -1} {
		a := fixture()
		a.SchemaVersion = version
		if _, err := Parse(encode(t, a)); !errors.Is(err, ErrUnsupportedSchema) {
			t.Fatalf("version %d: %v", version, err)
		}
	}
}

func TestValidateRejectsInvalidTrees(t *testing.T) {
	cases := map[string]func(*Aggregate){
		"nil root":            func(a *Aggregate) { a.Root = nil },
		"nil child":           func(a *Aggregate) { a.Root.Children = append(a.Root.Children, nil) },
		"duplicate id":        func(a *Aggregate) { a.Root.Children[1].ID = "b" },
		"cycle":               func(a *Aggregate) { a.Root.Children[0].Children = []*Node{a.Root} },
		"alias":               func(a *Aggregate) { a.Root.Children = append(a.Root.Children, a.Root.Children[0]) },
		"invalid id encoding": func(a *Aggregate) { a.Root.ID = "\xff" },
		"blank id":            func(a *Aggregate) { a.Root.ID = " \t " },
		"NaN":                 func(a *Aggregate) { a.Root.Effort = math.NaN() },
		"positive infinity":   func(a *Aggregate) { a.Root.Effort = math.Inf(1) },
		"negative infinity":   func(a *Aggregate) { a.Root.Effort = math.Inf(-1) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := fixture()
			mutate(&a)
			if err := Validate(a); err == nil {
				t.Fatal("invalid tree accepted")
			}
		})
	}
}

func TestValidateTextAndIdentifierBoundaries(t *testing.T) {
	fields := []struct {
		name string
		set  func(*Node, string)
	}{
		{"title", func(n *Node, s string) { n.Title = s }}, {"number", func(n *Node, s string) { n.Number = s }},
		{"note", func(n *Node, s string) { n.Note = s }}, {"owner", func(n *Node, s string) { n.Owner = s }},
		{"classification", func(n *Node, s string) { n.Classification = s }},
		{"description", func(n *Node, s string) { n.Dictionary.Description = s }},
		{"scope_in", func(n *Node, s string) { n.Dictionary.ScopeIn = s }},
		{"scope_out", func(n *Node, s string) { n.Dictionary.ScopeOut = s }},
		{"acceptance_criteria", func(n *Node, s string) { n.Dictionary.AcceptanceCriteria = s }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			for _, s := range []string{strings.Repeat("é", MaxTextBytes/2), strings.Repeat("x", MaxTextBytes+1)} {
				a := fixture()
				field.set(a.Root.Children[0], s)
				err := Validate(a)
				_, parseErr := Parse(encode(t, a))
				wantValid := len(s) == MaxTextBytes
				if (err == nil) != wantValid || (parseErr == nil) != wantValid {
					t.Fatalf("bytes=%d validate=%v parse=%v", len(s), err, parseErr)
				}
			}
			a := fixture()
			field.set(a.Root.Children[0], "\xff")
			if err := Validate(a); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid UTF-8 in %s: %v", field.name, err)
			}
		})
	}
	for _, size := range []int{MaxIdentifierBytes, MaxIdentifierBytes + 1} {
		a := fixture()
		a.Root.ID = strings.Repeat("é", size/2) + strings.Repeat("x", size%2)
		if err := Validate(a); (err == nil) != (size == MaxIdentifierBytes) {
			t.Fatalf("identifier bytes=%d: %v", size, err)
		}
		if _, err := Parse(encode(t, a)); (err == nil) != (size == MaxIdentifierBytes) {
			t.Fatalf("parsed identifier bytes=%d: %v", size, err)
		}
	}
}

func TestPreflightBoundsDecodedStringsAndObjectCount(t *testing.T) {
	for _, count := range []int{MaxTextBytes, MaxTextBytes + 1} {
		raw := strings.Replace(minimal, `"id":"r"`, `"id":"r","note":"`+strings.Repeat(`\u0061`, count)+`"`, 1)
		if _, err := Parse([]byte(raw)); (err == nil) != (count == MaxTextBytes) {
			t.Fatalf("escaped decoded bytes=%d: %v", count, err)
		}
	}
	// Fail before constructing models even for a wide malformed future member.
	raw := `{"future":[` + strings.Repeat(`{},`, 2*MaxNodes+MaxAssociations) + `{}]}`
	if _, err := Parse([]byte(raw)); !errors.Is(err, ErrLimit) {
		t.Fatalf("preflight object budget: %v", err)
	}
}

func chain(levels int) Aggregate {
	a := Aggregate{SchemaVersion: SchemaVersion, Revision: "0", Root: &Node{ID: "0", Dictionary: &Dictionary{Description: "root"}}}
	n := a.Root
	for i := 1; i < levels; i++ {
		child := &Node{ID: fmt.Sprint(i), Dictionary: &Dictionary{ScopeIn: "scope"}}
		n.Children = []*Node{child}
		n = child
	}
	return a
}

func TestHierarchyAndNodeLimits(t *testing.T) {
	for _, levels := range []int{MaxLevels, MaxLevels + 1} {
		a := chain(levels)
		err := Validate(a)
		_, parseErr := Parse(encode(t, a))
		if (err == nil) != (levels == MaxLevels) || (parseErr == nil) != (levels == MaxLevels) {
			t.Fatalf("levels=%d: %v / %v", levels, err, parseErr)
		}
	}
	for _, count := range []int{MaxNodes, MaxNodes + 1} {
		a := chain(1)
		for i := 1; i < count; i++ {
			a.Root.Children = append(a.Root.Children, &Node{ID: fmt.Sprint(i)})
		}
		err := Validate(a)
		_, parseErr := Parse(encode(t, a))
		if (err == nil) != (count == MaxNodes) || (parseErr == nil) != (count == MaxNodes) {
			t.Fatalf("nodes=%d: %v / %v", count, err, parseErr)
		}
	}
	// Mixed branching exercises pending siblings as well as visited nodes.
	a := chain(1)
	for i := 1; i < 100; i++ {
		branch := &Node{ID: fmt.Sprintf("branch-%d", i)}
		for j := 0; j < 100; j++ {
			branch.Children = append(branch.Children, &Node{ID: fmt.Sprintf("leaf-%d-%d", i, j)})
		}
		a.Root.Children = append(a.Root.Children, branch)
	}
	if err := Validate(a); err != nil {
		t.Fatal(err)
	}
	a.Root.Children[0].Children = append(a.Root.Children[0].Children, &Node{ID: "overflow"})
	if err := Validate(a); !errors.Is(err, ErrLimit) {
		t.Fatalf("branching overflow: %v", err)
	}
}

func TestPayloadBoundariesAndAggregateSize(t *testing.T) {
	raw := append([]byte(minimal), []byte(strings.Repeat(" ", MaxPayloadBytes-len(minimal)))...)
	if _, err := Parse(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(append(raw, ' ')); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversize: %v", err)
	}
	a := chain(1)
	// Escaping expands encoded bytes; checking decoded text alone is insufficient.
	for i := 1; i <= 24; i++ {
		a.Root.Children = append(a.Root.Children, &Node{ID: fmt.Sprint(i), Note: strings.Repeat("\x00", MaxTextBytes)})
	}
	if err := Validate(a); !errors.Is(err, ErrLimit) {
		t.Fatalf("encoded size not bounded: %v", err)
	}
	// Keep the reviewer's exact encoded-size counterexample as a regression.
	a = chain(1)
	for i := 1; i <= 128; i++ {
		a.Root.Children = append(a.Root.Children, &Node{ID: fmt.Sprint(i), Note: strings.Repeat("x", MaxTextBytes)})
	}
	last := a.Root.Children[len(a.Root.Children)-1]
	last.Note = "x"
	last.Note += strings.Repeat("x", MaxPayloadBytes-len(encode(t, a)))
	if len(encode(t, a)) != MaxPayloadBytes {
		t.Fatal("fixture is not at encoded boundary")
	}
	if err := Validate(a); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(encode(t, a)); err != nil {
		t.Fatal(err)
	}
	last.Note += "x"
	if err := Validate(a); !errors.Is(err, ErrLimit) {
		t.Fatalf("encoded byte overflow: %v", err)
	}
}

func TestAssociationsRequireUniqueSourcesAndLeafTargets(t *testing.T) {
	cases := map[string]func(*Aggregate){
		"activity parent":               func(a *Aggregate) { a.ActivityLinks[0].NodeID = "r" },
		"activity missing node":         func(a *Aggregate) { a.ActivityLinks[0].NodeID = "missing" },
		"cost parent":                   func(a *Aggregate) { a.CostLinks[0].NodeID = "r" },
		"cost missing node":             func(a *Aggregate) { a.CostLinks[0].NodeID = "missing" },
		"activity duplicate":            func(a *Aggregate) { a.ActivityLinks = append(a.ActivityLinks, a.ActivityLinks[0]) },
		"activity other leaf duplicate": func(a *Aggregate) { a.ActivityLinks = append(a.ActivityLinks, ActivityLink{"a", "schedule", "task"}) },
		"cost duplicate":                func(a *Aggregate) { a.CostLinks = append(a.CostLinks, a.CostLinks[0]) },
		"cost other leaf duplicate":     func(a *Aggregate) { a.CostLinks = append(a.CostLinks, CostLink{"b", "cost"}) },
		"linked leaf gains child":       func(a *Aggregate) { a.Root.Children[0].Children = []*Node{{ID: "new"}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := fixture()
			mutate(&a)
			if err := Validate(a); err == nil {
				t.Fatal("invalid association accepted")
			}
		})
	}
	a := fixture()
	a.ActivityLinks = []ActivityLink{{"a", "s", "task"}, {"b", "other", "task"}, {"a", "a/b", "c"}, {"b", "a", "b/c"}}
	if err := Validate(a); err != nil {
		t.Fatalf("composite activity identities confused: %v", err)
	}
	// Reparenting while retaining a leaf ID does not change its association.
	a = fixture()
	leaf := a.Root.Children[0]
	a.Root.Children = []*Node{{ID: "parent", Children: []*Node{leaf}}, a.Root.Children[1]}
	if err := Validate(a); err != nil {
		t.Fatal(err)
	}
	if a.ActivityLinks[0].NodeID != "b" {
		t.Fatal("association changed")
	}
}

func TestAllAssociationIdentifiersAreValidated(t *testing.T) {
	for i := 0; i < 5; i++ {
		for _, value := range []string{"", " ", "\xff", strings.Repeat("x", MaxIdentifierBytes+1)} {
			a := fixture()
			fields := []*string{&a.ActivityLinks[0].NodeID, &a.ActivityLinks[0].ScheduleChartID, &a.ActivityLinks[0].ActivityID, &a.CostLinks[0].NodeID, &a.CostLinks[0].CostEntryID}
			*fields[i] = value
			if err := Validate(a); err == nil {
				t.Fatalf("field %d invalid identity accepted", i)
			}
		}
	}
}

func TestAssociationLimitCombinesBothKinds(t *testing.T) {
	a := chain(1)
	for i := 0; i < MaxAssociations/2; i++ {
		a.ActivityLinks = append(a.ActivityLinks, ActivityLink{"0", "s", fmt.Sprint(i)})
		a.CostLinks = append(a.CostLinks, CostLink{"0", fmt.Sprint(i)})
	}
	if err := Validate(a); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(encode(t, a)); err != nil {
		t.Fatal(err)
	}
	a.CostLinks = append(a.CostLinks, CostLink{"0", "one-more"})
	if err := Validate(a); !errors.Is(err, ErrLimit) {
		t.Fatalf("combined count: %v", err)
	}
	if _, err := Parse(encode(t, a)); !errors.Is(err, ErrLimit) {
		t.Fatalf("combined parse count: %v", err)
	}
}

func TestRevisionTokensAndOverflow(t *testing.T) {
	for _, s := range []string{"", "00", "01", "+1", "-1", " 1", "1 ", "1.0", "1e2", "١", "18446744073709551616"} {
		if next, err := NextRevision(s); !errors.Is(err, ErrRevision) || next != "" {
			t.Fatalf("invalid revision %q: %q %v", s, next, err)
		}
		a := fixture()
		a.Revision = s
		if _, err := Parse(encode(t, a)); !errors.Is(err, ErrRevision) {
			t.Fatalf("parse revision %q: %v", s, err)
		}
	}
	for _, tc := range []struct{ in, out string }{{"0", "1"}, {"9007199254740993", "9007199254740994"}, {"18446744073709551614", "18446744073709551615"}} {
		if next, err := NextRevision(tc.in); err != nil || next != tc.out {
			t.Fatalf("next: %q %v", next, err)
		}
	}
	a := fixture()
	a.Revision = "18446744073709551615"
	if _, err := Parse(encode(t, a)); err != nil {
		t.Fatal(err)
	}
	if next, err := NextRevision(a.Revision); next != "" || !errors.Is(err, ErrRevisionOverflow) {
		t.Fatalf("overflow: %q %v", next, err)
	}
}

func TestWriteContextRejectsEachStaleAxis(t *testing.T) {
	current := Identity{"copy", "project", "chart", "lifetime"}
	maxID := strings.Repeat("x", MaxIdentifierBytes)
	boundary := Identity{maxID, maxID, maxID, maxID}
	if err := CheckWriteContext(boundary, boundary, "0", "0"); err != nil {
		t.Fatalf("maximum identifier bytes: %v", err)
	}
	if err := CheckWriteContext(current, current, "9007199254740993", "9007199254740993"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		requested := current
		fields := []*string{&requested.CopyID, &requested.ProjectID, &requested.ChartID, &requested.LifetimeID}
		*fields[i] += "-other"
		if err := CheckWriteContext(current, requested, "2", "2"); !errors.Is(err, ErrIdentityMismatch) {
			t.Fatalf("axis%d: %v", i, err)
		}
		for _, s := range []string{"", " ", "\xff", strings.Repeat("x", MaxIdentifierBytes+1)} {
			*fields[i] = s
			if err := CheckWriteContext(current, requested, "2", "2"); err == nil {
				t.Fatalf("requested invalid axis%d", i)
			}
			if err := CheckWriteContext(requested, current, "2", "2"); err == nil {
				t.Fatalf("current invalid axis%d", i)
			}
		}
	}
	if err := CheckWriteContext(current, current, "2", "1"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"01", "1"}, {"1", "01"}, {"", "0"}, {"0", ""}} {
		if err := CheckWriteContext(current, current, pair[0], pair[1]); !errors.Is(err, ErrRevision) {
			t.Fatal(err)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{minimal, "", `{"root":null}`, strings.Replace(minimal, `"r"`, `"\ud800"`, 1)} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		a, err := Parse(raw)
		if err != nil {
			if !reflect.DeepEqual(a, Aggregate{}) {
				t.Fatal("partial result")
			}
			return
		}
		if err := Validate(a); err != nil {
			t.Fatal(err)
		}
		b, err := Parse(encode(t, a))
		if err != nil {
			t.Fatal(err)
		}
		if string(encode(t, a)) != string(encode(t, b)) {
			t.Fatal("unstable model roundtrip")
		}
	})
}
