// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package wbs defines the proposed unified scope model. It is not registered as
// a chart kind and does not migrate or mutate existing project data.
package wbs

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const (
	SchemaVersion      = 2
	MaxPayloadBytes    = 8 << 20
	MaxNodes           = 10_000
	MaxLevels          = 64
	MaxTextBytes       = 64 << 10
	MaxIdentifierBytes = 256
	MaxAssociations    = 50_000
)

var (
	ErrInvalid           = errors.New("wbs: invalid aggregate")
	ErrUnsupportedSchema = errors.New("wbs: unsupported schema")
	ErrLimit             = errors.New("wbs: limit exceeded")
)

type Aggregate struct {
	SchemaVersion int            `json:"schema_version"`
	Revision      string         `json:"revision"`
	Root          *Node          `json:"root"`
	ActivityLinks []ActivityLink `json:"activity_links,omitempty"`
	CostLinks     []CostLink     `json:"cost_links,omitempty"`
}

type Node struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	Number         string      `json:"number,omitempty"` // Preserved display metadata, never identity.
	Note           string      `json:"note,omitempty"`
	Owner          string      `json:"owner,omitempty"`
	Effort         float64     `json:"effort,omitempty"` // Legacy generic units, not duration or money.
	Classification string      `json:"classification,omitempty"`
	Dictionary     *Dictionary `json:"dictionary,omitempty"`
	Children       []*Node     `json:"children,omitempty"`
}

type Dictionary struct {
	Description        string `json:"description,omitempty"`
	ScopeIn            string `json:"scope_in,omitempty"`
	ScopeOut           string `json:"scope_out,omitempty"`
	AcceptanceCriteria string `json:"acceptance_criteria,omitempty"`
}

type ActivityLink struct {
	NodeID          string `json:"node_id"`
	ScheduleChartID string `json:"schedule_chart_id"`
	ActivityID      string `json:"activity_id"`
}

type CostLink struct {
	NodeID      string `json:"node_id"`
	CostEntryID string `json:"cost_entry_id"`
}

// Validate checks structure and encoded size without modifying the aggregate.
// External source existence, ownership and authorization require a future
// transactional boundary. Callers must not mutate the aggregate concurrently.
func Validate(a Aggregate) error {
	if a.SchemaVersion != SchemaVersion {
		return ErrUnsupportedSchema
	}
	if _, err := revisionValue(a.Revision); err != nil {
		return err
	}
	if a.Root == nil {
		return fmt.Errorf("%w: missing root", ErrInvalid)
	}
	if len(a.ActivityLinks) > MaxAssociations || len(a.CostLinks) > MaxAssociations-len(a.ActivityLinks) {
		return fmt.Errorf("%w: associations", ErrLimit)
	}
	type visit struct {
		node  *Node
		level int
	}
	stack := []visit{{a.Root, 1}}
	nodes := make(map[string]*Node)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if v.level > MaxLevels || len(nodes) >= MaxNodes {
			return fmt.Errorf("%w: hierarchy", ErrLimit)
		}
		if v.node == nil {
			return fmt.Errorf("%w: nil child", ErrInvalid)
		}
		n := v.node
		if err := identifier(n.ID); err != nil {
			return fmt.Errorf("node id: %w", err)
		}
		if _, exists := nodes[n.ID]; exists {
			return fmt.Errorf("%w: repeated node identity", ErrInvalid)
		}
		nodes[n.ID] = n
		for _, s := range []string{n.Title, n.Number, n.Note, n.Owner, n.Classification} {
			if err := text(s); err != nil {
				return err
			}
		}
		if n.Dictionary != nil {
			d := n.Dictionary
			for _, s := range []string{d.Description, d.ScopeIn, d.ScopeOut, d.AcceptanceCriteria} {
				if err := text(s); err != nil {
					return err
				}
			}
		}
		if math.IsNaN(n.Effort) || math.IsInf(n.Effort, 0) {
			return fmt.Errorf("%w: nonfinite effort", ErrInvalid)
		}
		if len(n.Children) > MaxNodes-len(nodes) || len(stack) > MaxNodes-len(nodes)-len(n.Children) {
			return fmt.Errorf("%w: nodes", ErrLimit)
		}
		for i := len(n.Children) - 1; i >= 0; i-- {
			stack = append(stack, visit{n.Children[i], v.level + 1})
		}
	}
	activities := make(map[[2]string]bool)
	for _, link := range a.ActivityLinks {
		if err := leaf(nodes, link.NodeID); err != nil {
			return err
		}
		if err := identifier(link.ScheduleChartID); err != nil {
			return err
		}
		if err := identifier(link.ActivityID); err != nil {
			return err
		}
		key := [2]string{link.ScheduleChartID, link.ActivityID}
		if activities[key] {
			return fmt.Errorf("%w: repeated activity association", ErrInvalid)
		}
		activities[key] = true
	}
	costs := make(map[string]bool)
	for _, link := range a.CostLinks {
		if err := leaf(nodes, link.NodeID); err != nil {
			return err
		}
		if err := identifier(link.CostEntryID); err != nil {
			return err
		}
		if costs[link.CostEntryID] {
			return fmt.Errorf("%w: repeated cost association", ErrInvalid)
		}
		costs[link.CostEntryID] = true
	}
	return encodedSize(a)
}

func leaf(nodes map[string]*Node, id string) error {
	if err := identifier(id); err != nil {
		return err
	}
	n, exists := nodes[id]
	if !exists || len(n.Children) != 0 {
		return fmt.Errorf("%w: association must target an existing leaf", ErrInvalid)
	}
	return nil
}

func identifier(s string) error {
	if len(s) > MaxIdentifierBytes {
		return fmt.Errorf("%w: identifier", ErrLimit)
	}
	if strings.TrimSpace(s) == "" || !utf8.ValidString(s) {
		return fmt.Errorf("%w: blank or invalid identifier", ErrInvalid)
	}
	return nil
}

func text(s string) error {
	if len(s) > MaxTextBytes {
		return fmt.Errorf("%w: text", ErrLimit)
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: invalid text encoding", ErrInvalid)
	}
	return nil
}
