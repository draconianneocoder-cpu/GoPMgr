// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"

	"gopmgr/internal/crypto"
	"gopmgr/internal/pdfmeta"
	"gopmgr/internal/signing"
)

// ErrMissingRequired is returned by Validate when a required field is
// empty or missing.
var ErrMissingRequired = errors.New("documents: required field is empty")

// Validate checks that a document's content satisfies the schema for
// its Kind. Currently enforces:
//
//   - Every Field with Required=true must be present and non-empty.
//
// Returns a wrapped ErrMissingRequired so callers can errors.Is() it.
//
// Future enhancements: type checking (string vs number), enum
// validation (status values), and per-kind cross-field constraints.
func Validate(k Kind, contentJSON string) error {
	fields := EffectiveFields(k)
	if fields == nil {
		return fmt.Errorf("documents: unknown kind %q", k)
	}

	var content map[string]interface{}
	if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
		return fmt.Errorf("documents: invalid JSON content: %w", err)
	}

	for _, f := range fields {
		if !f.Required {
			continue
		}
		v, ok := content[f.Key]
		if !ok || isZero(v) {
			return fmt.Errorf("%w: %q", ErrMissingRequired, f.Key)
		}
	}
	return nil
}

func isZero(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case float64:
		return x == 0
	case bool:
		return !x
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	}
	return false
}

// RenderCharterPDF produces an archival-quality PDF for the Project
// Charter document. All 25 registered document kinds now have their
// own bespoke renderer; renderGenericPDF (see Render() below) remains
// as a forward-compatible fallback for a future kind added without
// one yet, not a currently-used path.
//
// Layout:
//
//	Title block (project name, charter date)
//	Section: Purpose / Business Need
//	Section: Objectives (bulleted)
//	Section: Scope (in / out / deliverables)
//	Section: Stakeholders (table)
//	Section: High-Level Schedule + Milestones (table)
//	Section: Budget
//	Section: Assumptions / Constraints / Risks
//	Section: Success Criteria
//	Section: Authorisation block
//	Footer:  RFC3339Nano generation timestamp, GoPMgr version
func RenderCharterPDF(content map[string]interface{}, projectName string) ([]byte, error) {
	pdf := newDocPDF("P")
	pdf.SetMargins(20, 18, 20)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetTitle("Project Charter", true)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 22)
	pdf.Cell(0, 12, getString(content, "project_name", projectName))
	pdf.Ln(14)

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(110, 110, 110)
	pdf.Cell(0, 6, "Project Charter")
	pdf.Ln(5)
	if d := getString(content, "charter_date", ""); d != "" {
		pdf.Cell(0, 6, "Charter date: "+d)
		pdf.Ln(5)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(4)

	// Sponsor / PM
	if v := getString(content, "sponsor", ""); v != "" {
		writeKV(pdf, "Sponsor", v)
	}
	if v := getString(content, "project_manager", ""); v != "" {
		writeKV(pdf, "Project manager", v)
	}
	pdf.Ln(2)

	// Sections
	writeSection(pdf, "Purpose / Business Need", getString(content, "purpose", ""))
	writeBulletSection(pdf, "Objectives", getStringSlice(content, "objectives"))
	writeBulletSection(pdf, "In Scope", getStringSlice(content, "scope_in"))
	writeBulletSection(pdf, "Out of Scope", getStringSlice(content, "scope_out"))
	writeBulletSection(pdf, "Deliverables", getStringSlice(content, "deliverables"))

	// Stakeholder table
	if stakeholders := getObjectSlice(content, "stakeholders"); len(stakeholders) > 0 {
		writeHeading(pdf, "Stakeholders")
		writeTable(pdf,
			[]string{"Name", "Role", "Interest / Influence"},
			[]float64{50, 50, 70},
			stakeholders,
			[]string{"name", "role", "interest"},
		)
	}

	// Schedule
	writeSection(pdf, "High-Level Schedule", getString(content, "high_level_schedule", ""))
	if ms := getObjectSlice(content, "milestones"); len(ms) > 0 {
		writeTable(pdf,
			[]string{"Milestone", "Target Date"},
			[]float64{110, 60},
			ms,
			[]string{"name", "date"},
		)
	}

	// Budget
	if b, ok := content["high_level_budget"].(float64); ok && b > 0 {
		writeKV(pdf, "High-level budget (USD)", fmt.Sprintf("%.2f", b))
		pdf.Ln(2)
	}

	writeBulletSection(pdf, "Assumptions", getStringSlice(content, "assumptions"))
	writeBulletSection(pdf, "Constraints", getStringSlice(content, "constraints"))
	writeBulletSection(pdf, "Initial Risks", getStringSlice(content, "risks"))
	writeBulletSection(pdf, "Success Criteria", getStringSlice(content, "success_criteria"))

	writeSection(pdf, "Authorisation", getString(content, "authorisation", ""))

	// Footer
	pdf.SetY(-20)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 5,
		fmt.Sprintf("Generated by GoPMgr at %s", time.Now().UTC().Format(time.RFC3339Nano)),
		"", 0, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ----- small helpers -----

func writeHeading(pdf *fpdf.Fpdf, text string) {
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(0, 80, 130)
	pdf.Cell(0, 7, text)
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)
}

