<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Developer Handbook

This is the long-form developer handbook for GoPMgr. It preserves detailed
implementation history, release-gate status, and lessons learned for
engineers and automated agents. For day-to-day agent operating rules, start
with `AGENTS.md`; use this handbook for deeper project background and
release-critical status checks.

**Update protocol**: when a session changes architecture, release gates, or
durable engineering rules, update the relevant handbook section or project
memory note. Do not use this file for private handoff state that belongs in
local-only `session-notes.md` or `.agent_memory/`.

---

## 1. What GoPMgr is

GoPMgr is a **local-first project controls desktop application** for technical, engineering, IT, construction, and administrative organizations. License: **GPL-3.0-or-later**. The user described it as a GPL-licensed alternative to centralized SaaS PM tools.

- **Backend**: Go 1.26.5, acts as a high-performance kernel for data integrity, scheduling math (CPM/EVM/MSPDI), authentication, document rendering, and PDF generation.
- **Frontend**: Svelte 5 (runes mode) + Vite 8 + TypeScript 6 + Tailwind 4 + ESLint 10, mounted in a desktop window via **Wails v2.13.0**.
- **Storage**: SQLite with WAL journaling. Per-user folder isolation; one `.pmforge` file per project.
- **Charting library**: Chart.js v4.4.6 on the frontend; `go-pdf/fpdf` for server-side PDF chart rendering (migrated from the archived `jung-kurt/gofpdf`, see ADR-003).
- **Crypto**: `golang.org/x/crypto/argon2` for password hashing (PHC string format), AES-256-GCM for encryption, X.509/RSA for digital signatures.
- **Rules engine**: `github.com/gorules/zen` (MIT) via its official Go binding (zen-go) — Launchpad seeding rules expressed as JDM data, not Go switch. Used by `internal/templates`.
- **Holiday data**: `rickar/cal/v2` (BSD-2-Clause) — country holiday datasets. Wrapped by `internal/calendar`.
- **CMS/PKCS#7**: GoPMgr builds the PAdES detached CMS structure in `internal/crypto/pdf_cms.go`, using `digitorus/pkcs7` OIDs/parsing helpers where useful. The PDF embedding path lives in `internal/pdfmeta/pdfmeta.go`.
- **DOCX writer**: `gomutex/godocx` (MIT, pure Go) — picked from pkg.go.dev after a survey. Used by `internal/export/docx.go`. ODT export (`internal/export/odt.go`) is hand-built because no equivalently-maintained pure-Go ODT generator exists (kpmy/odf hasn't been touched since 2014).

The app has reached **V2.x** maturity: all 22 chart kinds and all 25 document templates implemented end-to-end, combined report builder with embedded vector chart visualisations, self-heal with atomic database swap, multi-user accounts. The Agile Pack is the current frontier.

---

## 2. Directory layout

```
gopmgr/
├── AGENTS.md                    # current agent operating guide
├── DEVELOPER_HANDBOOK.md        # this developer handbook
├── README.md                    # user/contributor documentation (GFDL)
├── LICENSES/                    # REUSE-compliant license texts
├── Makefile                     # build/lint/test/package targets
├── go.mod / wails.json / .gitignore
├── scripts/
│   ├── check-release.sh         # version + REUSE + build gate
│   └── memory-safety-scan.sh    # go vet + custom safety greps (V2.x)
│
├── main.go                      # entry point (repo root; required by wails build): CLI dispatch + Wails bootstrap
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
│   │   ├── registry.go          # 21-kind taxonomy + 4 engines (DAG/Stats/Matrix/Flow)
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
│   │   └── assets/              # tracked PDF/A baseline + optional fetched font families
│   ├── kernel/scheduler.go      # CPM forward + backward pass + critical-path marking
│   ├── pdfmeta/pdfmeta.go       # XMP packet build + Catalog incremental-update inject (dep-free leaf)
│   ├── update/check.go          # update-check stub
│   └── users/store.go           # system.db + per-user folders
│
└── frontend/                    # Svelte 5 + Vite 8 + TypeScript 6
    ├── package.json / vite.config.ts / svelte.config.js
    ├── tailwind.config.js / postcss.config.js / tsconfig.json
    ├── index.html
    └── src/
        ├── main.ts / app.css / App.svelte
        ├── wails-window.d.ts    # TypeScript surface for window.go.main.App
        └── lib/
            ├── session.svelte.ts   # rune-based shared session state
            └── components/
                ├── GanttChart.svelte
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
- **`settings`** — singleton row (CHECK id=1). Columns: `default_password`, `export_theme`, `auto_repair`, `cert_path`, legacy `signature_enabled`, `signature_method` (`none`, `pades`, `gpg`), `gpg_key_id`, `default_font` (document-export font family; empty = catalog default), `agile_enabled` (Software-Dev Pack toggle; persisted so the pack state survives project close/reopen), and `compliance_mode` (fail-closed audit verification on project open). `default_font`, `agile_enabled`, `compliance_mode`, `signature_method`, and `gpg_key_id` are additive migrations via the `settingsMigrations` loop in `migrateLegacyColumns` (PRAGMA-probe pattern covering both `project` and `settings`).
- **`tasks`** — V1 scheduler tasks: `id`, `title`, `duration`, `precedents` (JSON array of IDs), `created_at`, `updated_at`.
- **`command_log`** — append-only command journal: `id`, `ts`, `actor`, `command`, `payload` (JSON).
- **`audit_log`** — legacy operational log: `id`, `ts`, `actor`, `action`, `target_id`, `details`. Indexed by target_id and ts.
- **`audit_events`** — tamper-evident compliance audit foundation: `project_id`, `sequence_number`, previous/current SHA-256 hashes, event/entity metadata, canonical before/after JSON, actor/session metadata, timestamp, and optional signature fields. Project create/update/delete, chart create/update/delete, document create/update/delete, baseline create/delete, scenario create/update/delete, scenario-chart copy create/update, document approval checkpoints, scenario-promotion approval checkpoints, document signature outcomes, and signed combined-report outcomes now write hash-chained events. Project Settings can export private JSON audit verification and repair-evidence artifacts.

### V2 tables (multi-entity model)
- **`project`** — one row per .pmforge: `id`, `name`, `description`, `status`, `phase`, `start_date`, `end_date`, `budget`, `budget_minor_units`, `owner`, timestamps. `budget_minor_units` is canonical for money; `budget` remains a compatibility/display value. Status ∈ {planning, active, on_hold, complete, cancelled}. Phase ∈ {initiation, planning, execution, monitoring, closing}.
- **`charts`** — unified table for all 22 chart kinds: `id`, `project_id`, `kind`, `title`, `data` (JSON), `config` (JSON), `template_id`, timestamps. FK ON DELETE CASCADE.
- **`scenarios` / `scenario_charts`** — Phase 1 what-if foundation. `scenarios` stores `id`, `project_id`, `name`, optional `source_baseline_id`, `description`, `is_active`, timestamps, and a partial unique index enforcing one active scenario per project. `scenario_charts` stores isolated chart/config/baseline-data copies keyed to a scenario so later what-if edits do not mutate the live chart. Project Settings can create/edit/delete scenarios, copy live chart or saved-baseline data into a selected scenario, and open an isolated copy in the dedicated scenario chart editor. The scenario editor edits, compares against captured baseline data, and promotes copied scenario chart data back to a named baseline.
- **`documents`** — unified for all 25 doc kinds: `id`, `project_id`, `kind`, `title`, `content` (JSON), `template_id`, `version` (monotonic), `status` (draft|review|approved|archived), timestamps.
- **`templates`** — user-saved templates: `id`, `scope` ('chart' or 'document'), `kind`, `name`, `description`, `defaults` (JSON), `is_builtin`, `created_at`.

### Agile tables (V2.x — Software-Dev Pack)
- **`agile_boards`** — `id`, `project_id`, `name`, `is_default`, timestamps.
- **`agile_columns`** — `id`, `board_id`, `name`, `order_idx`, `wip_limit` (0 = unlimited).
- **`agile_work_items`** — `id`, `project_id`, `type` (story|bug|task|epic), `title`, `description`, `state` (column ID or "backlog"), `points`, `assignee`, `sprint_id`, `priority` (low|medium|high|urgent), `order_idx`, timestamps, `closed_at`.
- **`agile_sprints`** — `id`, `project_id`, `name`, `goal`, `status` (planning|active|complete), `start_date`, `end_date`, `capacity` (story points), `created_at`.
- **`agile_deployments`** — `id`, `project_id`, `ts`, `version`, `successful`, `lead_time_hours`, `restore_time_hours`, `notes`.

### System database (top-level, NOT per-project)
- **`<data-root>/system.db`** (macOS `~/Library/Application Support/PMForge`, else `~/Documents/PMForge`; see `users.DefaultRootDir`) holds account credentials:
- **`users`** — `username` (PK), `display_name`, `password_hash` (PHC Argon2id), `data_dir`, `created_at`, `last_login`.

---

## 4. Coding conventions

### SPDX headers — REQUIRED on every source file

```go
// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later
```

HTML-style comment for Svelte / HTML / Markdown files. Documentation files use `GFDL-1.3-or-later`; tiny configs may use `CC0-1.0`. `make license-check` runs `reuse lint`.

### Go conventions

- **Package-level doc comment** on every package's primary file. Comments are `//`-style, full sentences, end with period.
- **Error wrapping**: use `fmt.Errorf("context: %w", err)`. For recoverable paths that the UI needs to introspect, use `debug.Wrap(err, "TAG").ToError()`.
- **No goroutines** in GoPMgr's own code today — the Wails runtime is the only goroutine spawner.
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
- **Layout-only renderers**: backend chart layout (`charts.Layout()`) emits JSON. Frontend renders SVG/Chart.js. For PDF embed, `pdfrender.RenderChartToPDF()` draws the same layout with fpdf primitives — vector, not PNG.

---

## 5. Build / run / test workflow

```sh
# First-time setup
go mod tidy
(cd frontend && npm ci)                  # use npm ci, not npm install (see npm-ci lesson below)
make fonts                               # download bundled TTFs into internal/fonts/assets
(cd LICENSES && reuse download --all)   # optional, for `make license-check`

# Dev loop (hot-reload Go + Svelte)
wails dev

# Production binary (embeds frontend via go:embed)
make build

# Quality gates
make lint              # golangci-lint + npm run lint
make test              # go test . ./internal/... (GoPMgr-owned Go packages)
make race              # go test -race . ./internal/... (V2.x concurrency hardening gate)
make memory-scan       # scripts/memory-safety-scan.sh (V2.x)
make frontend-stability # svelte-check --fail-on-warnings + Sigma regression gates
make frontend-build-budget # Vite build without large main bundle regressions
make license-check     # reuse lint
make check-release     # full release gate, including build, PDF/A, and PAdES

# Packaging (host-local deterministic tarballs; cross-platform targets require matching runners)
make package-linux / package-windows / package-darwin
```

**First-launch behaviour**: creates `<data-root>/system.db` (account list) and provisions `<data-root>/<username>/{projects,certs,exports}/` for each user (chmod 0700 on POSIX). The data root is `~/Library/Application Support/PMForge` on macOS (off iCloud and outside the TCC-protected Documents folder) and `~/Documents/PMForge` elsewhere; on macOS an existing `~/Documents/PMForge` install is copied into the new root on first launch via `users.MigrateLegacyRoot` (`NewApp` calls it before `users.Open`).

---

## 6. Locking & concurrency invariants

**The Wails runtime dispatches every frontend call on a separate goroutine.** The App struct in the root `main.go` is therefore accessed concurrently and must be guarded.

### App locking rules
- `App.mu` is a `sync.RWMutex` (V2.x hardening).
- **Mutable fields** under the lock: `user`, `db`, `dbPath`, `adminSvc`.
- **Read-only fields** (set once in NewApp): `store`. May be read without the lock. `shutdown()` closes the store but **never reassigns the pointer** — nilling it would be a write racing the lock-free reads this invariant permits.
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
1. `go vet . ./internal/...` — standard correctness checks.
2. Custom grep gate (scripts/memory-safety-scan.sh) for:
   - `os.Open(` without nearby `defer .*Close()`
   - `sql.Open(` without nearby `defer .*Close()`
   - `unsafe.Pointer` (forbidden in this codebase)
   - missing `errors.Is`/`errors.As` against package `Err*` sentinels
3. (When installed) advisory `staticcheck . ./internal/...` for deeper analysis.
4. (When installed) advisory `gosec . ./internal/...` for security-flavoured patterns.

A new contribution should land **with `make memory-scan` passing**. Optional scanners report findings without failing by default so the release gate is not dependent on locally installed tools; set `GOPMGR_STRICT_OPTIONAL_SCANS=1` when you want optional staticcheck/gosec/govulncheck findings to fail the gate. The gate is wired into `make check-release`.

---

## 8. Feature coverage (live status)

### Charts: 21/21 implemented end-to-end (UI + Go layout + frontend renderer + PDF embed)
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
- **Full document create→edit→export loop for all 25 kinds.** Dashboard template cards are clickable buttons; `App.svelte` routes both `charter` and `documents` views to `CharterEditor.svelte` (the generic document editor); header toolbar exposes DOCX, ODT, PDF, and Signed-PDF export for every kind.
- **Delete buttons for charts and documents in Dashboard.** Inline two-step confirm pattern (click Delete → confirm → delete) with local state filter; no page reload.
- **Export & Signature settings in Project Settings panel.** `ProjectSettings.svelte` now reads/writes `export_theme`, `auto_repair`, `cert_path`, `signature_enabled` from the settings DB row and exposes JSON audit verification and repair-evidence exports. Font picker (family dropdown + Import button) also lives there.
- **Ctrl+S keyboard shortcut in all editors.** `CharterEditor.svelte`, `_layered_editor_shell.svelte`, and `_stats_editor_shell.svelte` all register a `keydown` listener in `onMount` and remove it in `onDestroy`.
- **Dirty indicator and status dropdown in CharterEditor.** Baseline `lastSavedContent`/`lastSavedTitle` set after load; `dirty` derived state drives an amber "Unsaved changes" badge. Status dropdown (`draft|review|approved|archived`) in the header calls `save()` on change.

### Agile Pack (V2.x — complete)
- **Backend**: schema (5 tables in db/sqlite.go), types (agile/agile.go), CRUD storage (agile/store.go), DORA metrics with elite/high/medium/low classification (agile/dora.go), Wails methods in the root main.go §Agile Pack.
- **Frontend**: KanbanBoard (drag-and-drop with WIP badges), Backlog (priority + drag reorder + Start-work), SprintList (planning/active/complete lifecycle with single-active invariant), DORADashboard (4 KPI cards + deploy-trend line via StatsChart + inline +Deployment form). All live under `frontend/src/lib/components/agile/`.
- **Wiring**: 4 new session view union members (`kanban`, `backlog`, `sprints`, `dora`), App.svelte routes, Dashboard "Software-Dev Pack" section with enable/disable toggle backed by `App.AgileEnabled` / `App.SetAgileEnabled`. As of 2026-06-04, `AgileEnabled` is **persisted to `settings.agile_enabled`** (not in-memory only); `SetAgileEnabled` does a DB roundtrip and updates `agile.PackEnabled` as a cache.

### Memory & concurrency gates (V2.x)
- **`make memory-scan`** runs `scripts/memory-safety-scan.sh`. Currently passing in the sandbox; on a dev box with Go in PATH it also runs `go vet` and a Go-helper scan for unclosed `os.Open` handles. Optional integrations: `staticcheck`, `gosec`, `govulncheck` — auto-detected.
- **`make race`** runs `go test -race . ./internal/...`.
- Both are wired into `scripts/check-release.sh` so the release gate fails if either does.

### Remaining V2 TODOs (status snapshot)
1. ~~DOCX / ODT export.~~ **Done.** `internal/export/docx.go` uses `gomutex/godocx`; `internal/export/odt.go` is hand-built (no maintained ODT library exists). App methods `ExportDocumentDOCX` / `ExportDocumentODT`.
2. **PDF/A-3 strict conformance** — partial, advanced 2026-05-20, 2026-05-25, 2026-06-06, and 2026-07-27. (i) The dependency-free `internal/pdfmeta` package builds the canonical XMP packet AND injects it into the PDF Catalog via a spec-conformant **incremental update** (`InjectXMPStream`); `documents.Render()` tags every generated PDF (fail-soft). (ii) **Font embedding is now available** via `internal/fonts` — Source Sans 3 is tracked as the deterministic baseline, while optional families are fetched by `make fonts`; the "register under Helvetica" trick replaces the non-embeddable core fonts. (iii) OutputIntent + ICC profile injection is implemented (`InjectOutputIntent`, `MakePDFA3`, `make icc`) and used when an ICC profile is embedded. (iv) The schedule-report, document, combined-report, and Monte Carlo risk-report samples now pass `make check-pdfa` with veraPDF's PDF/A-3b profile after adding binary header comments, trailer IDs, stream-length correctness, latest-incremental Catalog rewrites, and embedded Source Sans 3 for representative exports. The gate is now a **hard release blocker**: `check-release.sh` exits non-zero if any representative sample fails PDF/A-3b validation (2026-06-08).
3. ~~CMS/PKCS#7 + PAdES signature widget embedding.~~ **Done** via GoPMgr's detached CMS encoder plus `pdfmeta.InjectPAdESSignature`. The PAdES path appends a `/Sig` dictionary, invisible `/Widget` field, `/AcroForm`, fixed-width `/ByteRange`, signed `/M` timestamp, and padded `/Contents` in the final incremental update. `make check-pades` verifies a timestamped local fixture, and `make check-pades-external` extracts the embedded CMS for OpenSSL detached verification, checks `qpdf --check`, requires `pdfsig` to report a valid signature, verifies veraPDF signature metadata, and requires DSS to classify the self-signed fixture as `PAdES-BASELINE-T` when those tools are installed. Release-certificate and TSA trust-chain validation remain indeterminate until trusted sources are configured; `make check-pades-trusted` records `NOT_CONFIGURED` without a source and otherwise distinguishes locally verified CLI trust from structurally valid but indeterminate trust. Set `GOPMGR_PADES_TRUSTED_REQUIRED=1` when a release process must reject anything except `TRUST_VERIFIED`. Acrobat trust-panel evidence remains separately manual. Users can choose PAdES Baseline B, detached GnuPG sidecar signing, or no digital signature for print-and-wet-sign exports.
   **RFC 3161/PAdES-T foundation added 2026-07-25.** `internal/rfc3161.Client`
   creates nonce-bound SHA-256 requests and validates the HTTP response,
   CMS/TSTInfo content type, token signature, imprint, nonce, requested
   policy, generation time, timestamping-only critical EKU, and optional
   caller-provided trust roots. It requires HTTPS, rejects redirects, and
   bounds response size. `crypto.SignatureTimestampImprint` hashes the existing
   signer signature value, and `crypto.AddSignatureTimestamp` revalidates and
   inserts the token as the DER `signatureTimeStampToken` unsigned attribute
   without modifying signed attributes or signature bytes. Project settings
   now persist an opt-in credential-free HTTPS endpoint, optional policy OID,
   and optional PEM root. `signing.ApplyPAdES` wires both signed export paths
   through one fail-closed Baseline B/T pipeline and records the actual
   baseline and TSA trust outcome in the audit chain.
4. ~~Wails file-picker for certs.~~ **Done.** `App.ChooseCertFile` calls `wailsruntime.OpenFileDialog`.
5. ~~HTTPS update channel with signed release manifest.~~ **Done.** `internal/update` fetches a signed JSON manifest, verifies Ed25519, returns `Status`. `ManifestURL` and `UpdateChannelPublicKey` set at build time via `-ldflags`.
8. ~~Per-user database encryption-at-rest decision.~~ **Implemented 2026-06-13.** New per-user `.pmforge` project databases are SQLCipher-encrypted with the user's DEK; existing plaintext project databases can be migrated from Project Settings after recovery codes are reissued. `system.db` remains plaintext by design and stores password hashes plus wrapped DEKs, not project records. `.pmba` bundles preserve encrypted `project.pmforge` bytes. OS-level encryption (FileVault / BitLocker / LUKS) remains recommended whole-device defence in depth.
9. ~~Bespoke renderers for the 24 non-Charter document kinds.~~ **Done.** All 23 bespoke renderers + 2 aliases shipped (see §8 feature coverage). `internal/documents/documents_test.go` adds 33 tests: registry (All/Get/ByPhase), DefaultContent round-trip for all 25 kinds, and `TestRender_AllKindsProduceValidPDF` which smoke-tests every dispatcher branch (2026-06-08).
10. ~~Embed chart visualisations in combined reports.~~ Done in earlier slice.
13. ~~Account recovery codes.~~ **Done.** 8 Argon2id-hashed codes generated at account creation, redeemable once each. `App.IssueRecoveryCodes` + `App.ResetWithRecoveryCode`. Frontend: `RecoveryReset.svelte`.

### Still deferred to V3
- ~~Strict PDF/A-3 release claim~~ **Done (2026-06-08, expanded 2026-06-29).** The representative schedule-report, document, combined-report, and Monte Carlo risk-report samples pass veraPDF PDF/A-3b; `make check-pdfa` is a hard gate in `check-release.sh`. V3 remainder: Acrobat coverage and trusted signing chain.
- External PAdES validation hardening — the widget, application signing pipeline, and PAdES-T mutator are exercised by deterministic tests and `make check-pades`; OpenSSL detached CMS verification, local `qpdf`/`pdfsig` checks, veraPDF signature feature extraction, and DSS `PAdES-BASELINE-T` fixture classification are covered by `make check-pades-external`. `make check-pades-trusted` can verify a separately supplied sample against the local CLI trust store, but sample signed PDFs still need Acrobat evidence and a real trusted signer/TSA chain before treating interoperability as fully battle-tested.
- CPM/PDM dependency-lag editor design if task-level precedence relationships need visual lag editing beyond the shipped Timeline project/sprint date dragging.

### Scheduling core roadmap (V3) — added 2026-06-10

Canonical list lives in this Developer Handbook. README.md is now the public overview and documentation index. GoPMgr stays local-first; the roadmap deepens the scheduling kernel in dependency order: (14) date-anchored calendar-aware CPM — **done 2026-06-10** (`kernel.AnchorSchedule`; anchored MSPDI/schedule-report exports; CPM editor shows real dates via `charts.LayoutWithSchedule`; date-axis Gantt strip deferred to item 20), (15) dependency types FS/SS/FF/SF + lag — **done 2026-06-10** (`kernel.Link` + PDM passes with projectEF-bounded LF; `dag.ParseLinkLabel`; "Incoming links" edge-label editor in the layered shell; legacy Precedents preserved), (16) task constraints — **done 2026-06-10** (`kernel.ConstraintType` ASAP/ALAP/SNET/FNLT/MFO; `ApplyConstraintDates` arming; links-win-with-violation-flag semantics; negative float = super-critical; CPM editor dropdown/date/amber marker; `dag.LayoutCPMScheduled`), (17) progress/milestones/baselines — **done 2026-06-10** (`Task.PercentComplete/Milestone/ActualStart/ActualFinish`; `baselines` table + CRUD; `kernel.CompareSchedules`; Set-baseline button + variance rows in CPMEditor; actual-date entry UI deferred to item 18 where AC matters), (18) Earned Value Management — **done 2026-06-10** (`kernel.ComputeEVM` PV/EV/AC + SV/CV/SPI/CPI/EAC/ETC/VAC at a status day; Task.BudgetedCost/ActualCost; `App.ComputeScheduleEVM` requires a project start date; EVM panel in CPMEditor via new asideExtra shell slot; the docs-must-not-claim-EVM rule is retired — the claim is now true; report-renderer EVM sections remain an optional follow-up), (19) resource layer — **kernel core + assignment UI + Level/Histogram actions done 2026-06-10; named calendar persistence/UI advanced 2026-06-26** (`Assignment`/`ResourceUsage`/`DetectOverallocations`/`LevelResources`; CPMEditor Assignments section with stakeholder datalist; overallocation flags with orange edge strip; `App.LevelChartResources` persists delays as SNET pins + shell `reloadFromDB` guards against stale-doc clobber; `App.GenerateResourceHistogram` snapshots demand into a Bar chart keyed by `source_chart_id`; availability column on stakeholders feeds capacity maps; `ResourceCapacityPlan`/`ResourceCalendar` support day overrides, weekly capacity, max-unit caps, calendar IDs, and skill tags; `resource_calendars` persists named calendars and Project Settings exposes weekly/day capacity inputs; `LevelChartResources` now uses persisted calendars), (20) schedule interchange + first-class Gantt — **done 2026-06-10** (`export.FromMSPDI` import with typed links/lag/milestones/percent/assignments and summary-row skipping; `ToMSPDI` enriched for round-trip with PredecessorLink/Milestone/PercentComplete/Resources/Assignments; `App.ImportMSPDIChart` + Dashboard button; file start date adopted when project lacks one; .mpp binary out of scope; Gantt is the 21st chart kind: dag.LayoutGantt[Scheduled], pdfrender renderer, GanttEditor.svelte with grid/links/bars/deps/baseline ghosts; registry tests updated 20→21 and README chart counts swept). Items 14–18 are kernel-pure. This supersedes the older "CPM/PDM dependency-lag editor design" bullet above (now roadmap item 15).

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
- **The Wails runtime spawns goroutines per call.** The hardening pass confirmed GoPMgr itself spawns zero — the goroutine grep returns empty after the regex tightening. All concurrent state is the App struct, fully guarded.

### 2026-05-19 — SOW + Closure + Stakeholder Analysis renderers + pure-data unit tests
- **Bespoke coverage 8/25.** Statement of Work (prose + sign-off), Project Closure (mixed prose + lessons-learned table + sign-off line), Stakeholder Analysis (per-stakeholder cards grouped by quadrant). The three together demonstrate the FOUR distinct shape patterns we've now established:
  1. **Prose with sign-off** (Charter, Statement of Work) — portrait, section heads, signature lines at the bottom.
  2. **Status snapshot** (Status Report) — portrait, traffic-light badges at the top, bulleted sections.
  3. **Sorted table** (Risk Register, Communication Plan) — landscape, color-banded first column, sorted/grouped rows.
  4. **Hybrid card list** (Project Plan, Project Closure, Stakeholder Analysis) — portrait, mix of prose sections + bordered cards.
  Future bespoke renderers should pick the closest match and copy the helpers from that file (per DEVELOPER_HANDBOOK.md §10's "each renderer self-contained" rule).
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
- **App.templates is intentionally non-fatal.** If zen-go fails to initialise the JDM engine at startup, we log and continue — the Launchpad falls back to "no auto-seed" and the rest of the app keeps working. A misconfigured rule should never brick GoPMgr.

### 2026-05-14 — Agile Pack frontend
- **Native HTML5 drag-and-drop is sufficient** for the Kanban board and Backlog reorder. No external DnD library needed; `draggable="true"` + `ondragstart` / `ondragover` / `ondrop` covers it. The reorder pattern (drag a list item, push positions through `order_idx`) matches what `ReportComposer.svelte` already does — two cases now, established pattern.
- **DORADashboard reuses `StatsChart.svelte`** for the deploy-trend mini-chart by constructing a `StatsLayout` inline. Cross-feature reuse: the stats engine wasn't meant for agile, but it just works because the layout types are public. Confirms the registry+layout architecture pays off.
- **Single-active-sprint is GUI-enforced**, not schema-enforced. When the user clicks "Start" on a planning sprint, `SprintList.activate()` first sweeps any other `active` sprint to `complete` then activates the target. Keeping this in the frontend means the backend stays simple and the rule is visible/testable in one place.
- **WorkItemEditor uses a `lastItemID` sentinel** to decide when to re-seed the local `draft` from the `item` prop. Without this, parent-side optimistic updates would clobber unsaved edits every time the parent re-renders. The sentinel pattern is reusable for any "edit a record in a modal" component.
- **AgileEnabled is in-memory only** (per DEVELOPER_HANDBOOK.md §8). The Dashboard's toggle calls `SetAgileEnabled` which flips `agile.PackEnabled` in process. Persisting this across restarts is a one-line addition to `settings` later if needed.
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
- **What's next.** Bespoke coverage saturated. The next investment areas per DEVELOPER_HANDBOOK.md §8 are: (a) PDF/A-3 strict conformance validation (veraPDF gate hardening now that font embedding, Catalog XMP, and OutputIntent/ICC code exist), (b) external PAdES validator hardening for signed sample PDFs, (c) per-user encryption at rest (SQLCipher), (d) PDM date-dragging on the Timeline. All four are V3 milestones requiring significantly larger slices.

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
- **`parseDate` accepts both YYYY-MM-DD and RFC3339** so the same helper works whether the date came from a Wails form (typically RFC3339Nano) or from the user typing into a string field in the JSON. Pull this into a shared `internal/documents/dates.go` if a fourth renderer needs it — for now it's local-to-file per DEVELOPER_HANDBOOK.md §10's self-contained rule.
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
  - **Scope Statement** follows the Charter/SOW prose pattern (portrait A4, section headings, bulleted lists) but adds a teal left-rule accent on the Acceptance Criteria block to visually mark the formal verification gate. Shares `getString` / `getStringSlice` from charter.go because it also lives in package documents, but all drawing helpers are local per DEVELOPER_HANDBOOK.md §10.
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
- **Original packaging decision (superseded 2026-07-27):** font binaries were initially left uncommitted and fetched with `make fonts`. Clean hosted builds therefore omitted every TTF and strict PDF/A samples fell back to non-embeddable core Helvetica. Four Source Sans 3 faces are now tracked as the deterministic archival baseline and guarded by `make required-font-assets`; optional families remain fetch-on-demand. Graceful fallback applies to optional choices, not the PDF/A baseline.
- **The killer integration trick: register the chosen family under the name "Helvetica".** All 276 `SetFont(...)` calls across documents+export use `"Helvetica"`. gofpdf's `AddUTF8FontFromBytes` *overrides a core-font family name* when you register an embedded TTF under it. So `Manager.RegisterAs(pdf, family, "Helvetica")` swaps the font for the ENTIRE renderer codebase with zero per-renderer SetFont changes. The only renderer change was `gofpdf.New("P"/"L", "mm", "A4", "")` → `newDocPDF("P"/"L")` (a helper that applies the active font), done as a mechanical perl pass across 24 files and verified by a clean compile.
- **gofpdf UTF-8 path is TrueType-only.** `validateTrueType` checks the sfnt signature and rejects OpenType/CFF ("OTTO"), WOFF, and collections ("ttcf") with actionable errors. `ImportFont` enforces `.ttf` + signature before copying into `<user>/fonts/`.
- **Wiring**: `documents.UseFont(mgr, family)` installs the applier hook (mutex-guarded; the Wails runtime renders on arbitrary goroutines). App calls it from `OpenProject` (apply saved `settings.default_font`), `CloseProject` (revert), and `SetDefaultFont` (apply immediately). New Wails methods: `ListFonts`, `ImportFont` (native file dialog, like `ChooseCertFile`), `GetDefaultFont`, `SetDefaultFont`. New TS interface `FontFamilyInfo` in wails-window.d.ts.
- **REUSE.toml added** (first one in the repo) to declare licenses for fetched `.ttf` binaries, embedded ICC profiles, generated lockfiles, and other files that cannot carry inline SPDX headers. OFL-1.1 + LicenseRef-Bitstream-Vera are documented in LICENSES.md.
- **FOUND + FIXED a latent compile error**: `internal/documents/report.go` called `pdf.GetPageHeight()`, which does NOT exist in the pinned gofpdf v1.16.2 (it has `GetPageSize() (w, h)`). The `documents` package had therefore never compiled — masked in the sandbox because `export` always failed first on godocx/pkcs7 resolution. Fixed to `_, pageH := pd.GetPageSize()`. **Lesson: the combined-report chart-embed path (report.go) was shipped untested against the pinned gofpdf version. Worth a smoke test on the user's machine.** When verifying, build `./internal/documents/` in isolation — it has no godocx/pkcs7 deps and now compiles cleanly in the sandbox.
- **Remaining for the frontend**: a Settings-panel font picker (dropdown over `ListFonts()`, an "Import font…" button calling `ImportFont()`, persisted via `SetDefaultFont`). The backend is complete; this is Svelte work.
- **Sandbox build note**: `go build ./internal/documents/ ./internal/fonts/ ./internal/pdfmeta/ ./internal/charts/... ./internal/db/` all succeed. `export` and `cmd/gopmgr` still can't build in the sandbox (godocx v0.1.16 + pkcs7 pinned revisions don't resolve) — a pre-existing limitation, not introduced here.

### 2026-06-04 — CPM kernel + DORA classification tests
- **`internal/kernel` now has 10 unit tests covering every branch of CalculateCPM and topoSort.** Cases: empty map, single task, linear chain (A→B→C), diamond network (A→B/C→D with longer branch on critical path), parallel equal-length paths (both critical), zero-duration milestones, cycle detection (mutual reference + self-loop). `topoSort` tests cover dependency ordering and alphabetical determinism. The package doc comment explicitly noted isolation testing was intended — this was pure overdue work.
- **`internal/agile/dora.go` now has 35 unit tests (in `dora_test.go`).** Covers all four classification functions at each band boundary (`classifyDeployFrequency`, `classifyLeadTime`, `classifyCFR`, `classifyMTTR`), the `median` helper (empty, odd, even, unsorted input), the `formatFloat1` shim (zero, whole, decimal, negative), and `ComputeDORA` end-to-end (empty, window filtering, default window fallback, elite-team scenario, daily trend length, medium CFR scenario).
- **Test misread correction: deploy-frequency thresholds.** 0.5 deploys/day is "high" (not "elite" — elite requires ≥ 1.0/day). 1/14-day is "medium" (not "high" — high requires ≥ 1/7-day). Both the code and DORA spec are correct; the initial test expectations were wrong. This illustrates why boundary tests should be written from the code, not from memory.
- **`range N` syntax is idiomatic Go 1.22+.** Used in `dora_test.go` for the elite-team loop; this Go module targets 1.26.5 so no compatibility concern.

### 2026-06-04 — Sigma tollgate + stats tests
- **`internal/sigma/tollgate` now has 23 unit tests in `readiness_test.go`.** Covers all four phase checkers (Define, Analyze, Improve, Control) and the `CheckPhase` router, including the 80%-threshold for Define (5/7 ≠ advance, 6/7 = advance), the 100%-threshold for Analyze/Improve/Control, CTQ spec-limit requirement, minimum character lengths for all five charter text fields, SIPOC element count, fishbone causes vs. 5-Whys drill-down depth (3 levels minimum), solution count + impact/effort scoring + selection, control item owner + response-plan presence, and the Measure phase auto-approve default arm.
- **`internal/sigma/stats` now has 10 unit tests in `basic_test.go`.** Covers `CalculateDescriptive` (empty error, single value, odd/even count, positive std dev), `CalculateCapability` (empty error, zero-std-dev error, Cp formula positive, Cpk < Cp for off-center process, DPMO band at sigma ≥ 6 = 3.4 defects/million).
- **Boundary-value misread lesson (second occurrence).** In the Define-phase test, "Also short." (11 chars) satisfied the BusinessCase ≥ 10 minimum. The pattern: always verify lengths in Go before writing a test that assumes a string is "too short."
- **`range N` is idiomatic in Go 1.22+ (this module targets 1.26.5).** Used in stats_test.go loops; avoids the `for i := 0; i < N; i++` boilerplate.

### 2026-06-04 — PERT math, RACI validation, AES-GCM crypto tests
- **`internal/charts/dag` now has 6 PERT unit tests in `pert_test.go`.** Verifies the textbook beta-distribution formulas (E=(O+4M+P)/6, V=((P-O)/6)^2, σ=√V) against hand-calculated values, the all-zero no-op guard, the certain-duration case (V=σ=0), structural invariants (StdDev=√Variance, Duration=Expected), and the symmetric-range case. `annotatePERT` is unexported but accessible from within `package dag`.
- **`internal/charts/matrix` now has 12 RACI unit tests in `raci_test.go`.** Covers `ParseRACI` (empty string, `"{}"` early-return path, invalid JSON, valid document), `LayoutRACI` cell-grid size (roles×tasks), zero-Accountable issue, multiple-Accountable issue, exactly-one-A no-issue, zero-Responsible issue, valid complete matrix, empty document, and `Validation.AddIssue` incrementing ErrorCount. Found that `ParseRACI("{}")` returns early before the nil-Assignments guard — documented in the test comment.
- **`internal/crypto` now has 6 AES-GCM+Argon2id tests in `encrypt_test.go`.** The three cheap tests (empty-password errors, truncated ciphertext) run in <1 ms. The three Argon2id-heavy tests (roundtrip, wrong-password, fresh-nonce) are guarded with `t.Skip` in short mode; on this machine they each take ~0.02-0.03 s because Go is fast with argonThreads=4. The guard stays for CI environments with restricted memory.
- **19 packages now have test coverage.** Remaining `[no test files]` packages: `admin`, `charts/flow`, `charts/pdfrender`, `charts/stats`, `cli`, `debug`, `sigma/charts`, `sigma/domain`, `sigma/service`. The pure-data leaf packages (dag, matrix, kernel, crypto, sigma/tollgate, sigma/stats, agile/dora) are now covered.

### 2026-06-04 — Pareto sort + Control chart tests (charts/stats)
- **`internal/charts/stats` now has 27 unit tests in `stats_test.go`.** Covers `ParsePareto` (empty/`"{}"` early-return, invalid JSON, valid doc), `LayoutPareto` (descending sort by count, exact cumulative-percentage values at 50/80/100%, zero-total stays all-zero, dashed 80% annotation present, YAxisRight min=0 max=100, kind="pareto" with bar+line series), `ParseControl` (same early-return and JSON patterns as Pareto), `LayoutControl` (auto-compute mean±3σ when Mean=UCL=LCL=0 verified against known values, explicit limits are not overridden, above-UCL flag at correct point index, below-LCL flag, no flags when all within limits, empty Y produces no flags, Categories derived from floatsToStrings(X)), and the unexported helpers `computeMean` (known values + empty=0) and `computeStdDev` (sample std dev sqrt(sum/n-1), single-element=0, empty=0).
- **20 packages now have test coverage.** Remaining `[no test files]` packages: `admin`, `charts/flow`, `charts/pdfrender`, `cli`, `debug`, `sigma/charts`, `sigma/domain`, `sigma/service`. All pure-data leaf packages are now covered (dag, matrix, kernel, crypto, sigma/tollgate, sigma/stats, agile/dora, charts/stats).
- **`computeStdDev` uses n-1 (sample std dev).** For `[1,2,3]` with mean=2: sum of squares=2, divided by 2, sqrt=1.0. Future Control chart consumers expecting population std dev should note this distinction.

### 2026-06-04 — debug error envelope, sigma/charts Pareto, cli version tests
- **`internal/debug` now has 9 unit tests in `report_test.go`.** Covers `Wrap` with a non-nil error (Context/Message/Cause fields), `Wrap` with nil (Message==context, Cause==""), file:line capture (File ends with `_test.go` — Wrap records the immediate caller), non-empty Stack, nanosecond-resolution Timestamp within ±1s, `ToError()` returning a non-nil error whose string equals Message, round-trip through `ToError`/`Report` recovering the original ErrorReport, and `Report` returning false for plain `errors.New` and for nil.
- **`internal/sigma/charts` now has 10 unit tests in `pareto_test.go`.** Covers `CalculatePareto` error paths (empty input, length mismatch, zero total), single-item edge case (pct=100, cum=100), descending sort by count, exact percentage values, exact cumulative percentage values (50/80/100 for input 50/30/20), structural invariant (last CumulativePercentage == 100.0), stable sort for equal counts, and output-length matches input.
- **`internal/cli` now has 3 unit tests in `parser_test.go`.** Covers `Version` non-empty, `PrintVersion` stdout output containing "GoPMgr", `Version`, and "GPL" (via `os.Pipe` capture), and `Config` zero-value coherence (bool fields default false, string fields default empty). `ParseFlags()` is not unit-tested because it calls `flag.Parse()` against the global `flag.CommandLine` and `os.Args` — the safe test boundary is the banner and the type structure.
- **23 packages now have test coverage.** Remaining `[no test files]` packages: `admin`, `charts/flow`, `charts/pdfrender`, `sigma/domain`, `sigma/service`. All pure-function leaf packages are now covered; remaining gaps require SQLite or are type-only definitions with no logic.

### 2026-06-04 — Flow chart layout tests (charts/flow)
- **`internal/charts/flow` now has 33 unit tests in `flow_test.go`.** Covers: `ParseWorkflow`/`ParseActivity` (empty string, `"{}"`, invalid JSON, valid document), `EncodeWorkflow` round-trip, `layerNodes` (linear chain A→B→C giving ranks 0/1/2, diamond A→B/C→D giving D rank 2, mutual-cycle returning ok=false, alphabetical queue ordering verified on three parallel sources), `resolveWorkflowShape` (all six known shapes pass through; unknown defaults to "action"), `resolveActivityShape` (all six known shapes pass through; unknown defaults to "activity"), `activityNodeSize` (initial/final=28×28, fork/join=SwimlaneWidth-40×8, activity=NodeWidth-20×NodeHeight), `hasDefaultLane` (all-assigned=false, empty SwimlaneID=true, unknown SwimlaneID=true), `LayoutWorkflow` (empty nodes returns empty layout, single-node geometry X=0/Y=0/W=150/H=60, decision node taller than action, linear chain B.Y equals rowStride, cycle returns ErrCycle, three parallel nodes all X≥0, edge label preserved), `LayoutActivity` (empty nodes returns swimlane bands with correct X offsets, cycle returns ErrCycleActivity, unassigned node triggers default lane with ID="" in output).
- **24 packages now have test coverage.** Remaining `[no test files]`: `admin`, `charts/pdfrender`, `sigma/domain`, `sigma/service`. The remaining gaps all require SQLite or are pure type definitions with no logic to test.
- **`layerNodes` uses Kahn's algorithm with a sorted queue for deterministic output.** The alphabetical ordering is enforced by `sort.Strings(queue)` after every indegree-zero node is pushed. Tests rely on this guarantee for layer-content assertions.
- **Activity layout adds an "(unassigned)" swimlane on demand.** The `hasDefaultLane` check runs before layout; if any node has an empty or unknown SwimlaneID, an extra column appears at the right of the canvas with `ID=""`. Tests confirm both the presence detection and the output lane count.

### 2026-06-04 — WBS, Fishbone, Causal Tree, Layered layout tests (charts/dag)
- **`internal/charts/dag` now has 43 tests total (37 new in `dag_test.go` + 6 existing in `pert_test.go`).** New tests cover: `Parse` (empty string → ErrEmptyTree, null root → ErrEmptyTree, invalid JSON, valid document), `Renumber` (single node "1", two children "1.1"/"1.2", three-level "1.1.1", nil/empty no panic), `FlattenLeaves` (single root is a leaf; parent with children is excluded), `TotalEffort` (sums leaf efforts, ignoring parent's own Effort field), `LayoutWBS` (nil root → empty, single node has non-negative XY and positive canvas, parent+children → 2 edges), `itoa` (0→"0", 1→"1", 10→"10", 123→"123"), `ParseLayered` (empty, invalid JSON), `LayoutLayered` (empty, single node Y≥0, linear chain A.Depth=0/B.Depth=1 and B.X>A.X, cycle → ErrCycle, two parallel nodes both Y≥0 after shiftY pass), `barycenter` (no neighbours → self pos, two neighbours → mean 2.0), `findMinY` (empty → 0, negative Y → min), `ParseFishbone` (empty, invalid JSON), `LayoutFishbone` (no categories → 1 effect node, with category → effect present, 1-category 2-causes → 4 total nodes, canvas size positive), `ParseCausalTree` (empty, invalid JSON), `LayoutCausalTree` (nil root → ErrNoRoot, single node → 1 node 0 edges, root+2 children → 3 nodes 2 edges).
- **`within` helper from `pert_test.go` is shared.** Both files live in `package dag`; new dag test files must not re-declare `within`.
- **`LayoutLayered` shifts Y when the centering offset produces negative coordinates.** Two nodes in the same layer get `offsetY = -(N-1)*rowStride/2` which is negative; the `findMinY + shiftY` pass corrects this so all output Y ≥ 0.
- **`TotalEffort` ignores parent-node effort.** Only leaf nodes (no children) contribute to the sum. A parent's `Effort` field is irrelevant — effort is meant to be estimated at the work-package level.

### Future sessions: append below
<!-- yyyy-mm-dd — short title -->
<!-- - one-line takeaway -->

### 2026-06-04 — Chart count audit: 19 → 20 everywhere; race + memory-scan clean
- **Registry has 20 chart kinds, not 19.** 6 DAG + 8 Stats + 4 Matrix + 2 Flow = 20. The off-by-one originated in the initial project scaffold comment before the 20th kind was wired up. All references to "19 chart kinds" in README.md (7 sites), DEVELOPER_HANDBOOK.md (3 sites), and `internal/charts/registry.go` package comment are now corrected to 20.
- **"Five engines" corrected to "four engines" in two places.** `registry.go` package comment and README.md both said "five engines"; only four Engine constants exist (DAG, Stats, Matrix, Flow). The five *renderer files* in `pdfrender/` (dag, fishbone, flow, matrix, stats) are correctly five because Fishbone has its own renderer file, but the taxonomy engine count is four.
- **`make race` passes clean** across all 28 packages — no data races detected.
- **`make memory-scan` passes clean** — `go vet` clean, goroutine inventory zero GoPMgr spawns, gosec clean, govulncheck reports zero vulnerabilities in GoPMgr's own code.
- **28 packages have test coverage; `sigma/domain` is intentionally excluded** (pure type constants and struct definitions — no logic to test).

### 2026-06-04 — Settings tests + UX hardening (Ctrl+S, dirty indicator, status dropdown, delete buttons, font/export settings)
- **`AgileEnabled` persistence shipped with only a `go build` check — now covered by unit tests.** `internal/db/settings_test.go` uses the existing `newBackupTestDB(t)` helper (same db package) and covers: defaults when no row exists (`ExportTheme=="modern"`, `AutoRepair==true`, `AgileEnabled==false`), full enable/disable roundtrip, `agile_enabled` column presence after migration, and all-field preservation on `SaveSettings`. Run with `go test ./internal/db/ -run TestSettings`.
- **Drop auto-save in CharterEditor — version inflation.** `SaveDocument` increments `version` monotonically on every call. Auto-saving on every keystroke would mint dozens of versions per typing session with no user value. Explicit save (button + Ctrl+S) is the right contract for documents.
- **Ctrl+S requires a `keydown` listener, not a global shortcut.** All three editor shells register `window.addEventListener('keydown', handleKeyDown)` in `onMount` and remove it in `onDestroy`. The handler calls `void save()` (chart shells) or `save()` (CharterEditor) on `Ctrl+S` / `Meta+S` with `e.preventDefault()` to suppress the browser's native save dialog.
- **Dirty tracking baseline must be set after content is parsed, not after the DB read.** `lastSavedContent = JSON.stringify(content)` is set in `onMount` after the `JSON.parse(doc.content)` step; using `doc.content` directly would differ from the re-serialised form and falsely flag clean documents as dirty on load.
- **Status dropdown calls `save()` immediately on change.** This is user-intentional (changing status is a deliberate action), so version increment is acceptable here unlike keystroke-level auto-save.
- **AgileEnabled: `AgileEnabled()` now returns `(bool, error)` and reads from DB.** `SetAgileEnabled(enabled bool)` returns `error` and persists via `GetSettings()+SaveSettings()`. The in-memory `agile.PackEnabled` is updated as a cache; functions that only need the pack state still read the cache for speed, while the DB is the source of truth on next open.
- **`settingsMigrations` loop replaces the single `default_font` migration block.** Adding a new settings column now requires one extra `{name, ddl}` struct in the loop — no other changes. The loop is in `db.Database.Migrate()` inside `migrateLegacyColumns`.
- **`svelte-check --fail-on-warnings` remains clean (0 errors, 0 warnings)** after all frontend changes in this session. Run before every commit.

### 2026-05-25 — PAdES ByteRange hardening
- **PAdES signing must be the final PDF mutation.** Render any visible signature block before calling `pdfmeta.InjectPAdESSignature`; appending a separate appearance PDF or injecting PDF/A metadata after signing leaves bytes outside the signed `/ByteRange`.
- **`/ByteRange` patching needs fixed-width space.** The signature dictionary now reserves a fixed-width `/ByteRange` slot and signs exactly the two declared ranges, excluding the complete `<...>` `/Contents` hex string. The regression test reconstructs those ranges from the final PDF and compares them to the callback input.
- **Invisible signature widgets still need widget shape.** The PAdES field now writes `/Subtype /Widget` with `/Rect [0 0 0 0]` and the AcroForm field reference, so readers see a concrete invisible signature field rather than only a detached signature dictionary.

### 2026-05-25 — Frontend compile recovery after signed-export/Sigma merge
- **`npm run check` is back to 0 errors.** The blocking failures were malformed signed-report state, stale component import paths, invalid Svelte 5 event modifier syntax, missing Wails ambient method/type declarations, Sigma route state using a nonexistent `session.viewId`, and Svelte 4-style Sigma props in runes-mode components.
- **Use `session.editingId` for routed record IDs.** `goto(view, editingId)` is the app's existing route contract; new feature views should not introduce parallel `viewId` fields unless the session model is deliberately changed everywhere.
- **Wails bridge declarations must track real `*App` methods.** Signed PDF/report exports, schedule report exports, ProjectMeta industry fields, and Sigma methods/types now live under `window.go.main.App` in `frontend/src/wails-window.d.ts`. Verify against the root `main.go` before adding names.
- **Remaining frontend debt is warning-level, not compile-blocking.** `svelte-check` still reports accessibility/deprecated-event warnings, especially in Sigma helper components and the signature modal. The production build also emits the existing large-chunk warning. Treat warning cleanup as a follow-up hardening slice.

### 2026-05-25 — veraPDF gate hardening
- **`scripts/validate-pdfa.sh` now has a testable helper layer.** `scripts/validate-pdfa-lib.sh` owns compliance-output parsing, Docker path mapping, portable veraPDF executable lookup, archive validation, and stale-wrapper detection; `scripts/validate-pdfa-lib_test.sh` covers those behaviors plus an integration path with a fake veraPDF CLI.
- **Do not grep text output for `compliant`.** That false-positives on "not compliant". The gate now requests XML and accepts only explicit `<isCompliant>true</isCompliant>` (or JSON `isCompliant: true` if a future runner emits JSON).
- **Generate validation samples inside the repo, not `/tmp`.** Docker receives `/work/...` paths for samples under `.tmp/gopmgr-pdfa-test`; CLI mode receives host paths. This matters because the GoPMgr workspace path contains spaces and Docker cannot see host-only `/tmp` paths unless mounted.
- **The sample generator must set `ExportOptions.Format`.** Missing `FormatPDF` made the old gate "pass" with no samples after `[EXPORT_FORMAT_UNKNOWN] unknown format ""`. Sample-generation failure is now a real gate failure; missing veraPDF tooling remains a soft skip.
- **Stale/corrupt veraPDF downloads are ignored.** The installer validates downloaded zip/jar files before accepting them and refreshes wrapper scripts that point at invalid jars. On this machine, Docker is absent and auto-install still cannot fetch a valid veraPDF artifact, so `make check-pdfa` skips cleanly rather than validating.

### 2026-05-25 — Frontend stability/performance hardening
- **Keep `xlsx` lazy-loaded in the Sigma import flow.** `SigmaProjectView.svelte` now imports `xlsx` only inside the spreadsheet-import path, so Vite splits it into `dist/assets/xlsx-*.js` instead of forcing every GoPMgr launch to parse the spreadsheet engine.
- **`scripts/frontend-stability-check.sh` protects this boundary.** The guard fails on static Sigma `xlsx` imports, deprecated Svelte 4 `on:*=` directives in Sigma components, `createEventDispatcher` usage in Sigma components, and SVG text actions without keyboard handlers in `SigmaFishbone.svelte`.
- **Sigma save notifications use Svelte 5 callback props.** `SigmaVoCCTQ`, `SigmaSIPOC`, `SigmaSolutionMatrix`, and `SigmaControlPlan` expose optional `onSaved` callbacks instead of dispatching legacy component events; parent calls should pass function props such as `onSaved={loadCharter}`.
- **Frontend warnings are now a hard gate.** `scripts/frontend-stability-check.sh` runs `svelte-check --fail-on-warnings`; future Svelte diagnostics must be fixed rather than tolerated. Current `npm run check` from `frontend/` reports 0 errors and 0 warnings.
- **Route-level feature islands are lazy-loaded from `App.svelte`.** App no longer eagerly imports every chart, document, Agile, project, and Sigma component at launch. The current production build has no Vite large-chunk warning; `index` is roughly 48 kB minified / 19 kB gzip, with heavy surfaces split into route chunks plus `StatsChart` (~188 kB) and `xlsx` (~429 kB) async chunks.
- **`scripts/frontend-build-budget.sh` protects the split.** It runs the production build and fails if Vite emits a large-chunk warning or if the main `index-*.js` chunk exceeds 500,000 bytes. Prefer lazy route/component splits over raising the Vite warning limit.

### 2026-05-25 — Release gate scope and deterministic build hardening
- **Do not use the unscoped all-packages pattern for Go quality gates in this repo.** With `frontend/node_modules` installed, it discovers npm dependency packages such as `frontend/node_modules/flatted/golang/pkg/flatted`. Use `. ./internal/...` for GoPMgr-owned Go gates.
- **`scripts/release-gate-scope-check.sh` protects release wiring.** It fails on unscoped Go quality commands and requires `check-release.sh` to include the frontend stability and bundle-budget gates.
- **Optional scanners are advisory by default.** `memory-safety-scan.sh` still runs detected `staticcheck`, `gosec`, and `govulncheck`, but only mandatory checks fail by default. Set `GOPMGR_STRICT_OPTIONAL_SCANS=1` for security-focused strict runs. This avoids release-gate behavior changing just because one developer has `gosec` installed.
- **Wails CLI builds require the main package at the repo root.** GoPMgr's entrypoint was moved from `cmd/gopmgr/main.go` to `./main.go` (with its `*_test.go` files) so `make build` can run `wails build` directly. `wails build` builds the frontend into the repo-root `frontend/dist`, embeds it via the root `go:embed`, generates the Wails bindings, injects the `desktop,production` tags, and links the platform frameworks (on macOS, `UniformTypeIdentifiers` for `UTType`) - the work the old hand-rolled `go build ./cmd/gopmgr` had to replicate (and which failed at runtime without the tags and at link without the framework). Install the CLI with `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0` to match the pinned project version. `scripts/wails-build.sh` removes extended attributes from the completed bundle and ad-hoc signs it after Wails returns; this keeps binding generation enabled while preventing macOS provenance/resource-fork metadata from breaking bundle verification.
- **`check-release.sh` now runs the complete local release gate successfully on this machine.** It verifies scope, memory safety, frontend warning-clean state, frontend bundle budget, race detector, deterministic build, and the PDF/A soft gate. `reuse` still skips if the tool is not installed.

### 2026-05-26 — Deterministic package targets
- **Package targets now use `scripts/package.sh`, not Wails CLI packaging.** The script calls the proven `make build` path, stages `gopmgr` with `README.md` plus `LICENSES/`, and writes `build/packages/gopmgr-<goos>-<goarch>.tar.gz`.
- **Packaging is host-local by design.** `package-darwin` runs on macOS; `package-linux` and `package-windows` fail fast with a clear message unless run on matching hosts/CI runners. This avoids pretending that CGO/Wails cross-packaging is portable from one desktop machine.
- **`scripts/release-gate-scope-check.sh` also rejects Wails CLI package invocations.** Future package target edits should keep using the deterministic script unless the repo intentionally reintroduces app-bundle packaging with a verified root-main Wails layout.

### 2026-05-26 — Strict gosec and Sigma persistence hardening
- **Strict optional scanners are now clean on this machine.** `GOPMGR_STRICT_OPTIONAL_SCANS=1 make memory-scan` passes with gosec installed; normal `make memory-scan` remains clean. Keep any future `#nosec G304` comments narrow and tied to a real product boundary, such as user-selected certificate/export/font paths or `os.CreateTemp` paths created by GoPMgr itself.
- **Sigma persisted JSON must fail loudly when corrupt.** `SigmaGetCharter`, `SigmaGetFishbone`, `SigmaGetSolutions`, `SigmaGetControlPlan`, `SigmaGetSIPOC`, and `SigmaGetVoC` now return contextual decode errors instead of silently treating malformed JSON as empty domain data. The regression tests insert corrupt JSON directly into SQLite so the failure mode stays covered.
- **Fishbone storage shape is full `FishboneData`, not bare branches.** `SigmaSaveFishbone` writes the full object; `SigmaGetFishbone` now reads that shape and preserves the legacy bare-`[]FishboneBranch` fallback. Without this, saved causes could disappear on reload because the previous getter ignored the unmarshal error.
- **Argon2 PHC parsing must validate bounds before calling `argon2.IDKey`.** Malformed hashes with `p=256`, zero parameters, empty salt, or empty key material can otherwise panic or truncate during conversion. Keep these checks before the `uint8` / `uint32` conversions.
- **Export and account artifacts should default private.** Sigma reports, audit CSV exports, backup bundles, the Sigma export directory, and the GoPMgr system root now use `0600`/`0700` permissions where GoPMgr owns the write path. Per-user subdirectories already used `0700`; the root now matches the isolation claim in §5.

### 2026-05-26 — Backup and audit artifact durability
- **Never string-interpolate `VACUUM INTO` paths.** A backup/snapshot destination containing a single quote used to fail with a SQLite syntax error. `CreateSnapshot` now binds the target path as a SQLite parameter, and regression tests cover both direct snapshots and `.pmba` archival bundles with quoted destination names.
- **Archival writers must finalize explicitly.** `CreateArchivalBundle` now returns errors from `zip.Writer.Close`, archive-file close, and source-file close when those are the first failure. A backup function returning nil means the zip central directory and underlying file close both completed.
- **Audit CSV export now checks flush and close errors.** `ExportAuditCSV` explicitly flushes, checks `csv.Writer.Error`, checks row iteration, and returns close errors when no earlier error occurred. The regression test verifies a private `0600` CSV with comma/newline escaping intact.

### 2026-05-26 — Update-channel fail-closed hardening
- **Manifest URLs must be HTTPS.** `CheckLatest` now rejects configured non-HTTPS or hostless manifest URLs before issuing a network request, matching the package threat model that the signed release manifest is fetched over HTTPS. Tests cover the fail-closed status path.
- **Manifest bodies are bounded explicitly.** `readManifestBody` reads at most `maxManifestBytes + 1` and returns a clear "manifest too large" error if the server exceeds 64 KiB, rather than passing a silently truncated body into signature verification. Keep this limit check before `VerifyManifest`.

### 2026-05-26 — Existing directory permission repair
- **`MkdirAll(path, 0700)` is not enough for privacy.** It applies the mode only when the directory is newly created; existing `0755` GoPMgr roots or per-user folders stayed too broad. `users.ensurePrivateDir` now runs `MkdirAll` and then `Chmod(0700)` for the system root plus each account's `projects`, `certs`, and `exports` directories.
- **Directory-mode gosec suppressions must explain directory semantics.** `#nosec G302` is acceptable on `Chmod(..., 0700)` only where the target is a private directory; files should remain `0600` or stricter.

### 2026-05-26 — Recovery-code paste tolerance
- **Recovery-code canonicalisation must strip all whitespace, not just spaces.** Users often paste backup codes with tabs, newlines, or wrapped clipboard text. `canonicalise` now removes Unicode whitespace plus dashes and uppercases before Argon2 verification; the regression test exercises lower-case pasted codes with tabs/newlines.

### 2026-05-26 — SQLite file permission repair
- **Private directories do not guarantee private SQLite files.** `sql.Open` creates `system.db` and `.pmforge` files using the process umask, which can leave them `0644` even inside `0700` directories. `InitDB` and `users.Open` now explicitly chmod the main database file plus existing `-wal`/`-shm` sidecars to `0600` after migration.
- **Repair existing database file modes on open.** Tests cover both new and pre-existing broad `0644` files so upgrades tighten old installs as well as fresh databases.

### 2026-05-26 — Self-heal swap preflight hardening
- **Do every non-mutating `SwapInSnapshot` preflight before closing the live DB.** The swap path now rejects missing, non-regular, or SQLite-invalid `.bak` snapshots before touching the live handle, so bad recovery artifacts leave the current database open and usable.
- **Stale `.corrupt` cleanup must fail loudly.** A non-removable existing forensic path now returns a contextual `clear stale corrupt` error before the live file is moved aside, rather than surfacing a later rename failure after the connection is closed.
- **Rollback failures need to be visible.** If the snapshot rename fails after the live DB has moved to `.corrupt`, the rollback attempt is still made and any rollback error is included in the returned error instead of being discarded.

### 2026-05-26 — ID entropy failure hardening
- **Do not use `crypto/rand.Read` in recoverable code paths on Go 1.26.** In this toolchain it fatals the process if the reader fails. GoPMgr's DB and Agile ID generators now use `io.ReadFull(rand.Reader, ...)` and return contextual errors instead of crashing or emitting zero IDs.
- **Generated IDs are part of persistence correctness.** `UpsertProject`, chart/document/stakeholder saves, and Agile board/column/work-item/sprint/deployment saves now abort when entropy is unavailable, so a failed CSPRNG cannot create predictable or colliding primary keys.
- **Tests should force entropy failure through `crypto/rand.Reader`.** The regression tests replace the reader with an erroring source and assert that persistence APIs fail before any write that would rely on a generated ID.

### 2026-05-31 — Agile default board self-repair
- **`EnsureDefaultBoard` must repair missing standard columns on existing boards.** A default board row can survive a partial seed, manual table edit, or interrupted migration while its `todo`/`doing`/`review`/`done` columns are incomplete. The store now replays idempotent column inserts before returning the board.
- **Default board creation should be transactional.** Board and column seeding now happen in one transaction so a new default board is not committed without its standard columns.
- **Do not overwrite customized columns during repair.** Missing defaults are inserted with `ON CONFLICT DO NOTHING`, preserving an existing column's name, order, and WIP limit.

### 2026-05-31 — Recoverable entropy reads
- **Use `io.ReadFull(rand.Reader, ...)` for recoverable random-byte generation.** `crypto/rand.Read` can fatal the process on this Go toolchain when the reader fails, so password salts, recovery codes, DB IDs, and Agile IDs now use `io.ReadFull` and return contextual errors instead.
- **Keep signing on signer APIs.** `rsa.SignPKCS1v15(rand.Reader, ...)` already reports entropy/signature failures as an error, so it is not the same hazard as direct `rand.Read`.
- **Entropy-failure tests should assert errors, not zero output.** The auth and recovery-code tests replace `crypto/rand.Reader` with an erroring source and require `HashPassword` / `generateCode` to return their existing contextual errors.

### 2026-05-31 — Authentication persistence errors
- **Successful authentication must not hide post-auth write failures.** `Authenticate` now returns contextual errors if `last_login` cannot be updated, matching its documented behavior and surfacing system database write faults.
- **Transparent password rehash is a persistence operation, not best-effort logging.** If a stored hash needs stronger Argon2id parameters, entropy-generation or `password_hash` update failures now return errors instead of silently leaving the weaker hash in place.
- **SQLite triggers are useful durability test fixtures.** The auth regression tests use `RAISE(ABORT, ...)` triggers to force specific metadata-write failures without corrupting the database file or relying on platform permissions.

### 2026-05-31 — Atomic backup publication
- **Do not create the destination `.pmba` until snapshot preparation succeeds.** `CreateArchivalBundle` now clears and creates the SQLite snapshot before opening any archive output, so a blocked stale temp snapshot cannot leave an empty backup file behind.
- **Publish backups through a side-by-side temp archive.** The zip is written to `<dest>.tmp.archive`, explicitly closed, and only then renamed into place. Cert/manifest/zip failures leave no destination archive for users or automation to mistake as valid.
- **Temp cleanup errors matter only on success.** Snapshot cleanup is returned if it is the only failure; temp archive cleanup is best-effort after an already-failed backup so the primary user-facing error is preserved.

### 2026-06-04 — Document create→edit→export loop (all 25 kinds)
- **All 25 document template items in the Dashboard are now clickable.** The "Available document templates" list was non-interactive `<li>` text. Each item is now a `<button>` that calls `NewDocument(kind, name)` and routes to the document editor. The new `newDocument(kind, title)` helper in `Dashboard.svelte` routes to the `'documents'` view; the pre-existing `newCharter()` keeps routing to `'charter'` for the featured card.
- **`App.svelte` now has a `documents` route loader** that points to `CharterEditor.svelte`. Previously, only `charter` and `report_composer` were wired; any non-charter document opened from the existing-documents list fell to the "no editor" fallback screen. The `CharterEditor` component is already fully generic — it fetches the document by `session.editingId`, looks up the `DocumentDefinition` by `doc.kind`, and renders all fields via `DocumentFieldEditor` — so pointing `documents` at it costs one route-loader line.
- **DOCX and ODT export buttons are now in the CharterEditor header.** Backend methods `ExportDocumentDOCX` / `ExportDocumentODT` existed since 2026-05-16 but had no frontend entrypoint. Added `exportDOCX()` / `exportODT()` functions (same save-then-export pattern as `exportPDF()`) and two header buttons alongside the existing PDF and Signed PDF buttons.
- **Excel-alias fallback was hardcoded to `charter_word` — fixed.** `CharterEditor.onMount` had `all.find(d => d.kind === 'charter_word')` as the fallback for a definition with empty fields. There are **two** empty-fields Excel aliases: `charter_excel` and `plan_excel`. The hardcoded fallback would load charter fields for any `plan_excel` document, causing silent data corruption. Fixed to derive the sibling word-kind from the current kind: `doc.kind.endsWith('_excel') ? doc.kind.replace('_excel', '_word') : null`. The guard also tightens the condition to only trigger on `_excel` kinds, so non-Excel kinds with hypothetically empty fields do not fall through.

### 2026-06-04 — User font directory privacy repair
- **Imported font storage must repair existing directory modes.** `ImportFont` now uses `ensurePrivateDir` for the user font directory, so a pre-existing broad `0755` directory is tightened to `0700` before user-supplied font files are copied into it.
- **Test existing directories, not only fresh installs.** The font regression creates a broad directory first, imports a `.ttf`, and verifies the directory mode is repaired. Keep this pattern for privacy-sensitive local storage paths where `MkdirAll(..., 0700)` alone does not upgrade old installs.

### 2026-06-05 — Sigma report export directory privacy repair
- **Sigma report exports must repair existing export directory modes.** `GenerateSigmaReport` writes PDFs as `0600`, but `getExportDir` previously left a pre-existing broad `$HOME/GoPMgr/exports` directory untouched. It now chmods the directory back to `0700` after `MkdirAll`.
- **Keep gosec suppressions directory-specific.** `#nosec G302` is acceptable on the Sigma export directory chmod because the target is a private directory. The report file itself remains `0600`, and the regression covers the upgrade path from an existing `0755` directory.

### 2026-06-05 — Secure archive audit fail-closed
- **SecureArchive success requires a durable `ARCHIVE_CREATED` audit row.** If the archive bundle is written but the success audit insert fails, `SecureArchive` now removes the just-created archive and returns the audit error instead of reporting success with an unaudited artifact.
- **Use SQLite triggers for audit-failure regressions.** The admin regression blocks only `ARCHIVE_CREATED` inserts, calls the real archive workflow in a temp working directory, and verifies no `PMForge_Archive_*.pmba` file is left behind after the forced audit failure.

### 2026-06-06 — PAdES external validator hardening
- **CAdES/PAdES CMS needs `SigningCertificateV2` for Poppler validation.** OpenSSL verified the detached CMS without it, but `pdfsig` reported the signature invalid until `Signer.SignPDFCMS` added the RFC 5035 `signingCertificateV2` signed attribute binding the signer cert hash plus issuer/serial into the signed attributes.
- **External validator harnesses must fail on validator failures through `tee`.** `scripts/validate-pades-external.sh` now uses `pipefail`; `qpdf --check` failure and missing `pdfsig` valid-signature output are hard failures instead of being masked by the report pipe.
- **The local signed sample must be a syntactically valid PDF, not only ByteRange-verifiable bytes.** The generated sample now has a real one-page Pages tree so `qpdf --check` validates the same artifact used for CMS and `pdfsig` checks.

### 2026-06-06 — PDF/A-3b schedule gate hardening
- **Use the installed veraPDF before attempting stale auto-downloads.** `scripts/validate-pdfa.sh` now prefers `verapdf` on `PATH`, then falls back to the `/tmp` wrapper/download path. The helper test injects a fake CLI through `PATH` so it remains hermetic.
- **Validate the intended profile explicitly.** The gate now calls veraPDF with `-f 3b`; otherwise veraPDF can default to PDF/A-1b and report irrelevant failures, including embedded-file restrictions that are valid for PDF/A-3.
- **Incremental updates must rewrite from the latest object revision.** `MakePDFA3` injects XMP, then OutputIntent; `findObjectBody` must return the latest Catalog object or the second rewrite drops `/Metadata`.
- **PDF/A stream lengths exclude the EOL marker before `endstream`.** Metadata and ICC streams now always write a separate EOL before `endstream`, so `/Length` matches the payload bytes veraPDF counts.
- **gofpdf schedule reports need PDF/A post-processing beyond XMP.** `MakePDFA3` now adds the required binary header comment and trailer `/ID`; schedule PDF exports register bundled Source Sans 3 as the Helvetica alias when the font assets are available, avoiding core-font PDF/A failures.
- **Representative PDF/A samples should use public export APIs.** `scripts/validate-pdfa.sh` now generates a schedule report through `export.GenerateArchivalReport`, a standalone charter through `documents.Render`, a combined report through `documents.BuildCombinedReport`, and a Monte Carlo risk report through `export.GenerateMonteCarloRiskReport`, all with Source Sans 3 registered where needed.

### 2026-06-06 — V2 encryption-at-rest stopgap
- **Historical note, superseded 2026-06-13.** This stopgap said not to imply GoPMgr encrypted `.pmforge` databases at rest. That was correct before SQLCipher landed; current release docs now state the implemented behavior: new project DBs are SQLCipher-encrypted, Settings can migrate existing plaintext DBs after recovery-code reissue, and OS-level disk encryption remains whole-device defence in depth.
- **Guard release security claims with a cheap textual gate.** `scripts/release-gate-scope-check.sh` now fails if README says SQLCipher/native database encryption is still deferred, if `go.mod` lacks `github.com/mutecomm/go-sqlcipher/v4`, or if README stops documenting SQLCipher-encrypted per-user `.pmforge` project databases.

### 2026-06-06 — Timeline date-dragging
- **Keep timeline editing scoped to real timeline boundaries.** `MoveTimelineEntry` updates project start/end and sprint start/end dates, returns a rebuilt timeline, and rejects deployment moves because deployments are DORA history.
- **Expose editability from the backend.** `timeline.Entry` now carries `editable` and `edit_field`; the Svelte view does not infer write permissions from labels or colors.
- **The build/ ignore must keep the Wails scaffold trackable.** `.gitignore` ignores everything under `build/` except `build/darwin/Info.plist` and `build/darwin/Info.dev.plist` (the Wails macOS bundle scaffold, which sets `CFBundleIdentifier dev.gopmgr.GoPMgr`). Compiled output (`build/bin`, `build/packages`) stays ignored. `make clean` therefore deletes `build/bin`/`build/packages` but never the tracked scaffold.
- **Generated embed output needs no special handling now.** The root `main.go` embeds the repo-root `frontend/dist`, which is gitignored; `reuse lint` skips gitignored paths, so no pre-clean is required. `wails build` regenerates `frontend/dist` on each build.

### 2026-06-07 — veraPDF PAdES feature extraction
- **veraPDF is a useful PAdES feature extractor, not the primary signature-validity oracle.** `scripts/validate-pades-external.sh` now runs `verapdf --off --extract signature --format xml` and checks for `Adobe.PPKLite` plus `ETSI.CAdES.detached`; `pdfsig` remains the local validity gate for `Signature Validation: Signature is Valid`.
- **Keep verbose validator artifacts out of the report body.** veraPDF includes the padded CMS contents in feature output, so the harness writes the XML to `.tmp/gopmgr-pades-test/verapdf-signature-features.xml` and records only the pass/fail line plus artifact path in the human report.
- **Use fake-validator tests for optional external tools.** `scripts/validate-pades-external_test.sh` injects a fake `verapdf` through `PATH`, proving the branch runs deterministically even on machines without the real CLI.
- **PAdES validation scripts share generated state and need coordination.** `validate-pades.sh` recreates `.tmp/gopmgr-pades-test`; external validators read from that same directory. Both scripts now use `.tmp/gopmgr-pades-test.lock`, and `scripts/validate-pades-parallel_test.sh` guards concurrent local/external runs.

### 2026-06-07 — DSS PAdES baseline-B validation
- **PAdES baseline-B forbids CMS `signing-time`.** `internal/crypto/pdf_cms.go` now builds GoPMgr's detached CMS directly so the signed attributes include `contentType`, `messageDigest`, and `SigningCertificateV2`, but omit CMS `signing-time`.
- **The PDF signature dictionary still needs `/M`.** `pdfmeta.InjectPAdESSignature` writes `/M (D:YYYYMMDDHHmmSSZ)` into the signed byte range. This first allowed DSS to classify the fixture as Baseline B; the timestamped fixture is now classified as Baseline T.
- **DSS is an executed external validator when installed.** `scripts/validate-pades-external.sh` runs `dss-validation-tool validate`, records `.tmp/gopmgr-pades-test/dss-validation-output.txt`, fails on DSS PAdES baseline warnings, and now requires `signature.format=PAdES-BASELINE-T` when the wrapper emits that field. `NO_CERTIFICATE_CHAIN_FOUND` remains expected for the self-signed signer and TSA.
- **Release docs should not regress to stale DSS TODOs.** `scripts/release-gate-scope-check.sh` requires README and this handbook to mention the current DSS `PAdES-BASELINE-T` fixture result and rejects old wording that treats DSS as unrun.

### 2026-07-25 — RFC 3161 timestamp client foundation

- **Keep timestamp protocol validation separate from PAdES mutation.**
  `internal/rfc3161.Client` accepts the digest of the existing CMS signature
  value, returns a validated raw timestamp token, and does not modify PDF or
  CMS bytes. The next integration must add the token as the
  `signatureTimeStampToken` unsigned CMS attribute after Baseline-B signing.
- **A valid token is not automatically a trusted token.** CMS signature,
  TSTInfo content type, nonce, imprint, policy, generation time, and
  timestamping-only critical EKU checks always run. `TrustStatus` remains
  `not_evaluated` unless the caller supplies roots and chain verification
  succeeds; UI and audit wording must preserve that distinction.
- **Treat a configured TSA as a strict network boundary.** Requests require an
  absolute HTTPS URL, never follow redirects, use the RFC MIME types, carry a
  128-bit nonce, and reject non-success HTTP status, malformed media types,
  oversized replies, protocol rejection, trailing ASN.1 data, and tampering.
  Tests inject the transport and entropy source, so CI never depends on a live
  TSA.

### 2026-07-25 — PAdES-T CMS timestamp embedding

- **Hash the signature OCTET STRING, not the PDF or complete CMS.**
  `crypto.SignatureTimestampImprint` parses GoPMgr's detached, single-signer
  CMS and returns SHA-256 over `SignerInfo.signature`, matching RFC 5126.
- **Timestamping must not invalidate the original signature.**
  `crypto.AddSignatureTimestamp` revalidates the returned token, appends the
  DER `id-aa-signatureTimeStampToken` unsigned attribute, sorts unsigned
  attributes canonically, and preserves signed attributes, signature bytes,
  certificates, and detached content. Multiple independent TSA tokens remain
  supported.
- **Reserve the PDF signature slot before signing.**
  `pdfmeta.InjectPAdESSignature` now reserves 32 KiB rather than 8 KiB because
  an RFC 3161 token and TSA chain are inserted only after the ByteRange layout
  is fixed. The local fixture passes OpenSSL, qpdf, pdfsig, and veraPDF; DSS
  classifies it as `PAdES-BASELINE-T`. The self-signed fixture remains
  indeterminate for trust, which is expected and must not be presented as a
  trusted timestamp.

### 2026-07-25 — Fail-closed application PAdES-T integration

- **Treat timestamp settings as non-secret trust policy.** Project settings
  persist only the opt-in flag, credential-free HTTPS endpoint, optional policy
  OID, and optional PEM trust-root path. User-info, query strings, and fragments
  are rejected so API keys cannot leak into project databases or logs.
- **Use one PAdES pipeline for every application export.**
  `signing.ApplyPAdES` signs Baseline B when timestamping is disabled and
  requests, validates, and embeds the RFC 3161 token when enabled. Document and
  combined-report exports both use it after PDF/A rendering.
- **Never downgrade an explicit timestamp request.** A TSA transport, policy,
  trust, token, or embedding failure returns no PDF bytes. Audit checkpoints
  record `pades_b_signed`, `pades_t_not_evaluated`, or `pades_t_verified` so
  cryptographic validity is not confused with configured chain trust.
- **Keep CI independent of public TSAs.** The signing pipeline uses the narrow
  `TimestampRequester` interface. Tests inject a deterministic token producer
  and cover successful Baseline B/T results plus the fail-closed error path.

### 2026-07-25 — Legacy PDF signature fallback retirement

- **A CMS blob in a PDF comment is not a PAdES signature.**
  `internal/export` no longer appends `%%GoPMgrCMSSignature` when real
  `/Sig` embedding fails. Certificate, CMS, or PDF mutation failures return an
  error and no bytes.
- **Compatibility helpers still use the canonical pipeline.**
  `documents.RenderSigned` retains its Baseline B contract but delegates to
  `signing.ApplyPAdES` after the normal `documents.Render` PDF/A pass. This
  removes duplicated metadata/signature sequencing without changing the public
  function signature.
- **Test structure, not a success flag.** Regression tests inject in-memory
  signers and require `/Type /Sig`, `/ByteRange`, and
  `/SubFilter /ETSI.CAdES.detached`. Separate missing-key tests prove both
  compatibility paths return no bytes instead of producing a marker fallback.

### 2026-07-26 — Application-level PAdES-T export contract tests

- **Test the application orchestration, not only the signing primitive.**
  `app_pades_export_test.go` drives both signed document and combined-report
  exports through rendering, Baseline T mutation, private file creation, and
  audit checkpoint writes.
- **Keep test substitution local to one export call.** The private
  `padesExportRuntime` replaces only certificate loading and timestamp
  preparation. It is passed to private worker methods rather than stored on
  `App` or in mutable package globals, so concurrent Wails requests cannot
  observe a test hook.
- **Require cryptographic structure and business evidence together.** Tests
  require `/Sig`, `/ByteRange`, the ETSI CAdES subfilter, and the CMS
  `id-aa-signatureTimeStampToken` attribute in the written 0600 file. They
  separately require `pades_t_verified` for configured trust and
  `pades_t_not_evaluated` when no TSA root is configured.
- **Keep CI offline and deterministic.** In-memory signing and TSA
  certificates produce a token bound to the actual CMS signature imprint.
  No certificate store, network timestamp authority, or shared test state is
  required.
- **Do not give fixed protocol instants fixed certificate lifetimes.** The
  RFC 3161 fixtures keep deterministic TSTInfo timestamps, but derive test
  certificate validity from both that instant and the current CMS signing
  clock. This prevents calendar-driven expiry from masking the validation
  branch each test intends to exercise.

### 2026-07-26 — Fresh external PAdES evidence provenance

- **Default validation must regenerate, not merely find, its fixture.**
  `make check-pades-external` now regenerates the timestamped PDF inside the
  existing PAdES lock on every run. A non-empty artifact from an earlier
  checkout can no longer be reported as current-build evidence.
- **Explicit input is a separate, non-mutating mode.**
  `validate-pades-external.sh /path/to/file.pdf` resolves and validates the
  supplied PDF without invoking the local generator. The harness clears only
  its known derived artifacts first, preventing stale validator output while
  preserving the input. Reports distinguish `generated_current_checkout` from
  `supplied_pdf`.
- **Bind evidence to bytes and implementation state.** Reports include the
  validation UTC time, repository revision and dirty state, SHA-256 hashes for
  the validator and generator scripts, and hashes for the PDF, extracted CMS,
  and signed ByteRange content.
- **Keep assertions inside the same critical section.** The shell regression
  test owns the shared lock across stale-fixture setup, generated and supplied
  validation, and report assertions. Child scripts inherit the lock marker,
  preventing a waiting generator from deleting evidence between execution and
  inspection.

### 2026-07-26 — Trusted-source PAdES outcome hardening

- **Never collapse unknown trust into `PASS`.** The trusted-source report now
  uses `TRUST_VERIFIED` only when `pdfsig` explicitly confirms certificate
  trust. A valid signature without that proof is
  `STRUCTURE_VALID_TRUST_INDETERMINATE`; missing required `qpdf`/`pdfsig`
  validators and failed validators have separate `VALIDATION_INCOMPLETE` and
  `VALIDATION_FAILED` outcomes.
- **Required mode means verified trust, not merely configured input.**
  `GOPMGR_PADES_TRUSTED_REQUIRED=1` fails for `NOT_CONFIGURED`, indeterminate
  trust, incomplete validation, invalid input, and validation failure.
- **Treat trusted evidence as one locked, reproducible snapshot.** The harness
  resolves the supplied path, hashes the unchanged PDF and validator, records
  checkout/UTC provenance, clears known derived artifacts under a dedicated
  lock, and atomically publishes the report. The regression test holds that
  lock through its fake-validator matrix and evidence assertions.
- **Keep Acrobat evidence independent.** Local CLI trust reflects the
  certificate store and policy available to `pdfsig`; the report continues to
  require a separately archived Acrobat trust-panel capture for release
  interoperability evidence.

### 2026-07-26 — Automatic PAdES harness regressions

- **Run the evidence harnesses automatically, not only during manual
  hardening.** `make pades-harness-tests` combines the real local
  CMS/RFC-3161/PDF generator with the external-validator, parallel-locking, and
  trusted-source shell regression matrices.
- **Keep CI deterministic without weakening real validation.** The dedicated
  Ubuntu CI job installs `qpdf` and `pdfsig` for the generated sample. The
  regressions provide controlled veraPDF and DSS command output so parser and
  classification branches do not depend on network downloads or public trust
  stores.
- **Guard every integration point.** `make check-release` now runs the
  aggregate target, while `make release-scope` requires the Make target plus
  both release and CI invocations. `check-pades-trusted` and the aggregate
  target are explicitly phony so same-named filesystem entries cannot suppress
  execution.
- **Do not turn a hermetic test into a trust claim.** The trusted-source matrix
  proves status handling only. A public trust claim still requires a real
  release-certificate sample and separately archived Acrobat evidence.

### 2026-07-26 — Tag-triggered release preflight

- **A green branch run is not a tag gate.** The Release workflow is triggered
  independently by `v*` tags, so its package matrix now depends on a blocking
  `preflight` job that checks out the tagged commit and runs the full
  `make check-release` gate before any installer is built or uploaded.
- **Bind package versions to the embedded application version.**
  `check-release-tag.sh` accepts the exact GA tag or a valid SemVer prerelease
  of the CLI/`wails.json` version. It rejects mismatched bases, build metadata,
  empty identifiers, and numeric prerelease identifiers with leading zeroes.
- **Pin tools and fail closed.** The Ubuntu preflight installs Wails v2.13.0,
  REUSE 6.2.0, `qpdf`, and `pdfsig`, then places the pipx user bin directory on
  the following steps' path. The full gate therefore cannot silently skip
  licensing because `reuse` was installed but undiscoverable.
- **Pin the PDF/A container to the official CLI distribution.** The inherited
  gate previously referenced the mutable and noncanonical
  `verapdf/verapdf:latest`. It now binds `verapdf/cli:v1.30.2` to the same
  `VERAPDF_VERSION` used by the direct-download fallback, and the helper
  regression rejects future `latest` drift. `make check-release` runs that
  hermetic Docker/output-parser regression before strict conformance validation
  so the contract is exercised even on hosts that use a local veraPDF CLI.
- **Guard the workflow dependency, not just the job's presence.**
  `make release-scope` requires the tag-aware command and verifies that the
  package matrix declares `needs: preflight`; leaving an unused preflight job
  in the workflow cannot satisfy the release guard.

### 2026-07-26 — Configuration format and parsing gate

- **Use the consumer's native format rather than a repository-wide
  preference.** GitHub Actions, Dependabot, and nFPM remain YAML; Gitleaks and
  REUSE remain TOML. Golangci-lint supports both, but its established YAML
  file stays aligned with the tool's primary examples.
- **Parse what release automation will consume.** `make config-check` tests
  malformed and duplicate-key cases, parses the seven tracked configuration
  files, validates their essential top-level contracts, and rejects
  unclassified YAML/TOML additions. `make verify` and `make check-release`
  both run the gate.
- **Keep one CI authority.** The stale `.gitlab-ci.yml` was removed after the
  repository and handbook had already moved to GitHub Actions. The inventory
  guard rejects its accidental reintroduction instead of leaving an
  unmaintained pipeline that can diverge from release behavior.
- **Keep configuration parsers out of the product binary.** The YAML and TOML
  packages are used by the standalone `scripts` command only; GoPMgr project
  data and report inputs/outputs do not gain a new serialization format.

### 2026-07-26 — Reproducible native installer tools

- **Pin fetched packaging tools in one reviewable record.**
  `scripts/release-tool-versions.env` binds nFPM v2.47.0 and the Chocolatey
  NSIS package at 3.12.0. The release workflow loads that record instead of
  selecting `@latest` or an unversioned package.
- **Verify the binary/package that actually reached the runner.** A binary
  built by `go install` reports `dev` through `nfpm --version`, so Linux checks
  Go's embedded module metadata instead. Windows checks Chocolatey's exact
  installed package record and confirms `makensis` is discoverable before
  Wails starts.
- **Do not install an unused macOS dependency.** GoPMgr's dependable tag path
  is the staged `hdiutil` image. `create-dmg` drives Finder through AppleScript
  and remains an explicit `GOPMGR_FANCY_DMG=1` local option; removing its
  unconditional Homebrew install reduces release network and UI-automation
  inputs.
- **Test the failure classes, then guard the integration points.**
  `make installer-tool-pins` mutates isolated workflow fixtures for mutable,
  missing, invalid, and bypassed pins, then checks the live files.
  `make verify`, `make check-release`, and `make release-scope` keep the
  contract in ordinary commits and tag preflight. Native RC installation is
  still required because a source guard cannot prove hosted-runner packaging.

### 2026-07-26 — Published release history correction

- **GitHub is the authority for published releases.** Live tag and release
  inspection found only `v0.9.1-alpha`; two unpublished local tags both pointed
  to commits already reachable from `main`, so the stale refs were removed
  without losing history.
- **Draft notes are not release evidence.** The unpublished draft release note
  and public claims that its native pipeline passed were removed.
  README and ROADMAP now distinguish the published alpha from unreleased 1.1.0
  development.
- **Keep prerelease instructions symbolic until publication.**
  `check-release-reference-truth.sh` rejects concrete candidate identifiers and
  candidate release-note filenames unless they appear in
  `docs/published-release-tags.txt`, which is updated only after live GitHub
  verification. Its isolated Git fixtures prove symbolic guidance passes, both
  false-reference classes fail, and a recorded publication is allowed; `make
  release-scope` runs the test and live check.

### 2026-07-26 — Deterministic Windows NSIS scaffold

- **Track authored inputs, not generated payloads.** GoPMgr now owns
  `build/windows/installer/project.nsi`, `build/windows/info.json`, and
  `build/windows/wails.exe.manifest`. `.gitignore` continues to exclude the
  derived icon, WebView2 bootstrapper, binaries, and `wails_tools.nsh`; the
  pinned Wails v2.13.0 CLI regenerates that macro file for current project
  metadata.
- **Preserve user data on uninstall.** The branded NSIS flow displays the GPL,
  installs shortcuts and Wails associations, and removes only the installed
  program plus disposable WebView cache. Inline comments and a regression
  reject removal of the user's Documents/PMForge tree or `.pmforge` files.
- **Windows had silently omitted embedded analytics.** Unlike the Linux and
  macOS legs, its Wails command lacked `-tags duckdb`. The workflow now embeds
  DuckDB and runs `verify-duckdb-linked.sh` against `gopmgr.exe` before
  collecting the installer.
- **Separate syntax evidence from native evidence.**
  `make windows-installer-scaffold` runs isolated mutation fixtures and, when
  NSIS is installed, compiles a harmless installer against the pinned Wails
  macros. This proves template compatibility but not MinGW/CGO linkage,
  installation, first-run account creation, or uninstall behavior on Windows.

### 2026-07-27 — GitHub prerelease classification

- **SemVer suffixes do not classify GitHub Releases automatically.** The
  publish job previously passed no `--prerelease` flag to `gh release create`,
  so an alpha or RC tag could be displayed as a normal release despite passing
  tag preflight.
- **Keep classification executable and tested.**
  `release-publication-flag.sh` translates a validated suffix-bearing tag into
  one optional CLI argument. The workflow preserves that output as one array
  element, and isolated regressions cover GA, alpha, RC, invalid input, missing
  classification, and a dropped publication argument.
- **Check before creating a tag.** The defect was found after the exact
  `v1.1.0-alpha.1` local preflight passed but before the tag existed locally or
  remotely, so no incorrectly classified GitHub Release required deletion.
- **Hosted runner tools must be explicit.** The first tagged preflight then
  exposed that Ubuntu did not provide `rg`, which the release-scope and
  installer-pin contracts require. The workflow now installs `ripgrep`, checks
  command discovery before the gate, and guards both requirements in source so
  runner-image drift reports one actionable dependency failure.
- **Never suppress blocking-gate diagnostics.** After the dependency repair,
  Linux reached the race detector but `check-release.sh` redirected its output
  to `/dev/null`, leaving only an exit code. Race output now streams to hosted
  logs, and ordinary CI installs the same `ripgrep` dependency before
  `make verify`; the gate remains blocking and unchanged in scope.
- **Audit an auto-merged dependency batch before retagging.** Two Dependabot
  frontend updates landed while the release run was active. Their build and
  lint jobs passed, but `npm audit` found transitive development dependency
  `brace-expansion` 5.0.7 through ESLint/minimatch. Regenerating the lockfile
  selected 5.0.8, clearing the high-severity unbounded-expansion advisory
  without changing a direct dependency.
- **Time-based protocol fixtures must cover every clock they encode.** Hosted
  CI exposed fixed RFC 3161 `TSTInfo` timestamps that had aged beyond TSA
  certificates based only on `time.Now() +/- 24h`. PAdES test certificates now
  derive their bounds from both the deterministic token time and digitorus's
  wall-clock CMS signing attribute, with a safety margin. This preserves exact
  timestamp assertions while preventing a date-dependent expiry.
- **Keep frontend runtime floors identical across CI and release.** All GitHub
  frontend jobs now use Node.js 26, and `make release-scope` rejects workflow
  drift. Local Node 26 checks and hosted setup/build/lint jobs passed before the
  release workflow pin was adopted.
- **Node Web Storage is not jsdom Web Storage.** Node 26 enables a process-wide
  `localStorage` global, but without `--localstorage-file` it shadows jsdom with
  an unusable value on Linux workers. Frontend code uses `window.localStorage`
  at the browser boundary, and Vitest passes `--no-experimental-webstorage` to
  its workers so jsdom remains the browser-storage provider. The stability gate
  deliberately enables Node Web Storage in its parent process, reproducing the
  collision on Node 24 while rejecting bare storage references in production
  source.
- **A hermetic validator test must select its fake backend explicitly.** The
  PDF/A helper test injected fake `verapdf` and `docker` binaries but its CLI
  phase still preferred a host-installed Docker. GitHub therefore ran a real
  container inside an allegedly isolated regression. The CLI phase now sets
  `GOPMGR_VERAPDF_FORCE_CLI=1`, while the Docker phase keeps its fake first on
  `PATH`; release-gate output is streamed so future failures retain evidence.

### 2026-07-27 — First packaged 1.1.0 alpha

- **Published evidence is narrower than installation evidence.** Release run
  `30302650798` passed the full tag preflight and native build matrix, then
  published `v1.1.0-alpha.1` as a GitHub prerelease with Windows x86-64,
  macOS arm64, Debian/Ubuntu x86-64, and RPM x86-64 assets. This proves native
  compilation and packaging, not first-launch or uninstall behavior.
- **Rerun transient infrastructure at its smallest boundary.** The initial
  Linux leg encountered a one-byte size mismatch while Google's Chrome apt
  mirror was synchronizing. Windows and macOS completed successfully, so only
  the failed Linux job was rerun. Its package build passed and the retained
  matrix artifacts flowed into the publication job.
- **Keep published-release records synchronized after live verification.**
  `docs/published-release-tags.txt`, README, installation guidance, release
  preflight, and the code map were updated only after GitHub reported a
  non-draft prerelease with all four expected assets.
- **Isolate first-launch tests from developer data.** The published DMG was
  mounted read-only and launched on an M4 Mac with a temporary
  `XDG_DATA_HOME`. GoPMgr created private `GoPMgr`, log, and `system.db`
  paths, reported that no administrator existed, and exposed the first-user
  administrator checkbox. The app then exited cleanly without submitting an
  account, leaving the developer's normal Application Support data untouched.

### 2026-07-28: Recoverable clean-test reset

- **A reset for testers must not be a delete command.** The published DMG's
  isolated administrator lifecycle proved that moving the complete GoPMgr
  data root aside is sufficient to restore first-launch onboarding.
  `scripts/reset-clean-test.sh` therefore renames the active root to a
  timestamped sibling and prints the restorable path; it never removes project
  or account data.
- **Guard the path and the database lifecycle.** The helper accepts only an
  absolute directory named `GoPMgr`, rejects symlinked roots and backups,
  refuses to run while a packaged or development GoPMgr process is active,
  and never overwrites an existing root or backup. Restore accepts only a
  sibling backup created by the clean-test naming contract.
- **Test destructive-adjacent tooling entirely in fixtures.**
  `scripts/reset-clean-test_test.sh` creates sentinel account and project
  files under a temporary parent, verifies reset preservation and restoration,
  and exercises invalid-name and symlink refusals. `make verify` runs this
  fixture test, never the real reset target.

### 2026-07-28: Installed macOS upgrade and reinstall lifecycle

- **Test the installed bundle, not only the mounted image.** Published Alpha 1
  was copied to `/Applications` and launched from that exact path on an M4
  Mac. A current-`main` DMG was then built from synchronized commit `8643943`,
  validated with `hdiutil`, architecture inspection, DuckDB linkage, and
  strict ad-hoc signature verification, and copied over the Alpha bundle.
- **Separate application replacement from data preservation.** The installed
  bundle was removed to a recoverable temporary backup and reinstalled from
  the current-`main` DMG. The isolated `system.db` checksum remained identical
  across upgrade, removal, and reinstall; SQLite still reported one
  administrator and eight recovery codes. Each installed build opened the
  existing-administrator sign-in screen.
- **Prerelease package and bundle versions intentionally differ.** The Alpha 1
  package name carries `v1.1.0-alpha.1`, while macOS metadata keeps the clean
  `1.1.0` version of record as required by the release contract. Binary
  checksums, rather than Finder's version display, proved that the current
  `main` build replaced the published Alpha binary.
- **Keep real developer data outside lifecycle tests.** Both normal GoPMgr
  data roots retained their original inode, modification time, permissions,
  and `system.db` SHA-256 checksum. All application launches used an isolated
  `XDG_DATA_HOME`.

### 2026-07-28: Beta backlog and macOS green window control evaluation

- **The disabled green control is not a Svelte defect.** GoPMgr's
  `options.App` is resizable and has no maximum dimensions, but leaves
  `Mac` nil. Wails 2.13.0 initializes its Darwin `zoomable` flag only inside
  the non-nil macOS options branch; its Cocoa bridge then disables
  `NSWindowZoomButton`. The installed app's Accessibility tree independently
  reports the full-screen/zoom control as disabled.
- **Prefer the narrow native fix.** The Beta implementation should supply
  explicit `mac.Options` with zoom enabled and retain the standard title bar,
  native Window menu, minimum size, and Cocoa ownership of restoring the
  previous frame. A custom Svelte traffic-light control would duplicate native
  accessibility and window-state behavior.
- **Separate two restoration promises.** The P0 fix is in-session restoration
  to the pre-zoom frame. Remembering a normal frame across application
  relaunches is a P2 improvement because it also needs display-topology
  validation and off-screen clamping.
- **Keep Beta work operationally bounded.**
  `docs/beta-release-backlog.md` is the running list for fixes, features,
  improvements, and native release evidence. The first Beta stabilizes the
  existing 1.1.0 feature set rather than adding scheduling or report scope.

### 2026-06-08 — PDF/A-3 gate promoted to hard
- **`make check-pdfa` is now a hard release blocker.** Representative samples (schedule report, document charter, combined report, and Monte Carlo risk report) pass veraPDF PDF/A-3b. `scripts/check-release.sh` now exits non-zero when any sample fails instead of printing a warning and continuing.
- **Remove "soft gate" wording when the gate passes reliably.** The `validate-pdfa.sh` header comment and the "soft for now" check-release comment both said "warn, don't fail" -- these were vestigial once all samples passed. Gate promotion requires two things: (1) all representative samples pass, (2) the release script actually exits on failure.
- **`admin_test.go` gained `TestSecureArchiveRemovesArchiveWhenCreatedAuditLogFails`.** Uses a SQLite trigger to block the `ARCHIVE_CREATED` audit row, confirms `SecureArchive` returns `AUDIT_LOG_WRITE_FAILED`, and asserts the archive file is cleaned up. Tests run clean including this new case.

### 2026-06-08 — Matrix engine layout tests (swot, stakeholder, generic)
- **Coverage asymmetry is a reliable "untested real logic" signal.** `charts/matrix` sat at 29.5% while sibling engines (dag 83.7%, flow 94.9%, stats 86.0%) were high. Cause: only `raci.go` had a test; `swot.go`, `stakeholder.go`, and `generic.go` Parse/Layout functions were 0%. Added `swot_test.go`, `stakeholder_test.go`, `generic_test.go` → package now 95.8%, race-clean.
- **Apply the glue-vs-logic discriminator before chasing a low number.** Low coverage in `cli` (5%), `cmd/gopmgr`, `pdfrender`, and `export` is structural — `flag` registration, Wails App methods, gofpdf draw calls. Those are uncoverable-by-nature and refactoring a launch entry point to test stdlib boilerplate is risk without reward. The matrix functions, by contrast, are pure parse + layout math (quadrant classification, sqrt(n) micro-grid placement, ragged-array normalisation) — real behaviour worth pinning.
- **`LayoutStakeholder` single-point invariant makes a clean assertion.** With n=1 in a bucket, the micro-grid formula collapses to exactly the quadrant centre, so each of the four Power×Interest combinations maps to a known (x,y). Used that to verify quadrant routing without reverse-engineering the grid spread.
- **Remaining matrix gaps are defensive guards, not logic.** The uncovered `n==0`/`cols<1` branches in `LayoutStakeholder` are unreachable (a bucket only exists with ≥1 member; `ceil(sqrt(n≥1))≥1`). Left untested deliberately rather than contorting tests to hit dead guards.

### 2026-06-08 — Documents package unit tests
- **Mirror the charts smoke-test pattern for the documents package.** `internal/documents/documents_test.go` adds 33 tests: `TestAll_Returns25Definitions`, `TestAll_ReturnsCopy_NotMutable`, `TestAll_KindsMatchGetLookup`, `TestGet_KnownKind_ReturnsDefinition`, `TestGet_UnknownKind_ReturnsFalse`, `TestByPhase_SumEqualsAll`, `TestDefaultContent_AllKindsProduceValidJSON` (25 sub-tests), `TestDefaultContent_UnknownKind_ReturnsBraces`, and `TestRender_AllKindsProduceValidPDF` (25 sub-tests). All 33 pass, race-clean.
- **`DefaultContent` is the right smoke-test seed for renderer tests.** It generates schema-valid zero-value JSON for every kind (resolving the two Word/Excel alias pairs at runtime), so the render smoke test expands automatically when new kinds are added without needing per-kind DataExample strings in the registry.
- **`forvar` captures are redundant from Go 1.22.** Range-loop variables are re-scoped per iteration in 1.22+; `d := d` inside the loop body is not needed. Use the IDE `forvar` diagnostic as the trigger to remove them.
- **The stale TODO #9 ("bespoke renderers pending") is now closed.** All 23 bespoke renderers + 2 aliases are wired into the `renderRaw` dispatch switch; TODO #9 in §8 is marked done.

### 2026-06-08 (later) — PDF/A-3 gate: closed the "missing tooling = silent pass" hole
- **A "hard" gate that skips when the validator is absent is still soft.** The earlier promotion made `check-release.sh` exit on *sample* failure, but `validate-pdfa.sh` still `exit 0`d ("SKIP") whenever veraPDF could not be obtained, the ICC profile was missing, or no samples were found. In any environment without Docker/veraPDF (the common CI default), the "hard" wrapper therefore passed **vacuously** — certifying PDF/A-3 it never checked. A release gate must fail when it *cannot* verify, not only when verification fails.
- **Strictness is now an explicit switch, strict by default.** `validate-pdfa.sh` reads `GOPMGR_PDFA_STRICT` (default `1`). Unmet preconditions route through `pdfa_precondition_unmet`: strict → print `FAIL` and `exit 1`; non-strict → print `SKIP` and `exit 0`. `check-release.sh` invokes the script with `GOPMGR_PDFA_STRICT=1` explicitly so the release path is immune to a future default change; `GOPMGR_PDFA_STRICT=0 make check-pdfa` preserves local ergonomics on machines without Docker/veraPDF. An actually non-compliant sample fails in **either** mode — strictness only governs the can't-even-run preconditions.
- **`ICC_PROFILE` and the strict flag are env-overridable for hermetic testing.** Added `GOPMGR_ICC_PROFILE` so the precondition branches can be exercised (point it at a nonexistent path) without deleting the tracked sRGB profile. Verified all four matrix cells: {ICC-missing, veraPDF-missing} × {strict→exit 1, non-strict→exit 0}, plus the happy path (real veraPDF 1.30.2, strict default) which still reports all three samples `isCompliant="true"` (146 passed / 0 failed rules) and the existing `validate-pdfa-lib_test.sh` integration test.
- **veraPDF has no GitHub releases — the script's GitHub auto-download path is dead (404s).** Acquisition order that actually works: Docker image, then a `verapdf` already on `PATH`. The izpack installer from `software.verapdf.org/releases/verapdf-installer.zip` can be driven unattended via the console installer (`-console`, answer `1` / target path / `O` / per-pack `Y`·`N`), but CI should just provide Docker or a preinstalled CLI. Left the best-effort downloader in place (it's hermetically tested and harmless), but strict mode now turns its failure into a real gate failure instead of a skip.
- **Sandbox note for future sessions:** the mounted working copy disallows `unlink`/`rm` (EPERM) even for files this user owns, while *create* and *overwrite* succeed. `validate-pdfa.sh` does `rm -rf "$SAMPLE_DIR"`, so it can't run in place here; exercise it against a `cp -a`'d copy of `internal/ cmd/ scripts/ go.mod go.sum` under `/tmp` (tmpfs) instead. Go is not preinstalled in the sandbox; fetch `go1.26.x.linux-arm64` to `/tmp`.

### 2026-06-09 — update and auth package tests (isNewer, VerifyManifest, NeedsRehash)

- **Apply the glue-vs-logic discriminator before chasing any low-coverage number.** `internal/update` had three pure functions (`isNewer`, `splitVer`, `atoi`) at 0% coverage despite being real algorithmic logic. `internal/auth` had `NeedsRehash` at 0%. Both were the right targets; `CheckLatest` (HTTP orchestration), `Check` (CLI entry point), `CheckLatest`'s HTTP transport paths, and argon2-calling happy-path branches were correctly skipped.
- **Ed25519 test construction: sign the raw `payloadJSON` bytes, not the base64-encoded form.** `VerifyManifest` calls `ed25519.Verify(pubkey, payloadBytes, sig)` where `payloadBytes` are the decoded raw JSON. In tests: `json.Marshal(payload)` → `sig := ed25519.Sign(priv, payloadJSON)` → `PayloadB64 = base64.StdEncoding.EncodeToString(payloadJSON)`. Signing the base64 form instead produces a silently wrong test that always gets `ErrInvalidSignature`.
- **Minimize argon2 round-trips in tests.** Argon2id is intentionally slow (64 MiB, 3 iterations, 4 threads). Cover `HashPassword` happy path + `VerifyPassword` happy path + `ErrMismatch` in one `TestHashVerifyPassword_RoundTrip` test. All other `VerifyPassword` error paths are tested with hand-crafted PHC strings that are rejected before `argon2.IDKey` is called.
- **Test counts from `grep -c "^func Test"` before writing notes.** The prior session had a 48-vs-40 discrepancy because the count was written from memory. Always run the grep and state: new tests added vs. file totals separately.
- **`VerifyManifest`'s post-verify payload parse error is reachable without compromising a key.** Sign raw non-JSON bytes (`[]byte("not-json")`) with the real private key; the signature verifies, then `json.Unmarshal(payloadBytes, &p)` fails. This hits the final uncovered branch for 100% on `VerifyManifest` at essentially zero cost.
- **`cmd/gopmgr` does not build without a pre-built `frontend/dist`.** `go test ./internal/... ./cmd/...` exits 1 on `pattern all:frontend/dist: no matching files found` even when all internal packages pass. The correct wording is "all internal packages pass race-clean; `cmd/gopmgr` not tested (requires built `frontend/dist`)."

### 2026-06-09 — stats package: six remaining stat engine tests
- **Coverage asymmetry applies within a package too.** `charts/stats` sat at 42% after the 2026-06-04 session that only added Pareto and Control tests. The six remaining engines (Line, Bar, Pie, BurnUp, BurnDown, CumulativeFlow) were all at 0% despite being pure parse+layout math. Added `stats_remaining_test.go` → package now 95.3%, race-clean.
- **Apply the glue-vs-logic discriminator within `charts/stats` too.** All eight stat engines are pure `json.Unmarshal` + value computation with no gofpdf calls. Every layout function is worth testing; every `ParseXxx` success path is implicitly exercised by layout tests, so 83.3% on `ParseXxx` functions is the right stopping point rather than adding redundant valid-doc parse tests.
- **Derive expected values from the code's own formula, not intuition.** `computeIdealBurnDown([]float64{10}, 5)`: step = 10/(5-1) = 2.5, so out = [10, 7.5, 5, 2.5, 0]. Pinning this numerically catches both off-by-one errors in the index and float-precision regressions.
- **The `out[i] < 0` clamp in `computeIdealBurnDown` is a defensive guard against negative input; unreachable for valid non-negative remaining.** Any burn-down document with negative `remaining[0]` could trigger it, but that is invalid input — don't contort tests to exercise it. Leave the guard in place.
- **`LayoutCumFlow` alphabetical-fallback ordering must be asserted, not just trusted.** `sort.Strings` on a map's keys is deterministic, but the test documents the canonical order (doing, done, todo) so any future drift in key collection is caught immediately.
- **`LayoutPie` zero-total guard is real logic worth a dedicated test.** Division-by-zero protection that silently returns 0% when all slice values are zero is a deliberate user-visible choice (no NaN in the JSON), not defensive boilerplate.

### 2026-06-09 — charts dispatcher and pdfmeta trivial tests

- **Dark parse-error arms in a dispatcher are the highest-value uncovered lines.** `engines.go:Layout()` sat at 74.5% with all 20 `if err != nil { return LayoutResult{}, err }` paths dark because `TestLayout_AllKindsHaveDataExample` only exercised happy paths. A single `TestLayout_AllKinds_RejectsBadJSON` table test over `All()` covers every parse-error arm in one sweep.
- **`"{bad}"` is the right bad-JSON sentinel for parse-error table tests.** It is neither `""` nor `"{}"` (both of which many parsers accept as zero-value early returns), so it always reaches `json.Unmarshal` and returns a syntax error, regardless of the parser's empty-string handling.
- **Layout-error paths (cycle detection, nil-root) need their own targeted tests.** `TestLayout_AllKinds_RejectsBadJSON` stops at parse errors; the layout-error arms (Network/PERT/CPM cycle, CauseAndEffect nil-root, Workflow/Activity cycle) each need one dedicated test with a structurally-valid but semantically invalid document. A single `cycleJSON` constant with a mutual A↔B edge exercises all five cyclic-layout cases.
- **`dag.ParseCausalTree("{}")` returns a zero-value doc (no error); `dag.LayoutCausalTree` returns `ErrNoRoot` for `Root==nil`.** The two-step path is not obvious from the function names. `TestLayout_CauseAndEffect_NilRootError` with `"{}"` input is the canonical way to exercise this arm.
- **`DefaultICCProfile` and `HasDefaultICC` are 0% until explicitly tested.** Both are pure accessor functions with real behaviour (copy-on-return, non-empty guard) worth pinning. A three-test block covers: non-nil return, copy semantics, and `HasDefaultICC() == true`.
- **`xmlEscape` at 50% means the 5 special-char branches are dark.** A single `TestXmlEscape_AllSpecialChars` with `&<>"'` as input covers all five `case` arms in one assertion.

### 2026-06-09 — agile/dora formatHours and calendar country coverage

- **A package-level coverage number hides glue vs logic composition.** `agile` at 48.3% looks low, but `store.go` is pure SQLite CRUD (intentionally untested) and accounts for the majority of the package. `dora.go` functions were individually 97–100% already; only `formatHours` (41.7%) and the `now.IsZero()` branch (2.9%) remained. Check function-level breakdown (`go tool cover -func`) before spending effort on a package.
- **Direct tests of unexported pure-formatter functions are higher value than more ComputeDORA integration tests.** `formatHours` is in `package agile`, reachable from the test file. Calling `formatHours(72)` directly pins the `"3 d"` branch in one line; achieving the same via `ComputeDORA` requires constructing a deployment with a 72-hour lead time and then checking a deeply nested label field. Go's white-box testing (package-internal tests) makes direct formatter tests the right choice.
- **Compute expected formatter output before writing tests.** `formatHours(800)`: 800/24=33.3d, 33.3/7=4.76wk → `formatFloat1(4.76)` = `int64(4.76*10+0.5)=48`, whole=4, frac=8 → `"4.8 wk"`. Derive from the code; don't guess.
- **A `time.Time{}` zero-value test exercises the `if now.IsZero()` guard without any test clock.** Pass `time.Time{}` as the `now` argument to `ComputeDORA` and assert `!res.From.IsZero()`. The function falls back to `time.Now()`, so `From` is set to a real past timestamp.
- **Country-code switch arms are worth one test each: the tested behavior is AddHoliday, not just a switch.** Each `For("XX")` arm calls a different `bc.AddHoliday(xxx.Holidays...)` which loads a distinct holiday pack. A Christmas check (Dec 25) is the most portable cross-country assertion: present in `us`, `gb`, `ca`, `de`, `fr`, and `au.HolidaysNSW`. Verifying `CountryCode` alongside `IsHoliday` also pins the case–normalization contract (UK → CountryCode "UK", not "GB").
- **`WorkdaysFrom` backward walk is a real code path, not a mirror.** The `step = -1; days = -days` branch in `WorkdaysFrom` is never hit by forward-only tests. A single `WorkdaysFrom(Monday, -1) == Friday` test closes it and documents the expected behavior for future readers.

### 2026-06-09 — sigma/stats capability bands and timeline parseDate

- **Construct a dataset with a known sample StdDev to test downstream index math exactly.** `CalculateCapability` was at 76.9% because the DPMO ladder (`sigma>=5/4/3/2/<2`) was dark; only the top band was tested. The dataset `{-1, 1}` has sample StdDev exactly √2 (variance = 2, n-1 denominator). With a centered spec `USL=H, LSL=-H`, the code reduces to `cpk = H/(3σ)` and `sigmaLevel = H/σ + 1.5`. Setting `H = math.Sqrt2 * k` makes `sigmaLevel = k + 1.5` exactly, so a table of k-values drives the function into every DPMO band deterministically. Pick each k so its sigmaLevel sits >=0.3 inside its band; float rounding then cannot flip a `>=` boundary.
- **RFC3339Nano is a superset of RFC3339; the explicit RFC3339 fallback in `parseDate` is dead code.** A string that fails `time.Parse(RFC3339Nano, ...)` but passes `time.Parse(RFC3339, ...)` does not exist (the `.999999999` fraction is optional in the Nano layout). So `parseDate` caps at 88.9%; the reachable gap is the final non-empty-garbage `return false`, which a direct `TestParseDate` table closes. Leave the RFC3339 branch in place as defensive code, same call as the `out[i] < 0` clamp and the `now.IsZero()` guard from prior sessions.
- **`time.Time` zero-value (n-1) sample StdDev: gonum `stat.StdDev(values, nil)` uses the n-1 (Bessel-corrected) denominator.** Worth pinning when you reverse-engineer an expected σ: `{-1,1}` gives √2 (n-1), not 1 (population). Deriving the wrong denominator silently shifts every capability index.

### 2026-06-09 — charts/dag encoders and kind-specific layout wrappers

- **`charts/dag` was the laggard pure-logic engine at 83.7% (siblings flow/stats/matrix all 94-96%) because in-package coverage misses cross-package callers.** `LayoutCPM/Network/PERT` showed 0% in `go test ./internal/charts/dag/` even though `charts/charts_test.go` exercises them through the `Layout()` dispatcher: per-package coverage only counts the package's own `_test.go` files. Direct in-package tests of the wrappers are what move the dag number.
- **An Encode round-trip (`Parse(Encode(doc))`) closes two gaps at once.** The four `Encode*` functions were 0% and the matching `Parse*` success paths were the uncovered 16.7% (existing Parse tests only covered empty + invalid JSON). One round-trip test per pair covers the encoder and the parser's happy path together.
- **`json.Marshal` of these plain structs never fails, so the `Encode*` error guard caps coverage at 75%.** No channels, funcs, or cyclic pointers in WBS/Layered/Fishbone/CausalTree docs, so `json.Marshal` cannot error. Leave the `if err != nil` arm as defensive code, same call as the RFC3339 fallback and the `out[i] < 0` clamp from prior sessions. Do not contort a test to force a marshal failure.
- **`LayoutCPM`/`LayoutPERT` mutate the caller's node slice in place; assert on the input slice, not the `Layout` output.** `NodeLayout` (the visual output) carries no ES/EF/IsCritical/Expected fields - those are written back onto the `LayeredNode` slice, whose backing array is shared even though `doc` is passed by value. Build `nodes := []LayeredNode{...}`, call the wrapper, then check `nodes[i].IsCritical`/`.Expected`. A linear chain has zero float throughout, so every node is critical: the simplest CPM happy-path assertion.
- **`walk(nil, ...)` directly covers the nil guard.** White-box (package `dag`) tests can call the unexported `walk` with a nil node to exercise the `if n == nil { return }` arm that `FlattenLeaves`/`TotalEffort` never hit with well-formed trees.

### 2026-06-09 — documents pure helpers (date window, aggregation, issue classification)

- **`internal/documents` is ~95% gofpdf glue but hides a real seam of pure helpers.** The `Render*PDF`/`*Section`/`*Bullets`/`draw*` functions are gofpdf draw calls (intentionally untested), but each renderer is fed by pure transforms: `normalise*` (map -> typed struct), aggregations (`sumExecutionCost`, `procurementTotal`, `budgetSubtotal`), date math (`computeProjectWindow`, `parseDate`), and issue classification (`partitionIssues`, `isIssueResolved`, `issueSeverityOrder`). These are white-box testable and were the package's only legitimate logic targets. Pinned all nine to 100%; package moved 39.3% -> 40.5% (the small delta is expected: the glue dominates the statement count).
- **`computeProjectWindow` Days is inclusive (`+1`); assert the exact value, not `>0`.** Jan 1 -> Jan 10 is 10 days, not 9. The off-by-one is the function's whole purpose. The non-obvious branch (a chunk of its old 35.7%) is the third `if`: a task with only a start date still extends `maxT` via `s.After(maxT)`, giving `Start == End` and `Days == 1`.
- **Do not mechanically test the ~20 near-identical `normalise*`/`getStringX`/`getFloatX` accessors.** They share one pattern: a type assertion falling back to a zero value on a missing or wrong-typed key. One representative test (`TestNormaliseExecutionTasks_DefaultsOnBadInput`, passing `123` for a string field and `"not a number"` for a float field plus an empty map) pins the contract. Replicating it per file is noise, not coverage.
- **Issue classification logic is in the trim+case-fold and the severity sort, not the counts.** `isIssueResolved` lowercases and trims (so `"Closed "`, `"  DONE"`, `"RESOLVED"` all match); an empty status is open. `partitionIssues` sorts each partition by `issueSeverityOrder` ascending (critical=0 leads). Assert the returned order (critical before high before medium), not just `len(open)`.
- **There are now two `parseDate`s in the tree with different signatures.** `timeline.parseDate` returns `(time.Time, bool)`; `documents.parseDate` (in `execution_plan.go`) returns a bare `time.Time` and loops `{"2006-01-02", RFC3339, RFC3339Nano}`. No collision (different packages), but assert against each one's actual signature; don't copy timeline's `ok` checks into documents.
- **The pure-logic well is now near-dry.** After this, the remaining low-coverage packages (`cli`, `export`, `charts/pdfrender`, `sigma/service`, `db`) are predominantly glue (flag registration, file writers, gofpdf, SQLite CRUD) already correctly rejected by the discriminator. A future survey turning up "no legitimately testable target, stop" is a valid outcome, not a reason to reach for glue.

### 2026-06-09 — stale-doc TODO cleanup (report.go, engines.go)

- **With the coverage well dry, the next legitimate work is closing stale TODO/completion comments that contradict shipped code.** Grepping `TODO|FIXME|this v1|follow-up|not yet|do not yet` over `internal`/`cmd` (excluding `_test.go`) surfaces them. Two were materially wrong:
  - `documents/report.go` claimed "charts are referenced only by ID in this V1 ... embedding ... as raster images is a follow-up." The code already embeds each `chart_ref` as a *vector* visualisation on its own page via `pdfrender.RenderChartToPDF` (confirmed by reading `BuildCombinedReport`/`renderSectionBody`), matching README TODO #12 (Done). Rewrote the comment to describe actual behavior.
  - `charts/engines.go` claimed "Stats / Matrix / Flow families return ErrEngineNotImplemented" and "DAG fully implemented in V2.1." All 20 kinds have switch arms (the `TestLayout_AllKindsHaveDataExample` test exercises every one), so that text was stale. Rewrote to list all four families as implemented.
- **`ErrEngineNotImplemented` is NOT dead code despite all kinds being implemented; verify usage before deleting an error var.** It is still the switch's default-return (engines.go ~228) and is handled non-fatally in `main.go` (`errors.Is(err, charts.ErrEngineNotImplemented)`). It guards the case where a future registry entry is added without a renderer arm. Keep the var; only the surrounding doc text was stale. The lesson: a "not yet implemented" *string* can be a live defensive default, not evidence of incomplete work - read the call sites.
- **Historical note, superseded 2026-06-13 and the 2026-06-22 README rewrite.** At the time, README's "Real TODOs in the V2 scaffold" list had only non-code items left and #8 still deferred SQLCipher. SQLCipher encryption-at-rest has since landed, and README is now a public overview. Future status work should read this Developer Handbook plus current focused docs rather than trusting this 2026-06-09 snapshot.

### 2026-06-09 — pdfrender error-sentinel robustness (errors.Is over string compare)

- **`pdfrender.isEngineNotImpl` compared `err.Error()` against a hardcoded copy of the charts error string.** `pdfrender/dispatcher.go` already imports `internal/charts`, so the brittle `err.Error() == "charts: engine renderer not yet implemented"` was replaceable with `errors.Is(err, charts.ErrEngineNotImplemented)`. The string compare was a latent bug: it silently breaks if the message text drifts (and I had just edited code next to that very error string the prior session) and it does not unwrap, so a wrapped sentinel would be misclassified as a hard failure. This is the kind of real fix left once the coverage well is dry: grep the codebase for `err.Error() ==` / `strings.Contains(err.Error()` to find string-based error matching that should be `errors.Is`/`errors.As`.
- **The regression test must include the wrapped case, because that is the behavior the fix actually buys.** `TestIsEngineNotImpl` asserts nil->false, sentinel->true, `fmt.Errorf("...: %w", sentinel)`->true, unrelated->false. The wrapped-sentinel row is the one a string compare against `err.Error()` would fail; without it the test would pass against the old brittle code too and prove nothing.
- **A near-zero coverage package (`pdfrender` at 1.8%) can still host a worthwhile pure-logic test.** Almost all of pdfrender is gofpdf draw glue, but `dispatcher.go` has three pure helpers (`fit`, `parseBody`, `isEngineNotImpl`) and a white-box `pdfrender_test.go` already pins the first two. The package percentage stays low (glue dominates) but the helper is now correct and guarded.

### 2026-06-09 — CRITICAL: frontend did not run; a rune in a plain .ts crashed mount

- **The whole app failed to mount and every existing gate was green.** `src/lib/toast.ts` used the `$state` rune, but Svelte 5 only compiles runes in `.svelte`, `.svelte.js`, or `.svelte.ts` files. In a plain `.ts`, `$state` resolves to Svelte's runtime stub that throws `rune_outside_svelte` on call. `App.svelte` -> `ToastContainer.svelte` -> `toast.ts` imports it at module load, so the error threw synchronously and `#app` rendered nothing (`childCount: 0`). Fix: rename to `toast.svelte.ts` and update the ~12 importers to the project's extension convention (`from '../toast.svelte'`, matching how `session.svelte.ts` is imported as `'session.svelte'`).
- **`svelte-check` AND `vite build` both pass on this bug.** svelte-check passes because Svelte ships *ambient TypeScript types* for `$state` (so the type system is happy in any `.ts`); `vite build` passes because esbuild bundles the call without knowing it is special. The throw only happens at *runtime*. The release gates (`check-release.sh` frontend stability + build budget) run check and build but never launch the UI, so a runtime-only break is invisible to them. **Lesson: "check passes + build passes" is not "the app runs." For any frontend change, load the app (`npm run dev`, then a browser/preview tool) and confirm `#app` actually mounts.**
- **To verify the foundation screens without the Go backend: they render under plain `npm run dev`.** `App.svelte`'s `onMount` guards on `window.go?.main?.App?.CurrentUser` and returns early when the Wails bindings are absent, so it stays on the Login route. Backend-dependent routes won't load, but login/create-account/recovery and all global CSS do - enough to confirm mount, focus rings, and theming. A `.claude/launch.json` (`npm --prefix frontend run dev`, port 5173) is committed so the preview tool can drive it.
- **Guard against regressions of this class with a runtime gate (now implemented).** `make frontend-smoke` / `scripts/frontend-smoke-check.sh` loads and SSR-renders `App.svelte` through the real Vite + Svelte compiler and fails if any module throws at load or render. It is wired into `check-release.sh` (step 4b). See the dedicated lesson below.

### 2026-06-09 — frontend UI/UX polish (global foundation in app.css)

- **Open-ended "polish" is best spent on the global foundation, not 60 component rewrites.** All these landed in `app.css`/`index.html` and improve every screen at once:
  - **Keyboard focus ring.** 40 files used Tailwind `outline-none` (which is a *transparent* outline, not `outline: none`) and 0 used `focus-visible`/`focus:ring`, so keyboard users had no visible focus on buttons. An *unlayered* `:focus-visible` rule (written after the `@tailwind` directives, so it outranks the layered `.outline-none` utility per CSS cascade-layer precedence) restores a 2px accent ring. Scope it to interactive elements (`a, button, input, select, textarea, summary, [tabindex]:not([tabindex='-1'])`), not `*`, to avoid ringing programmatically-focused container divs.
  - **`prefers-reduced-motion`** media block neutralises animations/transitions app-wide. Keep a *text label* next to any spinner (App.svelte route loader) so the signal survives when motion is frozen.
  - **`color-scheme: dark` + `accent-color`** on `:root` make native scrollbars/checkboxes/date-pickers render dark and on-brand; both degrade gracefully on old WebViews.
  - **No flash-of-white on launch:** inline `style="background-color:#020617"` (slate-950) on `<html>` so the first paint before Tailwind loads is already dark. Desktop WebView apps otherwise flash white on cold start.
- **Verify visual changes in a real browser; a passing build proves none of them.** Used the preview tool: confirmed the focus rule is live in the cascade with the right value, `color-scheme: dark` applied, html bg `#020617`, the reduced-motion media rule present, and `onMount` autofocus put the cursor in the username field. A headless preview cannot hold document focus, so `:focus-visible` cannot be screenshotted mid-keyboard-nav - confirm the rule is loaded and correct instead, and say so honestly rather than implying a screenshot you could not take.

### 2026-06-09 — frontend runtime smoke gate (catches "app does not mount")

- **`make frontend-smoke` is a true runtime gate with zero new dependencies.** `frontend/scripts/smoke-mount.mjs` spins up a Vite dev server in middleware mode and `ssrLoadModule('/src/App.svelte')`, then SSR-renders it via `svelte/server`. Importing App.svelte transitively executes the whole synchronous module graph (ToastContainer -> toast.svelte.ts -> session, etc.); a rune misused in a plain `.ts` throws at that load, failing the gate. Reuses Vite (already a devDep) instead of adding jsdom/Playwright/vitest, which suits the project's minimal-dependency ethos.
- **SSR is the right execution mode here precisely because effects do not run.** App.svelte's `onMount` touches `window.go` and `$effect` fires the dynamic route imports; under SSR both are skipped, so the foundation loads in Node without the Wails backend or a DOM. That is exactly the surface that must never crash on import. Do not `ssrLoadModule('/src/main.ts')` - main.ts calls `mount()` against `document` at top level and would throw `document is not defined` (a false positive). Target the root component, not the client entry.
- **Load `svelte/server`'s `render` through the same `ssrLoadModule` pipeline as the component**, so they share one Svelte instance (Vite may process svelte for SSR differently than a direct Node import).
- **A regression gate is worthless until you prove it fails on the bug.** Verified end-to-end: dropped a plain `.ts` using `$state` into App's graph and confirmed `make frontend-smoke` exits 1 with "the app failed to load or render ... #app would not mount", then restored via `git checkout` and confirmed exit 0. Note the error string differs by context (`rune_outside_svelte` in the browser stub vs. `$state is not defined` ReferenceError under Node SSR) - the gate keys off *any* throw, not a specific message, so both are caught.
- **Wiring:** `scripts/frontend-smoke-check.sh` (bash wrapper, `cd frontend && node scripts/smoke-mount.mjs`), `make frontend-smoke` target, and `check-release.sh` step 4b after the stability gate. Requires `frontend/node_modules` like the other frontend gates.

### 2026-06-20 — Code quality audit + data management controls

- **`CloneProject` WAL checkpoint race fixed.** When the source path is the currently-open project (`samePath(a.dbPath, clean)` true), the code now calls `openDB.CreateSnapshot(dest)` (`VACUUM INTO ?`) rather than a raw `copyFile`. `VACUUM INTO` checkpoints all WAL data atomically before writing the snapshot. The raw-copy path is retained for closed/external projects. Both paths `os.Chmod(dest, 0o600)` on success.
- **Audit logging for destructive operations.** `DeleteChart`, `DeleteDocument`, and `DeleteWorkItem` in `main.go` each call `d.LogAction(actor, action, id, "")` (best-effort via `_ =`) before executing the deletion. `DeleteWorkItem` was also refactored to take an explicit `requireDB()` guard before delegating to `agileStore()` — it previously lacked a nil check.
- **Orphaned `Settings.svelte` deleted.** `frontend/src/lib/components/Settings.svelte` was a 94-line component never wired into App.svelte's routing; `AppSettings.svelte` (app-level) and `ProjectSettings.svelte` (per-project) covered its intended purpose. Removed with `git rm -f`.

### 2026-06-20 — UI/UX polish pass (AppSettings, Dashboard)

- **`AppSettings.svelte` Save button moved to page-level footer.** Previously nested inside the "Defaults for new projects" section, making it look like a section-scoped action. Moved outside all `<section>` blocks into a standalone footer `div`. Error display: the existing top-level error alert already covers both load and save failures; the footer does not duplicate it (only the "Saved." success span lives in the footer).
- **`Dashboard.svelte` `newCharter`/`newDocument` now surface errors via toast.** Both async functions were fire-and-forget; they now wrap the Wails call in try/catch and call `showToast(message, 'error')` on failure.
- **Document status shown as styled badge.** Added a `docStatusStyles` map (draft/review/approved/archived) and replaced the raw text status cell with a badge rendered via `border` + background color classes, matching the existing `statusStyles` pattern on Portfolio.
- **Delete buttons accessibility hardened.** "Del" labels renamed to "Delete" with `aria-label` attributes on chart and document delete buttons.

### 2026-06-20 — APFS username-collision security fix + regression tests

- **APFS case-insensitive collision:** On macOS APFS `~/Documents/PMForge/James` and `~/Documents/PMForge/james` are the same physical directory. Because `CreateAccount` used `WHERE username = ?` (case-sensitive), both accounts were created and shared a data directory, leaking project filenames via `ListProjects()`. Fix: `WHERE lower(username) = lower(?)` in `internal/users/store.go`. No DB constraint added — the live `system.db` already contained both rows; a COLLATE NOCASE UNIQUE index would have failed on the existing duplicate. Application-level check is correct for tolerating the legacy collision while blocking new ones.
- **`TestCreateAccount_RejectsCaseVariantUsername`** (`store_test.go`): Creates "alice", asserts "Alice"/"ALICE"/"aLiCe" each return `ErrUserExists`. Filesystem-independent — exercises the SQL check, passes on case-sensitive CI.
- **`TestHasLegacyRecoveryCodeWraps`** (`dek_test.go`): Covers all three states — no codes (false), nil-DEK legacy codes (true, blocks encryption enablement), codes with DEK (false, guard cleared). Coverage: 0% → 80%.
- **`RemainingRecoveryCodes` skip:** `SELECT COUNT(*)` always returns one row; `sql.ErrNoRows` branch is unreachable. Glue-vs-logic discriminator applied: no test written.
- **Duplicate account deleted:** Removed `James` row from live `system.db` via `DELETE FROM users WHERE username = 'James'`. No filesystem changes — APFS shared the directory with `james`.

### 2026-06-20 — REUSE compliance + WAL/audit test coverage

- **REUSE gate restored.** Three root causes: (1) stale `cmd/gopmgr/frontend/dist/` from the 2026-06-15 main-package relocation — not gitignored at that path, deleted via `rm -rf cmd`, `/cmd/` added to `.gitignore`; (2) `frontend/package.json.md5` not gitignored — added; (3) `build/bin/**` + `build/packages/**` are gitignored but `reuse` 6.x scans them anyway — added REUSE.toml glob annotations. All 11 `make check-release` gates pass.
- **`audit_actions_test.go` (4 tests, race-clean).** `TestCloneOpenProject_DataSurvivesSnapshot` saves a chart to the open project before cloning, then opens the clone and asserts the chart is present — the actual VACUUM INTO invariant (a raw `copyFile` can miss data still in the WAL). `TestDeleteChart/Document/WorkItem_WritesAuditLog` each save an entity, delete it, and query `app.db.Conn` directly to confirm one `audit_log` row with the correct action and target_id. All 11 `make check-release` gates pass on the final tree.

### 2026-06-20 — Error reporting system + code quality audit

- **`debug.Wrap` logs to the persistent log as a side effect.** `Wrap()` in `internal/debug/report.go` now calls `log.Printf` when `err != nil`, so the error reaches the dated log file without the call site doing anything extra. Consequence: any call site that used to add its own `log.Printf("[trace] ...")` after calling `debug.Wrap` is now double-logging. The canonical example is `internal/admin/workflow.go`'s `LogSignatureEvent` — its `log.Printf("[signature trace]...")` was removed; `debug.Wrap(err, "PDF_SIGNATURE_ERROR")` already emits the line.
- **`fmt.Printf` in library code (not CLI) is a null sink in Wails builds.** When Wails launches the app as a bundle the OS discards stdout. `internal/export/engine.go`'s `fmt.Printf("[export] %s report at %s\n", ...)` was producing output that no one ever saw. Routing it through `log.Printf` (which is teed to the dated log file via `applog.Init`) is the correct fix. Grep for `fmt.Printf` in non-CLI packages to find this class of bug.
- **`console.error` in Wails desktop apps is invisible to users.** There is no DevTools console in a packaged Wails binary. `console.error` is the right pattern for a web app where users can open DevTools; for a desktop app it is silent error-swallowing. Replace actionable errors with `showToast(..., 'error')` so users see what went wrong and can self-report. `SigmaProjectView.svelte`'s `loadToolStatus` catch block is the example.
- **Check call frequency before changing silent errors to toasts.** A toast that fires from a `$effect` re-running every reactive update spams the user. Before replacing `console.error` with `showToast`, grep for all call sites of the function and confirm it is one-shot (called once on mount or once per user action), not invoked from a poll or continuous `$effect`.
- **Selective staging with `git apply --cached` when prior-session hunks ride along.** If a file shows ` M` in git status BEFORE you touch it in the current session, a whole-file `git add <file>` sweeps in those pre-existing hunks. The correct non-interactive approach: (1) `git show HEAD:<file> > /tmp/head_version` to get the clean base; (2) apply only your specific changes to the temp file; (3) `diff -u /tmp/head_version /tmp/mine | sed 's|/tmp/head_version|a/<path>|; s|/tmp/mine|b/<path>|' > /tmp/patch.diff`; (4) `git apply --cached /tmp/patch.diff` to stage exactly those hunks. Confirm with `git diff --cached` before committing.

### 2026-06-20 — Administrator role: bootstrap flow, gated account creation, admin panel

- **Admin bootstrap sentinel is `HasAnyAdmin()`.** Pre-admin state: any user (or no user) may claim the admin role during CreateAccount or via App Settings → "Become administrator." Post-admin state: CreateAccount is gated — callers without a signed-in admin session get an error. The sentinel is checked at the top of `App.CreateAccount` before any account-creation logic runs, so there is no TOCTOU window.
- **`SetAdmin` last-admin guard must check the target's current role first.** The naive guard (`COUNT(*) WHERE is_admin = 1 ≤ 1 → ErrLastAdmin`) incorrectly fires when demoting a non-admin user while exactly one real admin exists. Fix: query `SELECT is_admin FROM users WHERE username = ?` first; only apply the count check when `targetIsAdmin == 1`. `DeleteAccount` already had the correct pattern — mirror it in `SetAdmin`.
- **PRAGMA table_info's `dflt_value` column is NULL for columns with no default.** Scanning it into a plain `string` fails with "converting NULL to string is unsupported". Always scan dflt_value into `sql.NullString` in migration helpers. This is the pattern used in `migrateAdminColumn()`.
- **Two-step confirm for destructive admin actions.** First click on "Delete" or "Grant/Remove admin" sets a `pendingDelete`/`pendingRoleChange` state string. A second click on the same action (now showing "Confirm") executes it. No time-delay required; the visual change between the two clicks is sufficient. Cancel link clears the pending state.
- **In-memory `a.user.IsAdmin` can go stale** if another admin demotes the current user while they are signed in. Acceptable trade-off for a local single-machine app — the session is refreshed at the next sign-in. Document in the source, do not add a re-fetch loop.
- **Authorization tests belong to the feature, not to an afterthought.** The advisor caught that 8 files of call-site updates (adding the `isAdmin bool` arg) produced zero new tests for the privilege logic itself. Added 8 store-level tests (`TestHasAnyAdmin_*`, `TestSetAdmin_*`, `TestDeleteAccount_*`) and 7 app-level tests (`TestCreateAccount_*`, `TestBecomeAdmin_*`, `TestAdminDeleteUser_*`, `TestAdminSetUserRole_*`). These are all pure-data, no Wails needed.
- **`AdminPanel.svelte` and `AppHeader.svelte` verified by svelte-check only** (no admin session available in the Vite dev preview without the Go backend). Confidence level is stated explicitly: type-correct, not runtime-verified. This caveat lives in the commit message, not the DEVELOPER_HANDBOOK.md.

### 2026-06-20 — In-app Help Guide

- **Route wiring pattern for a new top-level view.** Four files, one change each: (1) `session.svelte.ts` — add the view name to the `view` union type; (2) `App.svelte` — add a `routeLoaders` entry mapping the name to a lazy `import()`; (3) `AppHeader.svelte` — add to `baseNav` and extend the `active` prop union; (4) `main.go` — add a menu item in `buildAppMenu()` that calls `emit("menu:<name>")` and add a matching `rt.EventsOn("menu:<name>")` handler in App.svelte's `onMount`. The guide's nav item appears in the Help menu and as the fourth top-bar tab.
- **Launchpad seeds must be verified against `launchpad_seeds.json`.** The actual seed combinations and their artifacts do not match any reasonable intuition based on industry convention. For example, Construction + Waterfall seeds `['wbs','statement_of_work','risk_register','cpm']` — not Charter + Gantt. Business + OKRs seeds `['plan_word','stakeholder_analysis_doc','status_report']` — not Charter + Stakeholder Matrix. Never guess; always read the JDM file at `internal/templates/launchpad_seeds.json`.
- **The Fishbone's default categories are People, Process, Equipment, Materials, Environment, Measurement** — confirmed in `FishboneEditor.svelte` as the `SIX_MS` constant. "Methods" is a common textbook variant but is NOT what GoPMgr seeds. The sixth M is "Measurement."
- **The VoC/CTQ component is a direct Need → CTQ mapping**, not a three-tier Need → Driver → CTQ tree. Each entry has: customer_need, ctq, lower_spec, upper_spec, measurement, data_collection, priority, source. The "CTQ tree" label in the component's heading is a summary, not an accurate description of the data model.
- **The DEK (Data Encryption Key) is wrapped separately by the password AND by each recovery code.** `internal/users/dek.go` stores `wrapped_dek_pw` in `users` and `wrapped_dek` in `recovery_codes`. A passphrase reset via `ResetWithRecoveryCode` unwraps the DEK from the matching code and re-wraps it under the new password in a single transaction. Legacy codes issued before encryption was enabled have an empty `wrapped_dek` — the `HasLegacyRecoveryCodeWraps` guard blocks encryption enablement until codes are reissued and carry a DEK wrap.
- **`OpenLogsFolder` opens the LOG directory, not the data directory.** The data directory path is exposed separately via `GetAppInfo().DataLocation`. These are two different paths. When documenting what "Open Logs Folder" does, say "log directory" only.
- **`HelpGuide.svelte` is a zero-backend component.** No `window.go.*` calls; only `$state` for the active section cursor. Type-checks cleanly with `svelte-check` (0 errors on 210 files). Keep it that way — the guide must render even in pure Vite dev mode without a running Go backend.

### 2026-06-21 — V2.x feature sweep: Portfolio, autosave, HTML export, native dialogs, attribution sweep

- **Portfolio.svelte is the post-login landing screen.** Shows all projects with status badges, doc/chart counts, and quick-nav links. Route: `'portfolio'`. Wired via `goto('portfolio')` from Dashboard and ProjectPicker. `AppHeader.svelte` provides the shared top-bar nav tab strip; any new top-level view should import it and pass the correct `active` prop.
- **Autosave is a single shared module (`autosave.svelte.ts`).** All chart editors register in `onMount` and unregister in `onDestroy`. Pattern: `stopAutosave = autosave.register(() => JSON.stringify(doc), () => save())`. The module diffs snapshots so unchanged docs don't trigger a save. File uses the `.svelte.ts` extension because it contains `$state` runes; import path is `'../autosave.svelte'` (no `.ts` suffix, matching the established `session.svelte.ts` convention).
- **HTML schedule-report export added.** `internal/export/html.go` provides `renderHTML(payload, opts)` dispatched by `case FormatHTML:` in the engine. `ProjectSettings.svelte` now offers CSV/HTML/MSPDI buttons alongside PDF/DOCX/ODT. Verified `ExportScheduleReport{CSV,HTML,MSPDI}` exist at main.go lines 1958–1973.
- **Native app error dialogs split by platform.** `internal/applog/dialog_darwin.go`, `dialog_other.go`, `dialog_windows.go` use build tags. Cross-compile verified clean: `GOOS=windows go build ./internal/applog/...` and `GOOS=linux go build ./internal/applog/...` pass (the templates linker issue in `internal/templates/jdm.go` is pre-existing and unrelated).
- **`time.Time` fields in Wails-facing DB structs must be `string`.** An empty `time.Time` serializes to a zero-string the bridge cannot re-parse from JS. Changed `CreatedAt`/`UpdatedAt` in `internal/db/{charts,documents,project,stakeholders}.go` from `time.Time` to `string`; scan helpers removed `time.Parse` calls and assign directly. RFC3339 strings round-trip cleanly through the bridge.
- **Multi-return Go functions serialize as `null` in WebKit Wails.** Both `EnsureDefaultBoard` and `CreateProjectFromLaunchpad` returned tuples (`board, columns, nil`). WebKit serializes multi-return as `null` on the frontend. Both migrated to named single-return structs: `BoardWithColumns{Board: board, Columns: columns}` and `LaunchpadResult{Project: p, Path: path}`. Frontend callers migrated from `const [a,b] = await ...` to `const res = await ...; res.board; res.project`.
- **ProjectPicker two-step delete confirm.** `confirmingDelete` / `busyPath` state pair: first click sets the pending item, second click executes. Same pattern used in AdminPanel. `CloneProject` uses `CreateSnapshot` (WAL-safe `VACUUM INTO`) for the open project and raw copy for external project paths.
- **`git rm --cached` is the correct tool to untrack a committed file without deleting it.** `git rm -r --cached .agent_memory/` and `git rm --cached .claude/settings.local.json` removed those paths from git tracking while preserving local files. After adding them to `.gitignore`, `git add .` correctly skips both. This is the pattern for "was committed, should be local-only going forward."
- **SPDX copyright attribution sweep: "James L. Burns and The GoPMgr Contributors".** All file headers and `REUSE.toml` updated from "The GoPMgr Contributors". This was a deliberate attribution decision by the project owner; future files must use the updated form. Such a sweep belongs in its own commit (or led prominently in a combined commit message) so a future `git log --all-match` of "SPDX" or "attribution" finds it without noise.
- **Verify cross-platform builds for new platform-guarded files before committing.** The dialog files introduce the first platform split under `internal/applog/`. Run `GOOS=windows go build ./internal/...` and `GOOS=linux go build ./internal/...` before every commit that adds or modifies `_darwin.go` / `_other.go` / `_windows.go` platform files.

### 2026-06-21 — Concurrency and correctness audit (full backend + frontend review)

- **All Sigma* methods had a nil-pointer dereference race.** Each method called `requireDB()`, released the RLock, then accessed `a.sigmaSvc` without any lock. Adding a `requireSigmaSvc()` helper (same pattern as `requireDB()` — RLock, copy, RUnlock) and bulk-replacing all ~20 callers eliminates the race. Pattern: any field on `a` that can be set or cleared by concurrent Wails calls needs its own `require*()` accessor.
- **IssueRecoveryCodes had a DEK use-after-wipe bug.** `dek := a.dek` copies the slice header but shares the backing array. If `Logout()` zeros the backing array while the codes are being issued, the DEK silently becomes all-zeros. Use `requireDEKLocked()` (which does `make + copy`) whenever callers need the DEK to outlive the lock release.
- **RepairAndSwap must keep sigmaSvc in sync with db.** The atomic swap updated `a.db` and `a.adminSvc` but left `a.sigmaSvc` pointing at the closed database. Any post-swap Sigma call would access a closed handle. Fix: always update *all* service references that wrap the swapped database together in the same `Lock()` block.
- **`PackEnabled bool` data race: use atomic.Bool.** A plain `bool` accessed from multiple Wails goroutines is a data race. Changed `agile.PackEnabled` to `sync/atomic.atomic.Bool`; all read sites use `.Load()`, all write sites use `.Store()`. This is the canonical Go 1.19+ pattern for single-flag shared state.
- **TOCTOU comment in recovery.go was overstated.** A plain `BEGIN` (deferred) does NOT fully prevent two concurrent calls from both passing hash verification — it only prevents cursor-across-write conflicts and makes scan+write atomic per connection. `BEGIN IMMEDIATE` would be the correct TOCTOU barrier. The comment now accurately describes what the transaction does provide. The risk is acceptable for a single-user desktop app where concurrent recovery resets are not an expected workload.
- **SignCertificateModal `onConfirm` receives a signing-options object from the modal.** The modal supports `none`, `pades`, and `gpg`; PAdES carries the effective certificate path and per-operation password, while GnuPG carries an optional key ID. Callers (`Dashboard.svelte`, `CharterEditor.svelte`, `ReportComposer.svelte`) must branch on `options.method` instead of assuming a certificate workflow.
- **Plain Export PDF must be disabled while the Sign modal is open.** Adding `|| showSignModal` to both export-button disabled conditions prevents a concurrent plain export from starting while the signed export is in progress. This is the "sibling-UI guard" pattern — any time two actions share output state, the disabled condition of each must gate on the other's in-flight state.
- **Scope-filter before calling a code review complete.** Use `git diff HEAD --name-only -- '*.go' '*.svelte' '*.ts' | while read f; do n=$(git diff HEAD -- "$f" | grep -E '^[+-]' | grep -vE '^[+-]{2}|SPDX-FileCopyright|<cosmetic pattern>' | wc -l); [ "$n" -gt 0 ] && echo "$n $f"; done | sort -rn` to isolate files with real functional diffs from cosmetic sweeps. Cross-check the output against what was actually reviewed before declaring done.
- **Tuple-to-struct API migrations must be aligned frontend and backend.** `EnsureDefaultBoard` and `CreateProjectFromLaunchpad` both changed from returning multiple Go values (serialized as a JSON array by Wails) to returning named structs (serialized as a JSON object). Frontend callers migrated from destructuring `const [a, b] = await ...` to `const res = await ...; res.field`. Always verify the Go return type matches the frontend access pattern when both sides change.

### 2026-06-22 — Wire sigma workspace into Dashboard navigation

- **Methodology-gated sections use `{#if session.project?.methodology === 'six_sigma'}`.** The optional chain is required: `session.project` is null before a project is open, so a bare `session.project.methodology` would throw. Gate is placed after the nav row and before the charts section so sigma-methodology projects get the Process Excellence card at the top of the content area. Non-sigma projects are completely unaffected.
- **Verify the methodology string traces through the creation path before relying on it in a gate.** `ProjectLaunchpad.svelte` uses `let methodology = $state('')`; the selector sets `methodology = m.id` (line 241, where `m.id` is `'six_sigma'` verbatim); `CreateProjectFromLaunchpad` receives and stores it directly. Confirmed by reading the component — no label transform, no enum mapping.
- **`SigmaWorkspace` is a global sigma-project list, not scoped to the current `.pmforge` project.** `SigmaListProjects()` returns all sigma projects regardless of which GoPMgr project is open. The Dashboard card is a navigation entry to that global workspace, not a scoped view. Card copy should reflect this (e.g., "DMAIC project tracking", not "this project's DMAIC"). The design is intentional — confirmed before wiring.
- **One entry point unlocks the full reachability chain.** Before this change, `sigma_dashboard` / `sigma_project` / TollgateChecklist were all unreachable. Adding the single Dashboard card restores the chain: Dashboard → SigmaWorkspace → SigmaProjectView → TollgateChecklist. No additional wiring is needed.

### 2026-06-22 — Recent-changes review + roadmap/doc reconciliation

- **Reviewed the last ~20 commits** (Six Sigma Dashboard wire-up, the 2026-06-21 concurrency/correctness sweep, the SPDX + V2.x QA sweep, the admin-role and Help-Guide work). No correctness regressions found; the concurrency fixes (requireSigmaSvc nil-guard, DEK deep-copy in IssueRecoveryCodes, RepairAndSwap sigma-service reassignment, recovery TOCTOU tightening, PackEnabled→atomic.Bool) are coherent and self-consistent with the shipped code.
- **Corrected a stale roadmap status in README.md.** Scheduling roadmap **item 19** still carried the header `*Kernel core landed 2026-06-10; UI layer remaining.*`, which contradicted its own body and DEVELOPER_HANDBOOK.md: the assignment UI, Level, and Histogram actions all shipped 2026-06-10 (confirmed in `CPMEditor.svelte` — Assignments section, `LevelChartResources`, `GenerateResourceHistogram`). Header updated to "Done 2026-06-10" so the canonical status doc no longer self-contradicts.
- **Superseded the 2026-05-20 note "Remaining for the frontend: a Settings-panel font picker."** Shipped: `ProjectSettings.svelte` has the `ListFonts()` dropdown, the "Import font…" button calling `ImportFont()`, and persistence via `SetDefaultFont` (verified in source). The font subsystem is complete end-to-end, backend and frontend.

### 2026-06-22 — README public rewrite and user-guide split

- **README.md is now the public overview and documentation index.** It no longer carries the completed V2 TODO ledger, coverage tables, directory deep dive, or dated implementation history. Keep release-scope strings there for SQLCipher, FileVault/BitLocker/LUKS, PDF/A, and the current DSS PAdES fixture classification; put detailed implementation status in this Developer Handbook or focused docs.
- **User workflow material moved to `docs/user-guide.md`.** The new guide covers Launchpad, Portfolio/Dashboard, Project Settings, Application Settings, Stakeholders/Timeline/Budget, charts, documents, combined reports, schedule interchange, PDF signing, recovery codes, fonts, logs, and save behavior.
- **The scheduling roadmap canonical record now lives in this Developer Handbook.** Do not point future agents to README for detailed roadmap/status history.
- **Superseded the 2026-05-25 note "svelte-check still reports accessibility/deprecated-event warnings."** `npm run check` now reports **0 errors and 0 warnings** across 210 files (the earlier Sigma-helper / signature-modal warnings were cleaned up during the 2026-06 UI-polish work). The large-chunk production-build warning is the only remaining advisory and is benign.
- **Genuinely outstanding roadmap follow-ups (no code written this session — documentation pass only):**
  - **EVM in the document-renderer path is complete as of 2026-06-26.** Status Reports now expose a linked CPM schedule field; combined reports resolve CPM/Gantt chart EVM from chart refs and render Earned Value under the referencing section when cost/progress data is present.
  - **Resource calendar slice complete as of 2026-06-26** (item 19 / Phase 1 continuation). Named calendar storage, the Project Settings capacity panel, generated histogram capacity overlays, and layout-path calendar-aware over-allocation checks now exist.
  - **SigmaWorkspace global scope** (known, by design). The Dashboard card navigates to a global DMAIC project list, not a view scoped to the current `.pmforge` project. Tracked as an intentional limitation, not a defect.
- **Verification note for this session.** The Go toolchain is not present in this review environment, so `go build` / `go test` were not run here; backend claims were verified by reading the shipped source. Frontend `svelte-check` ran clean (the macOS `node_modules` needed the Linux rollup binary installed `--no-save`; no tracked files were modified). Recommend a local `make race` + `npm run check` before the next tag to re-confirm the green baseline on a full toolchain.

### 2026-06-22 — GitHub migration: Actions CI + Dependabot remediation

- **CI/CD moved from GitLab to GitHub Actions.** `.github/workflows/ci.yml` (verify / build / lint / security) + `release.yml` (native multi-OS Wails builds → GitHub Release) replace `.gitlab-ci.yml`. Three clean-checkout gotchas are baked in: (a) the root package's `//go:embed all:frontend/dist` needs the frontend built before any Go compile, so CI builds it (or stubs the dir for vet/lint); (b) Wails was bumped 2.9.2 → **v2.13.0** and the runtime/workflow CLI pins stay aligned; (c) strict PDF/A requires embedded fonts, so the four Source Sans 3 faces are tracked and checksum-verified while `make fonts` remains optional for the larger catalog. The app builds under the repo's Go 1.26.5. golangci-lint is pinned to v2.12.2 (action `golangci/golangci-lint-action@v9`); all workflow actions use node24-era major versions (checkout@v7, setup-go@v7, setup-node@v7, upload-artifact@v7, download-artifact@v8).
- **Migrated off the SheetJS `xlsx` npm package** (James's decision, 2026-06-22). npm's `xlsx` is frozen at 0.18.5 with unpatched prototype-pollution + ReDoS CVEs (SheetJS publishes fixes only via their own CDN now), so it was a permanent Dependabot dead-end. Replaced with **`read-excel-file`** (maintained, npm-native) in `SigmaProjectView.svelte` — the only consumer. **API note for v9:** the default export returns *all* sheets (`[{ sheet, data }]`); use the named `readSheet` export from `read-excel-file/browser` to get the first sheet's rows directly. Import the `/browser` subpath — the bare `read-excel-file` specifier has no `.` export and fails both Vite resolve and `svelte-check`. **Capability change:** legacy binary `.xls` is no longer parseable (read-excel-file is `.xlsx`-only); the import handler now shows a "re-save as .xlsx or CSV" message and the file picker drops `.xls`. Verified: `svelte-check` 0/0, Vite build + bundle-budget gate pass, Sigma stability gate pass.
- **Still open (James to run on his Mac, where Go + native npm live):** bump `golang.org/x/crypto` + `golang.org/x/net` (`go get ...@latest && go mod tidy`) to clear the remaining Go alerts — none are reachable (GoPMgr uses only `x/crypto/argon2` + `pkcs12`; every CVE is in `x/crypto/ssh`), but the bump is cheap hygiene. The remaining npm advisories are all **dev-only** (vite/esbuild/js-yaml/launch-editor — dev-server/build, never shipped); `cd frontend && npm i -D vite@latest && npm i` clears most.

### 2026-06-26 — Phase 1 money foundation and EVM exact cents

- **Money is now represented canonically as integer minor units.** `internal/money` stores signed cents and uses `math/big.Rat` for rate x fractional-effort calculations before rounding once to cents. Database migrations add `project.budget_minor_units`, `stakeholders.hourly_rate_minor_units`, and `stakeholders.contract_value_minor_units`; fresh schema declarations use `NUMERIC` for legacy display columns and integer minor-unit columns for arithmetic.
- **Budget and portfolio rollups use minor units.** `budget.Compute` returns existing display fields plus `*_minor_units`; DuckDB portfolio rollups stage money as `BIGINT` minor units and derive display floats after aggregation.
- **Portfolio EVM is coverage-aware and weighted.** `RunPortfolioAnalytics` computes each readable project's current-schedule EVM in the Go kernel at a single UTC status date, then hands only exact minor-unit rows to DuckDB. Committed cost remains contracts + estimated labour; EVM AC comes from task actuals. `EVMAvailable` and included/excluded counts prevent missing dates, malformed/cyclic schedules, or absent task budgets from appearing as zero. Keep portfolio SPI/CPI as `ΣEV/ΣPV` and `ΣEV/ΣAC`; averaging project ratios is mathematically wrong. Normalize the reporting instant to UTC midnight before `DayOffset` or a midday timestamp overstates PV by one working day.
- **EVM task money now follows the same rule.** `kernel.Task`, `TaskEV`, and `EVMetrics` carry minor-unit fields for BAC/PV/EV/AC/SV/CV/EAC/ETC/VAC. Existing `budgeted_cost` / `actual_cost` fields remain for UI compatibility, but `budgeted_cost_minor_units` / `actual_cost_minor_units` win when present. CPMEditor updates the minor-unit fields whenever the visible cost inputs change.
- **Split-task PV follows persisted work, not the visual span.** `cpmChartDataToKernelTasks` validates and maps relative `work_segments` to `kernel.Task.PlannedWorkSegments`; `ComputeEVM` integrates segment widths so PV plateaus during idle gaps. Keep `Task.WorkDays` reserved for absolute resource-capacity occupancy and preserve the half-open, task-relative `WorkSegment` contract when adding schedule adapters. `loadCurrentProjectSchedule` propagates malformed current-chart errors rather than silently reporting against potentially stale V1 tasks.
- **Risk Register to Risk Matrix synchronization is explicit and one-way.** `SyncRiskRegisterToMatrix` reads the persisted document, verifies project/kind ownership, maps register rows to `matrix.RiskItem`, and refuses the entire write if layout validation finds an invalid row. The generic document editor saves first and exposes **Refresh Risk Matrix** only when a matrix is linked. Do not add automatic bidirectional saves: they make cross-artifact conflict ownership ambiguous and can silently overwrite chart-only analysis.
- **Remaining Phase 1 sequencing:** resource-calendar/capacity planning built on the existing resource layer; tamper-evident audit now has an additive schema/API foundation; scenario analysis has metadata/schema, isolated chart/baseline partitions, Wails bridges, Project Settings metadata, chart-copy, dedicated editor routing, comparison, and promotion UI. Avoid reintroducing floating-point currency in SQL or Go.
- **Tamper-evident audit foundation landed 2026-06-26.** `audit_events` stores per-project sequence numbers, `previous_event_hash`, `event_hash`, canonical before/after JSON, actor/session fields, UTC timestamp, signature status, and optional signature blob. `AppendAuditEvent` computes the SHA-256 chain and `VerifyAuditChain` detects sequence, previous-hash, and event-hash tampering. Project, chart, document, baseline, scenario, scenario-chart copy, document approval checkpoint, scenario-promotion approval checkpoint, document-signature, and signed combined-report mutations now write this table, project open runs verification when `settings.compliance_mode` is enabled, and Project Settings exports private JSON audit verification and raw repair-evidence artifacts.

### 2026-06-28 — Monte Carlo scheduling foundation and CPMEditor workflow

- **Monte Carlo backend contract landed in `internal/kernel`.** `kernel.Task` now carries optional `DurationEstimate` values (`optimistic`, `most_likely`, `pessimistic`, `distribution`) and `RunMonteCarlo(tasks, iterations, workers)` returns deterministic P50/P80/P90 finish percentiles, per-task P50/P80/P90 sampled duration percentiles, critical-path frequency, and top-10 tornado risk drivers.
- **Supported distributions:** triangular (default), beta-PERT, and normal. Sampling uses Gonum `distuv`, deterministic per-iteration PCG sources, and copy-on-write task maps so simulations do not mutate the live schedule.
- **Failure behavior:** invalid iteration counts, nil tasks, map-key/task-ID mismatches, negative durations, invalid estimate ordering, unsupported distribution names, and cyclic schedules return `SimResult{Valid:false, Error:...}` rather than panicking.
- **CPMEditor workflow landed.** `App.RunChartMonteCarlo` exposes the simulation for CPM charts, `wails-window.d.ts` declares the bridge/result types, and `CPMEditor.svelte` lets users seed per-task three-point estimates, choose a distribution, run 100-10,000 iterations, review P50/P80/P90, a cumulative finish-probability S-curve, and tornado risk-driver bars ranked by critical-path frequency multiplied by P90-P50 duration spread, then export a PDF/A-tagged Monte Carlo risk report.
- **Phase 1 Monte Carlo workflow status:** kernel simulation, CPMEditor workflow, S-curve, tornado drivers, and PDF/A risk report are implemented.

### 2026-06-23 — DuckDB analytics engine (ADR-002 Option B, Phases A–E) + frontend npm-ci lesson

- **Decision (ADR-002): do NOT replace SQLite/SQLCipher with DuckDB; add DuckDB as an embedded, in-memory analytical engine in production/package builds.** Evaluation in `docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md`; design + phased plan in `docs/design/duckdb-analytics-engine.md`. DuckDB's encryption is real (v1.4, AES-GCM-256) but new/non-NIST and needs the `httpfs`/OpenSSL extension (auto-installed from the internet) to *write* — a poor fit for local-first. SQLCipher stays the system of record; ADR-001 unchanged.
- **`internal/analytics` is build-tag gated, but installer builds now enable the tag.** `analytics.go` (Engine interface + types + `ErrAnalyticsUnavailable`), `stub.go` (`//go:build !duckdb`, explicit no-DuckDB developer fallback), `duckdb.go` (`//go:build duckdb`, in-memory DuckDB over a single pinned `database/sql` connection, extension autoinstall/autoload disabled). `make build` passes `-tags duckdb`, and `scripts/verify-duckdb-linked.sh` checks the built binary metadata so package builds cannot silently ship the stub.
- **`App.RunPortfolioAnalytics`** (main.go, near `ComputeBudget`) enumerates readable projects with the session DEK, builds per-project cost metrics (`project.BudgetMinorUnits` + `budget.Compute(...).CommittedMinorUnits`), and aggregates via the engine as integer minor units. Untagged developer builds still short-circuit to `ErrAnalyticsUnavailable` before any scan. EV/PV aggregation deferred (SPI/CPI report 0 = n/a). `analytics` imports duckdb-go but NOT Wails, so the `-tags duckdb` CI job needs no GTK/WebKit.
- **Bonus:** the duckdb-go dependency tree lifted `golang.org/x/crypto` → 0.47 and `x/net` → 0.49 (from 0.33/0.35), clearing the remaining Go Dependabot alerts.
- **Historical frontend install lesson, superseded by the testing-branch major migration.** The former Vite 5/Svelte plugin 4 lockfile could not be freshly resolved safely, so `npm ci` was mandatory. The frontend now uses the compatible Vite 8/Svelte plugin 7 stack, but the reproducibility rule remains: use `npm ci` for clean builds and change `package.json` plus `package-lock.json` together through an intentional `npm install` only when upgrading dependencies.

### 2026-06-23 — Click-installable release packaging (.deb/.rpm/.exe/.dmg)

- **release.yml now builds native installers per OS** (was tar.gz/zip): Linux x86_64 → `.deb`+`.rpm` (nfpm, `build/linux/nfpm.yaml`); Windows x86_64 → NSIS `.exe` (`wails build -nsis`, runner installs NSIS via choco); macOS **arm64 / Apple Silicon** → `.dmg` (`scripts/package-macos.sh`, create-dmg with hdiutil fallback). The macOS matrix switched from `darwin/universal` to `darwin/arm64`.
- **Hosted Windows PATH refreshes are not implicit.** Chocolatey can install
  NSIS 3.12.0 successfully while the active Git Bash process still cannot find
  `makensis`. The workflow verifies the executable at
  `C:\Program Files (x86)\NSIS`, prepends its POSIX path for the current shell,
  and writes the native path to `GITHUB_PATH` for the later Wails step.
- **Native package inputs and macOS scratch space are explicit.** Release/CI
  builds use only the tracked Source Sans 3 baseline instead of opportunistic
  optional-font downloads, keeping platform packages reproducible. The hosted
  macOS job clears disposable Go module/build and npm caches after compiling
  DuckDB; `hdiutil` otherwise runs out of scratch space while staging the DMG.
  This cleanup stays in the ephemeral workflow, not the local packaging script.
- **NSIS fixture paths cross a shell boundary on Windows.** Homebrew NSIS
  accepts the fixture's script-relative POSIX path, but native `makensis.exe`
  launched from Git Bash needs an absolute Windows path. The validation script
  uses `cygpath` on Windows and non-empty dummy inputs so the same template
  compile is meaningful on both hosts.
- **Packaging assets are tracked** despite the broad `build/` ignore: `.gitignore` exempts `build/linux/gopmgr.desktop` and `build/linux/nfpm.yaml` (same trick as the darwin Info.plist scaffold). The icon is `build/appicon.png` → `/usr/share/pixmaps/gopmgr.png`; the `.desktop` → `/usr/share/applications/`.
- **Linux release target moved to Ubuntu 24.04+ WebKit2GTK 4.1** (2026-06-26). CI/release Linux runners now use `ubuntu-24.04`, install `libwebkit2gtk-4.1-dev`, and pass Wails' `webkit2_41` tag. Wails v2 still links GTK3 (`gtk+-3.0` in the upstream cgo files); true GTK4/WebKitGTK 6.0 requires a future Wails migration rather than a package-name change. `make linux-runtime-target` guards this target.
- **Signing/notarization is OFF** (owner decision 2026-06-23 — unsigned now, sign later). Packages install/run but show Gatekeeper/SmartScreen "unidentified developer" warnings. Hook: `MACOS_SIGN_IDENTITY` env in `scripts/package-macos.sh` (codesign + a commented notarytool block); Windows signing is a TODO. Add certs as CI secrets to enable.
- **Verify by tag.** The tag workflow now runs a full Ubuntu preflight before
  the native package matrix, but the Linux deb/rpm dependency names, Windows
  NSIS output, and macOS DMG remain native-runner integration evidence. Use a
  SemVer prerelease tag matching the version of record, then inspect and
  install every artifact. Do not publish release notes or claims until that
  tag actually exists on GitHub. Pre-flight checklist:
  `docs/release-preflight.md`.
- **Version of record is clean semver `1.1.0`** (2026-06-23 normalization). `internal/cli/parser.go` `Version` and `wails.json` `productVersion` must be equal (gated by `check-release.sh`) and a valid package version; `Info.plist` fills `CFBundleVersion` from `productVersion`, and the deb/rpm/dmg/exe version comes from the git tag (`${GITHUB_REF_NAME#v}`). `check-release-tag.sh` now requires the GA tag or a SemVer prerelease of that exact base version. Tag `v1.1.0` and all three channels read identically. The old `1.1.0-V1-Expansion` codename moved to release notes (rpm forbids `-` in Version; the codename was not valid semver). The updater (`internal/update/check.go`) still tolerates suffixed remote versions.
- **Supply-chain gates (2026-06-23).** CI gained a blocking `govulncheck` job (`vuln` in `ci.yml`). Static review: `docs/security-quality-review-2026-06-23.md`.
- **End-user docs:** `docs/INSTALL.md` covers per-format install + run-from-source; the in-app Help Guide gained an **Installing & Running** section (`HelpGuide.svelte`, Reference group). README's Quick Start now uses `npm ci` (not `npm install`).

### 2026-08-04 — PMForge → GoPMgr rename: the persistence-boundary rule, now test-enforced

- **Branding and persisted state are different things, and a blanket rename can't tell them apart.** The project renamed from PMForge to GoPMgr (avoiding confusion with the unrelated getpmforge.com). A repo-wide find/replace pass briefly renamed four literals that name state already written to real users' disks, before a manual revert caught it. That near-miss is the reason this entry and the tests below exist: nothing failed the build when it happened.
- **Historical note (superseded the same day):** this entry originally called the four literals below permanently frozen — never to become "GoPMgr". That held for a few hours. A same-day follow-up conversation asked for the rename anyway, this time **with** the dual-read compatibility layers a same-day-later entry below describes; all four were renamed as of the "PMForge → GoPMgr persistence-literal rename, implemented" entry further down. Kept here unedited as the record of what "frozen" meant and why, since the pinning tests and their break-verification method are still exactly how the later rename was itself verified — only the "never rename" conclusion changed.
- **The four literals below**, each intentionally still "PMForge"/`.pmforge` *as of this 2026-08-04 entry* (superseded later the same day — see above), and the test that pinned it at the time:
  - `internal/users/store.go` `DefaultRootDir()` — the data-root directory is `~/Library/Application Support/PMForge` (macOS) or `~/Documents/PMForge` (Linux/Windows/`$XDG_DATA_HOME`). Renaming the leaf, or swapping either branch's parent directory while keeping the leaf, orphans every existing install's accounts and projects. Pinned by `internal/users/root_dir_test.go`'s `TestDefaultRootDirPlatformDefault_UsesPMForgeLeaf`, which asserts the *full* resolved path on both branches — CI runs on Linux, so a test asserting only the darwin branch's full path would have shipped green on the platform that actually gates merges while leaving the Linux/Windows path pinned to nothing but its leaf.
  - `internal/users/store.go` `legacyMacRootDir()` — the pre-relocation `~/Documents/PMForge` path `MigrateLegacyRoot` reads from. Pinned by `TestLegacyMacRootDir_UsesPMForgeLeaf`.
  - The `.pmforge` project-file extension (checked in `app_projects.go`'s `projectPathFor`, used throughout `internal/db`). Pinned by `TestProjectPathForRejectsWrongExtension` in `project_path_confinement_test.go`, which specifically includes a `.gopmgr` case (the exact adjacent-rename mistake this entry is about) alongside a `.pmforge` positive control.
  - The `project.pmforge` zip entry name inside `.pmba` backup archives (`internal/db/backup.go` writes it, `backup_restore.go` reads it back by that same literal). Pinned by `TestCreateArchivalBundleAcceptsQuotedDestination` in `internal/db/backup_test.go`.
  - `PMForge_Archive_` (the `.pmba` filename prefix, `internal/admin/workflow.go`) is on this list for continuity, not compatibility — it's generated fresh every call and nothing reads it back, so keeping it just avoids a backups folder with both `PMForge_Archive_*` and `GoPMgr_Archive_*` files and no visible reason they differ.
- **A pinning test has to fail under the mutation it claims to catch, or it isn't pinning anything — verify this by actually breaking the literal, not by inspection.** The first version of the extension-check test passed even after the `.pmforge` literal was changed to `.gopmgr`, because it went through `DeleteProject`'s full path, which also opens the target as an encrypted SQLite database for its audit-log entry — a fake test file failed there regardless of which extension the check required, so the assertion passed for the wrong reason. Rewritten to call `projectPathFor` directly (the actual validation point) plus a `.pmforge` positive control, it now fails exactly when the check is broken. Every test above was confirmed this way: change the production literal, watch the test go red, revert.
- **Round-trip tests don't catch lockstep renames.** A test that creates a backup and restores it will pass even if `project.pmforge` were renamed identically on both the write side (`backup.go`) and read side (`backup_restore.go`) — the pair still agrees with each other. Only asserting the literal *inside a produced artifact* (open the zip, look for an entry named exactly `project.pmforge`) catches that. The equivalent check already existed for this one; it just needed the comment above explaining why the assertion isn't arbitrary.
- **Verified against a real pre-rename install, not just fixtures.** The renamed `gopmgr` binary was pointed at a real `~/Library/Application Support/PMForge` install on the development machine and correctly opened its actual `project.pmforge` file (detecting SQLCipher encryption and requesting credentials, rather than failing to find or parse it) — proof the rename didn't break data compatibility in practice, not only by construction.

### 2026-08-04 — PMForge → GoPMgr persistence-literal rename, implemented

- **What changed, per literal** (see `docs/security-quality-review-2026-08-04.md` for the decomposition that led here, and ROADMAP's "Next Recommended Audit" for the user-facing summary):
  - **Data-root directory** `PMForge` → `GoPMgr`. `DefaultRootDir()` now resolves to `~/Library/Application Support/GoPMgr` (macOS) / `~/Documents/GoPMgr` (elsewhere/`$XDG_DATA_HOME`). `legacyMacRootDir()` was replaced by `legacyRootCandidates()`, which returns an *ordered list* rather than one path: on macOS, the most-recent pre-rename default (`Application Support/PMForge`) is checked before the older pre-2026-06 `Documents/PMForge`, because a user could in principle have stale data at the older path too and the newer one must win. `MigrateLegacyRoot` now loops candidates and copies (never moves) the first one with a `system.db` into the new root, unchanged in spirit from the original single-candidate version.
  - **Project-file extension** `.pmforge` → `.gopmgr` (`app_projects.go`). `newProjectPath` only ever writes `.gopmgr` now. `isProjectExtension` accepts both, used by `projectPathFor` (the path-confinement check — widening which extensions pass does **not** loosen the parent-directory containment check underneath it, which is what actually stops path traversal) and `enumerateProjects` (which already had a legacy-flat-vs-subfolder dual-layout precedent to extend: it now tries `project.gopmgr` then falls back to `project.pmforge` per subfolder, and checks both extensions in the flat-layout branch).
  - **Backup archive entry** `project.pmforge` → `project.gopmgr` (`internal/db/backup.go`/`backup_restore.go`), gated by a new `BackupManifest.SchemaVersion` 2 (was 1; the field already existed, unused for this purpose until now). `RestoreArchivalBundle` resolves the expected entry name from the manifest's declared schema version via `schemaProjectEntry`, rather than checking whichever of the two names is present in the zip — sniffing would let a tampered archive swap in the wrong entry under the wrong version's name, since the entry-name validation pass runs before the manifest is even parsed. `.pmba` the archive-file extension itself was deliberately left alone: it's read back by `backup_restore.go` too, so renaming it would add a *fifth* dual-read path for a container format, not the two files it holds.
  - **Backup filename prefix** `PMForge_Archive_` → `GoPMgr_Archive_` (`internal/admin/workflow.go`). No compatibility branch needed — nothing reads this prefix back — so this one really was a pure find/replace plus updating the two tests that reference the literal.
- **Verification, sequenced one literal at a time, not as one batch change.** Following the same-day advisor guidance: cheapest/most-isolated first (`PMForge_Archive_`), then the data-root migration, then the backup entry, then the extension (highest blast radius, since it sits inside a security confinement check). After each step: a full Go build, vet, and test pass (`make test`) plus `make frontend-stability` before moving to the next, so a regression couldn't be attributed to the wrong step.
- **Old-format fixtures, not round-trip tests, prove the compatibility branches.** Consistent with the "round-trip tests don't catch lockstep renames" lesson from the entry above: `TestRestoreArchivalBundleReadsSchemaVersion1Archive` hand-builds a schema-1 archive (correct SHA-256 digest included) byte-for-byte the way a pre-rename release wrote one, since `CreateArchivalBundle` itself only ever writes the new format now and can no longer produce a v1 archive to round-trip against. Similarly, `TestMigrateLegacyRoot_FindsCurrentPMForgeInstall` and `...FallsBackToPreRelocationInstall` (`internal/users/migrate_root_test.go`) lay out a fake `$HOME` with a real "PMForge"-named install and call the actual public `MigrateLegacyRoot`/`DefaultRootDir` entry points main.go's `NewApp` goes through, not just the lower-level path-pair function the pre-existing tests exercised.
- **Every changed or newly-added assertion — Go and frontend — was break-verified**: temporarily revert the production literal to its old value (or delete the compatibility branch entirely), confirm the specific test/assertion goes red with the expected message, then restore. This includes tests inverted rather than newly written: `TestProjectPathForRejectsWrongExtension` (which asserted `.gopmgr` was *rejected*) became `TestProjectPathForAcceptsCurrentAndLegacyExtension`, with a comment explaining the inversion so a future reader doesn't mistake it for scope creep.
- **Frontend copy updated in the same commit, not left to drift further.** `HelpGuide.svelte`, `Dashboard.svelte`, and `ProjectLaunchpad.test.ts` now name the current paths/extension; `HelpGuide.svelte` additionally tells users upgrading from PMForge that their data folder was copied forward automatically, and still documents `.pmforge` as still-openable (not as current) since the app keeps reading it indefinitely. `frontend/src/lib/persistence-boundary-strings.test.ts` was updated in lockstep and re-verified the same way as the Go pins.
- **Historical note, corrected later the same day:** this entry originally said a fifth literal, `internal/export/sigma_report.go`'s `~/PMForge/exports`, was renamed to `~/GoPMgr/exports` on the same "safe, nothing reads it back" reasoning as `PMForge_Archive_`. That was the wrong fix. `GenerateSigmaReport`'s home-relative `getExportDir()` predates (by about 18 minutes, per `git log`) the per-user `<DataDir>/exports` pattern every *other* exporter in the codebase uses (`app_documents.go` ×6, `app_settings.go` ×2, `app_charts.go`, `app_fonts.go`) — it was a leftover from before that pattern existed, not a deliberate second export location. Renaming its literal preserved a bug: Sigma reports for every GoPMgr user on a machine landed in one shared, non-per-user directory, contradicting the documented "exports live under your GoPMgr data folder in `<username>/exports/`" behavior and, on macOS, bypassing the `Application Support` relocation entirely. Fixed in a follow-up pass: `GenerateSigmaReport` now takes `outDir` as a parameter instead of computing its own; `app_sigma.go`'s `SigmaExportProjectReport` passes `filepath.Join(u.DataDir, "exports")`, matching every other exporter. `getExportDir()` was deleted. Since the `~/GoPMgr/exports` version of this bug only ever existed in this session's local, unpushed commit, the only stale directory a real user could actually have is the pre-rename `~/PMForge/exports` — orphaned generated PDFs there, same non-data-loss category as before, just for the right underlying reason this time.
- **An adversarial post-implementation review (run per the user's "assume this is flawed, be skeptical" instruction) found one blocking defect: `$XDG_DATA_HOME` installs were silently orphaned.** `legacyRootCandidates()` returned `nil` under an `XDG_DATA_HOME` override — correct *before* this rename, when `DefaultRootDir()` resolved to `$XDG_DATA_HOME/PMForge` both "before" and "after" and there was genuinely nothing to migrate from. This rename changed `DefaultRootDir()`'s override behavior to `$XDG_DATA_HOME/GoPMgr` without revisiting that `nil`, so a real `$XDG_DATA_HOME/PMForge` install — a routine Linux desktop-environment configuration, not just a test knob — would get a brand-new empty root with no error and no migration attempt. `TestLegacyRootCandidates_EmptyUnderXDGOverride` had locked the bug in with a now-false comment ("only one location has ever existed"); inverted into `TestLegacyRootCandidates_UnderXDGOverride_ReturnsPMForgeCandidate` the same way `TestProjectPathForRejectsWrongExtension` was inverted elsewhere in this rename. Added `TestMigrateLegacyRoot_FindsXDGInstall` (`internal/users/migrate_root_test.go`), a fixture test calling the real public `DefaultRootDir`/`MigrateLegacyRoot` entry points under an actual `XDG_DATA_HOME` override — both new/changed assertions break-verified by reverting the fix and confirming red. **Why the original break-verification missed this:** the step-2 break test (`legacyRootCandidates` → `return nil`) only confirmed the two tests that call `t.Setenv("XDG_DATA_HOME", "")` first, so by construction neither could exercise the override branch — the XDG path simply had no test to break. Lesson: break-verifying a function means verifying every branch has a test that reaches it, not just verifying the tests that already exist go red.
- **The `root_dir_test.go` leaf/full-path assertions (`TestDefaultRootDirXDGOverride_UsesGoPMgrLeaf`, `...PlatformDefault_UsesGoPMgrLeaf`) were changed in lockstep with `DefaultRootDir()` but not independently broken during the original pass** — caught by the same adversarial review, which pointed out the handbook claimed 100% break-verification coverage without it. Both were then broken (reverted to expect `"PMForge"`) and confirmed to fail red before being restored, closing that gap for real.

### 2026-08-04 — Frontend copies of the frozen literals, now pinned; a fourth gap found and closed

- **The Go-side pinning tests above only covered the Go side.** ROADMAP's "Next Recommended Audit" flagged that `frontend/src/lib/components/HelpGuide.svelte`, `frontend/src/lib/components/project/Dashboard.svelte`, and `ProjectLaunchpad.test.ts` all hand-type the same data-root and `.pmforge`-extension strings the Go tests pin, with nothing catching drift on that side. First checked whether the better fix applied — a Wails binding exposing `users.DefaultRootDir()`'s resolved value, so the frontend could render the real value instead of a second copy — and confirmed none exists (`main.go`'s `App` type binds only `Greet`, `SetUnsavedChanges`, and lifecycle hooks; no root/log-path getter). Added `frontend/src/lib/persistence-boundary-strings.test.ts` instead, importing each source file as raw text (Vite's `?raw`) and asserting the exact macOS/Linux/Windows data-root strings and the `.pmforge` extension are present — mirroring the Go tests' "hardcode the literal, don't derive it from a shared constant" approach, since there is no single source importable from both a `.go` file and a `.svelte` file.
- **Found a second gap while reading the surrounding code for this: the fourth frozen literal, `PMForge_Archive_`, was never actually pinned.** `DEVELOPER_HANDBOOK.md` (this section, prior entry) lists it as frozen, and `internal/admin/admin_test.go` references the string — but only inside `TestSecureArchiveRemovesArchiveWhenCreatedAuditLogFails`'s cleanup glob, which asserts *zero* matches on a path where `SecureArchive` is expected to fail and remove its own output. That test would keep passing unchanged if the prefix were renamed, because an absence check can't tell "renamed and still cleaned up correctly" apart from "still named PMForge_Archive_ and cleaned up correctly." Added `TestSecureArchiveUsesPMForgeArchivePrefix`, which exercises the *success* path and asserts the prefix against a real archive filename `SecureArchive` returned.
- **Every assertion added this entry was confirmed the same way as the original four**: change the production literal, watch the specific test go red, revert. All five (four frontend + one backend) verified this way; see `docs/security-quality-review-2026-08-04.md` for the finding-by-finding writeup.
- **A same-day follow-up request asked to rename these frozen literals to GoPMgr/gopmgr outright.** Initially declined without further scoping, since a blanket "rename them" instruction doesn't distinguish four literals with very different blast radii. Given a per-literal decomposition and a follow-up "include everything, add a migration/dual-read" confirmation, the rename was implemented — see the "PMForge → GoPMgr persistence-literal rename, implemented" entry below for what actually shipped, and `docs/security-quality-review-2026-08-04.md` for the audit that led into it.

### 2026-08-04 — Sigma report exports were writing outside per-user isolation; fixed to match every other exporter

- **`internal/export/sigma_report.go`'s `GenerateSigmaReport` computed its own output directory via `os.UserHomeDir()`, independent of the signed-in user.** Every other exporter in the app (`app_documents.go` ×6 call sites, `app_settings.go` ×2, `app_charts.go`, `app_fonts.go`) writes to `filepath.Join(u.DataDir, "exports")` — the per-user directory `users.Open`/`CreateAccount` already creates with private permissions. Sigma reports instead landed in one directory shared by every GoPMgr account on the machine (`~/PMForge/exports`, later `~/GoPMgr/exports` in an earlier version of this rename — see the corrected note on the prior entry), not respecting the app's multi-user model (`internal/users` package doc: "Multiple GoPMgr users on the same OS account is the supported model") and not matching documented behavior (README/HelpGuide: "exports live under your GoPMgr data folder in `<username>/exports/`"). `git log -S` confirms `getExportDir()` predates the per-user `DataDir/exports` pattern by about 18 minutes in the same initial commit sequence — a leftover from before that pattern existed, not a deliberate second location.
- **Fix:** `GenerateSigmaReport` now takes `outDir string` as a parameter instead of deriving it; `app_sigma.go`'s `SigmaExportProjectReport` (which previously didn't call `a.requireUser()` at all) now does, and passes `filepath.Join(u.DataDir, "exports")` — the same pattern every other exporter already uses. `getExportDir()` was deleted. The directory-creation and 0700-tightening behavior (`os.MkdirAll` + `os.Chmod`) was kept *inside* `GenerateSigmaReport` rather than moved to the App layer like the other exporters, since it was already tested (`TestGenerateSigmaReportTightensExistingExportDirectory`) and removing a tested permission assertion to match the (slightly laxer) convention elsewhere would have been a silent security regression, not a cleanup.
- **Both `internal/export/sigma_report_test.go` tests were updated to pass an explicit temp `outDir`** instead of relying on `$HOME`, and break-verified by temporarily hardcoding `outDir` to a different path inside the function — both tests failed red (one on a missing directory, one on the wrong permission bits) before the revert.
- **Scope note:** this was found via a `filepath.Join(u.DataDir, "exports")` occurrence-count sweep prompted by re-examining the "fifth literal" renamed in the entry above, not by a bug report. The other 9 exporter call sites were deliberately left as-is — sharing them into one helper is a different, unrelated change.
- **Coverage gap, left unpinned on purpose:** the `internal/export` tests break-verify that `GenerateSigmaReport` honors whatever `outDir` it receives; nothing pins that `app_sigma.go` actually hands it `u.DataDir/exports` rather than some other path. That would need App-layer test scaffolding (account + project + sigma service), which the other 9 exporter call sites also lack — pinning only this one would be its own inconsistency. Noted here so the App-layer wiring isn't assumed to be covered when it isn't.
- **`os.Chmod(outDir, 0o700)` now tightens a directory shared by all exporters, not one owned solely by Sigma.** Before this fix it ran against `~/GoPMgr/exports`, used only by Sigma reports; now it runs against `<DataDir>/exports`, used by all ten exporters. `CreateAccount` already creates that directory at `0700`, so the chmod is normally a no-op, and tightening is still security-positive on the rare path where it does fire — but `TestGenerateSigmaReportTightensExistingExportDirectory`'s assertion shifted from "an exporter fixing its own directory" to "one exporter tightening a directory the others also write to." Recorded so the assertion doesn't read as arbitrary later.

### 2026-08-04 — Migrated accounts silently kept reading/writing the deleted old root; DataDir is now recomputed, not stored

- **`users.Account.DataDir` was trusted verbatim from the `data_dir` column, but the column is written exactly once, at `CreateAccount` time, as `filepath.Join(s.rootDir, username)`.** `Store.Authenticate` and `Store.List` both scanned it straight back out instead of recomputing it. Before this rename, `DefaultRootDir` was stable for a given machine, so the column and the live root always agreed. The PMForge → GoPMgr rename (and, retroactively, the earlier `3c23265` macOS iCloud/Documents relocation) changed that: `MigrateLegacyRoot` copies a legacy install's `system.db` into the new root, but every row inside that copied database still had the *old* root baked into `data_dir`. After migration, an account's `DataDir` pointed at the just-copied-from, no-longer-canonical old tree — files under the new root were correct and unused, while the app kept reading and writing under the old one. `MigrateLegacyRoot`'s own doc comment invites the user to "delete the old copy at leisure," so this was a real, reachable data-loss path: the app would appear to keep working right up until that cleanup happened, then silently lose access to every project.
- **Fix:** `internal/users/store.go`'s `Authenticate` and `List` now recompute `DataDir` as `filepath.Join(s.rootDir, <scanned username>)` instead of scanning the column into `Account.DataDir`. `s.rootDir` is the live root the `Store` was `Open`ed with, so `DataDir` is always correct for wherever the data actually lives now, independent of which root was active when the account was created. The username used is the one just scanned from the row (not the caller-supplied parameter), since the duplicate check in `CreateAccount` is case-insensitive (`lower(username)`) but the directory is created with the literal string — recomputing from the wrong case would resolve to a different path than the one that was actually created. The `data_dir` column itself is still written by `CreateAccount` and kept in the schema for debuggability; it is no longer authoritative, and the doc comment on `Authenticate` says so.
- **New end-to-end regression, `TestPreRenameInstallIsUsableAfterMigration` (root `migration_e2e_test.go`):** builds a real pre-rename install — genuine `system.db` via `users.Open`, a real account via `App.CreateAccount`, a real project via `App.CreateProject` — at a `"PMForge"`-suffixed root under an `XDG_DATA_HOME` override (hermetic on any OS, no reliance on the real machine's home directory). It then resolves the new root through the real `DefaultRootDir`/`MigrateLegacyRoot` entry points, **deletes the old root** to simulate the user acting on the "delete at leisure" invitation, opens a fresh `Store`/`App` against only the new root, and asserts login succeeds, `DataDir` is under the new root, and the project is found. The existing `MigrateLegacyRoot*` tests only assert that files get copied; this one proves the migrated install is actually usable once the old copy is gone — which is what the shipped feature is for. Break-verified: reverting `Authenticate`'s `DataDir` line back to the stored column produces the expected red (`DataDir after migration = .../PMForge/alice, want .../GoPMgr/alice`), confirmed, then restored.
- **Scope note:** this predates the rename — the same latent bug existed for any machine that hit the `3c23265` iCloud/Documents relocation, it just had fewer affected users since that was a narrower trigger. The rename made it reachable for everyone using `MigrateLegacyRoot`, which is why it surfaced now. The fixture is current-schema data in an old-layout location; it does not cover old-schema `system.db` data (schema migrations are additive-only per `migrate()`, so that axis is expected to work the same way, but it isn't exercised here).
- **No repair pass needed for already-migrated installs.** `git describe --tags b1c8336` (the commit that introduced the GoPMgr-suffixed `DefaultRootDir`) is `v1.1.0-alpha.1-20-gb1c8336` — 20 commits past the latest tag, unreleased. No user has run the buggy migration, so there is nothing on any real disk to repair; deriving `DataDir` on read is a complete fix on its own.
- **Swept for the same bug class elsewhere before calling this done.** `data_dir` was a stored absolute path that survived a root move; the same failure mode would apply to any other persisted absolute path derived from the pre-migration root. Grepped for JSON fields shaped like a path/dir/location/recent/last-opened (`AppSettings` in `app_projects.go` — only holds `AutoSaveSeconds`) and for a recent/last-opened-project concept anywhere in the Go code: none exists. `data_dir` was the only instance of this pattern in the codebase.

### 2026-08-04 — Frontend coverage: measured, not just counted; one scoped tranche added

- **"57 vitest tests across 233 Svelte files" is a file-count ratio, not a coverage measurement, and doesn't say where the risk actually is.** Installed `@vitest/coverage-v8` (devDependency) and ran `vitest run --coverage`. The default report only counts files already imported by a test and showed 61% — misleading, since it silently excludes everything untouched. Re-run with `--coverage.all --coverage.include='src/**/*.{ts,svelte}'` — now `npm --prefix frontend run test:coverage`, added as a `package.json` script so the correct invocation is encoded rather than left as prose someone has to retype exactly — to force every source file into the denominator: **true repo-wide coverage before this tranche was 14.36% of statements (1895/13195)**. Most of that gap is presentational `.svelte` components; the codebase has very few pure-logic `.ts` modules outside components (`theme.ts` and the two `charts/*.ts` helper modules were already at 90–100%).
- **Did not attempt to close the ratio.** Writing render-smoke tests for 200+ components would move the file-count ratio without adding real signal — a test that only proves a component doesn't throw on mount isn't break-verifiable against a specific regression, which is the standard the rest of this session's work was held to. Chose one bounded, real class instead: pure logic modules with zero coverage and genuine correctness stakes if they silently regress.
- **Checked first whether this session's own path-echoing risk (flagged when planning this work) was already covered.** Grepped for components referencing `.gopmgr`/`.pmforge`/`exports`/`DataDir`: only `HelpGuide.svelte` and `Dashboard.svelte` are real path-echoing components, and both are already pinned by `src/lib/persistence-boundary-strings.test.ts` from earlier in this rename. `RiskMatrixEditor.svelte`/`StatsChart.svelte`/`ProjectSettings.svelte` matched the grep on unrelated uses of "exports"/"gopmgr" — not a gap.
- **Added, both previously at 0% coverage, both break-verified (one assertion each mutated, confirmed red, reverted):**
  - `src/lib/terminology.test.ts` — pins `term()`/`capitalised()`: per-methodology overrides (scrum/kanban/cpm), fallback to the generic English word for a methodology that doesn't override a given term, case-insensitive methodology matching, and fallback for an unrecognised methodology. This table drives user-visible labels across the whole GUI (`$: term(project.methodology, 'task')`); a wrong entry silently mislabels every work item for that methodology, with no error and no visual signal it's wrong.
  - `src/lib/components/charts/_flow_shapes.test.ts` — pins `shapePath`/`shapeFill`/`shapeTextFill`/`edgePath`/`edgeLabelPosition`, the shared SVG geometry for `WorkflowEditor`/`ActivityEditor`. Pins exact path strings for the oval/diamond/parallelogram shapes and the offset math for edge routing and label placement — geometry bugs here render silently wrong (a lopsided diamond, an edge through the wrong point) rather than throwing, so nothing else in the suite would catch a regression. The `a_decision matches decision` case is a consistency pin only — it would stay green if both branches were mutated identically to the same wrong value; the exact-path assertion in the `decision` case above it is what actually break-verifies the diamond geometry.
- **Result: 61 → 81 vitest tests (+20), 14 → 16 test files.** True repo-wide statement coverage moves from 14.36% to 14.69% (1939/13195) — 20 new tests barely move a 13,195-statement denominator, and the point of this tranche was the two modules it pins, not the percentage. The remaining gap is the same untested `.svelte` mass described above.
- **Left uncovered, on purpose:** every other 0%-covered `.svelte` component (chart editors, sigma workspace views, project-settings sub-forms). These are presentational-plus-logic Svelte components, which need `@testing-library/svelte` component-mount tests rather than plain unit tests — a materially larger and different-shaped effort than this tranche. If picking this back up, prioritize by the same standard applied here: components where a wrong value can silently corrupt stored project data (form/validation logic), not by file count.
- **`@vitest/coverage-v8` is committed as a real devDependency, not a one-off local install.** Without it and the `test:coverage` script, the 14.36%/14.69% figures in this entry aren't reproducible by the next person — an unverifiable number in a handbook is worse than none. `npm audit` after the install still reports 0 vulnerabilities (15 packages added, all coverage-tooling transitive deps). `frontend/coverage/` (the report output directory) is now gitignored.
- **`test:coverage` is a manual measurement command, not a gate.** It is not called by `make frontend-stability`, `make verify`, or any CI workflow, and no coverage threshold is enforced or configured. Run it on demand; don't assume `make verify` failing (or passing) says anything about coverage.

### 2026-08-04 — `make verify` crashed locally on Homebrew's `makensis`; added an explicit, CI-inert local skip

- **`scripts/validate-windows-nsis-template.sh` (step 3 of the `windows-installer-scaffold` `make verify` target) aborts with `libc++abi: ... std::bad_alloc` on this machine's Homebrew `makensis` (3.12, `arm64_tahoe` bottle, macOS 26.6).** Isolated to the toolchain, not GoPMgr's `.nsi` template: a trivial 3-line fixture (`OutFile`/`Section`/`SectionEnd`, no GoPMgr content) crashes identically. `brew reinstall nsis` reinstalled the same bottle and did not clear it — an upstream Homebrew/NSIS arm64/macOS-26 build defect, not something fixable from this repo.
- **CI is unaffected.** `.github/workflows/release.yml` only reaches this script under `if: runner.os == 'Windows'`, where NSIS is installed via `choco install nsis` (native Windows `makensis.exe`), never Homebrew. The blast radius of this bug is exactly one platform's local `make verify` run, not any release artifact.
- **Fix: `GOPMGR_SKIP_NSIS_COMPILE`, an explicit developer-set env var, skips only the `makensis` invocation and the output-file check that depends on it.** Everything above it in the script — pinned Wails asset presence, `wails_tools.nsh` template field resolution — still runs. Unset by default, so a normal `make verify` still crashes loudly on a genuinely broken local toolchain rather than silently reporting success; this is *not* auto-detection of "makensis present but broken," which would risk masking a real break on a machine where NSIS actually works. Prints a line making clear a skip is not a pass. Verified: unset still reproduces the abort (exit 134); set, it exits 0 with the skip message; `check-windows-installer-scaffold.sh`/`_test.sh` (which assert on the literal `bash scripts/validate-windows-nsis-template.sh` line in the workflow file, not the script's internals) are unaffected either way; `GOPMGR_SKIP_NSIS_COMPILE=1 make verify` now passes end-to-end on this machine.
- **Not attempted:** pinning an older Homebrew `makensis` bottle (`brew extract` / a pinned-formula install) to find a working version. Low odds of success against what presents as a genuine upstream build defect, and multi-turn effort disproportionate to a local-only, CI-inert issue.

---

### 2026-08-04 — 100% coverage: scope, exclusions, and the ratchet (Phase 0)

- **Goal, agreed with the user before any implementation:** full-repo 100% statement coverage, defined precisely rather than left as an aspiration. "100%" here means: `scripts/coverage-go.sh default` AND `scripts/coverage-go.sh duckdb` (each excludes `frontend/node_modules` at the package-list level, per `release-gate-scope-check.sh`'s "Go quality gates must target `. ./internal/...` instead of `./...`" rule, plus the platform-file exclusions below) both independently at 100% of statements, AND `npm --prefix frontend run test:coverage` (full `src/**/*.{ts,svelte}` denominator, not the default imported-files-only report) at 100%. Frontend tests must assert real behavior (props → correct render, interaction → correct state/callback, bad input → rejection) and be break-verified; render-smoke tests that only prove a component doesn't throw don't count. This deliberately extends, not reverses, the standard recorded in the "Frontend coverage: measured, not just counted" entry above.
- **Two build configurations must each reach 100% independently, not be merged into one number.** `internal/analytics` compiles `stub.go` under the default build and `duckdb.go` under `-tags duckdb` — different files, different statement counts (100% vs 80.6% today). `go tool cover` cannot merge profiles from two different file sets into one meaningful percentage; `scripts/coverage-go.sh <default|duckdb>` runs each independently and every downstream number (the ratchet, the eventual hard gate) stays two numbers, not one.
- **Excluded from the Go denominator, defined in `scripts/coverage-exclude-go.txt` (the one authoritative, machine-read list — this entry describes it, the file enforces it):**
  - `internal/applog`'s dialog/opendir split is 3-way (`_darwin.go`, `_windows.go`, `_other.go` via `!darwin && !windows`), not 2-way — all 6 files excluded, symmetrically. This was caught by adversarial review before the first commit: CI (`.github/workflows/ci.yml`) runs `make verify` on `ubuntu-24.04`, where the `_other.go` arm compiles instead of the `_darwin.go` arm this list was first written against on a macOS dev machine. Excluding only the darwin/windows arms (as originally written) would have made the Go coverage denominator platform-dependent — CI's first run would have counted the `_other.go` files (mostly uncovered) that the macOS-recorded baseline never saw, producing a false "REGRESSED" failure on the very first push. All three arms are excluded so the denominator is identical on every host.
  - `internal/templates/jdm_windows.go` — windows-only, excluded. Its sibling `internal/templates/jdm.go` (`!windows`) is **not** excluded: it compiles on both this macOS dev machine and CI's `ubuntu-24.04`, so it's ordinary cross-platform business logic, covered normally like any other file — the platform split here isn't symmetric the way `applog`'s is, and treating it as if it were would wrongly exclude 121 lines of code CI can actually exercise.
  - 6 platform-narrow files total, 219 lines. Genuinely uncoverable on a single-host Go toolchain without cross-GOOS CI; not the same case as the `runtime.GOOS ==` conditionals below.
  - `gopmgr/frontend/node_modules/flatted/golang/pkg/flatted` — vendored third-party Go code `go list ./...` sweeps in incidentally. Excluded at the package-list level in `scripts/coverage-go.sh` (never even compiled for coverage), not just filtered from the profile.
  - `main()`'s literal entrypoint — conventional exclusion for any Go program's bootstrap function; will be documented per-function when Phase 4 (root/App layer) reaches it, not blanket-excluded here.
- **NOT excluded, refactored instead:** the 6 `runtime.GOOS ==` conditionals in `main.go` (4) and `internal/users/store.go` (2, inside `DefaultRootDir`/`legacyRootCandidates` — the exact functions behind the `f1a5501` migration data-loss fix). These compile on every platform; both arms are reachable in a single test run once the OS check sits behind an injectable seam. Checked first whether the 5 excluded platform-suffixed files are *selected by* one of these 6 conditionals (they're not — filename-suffix build tags and `runtime.GOOS` checks don't interact here), so the exclusion list and the seam refactor are independent changes. The seam refactor is scoped as its own commit, sequenced after `internal/cli`/`internal/templates` in Phase 1, specifically because `store.go` is the file this session's data-loss fix (`f1a5501`) landed in — `TestPreRenameInstallIsUsableAfterMigration` and the rest of `internal/users`'s suite must pass unchanged before and after, and the seam itself must be break-verified in both arms.
- **Frontend exclusions:** `frontend/wailsjs/` (generated Wails bindings) — already outside the `src/**` include glob, no action needed. Nothing else currently excluded.
- **Enforcement: a ratchet, not an immediate hard floor — and NOT currently wired into `make verify`.** `scripts/coverage-ratchet.sh` (run manually, or via `make coverage-ratchet`) fails only if any of the three marks (go_default, go_duckdb, frontend) drops below `coverage-baseline.json`'s recorded high-water mark — comparing exact covered/total statement counts as a float ratio, not a rounded percentage, so a one-statement regression is still caught. A hard 100%-required gate would fail on every commit until the very last one of what's estimated (see below) to be a multi-session effort, blocking all unrelated work for the duration; the ratchet lets other work land throughout and converts to a hard floor once 100% is actually reached. It is deliberately **not** a `verify` prerequisite yet: CI's `verify` job (`.github/workflows/ci.yml`) builds with `GO_TEST_TAGS=webkit2_41` only and installs no DuckDB CGO toolchain, so `coverage-go.sh duckdb` would hard-fail there today — a build failure the `GOPMGR_ALLOW_COVERAGE_REGRESSION` override below can't rescue, since that only skips the regression check, not a compile error. Caught by adversarial review before committing (an earlier draft wired it into `verify` and would have broken CI on the first push). Wire it in once CI installs the duckdb toolchain, or in Phase 7 when this converts to a hard floor. `make coverage-ratchet-update` re-runs and records any *improved* marks — run it after adding tests, never to paper over a real regression. Break-verified: a synthetic inflated baseline value correctly failed the ratchet (exit 1, "REGRESSED" reported), restored and confirmed exit 0.
- **`GOPMGR_ALLOW_COVERAGE_REGRESSION`, an explicit developer-set escape hatch, same shape as `GOPMGR_SKIP_NSIS_COMPILE`.** Caught by adversarial review before the first commit: a strict ratchet with no override fails `make verify` on *any* unrelated commit that adds a not-yet-tested line — a bugfix, a small feature, an untested error branch — for the entire multi-session duration of this effort. That's the same "blocks all unrelated work" problem the user chose a ratchet specifically to avoid, just moved one level down. Setting the var skips the fail-on-regression check for one run; it still computes and prints the real numbers, and `--update` still refuses to record a worse mark even with the var set, so a regression can be let through for one commit but can never get silently baked in as the new floor. Break-verified: with a synthetic regression, unset failed (exit 1), set exited 0 while printing "REGRESSED ... not failing" and leaving `coverage-baseline.json` unchanged even under `--update`.
- **The ratchet compares uncovered STATEMENT COUNT, not percentage — changed after the very first real coverage-improving commit tripped a false positive.** `internal/templates`'s cleanup (below) deleted 3 `jdm.go` branches proven dead by type, which removed statements that were already 100% covered. Deleting already-covered statements from a repo whose overall average is well below 100% mathematically *lowers* the percentage (removing above-average-coverage code from a below-average-overall pool always does) — the ratchet reported a "regression" for a change that made the codebase strictly better (less code, none of it untested). Uncovered count (`total - covered`) doesn't have this trap: deleting fully-covered code leaves it unchanged, only adding untested code or losing test coverage increases it, and `uncovered == 0` is an exact terminal condition for 100% with no float comparison. `coverage-baseline.json`'s stored `covered`/`total` fields are unchanged (uncovered is computed at comparison time); only the comparison logic in `coverage-ratchet.sh` changed. Break-verified: the original synthetic-inflated-baseline case (a negative uncovered count) still correctly fails, and this session's real deletion — which had needed `GOPMGR_ALLOW_COVERAGE_REGRESSION` under the old percentage comparison — now holds cleanly with no override. One consequence worth knowing: after a "held" (not "improved") comparison, `coverage-baseline.json`'s stored `covered`/`total` can lag what `coverage-go.sh`/`coverage-frontend.sh` print right now — e.g. after this deletion the file still shows `go_default: 8604/14606` while a fresh run prints `8602/14604`. That's expected, not a bug: `--update` only rewrites on genuine improvement (fewer uncovered), and uncovered count — the field the ratchet actually compares — is unaffected (`14606-8604 = 14604-8602 = 6002`) and self-heals the next time real progress lands. Don't "fix" a stale-looking baseline by hand-editing the file; let `--update` do it when uncovered count actually drops.
- **Cost estimate, derived from measured evidence, not guessed:** `ProjectLaunchpad.svelte` (small, cohesive) reached 91.9% (273/297 statements) with 2 well-aimed tests — ~136 statements/test. `ProjectSettings.svelte` (large, multi-section, already has a test file) sits at 48.2% (680/1,410) after 2 tests covering only its PAdES-timestamp section — most of the codebase's 233 Svelte files are architecturally closer to the latter than the former. Blending both cases: **11,256 uncovered frontend statements ≈ 80–280 additional interaction-tested component tests**, the dominant cost in this effort by roughly two orders of magnitude over the Go gap, and the reason Phase 6 (frontend components) is planned as a multi-session, batched effort rather than one pass.
- **Phase order is ROI-driven, not alphabetical:** `internal/cli` (4.5% today, almost entirely pure string-parsing functions — cheapest statements in the repo) and `internal/templates` (20.7%) come before the 70–99%-covered "completion tier" packages, which come before genuine construction-tier work (`charts/pdfrender`, `documents`, `update`, `db`, `agile`, `export`, `money`, `scripts`, `tools/update-manifest`), which comes before the root/App layer (48.2%, needs Wails-runtime-free test scaffolding), which comes before the frontend. Full phase list and the four scope decisions above were reviewed by an adversarial self-review pass before implementation began, per the user's explicit process for this work.
- **Baseline recorded this session (`coverage-baseline.json`):** go_default 8525/14611 (58.35%), go_duckdb 8603/14709 (58.49%), frontend 1939/13195 (14.69%). Everything past Phase 0 measures against these.

### 2026-08-04 — First wall: the exclusion mechanism is file-granular, `internal/templates` needed line-granular

- **`internal/templates/jdm.go`'s `Evaluate` had 4 defensive branches; adversarial review before writing any test sorted them into three different outcomes, not one.** `json.Marshal(req)` (req is `SeedRequest{Industry, Methodology string}`), the resulting `json.Unmarshal` into `map[string]any`, and `json.Marshal(result.Result)` (a `json.RawMessage`, whose `MarshalJSON` always returns `m, nil` or `"null", nil` — never an error) are all **provably unreachable by type**, not just unreached by today's tests. Per this repo's own rule against handling errors that can't happen, all three were **deleted**, which also removes them from the coverage denominator — no exclusion-list entry needed, because dead code that no longer exists can't be uncovered code. `e.z.Evaluate`'s error path, by contrast, is a real, reachable failure mode (crossing into an external C-bound library) — **tested**, using the exact fixture shape zen-go's own `TestEngine_ErrorTransparency` uses (a loader that returns an error), which exercises this package's own error-wrapping (`fmt.Errorf("templates: evaluate: %w", err)`) rather than pinning the library's internals.
- **The 4th branch, `NewEngine`'s loader "unknown key" case, is where 100% first collided with the tooling committed in `f4c5036`.** It's real, intentional defensive code — the package doc explicitly anticipates "future versions can ship organisation-specific overlay rules in a sibling JDM file," which is exactly the scenario that would call the loader with a second key. It is not dead by type, so it can't be deleted like the other three. But `internal/templates`' single JDM document (`launchpad_seeds.json`) never triggers it, and a search of zen-go's own test fixtures found no node type that causes a second loader call — reaching it would mean hand-crafting a decision graph purely to tick a coverage box, which was already ruled out as testing methodology (pins a fixture production will never produce, not this package's behavior). **`scripts/coverage-exclude-go.txt` is file-granular** (`coverage-go.sh` does `grep -Ev -f` against whole file paths in the profile) — there is no way to exclude one branch inside an otherwise-100%-reachable file without excluding the entire file, which would hide the ~15 statements around it that genuinely are covered.
- **Decision: leave it uncovered and recorded, not force a workaround.** `internal/templates` sits at 97.3% (up from 20.7%) rather than 100%. Extending the exclusion mechanism to line/branch granularity (e.g. `// coverage:ignore` comments parsed by `coverage-go.sh`) is deferred rather than built now, on a sample of one — Phase 2 (15 packages currently at 70–99%) will show whether this is a one-off or a recurring pattern before any tooling investment is justified. If it recurs, extend the mechanism then, with evidence of the real shape of the problem instead of guessing at it here.
- **A second, smaller instance of the same wall:** `seedDocument`'s "unknown document kind" branch is untestable through the public `Apply`/`applyOne` path (all 8 dispatched kinds are confirmed registered in `internal/documents` — checked, not assumed), but *is* directly reachable by calling the unexported `seedDocument` helper with an invalid kind from a same-package test, since its `kind` parameter is a plain string, not type-constrained to the 8 real values. Tested directly. This one didn't hit the wall — worth contrasting with `jdm.go`'s case so a future reader can tell "reachable via a different call path" from "reachable only by fixturing a third-party library's internals" apart.

### 2026-08-05 — Phase 1 complete: `runtime.GOOS` conditionals now covered via an injectable seam

- **The problem this closes:** `runtime.GOOS` is fixed for the lifetime of one compiled test binary — a `go test` run on this macOS dev machine can only ever exercise the `"darwin"` arm of a `runtime.GOOS == "darwin"` conditional, never the other side, no matter how the test is written. Six such conditionals existed: 4 in `main.go`'s `buildAppMenu` (App/Edit/Window menu roles vs. hand-built equivalents, and where the Quit menu item lives), 2 in `internal/users/store.go`'s `DefaultRootDir`/`legacyRootCandidates`. This is different from the file-granular platform split documented in the "100% coverage: scope and exclusions" entry above (`_darwin.go`/`_windows.go`/`_other.go` files, which genuinely can't all compile on one host) — these conditionals compile everywhere, so both arms *are* reachable in a single run once the OS check is behind a parameter instead of a direct `runtime.GOOS` read.
- **The seam pattern used everywhere here: extract a pure helper that takes `goos` as an explicit parameter, keep the public/production entry point reading `runtime.GOOS` and delegating to it.** `internal/users/store.go`: `DefaultRootDir()`/`legacyRootCandidates()` are unchanged in signature (still exported, still zero-argument, still read `runtime.GOOS`) — they now delegate to new unexported `defaultRootDirForGOOS(goos, home string) string` / `legacyRootCandidatesForGOOS(goos, home string) []string`, which tests call directly with both `"darwin"` and a non-darwin value in one test function. `main.go`: `buildAppMenu` gained a `goos string` parameter directly (it's already unexported and has exactly one call site, `buildAppOptions`, which now passes `runtime.GOOS`) — no pure-helper split needed since the whole function was already a reasonable unit to parameterize.
- **Proof the seam actually matters, not just a coverage-number exercise:** for each of the 3 mutations tested against `store.go`'s non-darwin/darwin branches, the *existing* `runtime.GOOS`-dependent tests (`TestDefaultRootDirPlatformDefault_UsesGoPMgrLeaf`, `TestLegacyRootCandidates_PrefersApplicationSupportOverDocuments`) stayed **green** under a mutation to the branch that isn't real-`runtime.GOOS` on this machine (they can only ever exercise the darwin arm here), while the new pure-helper tests correctly went red. That's the concrete, observed failure mode this refactor closes — not hypothetical.
- **`internal/users/store.go` is the file `f1a5501`'s migration-fix landed in.** Every existing test in the package, including `TestPreRenameInstallIsUsableAfterMigration`, was confirmed to pass unchanged both before and after this change — this session's standing rule (own commit, no bundled unrelated changes, existing suite verified around sensitive code) was followed exactly as planned when this item was first deferred from Phase 1.
- **Also closed, adjacent and cheap:** the `os.UserHomeDir()` error branch shared by both `DefaultRootDir` and `legacyRootCandidates` — unrelated to the GOOS seam, but a one-line `t.Setenv("HOME", "")` reliably and hermetically triggers it on POSIX (skipped on Windows, where `os.UserHomeDir` reads different env vars).
- **`main.go`'s `buildAppMenu` moved from 57.9% to 71.1%, not higher — the remaining gap is scoped out on purpose.** The uncovered lines are menu-item `Click` callback *bodies* (the closures that call `wailsruntime.EventsEmit`, `wailsruntime.Quit`, `wailsruntime.WindowToggleMaximise`, `wailsruntime.MessageDialog`, etc.) — these need a live Wails runtime context (`app.ctx`) to exercise meaningfully; calling them with a nil context just hits the existing `if app.ctx != nil` guard and returns without touching the Wails call. Covering these belongs to Phase 4 (root/App layer, which already has a pattern for constructing a Wails-runtime-free `*App`) rather than this narrowly-scoped GOOS-seam task — noted here so a future reader doesn't assume `buildAppMenu` was meant to reach 100% in this pass.
- **Test coverage ledger:** a new `TEST_COVERAGE_LEDGER.md` catalogs every test file in the repo (Go and frontend) with what it covers, how, and why — see that file for the authoritative, itemized inventory. This handbook continues to record *why* non-obvious testing decisions were made; the ledger records *what exists*.

### 2026-08-05 — Phase 2 begins: `internal/applog` to 100%; adversarial review of `a0360b5`; ledger drift enforcement

- **Retroactive adversarial review, per standing process.** `/advisor` had been unavailable for the entire prior session (flagged and disclosed at the time); it recovered this session and reviewed `a0360b5` (the Phase 1 completion commit) before any Phase 2 work started, as promised. Verdict: sound — the seam pattern and its break-verify proof were the strongest evidence in the commit. Two non-blocking findings, both addressed in this same commit rather than deferred:
  1. `TEST_COVERAGE_LEDGER.md`'s `internal/db` row for `backup_test.go` said schema-version dispatch covers "v1 `.pmforge` vs. v2 `.gopmgr` entries" as if one test proved both; the v1 *read* path is specifically pinned by `TestRestoreArchivalBundleReadsSchemaVersion1Archive`, the v2 path by the create/round-trip tests. Row corrected to name the specific tests.
  2. The ledger itself had no enforcement — a 150+-row hand-written document, in a repo where the whole point of this multi-session effort has been "don't trust prose, trust something that fails loudly." Addressed below.
- **Ledger methodology disclosed in the document itself.** Added a "Methodology and confidence" note to `TEST_COVERAGE_LEDGER.md`'s Purpose section: rows are written from test names, source, and doc comments, not fully re-verified against every assertion; a sample was checked directly during this review and held, but any single row should be treated as correctable, not authoritative. Converts an unstated risk into a declared one.
- **`scripts/check-coverage-ledger-current.sh` (new), wired into `make verify` as the `coverage-ledger-current` target.** Deliberately narrow: it only checks that every `*_test.go`/`*.test.ts` file's basename appears somewhere in `TEST_COVERAGE_LEDGER.md` — a presence check, not a content audit. It cannot catch a row that's become inaccurate for a file that still exists (that's what the methodology note above is for), only the more common and more silent failure mode: a new test file added and the ledger never updated, which nothing else in the repo would ever notice. Cheap (pure `find`/`grep`, no Go build, no test execution) so it's safe inside `verify`'s fast pre-commit gate, unlike `coverage-ratchet` which stays out for the DuckDB-toolchain reason recorded above. Prunes by `-name node_modules` (not a hardcoded `./frontend/node_modules` path), so it doesn't fail `verify` for unrelated reasons if a `node_modules` ever appears elsewhere in the tree — the same "don't block unrelated work" concern already applied to the coverage ratchet and the NSIS skip. Break-verified: an untracked `zzz_fake_test.go` dropped into `internal/applog/` was correctly flagged and failed the check; removing it restored a clean pass. Its own limit showed up on first use: the `internal/applog` row this same commit added initially said "15" tests where the file has 14 (an off-by-one from hand-counting, caught by adversarial review, not by this script) — the gate can prove a file is *mentioned*, not that a row's details are *right*, exactly the boundary the methodology note above states.
- **`internal/applog`: 72.5% → 100% of `applog.go`'s own statements (95.7% of the package total; the platform-narrow `dialog_darwin.go`/`opendir_darwin.go` stay excluded per the existing exclusion list).** The gap was concentrated in `Init`'s three "never fail outright" fallback branches and all of `Fatal` — chosen as the Phase 2 starting package per the advisor's guidance (`go tool cover -func` showed its gap was error-path shaped, cheap to close, vs. `internal/crypto`'s gap being spread across PDF/CMS signing fixture construction, materially more expensive per statement; advised finishing one cheap package first to get a real statements-per-hour figure before estimating the rest of the 15-package tier).
  - `Init`'s `MkdirAll`-fails and `OpenFile`-fails branches: forced with real-but-controlled filesystem obstacles — a plain file sitting where the `logs` subdirectory needs to be created (`MkdirAll` returns `ENOTDIR`), and the exact dated log filename Init will try to open pre-created as a directory (`OpenFile` returns "is a directory"). Chosen over `chmod`-based permission denial specifically because tests running as root (a real CI/container condition) bypass Unix permission checks entirely, which would make a permission-based test silently pass for the wrong reason on such a host.
  - `Init`'s "no writable log directory found" branch and `resolveLogDir`'s matching final-fallback logic: `os.TempDir()` always returns a non-empty value on every real POSIX/Windows host (`/tmp` default on Unix even with `TMPDIR` unset), so this branch is genuinely unreachable via environment manipulation on any real machine — the same class of "hard case" flagged for `internal/templates` in the entry above. Rather than leave it undocumented-and-uncovered again, `userHomeDir`/`tempDir` were extracted as package-level func vars (`var userHomeDir = os.UserHomeDir`, `var tempDir = os.TempDir`) — the identical injectable-seam pattern used for `runtime.GOOS` in Phase 1 — so tests can force both to fail and observe the real fallback behavior.
  - `Fatal` needed the same treatment regardless of any filesystem trick: it calls `ShowError` (a real native OS dialog via `osascript` on darwin) and `os.Exit(1)`, neither of which a test can safely call directly. Added `showError`/`osExit` func vars alongside the two above; `Fatal`'s body now calls the vars instead of `ShowError`/`os.Exit` directly, and tests substitute closures that record what was called instead of firing them for real.
  - `pruneOldLogs`'s `os.ReadDir` error branch: closed directly with a nonexistent directory, no seam needed.
  - Break-verified 5 distinct mutations: `Init`'s `MkdirAll`-fails branch, `Init`'s `OpenFile`-fails branch, `Init`'s `logDir == ""` branch, `resolveLogDir`'s temp-fallback branch, and `Fatal`'s `showError` call. All were eventually caught, but 2 of the 5 were **not** caught on the first attempt — worth recording precisely because this session's whole methodology rests on break-verify being trustworthy evidence, and the first pass here wasn't:
    - The `MkdirAll`-fails mutation (removing only the early `return`, leaving the log message in place) fell through into the `OpenFile` branch, which failed for its own reason and produced the identical `logPath == ""` observable — the test as originally written asserted only the outcome, not which branch produced it, so it stayed green under a mutation it was meant to catch.
    - The `logDir == ""` mutation had the identical failure shape: removing that branch let an empty `logDir` fall through to `os.MkdirAll("", dirPerm)`, which also fails and produces the same `logPath == ""`. Same root cause, found by re-running the same class of check against the second branch rather than assuming the first fix generalized.
    - Both fixed the same way: capture the actual log message (a real `os.Pipe()`-based stderr capture, since `Init`'s failure branches call `log.SetOutput(os.Stderr)` themselves and would silently discard a `log.SetOutput(&buf)` done from the test side) and assert on the specific text each branch emits ("cannot create log dir" vs. "no writable log directory found"), which only that branch produces. Re-running the same mutations against the tightened tests then failed correctly.
    - `captureStderr`'s restore is registered with `t.Cleanup` and its returned `stop()` is idempotent — caught in the same review pass: an `os.Stderr`-redirecting helper that only restores via an explicit `defer`/call site leaves the real `os.Stderr` pointed at an unread, unclosed pipe if the test exits early via `t.Fatalf` (which never returns to run code after it), silently swallowing every later test's stderr output in the same package and risking a full pipe buffer blocking a write.
    - `TestInit_OpenFileFails` blocks both today's and tomorrow's dated filename, not just today's: the test computes "today" once to build its filesystem obstacle, then `Init` calls `time.Now()` again itself: a run straddling midnight between the two calls would otherwise flake.
  - One test in this batch is disclosed as non-discriminating rather than fixed: `TestPruneOldLogs_UnreadableDirIsANoOp` exercises `pruneOldLogs`'s `os.ReadDir`-error branch for the coverage ratchet, but the branch's entire contract is "return early, touch nothing" — deleting that early return leaves `entries` nil, and ranging over a nil slice still returns cleanly with zero iterations, so no assertion this test could make would distinguish the branch running from the branch being deleted. Recorded here rather than left as an implicit gap, per this section's own "treat stale/overstated claims as bugs" standard.
- **Coverage ratchet updated:** go_default 8612/14605 (5993 uncovered) → 8628/14605 (5977 uncovered); go_duckdb 8690/14703 (6013 uncovered) → 8706/14703 (5997 uncovered). frontend unchanged (no frontend files touched this pass).

## 10. Quick map: "where do I add ..."

| Task                                      | File(s) to touch                                                          |
| ----------------------------------------- | ------------------------------------------------------------------------- |
| New chart kind                            | `internal/charts/registry.go` (Definition entry); pick or add engine pkg; engines.go switch; new Svelte editor; App.svelte route; Dashboard card. |
| New document kind                         | `internal/documents/registry.go` (Kind const + Definition in templates.go). Frontend create path is automatic: Dashboard fetches `ListDocumentKinds()` and renders a button per kind; the `documents` route in `App.svelte` already points to `CharterEditor` which handles any kind generically. |
| New document bespoke PDF renderer         | `internal/documents/<kind>.go` with `Render<Kind>PDF()`; switch in `documents.Render()`. |
| New database column                       | `internal/db/sqlite.go` Migrate() — additive only.                        |
| New CLI flag                              | `internal/cli/parser.go` Config struct + flag.*Var; handle in main.go.    |
| New Wails-exposed App method              | Add to `*App` in the root `main.go`; declare in `frontend/src/wails-window.d.ts`. |
| New shared editor pattern                 | `frontend/src/lib/components/charts/_*_shell.svelte` (snippet-based).     |
| Change SPDX license for a directory       | Update each file's header; add the SPDX ID to `LICENSES.md`.              |

---

**End of handbook.** Keep this file lean — link to source rather than duplicate it. Source is the ground truth; this file is the map.
