<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Architecture

PMForge is a local-first project controls desktop application for
technical, engineering, IT, construction, and administrative teams. The
application is intentionally offline-capable: project data, generated
documents, certificates, exports, and user account metadata live on the
local machine.

## Runtime Shape

- Backend: Go, exposed to the desktop UI through Wails v2.
- Frontend: Svelte 5 and TypeScript, bundled by Vite.
- Persistence: SQLite-compatible databases, with SQLCipher support for
  encrypted per-project `.pmforge` files.
- Documents: Go PDF/DOCX/ODT/XLSX/iCal renderers. PDF/A-3 and PAdES
  support are implemented in Go.
- Build: `make build` runs `wails build`, which builds the frontend into
  `frontend/dist`, embeds it via the root `main.go` `go:embed`, injects the
  `desktop,production` tags, links the platform frameworks, and compiles the
  Wails backend with CGO.

## Data Layout

On first run PMForge creates a local root under `~/Documents/PMForge`:

```text
~/Documents/PMForge/
  system.db
  <username>/
    projects/
    certs/
    exports/
```

`system.db` stores local users, Argon2id password hashes, recovery-code
metadata, and wrapped data-encryption keys. Per-project `.pmforge`
databases store the actual project records, charts, documents,
stakeholders, agile data, timeline data, and audit material.

The main project database model is deliberately compact. Chart and
document records use discriminator columns and JSON payloads so new
kinds can be added through registries without multiplying table
families.

## Important Packages

- `main.go` (repo root): Wails app object, CLI dispatch, account/session
  flow, project lifecycle, export entry points, and frontend embed. Lives at
  the root because `wails build` requires the main package there.
- `internal/users`: Local account store, Argon2id authentication,
  recovery codes, and wrapped DEK handling.
- `internal/db`: Project database schema, migrations, CRUD, backup,
  repair, audit, and SQLCipher migration helpers.
- `internal/sqlitedriver`: Central SQLite/SQLCipher driver registration.
- `internal/crypto`: AES-GCM utilities, key wrapping, X.509/RSA signing,
  and detached CMS helpers for PAdES.
- `internal/pdfmeta`: PDF incremental updates for XMP, output intents,
  and PAdES signature embedding.
- `internal/documents`: Document registry, default content, combined
  reports, and bespoke PDF renderers.
- `internal/charts`: Chart taxonomy, layout engines, and vector PDF
  renderers.
- `internal/kernel`: CPM scheduling, dependencies, constraints,
  baselines, EVM, resource calculations, and Monte Carlo schedule-risk
  simulation.
- `internal/calendar`: named resource calendars (weekly capacity and day
  overrides) used for calendar-aware leveling and over-allocation checks.
- `internal/money`: exact monetary arithmetic in integer minor units
  (`math/big.Rat` for rate x quantity, rounded once at the boundary).
- `internal/analytics`: DuckDB-backed in-memory portfolio rollups and
  CSV/TSV/Parquet/JSON data import (behind the `duckdb` build tag).
- `internal/export`: PDF, PDF/A, DOCX, ODT, XLSX, iCal, MSPDI, and Monte
  Carlo risk-report export paths.
- `internal/exportsafe`: neutralizes user-controlled values written to
  CSV/TSV exports against spreadsheet formula injection.
- `internal/templates`: Launchpad seeding rules embedded from JDM data.
- `internal/agile` and `internal/sigma`: Agile/software development and
  process excellence feature packs.
- `frontend/src`: Svelte application shell, global CSS, session store,
  Wails bridge types, and route/UI logic.

## Security Architecture

Authentication uses Argon2id PHC strings. The current encryption-at-rest
design keeps `system.db` plaintext enough to support login bootstrap,
but stores only password hashes, recovery metadata, and wrapped DEKs.
Project databases can be SQLCipher-encrypted with a per-user DEK. The
DEK is wrapped by the login password and by each valid recovery code so
password recovery can preserve encrypted projects.

Project encryption migration uses `sqlcipher_export` into a temporary
encrypted sibling, verifies the destination, retains the plaintext
source as `<project>.pre-encryption.bak`, and publishes the encrypted
database only after integrity checks pass.

## Frontend Architecture

The frontend is a desktop application UI, not a marketing site. It
should remain dense, utilitarian, and optimized for repeated project
management work. Wails injects `window.go.main.App`; keep
`frontend/src/wails-window.d.ts` aligned whenever backend methods are
added or changed.

Svelte runes belong in `.svelte`, `.svelte.ts`, or `.svelte.js` files.
The runtime smoke gate exists because some Svelte load-time failures are
not caught by TypeScript or Vite build alone.

## Release Architecture

`scripts/check-release.sh` is the release gate. It verifies version
consistency, REUSE/SPDX compliance, frontend budget and stability,
frontend runtime smoke, memory-safety checks, Go race tests, production
build, strict PDF/A-3 validation, and local PAdES validation.

Generated embed output under `frontend/dist` (repo root) is build output
embedded by the root `main.go`. `wails build` regenerates it as needed.