func writeSection(pdf *fpdf.Fpdf, heading, body string) {
	if body == "" {
		return
	}
	writeHeading(pdf, heading)
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 5, body, "", "L", false)
	pdf.Ln(2)
}

func writeBulletSection(pdf *fpdf.Fpdf, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	writeHeading(pdf, heading)
	pdf.SetFont("Helvetica", "", 10)
	for _, it := range items {
		pdf.Cell(5, 5, "•")
		pdf.MultiCell(0, 5, it, "", "L", false)
	}
	pdf.Ln(2)
}

func writeKV(pdf *fpdf.Fpdf, k, v string) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(40, 5, k+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 5, v, "", "L", false)
}

func writeTable(pdf *fpdf.Fpdf, headers []string, widths []float64, rows []map[string]interface{}, keys []string) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(235, 240, 245)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 6, h, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Helvetica", "", 9)
	for _, row := range rows {
		for i, k := range keys {
			v, _ := row[k].(string)
			pdf.CellFormat(widths[i], 6, v, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.Ln(2)
}

func getString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func getStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(v))
	for _, x := range v {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func getObjectSlice(m map[string]interface{}, key string) []map[string]interface{} {
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(v))
	for _, x := range v {
		if obj, ok := x.(map[string]interface{}); ok {
			out = append(out, obj)
		}
	}
	return out
}

// truncDoc truncates s to at most n bytes, appending a single "…" (a
// 3-byte UTF-8 character) when truncation occurs, without ever
// splitting a multi-byte UTF-8 character -- the defect every
// per-renderer trunc* wrapper in this package used to have
// independently (each sliced by byte index without a rune-boundary
// check: s[:n-1] + "…"), confirmed concretely via
// truncTC("Zoë Müller-Åström the Third Extraordinaire", 4), which
// returned the invalid-UTF-8 "Zo\xc3…" before this fix.
//
// This keeps n as a byte budget (matching every old per-file
// implementation) rather than switching to a rune-count budget. An
// earlier draft of this fix used rune count -- correct for validity,
// but wrong for layout: because these n values were tuned against
// byte-count truncation, and a single CJK character costs 3 bytes,
// rune-count truncation let a 64-byte call site keep 64 *characters* of
// CJK text instead of ~21, nearly tripling the string's rendered width
// (measured directly against issue_log.go's Description column with an
// embedded font: 46.5mm old vs. 133.6mm under rune-count truncation,
// against a 75mm-wide column -- a real table-layout overflow, not a
// theoretical one). Byte-budget truncation with a rune-boundary check
// avoids that regressing at every call site, not just the measured one:
// cut is always <= n-1 (it only ever moves left from there), and the
// old code always cut at exactly n-1, so this function's output is
// never more bytes than the old code's for the same s and n -- rendered
// width can only shrink or stay equal. The 46.5mm-both-ways measurement
// above is a confirming instance of that inspection-provable bound, not
// the basis for it. For ASCII input (byte count == rune count, so the
// back-off loop never executes) this is byte-for-byte identical to
// every old per-file implementation.
//
// n <= 0 is not guarded: cut starts at n-1 (negative for n<=0), but the
// loop condition `cut > 0` never executes a negative-index read, and
// the function clamps cut to 0 before slicing -- so unlike the
// byte-slicing/rune-slicing implementations that preceded this one,
// n <= 0 does NOT panic here; it degrades to a bare "…". Documented as
// a behavior note rather than added as a guarded precondition, since
// no call site in this package passes anything but a positive literal
// (16 to 120).
//
// The len(s) <= n passthrough branch returns s unmodified without
// re-validating it -- if s ever arrived already containing invalid
// UTF-8 (e.g. ending in a lone lead byte with no continuation byte),
// this function wouldn't repair it, and fpdf's UTF8-mode text path
// (reached whenever an embedded font is registered via
// SetFontApplier, fonts.go) reads continuation bytes by index without
// its own validity check -- traced concretely to
// fpdf's utf8toutf16 (github.com/go-pdf/fpdf@v0.9.0/util.go), which
// could read past a string's end on a sufficiently malformed trailing
// byte. Verified this isn't reachable through this package's own call
// path rather than assumed: every s these trunc* functions see
// originates from renderRaw's json.Unmarshal([]byte(contentJSON),
// &content) a few lines below, and Go's encoding/json always
// substitutes U+FFFD (valid UTF-8) for invalid byte sequences on
// string decode -- confirmed directly (a raw dangling 0xC3 lead byte
// embedded in JSON input decodes to a valid-UTF-8 Go string, never an
// invalid one). So s is guaranteed valid UTF-8 on every call site this
// package actually has.
func truncDoc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n - 1
	if cut < 0 {
		cut = 0
	}
	// Back off to the start of a rune (never a continuation byte) so a
	// multi-byte character that doesn't fully fit in the budget is
	// dropped whole rather than split.
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// Render dispatches to the kind-specific PDF renderer (or the generic
// fallback) and then embeds the canonical XMP metadata packet via a
// PDF incremental update. The XMP step is fail-soft: if injection
// fails for any reason, the un-tagged-but-valid PDF is returned rather
// than erroring the whole export. This keeps document export robust
// while still tagging every PDF that can be tagged.
func Render(kind Kind, contentJSON, projectName string) ([]byte, error) {
	raw, err := renderRaw(kind, contentJSON, projectName)
	if err != nil {
		return nil, err
	}

	def, ok := Get(kind)
	name := projectName
	if ok {
		name = projectName + ": " + def.Name
	}

	spec := pdfmeta.XMPSpec{
		Title:       name,
		Author:      "GoPMgr",
		Subject:     string(kind),
		CreatorTool: "GoPMgr",
	}

	// Prefer full PDF/A-3 (XMP + OutputIntent + ICC) when the profile
	// has been fetched via `make icc`. Fall back to XMP-only otherwise.
	if icc := pdfmeta.DefaultICCProfile(); len(icc) > 0 {
		if tagged, err := pdfmeta.MakePDFA3(raw, spec, icc); err == nil {
			return tagged, nil
		}
	} else if xmp := pdfmeta.BuildXMPPacket(spec); len(xmp) > 0 {
		if tagged, err := pdfmeta.InjectXMPStream(raw, xmp); err == nil {
			return tagged, nil
		}
	}

	// Fail-soft: return the valid but minimally-tagged PDF.
	return raw, nil
}

// RenderSigned renders the document and applies a PAdES Baseline B digital
// signature using the provided certificate. Application exports use
// signing.ApplyPAdES instead so project-level RFC 3161 settings can upgrade
// this same final mutation to Baseline T. This compatibility entry point
// remains useful to callers that explicitly require Baseline B.
//
//  1. Standard document rendering, including PDF/A-3 metadata.
//  2. Shared PAdES Baseline B signing pipeline.
//
// Callers that need a visible signature appearance must render it before
// this function signs the PDF. Appending another PDF after signing would
// leave those bytes outside the declared /ByteRange.
func RenderSigned(kind Kind, contentJSON, projectName, certPath, certPassword string) ([]byte, error) {
	signer, err := crypto.LoadCertificate(certPath, certPassword)
	if err != nil {
		return nil, fmt.Errorf("documents: load certificate for signing: %w", err)
	}
	return renderSignedWithSigner(kind, contentJSON, projectName, signer)
}

// renderSignedWithSigner is the testable compatibility seam behind
// RenderSigned. It intentionally passes nil timestamp configuration because
// this API promises Baseline B; application exports prepare project-level
// PAdES-T settings before calling the same signing pipeline.
func renderSignedWithSigner(
	kind Kind,
	contentJSON, projectName string,
	signer *crypto.Signer,
) ([]byte, error) {
	pdfBytes, err := Render(kind, contentJSON, projectName)
	if err != nil {
		return nil, err
	}
	signed, _, err := signing.ApplyPAdES(context.Background(), pdfBytes, signer, nil)
	if err != nil {
		return nil, fmt.Errorf("documents: apply PAdES Baseline B signature: %w", err)
	}
	return signed, nil
}

// renderRaw dispatches to the kind-specific PDF renderer, or falls
// back to a generic key/value renderer for kinds without bespoke
// layouts. It returns the PDF exactly as the renderer produced it,
// before XMP metadata injection.
func renderRaw(kind Kind, contentJSON, projectName string) ([]byte, error) {
	var content map[string]interface{}
	if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
		return nil, fmt.Errorf("documents: invalid JSON: %w", err)
	}

	switch kind {
	case KindProjectCharterWord, KindProjectCharterExcel:
		return RenderCharterPDF(content, projectName)
	case KindStatusReport:
		return RenderStatusReportPDF(content, projectName)
	case KindRiskRegister:
		return RenderRiskRegisterPDF(content, projectName)
	case KindProjectPlanWord, KindProjectPlanExcel:
		return RenderProjectPlanPDF(content, projectName)
	case KindCommunicationPlan:
		return RenderCommunicationPlanPDF(content, projectName)
	case KindStatementOfWork:
		return RenderStatementOfWorkPDF(content, projectName)
	case KindProjectClosure:
		return RenderProjectClosurePDF(content, projectName)
	case KindStakeholderAnalysis:
		return RenderStakeholderAnalysisPDF(content, projectName)
	case KindScopeStatement:
		return RenderScopeStatementPDF(content, projectName)
	case KindProjectBudget:
		return RenderProjectBudgetPDF(content, projectName)
	case KindRequirements:
		return RenderRequirementsPDF(content, projectName)
	case KindIssueLog:
		return RenderIssueLogPDF(content, projectName)
	case KindChangeRequest:
		return RenderChangeRequestPDF(content, projectName)
	case KindBusinessCase:
		return RenderBusinessCasePDF(content, projectName)
	case KindProcurementPlan:
		return RenderProcurementPlanPDF(content, projectName)
	case KindTeamCharter:
		return RenderTeamCharterPDF(content, projectName)
	case KindExecutionPlan:
		return RenderExecutionPlanPDF(content, projectName)
	case KindWBSDocument:
		return RenderWBSDocumentPDF(content, projectName)
	case KindRACIDocument:
		return RenderRACIDocumentPDF(content, projectName)
	case KindProjectProposal:
		return RenderProjectProposalPDF(content, projectName)
	case KindProjectSchedule:
		return RenderProjectSchedulePDF(content, projectName)
	case KindProjectBrief:
		return RenderProjectBriefPDF(content, projectName)
	case KindProjectOverview:
		return RenderProjectOverviewPDF(content, projectName)
	}

	// Generic fallback. Produces a presentable PDF for every Kind that
	// does not yet have a bespoke renderer — useful so the user can
	// always export what they typed even if the layout is plain.
	//
	// Adding a new bespoke renderer is documented in DEVELOPER_HANDBOOK.md §12.3.
	return renderGenericPDF(kind, content, projectName)
}

