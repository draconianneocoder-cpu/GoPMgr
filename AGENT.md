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
- **Rules engine**: `github.com/gorules/zen` (MIT) via its official Go binding (zen-go) — Launchpad seeding rules expressed as JDM data, not Go switch. Used by `internal/templates`.
- **Holiday data**: `rickar/cal/v2` (BSD-2-Clause) — country holiday datasets. Wrapped by `internal/calendar`.
- **CMS/PKCS#7**: `digitorus/pkcs7` (FreeBSD-2-clause) — wraps raw RSA signatures into the CMS SignedData structure Adobe Reader / PAdES expects. Used by `internal/crypto/pdf_sign.go`.
- **DOCX writer**: `gomutex/godocx` (MIT, pure Go) — picked from pkg.go.dev after a survey. Used by `internal/export/docx.go`. ODT export (`internal/export/odt.go`) is hand-built because no equivalently-maintained pure-Go ODT generator exists (kpmy/odf hasn't been touched since 2014).

The app has reached **V2.x** maturity: all 20 chart kinds and all 25 document templates implemented end-to-end, combined report builder with embedded vector chart visualisations, self-heal with atomic database swap, multi-user accounts. The Agile Pack is the current frontier.

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
│   │   ├── registry.go          # 20-kind taxonomy + 4 engines (DAG/Stats/Matrix/Flow)
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
- **`settings`** — singleton row (CHECK id=1). Columns: `default_password`, `export_theme`, `auto_repair`, `cert_path`, `signature_enabled`, `default_font` (document-export font family; empty = catalog default), `agile_enabled` (Software-Dev Pack toggle; persisted so the pack state survives project close/reopen). `default_font` and `agile_enabled` were added 2026-05-20 and 2026-06-04 respectively via the `settingsMigrations` loop in `migrateLegacyColumns` (PRAGMA-probe pattern covering both `project` and `settings`).
- **`tasks`** — V1 scheduler tasks: `id`, `title`, `duration`, `precedents` (JSON array of IDs), `created_at`, `updated_at`.
- **`command_log`** — append-only command journal: `id`, `ts`, `actor`, `command`, `payload` (JSON).
- **`audit_log`** — `id`, `ts`, `actor`, `action`, `target_id`, `details`. Indexed by target_id and ts.

### V2 tables (multi-entity model)
- **`project`** — one row per .pmforge: `id`, `name`, `description`, `status`, `phase`, `start_date`, `end_date`, `budget`, `owner`, timestamps. Status ∈ {planning, active, on_hold, complete, cancelled}. Phase ∈ {initiation, planning, execution, monitoring, closing}.
- **`charts`** — unified table for all 20 chart kinds: `id`, `project_id`, `kind`, `title`, `data` (JSON), `config` (JSON), `template_id`, timestamps. FK ON DELETE CASCADE.
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
make test              # go test ./cmd/... ./internal/... (PMForge-owned Go packages)
make race              # go test -race ./cmd/... ./internal/... (V2.x concurrency hardening gate)
make memory-scan       # scripts/memory-safety-scan.sh (V2.x)
make frontend-stability # svelte-check --fail-on-warnings + Sigma regression gates
make frontend-build-budget # Vite build without large main bundle regressions
make license-check     # reuse lint
make check-release     # version consistency + REUSE + build

# Packaging (host-local deterministic tarballs; cross-platform targets require matching runners)
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
1. `go vet ./cmd/... ./internal/...` — standard correctness checks.
2. Custom grep gate (scripts/memory-safety-scan.sh) for:
   - `os.Open(` without nearby `defer .*Close()`
   - `sql.Open(` without nearby `defer .*Close()`
   - `unsafe.Pointer` (forbidden in this codebase)
   - missing `errors.Is`/`errors.As` against package `Err*` sentinels
3. (When installed) advisory `staticcheck ./cmd/... ./internal/...` for deeper analysis.
4. (When installed) advisory `gosec ./cmd/... ./internal/...` for security-flavoured patterns.

A new contribution should land **with `make memory-scan` passing**. Optional scanners report findings without failing by default so the release gate is not dependent on locally installed tools; set `PMFORGE_STRICT_OPTIONAL_SCANS=1` when you want optional staticcheck/gosec/govulncheck findings to fail the gate. The gate is wired into `make check-release`.

---

## 8. Feature coverage (live status)

### Charts: 20/20 implemented end-to-end (UI + Go layout + frontend renderer + PDF embed)
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
- **Export & Signature settings in Project Settings panel.** `ProjectSettings.svelte` now reads/writes `export_theme`, `auto_repair`, `cert_path`, `signature_enabled` from the settings DB row. Font picker (family dropdown + Import button) also lives there.
- **Ctrl+S keyboard shortcut in all editors.** `CharterEditor.svelte`, `_layered_editor_shell.svelte`, and `_stats_editor_shell.svelte` all register a `keydown` listener in `onMount` and remove it in `onDestroy`.
- **Dirty indicator and status dropdown in CharterEditor.** Baseline `lastSavedContent`/`lastSavedTitle` set after load; `dirty` derived state drives an amber "Unsaved changes" badge. Status dropdown (`draft|review|approved|archived`) in the header calls `save()` on change.

### Agile Pack (V2.x — complete)
- **Backend**: schema (5 tables in db/sqlite.go), types (agile/agile.go), CRUD storage (agile/store.go), DORA metrics with elite/high/medium/low classification (agile/dora.go), Wails methods in cmd/pmforge/main.go §Agile Pack.
- **Frontend**: KanbanBoard (drag-and-drop with WIP badges), Backlog (priority + drag reorder + Start-work), SprintList (planning/active/complete lifecycle with single-active invariant), DORADashboard (4 KPI cards + deploy-trend line via StatsChart + inline +Deployment form). All live under `frontend/src/lib/components/agile/`.
- **Wiring**: 4 new session view union members (`kanban`, `backlog`, `sprints`, `dora`), App.svelte routes, Dashboard "Software-Dev Pack" section with enable/disable toggle backed by `App.AgileEnabled` / `App.SetAgileEnabled`. As of 2026-06-04, `AgileEnabled` is **persisted to `settings.agile_enabled`** (not in-memory only); `SetAgileEnabled` does a DB roundtrip and updates `agile.PackEnabled` as a cache.

### Memory & concurrency gates (V2.x)
- **`make memory-scan`** runs `scripts/memory-safety-scan.sh`. Currently passing in the sandbox; on a dev box with Go in PATH it also runs `go vet` and a Go-helper scan for unclosed `os.Open` handles. Optional integrations: `staticcheck`, `gosec`, `govulncheck` — auto-detected.
- **`make race`** runs `go test -race ./cmd/... ./internal/...`.
- Both are wired into `scripts/check-release.sh` so the release gate fails if either does.

### Remaining V2 TODOs (status snapshot)
1. ~~DOCX / ODT export.~~ **Done.** `internal/export/docx.go` uses `gomutex/godocx`; `internal/export/odt.go` is hand-built (no maintained ODT library exists). App methods `ExportDocumentDOCX` / `ExportDocumentODT`.
2. **PDF/A-3 strict conformance** — partial, advanced 2026-05-20, 2026-05-25, and 2026-06-06. (i) The dependency-free `internal/pdfmeta` package builds the canonical XMP packet AND injects it into the PDF Catalog via a spec-conformant **incremental update** (`InjectXMPStream`); `documents.Render()` tags every generated PDF (fail-soft). (ii) **Font embedding is now available** via `internal/fonts` — bundled TrueType families (fetched by `make fonts`) embed into PDFs through the "register under Helvetica" trick, replacing the non-embeddable core fonts. (iii) OutputIntent + ICC profile injection is implemented (`InjectOutputIntent`, `MakePDFA3`, `make icc`) and used when an ICC profile is embedded. (iv) The schedule-report, document, and combined-report samples now pass `make check-pdfa` with veraPDF's PDF/A-3b profile after adding binary header comments, trailer IDs, stream-length correctness, latest-incremental Catalog rewrites, and embedded Source Sans 3 for representative exports. Remaining for full release-grade conformance: prove the expanded soft gate is stable on release builders before promoting `make check-pdfa` to a hard release gate.
3. ~~CMS/PKCS#7 + PAdES signature widget embedding.~~ **Done** via `digitorus/pkcs7` plus `pdfmeta.InjectPAdESSignature`. The PAdES path appends a `/Sig` dictionary, invisible `/Widget` field, `/AcroForm`, fixed-width `/ByteRange`, and padded `/Contents` in the final incremental update. `make check-pades` verifies the local invariant, and `make check-pades-external` extracts the embedded CMS for OpenSSL detached verification, checks `qpdf --check`, and requires `pdfsig` to report a valid signature when those tools are installed; remaining hardening is Acrobat/DSS validation coverage when available.
4. ~~Wails file-picker for certs.~~ **Done.** `App.ChooseCertFile` calls `wailsruntime.OpenFileDialog`.
5. ~~HTTPS update channel with signed release manifest.~~ **Done.** `internal/update` fetches a signed JSON manifest, verifies Ed25519, returns `Status`. `ManifestURL` and `UpdateChannelPublicKey` set at build time via `-ldflags`.
8. ~~Per-user database encryption-at-rest decision.~~ **V2 stopgap decided.** README documents OS-level disk encryption (FileVault / BitLocker / LUKS) as the supported V2 path for raw-disk theft or admin-level host access. `scripts/release-gate-scope-check.sh` guards that README keeps both the OS-level encryption guidance and the SQLCipher/V3 deferral clear. Native database encryption remains a V3 design item because SQLCipher adds native packaging complexity and whole-file AES-at-rest needs crash-recovery semantics.
9. **Bespoke renderers for the 24 non-Charter document kinds** — Status Report shipped as the reference (`internal/documents/status_report.go`). The other 23 follow the AGENT.md §10 recipe. Generic renderer keeps every kind exportable in the meantime.
10. ~~Embed chart visualisations in combined reports.~~ Done in earlier slice.
13. ~~Account recovery codes.~~ **Done.** 8 Argon2id-hashed codes generated at account creation, redeemable once each. `App.IssueRecoveryCodes` + `App.ResetWithRecoveryCode`. Frontend: `RecoveryReset.svelte`.

### Still deferred to V3
- Strict PDF/A-3 release claim — the schedule-report, document, and combined-report samples now pass local veraPDF PDF/A-3b validation; still needs release-builder soak before the GUI/docs claim strict conformance as a hard gate.
- External PAdES validation hardening — the widget is embedded and locally sample-verified by `make check-pades`; OpenSSL detached CMS verification plus local `qpdf`/`pdfsig` checks are covered by `make check-pades-external`, but sample signed PDFs still need Acrobat/DSS validation before treating the implementation as fully battle-tested.
- Per-user database encryption at rest (SQLCipher/native implementation design).
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
- **What's next.** Bespoke coverage saturated. The next investment areas per AGENT.md §8 are: (a) PDF/A-3 strict conformance validation (veraPDF gate hardening now that font embedding, Catalog XMP, and OutputIntent/ICC code exist), (b) external PAdES validator hardening for signed sample PDFs, (c) per-user encryption at rest (SQLCipher), (d) PDM date-dragging on the Timeline. All four are V3 milestones requiring significantly larger slices.

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
- **REUSE.toml added** (first one in the repo) to declare licenses for fetched `.ttf` binaries, embedded ICC profiles, generated lockfiles, and other files that cannot carry inline SPDX headers. OFL-1.1 + LicenseRef-Bitstream-Vera are documented in LICENSES.md.
- **FOUND + FIXED a latent compile error**: `internal/documents/report.go` called `pdf.GetPageHeight()`, which does NOT exist in the pinned gofpdf v1.16.2 (it has `GetPageSize() (w, h)`). The `documents` package had therefore never compiled — masked in the sandbox because `export` always failed first on godocx/pkcs7 resolution. Fixed to `_, pageH := pd.GetPageSize()`. **Lesson: the combined-report chart-embed path (report.go) was shipped untested against the pinned gofpdf version. Worth a smoke test on the user's machine.** When verifying, build `./internal/documents/` in isolation — it has no godocx/pkcs7 deps and now compiles cleanly in the sandbox.
- **Remaining for the frontend**: a Settings-panel font picker (dropdown over `ListFonts()`, an "Import font…" button calling `ImportFont()`, persisted via `SetDefaultFont`). The backend is complete; this is Svelte work.
- **Sandbox build note**: `go build ./internal/documents/ ./internal/fonts/ ./internal/pdfmeta/ ./internal/charts/... ./internal/db/` all succeed. `export` and `cmd/pmforge` still can't build in the sandbox (godocx v0.1.16 + pkcs7 pinned revisions don't resolve) — a pre-existing limitation, not introduced here.

