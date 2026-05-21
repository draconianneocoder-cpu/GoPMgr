<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge

PMForge is a local-first project controls desktop application for
technical, engineering, IT, construction, and administrative
organizations. The Go backend acts as a high-performance kernel for
data integrity, scheduling math (CPM, EVM, MSPDI interchange), local
authentication, and document rendering. The Svelte 5 frontend
(mounted via Wails v2) provides the reactive UI.

This is the **V2 foundation**. V2 expands V1 with local multi-user
accounts, an extended data model covering all 19 chart types and 25
document types, and end-to-end implementations of the WBS chart and
Project Charter document as reference slices.

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

# 5. Full release gate (versions + REUSE + build)
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

The `kind` column is the discriminator. The 19 chart kinds map to
five engines (DAG, stats, matrix, flow, special); the 25 document
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
   PDF, no PNG screenshots and no headless browser. All 19 chart
   kinds are supported across the five engines (DAG, Fishbone, Flow,
   Matrix, Stats).

Implementation: `internal/documents/report.go` (`BuildCombinedReport`),
exposed as `App.ExportCombinedReport(reportTitle, subtitle, sections)`
on the Wails surface. main.go pre-resolves every referenced chart
from the database in one pass and threads the map through
`ReportSpec.ResolvedCharts` so this package stays database-free.

## Coverage table

### Chart types — 19 total, all in backend taxonomy

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
in `internal/documents/templates.go` and a default empty content
generator (`DefaultContent`). The generic renderer (`Render`)
produces a presentable PDF for every kind out of the box; bespoke
layouts live in dedicated files (currently only Charter has one).

| Phase       | Kind                              | Bespoke renderer? |
| ----------- | --------------------------------- | ----------------- |
| Initiation  | Project Charter (Word)            | **Yes** (charter.go) |
| Initiation  | Project Charter (Excel)           | Reuses Word charter layout |
| Initiation  | Business Case                     | Generic           |
| Initiation  | Project Proposal                  | Generic           |
| Initiation  | Stakeholder Analysis Document     | Generic           |
| Planning    | Project Plan (Word)               | Generic           |
| Planning    | Project Plan (Excel)              | Generic           |
| Planning    | Project Schedule                  | Generic           |
| Planning    | Work Breakdown Structure          | Generic           |
| Planning    | RACI Chart Document               | Generic           |
| Planning    | Risk Register                     | Generic           |
| Planning    | Scope Statement                   | Generic           |
| Planning    | Project Budget                    | Generic           |
| Planning    | Communication Plan                | Generic           |
| Planning    | Project Execution Plan            | Generic           |
| Planning    | Statement of Work                 | Generic           |
| Planning    | Procurement Plan                  | Generic           |
| Planning    | Requirements Document             | Generic           |
| Planning    | Team Charter                      | Generic           |
| Execution   | Project Brief                     | Generic           |
| Execution   | Project Overview                  | Generic           |
| Monitoring  | Status Report                     | Generic           |
| Monitoring  | Issue Log                         | Generic           |
| Monitoring  | Change Request Form               | Generic           |
| Closing     | Project Closure                   | Generic           |

The generic renderer reads the kind's `Field` definitions and emits
sections, bulleted lists, and tables automatically. Upgrading any of
these to bespoke is one new function in `internal/documents/<kind>.go`
plus one switch case in `documents.Render`.

---

## Directory layout

