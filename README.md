<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge

PMForge is a local-first desktop project-controls application for
technical, engineering, IT, construction, and administrative work. It is
built with a Go backend, Wails v2, and a Svelte 5 frontend.

The application keeps project data on the local machine, supports
multi-user local accounts, and provides planning, scheduling, document,
chart, export, and reporting tools without requiring a hosted service.

## Current Capability

- **Project controls:** lifecycle status, budgets, stakeholders, timeline
  events, project settings, audit records, repair, backup, and local
  project files.
- **Scheduling:** CPM schedules with typed dependencies, lag,
  constraints, baselines, progress, Earned Value Management, resources,
  Gantt charts, MSPDI import/export, CSV, HTML, and report exports.
- **Charts:** 21 chart kinds across DAG, flow, matrix, and statistical
  engines, with frontend editing and vector PDF rendering.
- **Documents:** 25 project document kinds with schema-driven editing,
  bespoke PDF renderers, DOCX/ODT export, and combined reports with
  embedded chart visualisations.
- **Methodology packs:** Agile/Software-Dev views for Kanban, Backlog,
  Sprints, and DORA metrics; Process Excellence views for Six Sigma/DMAIC
  work.
- **Security and compliance:** local Argon2id accounts, one-time recovery
  codes, SQLCipher-encrypted per-user `.pmforge` project databases, PDF/A
  validation, and PAdES signing support.

## Quick Start

```sh
go mod tidy
(cd frontend && npm ci)   # use npm ci, not npm install
make fonts

wails dev
make build
make verify
```

`make verify` is the fast local and CI gate: Go tests, frontend stability,
and frontend build-budget checks. Run it before ordinary commits.

The full release gate is:

```sh
make check-release
```

It includes version consistency, REUSE/SPDX compliance, frontend runtime
checks, release-scope guards, memory-safety scanning, race tests,
production build, encrypted database validation, strict PDF/A-3
validation, and local PAdES validation.

Useful focused gates:

```sh
go test . ./internal/...
go test -race . ./internal/...
npm --prefix frontend run check
npm --prefix frontend run build
make frontend-smoke
make check-encrypted-db
make check-pdfa
make check-pades
make check-pades-external
make release-scope
make license-check
```

## Toolchain

- Go: `go.mod` pins Go 1.26.4.
- Wails: the project uses Wails v2.12.0. Install the matching CLI with:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

- Node dependencies live under `frontend/`.
- CGO is required for the SQLite/SQLCipher driver path.
- `make build` is the supported production build path. It runs the Wails
  build through `scripts/wails-build.sh`.

See [DEPENDENCIES.md](DEPENDENCIES.md) for dependency policy and external
validator tools.

## Runtime Data

On first launch, PMForge creates a local data area under
`~/Documents/PMForge/` by default:

- `system.db`: local account metadata, password hashes, and wrapped DEKs.
- `<username>/projects/`: per-user project folders and `.pmforge` files.
- `<username>/certs/`: user certificate files.
- `<username>/exports/`: generated exports.
- `logs/`: dated startup and runtime diagnostics.

Per-user folders are created with private POSIX permissions where the
platform supports them. Project databases are stored as one `.pmforge`
file per project, with WAL/SHM sidecars when SQLite needs them.

## Security Model

New per-user `.pmforge` project databases are SQLCipher-encrypted with the
user's DEK. Existing plaintext project databases can be migrated from
Project Settings after recovery codes are reissued. `system.db` remains
plaintext by design and stores password hashes plus wrapped DEKs, not
project records.

OS-level disk encryption is still recommended as whole-device protection
for raw-disk theft or administrator-level host access: FileVault on macOS,
BitLocker on Windows, and LUKS on Linux.

At account creation, PMForge issues one-time recovery codes. For encrypted
project databases, valid recovery codes also wrap the user's DEK. If the
password and all valid wrapped recovery codes are lost, encrypted project
databases are unrecoverable by design.

See [SECURITY.md](SECURITY.md) and
[ADR-001](docs/design/ADR-001-database-encryption-at-rest.md) for the
full security architecture.

## PDF, Signing, and Release Claims

PMForge generates PDF/A-3b representative samples during release
validation. `make check-pdfa` validates schedule-report, document, and
combined-report samples with veraPDF and is strict by default: missing
validator tooling, a missing ICC profile, or an empty sample set fails the
gate unless `PMFORGE_PDFA_STRICT=0` is set for local convenience.

PAdES signing is applied as the final PDF mutation. `make check-pades`
generates a deterministic signed sample and verifies the embedded CMS
against the declared `/ByteRange`. `make check-pades-external` adds
external checks when tools are installed: OpenSSL, `qpdf`, `pdfsig`,
veraPDF signature feature extraction, and DSS. Current DSS coverage
classifies the deterministic self-signed sample as `PAdES-BASELINE-B`;
trusted-chain validation and Acrobat coverage still require a real trusted
signing source.

Public release claims are guarded by `make release-scope`.

## User Workflows

See [docs/user-guide.md](docs/user-guide.md) for the current user-facing
workflow guide:

- New project Launchpad and seeded artifacts.
- Portfolio, Dashboard, Project Settings, and Application Settings.
- Charts, documents, combined reports, and exports.
- Schedule import/export.
- PDF signing, fonts, logs, recovery codes, and auto-save.

The in-app Help Guide contains the most detailed end-user reference —
including an **Installing & Running** section — and is available from the
Help tab or the native Help menu. For installer downloads and per-platform
install steps, see [docs/INSTALL.md](docs/INSTALL.md).

## Developer Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md): runtime shape, data layout,
  package map, and release architecture.
- [TESTING.md](TESTING.md): focused and full verification gates.
- [SECURITY.md](SECURITY.md): local account model, encryption, secrets,
  PDF signing, and release-safety rules.
- [DEPENDENCIES.md](DEPENDENCIES.md): Go, frontend, and external tool
  dependencies.
- [docs/INSTALL.md](docs/INSTALL.md): end-user install guide
  (`.deb`/`.rpm`/AppImage/`.exe`/`.dmg`) and run-from-source steps.
- [docs/release-preflight.md](docs/release-preflight.md): go/no-go
  checklist before pushing a `v*` release tag.
- [STYLE.md](STYLE.md): repository, Go, frontend, and documentation style.
- [AGENTS.md](AGENTS.md): current automated-agent operating guide.
- [AGENT.md](AGENT.md): PMForge Developer Handbook with long-form
  implementation history, release-gate status, and lessons learned.

## Repository Layout

```text
pmforge/
├── main.go              # Wails entry point and App surface
├── internal/            # Go backend packages
├── frontend/            # Svelte frontend
├── docs/                # public design and user documentation
├── scripts/             # release, validation, and packaging scripts
├── build/darwin/        # tracked Wails macOS plist scaffold
├── AGENTS.md            # current agent operating guide
└── AGENT.md             # PMForge Developer Handbook
```

Generated outputs, local handoff notes, validation scratch space, bundled
font downloads, project databases, certificates, and package artifacts are
ignored by `.gitignore`.

## License

Source code is licensed under GPL-3.0-or-later. Documentation, including
this README, is licensed under GFDL-1.3-or-later. Small configuration files
may use CC0-1.0. See [LICENSES.md](LICENSES.md) and the SPDX headers in
individual files.
