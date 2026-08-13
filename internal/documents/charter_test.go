// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"errors"
	"testing"
	"unicode/utf8"
)

// documents_test.go's TestRender_AllKindsProduceValidPDF seeds every kind
// exclusively from DefaultContent(), which is zero-valued by design (see
// defaults.go's zeroFor). That means every "populated content" branch --
// tables, bulleted sections, key/value rows, and the generic fallback
// renderer's per-FieldKind switch -- has never executed under test. These
// tests populate real, in-schema content to exercise those branches.
//
// fpdf compresses content streams by default (no SetCompression(false) in
// newDocPDF), so asserting the populated PDF's raw bytes contain a specific
// field string is unreliable. Instead these tests assert the populated
// render is substantially larger than the same kind's empty/default-content
// render -- a cheap, deterministic proxy for "the extra content actually
// reached the page" that doesn't depend on stream compression internals.
//
// That comparison has its own hazard: every render's footer embeds
// time.Now().UTC().Format(time.RFC3339Nano), whose trailing-zero-trimmed
// fractional seconds vary in length call to call (confirmed empirically --
// a bare exact-equality assertion between two separately-timed renders
// flaked under `go test -race` with a real one-byte mismatch). Bare
// less-than-or-equal comparisons against a genuinely populated field are
// still safe in practice -- measured deltas across every test below ranged
// from 53 to 1614 bytes, all far above RFC3339Nano's worst-case ~9-byte
// jitter -- but assertGrew makes the safety margin explicit and keyed to a
// single named constant instead of relying on each test's field values
// happening to be long enough.

// growthTolerance bounds the byte-length jitter contributed solely by the
// footer timestamp's variable-precision fractional seconds. Comfortably
// above the observed worst case (a handful of bytes) and comfortably below
// the smallest measured genuine-content delta in this file (53 bytes for a
// single default-case KV row).
const growthTolerance = 12

// assertGrew fails the test unless got is larger than empty by more than
// growthTolerance -- i.e. the size difference is attributable to the
// content under test, not footer-timestamp jitter alone.
func assertGrew(t *testing.T, got, empty []byte, what string) {
	t.Helper()
	if delta := len(got) - len(empty); delta <= growthTolerance {
		t.Errorf("%s: delta = %d bytes (got %d, empty %d), want > %d (footer-timestamp jitter tolerance) -- the populated content may not have reached the page", what, delta, len(got), len(empty), growthTolerance)
	}
}

