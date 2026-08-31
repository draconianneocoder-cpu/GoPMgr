<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Developer Handbook

**Architecture, development, testing, maintenance, security, and release
manual for GoPMgr developers and maintainers**

## Preface

### Purpose

GoPMgr is a local-first desktop application for project controls. It
combines a Go backend, a Wails v2 desktop shell, and a Svelte
5/TypeScript frontend. The application supports planning, scheduling,
earned-value management, resource analysis, charts, documents, local
encrypted project storage, open-format exports, PDF/A-3 output, and
optional PAdES signing.

This handbook explains how to work on that system safely.

It is written for:

- developers making backend or frontend changes;
- maintainers reviewing architecture and compatibility;
- contributors adding project-control features;
- developers working on database, encryption, export, or signing code;
- release maintainers;
- automated coding agents operating in the repository.

The handbook is deliberately more instructional than `STYLE.md`,
`TESTING.md`, or `AGENTS.md`. Those documents define focused rules. This
document explains how those rules fit together during real development
work.

### Source-of-truth hierarchy

When documentation conflicts, use the most direct executable evidence
available.

A practical hierarchy is:

1. current source code and schema;
2. tests that exercise the relevant behavior;
3. release/build scripts and CI gates;
4. accepted ADRs and architecture documents;
5. focused repository policies such as `SECURITY.md`,
    `ERROR_HANDLING.md`, `STYLE.md`, `TESTING.md`, and
    `DEPENDENCIES.md`;
6. this handbook;
7. historical notes, issues, and Git history.

Do not preserve a handbook statement merely because it is written here.
Correct the handbook when implementation or an accepted architectural
decision changes.

### How to use this handbook

For a new contributor, read Parts I and II before making architectural
changes. For routine work, use the task-oriented chapters in Parts III
and IV. For release-sensitive work, read Parts V and VI before
implementation rather than waiting until release validation fails.

The appendices provide command, invariant, and checklist references for
day-to-day work.

### Chapter list

`AGENTS.md` asks automated agents not to read this handbook end to end
and to search it instead. This list exists for that: match a topic to a
chapter number, then jump or `rg` directly.

- **Part I — Getting Started**: 1 Runtime shape and technologies · 2
  Environment setup · 3 Repository tour
- **Part II — Architecture and Boundaries**: 4 Application boundaries · 5
  State and lifecycle · 6 Data architecture · 7 Compatibility contracts
- **Part III — Implementing Changes**: 8 Go backend development · 9 Error
  handling · 10 Svelte frontend development · 11 Persistence/migration
  development · 12 Adding or changing a feature
- **Part IV — Engineering Assurance**: 13 Testing strategy · 14
  Repository verification · 15 Debugging and troubleshooting · 16
  Concurrency and resource ownership · 17 Security engineering
- **Part V — Documents, Export, and Signing**: 18 Export development · 19
  PDF/A-3 · 20 PAdES signing
- **Part VI — Maintenance and Release Engineering**: 21 Dependency
  maintenance · 22 Documentation and ADRs · 23 Release engineering · 24
  Vulnerability management · 25 Agent-assisted development
- **Part VII — Contributor Workflow**: 26 Planning a change · 27
  Implementing a change · 28 Reviewing a change · 29 Validation before
  handoff
- **Appendices**: A Command reference · B System invariants · C
  Development checklists · D Troubleshooting matrix · E Glossary · F
  Related repository documentation

------------------------------------------------------------------------

# Part I — Getting Started

## 1. GoPMgr for Developers

### 1.1 Runtime shape

GoPMgr has four primary runtime concerns:

1. **Desktop/UI boundary** — Wails exposes Go application methods to
    the Svelte frontend.
2. **Application and domain logic** — root `app_*.go` files
    coordinate UI-facing operations while reusable domain logic lives
    under `internal/`.
3. **Local persistence** — `system.db` stores account/bootstrap
    information; per-project `.gopmgr` databases store project data and
    may be encrypted with SQLCipher.
4. **Document and export pipelines** — Go packages generate project
    reports and open-format exports, including PDF/A-3 and optional
    PAdES-signed PDF output.

The application is intentionally offline-capable. Project data,
generated documents, certificates, exports, and local account metadata
live on the user's machine rather than requiring a hosted service.

### 1.2 Major technologies

The current repository uses:

- Go 1.27.0;
- Wails v2.15.0;
- Svelte 5;
- TypeScript 6;
- Vite 8;
- Node.js 26 in CI/release workflows;
- SQLite-compatible storage with SQLCipher support;
- CGO;
- DuckDB for tagged analytics builds.

Version declarations in `go.mod`, `frontend/package.json`, release
workflows, and version-check scripts are authoritative.

### 1.3 Architectural principle

GoPMgr favors conservative, explicit, locally testable code.

A change should normally make the execution path easier — not
harder — to trace from:

``` text
Svelte interaction
    ↓
Wails-bound App method
    ↓
application orchestration
    ↓
focused internal package
    ↓
database / filesystem / renderer / exporter
    ↓
validated result
    ↓
frontend state or user-visible result
```

Do not add an abstraction merely because it is common in another
architecture. Add one when it solves a demonstrated GoPMgr problem and
leaves ownership clearer.

------------------------------------------------------------------------

## 2. Development Environment

### 2.1 Required baseline

A normal development checkout requires:

- Go 1.27.0;
- **Wails v2.15.0** (the exact CLI version, not "latest");
- Node.js and npm;
- a C compiler and CGO-capable Go toolchain;
- platform dependencies required by Wails.

Install the matching Wails CLI:

``` sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

Do not use an arbitrary newer Wails CLI. The repository intentionally
verifies that runtime, CLI, and documentation pins remain synchronized.

### 2.2 Initial checkout setup

From the repository root:

``` sh
make tidy
wails dev
```

`make tidy` runs `go mod tidy` and `npm --prefix frontend install`
together; it is the repository-sanctioned onboarding step rather than
running those commands separately.

For an existing development checkout, do not run dependency-mutating
commands automatically. Inspect `git status --short` first and
understand whether dependency metadata is expected to change.

### 2.3 Development mode

Use:

``` sh
make dev
```

or:

``` sh
wails dev
```

for Wails development mode with the Go backend and Svelte frontend.

### 2.4 Production build

Use:

``` sh
make build
```

The production build runs the repository's Wails build wrapper. Wails
builds the frontend, generates required bindings, embeds `frontend/dist`
through the root `main.go`, applies the desktop/production build
context, links required platform frameworks, and compiles the
CGO-enabled backend.

Production/package builds enable the DuckDB analytics build tag by
default.

### 2.5 A first verification pass

After setup, a useful baseline is:

``` sh
make verify
```

`make verify` is the regular local repository gate. It is intentionally
broader than a single test command. It checks repository configuration
and packaging contracts, Wails/toolchain consistency, Go tests, code-map
currency, frontend stability, frontend build budgets, coverage-ledger
currency, and additional regression guards.

A successful `make verify` is not equivalent to the full release gate.

------------------------------------------------------------------------

## 3. Repository Tour

### 3.1 Top-level structure

``` text
main.go                 Wails entry point, application object, frontend embed
app_*.go                UI-facing adapters and application orchestration
internal/               Domain and infrastructure packages
frontend/               Svelte 5 + TypeScript desktop UI
docs/                   User, design, release, and supporting documentation
scripts/                Validation, packaging, release, and repository checks
tools/                  Repository development tools
code-map/               Generated/checked architectural and API maps
build/                  Platform packaging inputs and build configuration
```

### 3.2 Root application package

The main Go package lives at the repository root because the Wails build
expects the main package there.

Root `app_*.go` files form the Wails-facing application layer. They may:

- validate bridge input;
- obtain the current authenticated user or project state;
- coordinate one or more focused internal packages;
- translate internal results into bridge-safe values;
- return user-facing or structured errors.

They should not become a second domain layer.

When an operation is reusable outside one Wails method, complex enough
to require its own tests, or naturally belongs to a domain, prefer a
focused package under `internal/`.

### 3.3 Important internal packages

`internal/` currently holds 30 top-level packages (directories; `internal/sigma`
counts once here despite holding five subpackages — see its own note below).
The table groups them by concern; regenerate this list from `ls internal/` and
`code-map/package-dependencies.json` if it drifts.

**Accounts, security, and persistence**

| Package | Responsibility |
| — | — |
| `internal/users` | Local multi-user accounts, per-user data directories, recovery-code metadata, wrapped DEKs |
| `internal/auth` | Argon2id password hashing and constant-time verification |
| `internal/db` | Project schema, migrations, CRUD, backup, repair, audit, SQLCipher migration helpers |
| `internal/sqlitedriver` | Central SQLite/SQLCipher driver registration |
| `internal/crypto` | AES-256-GCM utilities, key wrapping, X.509/RSA signing, detached CMS helpers |
| `internal/rfc3161` | Fail-closed RFC 3161 timestamp-authority client for the signing pipeline |
| `internal/signing` | GnuPG detached-signature and PAdES PDF-signing workflows |
| `internal/debug` | Structured diagnostic reports for repair/self-heal paths (`debug.Wrap`) |

**Documents, charts, and export**

| Package | Responsibility |
| — | — |
| `internal/pdfmeta` | Byte-level PDF metadata: XMP packet construction, incremental updates, PAdES embedding |
| `internal/documents` | Document taxonomy (25 kinds), schemas, templates, validation, bespoke PDF/DOCX/ODT renderers |
| `internal/charts` | Taxonomy and dispatch for 22 chart/diagram types, layout engines, vector PDF renderers |
| `internal/export` | Converts internal data models to PDF/A, DOCX, ODT, XLSX, CSV, iCal, MSPDI, risk-report output |
| `internal/exportfs` | Publishes user-requested exports without silently replacing an existing artifact; shared file-safety policy for primary PDFs and generated sidecars |
| `internal/reporting` | Combined-report preflight, assembly, provenance output |
| `internal/exportsafe` | Spreadsheet-formula-injection neutralization (CWE-1236) for CSV/TSV |
| `internal/fonts` | Embedded TrueType font management for generated PDFs |
| `internal/admin` | Administrative Pack: document control, secure archiving, signature-event logging |

**Scheduling, cost, and analytics**

| Package | Responsibility |
| — | — |
| `internal/kernel` | CPM, dependencies, constraints, baselines, EVM, resources, Monte Carlo schedule risk |
| `internal/calendar` | Thin wrapper over `rickar/cal/v2` for holiday lookups and working-day math |
| `internal/money` | Exact monetary arithmetic (integer minor units plus `math/big.Rat`) |
| `internal/budget` | Cost-rollup engine spanning vendor contract values and agile work-item cost |
| `internal/catalog` | A signed-in user's reusable suppliers and items, kept separate from `system.db` and from individual (portable, self-contained) project files |
| `internal/timeline` | Assembles dated project entities into one chronological stream (Timeline view, iCal export) |
| `internal/analytics` | DuckDB-backed portfolio analytics and data import, built under the `duckdb` tag |

**Feature packs and platform**

| Package | Responsibility |
| — | — |
| `internal/templates` | Project Launchpad seeding of starter artifacts via JDM (JSON Decision Model) rules |
| `internal/agile` | Software-Dev Pack: Kanban boards, sprints, work items, DORA metrics |
| `internal/sigma` | Namespace directory (not itself a package) for the Six Sigma/DMAIC feature pack — see `sigma/domain`, `sigma/service`, `sigma/stats`, `sigma/tollgate`, `sigma/charts` |
| `internal/applog` | Process-level diagnostic logging and fatal-startup handling for the Wails GUI |
| `internal/cli` | GNU-style command-line flag parsing for the headless/CLI entry points |
| `internal/update` | Fetches the signed release manifest over HTTPS and checks for a newer version (Ed25519-pinned) |

`internal/sigma` has no `package sigma` declaration at its root — treat
it as a directory of five independent subpackages, not a single import.

Before adding a package, inspect the nearest existing package. Prefer
extending an established domain boundary over introducing a parallel
abstraction.

### 3.4 Frontend structure

GoPMgr's frontend is a single-page application: there is no client-side
router or `routes/`/`pages/` directory. Screens are composed from
components and shown/hidden by application state.

``` text
frontend/src/
  App.svelte, app.css, main.ts   Application shell and global styles
  wails-window.d.ts              Hand-written ambient bridge declarations
  lib/
    components/                  Svelte UI components, grouped by domain
      admin/ agile/ auth/ charts/ documents/ help/ project/ sigma/
    *.svelte.ts                  Svelte 5 rune-based state modules
                                  (session.svelte.ts, autosave.svelte.ts,
                                  toast.svelte.ts)
    *.ts                         Pure TypeScript helpers (native-close.ts,
                                  rebase-editable-changes.ts, theme.ts,
                                  terminology.ts, methodologies.ts)
    *.test.ts                    Colocated Vitest unit tests