func renderGenericPDF(kind Kind, content map[string]interface{}, projectName string) ([]byte, error) {
	def, ok := Get(kind)
	if !ok {
		return nil, fmt.Errorf("documents: unknown kind %q", kind)
	}

	pdf := newDocPDF("P")
	pdf.SetMargins(20, 18, 20)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetTitle(def.Name, true)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 12, projectName)
	pdf.Ln(12)
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(110, 110, 110)
	pdf.Cell(0, 6, def.Name)
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0)

	// Render fields in registry order so the document layout is
	// deterministic across exports.
	fields := EffectiveFields(kind)
	for _, f := range fields {
		v, ok := content[f.Key]
		if !ok {
			continue
		}
		switch f.Type {
		case FieldStringArr:
			writeBulletSection(pdf, f.Label, toStringSlice(v))
		case FieldObjectArr:
			if objs := toObjectSlice(v); len(objs) > 0 {
				writeHeading(pdf, f.Label)
				renderObjectArray(pdf, f, objs)
			}
		case FieldText:
			writeSection(pdf, f.Label, toString(v))
		case FieldNumber:
			if n, ok := v.(float64); ok && n != 0 {
				writeKV(pdf, f.Label, fmt.Sprintf("%.2f", n))
			}
		case FieldBool:
			if b, ok := v.(bool); ok {
				writeKV(pdf, f.Label, fmt.Sprintf("%t", b))
			}
		default:
			if s := toString(v); s != "" {
				writeKV(pdf, f.Label, s)
			}
		}
	}

	pdf.SetY(-20)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 5,
		fmt.Sprintf("Generated by GoPMgr at %s", time.Now().UTC().Format(time.RFC3339Nano)),
		"", 0, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderObjectArray(pdf *fpdf.Fpdf, f Field, objs []map[string]interface{}) {
	if len(f.ObjectShape) == 0 || len(objs) == 0 {
		return
	}
	// Build header from sub-field labels.
	headers := make([]string, len(f.ObjectShape))
	keys := make([]string, len(f.ObjectShape))
	for i, sub := range f.ObjectShape {
		headers[i] = sub.Label
		keys[i] = sub.Key
	}
	// Stringify cells (numbers/dates rendered as %v).
	stringified := make([]map[string]interface{}, len(objs))
	for i, obj := range objs {
		row := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			row[k] = toString(obj[k])
		}
		stringified[i] = row
	}
	// Compute widths: equal split across the available 170mm body.
	w := 170.0 / float64(len(headers))
	widths := make([]float64, len(headers))
	for i := range widths {
		widths[i] = w
	}
	writeTable(pdf, headers, widths, stringified, keys)
}

func toString(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%.2f", x)
	case bool:
		return fmt.Sprintf("%t", x)
	}
	return fmt.Sprintf("%v", v)
}

func toStringSlice(v interface{}) []string {
	if arr, ok := v.([]interface{}); ok {
		out := make([]string, 0, len(arr))
		for _, x := range arr {
			out = append(out, toString(x))
		}
		return out
	}
	return nil
}

func toObjectSlice(v interface{}) []map[string]interface{} {
	if arr, ok := v.([]interface{}); ok {
		out := make([]map[string]interface{}, 0, len(arr))
		for _, x := range arr {
			if obj, ok := x.(map[string]interface{}); ok {
				out = append(out, obj)
			}
		}
		return out
	}
	return nil
}

// KindsSorted returns every Kind in stable name order. Useful for
// menus.
func KindsSorted() []Kind {
	defs := All()
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	out := make([]Kind, len(defs))
	for i, d := range defs {
		out[i] = d.Kind
	}
	return out
}