### 2026-06-04 — CPM kernel + DORA classification tests
- **`internal/kernel` now has 10 unit tests covering every branch of CalculateCPM and topoSort.** Cases: empty map, single task, linear chain (A→B→C), diamond network (A→B/C→D with longer branch on critical path), parallel equal-length paths (both critical), zero-duration milestones, cycle detection (mutual reference + self-loop). `topoSort` tests cover dependency ordering and alphabetical determinism. The package doc comment explicitly noted isolation testing was intended — this was pure overdue work.
- **`internal/agile/dora.go` now has 35 unit tests (in `dora_test.go`).** Covers all four classification functions at each band boundary (`classifyDeployFrequency`, `classifyLeadTime`, `classifyCFR`, `classifyMTTR`), the `median` helper (empty, odd, even, unsorted input), the `formatFloat1` shim (zero, whole, decimal, negative), and `ComputeDORA` end-to-end (empty, window filtering, default window fallback, elite-team scenario, daily trend length, medium CFR scenario).
- **Test misread correction: deploy-frequency thresholds.** 0.5 deploys/day is "high" (not "elite" — elite requires ≥ 1.0/day). 1/14-day is "medium" (not "high" — high requires ≥ 1/7-day). Both the code and DORA spec are correct; the initial test expectations were wrong. This illustrates why boundary tests should be written from the code, not from memory.
- **`range N` syntax is idiomatic Go 1.22+.** Used in `dora_test.go` for the elite-team loop; this Go module targets 1.26.4 so no compatibility concern.