```

The frontend is a desktop work application. It should remain dense,
clear, and optimized for repeated project-management work rather than
adopting marketing-site composition patterns.

------------------------------------------------------------------------

# Part II — Architecture and System Boundaries

## 4. Application Boundaries

### 4.1 Wails methods are adapters

A Wails-bound method should normally do four things:

1. validate bridge input;
2. obtain required application/session/project state;
3. call focused application/domain behavior;
4. return a bridge-safe result or error.

A warning sign is a Wails method that contains substantial scheduling
logic, SQL, cryptographic logic, PDF internals, or reusable report
assembly.

### 4.2 Keep domain logic below the bridge

The Svelte frontend should not become authoritative for:

- project invariants;
- encryption decisions;
- project path authorization;
- database consistency;
- financial calculations;
- schedule calculations;
- audit integrity;
- signature validity.

The frontend may perform usability validation, but the backend must
enforce security and persistence invariants.

### 4.3 Bridge declarations: hand-written vs. generated

Wails exposes backend methods through `window.go.main.App`. Two
independent artifacts describe that surface, and they are not the same
thing:

- `frontend/src/wails-window.d.ts` — a **hand-written** ambient
    TypeScript declaration. It lets Svelte components type-check before
    `wails dev`/`wails build` has run, and it is what most frontend code
    actually compiles against.
- `frontend/wailsjs/` (`go/main/App.d.ts`, `go/models.ts`,
    `runtime/runtime.d.ts`) — **generated** by the Wails CLI from the
    current Go method set on every `wails dev`/`wails build`.

When an exported backend method's name, parameters, or return shape
changes, update `wails-window.d.ts` by hand to match; nothing regenerates
it automatically, and nothing fails loudly if it drifts from the real
bridge. `make wails-version` (`scripts/check-wails-version.sh`) verifies
that the installed Wails CLI and documented version pins agree — it
does not check `wails-window.d.ts` against Go method signatures.

Do not treat a stale hand-written declaration as evidence of the backend
contract. Verify the Go method and the generated bindings in
`frontend/wailsjs/` directly.

------------------------------------------------------------------------

## 5. Application State and Lifecycle

### 5.1 State ownership

Application state protects the active user and active project database.

Methods that read or mutate project state must use the established
application helpers and locking patterns.

### 5.2 Locking rule

Hold locks only for the scope that actually requires protected access.

Do not:

- hold application locks while performing avoidable long-running work;
- expose partially initialized state;
- swap project state before the replacement is known to be valid;
- bypass established state helpers with ad hoc global state.

### 5.3 Project switching

Project switching is a lifecycle boundary.

Any asynchronous work started for Project A must be unable to overwrite
state belonging to Project B after the active project changes.

This matters especially for:

- editor saves;
- frontend timers;
- subscriptions;
- background calculations;
- delayed Wails responses.

Prefer explicit ownership and cancellation over "latest result probably
wins."

### 5.4 Background work

Do not introduce goroutines, timers, listeners, or frontend
subscriptions merely to make a call appear asynchronous.

When background work is necessary, make these properties explicit:

- owner;
- start condition;
- cancellation path;
- error path;
- cleanup;
- shutdown behavior;
- behavior when the active project changes.

------------------------------------------------------------------------

## 6. Data Architecture

### 6.1 Application data layout

GoPMgr resolves a private platform-specific application data root. A
configured XDG data location overrides the default where supported.

Conceptually:

``` text
<data-root>/
  system.db
  <username>/
    projects/
    certs/
    exports/
```

Supported legacy data roots are copied into the current location during
first launch after an applicable upgrade. The migration is intended to
be non-destructive.

### 6.2 `system.db`

`system.db` is the bootstrap/account database.

It stores local account information including:

- Argon2id password hashes;
- recovery-code metadata;
- wrapped data-encryption keys.

It deliberately remains openable before login. Do not put normal project
content in it.

### 6.3 `.gopmgr` project databases

Per-project `.gopmgr` files hold project data, including project-control
records, charts, documents, stakeholders, agile data, timeline data, and
audit material.

`.pmforge` is the legacy extension for the same file format, used by
projects created before the 2026-08-04 rename. GoPMgr still opens
`.pmforge` files directly — code that resolves or validates a project
path must accept both extensions rather than assuming `.gopmgr`.

These files are compatibility contracts.

A change to their schema, encryption behavior, migration path, backup
behavior, or interpretation must be treated as persistence work — not as
an ordinary internal refactor.

### 6.4 SQLite/SQLCipher driver ownership

`internal/sqlitedriver` owns SQLite/SQLCipher driver registration.

Do not register a second SQLite implementation casually. A driver change
can affect:

- encryption;
- database opening;
- migration;
- CGO;
- binary linkage;
- packaging;
- tests;
- backup/repair behavior.

### 6.5 Compact extensibility

Chart and document records use discriminator columns and JSON payloads
so new kinds can be registered without creating a new table family for
every type.

When adding a new kind, use the established registry and
payload-validation patterns.

------------------------------------------------------------------------

## 7. Compatibility Contracts

Treat the following as compatibility-sensitive unless source and tests
prove otherwise:

- existing `.gopmgr` files, and legacy `.pmforge` files still opened
    directly;
- `system.db` account/bootstrap metadata;
- migrations;
- backup formats;
- recovery metadata;
- exported wire/file formats;
- Wails bridge names used by the frontend;
- user-facing error strings that frontend code matches literally;
- signed-document byte structure;
- public release claims.

### 7.1 Migration rule

A migration must be additive where possible or provide an explicit,
tested recovery path.

Do not prove migration correctness only by:

1. writing data with the new implementation;
2. reading it with the same new implementation.

Use pre-change or representative fixtures where the compatibility
question requires them.

### 7.2 Determinism

Prefer deterministic ordering in:

- exported records;
- generated manifests;
- tests;
- serialized evidence;
- reproducible validation output.

Determinism makes regressions easier to identify and reduces false
diffs.

------------------------------------------------------------------------

# Part III — Implementing Changes

## 8. Go Backend Development

### 8.1 Package design

Prefer narrow, domain-oriented packages.

Avoid generic dumping grounds such as:

``` text
internal/common
internal/util
internal/helpers
```

unless an existing narrow pattern clearly justifies the package.

A package should make ownership clearer. If moving code into a helper
forces readers to jump files without reducing conceptual complexity,
keeping the logic local may be better.

### 8.2 Control flow

Use guard clauses for errors and edge cases. Keep the successful path
shallow.

Prefer:

``` go
if err != nil {
    return fmt.Errorf("load project: %w", err)
}

// normal path
```

over deeply nested conditionals.

### 8.3 Error position and wrapping

Return `error` as the final return value.

Wrap errors with operation context:

``` go
return fmt.Errorf("open project: %w", err)
```

Use `%w` when callers need to preserve error identity.

### 8.4 Imports and formatting

Group Go imports as:

1. standard library;
2. third-party;
3. first-party.

Run `gofmt` or the repository's accepted import formatter after edits.

### 8.5 JSON contracts

Use `snake_case` JSON tags on Wails-bound structs because existing
TypeScript code expects those wire names.

Treat wire-name changes as compatibility work.

### 8.6 Database writes

When multiple records or files must change as one operation, use
established transactional patterns.

Do not create partial persistence states merely to simplify code.

### 8.7 Security-sensitive randomness

Use `crypto/rand`.

For recoverable operations such as ID, salt, or recovery-code
generation, follow established error-returning randomness patterns so
entropy failures can propagate rather than terminate the application
unexpectedly.

------------------------------------------------------------------------

## 9. Error Handling

### 9.1 Sentinel errors

Packages with recoverable, caller-specific failure modes use sentinel
errors.

Example pattern:

``` go
var ErrNoProject = errors.New("db: no project initialised in this file")
```

Callers inspect them with:

``` go
errors.Is(err, db.ErrNoProject)
```

or:

``` go
errors.As(err, &target)
```

Never match behavior using `err.Error()` text.

### 9.2 Preserve sentinel identity

Wrapped sentinels must continue to satisfy `errors.Is`:

``` go
return fmt.Errorf("open project: %w", db.ErrNoProject)
```

Tests should verify the wrapped case when error identity is part of the
contract.

### 9.3 Structured diagnostic reports

GoPMgr uses `internal/debug.Wrap` for selected recoverable paths that
need structured diagnostic information, particularly
repair/snapshot/self-heal scenarios.

Use ordinary `fmt.Errorf` wrapping for normal error plumbing.

Do not expand structured diagnostic wrapping indiscriminately;
persistent diagnostic logging has privacy and maintenance consequences.

### 9.4 Fail-hard by default

Security, state, and integrity boundaries generally fail hard.

Examples include:

- encryption migration;
- project path confinement;
- account/session state.

Fail-soft behavior should be intentional and documented only when a
degraded result is more useful than complete failure.

### 9.5 User-facing error contracts

Some capitalized backend error strings intentionally cross the bridge
and may be matched literally by frontend code.

Before rewording an established user-facing error:

``` sh
rg 'exact error text' frontend
```

or otherwise search the frontend for dependencies.

### 9.6 Prohibited error patterns

Do not:

- use `panic`/`recover` for expected control flow;
- silently discard meaningful errors;
- compare errors by strings;
- compare wrapped sentinel errors with plain `==`;
- include passwords, DEKs, recovery codes, SQLCipher keys, or private
    key material in errors.

------------------------------------------------------------------------

## 10. Svelte Frontend Development

### 10.1 Frontend responsibility

The frontend owns interaction and presentation.

It may:

- collect and validate user input for usability;
- maintain editor/view state;
- invoke Wails methods;
- render domain results;
- show errors and recovery hints;
- manage component-local asynchronous work.

It must not replace backend enforcement of persistence or security
invariants.

### 10.2 Svelte 5 rules

Use Svelte 5 and TypeScript patterns already present in `frontend/src`.

Svelte runes belong only in:

- `.svelte`;
- `.svelte.ts`;
- `.svelte.js`.

### 10.3 Wails calls

User-triggered Wails operations should be handled explicitly.

Expected pattern:

``` text
user action
  → frontend validation
  → try Wails call
  → update state on success
  → show user-visible error on failure
