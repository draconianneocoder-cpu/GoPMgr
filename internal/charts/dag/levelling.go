// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"gopmgr/internal/kernel"
)

// levellingDigestVersion prefixes every digest, so a later change to the
// canonical form below is told apart from a schedule change.
const levellingDigestVersion = "v1:"

// LevellingInputsDigest identifies the chart content resource levelling
// read: each task's duration, milestone flag, constraint as the kernel
// reads it (and its date, for date constraints) and assignments, and each
// link's ends, type and lag. Labels, progress, costs, estimates, levelled
// starts, split segments and computed fields are left out, so editing
// them never makes levelling look out of date. The project start date, the
// work calendar and resource capacity outside the chart are not included
// either (a documented limit).
func LevellingInputsDigest(doc LayeredDocument) string {
	type assignment struct {
		Resource   string   `json:"resource"`
		Units      float64  `json:"units"`
		CalendarID string   `json:"calendar_id"`
		MaxUnits   float64  `json:"max_units"`
		SkillTags  []string `json:"skill_tags"`
	}
	type node struct {
		ID             string       `json:"id"`
		Duration       float64      `json:"duration"`
		Milestone      bool         `json:"milestone"`
		Constraint     string       `json:"constraint"`
		ConstraintDate string       `json:"constraint_date"`
		Assignments    []assignment `json:"assignments"`
	}
	type edge struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Label string `json:"label"`
	}

	nodes := make([][]byte, 0, len(doc.Nodes))
	for _, n := range doc.Nodes {
		c := kernel.ConstraintType(strings.ToUpper(strings.TrimSpace(n.Constraint)))
		var constraint, date string
		switch {
		case c == kernel.AsLateAsPossible:
			constraint = string(c)
		case kernel.IsDateConstraint(c):
			constraint, date = string(c), n.ConstraintDate
		}
		canon := node{ID: n.ID, Duration: n.Duration, Milestone: n.Milestone, Constraint: constraint, ConstraintDate: date}
		for _, a := range n.Assignments {
			tags := slices.Clone(a.SkillTags)
			sort.Strings(tags)
			canon.Assignments = append(canon.Assignments, assignment{a.Resource, a.Units, a.CalendarID, a.MaxUnits, tags})
		}
		raw, _ := json.Marshal(canon) // plain strings, numbers and slices: cannot fail
		nodes = append(nodes, raw)
	}
	// The ID leads each encoding, so a byte order sorts by ID and breaks
	// ties between duplicate IDs by the rest of the node.
	slices.SortFunc(nodes, bytes.Compare)

	edges := make([]edge, 0, len(doc.Edges))
	for _, e := range doc.Edges {
		typ, lag := ParseLinkLabel(e.Label)
		edges = append(edges, edge{e.From, e.To, FormatLinkLabel(typ, lag)})
	}
	slices.SortFunc(edges, func(a, b edge) int {
		return strings.Compare(a.From+"\x00"+a.To+"\x00"+a.Label, b.From+"\x00"+b.To+"\x00"+b.Label)
	})

	h := sha256.New()
	for _, raw := range nodes {
		h.Write(raw)
		h.Write([]byte{'\n'})
	}
	h.Write([]byte("edges\n"))
	edgeRaw, _ := json.Marshal(edges) // plain strings: cannot fail
	h.Write(edgeRaw)
	return levellingDigestVersion + hex.EncodeToString(h.Sum(nil))
}

// LevellingIsStale reports whether the chart's scheduling inputs changed
// since levelling stored LevelledInputs, while some task still carries a
// levelled start or split segments that the change may have made wrong.
func LevellingIsStale(doc LayeredDocument) bool {
	if doc.LevelledInputs == "" {
		return false
	}
	levelled := slices.ContainsFunc(doc.Nodes, func(n LayeredNode) bool {
		return n.LevelStart != "" || len(n.WorkSegments) > 0
	})
	return levelled && LevellingInputsDigest(doc) != doc.LevelledInputs
}
