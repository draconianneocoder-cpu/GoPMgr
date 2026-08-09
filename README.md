<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr

> Renamed August 2026 from PMForge to GoPMgr to avoid confusion with the
> unrelated online PM simulation platform at getpmforge.com.

**Latest published release:**
[v1.1.0-alpha.1](https://github.com/draconianneocoder-cpu/GoPMgr/releases/tag/v1.1.0-alpha.1)
(prerelease, published July 27, 2026). This packaged alpha is intended for
early compatibility, installation, and workflow testing. Its GitHub release
provides native installers for Windows x86-64, macOS Apple Silicon, and Linux
x86-64 (`.deb` and `.rpm`). The packages are not code-signed or notarized, so
follow the platform-specific warning guidance in
[docs/INSTALL.md](docs/INSTALL.md) and verify the release asset digest before
installing.

This alpha is the first packaged 1.1.0 snapshot and includes the capabilities
listed below: CPM scheduling, DuckDB-backed portfolio analytics, 22 chart
types, 25 document kinds, Agile and Six Sigma methodology packs, SQLCipher
encryption, Argon2id authentication, PDF/A-3 validation, and PAdES digital
signing. The `main` branch may advance beyond the release snapshot.

See [ROADMAP.md](ROADMAP.md) for what comes next and [VISION.md](VISION.md)
for design principles.

---

GoPMgr is a local-first desktop project-controls application for
technical, engineering, IT, construction, and administrative work. It is
built with a Go backend, Wails v2, and a Svelte 5 frontend.

The application keeps project data on the local machine, supports
multi-user local accounts, and provides planning, scheduling, document,
chart, export, and reporting tools without requiring a hosted service.

## Current Capability

- **Project controls:** lifecycle status, exact-cent budgets, stakeholders,
  timeline events, project settings, audit records, repair, backup, and local
  project files, including integrity-checked `.pmba` backup and same-account
  restore as a non-overwriting project copy.
- **Scheduling:** CPM schedules with typed dependencies, lag,
  constraints, baselines, progress, Earned Value Management,
  named resource calendars, calendar-aware leveling, Gantt charts, MSPDI
  import/export, CSV, HTML, and report exports.
- **Risk and what-if:** Monte Carlo schedule-risk simulation
  (triangular/beta-PERT/normal sampling, P50/P80/P90 finish curves,
  tornado drivers, PDF/A risk report), a 5x5 risk/issue/opportunity matrix,
  and isolated what-if scenarios that copy a chart or baseline into a
  partition for comparison and promotion.
- **Charts:** 22 chart kinds across DAG, flow, matrix, and statistical
  engines, with frontend editing and vector PDF rendering.
- **Documents:** 25 project document kinds with schema-driven editing,
  bespoke PDF renderers, DOCX/ODT export, and combined reports with
  embedded chart visualisations.
- **Methodology packs:** Agile/Software-Dev views for Kanban, Backlog,
  Sprints, and DORA metrics; Process Excellence views for Six Sigma/DMAIC
  work.
- **Analytics:** DuckDB-backed in-memory portfolio budget, committed-cost, and
  weighted EVM rollups using integer minor-unit money totals with explicit
  schedule-data coverage, plus CSV/TSV, Parquet, and JSON data import in
  production/package builds.
- **Security and compliance:** local Argon2id accounts, one-time recovery
  codes, SQLCipher-encrypted per-user `.gopmgr` project databases, PDF/A
  validation, and PAdES signing support.

## Quick Start

```sh
go mod tidy
(cd frontend && npm ci)   # use npm ci, not npm install
make fonts                # optional catalog families; Source Sans 3 is tracked

wails dev
make build
make verify
```

`make verify` is the fast local and CI gate: tracked YAML/TOML configuration,
the generated first-party package map,
native installer and Wails toolchain pins, Go tests, frontend stability, and
frontend build-budget checks. Run it before ordinary commits.

The full release gate is:

```sh
make check-release
```

It includes version consistency, configuration parsing and format policy,
immutable native-installer tool selection, REUSE/SPDX compliance, frontend
runtime checks, release-scope guards, memory-safety scanning, race tests,
production build, encrypted database validation, strict PDF/A-3 validation,
and deterministic PAdES harness regressions.
The tag-triggered Release workflow reruns this gate in a blocking preflight
before any installer is built or uploaded. It also rejects a GA or prerelease
tag whose base version does not match GoPMgr's version of record.

Useful focused gates:

```sh
go test . ./internal/...
go test -race . ./internal/...
npm --prefix frontend run check
npm --prefix frontend run build
make frontend-smoke
make config-check
make installer-tool-pins
make windows-installer-scaffold
make check-encrypted-db
make check-pdfa
make check-pades
make check-pades-external
make pades-harness-tests
make release-scope
make license-check
```

## Toolchain

- Go: `go.mod` pins Go 1.26.5.
- Wails: the project uses Wails v2.13.0. Install the matching CLI with:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

- Node dependencies live under `frontend/`.
- CGO is required for the SQLite/SQLCipher driver path and production DuckDB
  analytics builds.
- `make build` is the supported production build path. It runs the Wails
  build through `scripts/wails-build.sh` with the `duckdb` tag enabled.

See [DEPENDENCIES.md](DEPENDENCIES.md) for dependency policy and external
validator tools.

## Runtime Data

On first launch, GoPMgr creates a local data area in the platform's
per-user data location:

- **macOS:** `~/Library/Application Support/GoPMgr/`
- **Linux / Windows:** `~/Documents/GoPMgr/`
- `$XDG_DATA_HOME/GoPMgr/` overrides the default on any platform.

It contains:

- `system.db`: local account metadata, password hashes, and wrapped DEKs.
- `<username>/projects/`: per-user project folders and `.gopmgr` files
  (or `.pmforge`, for projects created before the 2026-08-04 rename —
  both extensions keep working indefinitely).
- `<username>/certs/`: user certificate files.
- `<username>/exports/`: generated exports.
- `logs/`: dated startup and runtime diagnostics.

On macOS, this data lives in `Application Support` rather than `Documents`
so it is not iCloud-synced between machines and does not trigger the
Documents-folder privacy prompt. An existing `PMForge`-named install (from
before the 2026-08-04 GoPMgr rename, at either `Application Support/PMForge`
or the older pre-2026-06 `~/Documents/PMForge/`) is migrated automatically
on first launch after upgrading (the original is left in place and can be
deleted once you've confirmed the move).

For repeatable first-launch testing, `make reset-clean-test` moves the active
data directory to a timestamped, restorable backup instead of deleting it.
See [docs/INSTALL.md](docs/INSTALL.md) for the guarded reset and restore flow.

Per-user folders are created with private POSIX permissions where the
platform supports them. Project databases are stored as one `.gopmgr`
file per project, with WAL/SHM sidecars when SQLite needs them.

## Security Model

New per-user `.gopmgr` project databases are SQLCipher-encrypted with the
user's DEK. Existing plaintext project databases can be migrated from
Project Settings after recovery codes are reissued. `system.db` remains
plaintext by design and stores password hashes plus wrapped DEKs, not
project records.

OS-level disk encryption is still recommended as whole-device protection
for raw-disk theft or administrator-level host access: FileVault on macOS,
BitLocker on Windows, and LUKS on Linux.

At account creation, GoPMgr issues one-time recovery codes. For encrypted
project databases, valid recovery codes also wrap the user's DEK. If the
password and all valid wrapped recovery codes are lost, encrypted project
databases are unrecoverable by design.

See [SECURITY.md](SECURITY.md) and
[ADR-001](docs/design/ADR-001-database-encryption-at-rest.md) for the
full security architecture.

## PDF, Signing, and Release Claims

GoPMgr generates PDF/A-3b representative samples during release
validation. `make check-pdfa` validates schedule-report, document,
combined-report, and Monte Carlo risk-report samples with veraPDF and is
strict by default: missing
validator tooling, a missing ICC profile, or an empty sample set fails the
gate unless `GOPMGR_PDFA_STRICT=0` is set for local convenience.

PAdES signing is applied as the final PDF mutation. Users can also export
without a digital signature for print-and-wet-sign workflows, or create an
ASCII-armored detached GnuPG `.asc` sidecar without mutating the PDF bytes.
`make check-pades` generates a self-contained timestamped sample and verifies
the embedded CMS against the declared `/ByteRange`.
`make check-pades-external` adds OpenSSL, `qpdf`, `pdfsig`, veraPDF signature
feature extraction, and DSS checks. DSS classifies the self-signed fixture as
`PAdES-BASELINE-T`; trusted-chain validation and Acrobat coverage still
require a real trusted signing source. When `GOPMGR_TRUSTED_SIGNED_PDF`
points at a trusted-certificate sample, `make check-pades-trusted` validates
the unchanged source and classifies the result as `TRUST_VERIFIED`,
`STRUCTURE_VALID_TRUST_INDETERMINATE`, `VALIDATION_INCOMPLETE`, or
`VALIDATION_FAILED`; a missing or empty configured path is `INPUT_INVALID`.
Without a source it records `NOT_CONFIGURED`; set
`GOPMGR_PADES_TRUSTED_REQUIRED=1` to fail unless the local CLI certificate
store produces `TRUST_VERIFIED`. The report records the normalized PDF path,
PDF and validator hashes, checkout revision and dirty state, and UTC validation
time. Acrobat trust-panel evidence remains a separate manual release artifact
because Acrobat and command-line validators can use different trust policies.
The external gate regenerates its default sample on every run and records the
source mode, checkout revision and dirty state, UTC validation time, and
SHA-256 hashes for the scripts, PDF, CMS, and signed ByteRange bytes. To inspect
an existing artifact without replacing it, run
`bash scripts/validate-pades-external.sh /absolute/path/to/signed.pdf`; supplied
PDFs are validated in place and identified as such in the report.
`make pades-harness-tests` runs the real local generator plus the external,
parallel-locking, and trusted-source regression matrices. It runs in pre-merge
CI and the full release gate. Its trusted-source cases use controlled
validator output to test classification behavior; they do not replace the real
release-certificate and Acrobat evidence described above.

Project Settings can opt PAdES exports into RFC 3161 timestamping. GoPMgr
accepts a credential-free HTTPS TSA endpoint, optional policy OID, and optional
PEM trust root. It creates nonce-bound requests, validates TSA responses and
certificates, computes the required signature-value imprint, and embeds the
token as a DER unsigned CMS attribute without changing the original signature.
Timestamping is fail-closed: when enabled, an unavailable or invalid TSA
produces no signed export instead of silently falling back to Baseline B.
Audit events distinguish Baseline B, Baseline T with unevaluated TSA trust, and
Baseline T chained to the configured root. Legacy PDF-comment signature
markers are not emitted: every successful PAdES export contains a real `/Sig`
dictionary and `/ByteRange`, and signing failures return no PDF bytes.
Deterministic application tests exercise both document and combined-report
PAdES-T exports without depending on a public timestamp service.

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
Help tab or the native Help menu. For per-platform install steps and package
names to use when release assets are available, see
[docs/INSTALL.md](docs/INSTALL.md).

## Developer Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md): runtime shape, data layout,
  package map, and release architecture.
- [TESTING.md](TESTING.md): focused and full verification gates.
- [SECURITY.md](SECURITY.md): local account model, encryption, secrets,
  PDF signing, and release-safety rules.
- [DEPENDENCIES.md](DEPENDENCIES.md): Go, frontend, and external tool
  dependencies.
- [docs/INSTALL.md](docs/INSTALL.md): end-user install guide
  (`.deb`/`.rpm`/`.exe`/`.dmg`) and run-from-source steps.
- [docs/release-preflight.md](docs/release-preflight.md): go/no-go
  checklist before pushing a `v*` release tag.
- [docs/beta-release-backlog.md](docs/beta-release-backlog.md): running
  stabilization, native validation, and release-trust work for the next Beta.
- [STYLE.md](STYLE.md): repository, Go, frontend, and documentation style.
- [AGENTS.md](AGENTS.md): current automated-agent operating guide.
- [DEVELOPER_HANDBOOK.md](DEVELOPER_HANDBOOK.md): GoPMgr Developer Handbook with long-form
  implementation history, release-gate status, and lessons learned.

## Repository Layout

```text
gopmgr/
├── main.go              # Wails entry point and App surface
├── internal/            # Go backend packages
├── frontend/            # Svelte frontend
├── docs/                # public design and user documentation
├── scripts/             # release, validation, and packaging scripts
├── build/darwin/        # tracked Wails macOS plist scaffold
├── AGENTS.md            # current agent operating guide
└── DEVELOPER_HANDBOOK.md             # GoPMgr Developer Handbook
```

Generated outputs, local handoff notes, validation scratch space, optional
font downloads, project databases, certificates, and package artifacts are
ignored by `.gitignore`. The Source Sans 3 PDF/A baseline is tracked and
verified by `make required-font-assets`.

## License

GoPMgr is free software: its source code is licensed under
**GPL-3.0-or-later**. User-facing documentation, including this README, is
licensed under **GFDL-1.3-or-later**; small configuration files and license
notes use **CC0-1.0**; bundled fonts carry their own OFL-1.1, Apache-2.0, or
Bitstream-Vera terms.

The project follows the [REUSE](https://reuse.software/) specification, so the
authoritative license of any file is the `SPDX-License-Identifier` in that
file (or in [REUSE.toml](REUSE.toml)). Run `make license-check` to verify.

See [LICENSE.md](LICENSE.md) for the full license statement and a summary of
what GPL-3.0-or-later means for you, and [LICENSES.md](LICENSES.md) for the
per-identifier rationale.
