<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Testing

GoPMgr uses focused package tests during development and broader gates
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

## Flakiness diagnostics

Use uncached, shuffled runs when investigating order-sensitive behavior and
record the seed printed by `go test` so a failure can be replayed exactly:

```sh
GOCACHE=/tmp/gopmgr-go-cache go test -count=1 -shuffle=on ./internal/...
GOCACHE=/tmp/gopmgr-go-cache go test -shuffle=<seed> ./path/to/package
```

Repeat only the suspected package or test with a bounded `-count`; do not add
suite retries or loosen assertions to hide an intermittent failure. For
date-sensitive logging tests, run the focused package under both `TZ=UTC` and
the supported local timezone. The PAdES harness serializes shared evidence
with a directory lock and now fails closed after a bounded wait (30 seconds by
default). Set `GOPMGR_PADES_LOCK_TIMEOUT_SECONDS` or
`GOPMGR_PADES_TRUSTED_LOCK_TIMEOUT_SECONDS` only for an explicit diagnostic;
the lock is never reclaimed automatically, so confirm no owner remains before
removing an abandoned lock. `scripts/validate-pades-parallel_test.sh` uses
isolated harness scratch/log directories and relies on the same lock for the
shared sample. Environment failures such as a sandbox denying local socket
creation or an installed validator aborting are reported as blocked evidence,
not converted into passing results.

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
make windows-installer-scaffold
make license-check
make release-scope
make memory-scan
make check-release
GOPMGR_RELEASE_TAG=v1.1.0 make tag-preflight
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
prerelease evidence.

`make windows-installer-scaffold` verifies the GoPMgr-owned NSIS entrypoint,
Windows version resources, DPI manifest, generated-file ignore boundary,
workflow ordering, DuckDB build tag, and post-build linkage check. Isolated
fixtures cover missing branding, destructive uninstall behavior, analytics
stub drift, and bypassed checks. When `makensis` is installed, the target also
compiles a harmless fixture installer against the pinned Wails macro template.
This proves NSIS syntax, not Windows binary linkage or installed-app behavior.
If `makensis` crashes rather than rejecting the template (some Homebrew
NSIS bottles abort with `std::bad_alloc` on certain macOS/arm64 hosts,
independent of any GoPMgr `.nsi` content), the failure output names
`GOPMGR_SKIP_NSIS_COMPILE`, an explicit env var that skips only the
compile step; unset by default, so a normal run still fails loudly.
Setting it is not a pass — it means NSIS syntax went unverified for that
run.

`make check-release` is the final gate. It runs `scripts/check-release.sh`,
which is the source of truth for exactly what it covers — read that script
rather than relying on a prose summary here, since a step list drifts as the
script grows. As of this writing it covers (non-exhaustively): the Wails
toolchain/CLI version check, version consistency, configuration format
policy, native installer tool pins, required font assets, the Windows
installer scaffold, the Linux runtime target, REUSE/SPDX, frontend build
budget, release-scope guards, frontend stability, frontend runtime smoke,
memory-safety scan, Go race tests, production build, DuckDB linkage,
encrypted-database validation, PDF/A-3 validation, and the PAdES harness
regression target. Pre-merge GitHub CI also runs `make pades-harness-tests`
in a dedicated job with `qpdf` and `pdfsig` installed.

`make tag-preflight` first tests and applies the publication-tag contract, then
runs the full release gate. The Release workflow supplies `GITHUB_REF_NAME` as
`GOPMGR_RELEASE_TAG` and blocks its Linux, macOS, and Windows package matrix on
this job. Accepted tags are `v<product-version>` and SemVer prereleases such as
`v<product-version>-rc.1`; mismatched versions, build metadata, empty
identifiers, and numeric prerelease identifiers with leading zeroes fail before
packaging. The release-scope gate also exercises
`release-publication-flag.sh`: suffix-bearing tags must supply GitHub CLI's
`--prerelease` flag, while a clean version tag must not. This distinction is
explicit because GitHub does not infer release classification from SemVer.

Run `make license-check` after adding files or generated assets. Run
`make release-scope` after documentation changes that touch release
claims, especially PDF/A, PAdES, encryption, or public-repo hygiene.
The release-scope gate also runs isolated release-reference regressions and
rejects concrete candidate tags or candidate release-note files while no such
release is listed in `docs/published-release-tags.txt`. Keep future prerelease
examples symbolic until the tag and GitHub release exist; update that snapshot
only after verifying the live release.

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