### 2026-06-04 — Sigma tollgate + stats tests
- **`internal/sigma/tollgate` now has 23 unit tests in `readiness_test.go`.** Covers all four phase checkers (Define, Analyze, Improve, Control) and the `CheckPhase` router, including the 80%-threshold for Define (5/7 ≠ advance, 6/7 = advance), the 100%-threshold for Analyze/Improve/Control, CTQ spec-limit requirement, minimum character lengths for all five charter text fields, SIPOC element count, fishbone causes vs. 5-Whys drill-down depth (3 levels minimum), solution count + impact/effort scoring + selection, control item owner + response-plan presence, and the Measure phase auto-approve default arm.
- **`internal/sigma/stats` now has 10 unit tests in `basic_test.go`.** Covers `CalculateDescriptive` (empty error, single value, odd/even count, positive std dev), `CalculateCapability` (empty error, zero-std-dev error, Cp formula positive, Cpk < Cp for off-center process, DPMO band at sigma ≥ 6 = 3.4 defects/million).
- **Boundary-value misread lesson (second occurrence).** In the Define-phase test, "Also short." (11 chars) satisfied the BusinessCase ≥ 10 minimum. The pattern: always verify lengths in Go before writing a test that assumes a string is "too short."
- **`range N` is idiomatic in Go 1.22+ (this module targets 1.26.4).** Used in stats_test.go loops; avoids the `for i := 0; i < N; i++` boilerplate.

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
- **`internal/cli` now has 3 unit tests in `parser_test.go`.** Covers `Version` non-empty, `PrintVersion` stdout output containing "PMForge", `Version`, and "GPL" (via `os.Pipe` capture), and `Config` zero-value coherence (bool fields default false, string fields default empty). `ParseFlags()` is not unit-tested because it calls `flag.Parse()` against the global `flag.CommandLine` and `os.Args` — the safe test boundary is the banner and the type structure.
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
- **Registry has 20 chart kinds, not 19.** 6 DAG + 8 Stats + 4 Matrix + 2 Flow = 20. The off-by-one originated in the initial project scaffold comment before the 20th kind was wired up. All references to "19 chart kinds" in README.md (7 sites), AGENT.md (3 sites), and `internal/charts/registry.go` package comment are now corrected to 20.
- **"Five engines" corrected to "four engines" in two places.** `registry.go` package comment and README.md both said "five engines"; only four Engine constants exist (DAG, Stats, Matrix, Flow). The five *renderer files* in `pdfrender/` (dag, fishbone, flow, matrix, stats) are correctly five because Fishbone has its own renderer file, but the taxonomy engine count is four.
- **`make race` passes clean** across all 28 packages — no data races detected.
- **`make memory-scan` passes clean** — `go vet` clean, goroutine inventory zero PMForge spawns, gosec clean, govulncheck reports zero vulnerabilities in PMForge's own code.
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
- **Wails bridge declarations must track real `*App` methods.** Signed PDF/report exports, schedule report exports, ProjectMeta industry fields, and Sigma methods/types now live under `window.go.main.App` in `frontend/src/wails-window.d.ts`. Verify against `cmd/pmforge/main.go` before adding names.
- **Remaining frontend debt is warning-level, not compile-blocking.** `svelte-check` still reports accessibility/deprecated-event warnings, especially in Sigma helper components and the signature modal. The production build also emits the existing large-chunk warning. Treat warning cleanup as a follow-up hardening slice.

