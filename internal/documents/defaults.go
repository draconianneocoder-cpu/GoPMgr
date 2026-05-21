// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "encoding/json"

// DefaultContent returns an empty, schema-valid JSON document for a
// given Kind. The frontend calls this when the user picks "New
// <kind>" from the menu so the editor opens with every field present
// (rather than typing-into-undefined surprises).
//
// The function walks the Kind's Definition.Fields and synthesises a
// zero value per Field.Type:
//
//   - string / text  → ""
//   - number         → 0
//   - bool           → false
//   - date           → "" (UI fills with today on focus)
//   - string_array   → []
//   - object_array   → [] (UI adds rows on demand)
//   - chart_ref      → ""
//
// Returns "{}" for unknown kinds so the GUI does not crash on a bad
// kind selector.
func DefaultContent(k Kind) string {
	def, ok := Get(k)
	if !ok {
		return "{}"
	}
	fields := def.Fields
	// The two paired kinds (Excel mirrors of Word docs) intentionally
	// have empty Fields lists in the registry; resolve those at
	// runtime to the canonical Word kind's fields.
	if len(fields) == 0 {
		switch k {
		case KindProjectCharterExcel:
			if d, ok := Get(KindProjectCharterWord); ok {
				fields = d.Fields
			}
		case KindProjectPlanExcel:
			if d, ok := Get(KindProjectPlanWord); ok {
				fields = d.Fields
			}
		}
	}

	m := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		m[f.Key] = zeroFor(f)
	}

	b, _ := json.Marshal(m)
	return string(b)
}

func zeroFor(f Field) interface{} {
	switch f.Type {
	case FieldNumber:
		return 0
	case FieldBool:
		return false
	case FieldStringArr:
		return []string{}
	case FieldObjectArr:
		return []map[string]interface{}{}
	default:
		// string, text, date, chart_ref
		return ""
	}
}

// EffectiveFields returns the schema fields actually rendered for a
// Kind, resolving the two Word/Excel pair aliases. Use this instead
// of `Get(k).Fields` when you need the full schema regardless of
// whether the kind is a Word or Excel variant.
func EffectiveFields(k Kind) []Field {
	def, ok := Get(k)
	if !ok {
		return nil
	}
	if len(def.Fields) > 0 {
		return def.Fields
	}
	switch k {
	case KindProjectCharterExcel:
		if d, ok := Get(KindProjectCharterWord); ok {
			return d.Fields
		}
	case KindProjectPlanExcel:
		if d, ok := Get(KindProjectPlanWord); ok {
			return d.Fields
		}
	}
	return nil
}