```

User-visible failures normally use the shared toast/error presentation
pattern.

Optional feature detection may fail silently only when the absence is
expected and the code contains a comment explaining why.

### 10.4 Stale asynchronous completion

Any asynchronous result must be checked against the state it was created
for.

A save started against an old project or old editor state must not:

- overwrite a newer edit;
- update a newly opened project;
- hide a newer failure;
- falsely report success.

### 10.5 Component cleanup

Timers, subscriptions, listeners, and asynchronous ownership must have
explicit cleanup when:

- a component is destroyed;
- a route changes;
- a project changes;
- the application lifecycle invalidates the work.

### 10.6 Frontend testability

Prefer extracting pure geometry, transformation, and message-building
logic into focused TypeScript modules when that makes behavior directly
testable without duplicating application logic.

Presentational components that do not need the Wails bridge should be
mountable directly in component tests.

------------------------------------------------------------------------

## 11. Persistence and Migration Development

### 11.1 Before changing persistence

Identify:

1. which database is affected;
2. whether existing files must remain readable;
3. whether encrypted files are affected;
4. migration requirements;
5. rollback/recovery behavior;
6. backup implications;
7. fixture requirements;
8. release/package implications.

### 11.2 Project database migrations

Use the established migration style in `internal/db`.

Do not invent a parallel migration mechanism for one feature.

### 11.3 Encryption migration

Plaintext-to-encrypted project migration is security-sensitive.

The established sequence is:

1. reject an already encrypted source;
2. verify the plaintext source;
3. export with `sqlcipher_export` into a temporary encrypted sibling;
4. verify encrypted integrity;
5. retain the plaintext source as `<project>.pre-encryption.bak`;
6. publish the encrypted database only after validation;
7. tighten permissions where supported.

Do not "simplify" this sequence without regression evidence and explicit
security review.

### 11.4 Recovery-code relationship

The user's DEK may be wrapped by both the login password and valid
recovery codes.

Before enabling encryption for a user whose recovery state cannot
preserve the same DEK, recovery codes must be reissued according to the
established application flow. Otherwise password recovery could orphan
encrypted projects.

------------------------------------------------------------------------

## 12. Adding or Changing a Feature

### 12.1 Plan the vertical slice

Before implementation, map the feature across these possible layers:

``` text
Requirement
    ↓
Domain owner
    ↓
Persistence impact
    ↓
Application/Wails method
    ↓
Frontend consumer
    ↓
Export/report behavior
    ↓
Tests
    ↓
