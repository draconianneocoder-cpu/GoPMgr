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
directly in tests; pure geometry and action message-builders live in sibling
modules (`*_geometry.ts`, `leveling_messages.ts`) with fast unit tests. Async
handler glue is covered by mounting the real editor against a mocked
`window.go.main.App` (see `GanttEditor.test.ts`): click an action, assert the
bridge result reaches the status DOM. `make frontend-stability` runs the Vitest suite, so it
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
make check-pades-trusted
make pades-harness-tests
```

`make check-pdfa` is strict by default. It needs veraPDF available
directly or through Docker and fails if conformance cannot be verified.
`scripts/validate-pdfa-lib_test.sh` hermetically checks output parsing,
mounted-path translation, and the pinned Docker image contract; the full
release gate runs it before strict conformance validation.
`make check-pades` is the deterministic local PAdES invariant gate.
`make check-pades-external` uses installed external validators such as
OpenSSL, qpdf, pdfsig, veraPDF, and DSS when present.
`make check-pades-trusted` classifies a separately supplied release-certificate
sample. `make pades-harness-tests` is the automatic regression target: it runs
the real local generator and isolated external-validator, shared-lock, and
trusted-source behavior matrices. The trusted-source matrix uses controlled
tool output and therefore proves result classification, not real certificate
trust.

## Release Gates

```sh
make config-check
make installer-tool-pins
make license-check
make release-scope
make memory-scan
make check-release
PMFORGE_RELEASE_TAG=v1.1.0-rc.1 make tag-preflight
```

`make config-check` runs table-driven malformed-input regressions, parses every
tracked YAML and TOML file, rejects duplicate YAML keys, checks required
top-level structures, and fails if a new configuration has not been explicitly
classified. GitHub Actions is the CI authority; the retired `.gitlab-ci.yml`
must not be reintroduced accidentally.

`make installer-tool-pins` first mutates isolated release-workflow fixtures to
prove that mutable nFPM installs, unversioned NSIS installs, unused
`create-dmg`, missing version loading, invalid version records, and stale local
guidance are rejected. It then checks the live workflow against
`scripts/release-tool-versions.env`. This is a deterministic source guard;
installing the resulting packages on Linux, macOS, and Windows remains required
release-candidate evidence.

`make check-release` is the final gate. It currently covers version
consistency, configuration format policy, native installer tool pins,
REUSE/SPDX, frontend build budget, release-scope guards, frontend stability,
frontend runtime smoke, memory-safety scan, Go race tests, production build,
PDF/A-3 validation, and the PAdES harness regression target. Pre-merge GitHub
CI also runs `make pades-harness-tests` in a dedicated job with `qpdf` and
`pdfsig` installed.

`make tag-preflight` first tests and applies the publication-tag contract, then
runs the full release gate. The Release workflow supplies `GITHUB_REF_NAME` as
`PMFORGE_RELEASE_TAG` and blocks its Linux, macOS, and Windows package matrix on
this job. Accepted tags are `v<product-version>` and SemVer prereleases such as
`v<product-version>-rc.1`; mismatched versions, build metadata, empty
identifiers, and numeric prerelease identifiers with leading zeroes fail before
packaging.

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
