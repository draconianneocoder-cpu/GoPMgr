<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr User Guide

This guide summarizes the main user workflows that were previously mixed
into the root README. The in-app Help Guide remains the most detailed
end-user reference: it is searchable from the sidebar, opens with a
ten-minute "Quick Start: Your First Project" tutorial, and ends with a
Troubleshooting & FAQ and a keyboard-shortcuts reference.

## First Run

GoPMgr stores local data in a private platform-selected application-data
folder. A configured XDG data location overrides the default where supported.
Supported legacy PMForge data is migrated automatically on first launch.
The first launch creates:

- `system.db` for local account metadata.
- A private per-user directory with `projects`, `certs`, and `exports`
  subdirectories.
- A `logs` directory for dated diagnostic logs.

Create the first account, save the one-time recovery codes, then create or
open a project.

For repeatable first-launch testing, quit GoPMgr and run
`make reset-clean-test` from the source checkout. The command moves the entire
data directory to a timestamped sibling backup instead of deleting it. Reopen
GoPMgr to exercise administrator creation against a fresh `system.db`; use
`scripts/reset-clean-test.sh --restore <backup>` after moving aside the
temporary test state. See `docs/INSTALL.md` for the complete guarded reset and
restore procedure.

## Project Launchpad

`New Project` opens the Launchpad instead of a blank one-field form. The
Launchpad asks for:

1. Industry: Business, Administration, Engineering, Software,
   Construction, or Custom.
2. Sub-category, tailored to the industry.
3. Methodology, such as Scrum, Kanban, CPM, Waterfall, Six Sigma, Lean,
   OKRs, PRINCE2, or PMBOK.
4. Project details and starter artifacts.

Each selection is retained on its step until **Continue** is chosen, so it can
be reviewed before advancing. The final step summarizes the project blueprint
and asks for a business-calendar policy plus an IANA time zone. The policy
controls holidays and working days; the time zone controls schedule boundaries
and time-series chart dates. Initial policies cover the United States, Canada,
Western Europe, Japan, and Australia.

The suggested starter artifacts come from the embedded Launchpad rule set.
The user can deselect any suggestion before creating the project.

## Portfolio and Dashboard

After sign-in, GoPMgr opens the Portfolio dashboard. It lists projects
with status, phase, dates, and chart/document counts. Open a project card
to enter its Dashboard.

Choose **Run rollup** to calculate portfolio budget and committed-cost totals
plus Earned Value metrics as of one displayed UTC reporting date. Every
portfolio money total, including Remaining, is calculated in the backend and
shown as an exact decimal value; the desktop renderer does not recompute money.
EVM includes
only projects with a start date and a valid, acyclic, costed current CPM
schedule. The panel shows coverage and warns when projects are excluded rather
than treating missing evidence as zero. Planned Value, Earned Value, Actual
Cost, SPI, and CPI use exact minor-unit totals; portfolio SPI is `ΣEV/ΣPV` and
portfolio CPI is `ΣEV/ΣAC`. They are not averages of individual project ratios.
Committed cost remains the separate contracts-plus-labour estimate used for
budget remaining. A rollup displays its ISO reporting currency and runs only
when every readable project uses that same currency. GoPMgr has no foreign
exchange conversion, so it refuses a mixed-currency portfolio rather than
showing a misleading total.

The Dashboard is the main project workspace:

- Charts and documents are listed as editable project artifacts.
- Both lists support inline delete with a two-step confirmation.
- Methodology-specific sections appear when relevant, including
  Software-Dev and Process Excellence entry points.
- A shared toolbar links to Dashboard, Projects, App Settings, and Help.

## Project Settings

Every open project has a Project Settings view. Use it to edit:

- Project name, description, owner, industry, sub-category, methodology,
  country code, lifecycle status, phase, dates, and budget.
- Export and signature settings.
- What-if scenarios: create, edit, delete, select the active scenario, and
  copy a source chart with current data or a saved schedule baseline into an
  isolated scenario partition. Copied scenario charts can be opened in a
  dedicated scenario editor, edited, compared against their captured baseline
  data, and promoted back to named schedule baselines from the editor.
- Compliance mode, which verifies the tamper-evident audit chain before a
  project opens and blocks the open if the chain has been altered. Project,
  chart, document, schedule-baseline, scenario, scenario-chart copy, document
  approval, scenario-promotion approval, document signature, and signed
  combined-report lifecycle actions are included in the chain. Use **Export
  audit verification report** to write a private JSON verification artifact to
  the user exports folder for compliance review. Use **Export audit repair
  evidence** before manual repair work to preserve the raw audit events and
  verification failure details separately.
