<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge User Guide

This guide summarizes the main user workflows that were previously mixed
into the root README. The in-app Help Guide remains the most detailed
end-user reference.

## First Run

PMForge stores local data under `~/Documents/PMForge/` by default. The
first launch creates:

- `system.db` for local account metadata.
- A private per-user directory with `projects`, `certs`, and `exports`
  subdirectories.
- A `logs` directory for dated diagnostic logs.

Create the first account, save the one-time recovery codes, then create or
open a project.

## Project Launchpad

`New Project` opens the Launchpad instead of a blank one-field form. The
Launchpad asks for:

1. Industry: Business, Administration, Engineering, Software,
   Construction, or Custom.
2. Sub-category, tailored to the industry.
3. Methodology, such as Scrum, Kanban, CPM, Waterfall, Six Sigma, Lean,
   OKRs, PRINCE2, or PMBOK.
4. Project details and starter artifacts.

The suggested starter artifacts come from the embedded Launchpad rule set.
The user can deselect any suggestion before creating the project.

## Portfolio and Dashboard

After sign-in, PMForge opens the Portfolio dashboard. It lists projects
with status, phase, dates, and chart/document counts. Open a project card
to enter its Dashboard.

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
  isolated scenario partition for later schedule comparison work.
- Compliance mode, which verifies the tamper-evident audit chain before a
  project opens and blocks the open if the chain has been altered. Project,
  chart, document, and schedule-baseline lifecycle actions are included in the
  chain, along with document signature success and failure checkpoints. Use
  **Export audit verification report** to write a private JSON verification
  artifact to the user exports folder for compliance review. Use **Export audit
  repair evidence** before manual repair work to preserve the raw audit events
  and verification failure details separately.
- Document font selection and per-project imported fonts.
- Schedule reports and project interchange exports.
- Database encryption migration for eligible plaintext project databases.

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

## Stakeholders, Timeline, and Budget

The Dashboard exposes:

- **Stakeholders:** a project address book for team members, vendors,
  sponsors, and external contacts. Stakeholders can carry rates,
  contract values, and availability.
- **Timeline:** a chronological project strip with project dates, sprint
  ranges, milestones, deployments, and country-aware holidays.
- **Budget:** a live rollup of project budget against vendor contracts
  and work-item estimates. Currency calculations are kept at cent precision,
  so fractional labour estimates round once at the money boundary.
- **Resource assignments:** CPM tasks can carry resource units, optional
  calendar labels, max-unit caps, and skill tags. Project Settings stores named
  resource calendars with weekly capacity and day overrides; leveling uses
  those calendars to delay contended tasks.

## Charts

PMForge supports 21 chart kinds across four engine families:

- DAG: WBS, Network, PERT, CPM, Gantt, Fishbone, Cause-and-Effect.
- Flow: Workflow, Activity.
- Matrix: RACI, SWOT, Stakeholder Analysis, Matrix Diagram.
- Stats: Line, Bar, Pareto, Pie, Burn-Up, Burn-Down, Cumulative Flow,
  Control.

Charts are edited in the app and can be embedded into PDF reports as
vector drawings rather than screenshots.

CPM charts can generate a Resource Histogram. The generated histogram shows
resource demand as bars and overlays dashed capacity lines from stakeholder
availability and Project Settings Resource Capacity calendars.
CPM and Gantt task over-allocation badges use the same Resource Capacity
calendars when a project has a start date.

## Documents and Combined Reports

PMForge supports 25 document kinds across the project lifecycle, including
charters, plans, schedules, budgets, risk registers, requirements, reports,
issue logs, change requests, and closure documents.

Every document kind can export to PDF, DOCX, and ODT. XLSX is available
where the document kind benefits from spreadsheet output.

Combined reports let users assemble several project documents into one
PDF. From the Dashboard, choose **New document -> Combined Report**, add
documents, order sections, optionally add section introductions, then
export the report. Chart references inside included documents render as
dedicated vector chart pages.

Status Reports can link a CPM schedule chart. When that linked schedule has
cost and progress data, combined reports include an Earned Value summary under
the Status Report section.

## Schedule Import and Export

From Project Settings, PMForge exports the current schedule to:

| Format | Extension | Use |
| --- | --- | --- |
| PDF / DOCX / ODT | `.pdf` / `.docx` / `.odt` | Reports |
| MS Project XML | `.xml` | Interchange |
| CSV | `.csv` | Spreadsheet task lists |
| HTML | `.html` | Browser viewing or publishing |

PMForge imports Microsoft Project XML (MSPDI, `.xml`) directly. Binary or
serialized formats such as `.mpp`, `.pod`, and `.mpx` should be resaved as
Microsoft Project XML from the source application before import.

## PDF Signing

PDF signing uses a `.p12` or `.pfx` certificate. Users can configure a
certificate in Project Settings or choose one directly from the Digital
Signature dialog during a signing export.

Signing is applied after rendering and PDF/A metadata injection. This order
is required because the signature covers byte ranges in the final PDF.

## Recovery Codes and Encryption

PMForge issues one-time recovery codes at account creation. Recovery codes
can reset an account password once and, for encrypted project databases,
also unlock the user's wrapped DEK.

After migrating an existing plaintext project database to encrypted
storage, recovery codes must be reissued so active codes can unlock the
DEK. If the password and all valid wrapped recovery codes are lost,
encrypted project databases are unrecoverable by design.

## Fonts

PMForge embeds TrueType fonts in generated PDFs. The bundled font catalog
is downloaded with:

```sh
make fonts
```

Users can also import a `.ttf` font from Project Settings. Imported fonts
are stored in the user's PMForge data area and can be selected for document
exports.

## Logs and Startup Diagnostics

PMForge writes dated diagnostic logs under the PMForge logs directory. If
startup fails, the app records the failure and shows a native OS error
dialog that names the log path.

The CLI maintenance paths continue to log to stderr, which is visible in
the terminal.

## Editor Save Behavior

All document and chart editors support Ctrl+S / Cmd+S. Auto-save is also
available from Application Settings. Auto-save is snapshot-based, so idle
editors do not rewrite unchanged data or churn `updated_at`.

Document editors show an unsaved-changes indicator and a status dropdown
for `draft`, `review`, `approved`, and `archived`.