```
pmforge/
├── LICENSES/                  # REUSE-compliant license texts
├── cmd/pmforge/main.go        # CLI dispatch + Wails bootstrap
├── internal/
│   ├── admin/workflow.go      # Administrative Pack (V1)
│   ├── agile/doc.go           # Software Dev Pack placeholder (V1)
│   ├── auth/password.go       # Argon2id PHC hash/verify           (V2)
│   ├── users/store.go         # system.db + per-user folders       (V2)
│   ├── charts/
│   │   ├── registry.go        # 19-kind taxonomy + 5 engines       (V2)
│   │   ├── engines.go         # Layout() dispatcher                (V2)
│   │   └── dag/wbs.go         # Full WBS data model + layout       (V2)
│   ├── cli/parser.go          # GNU-style flag parser (V1)
│   ├── crypto/
│   │   ├── encrypt.go         # AES-256-GCM + Argon2id KDF
│   │   └── pdf_sign.go        # X.509/RSA signing
│   ├── db/
│   │   ├── sqlite.go          # WAL SQLite + V1+V2 migrations
│   │   ├── settings.go
│   │   ├── audit.go
│   │   ├── repair.go
│   │   ├── backup.go
│   │   ├── ids.go             # short random IDs                   (V2)
│   │   ├── project.go         # Project CRUD                       (V2)
│   │   ├── charts.go          # Chart CRUD                         (V2)
│   │   └── documents.go       # Document CRUD                      (V2)
│   ├── debug/report.go        # ErrorReport + .ToError()
│   ├── documents/
│   │   ├── registry.go        # 25-kind taxonomy + Field types     (V2)
│   │   ├── templates.go       # All 25 default schemas             (V2)
│   │   ├── defaults.go        # DefaultContent + EffectiveFields   (V2)
│   │   └── charter.go         # Validate + RenderCharterPDF + Render(generic) (V2)
│   ├── export/                # PDF/XLSX/CSV/MSPDI renderers (V1)
│   ├── kernel/scheduler.go    # CPM forward+backward+critical (V1)
│   └── update/check.go        # update stub
├── frontend/
│   └── src/
│       ├── App.svelte         # routing                            (V2)
│       ├── main.ts
│       ├── app.css
│       ├── wails-window.d.ts  # full V2 API surface                (V2)
│       └── lib/
│           ├── session.svelte.ts                                   (V2)
│           └── components/
│               ├── GanttChart.svelte
│               ├── Settings.svelte
│               ├── admin/SignatureSettings.svelte
│               ├── auth/Login.svelte                               (V2)
│               ├── auth/CreateAccount.svelte                       (V2)
│               ├── project/ProjectPicker.svelte                    (V2)
│               ├── project/Dashboard.svelte                        (V2)
│               ├── charts/WBSEditor.svelte                         (V2)
│               ├── documents/CharterEditor.svelte                  (V2)
│               └── documents/DocumentFieldEditor.svelte            (V2)
├── scripts/check-release.sh
├── Makefile
├── go.mod
└── wails.json
```

---

## How to wire up the next chart or document type

The V2 foundation is deliberately shaped so each remaining chart/doc
type is a small, self-contained change. Use the WBS and Charter as
references.

### Adding a new chart renderer (e.g. RACI Matrix)

1. The kind already exists in `internal/charts/registry.go`.
2. Create `internal/charts/matrix/raci.go` with:
   - A struct for the data shape (`{roles, tasks, assignments}`).
   - A `Layout(doc, opt) Layout` function returning `NodeLayout`s.
3. Add a case in `internal/charts/engines.go`:
   ```go
   case KindRACI:
       doc, _ := matrix.Parse(rawData)
       layout := matrix.LayoutRACI(doc)
       body, _ := json.Marshal(layout)
       return LayoutResult{Engine: def.Engine, Kind: kind, Title: def.Name, Body: body}, nil
   ```
4. Create `frontend/src/lib/components/charts/RACIEditor.svelte`
   modelled on `WBSEditor.svelte`.
5. Add a routing branch in `App.svelte`.

### Adding a bespoke document renderer (e.g. Risk Register)

1. The kind already exists in `internal/documents/registry.go` and
   has a full schema in `templates.go`.
2. Create `internal/documents/risk_register.go` with a
   `RenderRiskRegisterPDF(content map[string]interface{}, projectName string) ([]byte, error)`
   function modelled on `RenderCharterPDF`.
3. Add a case in `documents.Render()`:
   ```go
   case KindRiskRegister:
       return RenderRiskRegisterPDF(content, projectName)
   ```