- Document font selection and per-project imported fonts.
- Schedule reports and project interchange exports.
- Database encryption migration for eligible plaintext project databases. The
  migration validates a private encrypted copy and preserves an independent,
  schema-migrated plaintext `.pre-encryption.bak` until the user removes it.
  A synchronous migration failure leaves the canonical plaintext project
  readable; protect or remove that backup only after verifying the encrypted
  project opens successfully.

The classification fields feed the Launchpad rules, terminology, and
calendar-aware timeline overlays.

## Application Settings

Application Settings holds user-level preferences that are separate from
per-project settings:

- Application theme.
- Default document font.
- Default export theme for new projects.
- Auto-save on/off and interval.
- Version and data-location information.
- The Beta Center, signed-update status, current release limitations, logs,
  and privacy-safe bug-report generation.

## Stakeholders, Timeline, Budget, and Cost Control

The Dashboard exposes:

- **Stakeholders:** a project address book for team members, vendors,
  sponsors, and external contacts. Stakeholders can carry rates,
  contract values, and availability.
- **Timeline:** a chronological project strip with project dates, sprint
  ranges, milestones, deployments, and country-aware holidays.
- **Budget:** a live rollup of project budget against vendor contracts
  and work-item estimates. The Project Settings value and every Budget-panel
  total cross the desktop boundary as exact two-decimal strings, not JavaScript
  numbers. This preserves values through the full signed 64-bit minor-unit
  range. The progress bar is visual context only; it does not feed a saved
  amount. Currency calculations are kept at cent precision, so fractional
  labour estimates round once at the money boundary.
- **Cost Control:** an independent, project-local ledger for planned,
  commitment, and actual entries. It does not import the Budget panel's
  stakeholder contracts or agile estimates, preventing double counting. New
  entries use exact decimal-string amounts in the project's locked reporting
  currency (USD by default for new and legacy projects). The Cost Control
  amount boundary uses a fixed two-decimal convention. Project Settings can
  select USD, EUR, GBP, CAD, AUD, or CHF only while the project has no
  Budget value, Cost Control entry, non-zero reserve balance, or approved Cost
  Control baseline. It cannot change while any of those values exists because
  Phase 1 has no FX conversion. The dashboard shows
  its separate legacy Budget rollup for context, then the Cost Control base
  plan, contingency reserve, cost baseline, management reserve, authorised
  funding, commitments, and actuals. The legacy Budget rollup does not feed
  Cost Control calculations. Phase 1 does not present an unallocated or
  remaining-spend figure because allocation, reserve drawdowns, and forecast
  policy are not yet defined.
  Contingency and management reserves are separate records and do not count as
  commitments or actual costs. Phase 1 does not yet provide reserve drawdowns,
  foreign exchange, time-phased cash flow, or
  automatic EVM integration. JPY cannot be selected for a new project or a
  currency change. Existing JPY projects remain readable, and their historical
  Cost Control values retain the original fixed two-decimal convention without
  conversion. Cost Control is read-only for those legacy JPY projects so a
  displayed value cannot be silently given new semantics. A future
  exponent-aware JPY delivery requires an explicit storage, migration, and
  interoperability design before GoPMgr claims that support.

Each Cost Control entry inherits three cost-type classifications: direct or
indirect attribution, fixed or variable behavior, and CapEx, OpEx, or not
applicable treatment. The dashboard keeps inactive types visible on historical
ledger rows but excludes them from the entry selector. Its classification table
reconciles every one of the three lenses independently to the same ledger
totals. Do not add values between lenses: the same ledger entry appears once in
each lens.

Use **Cost item or reference** to identify the material or input, an invoice or
supplier reference, or the basis of an overhead entry. It is the existing
required free-text description, not a separate structured record: Phase 1 has
no independently stored quantity, unit, supplier, invoice, SKU, or attachment
field.

Cost Control can record immutable approved baseline snapshots. A snapshot is
derived on the backend from the current planned entries and both reserve
balances; it excludes the legacy Dashboard Budget rollup. Approval records the
signed-in local account and a rationale in the audit chain. Its history shows
the version, local account, UTC approval time, rationale, baseline, and
authorised funding. This is not role authorization or an electronic signature.

