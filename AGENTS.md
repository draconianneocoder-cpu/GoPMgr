<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Agent Operating Guide

This repository is GoPMgr, a local-first desktop project controls
application built with a Go backend, Wails v3, and a Svelte 5 frontend.
Treat this file as the first project-local instruction source for
automated engineering work.

## Session Spin-Up

1. Confirm the active checkout with `pwd` and `git status --short`.
   Older handoffs may mention stale clone paths; verify before running
   release or build commands.
2. When present, read local `session-notes.md` and the relevant files in
   `.agent_memory/` before choosing backlog work or changing behavior.
3. Inspect existing source before editing. Do not read `DEVELOPER_HANDBOOK.md` in
   full unless required; it is a large developer handbook. Prefer targeted
   `rg` searches and small snippets.
4. Assume the worktree can contain unrelated user or agent changes. Do
   not revert, restage, or flatten changes you did not make.

## Work Rules

For build-tool provisioning or new shell/workflow assurance gates, read
[ADR-013](docs/design/ADR-013-engineering-build-assurance.md) and its
[baseline](docs/engineering/adr-013-baseline.md). Make remains canonical;
Wails' internal Taskfiles are subordinate build details. mise is approved for
staged auxiliary provisioning only. P0 checkout/performance characterization
is complete on the recorded macOS/Ubuntu hosts. P1 opt-in provisioning is
implemented; see [setup and proof limits](docs/engineering/adr-013-provisioning.md).
P2's [local ShellCheck inventory and adjudication](docs/engineering/adr-013-shellcheck.md)
is implemented. P3's [local shfmt style and read-only check](docs/engineering/adr-013-shfmt.md)
is implemented with separately reviewed normalization. Maintainer acceptance
precedes mandatory lint adoption. P4's [local workflow correctness check](docs/engineering/adr-013-actionlint.md)
is implemented. P5's [local workflow security policy](docs/engineering/adr-013-zizmor.md)
is implemented under the approved regular-persona contract; its documented
credential-detection gap is covered by the separately owner-approved
[P6 configuration guard](docs/engineering/adr-013-checkout-guard-plan.md).
P6-B makes [licensing validation read-only](docs/engineering/adr-013-license-check.md)
with explicit Finder cleanup separate. P6-D requires
[successful licensing at every release entry point](docs/engineering/adr-013-release-license-plan.md),
with no skip mode. P6-C [pins the CI vulnerability scanner](docs/engineering/adr-013-govulncheck.md).
Other P6 repairs and P7–P9 remain separate.
Do not infer new runtime, hosted-CI or release authority from this direction.

- Plan first for any multi-file change. Keep the plan scoped to a
  reviewable slice.
- For ADR-010 or ADR-011 work (plugins, MPXJ, worker IPC, large
  schedules, or AI/MCP), load
  `docs/agent/ADR-010-architecture-preservation-agent-prompt.md` first
  and follow it. The architecture was accepted on 2026-10-24; only the
  work slices authorized in
  `docs/review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md` may proceed.
- For ADR-012 work (`internal/analytics`, DataFusion, or the Rust
  analytics worker), load
  `docs/agent/adr-012-coding-agent-instructions.md` first and follow it.
  On 2026-11-02 the maintainer retired DuckDB in favour of a Go-native engine
  for both analytics operations (see
  `docs/review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md`). The DataFusion
  spike under `spikes/adr012-datafusion/` stays isolated evidence; production
  use of the Rust worker is still not authorized.
- Use repository patterns instead of inventing new architecture.
- Read existing files before editing. Do not guess APIs, versions,
  flags, commit SHAs, or package names; verify by reading code or
  documentation first.
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
- Be concise in public documentation. Summarize long generated output
  and point to the command or artifact that reproduces it.

## Go Engineering Rules

- Keep package boundaries domain-oriented. Do not add broad `common` or
  `util` packages unless a narrow existing pattern already supports it.
- Prefer synchronous APIs. Start background goroutines only when the
  owner, cancellation path, and cleanup lifecycle are explicit.
- Use guard clauses for error and edge handling. Keep the normal path
  shallow and readable.
- Return `error` as the last value, wrap it with operation context, and
  use `errors.Is` / `errors.As` for inspection.
- Run `gofmt` or `goimports` after Go edits. Keep imports grouped as
  standard library, third-party, then first-party packages.
- Use table-driven tests for multi-case logic. Use `t.Cleanup` for test
  cleanup that must run after `t.Fatal`.
- For complex comparisons, use `reflect.DeepEqual` (the project's current
  practice — no comparison-diff dependency such as `go-cmp` is vendored;
  don't add one without a deliberate dependency-addition decision, per
  DEPENDENCIES.md).
- Use `crypto/rand` for security-sensitive randomness. In recoverable
  paths such as IDs, salts, or recovery codes, use
  `io.ReadFull(rand.Reader, buf)` so entropy failures return errors
  instead of terminating the process.
- Use `snake_case` JSON tags on Wails-bound structs; the TypeScript
  surface and existing frontend code expect those wire names.

## Project Invariants

- `go.mod` requires Go 1.27.3 or later and pins Wails v3.1.0. CGO is required.
- The root `main.go` embeds repo-root `frontend/dist` through `go:embed` and
  calls `desktop.Main`. The Wails service `App` lives in `internal/desktop`;
  its 209 exported methods are frontend API, listed in `code-map/app-api.json`.
  `frontend/src/lib/desktop-api.ts` re-exports the generated App namespace from
  `frontend/bindings/gopmgr/internal/desktop/index.ts`. Generate real bindings
  before frontend checks; do not restore ambient `window.go` declarations.
  `domain-types.d.ts` contains frontend editor views, and `desktop-dtos.ts`
  explicitly normalizes nullable collections for those views. Raw generated
  calls retain wire null/empty distinctions. `desktop-runtime.ts` adapts
  `Events.On` and preserves listener-before-guard readiness and unsubscribe.
- `make build` calls `scripts/wails-build.sh` and the reviewed v3 Taskfiles.
  Use the task-local `.tmp/wails-v3/bin/wails3` CLI pinned to v3.1.0 and npm
  `@wailsio/runtime` 3.1.0. Product metadata is `build/config.yml` `info.version`.
  CGO Linux builds use GTK 4 and WebKitGTK 6.0 without legacy backend tags.
- The migration was approved on 2026-11-03; package/release qualification is
  separate. macOS 13 and Ubuntu 24.04 execution remain mandatory release gates
  on currently unavailable hosts. Windows NSIS qualification remains pending.
  Run CI-equivalent checks on local macOS/local Ubuntu; do not launch hosted CI.
- `system.db` stores local account metadata. Per-project `.gopmgr`
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
go test . ./internal/...
go test -race . ./internal/...
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
