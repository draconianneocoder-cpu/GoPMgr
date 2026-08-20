<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Developer Handbook

This handbook explains the stable development conventions. Source, tests, and
Architecture Decision Records are authoritative when this document conflicts
with them. Historical investigations belong in Git history, not in this guide.

## 1. Orientation

GoPMgr is a local-first desktop application. The root package is the Wails
application surface; domain packages live under `internal/`; the Svelte
frontend lives under `frontend/`. Read [ARCHITECTURE.md](ARCHITECTURE.md)
before changing package boundaries.

The required toolchain is Go 1.26.6 and **Wails v2.13.0**. Install the matching
CLI with:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

## 2. Repository layout

```text
main.go                 Wails entry point and bound application methods
app_*.go                UI adapters and application orchestration
internal/               Domain packages and infrastructure
frontend/               Svelte UI and generated Wails bridge types
docs/                   User, design, and release documentation
scripts/                Repeatable checks, packaging, and validation tools
```

Keep Wails-bound methods small. Validate bridge input, acquire the required
application state, call a focused package, and return a bridge-safe result.
Put reusable workflows such as report assembly or signed export orchestration
in an internal application package rather than expanding `main.go`.

## 3. Data and compatibility

`system.db` stores local account bootstrap data. Per-project `.gopmgr`
databases hold project records. SQLCipher support is centralized in
`internal/sqlitedriver`; do not register a second SQLite driver.

Treat existing project files, backup formats, account metadata, and exported
wire formats as compatibility contracts. A migration must be additive or have
an explicit, tested recovery path. Test pre-change fixtures rather than only
round-tripping data through the new implementation.

## 4. Coding conventions

- Prefer narrow, domain-oriented packages. Do not create catch-all `common` or
  `util` packages.
- Use guard clauses and wrap returned errors with operation context. Check
  sentinels with `errors.Is` or `errors.As`; never match error strings.
- Keep Go imports grouped as standard library, third party, then first party.
  Run `gofmt` after Go edits.
- Use `snake_case` JSON tags on Wails-bound types.
- Keep exported API documentation accurate and update it with behavior.
- Do not add background work without a clear owner, cancellation path, and
  cleanup lifecycle.

See [STYLE.md](STYLE.md) and [ERROR_HANDLING.md](ERROR_HANDLING.md) for the
full local rules.

## 5. Build and test

Run focused checks while developing, then expand them with the risk of the
change:

```sh
go test . ./internal/...
go test -race . ./internal/...
npm --prefix frontend run check
npm --prefix frontend run test
make frontend-smoke
make verify
```

`make build` builds `frontend/dist` and embeds it through `main.go`. Direct Go
tests also need that output present. Release-sensitive work additionally needs
the gates in [TESTING.md](TESTING.md), including `make check-release`.

CI's `govulncheck` job queries a live vulnerability database, so a commit with
no code change can go from green to red (or back) between runs as new
advisories publish. A stdlib finding usually means a newer Go patch release
fixes it — bump `go.mod`'s `go` directive and confirm `govulncheck -tags
webkit2_41 ./...` (the default build) and `govulncheck -tags
duckdb,webkit2_41 ./internal/analytics/...` (the DuckDB build, scoped to that
package — see the Makefile's own note on avoiding a bare `./...`, which can
expose unrelated generated-frontend Go packages) both come back clean before
assuming the repository itself regressed.

## 6. Concurrency and lifecycle invariants

The application state protects the active user and project database with its
own lock. A method that reads or changes project state must use the established
application helpers, hold the lock only for the required scope, and never
publish partially initialized state.

Frontend timers, subscriptions, and asynchronous saves need explicit cleanup
when a component is destroyed or a project changes. A stale completion must
not update a new project, overwrite newer editor state, or hide a failed save.
Do not introduce goroutines, timers, or listeners merely to make a call appear
asynchronous. When one is necessary, document its owner, cancellation, error
handling, and shutdown behavior beside the code.

## 7. Resource and security boundaries

Constrain every frontend-supplied project path to the signed-in user's project
directory before reading or writing it. Use temporary directories in tests and
avoid user-specific paths in source and documentation.

Never log or commit passwords, recovery codes, encryption keys, private
signing keys, real certificates, or real project data. Test-only credentials
must be clearly identified as deterministic fixtures and narrowly allowlisted.

For PDF output, render content first, then PDF/A metadata and output intent,
then PAdES as the final mutation. A change to a signed PDF requires fresh
signing and validator evidence.

## 8. Feature work

Before adding a feature, identify its domain package, persistence changes,
Wails adapter, frontend consumer, export behavior, and tests. Add only the
boundaries that the feature needs. Update [ARCHITECTURE.md](ARCHITECTURE.md),
an ADR, or the user guide when the change affects a durable contract.

For a new chart or document kind, register it through the existing registry,
validate persisted payloads, provide a focused renderer or exporter, and add
round-trip plus invalid-input coverage. Keep bespoke renderers self-contained;
share only stable primitives with demonstrated reuse.

## 9. Release and documentation

Do not make public claims from source inspection alone. Run the relevant
validator or keep the limitation explicit. `make release-scope` protects the
repository's encryption, PDF/A, and PAdES claims; `make license-check` checks
REUSE metadata.

Public documentation should describe current behavior, use portable commands,
and link to the command or source that verifies a claim. Do not include local
machine paths, personal account names, credentials, or session history.

## 10. Working agreements

Read [AGENTS.md](AGENTS.md) before automated work. Keep a change reviewable,
test behavior rather than implementation details, and stage only the files you
intend to commit. If a required check cannot run, record that limitation
instead of inferring a pass.