Use **Export printable financial report** in Cost Control to choose a PDF
location for one project's financial snapshot. It prints the Legacy Budget
context separately from Cost Control, then includes Cost Control ledger rows,
assessed reserve balances, and immutable baseline history. It does not combine
those sections or calculate forecast, allocation, drawdown, or remaining-funds
figures. The report is a printable export; use your PDF viewer's print command
when a paper copy is needed.
- **Resource assignments:** CPM tasks can carry resource units, optional
  calendar labels, max-unit caps, and skill tags. Project Settings stores named
  resource calendars with weekly capacity and day overrides; leveling uses
  those calendars to delay contended tasks.

## Kanban, Sprints, and DORA

Projects whose methodology enables the agile pack (Scrum, Kanban,
Scrumban) get four extra views:

- **Kanban board:** cards drag between columns and the move persists
  immediately. Column headers show a WIP indicator (count / limit) that
  changes tone when the limit is exceeded.
- **Backlog:** drag items to reorder priority, assign items to a sprint,
  and start work to move an item onto the board.
- **Sprints:** each sprint has a name, goal, start/end dates, and a
  story-point capacity. Sprints move planning -> active -> complete, and
  only one sprint is active at a time — starting a new one completes the
  previous active sprint.
- **DORA dashboard:** Deployment Frequency, Lead Time for Changes, Change
  Failure Rate, and MTTR, computed from deployments recorded with the
  inline "+ Record deployment" form (date, lead-time hours, failure flag,
  restore hours).

## Charts

GoPMgr supports 22 chart kinds across four engine families:

- DAG: WBS, Network, PERT, CPM, Gantt, Fishbone, Cause-and-Effect.
- Flow: Workflow, Activity.
- Matrix: RACI, SWOT, Stakeholder Analysis, Matrix Diagram, Risk Matrix.
- Stats: Line, Bar, Pareto, Pie, Burn-Up, Burn-Down, Cumulative Flow,
  Control.

The Dashboard presents these in one searchable chart-tool catalog. Filter by
engine family when the required visual is known, or search names and use-case
descriptions when it is not. The catalog opens automatically for a project
without charts and stays collapsed by default once the workspace contains
chart work, keeping existing artifacts ahead of creation controls.

Charts are edited in the app and can be embedded into PDF reports as
vector drawings rather than screenshots.

The Risk Matrix keeps risks, active issues, and opportunities on a 5×5
probability/impact grid. Each record can carry an owner, status, response,
and linked task. Optionally select that chart from a Risk Register document
so combined reports include the same vector heat map. After editing register
rows, save and choose **Refresh Risk Matrix** to replace the linked chart's
items. GoPMgr validates every row first and leaves the existing chart
unchanged if IDs or probability/impact values are invalid.

In the schedule editors, dependencies are created with the Connect
action (select the source node, click Connect, click the destination).
CPM and Gantt link labels set the dependency type and lag in days: `FS`,
`SS`, `FF`, or `SF` with optional `+n`/`-n` — for example `SS+2` or
`FS-1`; blank means `FS`. These labels drive the computed schedule. The
Gantt editor can also snapshot a baseline (**Set baseline**); grey ghost
bars then show drift against that snapshot as the plan changes.

CPM charts can generate a Resource Histogram. The generated histogram shows
resource demand as bars and overlays dashed capacity lines from stakeholder
availability and Project Settings Resource Capacity calendars.
CPM and Gantt task over-allocation badges use the same Resource Capacity
calendars when a project has a start date.

CPM charts also include a Monte Carlo risk panel. Add optional optimistic,
most-likely, and pessimistic duration estimates to tasks that carry schedule
uncertainty, choose triangular, beta-PERT, or normal sampling, then run the
simulation from the CPM editor aside. The result shows P50/P80/P90 finish-day
confidence points, a cumulative finish-probability S-curve, and a tornado
driver ranking that combines critical-path frequency with P90-P50 duration
spread. After a simulation runs, use **Export PDF/A** to save a Monte Carlo
risk report with the confidence summary, S-curve, tornado drivers, and narrative
summary. Tasks without estimates use their fixed CPM duration.

## Documents and Combined Reports

GoPMgr supports 25 document kinds across the project lifecycle, including
charters, plans, schedules, budgets, risk registers, requirements, reports,
issue logs, change requests, and closure documents.

The Dashboard's controlled-document index searches template names, purposes,
and kind identifiers, with filters for initiation, planning, execution,
monitoring, and closing. It opens automatically for a project without
documents and stays collapsed once document work exists.

Every document kind can export to PDF, DOCX, and ODT. XLSX is available
where the document kind benefits from spreadsheet output.
For document, combined-report, and schedule exports, GoPMgr opens a save
dialog so you choose a new output location. It refuses to replace an existing
selected file. Combined-report manifests and GnuPG `.asc` sidecars also must
not already exist, avoiding an unconfirmed overwrite of a paired artifact.
Chart, calendar, audit, and Sigma export locations retain their existing
separate workflows in this release.

