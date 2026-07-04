<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Testing

PMForge uses focused package tests during development and broader gates
before release or handoff. Do not claim a command passes unless it was
run in the current session.

## Fast Local Checks

```sh
go test . ./internal/...
npm --prefix frontend run check
git diff --check && git diff --cached --check
```

Use package-scoped variants while developing a narrow slice:

```sh
go test -count=1 ./internal/db
go test -count=1 ./internal/users ./internal/crypto
go test -count=1 .                 # root main package (App methods, CLI dispatch)
```

## Race and Runtime Checks

```sh
go test -race . ./internal/...
make frontend-smoke
```

Run race tests for concurrency-sensitive backend work and before
release claims. Run the frontend smoke gate for frontend changes because
it catches module-load and SSR-render failures that `svelte-check` and
`vite build` can miss.

## Frontend Checks

```sh
npm --prefix frontend run check
npm --prefix frontend run test   # Vitest component + unit tests (jsdom)
npm --prefix frontend run build
npm --prefix frontend run lint
make frontend-stability          # svelte-check + regressions + Vitest
make frontend-build-budget
make frontend-smoke
```

Component behaviour that `svelte-check` can only type-check is covered by
Vitest + `@testing-library/svelte` (jsdom). Presentational components (e.g.
`GanttBars.svelte`) render from props with no Wails bridge, so they mount
directly in tests; pure geometry lives in sibling `*_geometry.ts` modules
with fast unit tests. `make frontend-stability` runs the Vitest suite, so it
is part of `make verify` and `make check-release`. Test files
(`*.test.ts` / `*.spec.ts`) are excluded from the app `svelte-check`.

`make build` runs `wails build`, which builds the frontend into
`frontend/dist` and embeds it via the root `main.go` `go:embed` directive.
When running the Go gates directly (`go test . ...`), build the frontend
first (`make frontend-build-budget` or `npm --prefix frontend run build`)
so `frontend/dist` exists for the embed to compile.

## Document and PDF Gates

```sh
make check-pdfa
make check-pades
make check-pades-external
```

`make check-pdfa` is strict by default. It needs veraPDF available
directly or through Docker and fails if conformance cannot be verified.
`make check-pades` is the deterministic local PAdES invariant gate.
`make check-pades-external` uses installed external validators such as
OpenSSL, qpdf, pdfsig, veraPDF, and DSS when present.

## Release Gates

```sh
make license-check
make release-scope
make memory-scan
make check-release
```

`make check-release` is the final gate. It currently covers version
consistency, REUSE/SPDX, frontend build budget, release-scope guards,
frontend stability, frontend runtime smoke, memory-safety scan, Go race
tests, production build, PDF/A-3 validation, and PAdES local validation.

Run `make license-check` after adding files or generated assets. Run
`make release-scope` after documentation changes that touch release
claims, especially PDF/A, PAdES, encryption, or public-repo hygiene.

## Test Style

- Prefer table tests for parser, scheduler, renderer, and data-migration
  cases.
- Use temporary directories for database, export, and filesystem tests.
- Preserve deterministic fixtures. Avoid tests that depend on wall-clock
  time unless the clock is injected or fixed.
- For encryption work, test wrong-key rejection, keyless rejection,
  integrity checks, file-header encryption, and migration row parity.
- For PDF work, test structural invariants in addition to byte output.
  PDF signatures and metadata often require validator evidence, not just
  byte containment.
- For frontend regressions, add the narrowest check that catches the
  original failure class and keep the runtime smoke gate in mind.