### 2026-05-25 — veraPDF gate hardening
- **`scripts/validate-pdfa.sh` now has a testable helper layer.** `scripts/validate-pdfa-lib.sh` owns compliance-output parsing, Docker path mapping, portable veraPDF executable lookup, archive validation, and stale-wrapper detection; `scripts/validate-pdfa-lib_test.sh` covers those behaviors plus an integration path with a fake veraPDF CLI.
- **Do not grep text output for `compliant`.** That false-positives on "not compliant". The gate now requests XML and accepts only explicit `<isCompliant>true</isCompliant>` (or JSON `isCompliant: true` if a future runner emits JSON).
- **Generate validation samples inside the repo, not `/tmp`.** Docker receives `/work/...` paths for samples under `.tmp/pmforge-pdfa-test`; CLI mode receives host paths. This matters because the PMForge workspace path contains spaces and Docker cannot see host-only `/tmp` paths unless mounted.
- **The sample generator must set `ExportOptions.Format`.** Missing `FormatPDF` made the old gate "pass" with no samples after `[EXPORT_FORMAT_UNKNOWN] unknown format ""`. Sample-generation failure is now a real gate failure; missing veraPDF tooling remains a soft skip.
- **Stale/corrupt veraPDF downloads are ignored.** The installer validates downloaded zip/jar files before accepting them and refreshes wrapper scripts that point at invalid jars. On this machine, Docker is absent and auto-install still cannot fetch a valid veraPDF artifact, so `make check-pdfa` skips cleanly rather than validating.

