// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/gomutex/godocx"
	"github.com/gomutex/godocx/docx"

	"pmforge/internal/documents"
)

// RenderDocumentDOCX produces a Microsoft Word file for the given
// document. It walks the kind's Field definitions exactly like the
// generic PDF renderer in documents/charter.go, but emits headings,
// paragraphs, and tables via gomutex/godocx instead of gofpdf.
//
// Architecture:
//
//   - We chose gomutex/godocx after surveying pkg.go.dev: it's MIT,
//     pure Go, currently maintained, and has the high-level API the
//     PMForge field-walker needs (AddHeading / AddParagraph /
//     AddTable). No hand-rolled OOXML here.
//   - The function is field-driven, so any document kind that
//     populates its content JSON correctly produces a usable DOCX
//     — no per-kind code in this file.
//
// Used by documents.Render() when the caller asks for DOCX, and
// by the standalone Export menu's DOCX option.
func RenderDocumentDOCX(kind documents.Kind, contentJSON, projectName string) ([]byte, error) {
	def, ok := documents.Get(kind)
	if !ok {
		return nil, fmt.Errorf("export: unknown document kind %q", kind)
	}

	var content map[string]interface{}
	if contentJSON != "" {
		if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
			return nil, fmt.Errorf("export: invalid content JSON: %w", err)
		}
	}

	doc, err := godocx.NewDocument()
	if err != nil {
		return nil, fmt.Errorf("export: godocx new: %w", err)
	}

	// Title block
	if err := addHeadingDOCX(doc, projectName, 0); err != nil {
		return nil, err
	}
	if err := addHeadingDOCX(doc, def.Name, 1); err != nil {
		return nil, err
	}

	// Walk the kind's schema and emit content in registry order.
	for _, f := range documents.EffectiveFields(kind) {
		v, present := content[f.Key]
		if !present {
			continue
		}
		switch f.Type {
		case documents.FieldStringArr:
			arr := toStringSliceLocal(v)
			if len(arr) == 0 {
				continue
			}
			if err := addHeadingDOCX(doc, f.Label, 2); err != nil {
				return nil, err
			}
			for _, item := range arr {
				doc.AddParagraph("• " + item)
			}
		case documents.FieldObjectArr:
			objs := toObjectSliceLocal(v)
			if len(objs) == 0 {
				continue
			}
			if err := addHeadingDOCX(doc, f.Label, 2); err != nil {
				return nil, err
			}
			renderObjectArrayDOCX(doc, f, objs)
		case documents.FieldText:
			body := toStringLocal(v)
			if body == "" {
				continue
			}
			if err := addHeadingDOCX(doc, f.Label, 2); err != nil {
				return nil, err
			}
			doc.AddParagraph(body)
		case documents.FieldNumber:
			if n, ok := v.(float64); ok && n != 0 {
				p := doc.AddParagraph("")
				p.AddText(f.Label + ": ").Bold(true)
				p.AddText(fmt.Sprintf("%.2f", n))
			}
		case documents.FieldBool:
			if b, ok := v.(bool); ok {
				p := doc.AddParagraph("")
				p.AddText(f.Label + ": ").Bold(true)
				p.AddText(fmt.Sprintf("%t", b))
			}
		case documents.FieldChartRef:
			if id := toStringLocal(v); id != "" {
				p := doc.AddParagraph("")
				p.AddText(f.Label + ": ").Bold(true)
				p.AddText("(chart " + id + ")")
			}
		default:
			if s := toStringLocal(v); s != "" {
				p := doc.AddParagraph("")
				p.AddText(f.Label + ": ").Bold(true)
				p.AddText(s)
			}
		}
	}

	return renderDOCXToBytes(doc, "pmforge-docx")
}

func addHeadingDOCX(doc *docx.RootDoc, text string, level uint) error {
	if _, err := doc.AddHeading(text, level); err != nil {
		return fmt.Errorf("export: docx heading: %w", err)
	}
	return nil
}

func renderDOCXToBytes(doc *docx.RootDoc, tempPrefix string) (out []byte, err error) {
	// gomutex/godocx writes via a path; serialise to a temp file and read it
	// back so PMForge's export pipeline can continue returning []byte.
	tmp, err := os.CreateTemp("", tempPrefix+"-*.docx")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("export: close temp docx: %w; remove: %v", err, removeErr)
		}
		return nil, fmt.Errorf("export: close temp docx: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(tmpPath); err == nil && removeErr != nil && !os.IsNotExist(removeErr) {
			err = fmt.Errorf("export: remove temp docx: %w", removeErr)
		}
	}()

	if err := doc.SaveTo(tmpPath); err != nil {
		return nil, fmt.Errorf("export: docx save: %w", err)
	}
	out, err = os.ReadFile(tmpPath) // #nosec G304 -- tmpPath was created by os.CreateTemp in this function.
	if err != nil {
		return nil, fmt.Errorf("export: read temp docx: %w", err)
	}
	return out, nil
}

// renderObjectArrayDOCX writes an object-array field. We deliberately
// render it as a bullet list rather than a Word table, for two
// reasons:
//
//  1. godocx's table API surface (AddRow / AddCell / AddParagraph
//     chaining + return types) has shifted across minor versions,
//     so the safest cross-version code path is the paragraph-based
//     one that's been stable since v0.1.
//  2. Bulleted lists with bold labels still parse correctly when
//     the .docx is re-imported into Word / Google Docs / Pages.
//
// When a future version of godocx settles on a documented table
// signature, this function is the only place to upgrade.
func renderObjectArrayDOCX(doc *docx.RootDoc, f documents.Field, objs []map[string]interface{}) {
	if len(f.ObjectShape) == 0 {
		for i, obj := range objs {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			buf := bytes.Buffer{}
			for _, k := range keys {
				fmt.Fprintf(&buf, "%s: %v; ", k, obj[k])
			}
			doc.AddParagraph(fmt.Sprintf("(%d) %s", i+1, buf.String()))
		}
		return
	}
	for i, obj := range objs {
		// "1. " header line.
		p := doc.AddParagraph("")
		p.AddText(fmt.Sprintf("%d. ", i+1)).Bold(true)
		// One line per sub-field, "Label: value".
		for _, sub := range f.ObjectShape {
			val := fmt.Sprintf("%v", obj[sub.Key])
			if val == "<nil>" {
				val = ""
			}
			p := doc.AddParagraph("    ")
			p.AddText(sub.Label + ": ").Bold(true)
			p.AddText(val)
		}
	}
}

// ---- Local toString / toStringSlice / toObjectSlice helpers ----
//
// We duplicate the tiny helpers here so the export package doesn't
// drag the documents package's internal helpers across the public
// API boundary.

func toStringLocal(v interface{}) string {
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

func toStringSliceLocal(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		out = append(out, toStringLocal(x))
	}
	return out
}

func toObjectSliceLocal(v interface{}) []map[string]interface{} {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, x := range arr {
		if obj, ok := x.(map[string]interface{}); ok {
			out = append(out, obj)
		}
	}
	return out
}