Combined reports let users assemble several project documents into one
PDF. From the Dashboard, choose **Build combined report**, add
documents, order sections, optionally add section introductions, then
export the report. Chart references inside included documents render as
dedicated vector chart pages.

Before export, select a project-controls, construction, software-delivery, or
custom report profile and run **Preflight**. The built-in profiles provide
practical baselines informed by ISO 21502, ISO 19650, and ISO/IEC/IEEE 12207;
they are guidance, not a compliance certification. Custom selection keeps the
document and chart mix entirely under the user's control. Draft reports retain
findings in a visible quality page, while certified reports block unresolved
errors. Every combined-report PDF has a nearby `.manifest.json` provenance
sidecar containing the selected profile, source versions, timestamps, content
hashes, chart data/configuration for machine-readable parity, and preflight
findings.

Status Reports can link a CPM schedule chart. When that linked schedule has
cost and progress data, combined reports include an Earned Value summary under
the Status Report section. If resource leveling split a task into interrupted
work periods, its planned value remains flat during the intervening idle days
instead of being spread across the whole bar span.

## Schedule Import and Export

From Project Settings, GoPMgr exports the current schedule to:

| Format | Extension | Use |
| --- | --- | --- |
| PDF / DOCX / ODT | `.pdf` / `.docx` / `.odt` | Reports |
| MS Project XML | `.xml` | Interchange |
| CSV | `.csv` | Spreadsheet task lists |
| HTML | `.html` | Browser viewing or publishing |

GoPMgr imports Microsoft Project XML (MSPDI, `.xml`) directly. Binary or
serialized formats such as `.mpp`, `.pod`, and `.mpx` should be resaved as
Microsoft Project XML from the source application before import.

Before selecting a file, choose whether to import dependencies, percent
complete, and resource assignments. GoPMgr always preserves task identities,
names, durations, milestones, and the project start date it supports; its
stored mapping receipt records selected fields, intentional transformations,
and skipped summary or null rows. This makes the MSPDI round-trip boundary
auditable rather than silently lossy.

In Project Settings, schedule and time-series context can use the United
States, Canada, a supported Western European country, Japan, or Australia
holiday policy together with the corresponding selectable IANA time zone. The
selected context is retained with the project and returned with chart
layouts.

## PDF Signing

Project Settings lets users choose a default document signing method:

- **PAdES digital signature** embeds a PDF signature using a `.p12` or
  `.pfx` certificate. Users can configure a certificate in Project Settings
  or choose one directly from the Signature Options dialog during export.
  Project Settings can additionally enable RFC 3161 timestamping for
  PAdES Baseline T. Configure a credential-free HTTPS TSA endpoint, optional
  policy OID, and optional PEM trust root. Without a root, GoPMgr validates
  token integrity but records TSA chain trust as not evaluated.
- **GnuPG detached signature** exports the PDF and writes an ASCII-armored
  `.asc` sidecar. The PDF bytes are not modified after export; verify with
  `gpg --verify document.pdf.asc document.pdf`.
- **No digital signature** exports a plain PDF for print-and-wet-sign or
  external signing workflows.

PAdES signing is applied after rendering and PDF/A metadata injection. This
order is required because the signature covers byte ranges in the final PDF.
When timestamping is enabled, GoPMgr fails the export if the TSA request or
token validation fails; it never silently emits a Baseline B signature.
Endpoint URLs containing credentials, query strings, or fragments are rejected
so project databases do not become a credential store.

## Recovery Codes and Encryption

GoPMgr issues one-time recovery codes at account creation. Recovery codes
can reset an account password once and, for encrypted project databases,
also unlock the user's wrapped DEK.

After migrating an existing plaintext project database to encrypted
storage, recovery codes must be reissued so active codes can unlock the
DEK. If the password and all valid wrapped recovery codes are lost,
encrypted project databases are unrecoverable by design.

## Fonts

GoPMgr embeds TrueType fonts in generated PDFs. Source Sans 3 ships with
every supported build so PDF/A reports do not depend on a network download.
Developers can download the additional optional font catalog with:

```sh
make fonts
```

Users can also import a `.ttf` font from Project Settings. Imported fonts
are stored in the user's GoPMgr data area and can be selected for document
exports.

## Command-Line Maintenance

GoPMgr also runs headless for scriptable maintenance. Two flags are
global:

- `--version` prints the version banner and exits.
- `--update` checks the signed update channel (a no-op when the build has
  no update channel configured).