### 2026-05-25 — Frontend stability/performance hardening
- **Keep `xlsx` lazy-loaded in the Sigma import flow.** `SigmaProjectView.svelte` now imports `xlsx` only inside the spreadsheet-import path, so Vite splits it into `dist/assets/xlsx-*.js` instead of forcing every PMForge launch to parse the spreadsheet engine.
- **`scripts/frontend-stability-check.sh` protects this boundary.** The guard fails on static Sigma `xlsx` imports, deprecated Svelte 4 `on:*=` directives in Sigma components, `createEventDispatcher` usage in Sigma components, and SVG text actions without keyboard handlers in `SigmaFishbone.svelte`.
- **Sigma save notifications use Svelte 5 callback props.** `SigmaVoCCTQ`, `SigmaSIPOC`, `SigmaSolutionMatrix`, and `SigmaControlPlan` expose optional `onSaved` callbacks instead of dispatching legacy component events; parent calls should pass function props such as `onSaved={loadCharter}`.
- **Frontend warnings are now a hard gate.** `scripts/frontend-stability-check.sh` runs `svelte-check --fail-on-warnings`; future Svelte diagnostics must be fixed rather than tolerated. Current `npm run check` from `frontend/` reports 0 errors and 0 warnings.
- **Route-level feature islands are lazy-loaded from `App.svelte`.** App no longer eagerly imports every chart, document, Agile, project, and Sigma component at launch. The current production build has no Vite large-chunk warning; `index` is roughly 48 kB minified / 19 kB gzip, with heavy surfaces split into route chunks plus `StatsChart` (~188 kB) and `xlsx` (~429 kB) async chunks.
- **`scripts/frontend-build-budget.sh` protects the split.** It runs the production build and fails if Vite emits a large-chunk warning or if the main `index-*.js` chunk exceeds 500,000 bytes. Prefer lazy route/component splits over raising the Vite warning limit.

