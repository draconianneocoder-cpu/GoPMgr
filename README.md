<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge

PMForge is a local-first project controls desktop application for
technical, engineering, IT, construction, and administrative
organizations. The Go backend acts as a high-performance kernel for
data integrity, scheduling math (CPM, EVM, MSPDI interchange), local
authentication, and document rendering. The Svelte 5 frontend
(mounted via Wails v2) provides the reactive UI.

The app has reached **V2.x** maturity: all 20 chart kinds and all 25
document kinds are implemented end-to-end with bespoke PDF renderers,
DOCX/ODT export, and a combined report builder with embedded vector
chart visualisations. The Agile/Software-Dev Pack (Kanban, Backlog,
Sprints, DORA metrics) and the Process Excellence (Sigma) Pack are
complete. Local multi-user accounts with Argon2id authentication and
account recovery codes are in place.

Every file carries an SPDX header and is licensed GPL-3.0-or-later.

---

## Quick start

```sh
# 1. Dependencies
go mod tidy
(cd frontend && npm install)
make fonts          # download the bundled TrueType fonts (see "Fonts" below)

# 2. (optional) Drop the GPL/GFDL/CC0 license texts in place
pip install reuse && reuse download --all

# 3. Develop with hot-reload
wails dev

# 4. Build a production binary (embeds the built frontend via go:embed)
make build

# 5. Run the unit test suite
make test              # go test ./cmd/... ./internal/... (28 packages, ~2s)
make race              # same with -race (concurrency hardening)
make check-pades       # generate + locally verify an embedded signed PDF sample
make check-pades-external # verify extracted CMS/ByteRange with available external tools

# 6. Full release gate (versions + REUSE + build + frontend checks)
make check-release
```

> **Toolchain.** `go.mod` pins **Go 1.26.3** and **Wails v2.9.2** as
> specified in the design transcript. If your Go is older, edit the
> `go` line in `go.mod` and rerun `go mod tidy`. CGO is required
> because `mattn/go-sqlite3` is a C library.

> **First-run layout.** On first launch, PMForge creates
> `~/Documents/PMForge/system.db` (account list) and provisions
> `~/Documents/PMForge/<username>/{projects,certs,exports}/` for each
> user. POSIX permissions on the per-user folder are set to `0700` so
> other OS accounts cannot read PMForge data without elevation.

---

## V2 architecture

### Local multi-user authentication

- Argon2id password hashes in PHC string format
  (`$argon2id$v=19$m=65536,t=3,p=4$...$...`) stored in `system.db`.
- Per-user folder isolation under `~/Documents/PMForge/<username>/`.
- Transparent re-hashing on login if parameters have been strengthened
  since the account was created.
- Generic "invalid credentials" error path that does not distinguish
  unknown-user from wrong-password (timing-safe and message-safe).

### Unified data model

Two tables back every chart and document in the system:

```sql
charts    (id, project_id, kind, title, data JSON, config JSON, ...)
documents (id, project_id, kind, title, content JSON, version, status, ...)
```

The `kind` column is the discriminator. The 20 chart kinds map to
four engines (DAG, stats, matrix, flow); the 25 document
kinds map to one generic field-based editor + per-kind PDF renderers.
This avoids 44 separate code paths and lets every new kind be added
with one registry entry plus, optionally, a bespoke renderer.

### Project lifecycle

The `project` table tracks PMI process groups:

```
phase  := initiation | planning | execution | monitoring | closing
status := planning | active | on_hold | complete | cancelled
```

The Dashboard groups document templates by phase so the user can see
what they typically create at each stage of the project.

---

## Combined reports

PMForge can bundle multiple documents into a single PDF — useful for
the "Project Plan" pattern where a stakeholder-facing deliverable
aggregates Charter + Scope + Budget + Schedule + RACI + Status into
one file.

From the Dashboard, click **New document → Combined Report**. The
composer presents two columns: available project documents on the
left, the in-report section list on the right. Add documents, drag
them into order, optionally type a one-line intro per section, then
click **Export PDF**. The output lands in
`~/Documents/PMForge/<username>/exports/`.

The PDF contains:

