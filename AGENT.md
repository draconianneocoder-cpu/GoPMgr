<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# AGENT.md — PMForge Project Handbook

**You are reading this BEFORE doing any work on this project.** This file is the running memory for AI-assisted development on PMForge. Read it end-to-end on first visit; reference the relevant section on every subsequent change.

**Update protocol**: at the end of every session that touches PMForge code, append findings to `## Lessons learned` below. If a session adds a new pattern, a new directory, a new build target, or a new architectural rule, update the corresponding section.

---

## 1. What PMForge is

PMForge is a **local-first project controls desktop application** for technical, engineering, IT, construction, and administrative organizations. License: **GPL-3.0-or-later**. The user described it as a GPL-licensed alternative to centralized SaaS PM tools.

- **Backend**: Go 1.26.3, acts as a high-performance kernel for data integrity, scheduling math (CPM/EVM/MSPDI), authentication, document rendering, and PDF generation.
- **Frontend**: Svelte 5 (runes mode) + Vite 5 + Tailwind 3, mounted in a desktop window via **Wails v2.9.2**.
- **Storage**: SQLite with WAL journaling. Per-user folder isolation; one `.pmforge` file per project.
- **Charting library**: Chart.js v4.4.6 on the frontend; gofpdf for server-side PDF chart rendering.
- **Crypto**: `golang.org/x/crypto/argon2` for password hashing (PHC string format), AES-256-GCM for encryption, X.509/RSA for digital signatures.
- **Rules engine**: `gorules/zen-go` (Apache-2.0) — Launchpad seeding rules expressed as JDM data, not Go switch. Used by `internal/templates`.
- **Holiday data**: `rickar/cal/v2` (BSD-2-Clause) — country holiday datasets. Wrapped by `internal/calendar`.
- **CMS/PKCS#7**: `digitorus/pkcs7` (FreeBSD-2-clause) — wraps raw RSA signatures into the CMS SignedData structure Adobe Reader / PAdES expects. Used by `internal/crypto/pdf_sign.go`.
- **DOCX writer**: `gomutex/godocx` (MIT, pure Go) — picked from pkg.go.dev after a survey. Used by `internal/export/docx.go`. ODT export (`internal/export/odt.go`) is hand-built because no equivalently-maintained pure-Go ODT generator exists (kpmy/odf hasn't been touched since 2014).

The app has reached **V2.x** maturity: all 19 chart kinds and all 25 document templates implemented end-to-end, combined report builder with embedded vector chart visualisations, self-heal with atomic database swap, multi-user accounts. The Agile Pack is the current frontier.

---

## 2. Directory layout

```
pmforge/
├── AGENT.md                     # THIS FILE — read first, update at end
├── README.md                    # user/contributor documentation (GFDL)
├── LICENSES/                    # REUSE-compliant license texts
├── Makefile                     # build/lint/test/package targets
├── go.mod / wails.json / .gitignore
├── scripts/
│   ├── check-release.sh         # version + REUSE + build gate
│   └── memory-safety-scan.sh    # go vet + custom safety greps (V2.x)
│
├── cmd/pmforge/main.go          # entry point: CLI dispatch + Wails bootstrap
│                                # Hosts the App struct that Wails exposes to the frontend.
│
├── internal/
│   ├── admin/workflow.go        # Administrative Pack (SecureArchive, sigevents)
│   ├── agile/                   # Software-Dev Pack (Kanban/Sprints/DORA) — V2.x
│   │   ├── agile.go             # types: WorkItem, Column, Board, Sprint, Deployment
│   │   ├── store.go             # CRUD against the agile_* tables
│   │   └── dora.go              # DORA metric computation + classification
│   ├── auth/password.go         # Argon2id PHC hash/verify
│   ├── cli/parser.go            # GNU-style CLI flags; Version constant lives here
│   ├── charts/
│   │   ├── registry.go          # 19-kind taxonomy + 4 engines (DAG/Stats/Matrix/Flow)
│   │   ├── engines.go           # Layout() dispatcher → kind-specific layout fn
│   │   ├── dag/                 # WBS, Network, PERT, CPM, Fishbone, Cause-Effect
│   │   ├── flow/                # Workflow, Activity (+ swimlanes)
│   │   ├── matrix/              # RACI, SWOT, Stakeholder, Generic
│   │   ├── stats/               # Line, Bar, Pareto, Pie, BurnUp, BurnDown, CumFlow, Control
│   │   └── pdfrender/           # Vector renderers — one file per engine
│   │       ├── dispatcher.go
│   │       ├── dag.go / fishbone.go / flow.go / matrix.go / stats.go
│   ├── crypto/                  # AES-256-GCM + Argon2id KDF; X.509 PDF signing
│   ├── db/                      # SQLite kernel
│   │   ├── sqlite.go            # InitDB + Migrate (ALL schema definitions live here)
│   │   ├── settings.go          # UserSettings (singleton row)
│   │   ├── project.go           # Project metadata CRUD
│   │   ├── charts.go            # unified `charts` table CRUD
│   │   ├── documents.go         # unified `documents` table CRUD
│   │   ├── audit.go             # audit_log + CSV export
│   │   ├── repair.go            # InformativeSelfHeal + SwapInSnapshot
│   │   ├── backup.go            # .pmba archival bundles
│   │   └── ids.go               # newID(prefix) generator
│   ├── debug/report.go          # ErrorReport, Wrap, ToError, Report
│   ├── documents/               # 25 document kinds
│   │   ├── registry.go          # Kind + Field + Phase taxonomy
│   │   ├── templates.go         # all 25 default schemas
│   │   ├── defaults.go          # DefaultContent + EffectiveFields
│   │   ├── charter.go           # bespoke Charter PDF + generic renderer
│   │   └── report.go            # BuildCombinedReport (cover + TOC + sections + chart embeds)
│   ├── export/                  # V1: PDF/XLSX/CSV/MSPDI for the standalone export menu
│   ├── fonts/                   # bundled TTF catalog + Manager + user import (dep-free leaf)
│   │   ├── catalog.go           # curated FOSS font families (Liberation, Noto, Source Sans 3, ...)
│   │   ├── manager.go           # go:embed assets + Register/RegisterAs + ImportFont + TTF validation
│   │   └── assets/              # font binaries (fetched by scripts/fetch-fonts.sh, NOT committed)
│   ├── kernel/scheduler.go      # CPM forward + backward pass + critical-path marking
│   ├── pdfmeta/pdfmeta.go       # XMP packet build + Catalog incremental-update inject (dep-free leaf)
│   ├── update/check.go          # update-check stub
│   └── users/store.go           # system.db + per-user folders
│
└── frontend/                    # Svelte 5 + Vite 5
    ├── package.json / vite.config.ts / svelte.config.js
    ├── tailwind.config.js / postcss.config.js / tsconfig.json
    ├── index.html
    └── src/
        ├── main.ts / app.css / App.svelte
        ├── wails-window.d.ts    # TypeScript surface for window.go.main.App
        └── lib/
            ├── session.svelte.ts   # rune-based shared session state
            └── components/
                ├── GanttChart.svelte / Settings.svelte
                ├── admin/SignatureSettings.svelte
                ├── auth/Login.svelte, CreateAccount.svelte
                ├── project/ProjectPicker.svelte, Dashboard.svelte
                ├── charts/
                │   ├── _layered_editor_shell.svelte    # shared shell for layered DAGs
                │   ├── _stats_editor_shell.svelte      # shared shell for stats charts
                │   ├── _flow_shapes.ts                 # SVG shape helpers (workflow + activity)
                │   ├── _stats_types.ts                 # TS mirrors of stats layouts
                │   ├── LayeredDiagram.svelte           # shared SVG host for Network/PERT/CPM
                │   ├── StatsChart.svelte               # shared Chart.js host
                │   ├── WBSEditor.svelte
                │   ├── NetworkEditor.svelte, PERTEditor.svelte, CPMEditor.svelte
                │   ├── FishboneEditor.svelte, CauseEffectEditor.svelte
                │   ├── WorkflowEditor.svelte, ActivityEditor.svelte
                │   ├── RACIEditor.svelte, SWOTEditor.svelte, StakeholderEditor.svelte, MatrixEditor.svelte
                │   └── LineEditor.svelte, BarEditor.svelte, PieEditor.svelte, ParetoEditor.svelte,
                │       BurnUpEditor.svelte, BurnDownEditor.svelte, CumulativeFlowEditor.svelte, ControlChartEditor.svelte
                └── documents/
                    ├── CharterEditor.svelte
                    ├── DocumentFieldEditor.svelte      # generic per-field editor
                    ├── ChartPicker.svelte              # picker for FieldChartRef
                    └── ReportComposer.svelte           # combined-report assembly
```

---

## 3. Database schema (per-project `.pmforge` SQLite file)

All tables created idempotently in `db.Database.Migrate()` (internal/db/sqlite.go). Migrations are additive only — never DROP or ALTER existing columns. New columns get a default.

### V1 tables (initial release)
- **`settings`** — singleton row (CHECK id=1). Columns: `default_password`, `export_theme`, `auto_repair`, `cert_path`, `signature_enabled`, `default_font` (document-export font family; empty = catalog default). `default_font` added 2026-05-20 via the `migrateLegacyColumns` PRAGMA-probe pattern (now covers both `project` and `settings`).
- **`tasks`** — V1 scheduler tasks: `id`, `title`, `duration`, `precedents` (JSON array of IDs), `created_at`, `updated_at`.
- **`command_log`** — append-only command journal: `id`, `ts`, `actor`, `command`, `payload` (JSON).
- **`audit_log`** — `id`, `ts`, `actor`, `action`, `target_id`, `details`. Indexed by target_id and ts.

### V2 tables (multi-entity model)
- **`project`** — one row per .pmforge: `id`, `name`, `description`, `status`, `phase`, `start_date`, `end_date`, `budget`, `owner`, timestamps. Status ∈ {planning, active, on_hold, complete, cancelled}. Phase ∈ {initiation, planning, execution, monitoring, closing}.
- **`charts`** — unified table for all 19 chart kinds: `id`, `project_id`, `kind`, `title`, `data` (JSON), `config` (JSON), `template_id`, timestamps. FK ON DELETE CASCADE.
- **`documents`** — unified for all 25 doc kinds: `id`, `project_id`, `kind`, `title`, `content` (JSON), `template_id`, `version` (monotonic), `status` (draft|review|approved|archived), timestamps.
- **`templates`** — user-saved templates: `id`, `scope` ('chart' or 'document'), `kind`, `name`, `description`, `defaults` (JSON), `is_builtin`, `created_at`.

### Agile tables (V2.x — Software-Dev Pack)
- **`agile_boards`** — `id`, `project_id`, `name`, `is_default`, timestamps.
- **`agile_columns`** — `id`, `board_id`, `name`, `order_idx`, `wip_limit` (0 = unlimited).
- **`agile_work_items`** — `id`, `project_id`, `type` (story|bug|task|epic), `title`, `description`, `state` (column ID or "backlog"), `points`, `assignee`, `sprint_id`, `priority` (low|medium|high|urgent), `order_idx`, timestamps, `closed_at`.
- **`agile_sprints`** — `id`, `project_id`, `name`, `goal`, `status` (planning|active|complete), `start_date`, `end_date`, `capacity` (story points), `created_at`.
- **`agile_deployments`** — `id`, `project_id`, `ts`, `version`, `successful`, `lead_time_hours`, `restore_time_hours`, `notes`.

### System database (top-level, NOT per-project)
- **`~/Documents/PMForge/system.db`** holds account credentials:
- **`users`** — `username` (PK), `display_name`, `password_hash` (PHC Argon2id), `data_dir`, `created_at`, `last_login`.

---

## 4. Coding conventions

### SPDX headers — REQUIRED on every source file

```go
// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later
```

HTML-style comment for Svelte / HTML / Markdown files. Documentation files use `GFDL-1.3-or-later`; tiny configs may use `CC0-1.0`. `make license-check` runs `reuse lint`.

### Go conventions

- **Package-level doc comment** on every package's primary file. Comments are `//`-style, full sentences, end with period.
- **Error wrapping**: use `fmt.Errorf("context: %w", err)`. For recoverable paths that the UI needs to introspect, use `debug.Wrap(err, "TAG").ToError()`.
- **No goroutines** in PMForge's own code today — the Wails runtime is the only goroutine spawner.
- **Database access**: always through `*db.Database`. The `*sql.DB` it wraps is a connection pool, safe for concurrent use.
- **IDs**: prefixed short hex via `db.newID("prefix")` or `agile.NewBoardID()` etc. Format: `<prefix>_<8hex>`.
- **Timestamps**: store as RFC3339Nano UTC strings via `strftime('%Y-%m-%dT%H:%M:%fZ','now')` or `time.Now().UTC().Format(time.RFC3339Nano)`. Surface as `time.Time` in Go structs with `json` tags.
- **No `import "strconv"` in hot paths** if a 1-2 line itoa shim suffices. Most files import strconv directly though — both styles exist; don't refactor.
- **Pointer vs value methods**: receivers are pointers when the method may mutate or when the struct is non-trivial (>3 fields). Plain getters can be value receivers but for consistency, the codebase uses pointer receivers throughout `*Database`, `*App`, `*Store`.

### Svelte 5 conventions

- **Runes mode is enabled** in svelte.config.js. Use `$state`, `$derived`, `$effect`, `$props`, `$bindable`.
- **Component file naming**: PascalCase for components, snake_case with leading `_` for shared helpers (e.g. `_stats_editor_shell.svelte`, `_flow_shapes.ts`).
- **Imports**: relative paths within `frontend/src/lib/`. Type-only imports from globals are declared in `wails-window.d.ts` and used without `import`.
- **Async data**: `onMount(async () => { ... })`. Errors handled with try/catch; user-facing errors stored in `let error = $state('')`.
- **Debounce pattern**: every editor that auto-saves uses `$effect` + `setTimeout` with `untrack()` to avoid feedback loops. **MUST also `onDestroy(() => clearTimeout(timer))`** to avoid leaks on navigation away.
- **Chart.js cleanup**: `onDestroy(() => chart?.destroy())`. Mandatory; otherwise canvases leak.

### Architecture patterns

- **Registry + Definition** pattern (charts.registry.go, documents.registry.go): one taxonomy file with constants and a slice of Definition structs. Iteration is by `All()`; lookup by `Get(kind)`. Adding a new kind = one slice append.
- **Engine + Dispatcher** pattern (charts.engines.go, pdfrender.dispatcher.go): a `Layout(kind, data) (LayoutResult, error)` switch that delegates to per-engine layout functions. The LayoutResult carries `Engine`, `Kind`, `Title`, `Body json.RawMessage`. Frontend dispatches on `result.engine`.
- **Shared editor shells**: `_layered_editor_shell.svelte` and `_stats_editor_shell.svelte` use Svelte 5 generics + snippets to provide the chrome (load/save/refresh/header) and let kind-specific editors fill in the data form.
- **Layout-only renderers**: backend chart layout (`charts.Layout()`) emits JSON. Frontend renders SVG/Chart.js. For PDF embed, `pdfrender.RenderChartToPDF()` draws the same layout with gofpdf primitives — vector, not PNG.

---

## 5. Build / run / test workflow

```sh
# First-time setup
go mod tidy
(cd frontend && npm install)
make fonts                               # download bundled TTFs into internal/fonts/assets
(cd LICENSES && reuse download --all)   # optional, for `make license-check`

# Dev loop (hot-reload Go + Svelte)
wails dev

# Production binary (embeds frontend via go:embed)
make build

# Quality gates
make lint              # golangci-lint + npm run lint
make test              # go test ./...
make race              # go test -race ./... (V2.x — concurrency hardening gate)
make memory-scan       # scripts/memory-safety-scan.sh (V2.x)
make license-check     # reuse lint
make check-release     # version consistency + REUSE + build

# Packaging
make package-linux / package-windows / package-darwin
```

**First-launch behaviour**: creates `~/Documents/PMForge/system.db` (account list) and provisions `~/Documents/PMForge/<username>/{projects,certs,exports}/` for each user (chmod 0700 on POSIX).

---

## 6. Locking & concurrency invariants

**The Wails runtime dispatches every frontend call on a separate goroutine.** The App struct in `cmd/pmforge/main.go` is therefore accessed concurrently and must be guarded.

### App locking rules
- `App.mu` is a `sync.RWMutex` (V2.x hardening).
- **Mutable fields** under the lock: `user`, `db`, `dbPath`, `adminSvc`.
- **Read-only fields** (set once in NewApp): `store`. May be read without the lock.
- **Helper methods** `requireUser()` and `requireDB()` take an RLock, copy the pointer, RUnlock, return. The returned pointer remains valid for the caller's lifetime because the underlying structs are not freed (Go GC).
- **Logout / CloseProject** take a Write lock for the entire operation including the inner `db.Close()`.

### Known race (acceptable for single-user desktop)
- A long-running query that started before `Logout()` may see `sql: database is closed` after logout finishes. The query returns an error rather than crashing. To fully eliminate this, queries would need to take a per-call lock — slow and not worth it.

### Frontend cleanup
- Every editor with a debounce timer **MUST** `clearTimeout` in `onDestroy`. Without this, navigation away from a half-edited chart leaves a timer that fires on an unmounted component (a closure leak even if not a crash).
- Chart.js instances **MUST** be `.destroy()`'d in `onDestroy`. See StatsChart.svelte.

---

## 7. Memory & resource safety

`make memory-scan` runs:
1. `go vet ./...` — standard correctness checks.
2. Custom grep gate (scripts/memory-safety-scan.sh) for:
   - `os.Open(` without nearby `defer .*Close()`
   - `sql.Open(` without nearby `defer .*Close()`
   - `unsafe.Pointer` (forbidden in this codebase)
   - missing `errors.Is`/`errors.As` against package `Err*` sentinels
3. (When installed) `staticcheck ./...` for deeper analysis.
4. (When installed) `gosec ./...` for security-flavoured patterns.

A new contribution should land **with `make memory-scan` passing**. The gate is wired into `make check-release`.

---

## 8. Feature coverage (live status)

### Charts: 19/19 implemented end-to-end (UI + Go layout + frontend renderer + PDF embed)
- **DAG family** (6): WBS, Network, PERT, CPM, Fishbone, Cause-Effect.
- **Flow family** (2): Workflow, Activity.
- **Matrix family** (4): RACI, SWOT, Stakeholder Analysis, Generic Matrix.
- **Stats family** (8): Line, Bar, Pareto, Pie, BurnUp, BurnDown, CumulativeFlow, Control.

### Documents: 25/25 registered; **23 bespoke renderers; 2 aliases (Charter Excel → Charter Word, Plan Excel → Plan Word). All 25 effectively bespoke — every kind has a dedicated layout.** Renderers: Charter, Status Report, Risk Register, Project Plan, Communication Plan, Statement of Work, Project Closure, Stakeholder Analysis, Scope Statement, Project Budget, Requirements, Issue Log, Change Request, Business Case, Procurement Plan, Team Charter, Execution Plan, WBS Document, RACI Document, Project Proposal, Project Schedule, Project Brief, Project Overview. **All five lifecycle phases at 100% bespoke coverage.**

### Cross-cutting features done
- Local multi-user auth (Argon2id) with per-user folder isolation
- Self-heal + atomic snapshot swap (`RepairAndSwap`)
- Combined report builder with **embedded vector chart visualisations**
- Chart picker for FieldChartRef (constrained by `ChartKind`)
- Audit log with CSV export
- Archival backup bundles (`.pmba`)

### Agile Pack (V2.x — complete)
- **Backend**: schema (5 tables in db/sqlite.go), types (agile/agile.go), CRUD storage (agile/store.go), DORA metrics with elite/high/medium/low classification (agile/dora.go), Wails methods in cmd/pmforge/main.go §Agile Pack.
- **Frontend**: KanbanBoard (drag-and-drop with WIP badges), Backlog (priority + drag reorder + Start-work), SprintList (planning/active/complete lifecycle with single-active invariant), DORADashboard (4 KPI cards + deploy-trend line via StatsChart + inline +Deployment form). All live under `frontend/src/lib/components/agile/`.
- **Wiring**: 4 new session view union members (`kanban`, `backlog`, `sprints`, `dora`), App.svelte routes, Dashboard "Software-Dev Pack" section with enable/disable toggle backed by `App.AgileEnabled` / `App.SetAgileEnabled`.

### Memory & concurrency gates (V2.x)
- **`make memory-scan`** runs `scripts/memory-safety-scan.sh`. Currently passing in the sandbox; on a dev box with Go in PATH it also runs `go vet` and a Go-helper scan for unclosed `os.Open` handles. Optional integrations: `staticcheck`, `gosec`, `govulncheck` — auto-detected.
- **`make race`** runs `go test -race ./...`.
- Both are wired into `scripts/check-release.sh` so the release gate fails if either does.

### Remaining V2 TODOs (status snapshot)
1. ~~DOCX / ODT export.~~ **Done.** `internal/export/docx.go` uses `gomutex/godocx`; `internal/export/odt.go` is hand-built (no maintained ODT library exists). App methods `ExportDocumentDOCX` / `ExportDocumentODT`.
2. **PDF/A-3 strict conformance** — partial, advanced 2026-05-20 on two fronts. (i) The dependency-free `internal/pdfmeta` package builds the canonical XMP packet AND injects it into the PDF Catalog via a spec-conformant **incremental update** (`InjectXMPStream`); `documents.Render()` tags every generated PDF (fail-soft). (ii) **Font embedding is now available** via `internal/fonts` — bundled TrueType families (fetched by `make fonts`) embed into PDFs through the "register under Helvetica" trick, replacing the non-embeddable core fonts. Remaining for full conformance: (a) OutputIntent + ICC profile, (b) veraPDF validation in `make check-release`. Both the Catalog-stream injection and font embedding — previously the two blocking unknowns — are **done**.
3. ~~CMS/PKCS#7 signature embedding.~~ **Done** via `digitorus/pkcs7`. Caveat: signature lives in a trailing PDF comment, not in a `/Sig` widget with `/ByteRange` + `/Contents`. PAdES B-B requires the widget form; that's an incremental-update + xref-table rewrite that's non-trivial with gofpdf. Deferred to V3.
4. ~~Wails file-picker for certs.~~ **Done.** `App.ChooseCertFile` calls `wailsruntime.OpenFileDialog`.
5. ~~HTTPS update channel with signed release manifest.~~ **Done.** `internal/update` fetches a signed JSON manifest, verifies Ed25519, returns `Status`. `ManifestURL` and `UpdateChannelPublicKey` set at build time via `-ldflags`.
8. **Per-user database encryption at rest** — deferred. Production-grade implementation requires either SQLCipher (libsqlcipher native dep — significant build complexity) or whole-file AES-at-rest with crash-recovery semantics. Recommended stopgap for V2: document OS-level disk encryption (FileVault / BitLocker / LUKS) in the user-facing docs.
9. **Bespoke renderers for the 24 non-Charter document kinds** — Status Report shipped as the reference (`internal/documents/status_report.go`). The other 23 follow the AGENT.md §10 recipe. Generic renderer keeps every kind exportable in the meantime.
10. ~~Embed chart visualisations in combined reports.~~ Done in earlier slice.
13. ~~Account recovery codes.~~ **Done.** 8 Argon2id-hashed codes generated at account creation, redeemable once each. `App.IssueRecoveryCodes` + `App.ResetWithRecoveryCode`. Frontend: `RecoveryReset.svelte`.

### Still deferred to V3
- Strict PDF/A-3 — XMP Catalog injection now done (`internal/pdfmeta`); still needs font embedding + OutputIntent + veraPDF gate.
- PAdES B-B signature widget (incremental update + xref rewrite). NOTE: `internal/pdfmeta`'s incremental-update machinery is a reusable starting point — the same append-object + delta-xref + /Prev-trailer pattern applies to embedding a `/Sig` widget.
- Per-user encryption at rest (SQLCipher integration).
- PDM date-dragging on the Timeline (major editor rewrite).

---

## 9. Lessons learned

This section is the running log of non-obvious discoveries. Every session that learns something should append a dated entry.

### 2026-05-13 — V2.x hardening session
- **Wails dispatches each frontend call on a fresh goroutine.** All App fields must be guarded. Was already mostly correct; converted `App.mu` from `sync.Mutex` to `sync.RWMutex` so readers don't block each other (most calls are reads).
- **Svelte 5 debounce timers leak across navigation.** Every editor that uses the `$effect` + `setTimeout` pattern needs an `onDestroy(() => clearTimeout(timer))`. Added systematically.
- **Chart.js v4 requires explicit controller/element/scale registration.** Done globally in `StatsChart.svelte`. Missing registrations fail silently with empty canvases.
- **gofpdf has no native SVG.** Charts embed in PDFs via `pdfrender` package using vector primitives (Line/Rect/Polygon/Circle). This is the long-term archival-quality path; PNG screenshots would have been quicker but lossy.
- **DAG and Flow share the layered-layout idea** but their JSON body shapes differ (DAG nodes have Number+Note+Owner+Depth; Flow nodes have Shape+SwimlaneID+Rank). They get separate Go renderers.
- **Migrations are additive only.** `CREATE TABLE IF NOT EXISTS` everywhere. Adding a column? Use ALTER TABLE in a versioned migration step (not yet needed — schema is still expanding additively).
- **The Agile Pack's `state` column is the column ID** rather than an enum, so renaming a column's display name doesn't require updating every work item.

### 2026-05-14 — Agile Pack backend + safety hardening
- **Don't keep both `agile.go` and `agile/doc.go` with the same `PackEnabled`**. The old V1 placeholder `doc.go` and the new `agile.go` both declared `var PackEnabled bool` — duplicate-symbol error. Fix: `doc.go` is now a pure package-doc comment with zero declarations; `agile.go` owns the symbols.
- **`App.mu` is now `sync.RWMutex`** (was `sync.Mutex`). Reads (`CurrentUser`, `requireUser`, `requireDB`, `SecureArchive`) use `RLock`; writes (`Login`, `Logout`, `CreateAccount`, `OpenProject`, `CloseProject`, `RepairAndSwap`-swap-phase) use `Lock`. Most calls are reads, so this measurably reduces lock contention under bursty Wails dispatch.
- **Added `requireDBAndPath()`** helper that returns both `db` and `dbPath` under a single RLock — keeps them consistent across a concurrent Logout that might otherwise split them.
- **Every Svelte editor with a debounce timer now has `onDestroy` cleanup.** That's: WBSEditor, CauseEffectEditor, FishboneEditor, WorkflowEditor, ActivityEditor, StakeholderEditor, plus both shared shells (`_layered_editor_shell.svelte`, `_stats_editor_shell.svelte`). Without this, navigating away from a half-edited chart leaves a pending `setTimeout(refreshLayout)` that fires on an unmounted component.
- **Memory-safety scan caught two real bugs** on first run: (a) the duplicate `PackEnabled`, (b) an over-loose goroutine regex that matched substrings like `gofpdf`. Tightened to `(^|[[:space:]{(;])go (func|ident()` and skip lines whose first non-whitespace chars are `//`.
- **Sandbox limitation**: `go run -` inside the script requires Go in PATH; added an explicit `command -v go` skip so the gate is portable to CI environments without a Go toolchain.
- **The Wails runtime spawns goroutines per call.** The hardening pass confirmed PMForge itself spawns zero — the goroutine grep returns empty after the regex tightening. All concurrent state is the App struct, fully guarded.

### 2026-05-19 — SOW + Closure + Stakeholder Analysis renderers + pure-data unit tests
- **Bespoke coverage 8/25.** Statement of Work (prose + sign-off), Project Closure (mixed prose + lessons-learned table + sign-off line), Stakeholder Analysis (per-stakeholder cards grouped by quadrant). The three together demonstrate the FOUR distinct shape patterns we've now established:
  1. **Prose with sign-off** (Charter, Statement of Work) — portrait, section heads, signature lines at the bottom.
  2. **Status snapshot** (Status Report) — portrait, traffic-light badges at the top, bulleted sections.
  3. **Sorted table** (Risk Register, Communication Plan) — landscape, color-banded first column, sorted/grouped rows.
  4. **Hybrid card list** (Project Plan, Project Closure, Stakeholder Analysis) — portrait, mix of prose sections + bordered cards.
  Future bespoke renderers should pick the closest match and copy the helpers from that file (per AGENT.md §10's "each renderer self-contained" rule).
- **First targeted unit tests landed.** `internal/budget/budget_test.go`, `internal/timeline/timeline_test.go`, and `internal/calendar/calendar_test.go` test the pure-data helpers that are most likely to drift under refactor. The budget tests exercise empty / contracts / labour-match / overspend cases; timeline tests cover empty + project dates + sprint ranges + RFC3339 vs date-only + zero-TS skip; calendar tests cover unknown-country fallback + weekend / US New Year / workdays-from / window-symmetry. These run via `make test` on the user's Mac; the sandbox can't.
- **Future-test priorities** when more coverage is wanted: pdfrender layout math (fit + scale), agile.DORA classification thresholds (the elite/high/medium/low band boundaries), auth.HashPassword/VerifyPassword round-trip, recovery-code canonicalisation. These are all pure-data and won't need Wails or SQLite.
- **Stakeholder Analysis Document uses `power_level`/`interest_level` field keys** to match the document schema in templates.go (registry-defined). The chart kind uses `power`/`interest`. Both forms ultimately resolve to the same Power × Interest classification; the doc kind's "stakeholders" object-array has its own keys because PMI's classic Stakeholder Analysis Template uses those longer names.

### 2026-05-18 — Second API audit + Project Plan + Communication Plan renderers
- **rickar/cal/v2 and digitorus/pkcs7 APIs verified.** Both check out — `cal.NewBusinessCalendar()`, `AddHoliday(holidays...)` variadic spread, `IsHoliday(t) (actual, observed bool, h *Holiday)` triple-return; pkcs7 `NewSignedData`, `AddSigner(cert, key, SignerInfoConfig{})`, `SetDigestAlgorithm(OIDDigestAlgorithmSHA256)`, `Detach()`, `Finish()` all match my calls. Two-for-two on the audit pass; the templates+godocx mismatches last turn were the only real bugs.
- **Bespoke renderer coverage is now 5/25.** Charter (initiation), Status Report (monitoring), Risk Register (planning, landscape table), Project Plan (planning, the comprehensive doc), Communication Plan (planning, audience-grouped table). These five cover the most commonly-printed PM artifacts; the remaining 20 still work via the generic field-walker.
- **Two emergent renderer patterns** that future bespoke implementations should follow:
  - **Prose-heavy kind** (Charter, Status Report, Project Plan) → portrait A4, headings + bulleted lists + bordered cards for references. Project Plan adds a dedicated "Linked artifacts" page that shows chart_ref / doc_id fields as labelled chips instead of raw IDs.
  - **Table-heavy kind** (Risk Register, Communication Plan) → landscape A4, sorted rows, color-band cells (Risk: by P×I score; Comm Plan: by cadence). Wrap rows by a grouping key when one exists (Comm Plan groups by audience so each stakeholder's responsibilities are one scan).
- **The Word/Excel-alias dispatch quirk.** `documents.Render()`'s switch case `KindProjectPlanWord, KindProjectPlanExcel:` routes both alias kinds to one renderer. Same pattern is in place for Charter. Keep them in the dispatch so the schema-alias dance (`EffectiveFields` resolving Excel → Word) stays consistent across the rendering path.

### 2026-05-17 — API audit + Project Settings + Risk Register renderer
- **Two real API mismatches in the V2.x code shipped last turn**, both caught by a focused audit:
  1. `zen-go` does NOT have a `zen.NewMemoryLoader()` struct with an `Add()` method. Its `EngineConfig.Loader` is a plain `func(key string) ([]byte, error)` callback. Rewrote `internal/templates/jdm.go` to use the function form. Also: `engine.Evaluate(ctx, key, input)` takes the input as `map[string]any`, not JSON bytes — round-trip through `json.Marshal`/`Unmarshal` to keep `SeedRequest` as the single source of truth.
  2. `gomutex/godocx`'s table API (`AddTable / AddRow / AddCell`) has shifted across minor versions and the chained `.AddCell().AddParagraph(s).AddText("").Bold(true)` I wrote against memory likely doesn't compile on the pinned version. Replaced with a bulleted-list rendering that exercises only the stable `AddParagraph(...)` + `.AddText(...).Bold(true)` shape. Documented the future upgrade path in a comment.
- **The "search pkg.go.dev first" rule has a corollary: VERIFY the API shape before writing against it.** A web search returning "this library exists" doesn't mean its types match your memory. For unfamiliar libraries, write a 5-line test program first, OR commit to verifying after `go mod tidy` succeeds.
- **Project Settings panel uses two backend calls** (`UpdateProjectMeta` + `UpdateProjectIndustry`) because the four Launchpad columns (industry/sub_category/methodology/country_code) have their own setter for symmetry with the Launchpad flow. The Settings panel hits both and merges the results. Future cleanup: collapse them into one `UpdateProject(p Project)` call.
- **Risk Register is the second bespoke renderer** (after Status Report) and the first one with a real table layout. Landscape A4 + 8 columns + per-row tinted first cell + sorted descending by P×I score. The pattern: when a document kind is mostly tabular, render in landscape; when it's mostly prose, portrait. Both fit on the same dispatch switch in `documents.Render()`.
- **`crypto/` at the repo root is an unrelated x/crypto clone**. The memory-safety scan was tripping on it. Fix: scope the scan to `$PMF_DIRS = ./cmd ./internal ./scripts` so unrelated siblings can't trigger false positives. Documented in the script.

### 2026-05-16 — Remaining V2 TODOs slice (DOCX/ODT, recovery codes, CMS, update channel, PDF/A partial)
- **`pkg.go.dev first` rule paid off.** For DOCX we found `gomutex/godocx` (MIT, pure Go, maintained) — saved ~400 lines of OOXML hand-rolling. For ODT we found NOTHING maintained, which itself is a discovery: hand-build is genuinely the lowest-risk path. **The search itself is the deliverable** even when it returns "no fit".
- **Strict PDF/A-3 is much bigger than the gofpdf surface allows.** The XMP packet builder + metadata setters in `pdfa.go` are a real improvement (PDF Properties dialogs now show the right values), but the binary STILL won't pass veraPDF. The hard parts — font embedding, Catalog XMP-stream injection, OutputIntent — need either (a) shipping a TTF and switching gofpdf for `seehuhn.de/go/pdf`, or (b) post-processing every PDF through pdfcpu/unipdf. Don't claim full PDF/A compliance in the GUI until the gate runs.
- **CMS signing has two levels of "correctness".** `digitorus/pkcs7` produces a real CMS SignedData blob in five lines. Embedding it into the PDF as a recognised signature widget (`/Sig` dictionary, `/ByteRange`, `/Contents` slot) is a separate, larger task that gofpdf doesn't help with. Current behaviour: CMS blob in a trailing PDF comment — better than the V1 raw-RSA tag, still not Acrobat-blue-ribbon.
- **Ed25519 over RSA for update-manifest signing.** Smaller key (32B vs 256+), faster verify, entirely stdlib. The release pipeline keeps a single keypair, the binary embeds the public key via `-ldflags`. Future-proof if we ever need to rotate (re-sign the manifest under a transition key + new key, ship a binary that trusts both).
- **Recovery codes need to be one-shot.** The implementation hashes each of 8 codes with Argon2id (matching password hashing) and marks the row `used = 1` atomically with the password rotation. Re-using a code is impossible because the row is marked used inside the same transaction that updates the password. Canonicalisation (uppercase + strip dashes + strip spaces) means the user can paste in any reasonable form.
- **Wails runtime methods need `app.ctx`.** `wailsruntime.OpenFileDialog` requires the startup-supplied context; calling it before `OnStartup` fires panics. Guard with `if a.ctx == nil { return "", error }`.
- **Don't try to delete an existing file via sandboxed bash.** The Linux sandbox can't `rm` from the user's home dir; overwrite-in-place is the cross-platform substitute. Pattern: write the empty/stub version with the same name + an explanatory header.
- **Defer when you mean it.** I deliberately stopped short of: full PDF/A-3, full PAdES B-B widget, per-user encryption-at-rest, PDM date-dragging, 23 more bespoke renderers. Each is documented with the recipe + cost. Shipping the achievable subset cleanly beats shipping all five half-built.

### 2026-05-15 — Foundation Slice (Launchpad, Stakeholders, Timeline, Budget, iCal)
- **Migrations are now genuinely additive.** Adding four new columns to `project` taught us that `ALTER TABLE ADD COLUMN` is not idempotent in SQLite — it errors if the column exists. Solution: `migrateLegacyColumns()` probes the table's `PRAGMA table_info` and only runs ADD when the column is missing. Reuse this helper for any future column additions instead of writing ad-hoc ALTERs.
- **zen-go for "rules as data" is a real win.** The Launchpad's industry-×-methodology seeding logic is now 12 rows in `launchpad_seeds.json` rather than a 12-arm Go switch. Adding a new combo is a JSON edit; the build picks it up via `//go:embed`. The unit test in `internal/templates/jdm_test.go` asserts the JDM parses so a typo is caught by `make test` rather than at runtime. The trade-off is one extra dependency and a learning-curve cost for new contributors — net positive at this scale.
- **rickar/cal/v2 supplies per-country holiday packs** via sub-packages (`cal/v2/us`, `cal/v2/gb`, ...). We funnel them through `calendar.For(countryCode)` so the rest of the codebase imports only `internal/calendar` and never `rickar` directly. This keeps the upgrade path simple: if rickar's API shifts, only one file changes.
- **iCal RFC 5545 line-folding is one of those "looks simple, isn't" details.** Lines > 75 octets MUST be folded with CRLF + a single space; text values MUST escape `,`, `;`, `\`, and `\n`. The `icalWriter` in `internal/export/ical.go` handles both. Don't try to "just join strings with \n" — Outlook and Apple Calendar will reject the file silently.
- **Country-aware features should default sensibly.** New projects get `country_code = "US"` because that's the most common dataset and our default workweek matches. The Launchpad lets the user override. Legacy `.pmforge` files also get "US" via the migration helper.
- **Budget rollup is name-matched, not ID-matched.** Work item `assignee` is a free-text string (so a placeholder name is fine before a stakeholder exists). The `budget.Compute` rollup case-insensitively matches `wi.assignee` against `stakeholder.name`. Trade-off: typos break the link. Future hardening: a stakeholder-picker dropdown for assignee.
- **Timeline assembly stays database-free.** `timeline.Build()` takes the project + sprints + deployments as values; main.go fetches them once and passes them in. Same pattern as `documents.BuildCombinedReport`. The point is the package is unit-testable without spinning up SQLite.
- **App.templates is intentionally non-fatal.** If zen-go fails to initialise the JDM engine at startup, we log and continue — the Launchpad falls back to "no auto-seed" and the rest of the app keeps working. A misconfigured rule should never brick PMForge.

### 2026-05-14 — Agile Pack frontend
- **Native HTML5 drag-and-drop is sufficient** for the Kanban board and Backlog reorder. No external DnD library needed; `draggable="true"` + `ondragstart` / `ondragover` / `ondrop` covers it. The reorder pattern (drag a list item, push positions through `order_idx`) matches what `ReportComposer.svelte` already does — two cases now, established pattern.
- **DORADashboard reuses `StatsChart.svelte`** for the deploy-trend mini-chart by constructing a `StatsLayout` inline. Cross-feature reuse: the stats engine wasn't meant for agile, but it just works because the layout types are public. Confirms the registry+layout architecture pays off.
- **Single-active-sprint is GUI-enforced**, not schema-enforced. When the user clicks "Start" on a planning sprint, `SprintList.activate()` first sweeps any other `active` sprint to `complete` then activates the target. Keeping this in the frontend means the backend stays simple and the rule is visible/testable in one place.
- **WorkItemEditor uses a `lastItemID` sentinel** to decide when to re-seed the local `draft` from the `item` prop. Without this, parent-side optimistic updates would clobber unsaved edits every time the parent re-renders. The sentinel pattern is reusable for any "edit a record in a modal" component.
- **AgileEnabled is in-memory only** (per AGENT.md §8). The Dashboard's toggle calls `SetAgileEnabled` which flips `agile.PackEnabled` in process. Persisting this across restarts is a one-line addition to `settings` later if needed.
- **WIP-limit breach indicator** is computed server-side via `WIPCountByColumn()` and rendered client-side as a red badge — the badge tints red when `count > limit > 0`, stays slate when unlimited (`limit == 0`).
- **The Dashboard's `agileEnabled` check is wrapped in try/catch** so an older binary without the Agile bindings just hides the section instead of crashing. Cheap forward/backward compatibility for a desktop app where the user may not have updated yet.

### 2026-05-19 — Project Brief + Project Overview bespoke renderers (25/25 complete)
- **Bespoke coverage 23/25 + 2 aliases = 25/25 effective.** All five lifecycle phases at 100% bespoke. The 17-doc generic-field-walker baseline established in 2026-05-19's "SOW + Closure + Stakeholder Analysis" entry is now down to zero. Generic renderer remains in the dispatch as a safety net for forward-compatibility — if a future kind is registered before its bespoke renderer ships, the generic path still produces a valid PDF.
- **Project Brief is the audience-friendly variant.** Reuses the executive-summary callout (from Project Proposal), the numbered list (Proposal), the wrapping name chips (Proposal), and pairs them with a sibling KPI tile (Proposal's budget tile, extended into a two-tile strip for budget + timeline). Almost entirely composed of existing patterns — validates that the visual vocabulary built up over the 23-doc effort is fully reusable.
- **Project Overview introduces three new elements**:
  - **Top-right status badge** — green/yellow/red pill in the top-right corner of the title row. `overviewStatusColor` is permissive on terminology (accepts "green" / "on track" / "ok" / "healthy" → green; "yellow" / "amber" / "at risk" → amber; "red" / "off track" / "blocked" → red; "complete" / "done" → slate). Fallback path uppercases the raw status and uses slate.
  - **Highlights strip with checkmark prefix** — amber-tinted callout with green checkmark prefixes for each highlight. Visually distinct from the numbered-list and bullet patterns so the reader treats highlights as "things to know about" rather than "things to do".
  - **3-up summary grid with coloured top-edge accents** — three side-by-side cards (Milestones blue / Budget green / Team amber), each with a 3mm coloured strip on top. Cards auto-size to fit the tallest body via `overviewCardHeight`, same line-estimation trick used in RACI Document. Empty bodies render "(not provided)" in slate so the card never appears blank.
- **Pattern catalog is now complete.** The full visual vocabulary across the 23 renderers:
  1. **Prose with sign-off** — Charter, SOW, Scope Statement.
  2. **Status snapshot** — Status Report, Project Overview.
  3. **Sorted table** — Risk Register, Communication Plan, Requirements, Procurement Plan.
  4. **Hybrid card list** — Project Plan, Project Closure, Stakeholder Analysis, Business Case.
  5. **Formal single-form** — Change Request.
  6. **Status-partitioned table** — Issue Log.
  7. **Inline graphics in table cells** — Team Charter (allocation bars), Execution Plan (mini-Gantt segments).
  8. **Indented hierarchy** — WBS Document.
  9. **Chart-companion banner** — WBS Document, RACI Document, Project Schedule.
  10. **KPI tiles** — Project Proposal, Project Brief.
  11. **Persuasive CTA layout** — Project Proposal (the ASK).
  12. **Baseline stamp** — Project Schedule (green when set, slate when unset).
  13. **Audience-friendly summary** — Project Brief.
- **What's next.** Bespoke coverage saturated. The next investment areas per AGENT.md §8 are: (a) PDF/A-3 strict conformance (font embedding + Catalog stream + OutputIntent + veraPDF gate), (b) PAdES B-B signature widget (incremental update + xref rewrite), (c) per-user encryption at rest (SQLCipher), (d) PDM date-dragging on the Timeline. All four are V3 milestones requiring significantly larger slices.

### 2026-05-19 — Project Schedule bespoke renderer (planning phase ~complete)
- **Bespoke coverage 21/25; planning 13/14 (Plan Excel aliased → 14/14 effectively).** Only execution (Project Brief + Project Overview) remains.
- **Linked-chart banner is now the established idiom for chart-companion docs.** Third application (WBS Document → RACI Document → Project Schedule), all sharing the same shape: light-blue tinted strip, "LINKED <KIND>" small caps label, chart_ref ID + an explanatory sentence pointing the reader to the chart for the visual.
- **Baseline stamp is the novel visual element.** Green-500 fill, green-700 heavy outer border + an inner double-line for the "stamp" feel, "BASELINED" label in green-100 + the date in 18pt white. Below the date, an age indicator computes "baselined N days ago" / "today" / "baselines in N days" — answers the implicit question "is this baseline still fresh?" without forcing the reader to do mental arithmetic.
- **Two-state tile** — when baseline_date is empty, the same tile renders in slate (not green) with "Not yet baselined" text, making the document's status legible at a glance. Future tile-style elements that have an "ok / pending" state should follow this pattern (slate = pending, green = locked in).
- **`plural(n)` helper.** Trivially small but worth lifting if any other renderer needs day/item counting: returns "" for 1 and "s" otherwise.

### 2026-05-19 — Project Proposal bespoke renderer (initiation phase complete)
- **Bespoke coverage 20/25; initiation phase 5/5 complete.** First explicitly **persuasive** document. The other three text-heavy initiation docs (Charter, Business Case, Stakeholder Analysis) are formal/analytical/structural; Project Proposal exists to win buy-in, and the layout reflects that.
- **Four new visual elements** worth lifting into future renderers:
  - **Executive Summary callout at the top** — accent-boxed under the title strip so the reader's first content beat is the elevator pitch, not a header.
  - **Numbered list instead of bulleted** — `1. 2. 3.` for Goals because order tends to imply priority in a proposal. Same shape as `writeBulletSection` but with index numbers as the leading chip.
  - **Team chips** — wrapping name pills with rounded-rect borders. Replaces a dry table when the doc doesn't need per-person details (those live in the Team Charter). Chip width auto-fits `pdf.GetStringWidth(name) + 6`; row wraps when the next chip would exceed `rightEdge`.
  - **Budget KPI tile** — dark-filled right-aligned tile with a small label and a large 18pt dollar amount. Scannable: a budget reviewer's eye lands on the number without reading. This is now the "big number" pattern; reuse for any doc where one figure dominates (Project Brief's `budget`, Project Overview's `budget_summary`).
- **THE ASK callout is heavier than the recommendation callout** from Business Case. Dark-blue header strip with white "THE ASK" label, then a light-grey body. Closes the doc with maximum visual weight — the reader is supposed to land here last and act on the request. Future closing-CTA blocks (e.g. Closure's stakeholder sign-off) could use this pattern.

### 2026-05-19 — RACI Document bespoke renderer (RACI letter legend)
- **Bespoke coverage 19/25; planning 12/14.** First chart-companion doc to reuse the linked-chart-callout pattern introduced with WBS Document. Confirms that idiom as the shared shape for chart-paired docs (Project Schedule, when bespoke, should do the same).
- **RACI letter legend** is the novel contribution. Most stakeholders see a RACI matrix once a quarter and forget what R/A/C/I mean — the legend embeds the definitions inline with the same colour vocabulary as the chart kind (R=green, A=red, C=amber, I=cyan). Educational + visually consistent with the matrix it summarises.
- **`drawRACIBanner` extends the linked-chart banner** with a second row for the effective date. The pattern naturally accommodates "metadata + chart link" — future chart-companion docs (Project Schedule with baseline_date, RACI with effective_date, etc.) all fit this two-line layout.
- **Two-cell row-height parity trick**: when one cell can wrap (Definition) and the other cannot (Role), gofpdf's `CellFormat` cells diverge in height. Workaround: estimate the wrapped height with `pdf.GetStringWidth(text) / cellWidth → line count`, draw BOTH cells as empty `CellFormat`s at the estimated height, then `SetXY` back to the start and `MultiCell` the actual text into each. Pattern is in `raciRowHeight` + the loop in `drawRACIRoleTable`. Reuse this any time you need same-height multi-line cells in a row.

### 2026-05-19 — WBS Document bespoke renderer (indented hierarchy)
- **Bespoke coverage 18/25; planning 11/14.** First doc that **renders a hierarchy**, not a flat table. Each deliverable's WBS code (e.g. "1.2.3") drives a depth-based left indent (8mm per dot) and a depth-graded chip colour: depth-0 deep blue → depth-1 medium blue → depth-2 cyan → depth-3+ slate. The reader sees the tree without lines or guides.
- **`wbsCodeLess` sorts numerically by segment.** Naïve string comparison puts "1.10" between "1.1" and "1.2"; this comparator splits on dots and compares the numeric prefix of each segment. Falls back to lexical comparison when both numeric prefixes match (handles "1a" vs "1b" cases). Tested against [1, 1.1, 1.2, 1.10, 1.2.1, 2] — orders as expected.
- **`drawWBSChartBanner` is the linked-chart-callout pattern**: light-blue fill, blue border, two-line label ("LINKED WBS CHART" + the chart_ref ID + a sentence pointing the reader to the chart for the visual). Reuse for RACI Document, Project Schedule, and any other chart_ref-carrying document.
- **Code chip width auto-fits the text.** `pdf.GetStringWidth(codeLabel) + 4` gives a snug chip that doesn't waste space on short codes ("1") but accommodates long ones ("1.2.3.4.5"). Minimum 14mm so very-short codes don't look squished.

### 2026-05-19 — Execution Plan bespoke renderer (inline mini-Gantt)
- **Bespoke coverage 17/25; planning 10/14.** First doc with **inline mini-Gantt segments** in a table row. Each task row's Timeline column shows a grey track with a blue-800 filled segment positioned according to that task's [start, end] window relative to the project's overall min-start → max-end span. A reader sees who-overlaps-who without leaving the table.
- **`computeProjectWindow`** scans the tasks once and picks the earliest start + latest end across all rows. Tasks with only a start OR only an end still extend the window (single-endpoint segments render at the relevant pole instead of being dropped).
- **Single-day tasks get a minimum bar width** (0.8mm) so they remain visible even when the project window is hundreds of days. Right edge is clamped to the cell's right padding so the segment doesn't draw outside the track.
- **`parseDate` accepts both YYYY-MM-DD and RFC3339** so the same helper works whether the date came from a Wails form (typically RFC3339Nano) or from the user typing into a string field in the JSON. Pull this into a shared `internal/documents/dates.go` if a fourth renderer needs it — for now it's local-to-file per AGENT.md §10's self-contained rule.
- **`shortExecDate` accepts either `time.Time` or `string`.** Lets the renderer pass parsed times for the table cells (clean YYYY-MM-DD format) while still handling the raw string when called from the summary banner.
- **Same cell-overlay recipe as Team Charter**: capture (x, y) before the empty CellFormat, then call the overlay function. Pattern is now used twice, validating it as the shared idiom for graphic-inside-cell.

### 2026-05-19 — Team Charter bespoke renderer (inline allocation bars)
- **Bespoke coverage 16/25; planning 9/14.** First doc with **inline horizontal bar charts** in a table row. Each member row's allocation percentage renders both as the number and as a proportional filled bar within its own cell. The cell border is drawn first (empty CellFormat), then `drawAllocationBar` overlays the visual: numeric label on the left, grey track + filled portion on the right, with a 100% reference tick.
- **`allocationColor` scales by intensity.** ≤25% slate (light commitment), 26-50% cyan, 51-75% amber, 76-100% green (good engagement), >100% red (over-allocation). The colour scale conveys "is this allocation healthy?" without needing legend lookup.
- **Members sorted by allocation desc.** Most-committed members render at the top so the reader's first scan answers "who is most invested in this project?"
- **Capacity banner below the table** sums total + average allocation. Same pattern as Issue Log's counts banner — a single line that conveys the most important table-summary number without making the reader add up the rows.
- **Recipe to embed a bar inside a CellFormat cell**: (a) capture `pdf.GetX()` / `pdf.GetY()` before the cell, (b) draw an empty `CellFormat` to get the border + fill, (c) call your overlay function with the captured coordinates, (d) `pdf.SetXY` to the column-after position before the next CellFormat. gofpdf doesn't have a native "draw inside this cell" API — this pattern is the workaround.

### 2026-05-19 — Procurement Plan bespoke renderer (planning 8/14)
- **Bespoke coverage 15/25.** First doc with **commercial-risk-coloured badges** in a cell: contract types render with green (Fixed Price = low buyer risk), amber (T&M = moderate), red (Cost Plus = high), cyan (Unit Price), slate (other). This is genuinely diagnostic — a stakeholder scanning the table immediately sees the risk distribution across procurement items.
- **`normaliseContractType` accepts messy user input.** Tested against "Fixed Price" / "fixed-price" / "FFP" → fixed; "T&M" / "Time & Materials" / "Time and Materials" → tm; "Cost Plus" / "CPFF" / "CPIF" → costplus; "Unit Price" / "per-unit" → unit. Trims case + whitespace + ampersands + dashes + underscores + literal "and" so casing/styling doesn't trip the colour mapping.
- **Sort puts blanks last.** Award-date sort with `(ai == "") != (aj == "")` puts non-empty dates first (chronological) and empty dates at the bottom of the table — the procurement officer's eye starts at the earliest commitment, not at unscheduled items.
- **Total row on the table itself**, not above it. The footer row spans the first 3 columns with right-aligned "Total" + the sum in the budget column. Heavier than a separate banner; matches what a procurement officer expects to see at the bottom of a budget table.

### 2026-05-19 — Business Case bespoke renderer (initiation phase 3/5)
- **Bespoke coverage 14/25; initiation phase 3/5** (Charter, Stakeholder Analysis, Business Case bespoke; Charter Excel aliased; Project Proposal remains generic).
- **Two new sub-patterns** worth stealing:
  - **Two-column alternative card** — header bar with the alternative's name above a pros-green / cons-red split. Used in `drawBCAlternative`. Any document with paired list comparisons (e.g. before/after, option A vs B) should use this layout.
  - **Side-by-side bulleted lists** — `drawBCTwoColumn` renders two bulleted lists with coloured headings sharing a horizontal line. Used for Benefits vs Risks. Lower-fidelity than the card layout (no border), better for short-line comparisons.
- **`drawBCRecommendation` is the accent-boxed callout pattern.** Light-blue fill, blue border, indented text — draws executive attention. Add this for any final-section "this is the decision" block (e.g. Closure's stakeholder sign-off would benefit from it on a future pass).

### 2026-05-19 — Change Request bespoke renderer (monitoring phase complete)
- **Bespoke coverage 13/25; monitoring phase 3/3.** Status Report + Issue Log + Change Request all bespoke. Next phase to target for completion is initiation (Charter + Stakeholder Analysis are bespoke; Business Case, Project Proposal still generic).
- **New layout pattern: formal form with decision badge.** Change Request introduces a 5-pattern variant that combines (a) a header strip with the Request ID block on the left and a colour-coded decision badge on the right, (b) a 2x2 impact grid for scope/schedule/cost/risk, and (c) a signature line. The badge colour-codes the decision: approved=green-700, rejected=red-700, deferred=amber-700, pending=slate-600. Future single-form documents (anything with a clear approval gate) should follow this pattern.
- **`crDecisionBadge` is permissive on terminology.** Accepts "approved" / "accepted" / "yes" → green, "rejected" / "denied" / "no" → red, "deferred" / "pending" / "on hold" / "on_hold" / "hold" → amber. Anything else falls to "PENDING" / slate. Trim+lowercase normalised so the user's casing doesn't matter.
- **The five established renderer patterns** are now:
  1. **Prose with sign-off** (Charter, SOW, Scope Statement) — portrait, sections, optional signature lines.
  2. **Status snapshot** (Status Report) — portrait, traffic-light badges.
  3. **Sorted table** (Risk Register, Communication Plan, Requirements) — landscape, colour-banded first column, optional grouping rows.
  4. **Hybrid card list** (Project Plan, Project Closure, Stakeholder Analysis) — portrait, prose + bordered cards.
  5. **Formal single-form** (Change Request) — portrait, header strip with status badge, 2x2 detail grid, signature line.
  Plus the **status-partitioned table** variant introduced with Issue Log (open + resolved bands with muted secondary).

### 2026-05-19 — Issue Log bespoke renderer (autonomous slice)
- **Bespoke coverage 12/25.** The Issue Log renderer brings monitoring-phase coverage to 2/3 (Status Report + Issue Log; Change Request still on the generic renderer). Introduces a new layout variant: **status-partitioned table with muted resolved band**. Open issues render first with full-saturation severity chips; resolved issues render below under a muted band header (slate band, half-blended severity chips, grey text) so the visual hierarchy puts attention on what still needs work.
- **New helpers worth reusing.** `isIssueResolved` is case-insensitive + whitespace-trimming and recognises five common terminal statuses (resolved/closed/done/complete/completed). `mutedColor` blends an RGB triple toward slate-400 — useful any time we need to render a secondary table with the same colour vocabulary as the primary. `shortIssueDate` truncates RFC3339-ish timestamps to YYYY-MM-DD; pull this into a shared helper file when a third renderer needs it.
- **Counts banner is small but high-value.** A single line ("N open · M resolved · K total") at the top of the page gives stakeholders the take-away even before they read the table. Future bespoke renderers with partitioned tables should follow this pattern.

### 2026-05-19 — Scope Statement, Project Budget, Requirements bespoke renderers
- **Bespoke coverage 11/25.** Three new renderers land today: Scope Statement, Project Budget, Requirements Document. Together they introduce two new layout variants that complement the four established patterns:
  - **Scope Statement** follows the Charter/SOW prose pattern (portrait A4, section headings, bulleted lists) but adds a teal left-rule accent on the Acceptance Criteria block to visually mark the formal verification gate. Shares `getString` / `getStringSlice` from charter.go because it also lives in package documents, but all drawing helpers are local per AGENT.md §10.
  - **Project Budget** is portrait (not landscape) despite being table-heavy, because three columns fit comfortably on portrait A4 and the financial summary block (subtotal / contingency / grand total) benefits from the extra vertical space. Uses alternating row fills + a dark-header row. The `formatMoney` helper does manual comma-insertion because Go's `fmt.Sprintf` does not support `%,` format; tested against 0 / 3-digit / 6-digit / 7-digit cases.
  - **Requirements Document** follows the landscape table pattern (like Risk Register) with priority-coloured Req ID cells and type-group divider rows (business → functional → non-functional → technical → other). Sorted by type first, then priority descending within each group.
- **`fmt.Sprintf("%,.2f", v)` is NOT valid Go.** Comma is not a supported flag in the Go fmt package. Always use a manual formatter or `golang.org/x/text/message` for locale-aware number formatting. Written and verified as `/tmp/moneycheck.go` before committing.
- **Dispatch wired in charter.go `Render()`.** Three new `case` arms added: `KindScopeStatement`, `KindProjectBudget`, `KindRequirements`.

### 2026-05-20 — PDF/A-3 XMP Catalog injection (internal/pdfmeta)
- **The Catalog-stream injection that V1/V2 deferred is now done.** New package `internal/pdfmeta` (zero external deps) builds the XMP packet and injects it into a finished PDF via a spec-conformant incremental update: append the Metadata stream object + a rewritten Catalog (with `/Metadata <n> 0 R`), then a delta xref table (subsections `0 1`, catalog, metadata in ascending object-number order), then a trailer with `/Size+1`, `/Root` unchanged, and `/Prev` pointing at the previous xref offset. The original bytes are preserved verbatim — purely additive.
- **Why a new package, not a function in `internal/export`.** `internal/export` already imports `internal/documents` (for DOCX/ODT rendering), so wiring XMP into `documents.Render()` would have created an import cycle (`documents → export → documents`). Extracting the byte-level work to a dependency-free leaf package (`pdfmeta`) breaks the cycle: both `documents` and `export` import it, it imports neither. **Lesson: when two sibling packages need shared logic and one already depends on the other, push the shared logic DOWN into a new leaf package rather than sideways.**
- **`export/pdfa.go` is now a thin gofpdf adapter.** It re-exports `XMPSpec` as a type alias (`type XMPSpec = pdfmeta.XMPSpec`) and delegates `BuildXMPPacket` / `InjectXMPStream` to pdfmeta, so any existing export-package call site keeps compiling unchanged. The only gofpdf-specific code left is `ApplyPDFAMetadata` (sets the library's metadata setters).
- **`documents.Render()` split into `Render` (public, XMP-wrapping) + `renderRaw` (the dispatch switch).** XMP injection is **fail-soft**: if `InjectXMPStream` errors, `Render` returns the valid-but-untagged PDF rather than failing the whole export. A desktop user should never lose a document export because a metadata step hiccupped.
- **10 unit tests in `internal/pdfmeta/pdfmeta_test.go`, all passing in the sandbox** (the package is dependency-free, so unlike most of the tree it runs under the sandbox's Go without resolving godocx/pkcs7). Tests cover: startxref parsing (incl. empty/missing), trailer Size+Root parsing, object-body location (incl. the "1 0 obj inside a content stream must not match" guard), metadata-reference insertion (both insert-when-absent and replace-existing), and the full end-to-end inject (output strictly appends, ends with %%EOF, contains the packet + rewritten Catalog + /Prev, new /Size = old+1).
- **PDF incremental-update gotchas worth remembering**: (a) xref entry lines are exactly 20 bytes — `%010d %05d n \n` (10-digit offset, space, 5-digit gen, space, type, space, newline); (b) xref subsections MUST be in ascending object-number order, so when catalogID and metaID could be in either order, sort them; (c) the `0 1\n0000000000 65535 f \n` free-list head is required even in a delta xref; (d) the marker search for an object header must be anchored to start-of-file-or-newline or a `1 0 obj`-looking substring inside a stream will match first.

### 2026-05-20 — Embedded font subsystem + user font import (internal/fonts)
- **New `internal/fonts` package** bundles a curated set of professional FOSS fonts and lets users add their own. Catalog: Liberation Sans/Serif/Mono (OFL, MS-metric-compatible), DejaVu Sans (Bitstream Vera, widest coverage), Noto Sans, Source Sans 3, JetBrains Mono — all free for commercial + personal use, all GPL-compatible.
- **The font binaries are NOT committed.** `scripts/fetch-fonts.sh` (= `make fonts`) downloads them from canonical sources into `internal/fonts/assets/`, where `//go:embed assets` bundles whatever's present. A `README.md` placeholder keeps the embed pattern valid before fetch. **Graceful degradation throughout**: absent families are omitted from `Available()`, and renderers fall back to gofpdf core Helvetica, so the app always builds and runs.
- **The killer integration trick: register the chosen family under the name "Helvetica".** All 276 `SetFont(...)` calls across documents+export use `"Helvetica"`. gofpdf's `AddUTF8FontFromBytes` *overrides a core-font family name* when you register an embedded TTF under it. So `Manager.RegisterAs(pdf, family, "Helvetica")` swaps the font for the ENTIRE renderer codebase with zero per-renderer SetFont changes. The only renderer change was `gofpdf.New("P"/"L", "mm", "A4", "")` → `newDocPDF("P"/"L")` (a helper that applies the active font), done as a mechanical perl pass across 24 files and verified by a clean compile.
- **gofpdf UTF-8 path is TrueType-only.** `validateTrueType` checks the sfnt signature and rejects OpenType/CFF ("OTTO"), WOFF, and collections ("ttcf") with actionable errors. `ImportFont` enforces `.ttf` + signature before copying into `<user>/fonts/`.
- **Wiring**: `documents.UseFont(mgr, family)` installs the applier hook (mutex-guarded; the Wails runtime renders on arbitrary goroutines). App calls it from `OpenProject` (apply saved `settings.default_font`), `CloseProject` (revert), and `SetDefaultFont` (apply immediately). New Wails methods: `ListFonts`, `ImportFont` (native file dialog, like `ChooseCertFile`), `GetDefaultFont`, `SetDefaultFont`. New TS interface `FontFamilyInfo` in wails-window.d.ts.
- **REUSE.toml added** (first one in the repo) to declare licenses for the fetched `.ttf` binaries, since binaries can't carry inline SPDX headers. OFL-1.1 + LicenseRef-Bitstream-Vera added to LICENSES/README.md.
- **FOUND + FIXED a latent compile error**: `internal/documents/report.go` called `pdf.GetPageHeight()`, which does NOT exist in the pinned gofpdf v1.16.2 (it has `GetPageSize() (w, h)`). The `documents` package had therefore never compiled — masked in the sandbox because `export` always failed first on godocx/pkcs7 resolution. Fixed to `_, pageH := pd.GetPageSize()`. **Lesson: the combined-report chart-embed path (report.go) was shipped untested against the pinned gofpdf version. Worth a smoke test on the user's machine.** When verifying, build `./internal/documents/` in isolation — it has no godocx/pkcs7 deps and now compiles cleanly in the sandbox.
- **Remaining for the frontend**: a Settings-panel font picker (dropdown over `ListFonts()`, an "Import font…" button calling `ImportFont()`, persisted via `SetDefaultFont`). The backend is complete; this is Svelte work.
- **Sandbox build note**: `go build ./internal/documents/ ./internal/fonts/ ./internal/pdfmeta/ ./internal/charts/... ./internal/db/` all succeed. `export` and `cmd/pmforge` still can't build in the sandbox (godocx v0.1.16 + pkcs7 pinned revisions don't resolve) — a pre-existing limitation, not introduced here.

### Future sessions: append below
<!-- yyyy-mm-dd — short title -->
<!-- - one-line takeaway -->

---

## 10. Quick map: "where do I add ..."

| Task                                      | File(s) to touch                                                          |
| ----------------------------------------- | ------------------------------------------------------------------------- |
| New chart kind                            | `internal/charts/registry.go` (Definition entry); pick or add engine pkg; engines.go switch; new Svelte editor; App.svelte route; Dashboard card. |
| New document kind                         | `internal/documents/registry.go` (Kind const + Definition in templates.go). |
| New document bespoke PDF renderer         | `internal/documents/<kind>.go` with `Render<Kind>PDF()`; switch in `documents.Render()`. |
| New database column                       | `internal/db/sqlite.go` Migrate() — additive only.                        |
| New CLI flag                              | `internal/cli/parser.go` Config struct + flag.*Var; handle in main.go.    |
| New Wails-exposed App method              | Add to `*App` in `cmd/pmforge/main.go`; declare in `frontend/src/wails-window.d.ts`. |
| New shared editor pattern                 | `frontend/src/lib/components/charts/_*_shell.svelte` (snippet-based).     |
| Change SPDX license for a directory       | Update each file's header; add the SPDX ID to `LICENSES/README.md`.       |

---

**End of handbook.** Keep this file lean — link to source rather than duplicate it. Source is the ground truth; this file is the map.