### 2026-05-25 — Release gate scope and deterministic build hardening
- **Do not use the unscoped all-packages pattern for Go quality gates in this repo.** With `frontend/node_modules` installed, it discovers npm dependency packages such as `frontend/node_modules/flatted/golang/pkg/flatted`. Use `./cmd/... ./internal/...` for PMForge-owned Go gates.
- **`scripts/release-gate-scope-check.sh` protects release wiring.** It fails on unscoped Go quality commands and requires `check-release.sh` to include the frontend stability and bundle-budget gates.
- **Optional scanners are advisory by default.** `memory-safety-scan.sh` still runs detected `staticcheck`, `gosec`, and `govulncheck`, but only mandatory checks fail by default. Set `PMFORGE_STRICT_OPTIONAL_SCANS=1` for security-focused strict runs. This avoids release-gate behavior changing just because one developer has `gosec` installed.
- **Wails CLI builds require a root Go package; PMForge's entrypoint lives under `cmd/pmforge`.** `make build` now runs the frontend budget build, syncs `frontend/dist` into `cmd/pmforge/frontend/dist` for the existing `go:embed`, and then runs `go build ./cmd/pmforge`. Passing `-compiler gcc` to Wails was wrong because Wails expects a Go compiler there; it tried to run `gcc mod tidy`.
- **`check-release.sh` now runs the complete local release gate successfully on this machine.** It verifies scope, memory safety, frontend warning-clean state, frontend bundle budget, race detector, deterministic build, and the PDF/A soft gate. `reuse` still skips if the tool is not installed.

### 2026-05-26 — Deterministic package targets
- **Package targets now use `scripts/package.sh`, not Wails CLI packaging.** The script calls the proven `make build` path, stages `pmforge` with `README.md` plus `LICENSES/`, and writes `build/packages/pmforge-<goos>-<goarch>.tar.gz`.
- **Packaging is host-local by design.** `package-darwin` runs on macOS; `package-linux` and `package-windows` fail fast with a clear message unless run on matching hosts/CI runners. This avoids pretending that CGO/Wails cross-packaging is portable from one desktop machine.
- **`scripts/release-gate-scope-check.sh` also rejects Wails CLI package invocations.** Future package target edits should keep using the deterministic script unless the repo intentionally reintroduces app-bundle packaging with a verified root-main Wails layout.

### 2026-05-26 — Strict gosec and Sigma persistence hardening
- **Strict optional scanners are now clean on this machine.** `PMFORGE_STRICT_OPTIONAL_SCANS=1 make memory-scan` passes with gosec installed; normal `make memory-scan` remains clean. Keep any future `#nosec G304` comments narrow and tied to a real product boundary, such as user-selected certificate/export/font paths or `os.CreateTemp` paths created by PMForge itself.
- **Sigma persisted JSON must fail loudly when corrupt.** `SigmaGetCharter`, `SigmaGetFishbone`, `SigmaGetSolutions`, `SigmaGetControlPlan`, `SigmaGetSIPOC`, and `SigmaGetVoC` now return contextual decode errors instead of silently treating malformed JSON as empty domain data. The regression tests insert corrupt JSON directly into SQLite so the failure mode stays covered.
- **Fishbone storage shape is full `FishboneData`, not bare branches.** `SigmaSaveFishbone` writes the full object; `SigmaGetFishbone` now reads that shape and preserves the legacy bare-`[]FishboneBranch` fallback. Without this, saved causes could disappear on reload because the previous getter ignored the unmarshal error.
- **Argon2 PHC parsing must validate bounds before calling `argon2.IDKey`.** Malformed hashes with `p=256`, zero parameters, empty salt, or empty key material can otherwise panic or truncate during conversion. Keep these checks before the `uint8` / `uint32` conversions.
- **Export and account artifacts should default private.** Sigma reports, audit CSV exports, backup bundles, the Sigma export directory, and the PMForge system root now use `0600`/`0700` permissions where PMForge owns the write path. Per-user subdirectories already used `0700`; the root now matches the isolation claim in §5.

