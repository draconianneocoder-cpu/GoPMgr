// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"fmt"

	"pmforge/internal/agile"
	"pmforge/internal/db"
	"pmforge/internal/documents"
)

// Seeder applies seed actions to a freshly-created project. It wraps
// the project's *db.Database and the matching agile.Store so the
// dispatcher below stays narrow.
type Seeder struct {
	DB        *db.Database
	ProjectID string
}

// NewSeeder constructs a Seeder bound to a database + project.
func NewSeeder(d *db.Database, projectID string) *Seeder {
	return &Seeder{DB: d, ProjectID: projectID}
}

// Apply dispatches each seed string to its handler. Unknown seeds
// are silently skipped (so adding a JDM row that names a yet-to-be-
// implemented seed doesn't crash the project-creation flow).
//
// Returns a slice describing what was created so the GUI can show
// the user a "we set this up for you" toast.
type SeedReceipt struct {
	Seed string `json:"seed"`
	Kind string `json:"kind"` // "chart" | "document" | "board" | "sprint"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Apply runs every seed in order. The first error short-circuits;
// receipts of seeds that succeeded BEFORE the failure are returned
// alongside it so the caller can decide whether to keep or undo.
func (s *Seeder) Apply(seeds []string) ([]SeedReceipt, error) {
	out := make([]SeedReceipt, 0, len(seeds))
	for _, seed := range seeds {
		r, err := s.applyOne(seed)
		if err != nil {
			return out, fmt.Errorf("seed %s: %w", seed, err)
		}
		if r != nil {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (s *Seeder) applyOne(seed string) (*SeedReceipt, error) {
	switch seed {
	case "kanban":
		return s.seedKanban()
	case "backlog":
		return s.seedBacklog()
	case "sprint1":
		return s.seedFirstSprint()

	// Charts
	case "wbs":
		return s.seedChart("wbs", "Work Breakdown Structure",
			`{"root":{"id":"r","title":"Project root","children":[]}}`)
	case "cpm":
		return s.seedChart("cpm", "Project Schedule (CPM)",
			`{"nodes":[],"edges":[]}`)
	case "fishbone":
		return s.seedChart("fishbone", "Root cause (Fishbone)",
			`{"effect":"","categories":[]}`)
	case "control":
		return s.seedChart("control", "Process Control",
			`{"x":[],"y":[],"mean":0,"ucl":0,"lcl":0}`)
	case "pareto":
		return s.seedChart("pareto", "Pareto Analysis",
			`{"items":[]}`)
	case "cumulative_flow":
		return s.seedChart("cumulative_flow", "Cumulative Flow",
			`{"days":[],"states":{},"state_order":[]}`)
	case "swot":
		return s.seedChart("swot", "SWOT",
			`{"strengths":[],"weaknesses":[],"opportunities":[],"threats":[]}`)

	// Documents
	case "charter":
		return s.seedDocument("charter_word", "Project Charter")
	case "plan_word":
		return s.seedDocument("plan_word", "Project Plan")
	case "statement_of_work":
		return s.seedDocument("statement_of_work", "Statement of Work")
	case "scope_statement":
		return s.seedDocument("scope_statement", "Scope Statement")
	case "risk_register":
		return s.seedDocument("risk_register", "Risk Register")
	case "communication_plan":
		return s.seedDocument("communication_plan", "Communication Plan")
	case "status_report":
		return s.seedDocument("status_report", "Initial Status Report")
	case "stakeholder_analysis_doc":
		return s.seedDocument("stakeholder_analysis_doc", "Stakeholder Analysis")
	}
	// Unknown seed — JDM may name a seed the binary doesn't know yet.
	return nil, nil
}

// seedChart writes one new chart record with the given starter data
// and returns a SeedReceipt for the GUI.
func (s *Seeder) seedChart(kind, title, data string) (*SeedReceipt, error) {
	c, err := s.DB.SaveChart(db.Chart{
		ProjectID: s.ProjectID,
		Kind:      kind,
		Title:     title,
		Data:      data,
	})
	if err != nil {
		return nil, err
	}
	return &SeedReceipt{Seed: kind, Kind: "chart", ID: c.ID, Name: c.Title}, nil
}

// seedDocument creates a new document with the kind's default
// content (computed by documents.DefaultContent).
func (s *Seeder) seedDocument(kind, title string) (*SeedReceipt, error) {
	def, ok := documents.Get(documents.Kind(kind))
	if !ok {
		return nil, fmt.Errorf("unknown document kind %q", kind)
	}
	d, err := s.DB.SaveDocument(db.Document{
		ProjectID: s.ProjectID,
		Kind:      kind,
		Title:     coalesce(title, def.Name),
		Content:   documents.DefaultContent(documents.Kind(kind)),
		Version:   1,
		Status:    "draft",
	})
	if err != nil {
		return nil, err
	}
	return &SeedReceipt{Seed: kind, Kind: "document", ID: d.ID, Name: d.Title}, nil
}

// seedKanban ensures the default Kanban board exists. The agile
// package's EnsureDefaultBoard already creates it on first access,
// so this seed is effectively idempotent — calling it twice is fine.
func (s *Seeder) seedKanban() (*SeedReceipt, error) {
	store := agile.NewStore(s.DB.Conn, s.ProjectID)
	b, err := store.EnsureDefaultBoard()
	if err != nil {
		return nil, err
	}
	return &SeedReceipt{Seed: "kanban", Kind: "board", ID: b.ID, Name: b.Name}, nil
}

// seedBacklog seeds three placeholder work items in the backlog so a
// new Scrum/Kanban project doesn't open with an empty list.
func (s *Seeder) seedBacklog() (*SeedReceipt, error) {
	store := agile.NewStore(s.DB.Conn, s.ProjectID)
	for i, title := range []string{
		"Define the first user story",
		"Identify the project's MVP scope",
		"Schedule the team kickoff",
	} {
		if _, err := store.SaveWorkItem(agile.WorkItem{
			ProjectID: s.ProjectID,
			Type:      agile.WorkItemStory,
			Title:     title,
			State:     "backlog",
			Priority:  agile.PrioMedium,
			OrderIdx:  i,
		}); err != nil {
			return nil, err
		}
	}
	return &SeedReceipt{Seed: "backlog", Kind: "board", ID: "", Name: "Seeded backlog (3 items)"}, nil
}

// seedFirstSprint creates a planning-state "Sprint 1" so the user
// sees a target iteration on day one.
func (s *Seeder) seedFirstSprint() (*SeedReceipt, error) {
	store := agile.NewStore(s.DB.Conn, s.ProjectID)
	sp, err := store.SaveSprint(agile.Sprint{
		ProjectID: s.ProjectID,
		Name:      "Sprint 1",
		Goal:      "First iteration — establish the team's rhythm.",
		Status:    agile.SprintPlanning,
		Capacity:  20,
	})
	if err != nil {
		return nil, err
	}
	return &SeedReceipt{Seed: "sprint1", Kind: "sprint", ID: sp.ID, Name: sp.Name}, nil
}

func coalesce(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