Documentation / ADR
```

Not every feature needs every layer.

Add only the boundaries required by the behavior.

### 12.2 Questions to answer before coding

- Which existing package owns the domain concept?
- Does the feature alter a persisted contract?
- Does it need a new Wails method?
- What frontend state owns the interaction?
- Does it affect export/report output?
- Does it affect encryption, recovery, audit, signing, or path
    confinement?
- What failure modes need typed errors?
- Which existing test most closely resembles the required coverage?
- Does an ADR or architecture document need updating?

### 12.3 New chart or document kinds

Use the existing registry.

A new kind should normally include:

- registration;
- persisted payload validation;
- focused renderer/exporter behavior;
- round-trip coverage;
- invalid-input coverage.

Keep bespoke renderers self-contained. Share primitives after
demonstrated reuse, not in anticipation of hypothetical reuse.

### 12.4 Avoid opportunistic refactors

Do not combine an unrelated architecture cleanup with a feature change
merely because the same files are open.

Small, coherent diffs are easier to review and safer to validate.

------------------------------------------------------------------------

# Part IV — Engineering Assurance

## 13. Testing Strategy

### 13.1 Evidence-first testing

Tests exist to protect requirements, invariants, and known risks.

Do not add tests only to increase a percentage. A useful test should
fail when a meaningful behavior regresses.

### 13.2 Guarded native desktop verification on macOS

For manual native Wails verification of a built app, use the repository-owned
launcher instead of starting the bundle directly:

```sh
make build
bash scripts/launch-isolated-native.sh
```

The launcher resolves the invoking account's home directory before starting the
child, creates a retained temporary `HOME`, `XDG_DATA_HOME`, `TMPDIR`, and
`CFFIXED_USER_HOME`, and launches the direct bundle executable under macOS
Seatbelt. Its constant, parameterized profile denies both current and legacy
project-data roots: `Library/Application Support/GoPMgr`, `Library/Application
Support/PMForge`, `Documents/GoPMgr`, and `Documents/PMForge`. It first proves
that a temporary allowed path is writable and a separate disposable denied
probe is neither readable nor writable; existing protected roots are also
checked for denied metadata access before the app is launched.

The command validates the bundle executable and identifier, refuses to run
while a GoPMgr process is already present, and keeps the temporary root after
exit so a create/edit/quit/relaunch test can reuse the same data. It does not
create an administrator or project. It protects the identified project-data
roots only; it is not evidence that macOS/WebKit writes no other ancillary
host preferences or caches, nor is it GUI evidence for a financial workflow.
Quit the child before deleting the printed root. The fake-bundle regression is
part of `make verify` as `make native-isolation-launch-tests`.

### 13.3 Focus first, broaden later

During development, run the narrowest test that exercises the changed
behavior.

Examples:

``` sh
go test -count=1 ./internal/db
go test -count=1 ./internal/users ./internal/crypto
go test -count=1 .
```

Then broaden verification according to risk.

### 13.4 Backend baseline

``` sh
go test . ./internal/...
```

For the repository-defined package set, prefer the Make target when
appropriate:

``` sh
make test
```

The Makefile intentionally avoids an indiscriminate bare `./...` in
quality gates because generated frontend-related Go packages can expose
unrelated package behavior.

### 13.5 Race detection

Use:

``` sh
go test -race . ./internal/...
```

or:

``` sh
make race
```

for concurrency-sensitive changes and before release claims.

### 13.6 Frontend checks

Relevant commands include:

``` sh
npm --prefix frontend run check
npm --prefix frontend run test
npm --prefix frontend run build
npm --prefix frontend run lint
make frontend-stability
make frontend-build-budget
make frontend-smoke
```

`frontend-smoke` matters because type checking and a Vite build do not
catch every Svelte load-time/runtime failure.

### 13.7 Frontend test pattern

The repository uses:

- Vitest;
- `@testing-library/svelte`;
- jsdom;
- direct component tests;
- pure TypeScript helper tests;
- mocked `window.go.main.App` for selected interaction tests.

When fixing a frontend regression, add the narrowest test that
reproduces the original failure class and retain runtime smoke coverage
where applicable.

### 13.8 Filesystem and persistence tests

Use temporary directories.

Do not embed developer-specific local paths into tests.

### 13.9 Deterministic tests

Avoid wall-clock dependence unless the clock is fixed or injected.

Use deterministic fixtures for cryptographic and persistence tests where
security properties do not require fresh randomness.

### 13.10 Encryption tests

Encryption work should cover the relevant subset of:

- wrong-key rejection;
- keyless rejection;
- integrity verification;
- encrypted file header behavior;
- migration row parity;
- already-encrypted rejection;
- recovery behavior;
- backup behavior.

### 13.11 PDF tests

Do not treat byte containment as sufficient proof for PDF/A or PAdES.

Test structural invariants and use validators where the claim requires
validator evidence.

------------------------------------------------------------------------

## 14. Repository Verification

### 14.1 Regular local gate

``` sh
make verify
```

Use this before handoff for ordinary changes unless the change's risk
requires additional gates.

At the current repository state, `verify` includes checks for
configuration, installer/tool pins, Windows installer scaffolding,
required font assets, clean-test safety, Wails version consistency,
package-version helpers, branding, Go tests, code-map currency, frontend
stability/build budget, coverage-ledger currency, and selected
repository regressions.

### 14.2 Linting

``` sh
make lint
```

or:

``` sh
make lint-go
make lint-frontend
```

### 14.3 License metadata

Every tracked file requires appropriate SPDX/REUSE coverage.

After adding files or generated assets:

``` sh
make license-check
```

### 14.4 Diff hygiene

Before completion:

``` sh
git diff --check
git diff --cached --check
```

Inspect the actual diff. A green test suite does not prove that
unrelated files were not accidentally changed.

------------------------------------------------------------------------

## 15. Debugging and Troubleshooting

### 15.1 Go tests fail because `frontend/dist` is missing

The root Go package embeds `frontend/dist`.

If direct Go tests need the embedded output and it is absent, build the
frontend first:

``` sh
npm --prefix frontend run build
```

or use the repository frontend build target.

### 15.2 Frontend type checks pass but the app fails at load

Run:

``` sh
make frontend-smoke
```

The smoke gate exists specifically because Svelte
rune/module-load/runtime problems can escape static checks and a normal
build.

### 15.3 Race detector reports a failure

Do not silence or skip the test first.

Identify:

- which state is shared;
- who owns it;
- whether the established application lock/helper was bypassed;
- whether asynchronous completion outlived its project/component;
- whether a test itself introduces unsafe shared state.

### 15.4 Encryption migration fails

Do not replace the migration with an in-place shortcut.

Check:

- source integrity;
- whether the source is already encrypted;
- SQLCipher driver path;
- temporary destination;
- destination integrity;
- DEK availability;
- recovery-code state;
- backup/publish phase.

### 15.5 PDF/A validation fails

Run the strict PDF/A gate:

``` sh
make check-pdfa
```

The repository expects veraPDF directly or through the supported Docker
path. If conformance cannot be verified, do not convert that absence
into a passing claim.

### 15.6 PAdES validation fails

Start with:

``` sh
make check-pades
```

Then use:

``` sh
make check-pades-external
make pades-harness-tests
```

as required.

Separate:

- PDF structure;
- CMS/signature structure;
- tamper evidence;
- certificate-chain trust.

A self-signed deterministic test certificate can prove structure and
tamper detection but not public trust.

### 15.7 Vulnerability scan changes without a code change

Live vulnerability databases change.

When a vulnerability check changes state, inspect:

- the advisory;
- reachable symbols;
- affected dependency version;
- whether the vulnerable path is actually called;
- whether a fixed dependency/toolchain release exists.

Do not assume a newly red scan proves a repository regression, and do
not dismiss it merely because no source changed.

------------------------------------------------------------------------

## 16. Concurrency and Resource Ownership

### 16.1 Prefer synchronous APIs

Synchronous code is easier to reason about when the operation does not
need independent lifetime or concurrency.

### 16.2 Goroutine checklist

Before starting a goroutine, answer:

- Who owns it?
- How does it stop?
- What happens on project change?
- What happens on application shutdown?
- Where do errors go?
- What shared state can it touch?
- How is that state protected?
- How is its behavior tested?

If those answers are unclear, do not start the goroutine yet.

### 16.3 Frontend asynchronous checklist

For delayed frontend work, answer:

- Which component/project/editor instance owns the operation?
- Can the owner disappear before completion?
- Can a newer operation supersede it?
- How is stale completion rejected?
- How is cleanup performed?
- Can failure be accidentally hidden by a later state update?

------------------------------------------------------------------------

## 17. Security Engineering

### 17.1 Security boundary

GoPMgr is local-first. Its primary security boundary is the local
machine and OS account.

The application nevertheless must protect project data from:

- casual local disclosure;
- cross-user project access;
- unsafe path handling;
- accidental secret logging;
- encryption/recovery failures;
- release drift;
- signed-document tampering.

### 17.2 Project path confinement

Every frontend-supplied project path must be confined to the signed-in
user's own project directory before disk access.

Use the established `projectPathFor` path-confinement flow.

Never rely on the frontend to supply a safe path.

### 17.3 Secrets

Never log or commit:

- passwords;
- recovery codes;
- raw DEKs;
- real wrapped-key ciphertexts;
- SQLCipher keys or raw key DSNs;
- private signing keys;
- real local certificates unless explicitly intended and safe;
- real project databases;
- exports containing real user data.

Use temporary directories and deterministic fixtures in tests.

### 17.4 Authentication

Credentials are stored as Argon2id PHC strings.

Login failures should not expose whether the username or password was
the failing element.

### 17.5 Encryption key hierarchy

The intended hierarchy is:

``` text
login password ─┐
                ├─ wraps per-user DEK ─→ SQLCipher project databases
recovery code ──┘
```

The DEK is the project-database encryption key material. It should not
be stored unwrapped as normal persistent metadata.

### 17.6 Audit integrity

Project databases contain a tamper-evident `audit_events` hash chain.

Each event derives its hash from the previous event hash and
canonicalized payload data.

`VerifyAuditChain` checks sequence continuity, prior-hash linkage, and
recomputed hashes.

When Compliance Mode is enabled, project opening verifies the audit
chain and rejects altered projects.

Important limitation: audit integrity detects tampering; it does not
prevent a holder of the DEK from rewriting the database. It complements
encryption and OS-level protection.

------------------------------------------------------------------------

# Part V — Documents, Export, and Signing

## 18. Export Development

### 18.1 Export formats

The repository includes export paths for formats including:

- PDF;
- PDF/A;
- DOCX;
- ODT;
- XLSX;
- iCal;
- MSPDI;
- Monte Carlo risk reports.

Changes to exported formats are compatibility-sensitive when users or
external tools consume them.

### 18.2 Spreadsheet formula injection

User-controlled values written to CSV/TSV must pass through the
established `internal/exportsafe` protection.

When extending delimited exports, add regression tests for dangerous
input classes.

### 18.3 Validate claims, not appearances

A file opening successfully does not prove conformance.

For standards-based output, use the appropriate validator or keep the
limitation explicit.

------------------------------------------------------------------------

## 19. PDF/A-3

### 19.1 Mutation order

PDF/A metadata and output intent must be applied before PAdES signing.

Conceptually:

``` text
render PDF content
    ↓