The maintenance operations take a project file path — `.gopmgr`, or `.pmforge`
for a project created before the August 2026 rename — as the final argument:

- `--check` runs an integrity check and exits.
- `--repair` runs the self-healing repair workflow.
- `--vacuum` compacts the database (`VACUUM`).
- `--export-audit <path>` writes the audit log to CSV at `<path>`.
- `--stats` prints a compact project summary (status, phase, methodology,
  and chart/document/stakeholder/audit counts).
- `--schema-dump` prints the database's SQL schema to stdout.
- `--export <path>` renders the project's schedule report and writes it to
  `<path>`. `--format <fmt>` selects the format (`pdf` default, plus
  `docx`, `odt`, `xlsx`, `csv`, `html`, `mspdi`). Add `--encrypt` to write
  the bytes AES-256-GCM encrypted with the `--password-env` password.

For an **encrypted** project, pass credentials so the database can be
unlocked: `--username <name>` and `--password-env <ENVVAR>`, where the
named environment variable holds that account's password (the password is
never passed on the command line). `--encrypt` also uses `--password-env`
as the export password. Plaintext projects open without credentials. For
example:

```sh
export GOPMGR_PASSWORD
gopmgr --check \
  --username alice --password-env GOPMGR_PASSWORD \
  /path/to/project.gopmgr

# Headless schedule export to encrypted XLSX:
gopmgr --export schedule.xlsx --format xlsx --encrypt \
  --username alice --password-env GOPMGR_PASSWORD \
  /path/to/project.gopmgr
```

## Logs and Startup Diagnostics

GoPMgr writes dated diagnostic logs under the GoPMgr logs directory. If
startup fails, the app records the failure and shows a native OS error
dialog that names the log path.

The CLI maintenance paths continue to log to stderr, which is visible in
the terminal.

For common problems — forgotten passphrase, no-administrator lockout,
suspected corruption, unsigned exports, missing glyphs — see the
Troubleshooting & FAQ section of the in-app Help Guide.

## Keyboard Shortcuts and Accessibility

Shortcuts use Ctrl on Windows/Linux and Cmd on macOS:

| Shortcut | Action |
| --- | --- |
| Ctrl/Cmd+N | New project (Launchpad) |
| Ctrl/Cmd+O | Open project |
| Ctrl/Cmd+D | Portfolio dashboard |
| Ctrl/Cmd+, | Application settings |
| Ctrl/Cmd+W | Close the current project |
| Ctrl/Cmd+Q | Quit |
| Ctrl/Cmd+S | Save the open chart or document editor |
| Tab / Shift+Tab | Move between diagram nodes; cycle inside dialogs |
| Enter or Space | Select the focused diagram node |
| Esc | Close/cancel the open dialog from anywhere inside it |

GoPMgr is operable without a mouse and announces state to assistive
technology: every navigation announces the destination view, save
status and errors are live regions, rendered charts expose text
descriptions, Gantt bars have descriptive hover tooltips, dialogs trap
and restore focus, and animations honor the OS reduced-motion setting.

## Editor Save Behavior

All document and chart editors support Ctrl+S / Cmd+S. Auto-save is also
available from Application Settings. Auto-save is snapshot-based, so idle
editors do not rewrite unchanged data or churn `updated_at`.

Navigation, project close, and native window close protect registered
editor changes with Save, Discard, and Cancel choices. Sign-out cannot be
reached while an editor has unsaved changes, so no data loss is possible
there either. After the desktop renderer is ready, native close uses the
editor's current local dirty state;
a failed automatic or manual save remains dirty and must be retried or
explicitly discarded.

Open work-item, sprint, and stakeholder drafts are registered with the same
Save, Discard, and Cancel protection, but are not timed-auto-saved because a
successful modal save closes the editor. If a modal field changes while a save
is in progress, the first saved result stays in place and the later change
remains open for another Save before continuing.

Stakeholder money values are retained exactly while their stored minor-unit
amounts are safe JavaScript integers. If an older value is too large for that
representation, the app refuses to save it rather than risk rounding it; use a
version with decimal-string money transport before changing that record.

Document editors show an unsaved-changes indicator and a status dropdown
for `draft`, `review`, `approved`, and `archived`.

## Project Backups

Project Settings can create an integrity-checked `.pmba` backup containing a
database snapshot, a versioned manifest, and SHA-256 entry digests. Restore is
available from the project picker and always creates a new project instead of
overwriting existing data. Bundled certificates are checked but are not
automatically imported. An encrypted archive can be restored only by an
account that can unlock its project database.