4. (Optional) Create a bespoke editor at
   `frontend/src/lib/components/documents/RiskRegisterEditor.svelte`
   if the generic `DocumentFieldEditor` form isn't expressive enough.

Until step 2 lands, the generic renderer + the generic editor still
let the user view, edit, save, version, and PDF-export the document
end-to-end.

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

1. **DOCX / ODT export** — `internal/export/engine.go` returns
   `EXPORT_FORMAT_UNIMPLEMENTED`.
2. **PDF/A-3 conformance.**
3. **CMS/PKCS#7 signature embedding.**
4. **Wails file-picker for certs** in `SignatureSettings.svelte`.
5. **Update channel** in `internal/update/check.go`.
6. ~~Agile Pack.~~ **Done.** `internal/agile/` provides the Kanban
   board, Backlog, Sprint management, and DORA metrics with elite/
   high/medium/low classification. Frontend components live in
   `frontend/src/lib/components/agile/` and are reachable from the
   Dashboard's "Software-Dev Pack" section (toggle to enable).
7. ~~Database swap after self-heal in `internal/db/repair.go`.~~
   **Done.** `SwapInSnapshot` atomically renames the .bak into place;
   exposed as `App.RepairAndSwap` in the Wails surface.

### New in V2

8. **Per-user database encryption at rest.** Today the per-user
   folder relies on `chmod 0700`. Encrypting each `.pmforge` with a
   key derived from the user's password would also defeat raw-disk
   reads and is the strongest possible local-multi-user isolation.
9. **All 19 chart kinds are now implemented.** DAG (WBS, Network,
   PERT, CPM, Fishbone, Cause-and-Effect), Flow (Workflow, Activity),
   Matrix (RACI, SWOT, Stakeholder, Generic), and Stats (Line, Bar,
   Pareto, Pie, BurnUp, BurnDown, CumulativeFlow, Control) all have
   end-to-end backend layouts + frontend editors. Stats charts use
   Chart.js via the shared `StatsChart.svelte` host.
10. **24 bespoke document renderers.** The generic renderer covers
    all 25 kinds today; upgrading each one is similarly self-contained.
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
13. **Account recovery.** No "forgot password" flow today. A
    recovery-code option (mentioned at account creation) is the
    cleanest fit for a local-first app.

---

## License

Source code: **GPL-3.0-or-later**. Documentation: **GFDL-1.3-or-later**.
This README and small configuration files are released under
**CC0-1.0**. See `LICENSES/README.md`.

External libraries adopted in V2.x:

- [`gorules/zen-go`](https://github.com/gorules/zen-go) (Apache-2.0)
  — drives the Project Launchpad's seeding rules as JDM
  (JSON Decision Model) data. Adding industries/methodologies is a
  one-row edit in `internal/templates/launchpad_seeds.json`.
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

**Adding your own font.** In the app, import any TrueType (`.ttf`) file
via the font picker (backed by `App.ImportFont`). The file is validated
(OpenType/CFF `.otf`, WOFF, and collections are rejected — the PDF
engine embeds TrueType outlines only) and copied into your per-user
`fonts/` directory, after which it appears in the font list. Set the
document-export font with `App.SetDefaultFont`; the choice persists in
the project's `settings.default_font`.

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

Strict PDF/A-3 conformance still requires (a) shipping a TTF and
switching gofpdf to UTF-8-embed mode, (b) an OutputIntent + ICC
profile, and (c) running every release through veraPDF. The Catalog
metadata-stream injection — formerly the blocking unknown — is done.
Remaining items tracked as a V3 milestone in AGENT.md §8.

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
Use it to edit the project's name, description, owner, industry,
sub-category, methodology, country code, lifecycle status / phase,
start / end dates, and budget — every field the Launchpad asked
about is editable later. The classification fields (industry +
methodology + country) feed live into the Launchpad-seeding rules,
the terminology resolver, and the calendar holidays the Timeline
overlays. Budget feeds the Dashboard's Budget panel.

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