apply PDF/A metadata / XMP / output intent
    ↓
validate appropriate PDF/A invariants
    ↓
apply PAdES signature as final PDF mutation
```

### 19.2 Validation

Use:

``` sh
make check-pdfa
```

The gate is strict by default.

If the required validator is unavailable, install it for release
validation rather than weakening the public conformance claim.

------------------------------------------------------------------------

## 20. PAdES Signing

### 20.1 Signing is the final mutation

PAdES signs PDF byte ranges.

Any PDF mutation after signing can invalidate the signature.

Therefore:

``` text
content → PDF/A work → PAdES
```

is an invariant.

### 20.2 Local structural validation

Use:

``` sh
make check-pades
```

for deterministic local PAdES validation.

### 20.3 External validation

Use:

``` sh
make check-pades-external
```

to exercise available external tools such as OpenSSL, qpdf, pdfsig,
veraPDF, or DSS where installed.

### 20.4 Trusted-source validation

Use:

``` sh
make check-pades-trusted
```

for trusted-source evidence classification.

Do not confuse controlled classification tests with proof of real
certificate trust.

### 20.5 Harness regressions

Use:

``` sh
make pades-harness-tests
```

for the deterministic generator and validator-behavior regression
matrix.

------------------------------------------------------------------------

# Part VI — Maintenance and Release Engineering

## 21. Dependency Maintenance

### 21.1 Dependency changes are architectural work

A dependency can affect:

- build reproducibility;
- licensing;
- security;
- native linkage;
- package size;
- release tooling;
- platform compatibility;
- validation.

Do not add one solely to avoid writing a small amount of straightforward
code.

### 21.2 Before adding a dependency

1. Read the existing code path.
2. Check whether the standard library or an existing dependency is
    adequate.
3. Check whether the dependency duplicates an existing abstraction.
4. For security-sensitive code, inspect maintenance, license, native
    requirements, and packaging consequences.
5. Change Go/npm metadata only when the dependency change is
    intentional.
6. Run focused tests and relevant release gates.

### 21.3 Major dependency roles

Current important dependencies include:

- Wails for desktop runtime/bridge;
- go-sqlcipher for SQLite/SQLCipher;
- `golang.org/x/crypto` for cryptographic support;
- PKCS#7/RFC 3161 libraries for signing/timestamp support;
- `go-pdf/fpdf` for PDF generation;
- `godocx` for DOCX;
- `excelize` for XLSX;
- `rickar/cal` for holiday calendars;
- `zen-go` for JDM rules;
- Gonum for numerical/statistical work;
- DuckDB for tagged analytics builds.

Use `go.mod` and `frontend/package.json` for authoritative versions.

------------------------------------------------------------------------

## 22. Documentation and ADRs

### 22.1 Documentation standard

Public documentation must describe current behavior.

If something is:

- planned — say it is planned;
- experimental — say it is experimental;
- gated — name the gate;
- platform-limited — name the limitation;
- unverified — do not imply it was verified.

### 22.2 Evidence-linked claims

Prefer documentation that names the command or source that verifies a
claim.

Avoid pasting long generated logs into durable documentation. Summarize
the result and point to the reproducing command or artifact.

### 22.3 When to update architecture documentation

Architecture Decision Records live under `docs/design/` as
`ADR-NNN-title.md`, alongside (not separated from) other subsystem design
proposals. Current ADRs:

- `docs/design/ADR-001-database-encryption-at-rest.md`
- `docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md`
- `docs/design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md`

Update `ARCHITECTURE.md` or add/update an ADR when a change alters a
durable architectural contract, including:

- package boundaries;
- persistence authority;
- major dependency choice;
- encryption model;
- export/signing architecture;
- runtime shape;
- release architecture.

### 22.4 Documentation hygiene

Never include:

- developer-local absolute paths;
- personal account names;
- credentials;
- private handoff/session material;
- unpublished release claims presented as fact.

------------------------------------------------------------------------

## 23. Release Engineering

### 23.1 Release gate

The final repository release gate is:

``` sh
make check-release
```

It currently covers major release contracts including version
consistency, configuration policy, installer/tool pins, REUSE/SPDX,
frontend build/stability, release-scope guards, runtime smoke,
memory-safety scanning, race tests, production build, encrypted-database
validation, PDF/A validation, and PAdES regression validation.

Do not describe a release as validated unless the relevant gate actually
ran.

### 23.2 Tag preflight

For publication-tag validation:

``` sh
GOPMGR_RELEASE_TAG=v<version> make tag-preflight
```

Use the actual intended version/tag according to repository release
policy.

The tag preflight validates the publication-tag contract before running
the full release gate.

### 23.3 Release claims

`make release-scope` exists to prevent public claims — especially
encryption, PDF/A, PAdES, and release-state claims — from drifting away
from supported behavior.

Run it after documentation changes that affect those claims.

### 23.4 Native installers

Platform packaging has platform-specific evidence requirements.

Regular CI (`.github/workflows/ci.yml`, on push/PR) runs entirely on
`ubuntu-26.04` — it validates `make verify`, linting, the PAdES harness,
vulnerability scanning, and a Linux Wails build, but it never builds or
runs the macOS or Windows installer. The three-platform packaging matrix
(Linux `.deb`/`.rpm`, macOS `.dmg`, Windows `.exe`) only runs in
`.github/workflows/release.yml`, triggered by a `v*` tag push. A change
that only passed regular CI has not been exercised on macOS or Windows at
all.

The Linux packaging job specifically stays pinned to `ubuntu-24.04` rather
than following the rest of CI onto `ubuntu-26.04`: GitHub's `ubuntu-26.04`
hosted runner is still experimental, and this job only runs on a rare,
high-stakes tag push — a poor place to absorb a beta runner's instability.
Building on the older 24.04 baseline is deliberate build-old-run-new
packaging practice: the resulting `.deb`/`.rpm`'s only runtime requirement
is `libwebkit2gtk-4.1`, present on 24.04 and every release since, so it
installs and runs correctly on this project's actual target
audience — Ubuntu 26.04+ users — without the release pipeline itself
depending on an experimental runner. `scripts/check-linux-runtime-target.sh`
(run by `make check-release`) enforces this split explicitly.

A source/template check is not equivalent to installing and exercising
the resulting package on the target operating system.

Keep that distinction explicit in release notes and validation records.

------------------------------------------------------------------------

## 24. Vulnerability Management

### 24.1 Live advisories

Dependency vulnerability status can change without a source commit
because vulnerability databases are updated independently.

When a new advisory appears:

1. identify the affected module/toolchain;
2. determine whether GoPMgr reaches the vulnerable symbols;
3. check for a fixed version;
4. evaluate upgrade/build/package impact;
5. run relevant tests;
6. document any accepted temporary exposure accurately.

### 24.2 Do not overstate scanner results

"Scanner clean" is evidence from a specific tool at a specific time. It
is not proof that no vulnerability exists.

Likewise, a dependency advisory does not automatically prove the
application reaches the vulnerable code path.

------------------------------------------------------------------------

## 25. Agent-Assisted Development

### 25.1 Repository instructions apply to agents

Automated coding agents must read `AGENTS.md` before substantive work.

### 25.2 Session startup

Before editing:

``` sh
pwd
git status --short
```

Confirm the checkout. Do not assume an old handoff path is still
correct.

Read relevant source before proposing changes.

### 25.3 Dirty worktrees

Assume unrelated user or agent work may exist.

Do not:

- revert changes you did not make;
- stage the whole repository indiscriminately;
- rewrite unrelated files;
- "clean up" another task's work.

Prefer explicit path/hunk staging.

### 25.4 Planning

Plan multi-file work before implementation.

The plan should identify:

- behavior being changed;
- package ownership;
- compatibility risk;
- tests;
- documentation;
- security/release implications.

### 25.5 Completion evidence

Before an agent claims completion, it should report:

- changed files;
- relevant tests/checks actually run;
- failures or unavailable tools;
- unresolved risks;
- any architecture/security/release follow-up.

Never infer a pass for a command that did not run.

------------------------------------------------------------------------

# Part VII — Contributor Workflow

## 26. Planning a Change

Use this sequence for non-trivial work.

### Step 1 — Establish repository state

``` sh
pwd
git status --short
```

### Step 2 — Identify the owning domain

Search for the existing implementation, tests, and nearest package
patterns.

### Step 3 — Identify contracts

Ask whether the change touches:

- database schema;
- project files;
- bridge APIs;
- user-facing error strings;
- export formats;
- encryption/recovery;
- audit;
- signing;
- release claims.

### Step 4 — Identify tests before editing

Find the narrowest existing tests that demonstrate the expected style.

### Step 5 — Plan validation

Choose focused checks first and broader gates based on risk.

------------------------------------------------------------------------

## 27. Implementing a Change

1. Make the smallest coherent change that satisfies the requirement.
2. Follow existing package patterns.
3. Add/update focused regression tests with behavior changes.
4. Preserve error identity and compatibility contracts.
5. Keep frontend/backend ownership explicit.
6. Avoid unrelated refactors.
7. Format changed code.
8. Run focused tests immediately.

------------------------------------------------------------------------

## 28. Reviewing a Change

Review the diff as if the tests did not exist.

Check:

- Is the behavior in the correct package?
- Did the change introduce a second source of truth?
- Are Wails methods still adapters?
- Is project state accessed through established helpers?
- Are paths confined?
- Are secrets excluded from logs/errors?
- Are errors wrapped correctly?
- Can asynchronous work become stale?
- Is persistence migration safe?
- Are exports deterministic?
- Does signed-PDF mutation order remain correct?
- Are comments explaining non-obvious decisions rather than narrating
    syntax?
- Did documentation make a claim stronger than the evidence?

Then review the tests:

- Would they fail for the original defect?
- Do they test behavior rather than implementation trivia?
- Are fixtures deterministic?
- Are important failure paths covered?
- Is validator evidence required?

------------------------------------------------------------------------

## 29. Validation Before Handoff

For a normal change, start with relevant focused tests and then:

``` sh
make verify
git diff --check
git diff --cached --check
```

Add as applicable:

``` sh
make race
make lint
make license-check
make frontend-smoke
make check-encrypted-db
make check-pdfa
make check-pades
make check-pades-external
make release-scope
```

For release-sensitive work:

``` sh
make check-release
```

A command that could not run must be reported as not run, with the
reason.

------------------------------------------------------------------------

# Appendix A — Command Reference

| Command | Purpose |
| — | — |
| `make tidy` | `go mod tidy` + `npm --prefix frontend install`; the real onboarding step |
| `make dev` | Run Wails development mode |
| `make build` | Production Wails build with repository build wrapper |
| `make clean` | Remove build output and generated `frontend/wailsjs/` bindings |
| `make fonts`, `make icc` | Fetch required embedded font/ICC assets before first build |
| `make test` | Go test gate over repository-defined Go packages |
| `make race` | Go race-detector gate |
| `make verify` | Regular local pre-commit/handoff gate (config, installer/tool pins, Go tests, code-map currency, frontend stability/budget, coverage-ledger currency, and related contract checks — see 14.1) |
| `make lint` | Go + frontend lint (`lint-go` + `lint-frontend`) |
| `make frontend-stability` | Svelte checks and frontend regressions |
| `make frontend-build-budget` | Frontend build and route-split budget enforcement |
| `make frontend-smoke` | Svelte runtime load/render smoke check |
| `make check-encrypted-db` | SQLCipher project DB create/open/migration/backup validation |
| `make check-pdfa` | Strict PDF/A-3 validation |
| `make check-pades` | Deterministic local PAdES validation |
| `make check-pades-external` | External PAdES validators where available |
| `make check-pades-trusted` | Trusted-source PAdES evidence classification |
| `make pades-harness-tests` | PAdES generator/validator regression harness |
| `make license-check` | REUSE/SPDX validation |
| `make memory-scan` | Memory-safety hardening gate |
| `make release-scope` | Protect release/public-claim scope |
| `make check-release` | Full release gate |
| `make tag-preflight` | Publication-tag contract + full release gate |
| `make package-linux`, `make package-darwin`, `make package-macos-installer`, `make package-windows` | Platform installer packaging (exercised per-platform only in `release.yml`; see 23.4) |
| `make wails-version` | Verify installed Wails CLI matches the `go.mod`/docs pin |
| `make code-map` | Regenerate first-party package dependency/API maps |
| `make code-map-current` | Fail if checked-in code map is stale |
| `make coverage-ledger-current` | Fail if `TEST_COVERAGE_LEDGER.md` disagrees with the live test-file set |
| `make coverage-ledger-drift` | Fail if a ledger coverage-percentage heading no longer matches live `go test -cover` output (not in `verify`; needs the DuckDB toolchain) |
| `make coverage-ratchet` | Check recorded coverage high-water marks (not in `verify`; same DuckDB-toolchain reason) |
| `make coverage-ratchet-update` | Record legitimate improved coverage marks |

Run `make help` for the live Makefile target list — it has 56
targets; this table curates the ones a developer needs most often.

------------------------------------------------------------------------

# Appendix B — System Invariants

These are high-value rules that should remain easy to find.

1. The root main package remains compatible with the Wails build
    layout.
2. Wails-bound methods remain thin adapters/orchestrators rather than a
    replacement domain layer.
3. `internal/sqlitedriver` owns SQLite/SQLCipher driver registration.
4. `system.db` contains bootstrap/account metadata, not normal project
    content.
5. Existing `.gopmgr` files are compatibility contracts.
6. Persistence migrations require a tested compatibility/recovery
    story.
7. Active project/user state uses established locking and state
    helpers.
8. Partially initialized application state must never be published.
9. Stale asynchronous completion must not modify newer project/editor
    state.
10. Background work requires explicit ownership, cancellation, error
    handling, and cleanup.
11. Frontend-supplied project paths are confined to the signed-in user's
    project directory before disk access.
12. Passwords, recovery codes, raw DEKs, SQLCipher keys, and private
    signing keys must not appear in logs or commits.
13. Error identity is preserved with `%w` and inspected with
    `errors.Is`/`errors.As`.
14. User-facing error strings that are frontend contracts are changed
    only after checking consumers.
15. PDF content and PDF/A mutations occur before PAdES signing.
16. PAdES is the final PDF mutation.
17. Validator-dependent claims require validator evidence.
18. Public release claims must remain aligned with repository gates.
19. A check is never reported as passing unless it actually ran.
20. Unrelated worktree changes are not reverted, restaged, or rewritten.

------------------------------------------------------------------------

# Appendix C — Development Checklists

## Backend change

- [ ] Read the owning package and nearest tests.
- [ ] Confirm the behavior belongs in the selected package.
- [ ] Keep Wails adapters small.
- [ ] Preserve sentinel error identity.
- [ ] Use established database/state helpers.
- [ ] Add focused regression coverage.
- [ ] Run `gofmt`.
- [ ] Run focused Go tests.
- [ ] Broaden to `make verify` as appropriate.

## Frontend change

- [ ] Confirm Svelte/TypeScript ownership.
- [ ] Keep Wails declarations synchronized.
- [ ] Handle Wails promise failures explicitly.
- [ ] Protect against stale async completion.
- [ ] Clean up timers/listeners/subscriptions.
- [ ] Add component/unit regression coverage where useful.
- [ ] Run frontend check/test.
- [ ] Run `make frontend-smoke` for runtime-sensitive changes.
- [ ] Run `make verify`.

## Persistence change

- [ ] Identify `system.db` vs `.gopmgr` ownership.
- [ ] Define compatibility requirements.
- [ ] Use established migration helpers/patterns.
- [ ] Test pre-change/representative data where required.
- [ ] Test failure and recovery.
- [ ] Include encrypted database behavior if applicable.
- [ ] Run focused DB tests.
- [ ] Run `make check-encrypted-db` when encryption is affected.
- [ ] Run broader gates.

## Security-sensitive change

- [ ] Identify trust boundary.
- [ ] Add regression test before changing critical behavior where
    practical.
- [ ] Confirm no secret logging.
- [ ] Confirm path confinement.
- [ ] Confirm fail-hard behavior where integrity requires it.
- [ ] Review recovery behavior.
- [ ] Run relevant encryption/signing validators.
- [ ] Run race tests if shared state is involved.
- [ ] Run release-sensitive gates.

## PDF/signing change

- [ ] Preserve render → PDF/A → PAdES order.
- [ ] Test structural invariants.
- [ ] Run `make check-pdfa`.
- [ ] Run `make check-pades`.
- [ ] Run external/harness validation when applicable.
- [ ] Re-sign after any signed-PDF mutation.
- [ ] Avoid claiming trust from self-signed structural evidence.

## Release-sensitive change

- [ ] Run focused tests.
- [ ] Run `make verify`.
- [ ] Run `make license-check`.
- [ ] Run `make release-scope`.
- [ ] Run affected specialized validators.
- [ ] Run `make check-release`.
- [ ] Record unavailable platform/tool evidence explicitly.
- [ ] Verify public documentation matches actual evidence.

------------------------------------------------------------------------

# Appendix D — Troubleshooting Matrix

| Symptom | First checks |
| — | — |
| Root Go package will not compile after clean checkout | Confirm `frontend/dist` exists; build the frontend |
| Svelte checks pass but runtime mount fails | `make frontend-smoke` |
| Race detector fails | State ownership, project switching, goroutine lifetime, locks |
| Existing project no longer opens | Migration/schema compatibility, encryption key path, fixture regression |
| Encryption migration fails | Source integrity, already-encrypted state, DEK, SQLCipher export, destination integrity |
| User can reference an unexpected project path | `projectPathFor` confinement and regression tests |
| Export opens but standards validation fails | Use a format-specific validator; do not rely on visual inspection |
| PAdES becomes invalid after export processing | Check for mutation after signing |
| Vulnerability gate changes without source changes | Inspect the live advisory and reachable symbols |
| `wails-window.d.ts` disagrees with backend behavior | It is hand-written, not generated (4.3) — update it by hand against the Go method and `frontend/wailsjs/` |
| `make verify` fails on generated map/ledger | Regenerate only through the repository-owned target; inspect the diff before committing |
| Release documentation claims fail the scope gate | Reduce/correct the claim or supply the required gate-backed evidence |
| Cross-platform installer bug not caught by CI | Regular CI (`ci.yml`) runs Linux-only; macOS/Windows packaging only builds in `release.yml` at tag push (23.4) |

------------------------------------------------------------------------

# Appendix E — Glossary

**ADR** — Architecture Decision Record documenting a durable
architectural choice.

**CGO** — Go facility for calling C code. Required by important GoPMgr
native database/analytics paths.

**DEK** — Data-encryption key. GoPMgr uses a per-user 32-byte DEK for
encrypted project databases.

**EVM** — Earned Value Management.

**PAdES** — PDF Advanced Electronic Signatures. In GoPMgr, signing
must be the final PDF mutation.

**PDF/A-3** — Archival PDF conformance target used by GoPMgr's PDF/A
export path.

**PHC string** — Standard encoded representation used for password
hashes such as Argon2id.

**SQLCipher** — SQLite-compatible encrypted database technology used
for encrypted `.gopmgr` files.

**Wails bridge** — The Go-to-JavaScript binding surface exposed to the
Svelte desktop frontend.

**Wails-bound method** — An exported method on the application object
callable from the frontend.

------------------------------------------------------------------------

# Appendix F — Related Repository Documentation

Read these focused documents when working in the corresponding area:

- `ARCHITECTURE.md` — runtime shape, data layout, important
    packages, frontend/release architecture.
- `STYLE.md` — repository, Go, frontend, documentation, and
    security-sensitive coding conventions.
- `ERROR_HANDLING.md` — sentinel errors, wrapping, diagnostics,
    fail-soft/fail-hard behavior, frontend error handling.
- `TESTING.md` — local, frontend, race, PDF, PAdES, release, and
    test-style guidance.
- `SECURITY.md` — local accounts, encryption, secrets, document
    security, audit integrity, release safety.
- `DEPENDENCIES.md` — toolchain, Go/frontend dependencies, external
    validators/tools, dependency-change rules.
- `AGENTS.md` — repository operating rules for automated engineering
    agents.
- `TEST_COVERAGE_LEDGER.md` — test coverage inventory and assurance
    tracking.
- `code-map/` — generated architectural/API maps, regenerated by
    `make code-map`: `package-dependencies.json` (first-party import
    graph), `callgraph.json`, `public-api-map.json`, `symbols.json`, plus
    a hand-maintained `recent-decisions.md` index.
- `docs/design/` — ADRs (`ADR-NNN-title.md`) and other
    subsystem-specific design proposals, side by side in one directory.
- `Makefile` — authoritative local command orchestration.
- `scripts/` — executable validation and release policy.

------------------------------------------------------------------------

*This handbook is grounded against the repository state as of the
`internal/` package inventory, frontend structure, ADR set, CI workflows,
and Makefile targets verified during its 2026-08-19 revision. Re-verify
the package table, CI platform matrix, and command reference the next
time any of them changes materially — see the source-of-truth hierarchy
in the Preface.*
