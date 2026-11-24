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

## Table of contents

- [First Run](#first-run)
- [Project Launchpad](#project-launchpad)
- [Portfolio and Dashboard](#portfolio-and-dashboard)
- [Reusable suppliers and items](#reusable-suppliers-and-items)
- [Project Settings](#project-settings)
- [Application Settings](#application-settings)
- [Stakeholders, Timeline, Budget, and Cost Control](#stakeholders-timeline-budget-and-cost-control)
- [Kanban, Sprints, and DORA](#kanban-sprints-and-dora)
- [Charts](#charts)
- [Scheduling Constraints](#scheduling-constraints)
- [Documents and Combined Reports](#documents-and-combined-reports)
- [Schedule Import and Export](#schedule-import-and-export)
- [PDF Signing](#pdf-signing)
- [Recovery Codes and Encryption](#recovery-codes-and-encryption)
- [Administrator Access to Users' Data](#administrator-access-to-users-data)
- [Fonts](#fonts)
- [Command-Line Maintenance](#command-line-maintenance)
- [Logs and Startup Diagnostics](#logs-and-startup-diagnostics)
- [Keyboard Shortcuts and Accessibility](#keyboard-shortcuts-and-accessibility)
- [Editor Save Behavior](#editor-save-behavior)
- [Project Backups](#project-backups)
- [Subject index](#subject-index)
- [Glossary](#glossary)

Use the contents for reading order, the [subject index](#subject-index) for
alphabetical lookup, and the [glossary](#glossary) for meanings and distinctions.
The same reference terms are available in the in-app Help Guide.

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

**Start with a blank project** on the first step skips the wizard: it asks
only for a name and description and creates an empty project with the US
business calendar, which can be changed in Project Settings.

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

## Reusable suppliers and items

Choose **Suppliers & items** from an open project's Dashboard to maintain a
private reusable catalog for the signed-in user. Supplier records can hold a
name, postal address, phone, fax, email, primary contact, and notes. Item
records can hold a name, SKU, kind, default unit, and description. Archive a
record instead of deleting it when it should no longer be offered for future
selection.

The catalog is encrypted and separate from individual projects. Cost Control
ledger entries can independently store item, SKU, supplier display name,
invoice reference, quantity, unit, and attachments as project-local snapshots.
Editing a catalog record never rewrites a historical ledger entry.

In Cost Control, use **Find catalog item** or **Find catalog supplier** to
search active reusable records, then copy a selection into the entry. Item
copying fills item name, SKU, and default unit; supplier copying fills only the
supplier display name. Every copied value remains editable before save, and no
catalog ID/version is stored on the project ledger row. Supplier addresses and
contact details are not copied automatically. A catalog lookup problem does not
block direct manual entry.

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
  schema-migrated plaintext `.pre-encryption.bak` until the user removes it;
  Project Settings offers to delete it once the encrypted project is open.
  A synchronous migration failure leaves the canonical plaintext project
  readable; protect or remove that backup only after verifying the encrypted
  project opens successfully.

The classification fields feed the Launchpad rules and calendar-aware
timeline overlays.

## Application Settings

Application Settings holds user-level preferences that are separate from
per-project settings:

- Application theme.
- Default document font.
- Default export theme for new projects.
- Auto-save on/off and interval.
- Version and data-location information.
- The Beta Center: signed-update status, an opt-in toggle to check for
  updates automatically on launch (off by default), a manual "Check for
  updates" action, and — once a newer signed build is available — a
  "Download & Open Installer" action that verifies the download before
  handing it to the OS's own installer. Current release limitations, logs,
  and privacy-safe bug-report generation are also here.

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
  select USD, EUR, GBP, CAD, AUD, CHF, or JPY only while the project has no
  Budget value, Cost Control entry, non-zero reserve balance, or approved Cost
  Control baseline. It cannot change while any of those values exists because
  Phase 1 has no FX conversion. Documents, their DOCX and ODT exports,
  combined reports, and Six Sigma solution costs (the Solution Matrix and the
  Six Sigma report) label amounts in this currency. Schedule reports print
  earned value in it too, with thousands separators and whole yen for JPY.
  The dashboard shows
  its separate legacy Budget rollup for context, then the Cost Control base
  plan, contingency reserve, cost baseline, management reserve, authorised
  funding, commitments, and actuals. The legacy Budget rollup does not feed
  Cost Control calculations. Phase 1 does not present an unallocated or
  remaining-spend figure because allocation, reserve drawdowns, and forecast
  policy are not yet defined.
  Contingency and management reserves are separate records and do not count as
  commitments or actual costs. Phase 1 does not yet provide reserve drawdowns,
  foreign exchange, time-phased cash flow, or
  automatic EVM integration. Yen amounts show as whole numbers, such as
  JPY 150000. An older JPY project keeps every value exactly as recorded, so
  an amount entered with fractions of a yen still shows them, such as
  JPY 123.45; nothing is converted, and its Cost Control is editable again.

Each Cost Control entry inherits three cost-type classifications: direct or
indirect attribution, fixed or variable behavior, and CapEx, OpEx, or not
applicable treatment. The dashboard keeps inactive types visible on historical
ledger rows but excludes them from the entry selector. Its classification table
reconciles every one of the three lenses independently to the same ledger
totals. Do not add values between lenses: the same ledger entry appears once in
each lens.

Use **Cost item or reference** for the required free-text description of a
material, input, invoice, supplier reference, or overhead basis. The optional
**Procurement detail** fields independently store item name, SKU, supplier
display name, invoice reference, quantity, and unit on that project ledger
row. Quantity uses an exact three-decimal value and requires a unit; it does
not calculate or replace the entry amount. Search matches the description,
item name, SKU, supplier display name, and invoice reference. **Quantity by
item & unit** aggregates only
identical item/unit pairs without unit conversion. The form accepts direct
entry and can copy selected active catalog defaults into these editable
project-local snapshots; it never makes a posted row depend on the catalog.

Open **Attachments** on a ledger row to add and inspect bounded encrypted
files. **Export ledger attachments (.zip)** writes the project's original
attachment files and a manifest to a user-selected new ZIP file, checking each
fetched file against its stored byte count and SHA-256 before archiving. File
names inside the ZIP are adjusted where needed to follow Windows, macOS, and
Linux naming rules, and names that differ only in letter case get distinct
entries; the manifest keeps each original file name. The
printable financial report shows procurement detail and quantity aggregates,
but does not embed attachment bytes.

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
- **Work items:** each has a title, type, priority, estimate, assignee,
  column and sprint. The estimate field is called **Story points** in Scrum
  and Scrumban projects and **Points** in Kanban projects.
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
bars then show drift against that snapshot as the plan changes. **Baselines**
in the CPM and Gantt editors lists a chart's snapshots, newest first, and
deletes one after confirmation; variances then compare against the newest
remaining one. A baseline a what-if scenario uses as its source can't be
deleted until the scenario's source changes.

CPM charts can generate a Resource Histogram. The generated histogram shows
resource demand as bars and overlays dashed capacity lines from stakeholder
availability and Project Settings Resource Capacity calendars.
CPM and Gantt task over-allocation badges use the same Resource Capacity
calendars when a project has a start date. A resource is overallocated when,
at some moment, the tasks working then need more of it than it has. Two
half-day tasks for one person are fine one after the other and an overload
at the same time, even though both use half a day. A task that needs more
of a resource at once than it has (two people from a team of one, or full
time from someone half-time) is always an overload, and levelling cannot
place it. When an assignment names a resource calendar, that calendar sets
the capacity for that assignment, in the badges and in levelling alike;
the histogram's capacity line shows the resource's own calendar. The
histogram's bars are each day's total demand, which can be within capacity
on a day with an overload, because the overload is an overlap. Only the
tasks working at an overloaded moment get a badge. The split preview
judges capacity the same way.

CPM charts also include a Monte Carlo risk panel. Add optional optimistic,
most-likely, and pessimistic duration estimates to tasks that carry schedule
uncertainty, choose triangular, beta-PERT, or normal sampling, then run the
simulation from the CPM editor aside. The result shows P50/P80/P90 finish-day
confidence points, a cumulative finish-probability S-curve, and a tornado
driver ranking that combines critical-path frequency with P90-P50 duration
spread. After a simulation runs, use **Export PDF/A** to save a Monte Carlo
risk report with the confidence summary, S-curve, tornado drivers, and narrative
summary. Tasks without estimates use their fixed CPM duration.

The CPM editor aside also has a **Schedule quality** panel. Choose the
thresholds (GoPMgr default or DCMA-style, which is guidance, not a DCMA
assessment) and a status date, then choose **Run audit**. GoPMgr first saves
edits it has not saved yet, then checks the stored schedule for missing logic,
cycles, leads and long lags, hard and violated constraints, negative or high
float, invalid actual dates and missing progress. The audit changes nothing.
Rules that could not run are listed, with the reason, before the findings, so
an empty list never hides an unchecked rule. **Show task** selects a finding's
task on the chart.

## Scheduling Constraints

A constraint ties a CPM task to a date, or to its late dates. Set it in the
CPM editor: select the task, choose a **Constraint**, and, for the six types
that carry one, a **Constraint date**.

### The rule that governs every constraint: links win

GoPMgr schedules from the links first. A constraint can delay a task or limit
how late it may run, but it never moves a task earlier than its links allow,
and it never breaks a link. When the links and a constraint cannot both hold,
the task keeps the dates its links give it and the constraint is marked
**violated**. The editor then shows "Constraint violated". The task usually
gets negative float, and the schedule audit reports it under SQ-09.

This is how Primavera P6 treats its "Start On" and "Finish On" constraints.
Microsoft Project's default, "honor constraint dates", does the opposite for
its four inflexible constraints (must start on, must finish on, start no
later than, finish no later than): it holds the task to the date even when
that breaks the logic. A schedule moved between the two can therefore show
different dates for these tasks.

### The eight types

| Constraint | Date limits | Effect on the early dates | Effect on the late dates | Violated when | DCMA-style |
| --- | --- | --- | --- | --- | --- |
| **ASAP** (as soon as possible) | none | the task starts as soon as its links allow | none | never | none |
| **ALAP** (as late as possible) | none | the task is moved to its late dates, using up its own float | none | never | soft |
| **SNET** (start no earlier than) | start | the start is delayed to the date if the links allow an earlier one | none | never | soft |
| **SNLT** (start no later than) | start | none | the late start is capped at the date | the links make the task start after the date | hard |
| **FNET** (finish no earlier than) | finish | the finish is delayed to the date if the links allow an earlier one | none | never | soft |
| **FNLT** (finish no later than) | finish | none | the late finish is capped at the date | the links make the task finish after the date | hard |
| **MSO** (must start on) | start | the start is pulled to the date | the late start is pinned to the date | the links make the task start after the date, or a later task needs it to finish sooner | hard |
| **MFO** (must finish on) | finish | the finish is pulled to the date | the late finish is pinned to the date | the links make the task finish after the date, or a later task needs it to finish sooner | hard |

**Hard and soft.** Soft constraints only delay a task. Hard constraints fix
how late a task may run, so the schedule can no longer show what a delay does
to that date. The schedule audit counts the hard ones under SQ-08, and its
DCMA-style thresholds expect them on at most 5 % of tasks.

### Worked examples

These examples use a project starting Monday 1 June 2026 on a Monday-to-Friday
calendar. Each one is a test of the scheduler.

- **SNET.** Task B (2 days) follows A (1 day), so the links allow Tuesday 2 June.
  SNET Thursday 4 June delays B to start on Thursday. It is not violated.
- **SNLT.** B (2 days) follows A (1 day), and a separate 8-day task sets the
  project finish. B starts Tuesday. SNLT Thursday caps B's late start at
  Thursday, so B has 2 days of float instead of 5. If A took 4 days instead, B
  could not start until Friday. That would miss an SNLT Tuesday: B is violated,
  and both tasks show −3 days of float.
- **FNET.** A 2-day task with FNET Friday 5 June runs Thursday to Friday. A
  task whose links already make it finish later is not moved.
- **FNLT.** B (3 days) after A (3 days) cannot finish by FNLT Thursday 4 June.
  B is violated, and the squeezed late dates make the chain critical with
  negative float.
- **MSO.** B (2 days) after A (1 day) with MSO Thursday 4 June starts on
  Thursday and finishes on Friday. If A took 4 days, B could not start on
  Tuesday 2 June (an MSO Tuesday): B keeps the start its links give it, is
  violated, and shows −3 days of float.
- **MFO.** A 2-day task with MFO Wednesday 3 June runs Tuesday to Wednesday.

### Dates, calendars and milestones

- **Calendar.** Constraint dates count working days on the project's country
  calendar, from the project start date.
- **Non-working days and dates before the start.** An earliest limit reads
  such a date as the next working day, and a latest limit as the previous
  one. SNET or FNET on a Saturday takes effect from Monday; SNLT or FNLT on a
  Saturday means "by Friday"; an SNLT or FNLT dated before the project start
  cannot be met by a task with work, because no work happens before the
  start (a milestone at the start meets an FNLT on the working day before). MSO and MFO need
  the task to start or finish on the date itself, so on a non-working day or
  before the start they can never be met: the task is placed on the next
  working day and flagged as violated, and the audit says the calendar is
  the cause. Put must dates on working days within the project.
- **No start date.** Without a project start date, date constraints are not
  applied. The task panel says so, and so does the audit.
- **Milestones.** A milestone (zero duration) sits on the boundary after the
  work before it. After a task that ends on Wednesday, the milestone is
  scheduled at the start of Thursday, and the chart shows it on Thursday. A
  finish constraint means "by the end of that day", so an MFO or FNLT
  Wednesday milestone after such a task is met. For the same reason a
  milestone held by a finish constraint (MFO or FNET Wednesday) appears on the
  next working day, Thursday; a milestone held by a start constraint (MSO
  Wednesday) appears on Wednesday.
- **Durations under one day.** A task with a duration between 0 and 1 day,
  which PERT estimates can produce, is shown on the day its work starts, so
  two half-day tasks in a row both show on Monday. Its resource demand is
  counted on the same days, by the part of each day it uses, so the
  overallocation badge and histogram match its bar.
- **A date that cannot be applied.** If the constraint date is missing or
  not a valid date, or lies too far from the project start, GoPMgr does not
  apply the constraint. The node shows a grey **?** marker, the task panel
  says "Constraint not applied" and why, and the schedule audit reports it
  under SQ-09.
- **Lower-case values.** A constraint stored in lower case, for example from
  hand-edited data, is read the same way. The editor shows it under its usual
  name, and opening the task does not change the stored value.

### Constraints, levelling and imports

- **Levelling.** Levelling resources keeps each delay it adds as the task's
  **levelled start**, shown in the task panel, with the time of day when
  the task starts part-way through a day (on a standard 08:00 to 17:00 day
  with a lunch hour, so half a day in is 13:00). Only tasks that need one
  get one: a task its links already move as far, after a delayed task,
  gets none. It never writes, changes or removes a constraint. Each run recalculates every levelled start from the
  links and constraints, so a delay that is no longer needed is cleared. A
  levelled start delays the task like SNET but is never violated itself. If
  a delay pushes an SNLT, FNLT, MSO or MFO task past its date, the task is
  flagged, levelling names it, and the audit names levelling as the cause.
  Select **Clear** next to a levelled start to schedule that task from its
  links and constraint alone until you level again. An ALAP task that
  levelling places keeps its levelled start and stays there, rather than
  moving to its late dates, so it never lands on work levelling moved out
  of its way.
- **After you edit.** Editing a task does not clear its levelled start. When
  you change a duration, a link, a constraint or an assignment after
  levelling, the CPM and Gantt editors say "The schedule changed since
  levelling. Level again to update the levelled starts." It also appears
  when the project start date, the work calendar, or the capacity of a
  resource or calendar the chart's assignments use changes. You can
  dismiss the note. Charts levelled before 21 November 2026 never show it
  until levelled again, and charts levelled between then and 23 November
  2026 show it until levelled again.
- **Part days.** Levelling places each task at the first moment it fits:
  its earliest start, part days included, then each moment a task before it
  on the same resource finishes, then each following day. Part-day tasks for
  one person run one after the other within the day, so a chain of short
  tasks is not spread over separate days. Splitting a task around busy days
  books whole days; the chart keeps the split through edits in the CPM and
  Gantt editors, and every schedule, report, export and Gantt bar uses
  those days while they still fit the task. If an edit makes them stop
  fitting (a duration needing a different number of whole days, or a start
  part-way through a day), the task is scheduled and drawn in one piece
  again and the levelling note appears.
- **Charts levelled before 18 November 2026.** Earlier versions stored
  levelling delays as SNET constraints. Those SNETs look like any other and
  are kept. Remove the ones levelling added, then level again.
- **Monte Carlo.** Each simulated run applies date constraints and levelled
  starts as the schedule does, so with every duration at its estimate the
  simulation finishes when the schedule does. A must date (MSO, MFO) holds
  its task in every run. Without a project start date, neither is applied.
  A schedule the scheduler refuses as too long is refused by the simulation
  too.
- **Imports and exports.** MSPDI import brings each task's constraint type and
  date when **Constraints** is selected (the default). A constraint of an
  unknown type, or a dated one without a usable date, is left out and counted
  in the import receipt. Microsoft Project by default holds a task to its
  constraint date even against its links; GoPMgr keeps the links and flags the
  constraint instead, so imported dates can differ. Export writes each
  constraint and its date. Export also writes each task's levelled start as
  Microsoft Project's levelling delay (LevelingDelay): how long levelling
  held the task back from where its links and constraints put it, measured
  on the clock. Like Microsoft Project files, the file holds it in tenths of
  elapsed minutes, shown in elapsed days when it is a whole number of days
  and in elapsed minutes otherwise. (Microsoft's own example of this element
  shows the value in its display unit; GoPMgr follows the reading of
  MPXJ, the library most tools use to exchange these files.) Import reads it
  back when **Levelling delays** is selected (the default), whatever unit it
  is shown in, and stores it as a levelled start. It is placed using the
  file's own links and constraints even when you do not import them, and
  needs a project start date (the project's, or the file's when the project
  has none). Delays with no unit, on ALAP tasks, or that cannot be read are
  left out and counted in the import receipt; ALAP levelled starts are not
  exported, because how Microsoft Project applies a delay to an ALAP task
  could not be checked.
  Part-day start and finish times are written on the same 08:00 to 17:00
  day.

### Choosing a constraint

- **Prefer links.** A link explains why a date moves; a constraint only
  states that it may not.
- **Prefer soft constraints.** Use SNET or FNET when work cannot start or
  finish before a date, for example a delivery or a permit.
- **Use hard constraints sparingly.** They cap how late work may run. Treat a
  deadline as a deadline: an FNLT shows when the plan misses it, and an MFO or
  MSO also pins the late dates, hiding any slack.

### Charts saved before 18 November 2026

Earlier versions scheduled an MSO, SNLT or FNET task as ASAP. A chart that
holds one of these types now schedules by it, so its dates, float, critical
path and SQ-08 counts can change. Later the same day, constraints dated on
non-working days or before the project start became strict, as described
above, and tasks shorter than a day began to show on the day their work
starts. Saved baselines are not changed, so variances against them can change
too. No conversion is needed.

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

A combined report can also include a **schedule audit** of any CPM chart:
add it under **Schedule audits** and choose its thresholds and status date.
The audit runs when you export, so the PDF shows the findings for the chart as
stored at that moment, and the manifest records the chart revision, the inputs
the audit read and a digest of its result. Signed (PAdES or GnuPG) exports
can include a schedule audit; certified reports cannot.

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

PDFs are PDF/A-2u, the archival format the US National Archives prefers for
documents. Each file embeds its fonts and color profile, and its text copies,
searches, and reads aloud correctly. A PAdES signature keeps the file
PDF/A-2u.

A schedule can be at most 18,250 working days long. That is at least 50
years on any work calendar, and about 70 on a Monday-to-Friday one. GoPMgr
refuses to save a CPM or Gantt chart, or to import a file, whose task
duration, lag or overall length is beyond that, and names the task
responsible.

GoPMgr imports Microsoft Project XML (MSPDI, `.xml`) directly. Binary or
serialized formats such as `.mpp`, `.pod`, and `.mpx` should be resaved as
Microsoft Project XML from the source application before import.

Before selecting a file, choose whether to import dependencies, percent
complete, resource assignments, constraints and levelling delays (see
Scheduling Constraints). GoPMgr always preserves task identities,
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

The codes are shown once. **Copy** puts them on the clipboard, and GoPMgr
clears the clipboard 60 seconds later, or when you quit, if it still holds
those codes; anything you copied since is left alone. Clipboard history and
syncing apps can keep their own copy, so saving or printing is safer.
**Save as
.txt…** writes them to a new text file you choose, named with the date;
**Save printable PDF…** writes a one-page sheet with the codes, how to use
them, and a box to tick off each one. Print the sheet, then delete the PDF.
GoPMgr never replaces an existing file. Keep a copy somewhere other than
this computer. To make new codes, open App Settings › Account and choose
**Create new recovery codes**; they replace the old ones only after you
choose **Use the new codes**, so a file or sheet saved before that holds
codes that won't work if you keep your current ones.

After migrating an existing plaintext project database to encrypted
storage, recovery codes must be reissued so active codes can unlock the
DEK. If the password and all valid wrapped recovery codes are lost, you
can't open your encrypted projects again. An administrator can view them
read only through recorded access (below), but that doesn't give them back
to you.

## Administrator Access to Users' Data

The super administrator can open another account's projects from the Admin
panel with **Open data**, after giving a reason. They see each project read only:
its summary, schedule, costs, and the list of documents, not the documents'
contents. Nothing can be changed, exported, or printed. Every opening is
recorded in the Admin panel's account history with who opened which account,
when, and why. An account that has not signed in since the current
administrator key was made can't be opened until it does.

One administrator is the super administrator: on a new machine, the first
account. Only the super administrator changes administrators, opens users'
data, and holds the administrator key; other administrators create, disable,
and delete standard accounts. The super administrator can hand the role to
another administrator who has signed in with **Make super administrator**,
and becomes an ordinary administrator.

Under **Succession** in the Admin panel, the super administrator can name a
standby successor and set how many days they may go without signing in (7
to 365, 30 by default). The standby holds the administrator key but is an
ordinary administrator until then. Once the super administrator hasn't
signed in for that long, the standby sees **Become super administrator**
when they open the Admin panel; with no standby, or once the standby hasn't
signed in for that long either, any administrator does, but they won't hold
the administrator key, so no one can open users' data until it is replaced.
Whoever takes over chooses a reason, which sets how long the former super
administrator can't be disabled, deleted, or removed as an administrator:
vacation 30 days, parental leave 180, medical or convalescence leave 90, no
longer an employee 7, or another reason, described in a note, 30. Every
administrator can see the reason and note in the account history, so leave
out medical or personal details. The former super administrator becomes an
ordinary administrator, and the new one can hand the role back, which ends
the protection. Each change is recorded in the account history.

GoPMgr replaces the administrator key whenever someone who held it stops
holding it: after a hand-over or a takeover, or when the standby is changed,
removed, disabled, or deleted. Nobody has to do anything; every account is
moved to the new key, and the account history records it. Only the super
administrator and a standby they keep get the new key. If the panel says
your standby doesn't hold the administrator key, choose **Pass the key**. The
super administrator can also choose **Replace administrator key** under
Succession at any time. A copy of GoPMgr's account file made before a
replacement still opens the data sealed in it, so keep such copies safe.

The super administrator should also make a **recovery code** for the
administrator key: the Admin panel reminds them until one exists. The code
is shown once. Print it (GoPMgr lists only printers that print on paper;
choose one connected to this computer) or write it down, keep it somewhere
safe away from the computer, then type it back to confirm. If nobody can
open the administrator key any more, the super administrator enters the
code under Succession to restore it. Together with GoPMgr's account file
the code opens every user's data, so keep it as carefully as a password.
Replacing the administrator key voids the code, and the panel asks for a
new one.

If nobody holds the administrator key and the recovery code is lost, the
super administrator chooses **Make a new administrator key** under
Succession and says why. Every user's data moves to the new key the next
time they sign in; until then, no one can open it. A disabled account moves
only after it is enabled and signs in. Every user is shown the reason, so
leave out medical or personal details. If your standby still holds the key,
GoPMgr asks you to remove them as standby first, since the key isn't lost.

If an administrator opens your data, or the super administrator makes a new
administrator key, you see who did, when, and their reason on the first
screen after you sign in, until you choose **I've read this**; the Admin
panel's history records that you read it. App Settings, under
Account, lists every time it has happened. This relies on GoPMgr's account
record, which someone who can edit GoPMgr's files on this computer could
change.

## Fonts

GoPMgr uses the IBM Plex typefaces everywhere: IBM Plex Sans for the app and
for documents and reports, and IBM Plex Mono for codes and identifiers, such as
the recovery-code sheet. Every build includes them, so PDFs embed their fonts
and look the same on any computer, with no download.

Choose a project's document font in Project Settings, under Document Font.
IBM Plex Serif suits long documents meant for printing, and IBM Plex Sans
Condensed fits more text in dense tables. For documents in Japanese, Korean,
or Simplified Chinese, choose IBM Plex Sans JP, KR, or SC; each also covers
Latin text. The choice applies to the project's documents exported as PDF,
DOCX, or ODT; schedule reports always use IBM Plex Sans. You can also import a
`.ttf` font there. Imported fonts are stored in your GoPMgr data area and can
be selected for document exports. GoPMgr refuses a font whose license doesn't
allow embedding it in documents.

The app shows Arabic, Hebrew, Devanagari, and Thai text in IBM Plex. PDFs
can't lay out those scripts, so their letters print unjoined, backwards, or
with misplaced marks; export documents in those scripts as DOCX or ODT, which
word processors lay out correctly. For Traditional Chinese PDFs, import a font
that covers it.

PDF fonts can't show emoji. In a PDF, each emoji prints as � (an empty box in
IBM Plex Sans JP, KR, and SC), so a reader can tell something was there; the
app itself shows emoji normally.

DOCX, ODT, XLSX, and HTML files name their font but do not include it. Someone
who opens them without IBM Plex installed sees a similar font chosen by their
software.

Developers can download the additional optional font catalog with:

```sh
make fonts
```

## Command-Line Maintenance

GoPMgr also runs headless for scriptable maintenance. Two flags are
global:

- `--version` prints the version banner and exits.
- `--update` checks the signed update channel (a no-op when the build has
  no update channel configured).

The maintenance operations take a project file path — `.gopmgr`, or `.pmforge`
for a project created before the August 2026 rename — as the final argument:

- `--check` runs an integrity check and exits.
- `--repair` checks integrity and, if it finds damage, writes a copy rebuilt
  from the readable data beside the file as `<file>.bak`, without checking it
  or replacing the project, and prints how to use it. It exits 1 when it
  found damage (the project still needs repair) and 0 when the project is
  healthy, like `--check`. To repair a project in
  place, open it and use **Check and repair** in Project Settings › Data
  Protection, which checks a rebuilt copy before swapping it in and keeps the
  damaged file as `<file>.corrupt`. With **Check this project for damage when it opens** on
  (the default), each open runs an integrity check and the Dashboard shows a
  warning with a way to Check and repair if it finds damage; it never
  repairs on its own.
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

WBS preview refreshes and Save share one writer. Changes made while saving stay
in the working draft; newer preview requests are combined and processed after
the current write. Navigation remains guarded while a write is pending or newer
changes remain unsaved. A “Saved, but layout failed” message means the chart data
was saved, but its preview could not refresh; use Save again to refresh it.
Discard cannot undo a write already submitted to the database.

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

A restored project starts with signing and timestamping switched off, and
without the backup's certificate paths, GnuPG key or timestamp server: those
name files, keys and a server chosen on the machine that made the backup. Set
signing up again in Project Settings. The project's audit log records that
the settings were cleared.

A backup does not contain your signing certificate unless you ask. The
certificate is a `.p12` or `.pfx` file that holds your private signing key.
When the project has one, Project Settings shows "Include my signing
certificate file in this backup" beside the Create backup button. It is off
each time, restoring never needs it, and the project's audit log records when
a backup was made with it. Keep a backup made with it as private as the
certificate itself. Backups made before 2026-11-11 always included the file.
A backup that includes the certificate is refused, with a
message naming the file, when the project's certificate setting points at
anything else: a file with another extension, something that is not an
ordinary file, or a file larger than 16 MiB. Choose the certificate again in
Project Settings and retry. A certificate that has been moved or deleted is
left out without an error.

The digests catch a damaged backup, not a deliberately altered one: whoever
alters a backup can rewrite its digests too. Restore only backups you made or
trust. As a safeguard, GoPMgr refuses a backup whose project file contains
anything GoPMgr never creates, and tells you it may have been altered: a
database trigger, view or virtual table, an extra table or column, or one of
GoPMgr's own tables redefined with a different constraint, default or index.
Backups made by older versions of GoPMgr are still accepted.

Deleting a project from the project picker removes it permanently. The
picker's **Deleted projects** list records each deletion: when and by whom,
whether it finished, and whether the project's audit trail verified at that
moment. It is a record only; to bring a project back, restore it from a
backup.


## Subject index

Topics lead to instructions; definitions lead to individual glossary entries.
Abbreviations and alternate labels are indexed alongside full terms.

| Subject or term | Lookup |
| --- | --- |
| AC | [Definition](#user-term-actual-cost-ac) |
| Account recovery codes | [Definition](#user-term-recovery-codes) |
| Accountability | [Definition](#user-term-accountability) |
| Actual Cost (AC) | [Definition](#user-term-actual-cost-ac) |
| Administrator Access to Users' Data | [Topic](#administrator-access-to-users-data) |
| Administrator Escrow | [Definition](#user-term-administrator-escrow) |
| Administrator-Key Recovery Code | [Definition](#user-term-administrator-key-recovery-code) |
| Agile | [Definition](#user-term-agile) |
| ALAP | [Definition](#user-term-scheduling-constraint) |
| Application Settings | [Topic](#application-settings) |
| Archival PDF | [Definition](#user-term-pdf-a-2u) |
| ASAP | [Definition](#user-term-scheduling-constraint) |
| Audit Trail | [Definition](#user-term-audit-trail) |
| Autosave | [Definition](#user-term-autosave) |
| Backlog | [Definition](#user-term-backlog) |
| Backup | [Definition](#user-term-backup) |
| Baseline | [Definition](#user-term-baseline) |
| Belt Level | [Definition](#user-term-belt-level) |
| Budget | [Definition](#user-term-budget) |
| Burn-Down Chart | [Definition](#user-term-burn-down-chart) |
| Burn-Up Chart | [Definition](#user-term-burn-up-chart) |
| Business Case | [Definition](#user-term-business-case) |
| Card / Task (Kanban) | [Definition](#user-term-card-task-kanban) |
| Change Failure Rate | [Definition](#user-term-change-failure-rate) |
| Change Request | [Definition](#user-term-change-request) |
| Charts | [Topic](#charts) |
| Command-Line Maintenance | [Topic](#command-line-maintenance) |
| Compliance Mode | [Definition](#user-term-compliance-mode) |
| Control Chart | [Definition](#user-term-control-chart) |
| Control Limits | [Definition](#user-term-control-limits) |
| Cost Performance Index (CPI) | [Definition](#user-term-cost-performance-index-cpi) |
| CPI | [Definition](#user-term-cost-performance-index-cpi) |
| CPM | [Definition](#user-term-cpm) |
| Critical Path | [Definition](#user-term-critical-path) |
| Critical Path Method | [Definition](#user-term-cpm) |
| CTQ | [Definition](#user-term-ctq) |
| Cumulative Flow Diagram | [Definition](#user-term-cumulative-flow-diagram) |
| Current Plan | [Definition](#user-term-current-plan) |
| Cycle Time | [Definition](#user-term-cycle-time) |
| Data Encryption Key | [Definition](#user-term-dek) |
| DEK | [Definition](#user-term-dek) |
| Deployment Frequency | [Definition](#user-term-deployment-frequency) |
| Digital Signature | [Definition](#user-term-digital-signature) |
| DMAIC | [Definition](#user-term-dmaic) |
| Documents and Combined Reports | [Topic](#documents-and-combined-reports) |
| DORA Metrics | [Definition](#user-term-dora-metrics) |
| Earned Value (EV) | [Definition](#user-term-earned-value-ev) |
| Earned Value (EVM) | [Definition](#user-term-earned-value-management-evm) |
| Earned Value Management (EVM) | [Definition](#user-term-earned-value-management-evm) |
| Editor Save Behavior | [Topic](#editor-save-behavior) |
| Encryption at Rest | [Definition](#user-term-encryption-at-rest) |
| Epic | [Definition](#user-term-epic) |
| ES / EF | [Definition](#user-term-es-ef) |
| Estimate (methodology-specific) | [Definition](#user-term-estimate-methodology-specific) |
| EV | [Definition](#user-term-earned-value-ev) |
| EVM | [Definition](#user-term-earned-value-management-evm) |
| Export | [Definition](#user-term-export) |
| First Run | [Topic](#first-run) |
| Fishbone Diagram | [Definition](#user-term-fishbone-diagram) |
| Float (Slack) | [Definition](#user-term-float-slack) |
| FNET | [Definition](#user-term-scheduling-constraint) |
| FNLT | [Definition](#user-term-scheduling-constraint) |
| Fonts | [Topic](#fonts) |
| Forecast | [Definition](#user-term-forecast) |
| Gantt Chart | [Definition](#user-term-gantt-chart) |
| Hard Constraint | [Definition](#user-term-hard-constraint) |
| Issue | [Definition](#user-term-issue) |
| Issue Log | [Definition](#user-term-issue-log) |
| Iteration | [Definition](#user-term-iteration) |
| Kaizen | [Definition](#user-term-kaizen) |
| Kanban Board | [Definition](#user-term-kanban-board) |
| Kanban, Sprints, and DORA | [Topic](#kanban-sprints-and-dora) |
| Key Result (KR) | [Definition](#user-term-key-result-kr) |
| Keyboard Shortcuts and Accessibility | [Topic](#keyboard-shortcuts-and-accessibility) |
| Launchpad | [Definition](#user-term-launchpad) |
| Lead Time | [Definition](#user-term-lead-time) |
| Levelled Start | [Definition](#user-term-levelled-start) |
| Logs and Startup Diagnostics | [Topic](#logs-and-startup-diagnostics) |
| LS / LF | [Definition](#user-term-ls-lf) |
| MFO | [Definition](#user-term-scheduling-constraint) |
| Milestone | [Definition](#user-term-milestone) |
| Monte Carlo Simulation | [Definition](#user-term-monte-carlo-simulation) |
| MSO | [Definition](#user-term-scheduling-constraint) |
| MSPDI | [Definition](#user-term-mspdi) |
| MTTR | [Definition](#user-term-mttr) |
| Objective | [Definition](#user-term-objective) |
| OKRs | [Definition](#user-term-okrs) |
| P50 / P80 / P90 | [Definition](#user-term-p50-p80-p90) |
| PAdES | [Definition](#user-term-pades) |
| Pareto Chart | [Definition](#user-term-pareto-chart) |
| Passphrase | [Definition](#user-term-passphrase) |
| Password | [Definition](#user-term-passphrase) |
| PDF Advanced Electronic Signatures | [Definition](#user-term-pades) |
| PDF Signing | [Topic](#pdf-signing) |
| PDF/A | [Definition](#user-term-pdf-a-2u) |
| PDF/A-2u | [Definition](#user-term-pdf-a-2u) |
| PERT | [Definition](#user-term-pert) |
| Planned Value (PV) | [Definition](#user-term-planned-value-pv) |
| Planning Meeting | [Definition](#user-term-planning-meeting) |
| PMBOK | [Definition](#user-term-pmbok) |
| Portfolio and Dashboard | [Topic](#portfolio-and-dashboard) |
| PRINCE2 | [Definition](#user-term-prince2) |
| Process Group | [Definition](#user-term-process-group) |
| Project Backups | [Topic](#project-backups) |
| Project Charter | [Definition](#user-term-project-charter) |
| Project Database | [Definition](#user-term-project-database) |
| Project Launchpad | [Topic](#project-launchpad) |
| Project Settings | [Topic](#project-settings) |
| Pull System | [Definition](#user-term-pull-system) |
| PV | [Definition](#user-term-planned-value-pv) |
| RACI Matrix | [Definition](#user-term-raci-matrix) |
| Recovery Codes | [Definition](#user-term-recovery-codes) |
| Recovery Codes and Encryption | [Topic](#recovery-codes-and-encryption) |
| Resource Capacity | [Definition](#user-term-resource-capacity) |
| Resource Leveling | [Definition](#user-term-resource-leveling) |
| Retrospective | [Definition](#user-term-retrospective) |
| Reusable suppliers and items | [Topic](#reusable-suppliers-and-items) |
| Risk | [Definition](#user-term-risk) |
| Risk Register | [Definition](#user-term-risk-register) |
| Saved Change | [Definition](#user-term-saved-change) |
| Scenario | [Definition](#user-term-scenario) |
| Schedule Audit in Combined Reports | [Topic](#documents-and-combined-reports) |
| Schedule Import and Export | [Topic](#schedule-import-and-export) |
| Schedule Performance Index (SPI) | [Definition](#user-term-schedule-performance-index-spi) |
| Schedule Quality Audit | [Topic](#charts) |
| Scheduling Constraint | [Definition](#user-term-scheduling-constraint) |
| Scheduling Constraints | [Topic](#scheduling-constraints) |
| Scrum | [Definition](#user-term-scrum) |
| Scrumban | [Definition](#user-term-scrumban) |
| Sigma Level | [Definition](#user-term-sigma-level) |
| SIPOC | [Definition](#user-term-sipoc) |
| SNET | [Definition](#user-term-scheduling-constraint) |
| SNLT | [Definition](#user-term-scheduling-constraint) |
| Solution Matrix | [Definition](#user-term-solution-matrix) |
| SPI | [Definition](#user-term-schedule-performance-index-spi) |
| Sprint | [Definition](#user-term-sprint) |
| SQLCipher | [Definition](#user-term-sqlcipher) |
| Stakeholder | [Definition](#user-term-stakeholder) |
| Stakeholders, Timeline, Budget, and Cost Control | [Topic](#stakeholders-timeline-budget-and-cost-control) |
| Story (Scrum) | [Definition](#user-term-story-scrum) |
| Story Points | [Definition](#user-term-story-points) |
| SWOT Matrix | [Definition](#user-term-swot-matrix) |
| System Database | [Definition](#user-term-system-database) |
| Throughput | [Definition](#user-term-throughput) |
| Timeline | [Definition](#user-term-timeline) |
| Tornado Ranking | [Definition](#user-term-tornado-ranking) |
| User Story | [Definition](#user-term-user-story) |
| Velocity | [Definition](#user-term-velocity) |
| VoC | [Definition](#user-term-voc) |
| WBS | [Definition](#user-term-wbs) |
| WIP Limit | [Definition](#user-term-wip-limit) |
| Work Breakdown Structure | [Definition](#user-term-wbs) |
| Work Item | [Definition](#user-term-work-item) |
| Work Package | [Definition](#user-term-work-package) |

## Glossary

Definitions distinguish related concepts; they do not certify that a workflow,
format or platform has been qualified. Follow the relevant instructions and
[implementation status](STATUS.md) for current availability. Shared user-reference
entries are maintained in `frontend/src/lib/components/help-glossary.ts`.

| Term | Meaning | Compare with |
| --- | --- | --- |
| <a id="user-term-accountability"></a>**Accountability** | Ultimate ownership of an outcome. In RACI, the "A" — one person owns the result even if others do the work. | [RACI Matrix](#user-term-raci-matrix) |
| <a id="user-term-actual-cost-ac"></a>**Actual Cost (AC)** | Cost recorded for work performed on the stated accounting basis. It is not the budgeted value of that work or necessarily cash already paid. | [Earned Value (EV)](#user-term-earned-value-ev), [Planned Value (PV)](#user-term-planned-value-pv) |
| <a id="user-term-administrator-escrow"></a>**Administrator Escrow** | A separately protected administrator-key mechanism for enrolled user data. The documented recorded, read-only access rules still apply; escrow is not a user password or an unrestricted entitlement for every administrator. | [Administrator-Key Recovery Code](#user-term-administrator-key-recovery-code), [Recovery Codes](#user-term-recovery-codes) |
| <a id="user-term-administrator-key-recovery-code"></a>**Administrator-Key Recovery Code** | A code for restoring the administrator key under the documented recovery conditions. It is distinct from one-time account recovery codes and does not replace required account data or backups. | [Recovery Codes](#user-term-recovery-codes), [Administrator Escrow](#user-term-administrator-escrow) |
| <a id="user-term-agile"></a>**Agile** | Umbrella term for iterative delivery methodologies valuing customer collaboration and responding to change. Scrum, Kanban, and Scrumban are Agile frameworks. | — |
| <a id="user-term-audit-trail"></a>**Audit Trail** | Recorded events and integrity evidence for project operations. An intact audit chain is not encryption and does not certify that every business fact is correct. | — |
| <a id="user-term-autosave"></a>**Autosave** | Saving changed editor state on the configured cycle. An edit waiting for or still undergoing autosave is not yet saved; autosave is not an independent backup. | [Saved Change](#user-term-saved-change), [Backup](#user-term-backup) |
| <a id="user-term-backlog"></a>**Backlog** | Ordered list of work items waiting to be pulled into active work. In Scrum: the Product Backlog. In Scrumban: maintained continuously. | — |
| <a id="user-term-backup"></a>**Backup** | A retained copy of project data used for restoration. A report export is not a complete project backup; recovery codes are not copies of project data. | [Export](#user-term-export), [Recovery Codes](#user-term-recovery-codes) |
| <a id="user-term-baseline"></a>**Baseline** | A saved reference plan used to compare later scope, dates or costs. Capturing a GoPMgr baseline does not itself prove formal approval; distinguish it from the editable current plan. | [Current Plan](#user-term-current-plan), [Forecast](#user-term-forecast) |
| <a id="user-term-belt-level"></a>**Belt Level** | Six Sigma practitioner certification: Green Belt (part-time, local projects), Black Belt (full-time, cross-functional), Master Black Belt (strategic deployment). | — |
| <a id="user-term-budget"></a>**Budget** | A planned amount or approved limit on the stated basis. Distinguish budget from committed costs, actual costs and a forecast; do not add amounts with overlapping coverage. | [Actual Cost (AC)](#user-term-actual-cost-ac), [Forecast](#user-term-forecast) |
| <a id="user-term-burn-down-chart"></a>**Burn-Down Chart** | Remaining work (y) over time (x) with an ideal trajectory. Used in Scrum to track sprint progress. | — |
| <a id="user-term-burn-up-chart"></a>**Burn-Up Chart** | Cumulative scope completed vs. total scope. Distinguishes scope growth from delivery progress. | — |
| <a id="user-term-business-case"></a>**Business Case** | Document justifying the project investment: costs, benefits, risks, NPV/ROI. Primary go/no-go input. | — |
| <a id="user-term-card-task-kanban"></a>**Card / Task (Kanban)** | A single unit of work on the Kanban Board. | — |
| <a id="user-term-change-failure-rate"></a>**Change Failure Rate** | DORA metric: the share of deployments that cause a failure in production. Computed on the DORA dashboard from the deployments you record. | — |
| <a id="user-term-change-request"></a>**Change Request** | Formal proposal to modify a project baseline (scope, schedule, cost, quality). Requires review and approval. | — |
| <a id="user-term-compliance-mode"></a>**Compliance Mode** | Project Settings option that verifies the tamper-evident audit chain before a project opens and blocks the open if the chain has been altered. | — |
| <a id="user-term-control-chart"></a>**Control Chart** | Time series with UCL and LCL. Points outside limits or non-random patterns indicate special-cause variation. | — |
| <a id="user-term-control-limits"></a>**Control Limits** | Statistical boundaries on a Control Chart (±3σ from the process mean). Not the same as specification limits. | — |
| <a id="user-term-cost-performance-index-cpi"></a>**Cost Performance Index (CPI)** | EV divided by AC on the stated cost/progress basis. Below one indicates cost inefficiency; a zero denominator makes the ratio undefined, not zero performance. | [Schedule Performance Index (SPI)](#user-term-schedule-performance-index-spi) |
| <a id="user-term-cpm"></a>**CPM** | Critical Path Method: calculates dependency-based start/finish dates and float from modeled durations and relationships. It is not resource leveling. | [Resource Leveling](#user-term-resource-leveling), [Critical Path](#user-term-critical-path) |
| <a id="user-term-critical-path"></a>**Critical Path** | Dependency-connected tasks controlling the calculated project finish in the modeled schedule. Interpret float and constraints separately; a zero-float description alone is insufficient for constrained schedules. | [Float (Slack)](#user-term-float-slack), [Resource Leveling](#user-term-resource-leveling) |
| <a id="user-term-ctq"></a>**CTQ** | Critical to Quality. Measurable characteristic directly affecting customer perception of quality. Derived from VoC in Six Sigma. | — |
| <a id="user-term-cumulative-flow-diagram"></a>**Cumulative Flow Diagram** | Stacked area chart of work items in each workflow state over time. Primary Kanban health indicator. | — |
| <a id="user-term-current-plan"></a>**Current Plan** | The editable plan reflecting present choices. A baseline retains a reference for comparison; editing the current plan does not mean the reference was formally reapproved. | [Baseline](#user-term-baseline) |
| <a id="user-term-cycle-time"></a>**Cycle Time** | Time from when a work item starts to when it is completed. Key delivery predictability metric in Kanban. | [Lead Time](#user-term-lead-time) |
| <a id="user-term-dek"></a>**DEK** | Data Encryption Key: the per-user key used for encrypted project databases. It is stored wrapped on disk and unlocked in memory; it is not your passphrase, a recovery code or a signing key. | [Passphrase](#user-term-passphrase), [Recovery Codes](#user-term-recovery-codes), [Administrator Escrow](#user-term-administrator-escrow) |
| <a id="user-term-deployment-frequency"></a>**Deployment Frequency** | DORA metric: how often the team ships to production. Recorded per deployment on the DORA dashboard. | — |
| <a id="user-term-digital-signature"></a>**Digital Signature** | Cryptographic evidence linking signed bytes to a signer under a validation policy. It is not encryption, archival conformance or a guarantee of factual correctness. | [Encryption at Rest](#user-term-encryption-at-rest), [PDF/A-2u](#user-term-pdf-a-2u) |
| <a id="user-term-dmaic"></a>**DMAIC** | Six Sigma improvement cycle: Define → Measure → Analyze → Improve → Control. For improving existing processes. | — |
| <a id="user-term-dora-metrics"></a>**DORA Metrics** | Four DevOps metrics: Deployment Frequency, Lead Time for Changes, Change Failure Rate, Mean Time to Restore. Available in GoPMgr's DORA dashboard. | — |
| <a id="user-term-earned-value-ev"></a>**Earned Value (EV)** | The budgeted value of completed work under the stated progress basis. It differs from planned value (PV) and actual cost (AC); EVM is the overall management method. | [Planned Value (PV)](#user-term-planned-value-pv), [Actual Cost (AC)](#user-term-actual-cost-ac), [Earned Value Management (EVM)](#user-term-earned-value-management-evm) |
| <a id="user-term-earned-value-management-evm"></a>**Earned Value Management (EVM)** | Earned Value Management compares planned value, earned value and actual cost. EVM names the method; EV is the budgeted value of completed work, not the money spent. | [Earned Value (EV)](#user-term-earned-value-ev), [Cost Performance Index (CPI)](#user-term-cost-performance-index-cpi), [Schedule Performance Index (SPI)](#user-term-schedule-performance-index-spi) |
| <a id="user-term-encryption-at-rest"></a>**Encryption at Rest** | Protection for stored data using a key, such as an encrypted project database. It does not automatically protect exported files, an unlocked session or copies stored elsewhere. | [Digital Signature](#user-term-digital-signature), [DEK](#user-term-dek) |
| <a id="user-term-epic"></a>**Epic** | A large user story spanning multiple sprints, broken down into smaller stories before entering a sprint. | — |
| <a id="user-term-es-ef"></a>**ES / EF** | Earliest Start / Earliest Finish. Calculated during the forward pass through a CPM network. | — |
| <a id="user-term-estimate-methodology-specific"></a>**Estimate (methodology-specific)** | What each methodology usually calls an estimate. Scrum: Story Points. Kanban: Time Estimate. CPM: Duration. Waterfall: Duration. Lean: Effort. PRINCE2: Work Package Estimate. Six Sigma: Resource Plan. In GoPMgr, a work item's estimate is labelled Story points in Scrum and Scrumban projects and Points in others; CPM and Gantt tasks are estimated as a duration in days. | — |
| <a id="user-term-export"></a>**Export** | An output file made for another tool, viewing or distribution. It need not retain all editable project data and is not a backup or evidence of formal document approval. | [Backup](#user-term-backup), [Saved Change](#user-term-saved-change) |
| <a id="user-term-fishbone-diagram"></a>**Fishbone Diagram** | Also Ishikawa Diagram. Root-cause analysis: effect at the "head"; default cause categories (People, Process, Equipment, Materials, Environment, Measurement) are the "bones." Categories are editable. | — |
| <a id="user-term-float-slack"></a>**Float (Slack)** | Amount of time a task can be delayed without delaying the project end date (total float) or a successor's start (free float). | [Critical Path](#user-term-critical-path) |
| <a id="user-term-forecast"></a>**Forecast** | An estimate of a future outcome based on a named method and assumptions. It is neither an actual observation nor the approved reference plan. | — |
| <a id="user-term-gantt-chart"></a>**Gantt Chart** | Schedule bars on a time axis with dependency arrows, critical-path highlighting, and progress bars. Shares data with CPM in GoPMgr. | — |
| <a id="user-term-hard-constraint"></a>**Hard Constraint** | A scheduling constraint that limits how late a task may run: start no later than (SNLT), finish no later than (FNLT), must start on (MSO) or must finish on (MFO). It hides what a delay does to that date; the schedule audit counts these under SQ-08. Start or finish no earlier than (SNET, FNET) are soft. | [Scheduling Constraint](#user-term-scheduling-constraint) |
| <a id="user-term-issue"></a>**Issue** | A problem or condition that has occurred and needs handling. A risk is an uncertain event; a change request proposes an alteration to the plan. | [Risk](#user-term-risk), [Change Request](#user-term-change-request) |
| <a id="user-term-issue-log"></a>**Issue Log** | Tracks problems that have already occurred, with owner, priority, and resolution plan. Distinct from Risk Register (potential future events). | [Risk Register](#user-term-risk-register) |
| <a id="user-term-iteration"></a>**Iteration** | A repeated work and review cycle, often time-boxed, such as a Scrum Sprint. A PRINCE2 management stage is a governance interval; continuous-flow methods need not use fixed iterations. | — |
| <a id="user-term-kaizen"></a>**Kaizen** | Lean concept of continuous, incremental improvement made frequently by everyone involved in the process. | — |
| <a id="user-term-kanban-board"></a>**Kanban Board** | Visual work management: columns = workflow states; cards = work items. WIP limits cap items per column. | — |
| <a id="user-term-key-result-kr"></a>**Key Result (KR)** | OKRs: measurable, time-bound outcome tracking progress toward an Objective. Graded 0.0–1.0 at period close. | — |
| <a id="user-term-launchpad"></a>**Launchpad** | GoPMgr's project creation wizard. Guides through industry, methodology, and project name selection; seeds starter artifacts. | — |
| <a id="user-term-lead-time"></a>**Lead Time** | Total time from request to delivery. Includes queue time + cycle time. | [Cycle Time](#user-term-cycle-time) |
| <a id="user-term-levelled-start"></a>**Levelled Start** | The start resource levelling gave a CPM task: a date, with a time of day when the task starts part-way through a day. It delays the task like start no earlier than (SNET) but is not a constraint: each levelling run recalculates or clears it, and you can clear it in the task panel. A constraint date it pushes the task past is flagged. MSPDI files carry it as a levelling delay. | [Resource Leveling](#user-term-resource-leveling), [Scheduling Constraint](#user-term-scheduling-constraint) |
| <a id="user-term-ls-lf"></a>**LS / LF** | Latest Start / Latest Finish. Calculated during the backward pass through a CPM network. | — |
| <a id="user-term-milestone"></a>**Milestone** | A significant zero-duration schedule event. A Definition of Done is a completion-quality standard; a throughput target is a delivery-rate goal, not a milestone. | — |
| <a id="user-term-monte-carlo-simulation"></a>**Monte Carlo Simulation** | Schedule-risk technique that samples uncertain task durations thousands of times to estimate finish-date probabilities. Each run applies the chart's constraints and levelled starts as the schedule does. Run from the CPM editor; results include P50/P80/P90 dates, an S-curve, and a tornado driver ranking. | — |
| <a id="user-term-mspdi"></a>**MSPDI** | Microsoft Project Data Interchange — the .xml schedule format GoPMgr imports and exports. Resave .mpp/.pod/.mpx files as MS Project XML before importing. | — |
| <a id="user-term-mttr"></a>**MTTR** | Mean Time to Restore. DORA metric: average hours to recover from a failed deployment. | — |
| <a id="user-term-objective"></a>**Objective** | OKRs: qualitative, inspiring goal statement answering "where do we want to go?" Supported by measurable Key Results. | — |
| <a id="user-term-okrs"></a>**OKRs** | Objectives and Key Results. Goal-setting framework aligning teams to strategic outcomes through measurable, time-bound Key Results. | — |
| <a id="user-term-p50-p80-p90"></a>**P50 / P80 / P90** | Confidence points from a Monte Carlo run: the finish day the project beats in 50%, 80%, or 90% of simulations. Commit to P80/P90 dates when the cost of being late is high. | — |
| <a id="user-term-pades"></a>**PAdES** | PDF Advanced Electronic Signatures: signatures used to validate integrity and signer identity under a certificate/trust policy. Signing is not encryption and does not prove the report calculations are correct. | [PDF/A-2u](#user-term-pdf-a-2u), [Digital Signature](#user-term-digital-signature) |
| <a id="user-term-pareto-chart"></a>**Pareto Chart** | Bar chart sorted descending with a cumulative-percentage line. Based on the 80/20 principle: ~80% of effects come from ~20% of causes. | — |
| <a id="user-term-passphrase"></a>**Passphrase** | The account password used to authenticate and unlock protected key material. It is not the DEK itself; resetting a password is not equivalent to restoring encrypted project access. | [DEK](#user-term-dek), [Recovery Codes](#user-term-recovery-codes) |
| <a id="user-term-pdf-a-2u"></a>**PDF/A-2u** | An archival PDF conformance target with Unicode text mapping. Conformance does not prove accessibility, correct content or indefinite preservation; it does not encrypt or digitally sign the file. | [PAdES](#user-term-pades), [Encryption at Rest](#user-term-encryption-at-rest) |
| <a id="user-term-pert"></a>**PERT** | Program Evaluation and Review Technique. Network scheduling with three-point duration estimates per activity. | — |
| <a id="user-term-planned-value-pv"></a>**Planned Value (PV)** | The budgeted value of work scheduled by the stated cutoff and baseline. It is not actual spending or the value of work already completed. | [Earned Value (EV)](#user-term-earned-value-ev), [Actual Cost (AC)](#user-term-actual-cost-ac) |
| <a id="user-term-planning-meeting"></a>**Planning Meeting** | Scrum: Sprint Planning. Kanban: Replenishment Meeting. Lean: Plan-Do-Check-Act review. PRINCE2: Stage Planning Meeting. | — |
| <a id="user-term-pmbok"></a>**PMBOK** | Project Management Body of Knowledge. PMI's comprehensive PM framework. A knowledge standard, not a delivery methodology. | — |
| <a id="user-term-prince2"></a>**PRINCE2** | Projects In Controlled Environments. Process-based framework with seven principles, themes, and processes. Strong governance and stage-gate controls. | — |
| <a id="user-term-process-group"></a>**Process Group** | PMBOK grouping of related PM processes: Initiating, Planning, Executing, Monitoring & Controlling, Closing. Corresponds to GoPMgr's document Phase categories. | — |
| <a id="user-term-project-charter"></a>**Project Charter** | Foundational document formally authorizing a project. Captures purpose, objectives, scope, sponsor, high-level schedule and budget. | — |
| <a id="user-term-project-database"></a>**Project Database** | The .gopmgr database containing one project and its records. It is distinct from system.db, which holds local account metadata. | [System Database](#user-term-system-database) |
| <a id="user-term-pull-system"></a>**Pull System** | Work produced when downstream capacity exists, not pushed by a schedule. Core Lean and Kanban principle. | — |
| <a id="user-term-raci-matrix"></a>**RACI Matrix** | Responsibility assignment: Responsible (does work), Accountable (owns outcome), Consulted (input needed), Informed (kept updated). One A per task. | — |
| <a id="user-term-recovery-codes"></a>**Recovery Codes** | One-time account codes for password reset; valid wrapped codes can also unlock the DEK. Codes may need reissuing after encryption migration. Lost credentials are not always recoverable, and these are not administrator-key recovery codes. | [DEK](#user-term-dek), [Administrator-Key Recovery Code](#user-term-administrator-key-recovery-code) |
| <a id="user-term-resource-capacity"></a>**Resource Capacity** | The available working time or quantity for a resource over a period and calendar. Assignment demand may exceed capacity; capacity is not the amount of work already completed. | — |
| <a id="user-term-resource-leveling"></a>**Resource Leveling** | Delaying contended tasks so resource demand fits capacity. Uses the named resource calendars (weekly capacity + day overrides) from Project Settings. Each delay is kept as the task's levelled start; constraints are never changed. | [CPM](#user-term-cpm), [Resource Capacity](#user-term-resource-capacity) |
| <a id="user-term-retrospective"></a>**Retrospective** | Process improvement ceremony. Scrum: Sprint Retrospective. Kanban: Retrospective. PRINCE2: End-Stage Assessment lessons. | — |
| <a id="user-term-risk"></a>**Risk** | An uncertain event or condition that may affect objectives. Once a problem occurs it is handled as an issue; documenting a risk does not by itself implement a response. | [Issue](#user-term-issue), [Risk Register](#user-term-risk-register) |
| <a id="user-term-risk-register"></a>**Risk Register** | Catalogue of potential future risks with probability, impact, owner, mitigation strategy, and contingency plan. | [Issue Log](#user-term-issue-log) |
| <a id="user-term-saved-change"></a>**Saved Change** | An edit whose save operation completed successfully. An in-flight or pending edit is not yet saved. Exporting a file and retaining a backup are separate actions. | [Autosave](#user-term-autosave), [Export](#user-term-export), [Backup](#user-term-backup) |
| <a id="user-term-scenario"></a>**Scenario** | An isolated what-if partition. Charts copied into a scenario can be edited and compared against their captured baseline without touching the live plan, then promoted to a named schedule baseline or discarded. | — |
| <a id="user-term-schedule-performance-index-spi"></a>**Schedule Performance Index (SPI)** | EV divided by PV on the stated baseline/progress basis. It compares budgeted work, not calendar finish-date slip; a zero denominator makes the ratio undefined. | [Critical Path](#user-term-critical-path), [Cost Performance Index (CPI)](#user-term-cost-performance-index-cpi) |
| <a id="user-term-scheduling-constraint"></a>**Scheduling Constraint** | A rule tying a CPM task to a date or its late dates: ASAP, ALAP, SNET, SNLT, FNET, FNLT, MSO or MFO. Links always win: a constraint the links cannot meet is flagged as violated, not forced. | [Hard Constraint](#user-term-hard-constraint), [Float (Slack)](#user-term-float-slack), [Critical Path](#user-term-critical-path) |
| <a id="user-term-scrum"></a>**Scrum** | Agile framework with time-boxed sprints, Product Backlog, and three roles. Four ceremonies: Sprint Planning, Daily Scrum, Sprint Review, Retrospective. | — |
| <a id="user-term-scrumban"></a>**Scrumban** | Hybrid of Scrum's prioritized backlog and Kanban's continuous pull flow. No fixed sprint cadence. | — |
| <a id="user-term-sigma-level"></a>**Sigma Level** | A measure of process performance in Six Sigma. A quoted 3.4 defects per million for six sigma assumes the conventional 1.5-sigma shift; it is not automatic proof of a process capability. | — |
| <a id="user-term-sipoc"></a>**SIPOC** | High-level process map: Suppliers → Inputs → Process → Outputs → Customers. Defines process boundaries in DMAIC Define phase. | — |
| <a id="user-term-solution-matrix"></a>**Solution Matrix** | Six Sigma Improve tool. Evaluates solutions on Impact, Effort, Risk, and Cost. Statuses: Proposed, Pilot, Implemented. | — |
| <a id="user-term-sprint"></a>**Sprint** | Scrum time-boxed iteration (typically 2 weeks). Contains a Sprint Goal, committed backlog items, Review, and Retrospective. | — |
| <a id="user-term-sqlcipher"></a>**SQLCipher** | Open-source encrypted SQLite extension used by GoPMgr for database encryption at rest. Applies AES-256 to the entire project database file. | — |
| <a id="user-term-stakeholder"></a>**Stakeholder** | Individual or group with an interest in or influence over the project. Mapped by power/interest in the Stakeholder Analysis Matrix. | — |
| <a id="user-term-story-scrum"></a>**Story (Scrum)** | User-facing work item: "As a [role], I want [feature] so that [benefit]." Equivalent to a Task in most other methodologies. | — |
| <a id="user-term-story-points"></a>**Story Points** | Relative effort estimate in Scrum. Team-defined scale (typically Fibonacci). Not correlated to hours. | — |
| <a id="user-term-swot-matrix"></a>**SWOT Matrix** | 2x2 strategic analysis: Strengths, Weaknesses (internal) vs. Opportunities, Threats (external). | — |
| <a id="user-term-system-database"></a>**System Database** | The local system.db account-metadata database. It is distinct from each project database; encrypting a project does not mean every application file or export is encrypted. | [Project Database](#user-term-project-database) |
| <a id="user-term-throughput"></a>**Throughput** | Work items completed per unit time. Primary Kanban flow metric alongside cycle time. | — |
| <a id="user-term-timeline"></a>**Timeline** | GoPMgr view showing the project's chronological event stream as an SVG strip: sprint bands, milestones, and holiday markers. | — |
| <a id="user-term-tornado-ranking"></a>**Tornado Ranking** | Monte Carlo output ranking which tasks drive schedule risk, combining how often each lands on the critical path with its P90−P50 duration spread. Fix the top bars first. | — |
| <a id="user-term-user-story"></a>**User Story** | See Story (Scrum). | — |
| <a id="user-term-velocity"></a>**Velocity** | Average story points completed per sprint. Used for capacity planning and release forecasting. | — |
| <a id="user-term-voc"></a>**VoC** | Voice of Customer. Customer needs captured via surveys, interviews, or complaints. Input to CTQ derivation in Six Sigma. | — |
| <a id="user-term-wbs"></a>**WBS** | Work Breakdown Structure. Hierarchical decomposition of total project scope into deliverables and work packages. | [Work Package](#user-term-work-package), [Work Item](#user-term-work-item) |
| <a id="user-term-wip-limit"></a>**WIP Limit** | Work-In-Progress Limit. Maximum items in a Kanban column simultaneously. Prevents overloading and surfaces bottlenecks. | — |
| <a id="user-term-work-item"></a>**Work Item** | An item managed in a work workflow, such as a task or story. Its workflow state or estimate is not automatically a connected schedule activity or WBS work package. | [Work Package](#user-term-work-package) |
| <a id="user-term-work-package"></a>**Work Package** | A manageable unit of project scope in a WBS. It differs from a scheduled activity, an assigned resource and a report describing that work. | [Work Item](#user-term-work-item), [WBS](#user-term-wbs) |

The Six Sigma shift convention is explained by [ASQ](https://asqasktheexperts.org/2016/10/07/six-sigma-statistical-meaning/).