### 2026-05-26 — Backup and audit artifact durability
- **Never string-interpolate `VACUUM INTO` paths.** A backup/snapshot destination containing a single quote used to fail with a SQLite syntax error. `CreateSnapshot` now binds the target path as a SQLite parameter, and regression tests cover both direct snapshots and `.pmba` archival bundles with quoted destination names.
- **Archival writers must finalize explicitly.** `CreateArchivalBundle` now returns errors from `zip.Writer.Close`, archive-file close, and source-file close when those are the first failure. A backup function returning nil means the zip central directory and underlying file close both completed.
- **Audit CSV export now checks flush and close errors.** `ExportAuditCSV` explicitly flushes, checks `csv.Writer.Error`, checks row iteration, and returns close errors when no earlier error occurred. The regression test verifies a private `0600` CSV with comma/newline escaping intact.

### 2026-05-26 — Update-channel fail-closed hardening
- **Manifest URLs must be HTTPS.** `CheckLatest` now rejects configured non-HTTPS or hostless manifest URLs before issuing a network request, matching the package threat model that the signed release manifest is fetched over HTTPS. Tests cover the fail-closed status path.
- **Manifest bodies are bounded explicitly.** `readManifestBody` reads at most `maxManifestBytes + 1` and returns a clear "manifest too large" error if the server exceeds 64 KiB, rather than passing a silently truncated body into signature verification. Keep this limit check before `VerifyManifest`.

### 2026-05-26 — Existing directory permission repair
- **`MkdirAll(path, 0700)` is not enough for privacy.** It applies the mode only when the directory is newly created; existing `0755` PMForge roots or per-user folders stayed too broad. `users.ensurePrivateDir` now runs `MkdirAll` and then `Chmod(0700)` for the system root plus each account's `projects`, `certs`, and `exports` directories.
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
- **Do not use `crypto/rand.Read` in recoverable code paths on Go 1.26.** In this toolchain it fatals the process if the reader fails. PMForge's DB and Agile ID generators now use `io.ReadFull(rand.Reader, ...)` and return contextual errors instead of crashing or emitting zero IDs.
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
- **Sigma report exports must repair existing export directory modes.** `GenerateSigmaReport` writes PDFs as `0600`, but `getExportDir` previously left a pre-existing broad `$HOME/PMForge/exports` directory untouched. It now chmods the directory back to `0700` after `MkdirAll`.
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
- **Representative PDF/A samples should use public export APIs.** `scripts/validate-pdfa.sh` now generates a schedule report through `export.GenerateArchivalReport`, a standalone charter through `documents.Render`, and a combined report through `documents.BuildCombinedReport`, all with Source Sans 3 registered where needed.

### 2026-06-06 — V2 encryption-at-rest stopgap
- **Do not imply PMForge encrypts `.pmforge` databases at rest in V2.** README now states the supported V2 protection path: private per-user data directories plus OS-level disk encryption with FileVault, BitLocker, or LUKS.
- **Guard release security claims with a cheap textual gate.** `scripts/release-gate-scope-check.sh` now fails if README stops mentioning the OS-level disk-encryption path or the SQLCipher/V3 deferral. This keeps the release docs from drifting into an unsupported native-encryption claim.

---

## 10. Quick map: "where do I add ..."

| Task                                      | File(s) to touch                                                          |
| ----------------------------------------- | ------------------------------------------------------------------------- |
| New chart kind                            | `internal/charts/registry.go` (Definition entry); pick or add engine pkg; engines.go switch; new Svelte editor; App.svelte route; Dashboard card. |
| New document kind                         | `internal/documents/registry.go` (Kind const + Definition in templates.go). Frontend create path is automatic: Dashboard fetches `ListDocumentKinds()` and renders a button per kind; the `documents` route in `App.svelte` already points to `CharterEditor` which handles any kind generically. |
| New document bespoke PDF renderer         | `internal/documents/<kind>.go` with `Render<Kind>PDF()`; switch in `documents.Render()`. |
| New database column                       | `internal/db/sqlite.go` Migrate() — additive only.                        |
| New CLI flag                              | `internal/cli/parser.go` Config struct + flag.*Var; handle in main.go.    |
| New Wails-exposed App method              | Add to `*App` in `cmd/pmforge/main.go`; declare in `frontend/src/wails-window.d.ts`. |
| New shared editor pattern                 | `frontend/src/lib/components/charts/_*_shell.svelte` (snippet-based).     |
| Change SPDX license for a directory       | Update each file's header; add the SPDX ID to `LICENSES.md`.              |

---

**End of handbook.** Keep this file lean — link to source rather than duplicate it. Source is the ground truth; this file is the map.