1. A cover page with the report title, subtitle, author (your display
   name), project name, and a nanosecond-precision generation
   timestamp.
2. An auto-generated table of contents.
3. One section per included document, rendered with sub-headings that
   match each kind's schema.
4. **Embedded chart visualisations.** Every chart_ref field in an
   included document (a Project Plan's WBS / Schedule / RACI / etc.)
   is followed by a dedicated page rendering that chart via
   `internal/charts/pdfrender` — vector primitives straight to the
   PDF, no PNG screenshots and no headless browser. All 20 chart
   kinds are supported across the four engines (DAG, Flow, Matrix, Stats).

Implementation: `internal/documents/report.go` (`BuildCombinedReport`),
exposed as `App.ExportCombinedReport(reportTitle, subtitle, sections)`
on the Wails surface. main.go pre-resolves every referenced chart
from the database in one pass and threads the map through
`ReportSpec.ResolvedCharts` so this package stays database-free.

## Coverage table

### Chart types — 20 total, all in backend taxonomy

| Family    | Kind                       | Status                                              |
| --------- | -------------------------- | --------------------------------------------------- |
| DAG       | Work Breakdown Structure   | **Full** — visual tree editor + hierarchical layout |
| DAG       | Network Diagram            | **Full** — layered activity-on-node diagram         |
| DAG       | PERT Chart                 | **Full** — O/M/P → E + σ², layered                  |
| DAG       | CPM Chart                  | **Full** — reuses `internal/kernel` for ES/EF/LS/LF |
| DAG       | Fishbone Diagram           | **Full** — radial Ishikawa with 6 Ms preset         |
| DAG       | Cause-and-Effect Diagram   | **Full** — generic causal tree for 5-Whys analyses  |
| Stats     | Line Chart                 | **Full** — Chart.js host, multiple series, dashed   |
| Stats     | Bar Chart                  | **Full** — categorical, multi-series                |
| Stats     | Pareto Chart               | **Full** — sort+cum%, 80% reference, dual y-axis    |
| Stats     | Pie Chart                  | **Full** — auto percentages, tooltip slice details  |
| Stats     | Burn-Up Chart              | **Full** — completed vs scope, dashed scope line    |
| Stats     | Burn-Down Chart            | **Full** — actual + ideal trajectory                |
| Stats     | Cumulative Flow Diagram    | **Full** — stacked areas, reorderable states        |
| Stats     | Control Chart              | **Full** — UCL/LCL annotations, outlier highlights  |
| Matrix    | RACI Matrix                | **Full** — grid + R/A/C/I validation (one A per task) |
| Matrix    | SWOT Matrix                | **Full** — 2×2 quadrants with colour-coded panes    |
| Matrix    | Stakeholder Analysis Matrix| **Full** — Power × Interest plot, 4 strategies      |
| Matrix    | Matrix Diagram             | **Full** — editable m×n grid for traceability       |
| Flow      | Workflow Diagram           | **Full** — 6 node shapes, top-down rank layout      |
| Flow      | Activity Diagram           | **Full** — UML swimlanes, fork/join, decision       |

### Document types — 25 total, all in backend taxonomy

Organised here by lifecycle phase. Every entry has a complete schema
in `internal/documents/templates.go`, a default content generator
(`DefaultContent`), and a bespoke PDF renderer in a dedicated file.
The two Excel aliases share their Word counterpart's layout at
dispatch time; all 25 kinds effectively have dedicated rendering.

| Phase       | Kind                              | Renderer file                  |
| ----------- | --------------------------------- | ------------------------------ |
| Initiation  | Project Charter (Word)            | `charter.go`                   |
| Initiation  | Project Charter (Excel)           | Alias → charter_word           |
| Initiation  | Business Case                     | `business_case.go`             |
| Initiation  | Project Proposal                  | `project_proposal.go`          |
| Initiation  | Stakeholder Analysis Document     | `stakeholder_analysis.go`      |
| Planning    | Project Plan (Word)               | `project_plan.go`              |
| Planning    | Project Plan (Excel)              | Alias → plan_word              |
| Planning    | Project Schedule                  | `project_schedule.go`          |
| Planning    | Work Breakdown Structure Document | `wbs_document.go`              |
| Planning    | RACI Chart Document               | `raci_document.go`             |
| Planning    | Risk Register                     | `risk_register.go`             |
| Planning    | Scope Statement                   | `scope_statement.go`           |
| Planning    | Project Budget                    | `project_budget.go`            |
| Planning    | Communication Plan                | `communication_plan.go`        |
| Planning    | Project Execution Plan            | `execution_plan.go`            |
| Planning    | Statement of Work                 | `statement_of_work.go`         |
| Planning    | Procurement Plan                  | `procurement_plan.go`          |
| Planning    | Requirements Document             | `requirements.go`              |
| Planning    | Team Charter                      | `team_charter.go`              |
| Execution   | Project Brief                     | `project_brief.go`             |
| Execution   | Project Overview                  | `project_overview.go`          |
| Monitoring  | Status Report                     | `status_report.go`             |
| Monitoring  | Issue Log                         | `issue_log.go`                 |
| Monitoring  | Change Request Form               | `change_request.go`            |
| Closing     | Project Closure                   | `closure.go`                   |

A generic field-walker fallback still exists in the dispatch for
forward-compatibility if a new kind is registered before its bespoke
renderer ships. Adding a new kind = one registry entry in
`templates.go` + one bespoke file + one switch arm in
`documents.Render()`.

---

## Directory layout

```
pmforge/
├── AGENT.md                     # AI development handbook (read first)
├── LICENSES/                    # REUSE-compliant license texts
├── cmd/pmforge/main.go          # CLI dispatch + Wails bootstrap (App struct)
├── internal/
│   ├── admin/workflow.go        # Administrative Pack
│   ├── agile/                   # Software-Dev Pack (complete)
│   │   ├── agile.go             # types: WorkItem, Column, Board, Sprint
│   │   ├── store.go             # CRUD for all 5 agile tables
│   │   └── dora.go              # DORA metric computation
│   ├── auth/password.go         # Argon2id PHC hash/verify
│   ├── budget/                  # Budget rollup (stakeholder rates × work items)
│   ├── calendar/calendar.go     # Country holiday datasets (rickar/cal/v2 wrapper)
│   ├── charts/
│   │   ├── registry.go          # 20-kind taxonomy + 4 engines
│   │   ├── engines.go           # Layout() dispatcher
│   │   ├── dag/                 # WBS, Network, PERT, CPM, Fishbone, Cause-Effect
│   │   ├── flow/                # Workflow, Activity
│   │   ├── matrix/              # RACI, SWOT, Stakeholder, Generic
│   │   ├── stats/               # Line, Bar, Pareto, Pie, BurnUp, BurnDown, CumFlow, Control
│   │   └── pdfrender/           # Vector chart renderers for PDF embed
│   ├── cli/parser.go            # GNU-style flag parser
│   ├── crypto/
│   │   ├── encrypt.go           # AES-256-GCM + Argon2id KDF
│   │   └── pdf_sign.go          # X.509/RSA + PAdES B-B embedding
│   ├── db/
│   │   ├── sqlite.go            # WAL SQLite + all migrations
│   │   ├── settings.go          # UserSettings singleton
│   │   ├── project.go           # Project CRUD
│   │   ├── charts.go / documents.go / stakeholders.go
│   │   ├── audit.go / repair.go / backup.go / ids.go
│   ├── debug/report.go          # ErrorReport + .ToError()
│   ├── documents/               # 25 bespoke PDF renderers + registry
│   │   ├── registry.go / templates.go / defaults.go
│   │   ├── charter.go           # also hosts generic Render() dispatcher
│   │   └── <kind>.go            # one file per bespoke renderer (23 files)
│   ├── export/                  # DOCX (godocx), ODT (hand-built), XLSX, iCal, PDF/A
│   ├── fonts/                   # bundled TTF catalog + user import
│   ├── kernel/scheduler.go      # CPM forward+backward pass
│   ├── pdfmeta/pdfmeta.go       # XMP metadata + PAdES signature injection (dep-free)
│   ├── sigma/                   # Process Excellence (Six Sigma) Pack
│   ├── templates/               # Launchpad seeding rules (zen-go JDM)
│   ├── timeline/                # Timeline assembly + iCal export
│   ├── update/check.go          # Signed Ed25519 update-manifest checker
│   └── users/store.go           # system.db + per-user folder provisioning
├── frontend/
│   └── src/
│       ├── App.svelte           # lazy route loader (charter, documents, all charts, …)
│       ├── wails-window.d.ts    # TypeScript declarations for all App methods
│       └── lib/
│           ├── session.svelte.ts
│           └── components/
│               ├── auth/        # Login, CreateAccount, RecoveryReset
│               ├── project/     # ProjectPicker, Launchpad, Dashboard, Settings,
│               │                # StakeholderManager, TimelineView
│               ├── charts/      # 20 editor components + shared shells
│               ├── documents/   # CharterEditor (generic), DocumentFieldEditor,
│               │                # ChartPicker, ReportComposer
│               ├── agile/       # KanbanBoard, Backlog, SprintList, DORADashboard
│               └── sigma/       # SigmaWorkspace + per-tool views
├── scripts/
│   ├── check-release.sh / memory-safety-scan.sh / fetch-fonts.sh
│   ├── frontend-stability-check.sh / frontend-build-budget.sh
│   └── validate-pdfa.sh / validate-pades.sh
├── Makefile
├── go.mod / wails.json
```

---

## How to add a new chart or document kind

All 20 chart kinds and all 25 document kinds are fully implemented.
The taxonomy is designed for extension: adding a new kind is a small,
self-contained change.

### Adding a new chart kind

1. Add a `Definition` entry to `internal/charts/registry.go`.
2. Create or extend the engine package (`dag/`, `flow/`, `matrix/`,
   `stats/`) with a data struct + `Layout()` function.
3. Add a dispatch arm in `internal/charts/engines.go`.
4. Add a vector PDF renderer in `internal/charts/pdfrender/`.
5. Create `frontend/src/lib/components/charts/<Kind>Editor.svelte`.
6. Add a route entry in `App.svelte` and a dashboard card in
   `Dashboard.svelte`.

### Adding a new document kind

1. Add a `Kind` constant and a `Definition` (with full `Fields` slice)
   to `internal/documents/registry.go` and `templates.go`.
2. Create `internal/documents/<kind>.go` with a bespoke PDF renderer.
3. Add a dispatch arm in `documents.Render()`.
4. No frontend changes needed: `Dashboard.svelte` fetches
   `ListDocumentKinds()` and renders a create button automatically;
   the `documents` route in `App.svelte` already handles any kind
   through the generic `CharterEditor` (field-based) component.

---

## V1 → V2 schema migration

Because every V2 table is created with `CREATE TABLE IF NOT EXISTS`
and every column has a default, opening a V1 `.pmforge` file with the
V2 binary triggers an additive migration: the four new tables are
created, V1 tables are untouched. No downgrade is provided — opening
a V2 file with V1 will see the old four tables and ignore the new ones.

---

## Real TODOs in the V2 scaffold

These remain from V1 plus a handful of new V2 items.

### From V1

1. ~~DOCX / ODT export.~~ **Done.** DOCX uses
   `gomutex/godocx`; ODT is generated directly.
2. **PDF/A-3 conformance.** XMP Catalog metadata, embedded fonts,
   OutputIntent/ICC injection, PDF trailer IDs, and binary header
   comments are implemented. The schedule-report, document, and
   combined-report samples now pass the veraPDF PDF/A-3b gate locally;
   remaining work is release-builder soak before this becomes a hard
   release claim.
3. ~~CMS/PKCS#7 + PAdES signature embedding.~~ **Done.** Signed
   exports use CMS/PKCS#7 plus an embedded PDF signature dictionary,
   invisible widget field, `/ByteRange`, and padded `/Contents`.
   `make check-pades` generates a local signed sample and verifies the
   embedded CMS against the declared `/ByteRange`; `make
   check-pades-external` also verifies the sample with OpenSSL,
   `qpdf`, `pdfsig`, veraPDF signature feature extraction, and
   `dss-validation-tool` when those tools are installed. Current DSS
   coverage classifies the deterministic self-signed sample as
   `PAdES-BASELINE-B`; release-certificate trust-chain validation
   remains indeterminate until a trusted signing source is configured.
   Remaining external validation is Acrobat coverage for sample signed
   PDFs.
4. ~~Wails file-picker for certs.~~ **Done.**
5. ~~Update channel.~~ **Done.** Signed Ed25519 manifests are fetched
   and verified by `internal/update`.
6. ~~Agile Pack.~~ **Done.** `internal/agile/` provides the Kanban
   board, Backlog, Sprint management, and DORA metrics with elite/
   high/medium/low classification. Frontend components live in
   `frontend/src/lib/components/agile/` and are reachable from the
   Dashboard's "Software-Dev Pack" section (toggle to enable).
7. ~~Database swap after self-heal in `internal/db/repair.go`.~~
   **Done.** `SwapInSnapshot` atomically renames the .bak into place;
   exposed as `App.RepairAndSwap` in the Wails surface.

### New in V2

8. **Per-user at-rest protection.** V2 protects local project data
   with per-user data directories and private filesystem permissions.
   For raw-disk theft or admin-level host access, the supported V2
   path is OS-level disk encryption: FileVault on macOS, BitLocker on
   Windows, and LUKS on Linux. SQLCipher native database encryption is
   deferred to V3 because it adds native packaging complexity and must
   be designed with crash recovery and migration semantics.
9. **All 20 chart kinds are now implemented.** DAG (WBS, Network,
   PERT, CPM, Fishbone, Cause-and-Effect), Flow (Workflow, Activity),
   Matrix (RACI, SWOT, Stakeholder, Generic), and Stats (Line, Bar,
   Pareto, Pie, BurnUp, BurnDown, CumulativeFlow, Control) all have
   end-to-end backend layouts + frontend editors. Stats charts use
   Chart.js via the shared `StatsChart.svelte` host.
10. ~~Bespoke document renderers.~~ **Done.** All 25 document kinds
    now route to dedicated layouts, with the Word/Excel alias kinds
    sharing their canonical renderer.
11. ~~Chart picker in the field editor.~~ **Done.** `chart_ref`
    fields render via `ChartPicker.svelte`; document templates
    declare `ChartKind` so the picker filters to the appropriate
    family (e.g. wbs_ref → WBS charts only).
12. ~~Combined report builder.~~ **Done.** `App.ExportCombinedReport`
    assembles multiple documents into one PDF with a cover page,
    auto-generated table of contents, and **embedded chart
    visualisations** for every chart_ref field. Each referenced
    chart renders on its own page using the `pdfrender` engine
    (vector graphics, no PNG screenshots).
13. ~~Account recovery.~~ **Done.** Eight Argon2id-hashed one-time
    recovery codes are issued at account creation. The "Forgot
    password?" link on the login screen opens `RecoveryReset.svelte`.
    Backend: `App.IssueRecoveryCodes`, `App.ResetWithRecoveryCode`.

---

## License

Source code: **GPL-3.0-or-later**. Documentation: **GFDL-1.3-or-later**.
This README and small configuration files are released under
**CC0-1.0**. See `LICENSES.md`.

External libraries adopted in V2.x:

- [`github.com/gorules/zen`](https://github.com/gorules/zen) (MIT) via its
  Go binding (`zen-go`) — drives the Project Launchpad's seeding rules
  as JDM (JSON Decision Model) data. Adding industries/methodologies is
  a one-row edit in `internal/templates/launchpad_seeds.json`.
- [`rickar/cal/v2`](https://github.com/rickar/cal) (BSD-2-Clause) —
  maintained holiday datasets for ~40 countries. Used by
  `internal/calendar` (Timeline view holiday markers) and
  `internal/export/ical.go` (iCal export with optional holiday VEVENTs).
- [`digitorus/pkcs7`](https://github.com/digitorus/pkcs7) (FreeBSD
  2-clause) — wraps raw RSA signatures into the CMS/PKCS#7
  SignedData structure that PAdES validators look for. Used by
  `internal/crypto/pdf_sign.go`.
- [`gomutex/godocx`](https://github.com/gomutex/godocx) (MIT, pure
  Go) — DOCX writer used by `internal/export/docx.go`. Picked from
  pkg.go.dev after a survey; ODT export is hand-built because no
  equivalently-maintained pure-Go ODT generator exists.

All four licenses are GPL-3.0-compatible.

## Account recovery codes

At account creation, PMForge issues eight one-time **recovery codes**
(16 base32 chars each, dashed for legibility). They're Argon2id-hashed
in the system database; the plaintext is shown to the user exactly
once. Each code can reset the password once and is then permanently
marked used.

Login → "Forgot password? Use a recovery code" → username + code +
new password → done. The flow is built around `App.IssueRecoveryCodes`
and `App.ResetWithRecoveryCode`.

## Document export formats

Every document kind now exports to four formats:

| Format | Wails method            | Library             |
| ------ | ----------------------- | ------------------- |
| PDF    | `ExportDocumentPDF`     | `gofpdf` + custom   |
| DOCX   | `ExportDocumentDOCX`    | `gomutex/godocx`    |
| ODT    | `ExportDocumentODT`     | hand-built XML zip  |
| (XLSX) | (per-kind, where useful)| `xuri/excelize/v2`  |

All three text formats walk the kind's schema and emit headings,
paragraphs, bullet lists, and tables — so any of the 25 document
kinds round-trips through any format without per-kind code.

## Update channel

If the binary is built with `-ldflags` setting
`pmforge/internal/update.ManifestURL` and
`pmforge/internal/update.UpdateChannelPublicKey`, PMForge fetches a
signed JSON manifest at the URL and verifies its Ed25519 signature
against the embedded public key before reporting a newer version.
Builds without those `-ldflags` settings silently skip the check.

Generate the keypair once for your release pipeline; ship only the
public key in the binary. Manifest schema is documented at the top
of `internal/update/manifest.go`.

## Fonts

PMForge embeds TrueType fonts in generated PDFs (the gofpdf core fonts
are not embeddable and not permitted by strict PDF/A). The
`internal/fonts` package ships a curated catalog of professional,
modern, open-source families — all free for commercial AND personal
use and GPL-compatible:

| Family            | Category | License        | Notes                              |
| ----------------- | -------- | -------------- | ---------------------------------- |
| Liberation Sans   | sans     | OFL-1.1        | Arial-metric-compatible (default)  |
| Liberation Serif  | serif    | OFL-1.1        | Times-New-Roman-metric-compatible  |
| Liberation Mono   | mono     | OFL-1.1        | Courier-metric-compatible          |
| DejaVu Sans       | sans     | Bitstream Vera | Widest glyph coverage              |
| Noto Sans         | sans     | OFL-1.1        | Broad international coverage        |
| Source Sans 3     | sans     | OFL-1.1        | Adobe's modern professional sans   |
| JetBrains Mono    | mono     | OFL-1.1        | Modern monospaced                  |

The font binaries are **not committed** to the repository (they are
large and carry their own upstream licenses). Fetch them once:

```sh
make fonts          # or: scripts/fetch-fonts.sh
```

This downloads the `.ttf` files into `internal/fonts/assets/`, where
`go:embed` bundles them into the build. If a family isn't fetched, the
app falls back to the next available family (ultimately gofpdf's core
Helvetica) — it always builds and runs.

**Adding your own font.** `App.ImportFont` opens a native file dialog,
validates the `.ttf` signature (OpenType/CFF, WOFF, and collections are
rejected), and copies the file into your per-user `fonts/` directory.
`App.ListFonts` returns the available families; `App.SetDefaultFont`
persists the choice in `settings.default_font`. The font picker
(family dropdown + Import button) lives in the "Document Font" section
of **Project Settings**.

Implementation: the chosen family is registered under the name
"Helvetica" on each new PDF, so every renderer's existing
`SetFont("Helvetica", ...)` call transparently uses the embedded font
without per-renderer changes.

## PDF/A note

`internal/pdfmeta` builds the canonical XMP packet identifying a PDF
as PMForge-generated PDF/A-3 level B, and **injects it into the PDF
Catalog as a metadata stream** via a spec-conformant incremental
update (`InjectXMPStream`). `documents.Render()` tags every generated
document PDF automatically (fail-soft: a valid but un-tagged PDF is
returned if injection ever fails). `internal/export/pdfa.go` remains
as the thin gofpdf adapter that sets the library's Title / Author /
Subject / Creator / Keywords fields.

Strict PDF/A-3 conformance now depends on release validation rather
than missing renderer primitives: font embedding, Catalog XMP
metadata-stream injection, and OutputIntent + ICC injection are all
implemented. `make check-pdfa` validates schedule-report, document,
and combined-report samples against veraPDF's PDF/A-3b profile. All
three samples pass and the gate is a **hard release blocker** --
`check-release.sh` exits non-zero if any sample regresses. Remaining
V3 hardening (trusted signing chain, Acrobat coverage) is tracked in
AGENT.md §8.

## Project Launchpad

`New Project` no longer drops you into a blank one-field form. The
Launchpad walks you through four steps:

1. **Industry** — Business / Administration / Engineering / Software /
   Construction / Custom.
2. **Sub-category** — industry-aware (e.g. Software → Web / Mobile /
   AI / DevOps / Game).
3. **Methodology** — recommended set for the industry (Scrum, Kanban,
   CPM, Waterfall, Six Sigma, Lean, OKRs, PRINCE2). The user can
   override.
4. **Details & starter artifacts** — name, description, country
   (for holidays), and a checklist of suggested seed artifacts that
   the zen-go decision engine returned for the (industry, methodology)
   combo. Examples:
   - Software + Scrum → Kanban board, Project Charter, Backlog (3
     placeholders), Sprint 1.
   - Construction + Waterfall → WBS, Statement of Work, Risk Register,
     CPM schedule.
   - Engineering + Six Sigma → Control chart, Pareto, Fishbone.

The user can deselect any seed. After creation the seeded artifacts
appear on the Dashboard, ready to edit.

## Project Settings

Every open project shows a "Settings" link in the Dashboard header.
Use it to edit:

- The project's name, description, owner, industry, sub-category,
  methodology, country code, lifecycle status / phase, start / end
  dates, and budget.
- **Export & Signature settings** — export theme (`modern` / `classic`
  / `archival`), auto-repair toggle, signing certificate path, and the
  signature-enabled toggle. Changes are saved as the per-project
  settings singleton.
- **Document Font** — pick from the bundled catalog families or import
  your own `.ttf` file. The chosen family is applied to all PDF exports
  from this project immediately.

The classification fields (industry + methodology + country) feed live
into the Launchpad-seeding rules, the terminology resolver, and the
calendar holidays the Timeline overlays. Budget feeds the Dashboard's
Budget panel.

## Stakeholders & Budget

The Dashboard's top row now exposes:

- **Stakeholders** — project-level address book (team / vendor /
  sponsor / external). Each carries `hourly_rate` and `contract_value`.
- **Timeline** — the same data as the Dashboard's chart panel but
  rendered as a horizontal strip with country-aware holiday markers.
  Export the project to `.ics` (with or without holidays) from the
  same view.
- **Budget panel** — live rollup of `project.budget` vs
  Σ vendor contracts + Σ (work-item points × matched hourly rate).
  Tints red on overspend.

## Dashboard UX

The Dashboard shows two item lists — Charts and Documents. Both support
**inline delete** with a two-step confirm: click the trash icon, then
confirm with a second click. No browser confirm dialog; no page reload.
The item disappears from the list immediately on success.

## Editor shortcuts and document status

All editors (document, chart layered/DAG, chart stats) support
**Ctrl+S / Cmd+S** to save without reaching for the toolbar.

The **CharterEditor** (used for all 25 document kinds) shows:

- An **"Unsaved changes"** amber badge in the header when in-memory
  content differs from the last saved version.
- A **status dropdown** (`draft` → `review` → `approved` → `archived`)
  in the header. Selecting a new status saves the document immediately
  so the status transition is always persisted.

The Software-Dev Pack toggle (Kanban / Backlog / Sprints / DORA) now
**persists across project close and reopen** — it is stored in the
per-project settings database row rather than only in process memory.