func mustRender(t *testing.T, content map[string]interface{}, projectName string, render func(map[string]interface{}, string) ([]byte, error)) []byte {
	t.Helper()
	out, err := render(content, projectName)
	if err != nil {
		t.Fatalf("render: unexpected error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("render: output does not start with %%PDF- header")
	}
	return out
}

// ----- Validate / isZero -----

func TestValidate_AllRequiredFieldsPresent_ReturnsNil(t *testing.T) {
	content := `{
		"project_name": "Apollo",
		"sponsor": "Jane Sponsor",
		"project_manager": "Jo PM",
		"charter_date": "2026-01-15",
		"purpose": "Because we must."
	}`
	if err := Validate(KindProjectCharterWord, content); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidate_MissingRequiredKey_ReturnsErrMissingRequired(t *testing.T) {
	// "purpose" is Required=true but absent entirely from the content map.
	content := `{
		"project_name": "Apollo",
		"sponsor": "Jane Sponsor",
		"project_manager": "Jo PM",
		"charter_date": "2026-01-15"
	}`
	err := Validate(KindProjectCharterWord, content)
	if !errors.Is(err, ErrMissingRequired) {
		t.Fatalf("Validate() = %v, want errors.Is(err, ErrMissingRequired)", err)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("purpose")) {
		t.Errorf("Validate() error = %q, want it to name the missing field %q", err.Error(), "purpose")
	}
}

func TestValidate_RequiredFieldPresentButEmptyString_ReturnsErrMissingRequired(t *testing.T) {
	// "purpose" is present in the JSON but is the empty string -- isZero
	// must treat that the same as absent, not merely "key exists".
	content := `{
		"project_name": "Apollo",
		"sponsor": "Jane Sponsor",
		"project_manager": "Jo PM",
		"charter_date": "2026-01-15",
		"purpose": ""
	}`
	err := Validate(KindProjectCharterWord, content)
	if !errors.Is(err, ErrMissingRequired) {
		t.Fatalf("Validate() = %v, want errors.Is(err, ErrMissingRequired)", err)
	}
}

func TestValidate_InvalidJSON_ReturnsWrappedError(t *testing.T) {
	err := Validate(KindProjectCharterWord, `not json`)
	if err == nil {
		t.Fatal("Validate() = nil, want an error for malformed JSON")
	}
}

func TestValidate_UnknownKind_ReturnsError(t *testing.T) {
	err := Validate(Kind("no_such_kind"), `{}`)
	if err == nil {
		t.Fatal("Validate() = nil, want an error for an unregistered kind")
	}
}

func TestValidate_ScopeStatement_RequiredTextFieldPresent_ReturnsNil(t *testing.T) {
	// A second, independently-registered kind whose required field is
	// FieldText rather than FieldString/FieldDate, so this doesn't just
	// re-prove the charter case above under a different Kind constant.
	content := `{"project_name": "Apollo", "scope_description": "In scope: X. Out of scope: Y."}`
	if err := Validate(KindScopeStatement, content); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "x", false},
		{"zero float64", float64(0), true},
		{"non-zero float64", float64(1), false},
		{"false", false, true}, // documents current semantics: a deliberate "No" answer counts as zero/empty.
		{"true", true, false},
		{"empty slice", []interface{}{}, true},
		{"non-empty slice", []interface{}{"x"}, false},
		{"empty map", map[string]interface{}{}, true},
		{"non-empty map", map[string]interface{}{"k": "v"}, false},
		// json.Unmarshal into map[string]interface{} never produces an
		// int, so this exercises isZero's behavior for a type outside its
		// switch: the default fallthrough treats it as non-zero (valid),
		// not zero (missing). Pinning this rather than asserting it is
		// "correct" -- Validate() would silently accept a required field
		// holding a type its switch doesn't recognize.
		{"unrecognized type falls through to non-zero", 42, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isZero(tt.v); got != tt.want {
				t.Errorf("isZero(%#v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// ----- RenderCharterPDF: populated content -----

func TestRenderCharterPDF_PopulatedContent_LargerThanEmptyRender(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderCharterPDF)

	populated := map[string]interface{}{
		"project_name":    "Apollo Migration",
		"charter_date":    "2026-03-01",
		"sponsor":         "Jane Sponsor",
		"project_manager": "Jo PM",
		"purpose":         "Migrate the legacy ledger before the compliance deadline.",
		"objectives":      []interface{}{"Cut downtime to zero", "Preserve audit trail"},
		"scope_in":        []interface{}{"Ledger service", "Reporting API"},
		"scope_out":       []interface{}{"Billing UI"},
		"deliverables":    []interface{}{"Migration runbook", "Rollback plan"},
		// First-ever population of RenderCharterPDF's hand-written
		// writeTable() calls (headers/widths/keys literals at
		// charter.go's stakeholder and milestone tables) -- the
		// smoke test's zero-value seed always leaves both empty, so
		// this is the first test execution of either table.
		//
		// getObjectSlice type-asserts on []interface{} of
		// map[string]interface{}, the shape json.Unmarshal always
		// produces for a JSON array of objects -- the shape every real
		// caller uses, since RenderCharterPDF is only ever reached via
		// Render()/renderRaw() unmarshaling JSON content. A Go-native
		// []map[string]interface{} literal fails that type assertion
		// and silently skips the table; using []interface{} here
		// matches production, not just what compiles.
		"stakeholders": []interface{}{
			map[string]interface{}{"name": "Alex Stakeholder", "role": "Product Owner", "interest": "High / High"},
			map[string]interface{}{"name": "Sam Reviewer", "role": "Compliance", "interest": "High / Medium"},
		},
		"high_level_schedule": "Kickoff March, cutover June.",
		"milestones": []interface{}{
			map[string]interface{}{"name": "Kickoff", "date": "2026-03-01"},
			map[string]interface{}{"name": "Cutover", "date": "2026-06-01"},
		},
		"high_level_budget": 125000.0,
		"assumptions":       []interface{}{"Vendor API stays stable"},
		"constraints":       []interface{}{"Must not exceed budget"},
		"risks":             []interface{}{"Vendor delay"},
		"success_criteria":  []interface{}{"Zero data loss"},
		"authorisation":     "Approved by steering committee 2026-02-20.",
	}
	got := mustRender(t, populated, "Test Project", RenderCharterPDF)

	assertGrew(t, got, empty, "populated RenderCharterPDF output")
}

// The aggregate size-comparison test above populates every section at once,
// so it cannot tell "the stakeholder table stopped rendering" apart from
// "some other section grew a little less than usual" -- with that much
// other content present, dropping either literal writeTable() call still
// leaves the overall output larger than empty. Fault-seeding confirmed this
// gap: disabling the stakeholder table alone left the aggregate test
// passing. These two tests isolate each table by rendering it as the only
// populated field, so a regression in either specific writeTable() call
// site is caught on its own.

func TestRenderCharterPDF_StakeholderTable_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderCharterPDF)
	got := mustRender(t, map[string]interface{}{
		"stakeholders": []interface{}{
			map[string]interface{}{"name": "Alex Stakeholder", "role": "Product Owner", "interest": "High / High"},
		},
	}, "Test Project", RenderCharterPDF)
	assertGrew(t, got, empty, "RenderCharterPDF with only stakeholders populated")
}

func TestRenderCharterPDF_MilestoneTable_WrittenWhenPresent(t *testing.T) {
	empty := mustRender(t, map[string]interface{}{}, "Test Project", RenderCharterPDF)
	got := mustRender(t, map[string]interface{}{
		"milestones": []interface{}{
			map[string]interface{}{"name": "Kickoff", "date": "2026-03-01"},
		},
	}, "Test Project", RenderCharterPDF)
	assertGrew(t, got, empty, "RenderCharterPDF with only milestones populated")
}

// ----- renderGenericPDF: populated content across FieldKind branches -----
//
// renderRaw's switch (charter.go) currently has an explicit case for every
// one of the 25 registered Kinds, so renderGenericPDF is never reached
// through the public Render()/renderRaw() dispatch path today -- it exists
// as a forward-compatible fallback for a future Kind added without a
// bespoke renderer (see its doc comment). These tests call it directly
// (same package) since it is real, shipped production code whose branches
// deserve coverage independent of whether any live Kind currently reaches
// it through the dispatcher.

func TestRenderGenericPDF_ScopeStatement_AllFieldsTogetherProduceLargerOutput(t *testing.T) {
	// A realistic aggregate sanity check. This alone is NOT sufficient
	// assurance that each branch fired -- fault-seeding during
	// development proved a single dropped switch case (e.g. FieldText)
	// can still leave this larger than empty because the other
	// populated fields mask it. The per-field-kind tests below are what
	// actually pins each branch; keep this one for realistic end-to-end
	// shape only.
	empty, err := renderGenericPDF(KindScopeStatement, map[string]interface{}{}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(empty): %v", err)
	}

	content := map[string]interface{}{
		"project_name":        "Apollo Migration",
		"scope_description":   "In scope: ledger migration. Out of scope: billing UI.",
		"deliverables":        []interface{}{"Migration runbook"},
		"acceptance_criteria": []interface{}{"Zero data loss"},
		"exclusions":          []interface{}{"Billing UI"},
		"constraints":         []interface{}{"Fixed budget"},
		"assumptions":         []interface{}{"Vendor API stable"},
	}
	got, err := renderGenericPDF(KindScopeStatement, content, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(populated): %v", err)
	}
	if !bytes.HasPrefix(got, []byte("%PDF-")) {
		t.Fatalf("renderGenericPDF: output does not start with %%PDF- header")
	}
	assertGrew(t, got, empty, "populated renderGenericPDF output")
}

// Each of the following isolates exactly one FieldKind switch branch in
// renderGenericPDF by populating only the one field that exercises it,
// leaving every other registered field for that Kind absent. This is
// deliberately more tedious than one big combined-content test, but
// fault-seeding proved the combined version necessary: dropping the
// FieldText case, or the FieldObjectArr case, still left the combined
// test's output larger than empty because the surviving fields' own
// content was enough to mask the missing branch's contribution.

func TestRenderGenericPDF_StringArrField_WrittenWhenPresent(t *testing.T) {
	empty, err := renderGenericPDF(KindScopeStatement, map[string]interface{}{}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(empty): %v", err)
	}
	got, err := renderGenericPDF(KindScopeStatement, map[string]interface{}{
		"deliverables": []interface{}{"Migration runbook"},
	}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(deliverables only): %v", err)
	}
	assertGrew(t, got, empty, "renderGenericPDF with only a FieldStringArr populated")
}

func TestRenderGenericPDF_TextField_WrittenWhenPresent(t *testing.T) {
	empty, err := renderGenericPDF(KindScopeStatement, map[string]interface{}{}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(empty): %v", err)
	}
	got, err := renderGenericPDF(KindScopeStatement, map[string]interface{}{
		"scope_description": "In scope: ledger migration. Out of scope: billing UI.",
	}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(scope_description only): %v", err)
	}
	assertGrew(t, got, empty, "renderGenericPDF with only a FieldText populated")
}

func TestRenderGenericPDF_ObjectArrField_WrittenWhenPresent(t *testing.T) {
	// risks is a FieldObjectArr with 9 sub-fields. Exercises
	// renderObjectArray, writeTable (called here from the generic
	// fallback path rather than a bespoke renderer's literal call),
	// toObjectSlice, and toString for both string and numeric sub-field
	// values.
	empty, err := renderGenericPDF(KindRiskRegister, map[string]interface{}{}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(empty): %v", err)
	}
	got, err := renderGenericPDF(KindRiskRegister, map[string]interface{}{
		"risks": []interface{}{
			map[string]interface{}{
				"id": "R-1", "description": "Vendor API instability", "kind": "risk",
				"probability": 3.0, "impact": 4.0, "owner": "Alex Stakeholder",
				"mitigation": "Add retry budget", "status": "open", "linked_task": "T-42",
			},
		},
	}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(risks only): %v", err)
	}
	assertGrew(t, got, empty, "renderGenericPDF with only a FieldObjectArr populated")
}

func TestRenderGenericPDF_DefaultCaseField_WrittenWhenPresent(t *testing.T) {
	// risk_matrix_ref is a FieldChartRef, which falls into the switch's
	// default case (shared with FieldString/FieldDate).
	empty, err := renderGenericPDF(KindRiskRegister, map[string]interface{}{}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(empty): %v", err)
	}
	got, err := renderGenericPDF(KindRiskRegister, map[string]interface{}{
		"risk_matrix_ref": "chart-123",
	}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(risk_matrix_ref only): %v", err)
	}
	assertGrew(t, got, empty, "renderGenericPDF with only the default-case field populated")
}

func TestRenderGenericPDF_ProjectBudget_NumberFieldWrittenOnlyWhenNonZero(t *testing.T) {
	// contingency_pct = 0 must be SKIPPED (writeKV only fires for n != 0);
	// contingency_pct = 10 must be WRITTEN. Rendering both fields in one
	// call and only asserting overall size growth (as the other tests
	// do) couldn't distinguish "the zero was skipped" from "the zero was
	// written as 0.00" -- both produce a larger-than-empty PDF.
	//
	// Every renderGenericPDF call also writes a footer with
	// time.Now().UTC().Format(time.RFC3339Nano), whose trailing-zero
	// trimming makes the footer string (and so the compressed output) a
	// few bytes shorter or longer depending on the instant it's called
	// -- confirmed by a real, non-flaky-looking failure under `go test
	// -race` during development ("1961 bytes, want exactly 1962").
	// Asserting exact byte-length equality between two separate render
	// calls is therefore not safe; assertGrew's growthTolerance absorbs
	// that jitter while staying far smaller than a real writeKV row.
	zeroOnly, err := renderGenericPDF(KindProjectBudget, map[string]interface{}{"contingency_pct": 0.0}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(zeroOnly): %v", err)
	}
	nonZero, err := renderGenericPDF(KindProjectBudget, map[string]interface{}{"contingency_pct": 10.0}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(nonZero): %v", err)
	}
	assertGrew(t, nonZero, zeroOnly, "renderGenericPDF(contingency_pct=10) vs renderGenericPDF(contingency_pct=0)")

	nonZeroFull, err := renderGenericPDF(KindProjectBudget, map[string]interface{}{
		"currency":        "USD",
		"contingency_pct": 10.0,
		"total":           42000.0,
		"categories": []interface{}{
			map[string]interface{}{"category": "Labor", "amount": 30000.0, "notes": "Contractors"},
		},
	}, "Test Project")
	if err != nil {
		t.Fatalf("renderGenericPDF(nonZeroFull): %v", err)
	}
	assertGrew(t, nonZeroFull, zeroOnly, "populated renderGenericPDF output vs the zero-field render")
}

func TestRenderGenericPDF_UnknownKind_ReturnsError(t *testing.T) {
	_, err := renderGenericPDF(Kind("no_such_kind"), map[string]interface{}{}, "Test Project")
	if err == nil {
		t.Fatal("renderGenericPDF() = nil error, want an error for an unregistered kind")
	}
}

// ----- KindsSorted -----

func TestKindsSorted_ReturnsAllKindsInNameOrder(t *testing.T) {
	got := KindsSorted()
	all := All()
	if len(got) != len(all) {
		t.Fatalf("KindsSorted() returned %d kinds, want %d", len(got), len(all))
	}

	byKind := make(map[Kind]string, len(all))
	for _, d := range all {
		byKind[d.Kind] = d.Name
	}
	for i := 1; i < len(got); i++ {
		if byKind[got[i-1]] > byKind[got[i]] {
			t.Errorf("KindsSorted() not sorted by Name at index %d: %q > %q", i, byKind[got[i-1]], byKind[got[i]])
		}
	}

	seen := make(map[Kind]bool, len(got))
	for _, k := range got {
		if seen[k] {
			t.Errorf("KindsSorted() contains duplicate kind %q", k)
		}
		seen[k] = true
		if _, ok := byKind[k]; !ok {
			t.Errorf("KindsSorted() contains %q which is not in All()", k)
		}
	}
}

// ----- truncDoc (shared truncation helper) -----
//
// truncDoc replaced nine independently-duplicated, identically-buggy
// trunc* functions across this package's per-kind renderers (truncTC,
// truncC, truncClo, truncExec, truncProc, truncIssue, truncBudget,
// truncReq, truncR). All nine used to slice by byte index (s[:n-1]),
// which could split a multi-byte UTF-8 character and emit invalid
// UTF-8 -- confirmed at the time via
// truncTC("Zoë Müller-Åström the Third Extraordinaire", 4) returning
// "Zo\xc3…". truncDoc keeps n as a byte budget (not a rune-count budget
// -- see its doc comment in charter.go for why a rune-count version was
// tried first and rejected: it nearly tripled a CJK cell's rendered
// width against the same n, a real table-layout overflow measured
// directly against a real column width). The multi-byte-safety behavior
// is proven once here, on the shared implementation, rather than
// re-proven separately for each of the nine thin wrappers -- see
// TestTruncWrappersDelegate below for confirmation that each wrapper
// actually calls through to this function rather than, say, silently
// reverting to its own inline copy.

func TestTruncDoc(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than limit passes through unchanged", "Alex", 10, "Alex"},
		{"exact-length boundary passes through unchanged", "Alexandra", 9, "Alexandra"},
		{"longer than limit is truncated with an ellipsis", "Alexandra Extraordinaire", 10, "Alexandra…"},
		// n=1 behaves identically under the old byte-slicing code too
		// (s[:0] is always a valid empty slice) -- this pins the
		// boundary but does not discriminate the fix. n=0, below, is
		// the one that actually differs: confirmed by reverting truncDoc
		// to the old s[:n-1] body and re-running just this case, which
		// panics with "slice bounds out of range [:-1]" rather than
		// merely failing an assertion.
		{"n=1 produces a bare ellipsis without panicking", "Alexandra", 1, "…"},
		{"n=0 does not panic (unlike every old per-file implementation)", "Alexandra", 0, "…"},
		// The historical bug case, and one of only two cases in this
		// table confirmed (by running it against the reverted old body)
		// to actually discriminate the fix -- see TestTruncWrappersDelegate's
		// comment for the full revert-and-rerun methodology. Byte-index
		// slicing used to cut mid-character here (the old code's s[:3]
		// landed inside "ë"'s 2-byte encoding, producing invalid UTF-8).
		// truncDoc backs off from that same byte budget to the last full
		// rune boundary: "ë" doesn't fit in the 3 bytes available after
		// "Z" and "o" (2 bytes) leave only 1, so it's dropped whole
		// rather than split, keeping "Zo".
		{"multi-byte UTF-8 backs off to the last full rune within the byte budget", "Zoë Müller-Åström the Third Extraordinaire", 4, "Zo…"},
		{"non-ASCII input shorter than the byte budget passes through unchanged", "Zoë Müller-Åström", 30, "Zoë Müller-Åström"},
		// This is BYTE budget, not rune-count: a 12-rune CJK string is
		// 36 bytes (3 bytes/rune), which exceeds n=20 in bytes even
		// though 12 <= 20 in runes -- so this truncates, matching what
		// the old byte-based implementations already did (correctly, by
		// accident) for this exact case. The other of the two cases in
		// this table confirmed to discriminate the fix (the old code
		// produces a different, invalid-UTF-8 result here too).
		{"CJK input within rune-count but over the byte budget still truncates", "日本語のテスト文字列です", 20, "日本語のテス…"},
		// n=4 happens to land exactly on a rune boundary for this
		// particular 3-byte-only string (byte index 3 == the start of
		// the second rune), so the old code's s[:3] and the new
		// back-off produce the SAME output here -- confirmed by running
		// this case against the reverted old body, where it passes
		// rather than fails. Kept as a real, valid case (it does prove
		// truncDoc handles an exact rune boundary correctly), but NOT
		// counted among the cases that discriminate the historical bug.
		{"multi-byte UTF-8 at a 3-byte-rune boundary truncates cleanly", "日本語のテスト文字列です", 4, "日…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncDoc(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("truncDoc(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncDoc(%q, %d) = %q, which is not valid UTF-8", tt.s, tt.n, got)
			}
		})
	}
}

