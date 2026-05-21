// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package agile implements PMForge's Software-Dev Pack: Kanban
// boards, sprints, work items, and DORA metrics.
//
// File map:
//
//	agile.go   types (Board, Column, WorkItem, Sprint, Deployment)
//	           plus the PackEnabled in-memory toggle.
//	store.go   CRUD against the agile_* SQLite tables.
//	dora.go    DORA metric computation and classification.
//
// The package is opt-in: GUI entry points are hidden when
// PackEnabled is false. The underlying tables are still created
// at migrate time so re-enabling is lossless.
//
// See AGENT.md §8 ("Feature coverage") for the active checklist.
package agile

// (Intentionally no declarations — agile.go owns the symbols.)
