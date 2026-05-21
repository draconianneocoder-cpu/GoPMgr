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
	doc.AddHeading(projectName, 0)
	doc.AddHeading(def.Name, 1)

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
			doc.AddHeading(f.Label, 2)
			for _, item := range arr {
				doc.AddParagraph("• " + item)
			}
		case documents.FieldObjectArr:
			objs := toObjectSliceLocal(v)
			if len(objs) == 0 {
				continue
			}
			doc.AddHeading(f.Label, 2)
			renderObjectArrayDOCX(doc, f, objs)
		case documents.FieldText:
			body := toStringLocal(v)
			if body == "" {
				continue
			}
			doc.AddHeading(f.Label, 2)
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

	// gomutex/godocx writes via a path or io.Writer; we serialise
	// to a temp file and read it back so the export pipeline can
	// hand back bytes (PMForge always returns []byte from
	// renderers).
	tmp, err := os.CreateTemp("", "pmforge-docx-*.docx")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := doc.SaveTo(tmpPath); err != nil {
		return nil, fmt.Errorf("export: docx save: %w", err)
	}
	return os.ReadFile(tmpPath)
}

// renderObjectArrayDOCX writes an object-array field. We deliberately
// render it as a bullet list rather than a Word table, for two
// reasons:
//
//   1. godocx's table API surface (AddRow / AddCell / AddParagraph
//      chaining + return types) has shifted across minor versions,
//      so the safest cross-version code path is the paragraph-based
//      one that's been stable since v0.1.
//   2. Bulleted lists with bold labels still parse correctly when
//      the .docx is re-imported into Word / Google Docs / Pages.
//
// When a future version of godocx settles on a documented table
// signature, this function is the only place to upgrade.
func renderObjectArrayDOCX(doc *godocx.RootDoc, f documents.Field, objs []map[string]interface{}) {
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
