<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Agent Operating Guide

This repository is PMForge, a local-first desktop project controls
application built with a Go backend, Wails v2, and a Svelte 5 frontend.
Treat this file as the first project-local instruction source for
automated engineering work.

## Session Spin-Up

1. Confirm the active checkout with `pwd` and `git status --short`.
   Older handoffs may mention stale clone paths; verify before running
   release or build commands.
2. Read `session-notes.md` and the relevant files in `.agent_memory/`
   before choosing backlog work or changing behavior.
3. Inspect existing source before editing. Do not read `AGENT.md` in
   full unless required; it is a large legacy handbook. Prefer targeted
   `rg` searches and small snippets.
4. Assume the worktree can contain unrelated user or agent changes. Do
   not revert, restage, or flatten changes you did not make.

## Work Rules

- Plan first for any multi-file change. Keep the plan scoped to a
  reviewable slice.
- Use repository patterns instead of inventing new architecture.
- Add or update focused tests when behavior changes. Documentation-only
  changes should still be checked with `git diff --check`.
- Preserve REUSE/SPDX compliance. New tracked files need SPDX metadata
  directly or a `REUSE.toml` annotation when inline metadata is not
  possible.
- Keep `session-notes.md`, `.agent_memory/`, and other private handoff
  material out of public release scope unless the user explicitly asks
  otherwise.
- Stage intentionally by path or hunk. Avoid broad `git add .` in a
  dirty worktree.

## Project Invariants

- `go.mod` pins Go 1.26.3 and Wails v2.9.2. CGO is required.
- The production build embeds `cmd/pmforge/frontend/dist` through
  `go:embed`. `make build` creates that tree by building `frontend/dist`
  and copying it into the Go package.
- `system.db` stores local account metadata. Per-project `.pmforge`
  databases hold project data and are SQLCipher-capable through
  `internal/sqlitedriver`.
- PDF/A and PAdES are release-critical features. Preserve the order:
  render PDF content, apply PDF/A metadata and output intent, then apply
  PAdES as the final PDF mutation.
- Frontend changes need more than type checking. The runtime smoke gate
  exists because `svelte-check` and Vite build can miss load-time Svelte
  rune misuse.

## Useful Commands

```sh
go test ./cmd/... ./internal/...
go test -race ./cmd/... ./internal/...
npm --prefix frontend run check
npm --prefix frontend run build
make frontend-smoke
make check-pades
make check-pades-external
make check-pdfa
make license-check
make release-scope
make check-release
git diff --check && git diff --cached --check
```

Use focused commands during development, then broaden verification before
claiming completion. Do not claim a gate passes unless it was run in the
current session.

## Completion

Before ending a meaningful task:

1. Summarize changed files and verification evidence.
2. Record durable project state in `.agent_memory/` when the work changes
   architecture, release gates, security posture, or future handoff steps.
3. Leave unresolved issues explicit, especially when a command could not
   be run because of missing tools or existing unrelated failures.