// TestTruncWrappersDelegate confirms each of the nine per-file trunc*
// wrappers actually calls through to the shared truncDoc, using the
// same multi-byte input that used to expose the byte-slicing defect --
// a wrapper that silently kept its own old inline logic (a plausible
// copy-paste-revert slip when a fix touches nine files in one pass)
// would fail this on the exact input that matters, even though its
// ASCII-only behavior would look identical either way.
//
// Revert-and-rerun methodology: this test and TestTruncDoc's two
// discriminating cases (see their comments above) were confirmed to
// actually catch the historical bug, not assumed to from the fix's
// mechanism alone -- truncDoc's body was temporarily reverted to the
// old s[:n-1] + "…" slicing (no rune-boundary back-off) and these tests
// re-run against it: all nine wrapper subtests here failed, as did the
// two TestTruncDoc cases noted as discriminating, while the coincidence
// case (n lands exactly on a rune boundary) and the ASCII-only
// TestTruncTC cases all still passed, confirming they don't exercise
// the bug. The revert was never committed.
func TestTruncWrappersDelegate(t *testing.T) {
	const s = "Zoë Müller-Åström the Third Extraordinaire"
	const n = 4
	const want = "Zo…"

	wrappers := []struct {
		name string
		fn   func(string, int) string
	}{
		{"truncTC", truncTC},
		{"truncC", truncC},
		{"truncClo", truncClo},
		{"truncExec", truncExec},
		{"truncProc", truncProc},
		{"truncIssue", truncIssue},
		{"truncBudget", truncBudget},
		{"truncReq", truncReq},
		{"truncR", truncR},
	}
	for _, w := range wrappers {
		t.Run(w.name, func(t *testing.T) {
			if got := w.fn(s, n); got != want {
				t.Errorf("%s(%q, %d) = %q, want %q (does it still delegate to truncDoc?)", w.name, s, n, got, want)
			}
		})
	}
}
