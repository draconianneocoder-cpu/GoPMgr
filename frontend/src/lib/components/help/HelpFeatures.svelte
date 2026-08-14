<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import type { SectionId } from '../help-sections';

  let { active, nav }: { active: SectionId; nav: (id: SectionId) => void } = $props();
</script>

        <!-- ── Portfolio ──────────────────────────────── -->
        {#if active === 'portfolio'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Portfolio Dashboard</h2>
          <p class="text-sm text-slate-400 mb-5">The first screen after login. Shows all projects belonging to the signed-in user on this machine, with status, methodology, and last-modified metadata.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Accessing</h3>
            <p class="text-sm text-slate-300">Click "Dashboard" in the top navigation bar, or choose File &rarr; Dashboard. The Portfolio is also the default view after signing in.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Project Cards</h3>
            <p class="text-sm text-slate-300 mb-2">Each project appears as a card showing:</p>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Project name and status badge (Active / Done)</li>
              <li>Industry and methodology</li>
              <li>Last modified date</li>
              <li>Click the card to open the project</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Filtering and Search</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Search bar</span> — filter by project name as you type.</li>
              <li><span class="font-medium text-slate-100">Status tabs</span> — All / Active / Done, with counts per tab.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Portfolio Analytics</h3>
            <p class="text-sm text-slate-300 mb-2">
              Production and installer builds include DuckDB-backed in-memory portfolio
              analytics. Choose <span class="font-medium text-slate-100">Run rollup</span>
              for budget, committed cost, Planned Value, Earned Value, Actual Cost, SPI,
              and CPI as of the displayed UTC reporting date.
            </p>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Committed cost is contracts + labour estimate; EVM Actual Cost comes from schedule tasks.</li>
              <li>Only anchored, valid, acyclic, costed current CPM schedules contribute to EVM. The coverage count and warning make exclusions visible.</li>
              <li>Portfolio SPI = ΣEV/ΣPV and CPI = ΣEV/ΣAC. GoPMgr sums exact minor units before dividing; it never averages project ratios.</li>
              <li>The adjacent import action previews local CSV/TSV, Parquet, and JSON data in memory.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Creating Projects</h3>
            <p class="text-sm text-slate-300">Click "New Project" (top right) to launch the
              <button onclick={() => nav('getting-started')} class="text-cyan-400 underline hover:text-cyan-300">Project Launchpad</button>.
              Projects cannot be deleted from the Portfolio. Project files remain on disk; data directories are never removed by the application.
            </p>
          </section>

        <!-- ── Project Dashboard ──────────────────────────────── -->
        {:else if active === 'project-dashboard'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Project Dashboard</h2>
          <p class="text-sm text-slate-400 mb-5">The central hub for a project — lists all charts and documents, provides direct access to methodology-specific views, and surfaces export and signing actions per artifact.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Charts Panel</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Lists all charts in the current project with kind badge and creation date.</li>
              <li>Click a chart name to open its editor.</li>
              <li>The chart-tool catalog searches all 22 types by name and use, and filters them by rendering engine. It opens automatically when the project has no charts and stays compact for returning users.</li>
              <li>Delete (two-click confirm) removes the chart permanently.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Documents Panel</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Lists all documents with kind badge, phase badge, and last-modified date.</li>
              <li>Click a document to open its structured editor.</li>
              <li>The controlled-document index searches all 25 templates by name and purpose, with lifecycle-phase filters. It opens automatically when the project has no documents and stays compact for returning users.</li>
              <li><span class="font-medium text-slate-100">Build combined report</span> opens report assembly separately because it combines existing documents rather than creating a document template.</li>
              <li><span class="font-medium text-slate-100">Export</span> — export the document in the configured format (PDF, DOCX, ODT, etc.). See
                <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">Export &amp; Digital Signing</button>.
              </li>
              <li><span class="font-medium text-slate-100">Sign &amp; Export</span> — export as a digitally signed PAdES PDF. Requires a certificate to be configured in Project Settings.</li>
              <li>Delete (two-click confirm) removes the document permanently.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Agile Views</h3>
            <p class="text-sm text-slate-300">For projects with Agile features enabled (Scrum, Kanban, Scrumban), the sidebar exposes:</p>
            <ul class="text-sm text-slate-300 space-y-1 ml-3 mt-2">
              <li><span class="font-medium text-slate-100">Kanban Board</span> — visual card-based workflow.</li>
              <li><span class="font-medium text-slate-100">Backlog</span> — ordered list of user stories with estimates.</li>
              <li><span class="font-medium text-slate-100">Sprints</span> — manage sprint containers and pull backlog items.</li>
              <li><span class="font-medium text-slate-100">DORA Metrics</span> — deployment frequency, lead time for changes, change failure rate, mean time to restore.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Other Project Views</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Timeline</span> — chronological event strip. See <button onclick={() => nav('timeline')} class="text-cyan-400 underline hover:text-cyan-300">Timeline</button>.</li>
              <li><span class="font-medium text-slate-100">Stakeholders</span> — project stakeholder registry. See <button onclick={() => nav('stakeholders')} class="text-cyan-400 underline hover:text-cyan-300">Stakeholder Manager</button>.</li>
              <li><span class="font-medium text-slate-100">Report Composer</span> — assemble multi-document reports. See <button onclick={() => nav('report-composer')} class="text-cyan-400 underline hover:text-cyan-300">Report Composer</button>.</li>
              <li><span class="font-medium text-slate-100">Project Settings</span> — edit project metadata, what-if scenarios, scenario chart copies and editor access, scenario comparison, baseline promotion, export settings, compliance-mode audit verification, database encryption, document fonts, and Resource Capacity calendars. The scenario editor also compares and promotes copied charts. See <button onclick={() => nav('encryption')} class="text-cyan-400 underline hover:text-cyan-300">Database Encryption</button> and <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">Export &amp; Signing</button>.</li>
            </ul>
          </section>

        <!-- ── Kanban, Sprints & DORA ──────────────────────────────── -->
        {:else if active === 'agile-boards'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Kanban, Sprints &amp; DORA</h2>
          <p class="text-sm text-slate-400 mb-5">
            The agile workspace: a drag-and-drop Kanban board, a prioritised backlog, sprint
            management, and a DORA delivery-metrics dashboard. All four appear in the project
            sidebar when your methodology enables the agile pack (Scrum, Kanban, Scrumban).
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Kanban board</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Columns run left to right; each card is a work item. <span class="font-medium text-slate-100">Drag a card between columns</span> to change its state — the move is saved immediately.</li>
              <li>Each column header shows a <span class="font-medium text-slate-100">WIP indicator</span> (count / limit). When a column exceeds its work-in-progress limit the badge changes tone — your cue to finish before starting.</li>
              <li>Click a card to edit its title, description, points, priority, and assignee; use the add button to create a card in the first column.</li>
              <li>Card edges are tinted by priority so the board reads at a glance.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Backlog</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">Drag items up or down</span> to reorder priority; the order persists.</li>
              <li>Assign an item to a sprint from its row — it stays in the backlog until you <span class="font-medium text-slate-100">start work</span>, which moves it onto the board.</li>
              <li>Story points and priority are edited in the same work-item editor the board uses.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Sprints</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Create a sprint with a name, goal, start/end dates, and a capacity in story points.</li>
              <li>Sprints move <span class="font-medium text-slate-100">planning → active → complete</span>. Only one sprint is active at a time: clicking Start on a planning sprint automatically completes any other active sprint.</li>
              <li>Burn-Up and Burn-Down charts (created from the Dashboard) visualise sprint progress against capacity.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">DORA dashboard</h3>
            <p class="text-sm text-slate-300 mb-2">
              Four delivery KPIs with industry classification badges, a daily-deploy trend line,
              and a deployment log (most recent 50):
            </p>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">Deployment Frequency</span> — how often you ship.</li>
              <li><span class="font-medium text-slate-100">Lead Time for Changes</span> — commit-to-production hours.</li>
              <li><span class="font-medium text-slate-100">Change Failure Rate</span> — share of deployments causing a failure.</li>
              <li><span class="font-medium text-slate-100">Time to Restore (MTTR)</span> — hours to recover when one does.</li>
            </ul>
            <p class="text-sm text-slate-400 mt-2">
              Record each release with <span class="font-medium text-slate-100">+ Record deployment</span> —
              date, lead-time hours, whether it failed, and restore hours. The KPIs and badges
              recompute from what you log; there is no external integration to configure.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Related</h3>
            <p class="text-sm text-slate-300">
              Methodology guidance:
              <button onclick={() => nav('scrum')} class="text-cyan-400 underline hover:text-cyan-300">Scrum</button>,
              <button onclick={() => nav('kanban')} class="text-cyan-400 underline hover:text-cyan-300">Kanban</button>,
              <button onclick={() => nav('scrumban')} class="text-cyan-400 underline hover:text-cyan-300">Scrumban</button>.
              Progress charts: see the
              <button onclick={() => nav('charts')} class="text-cyan-400 underline hover:text-cyan-300">Charts reference</button>.
            </p>
          </section>

        <!-- ── Budget ──────────────────────────────── -->
        {:else if active === 'budget'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Budget</h2>
          <p class="text-sm text-slate-400 mb-5">
            A live rollup on the Project Dashboard comparing your budget cap against money you
            have effectively committed. No spreadsheet upkeep — it recomputes from data you
            already maintain.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">How the numbers are built</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">Budget</span> — the cap you set in <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>.</li>
              <li><span class="font-medium text-slate-100">Contracts</span> — the sum of contract values on vendor stakeholders.</li>
              <li><span class="font-medium text-slate-100">Labour estimate</span> — work-item points × the assignee's hourly rate from the <button onclick={() => nav('stakeholders')} class="text-cyan-400 underline hover:text-cyan-300">Stakeholder Manager</button>.</li>
              <li><span class="font-medium text-slate-100">Committed</span> = contracts + labour estimate; <span class="font-medium text-slate-100">Remaining</span> = budget − committed.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Reading the panel</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>The progress bar shows committed as a share of budget and turns <span class="text-red-400 font-medium">red past 100%</span>.</li>
              <li>A per-category breakdown appears when stakeholders carry categories, so you can see where the money concentrates.</li>
              <li>All arithmetic is integer cents — fractional labour estimates round exactly once, at the money boundary, so totals never drift.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Getting started</h3>
            <p class="text-sm text-slate-300">
              If the panel shows its empty state, set a budget in Project Settings and add
              stakeholder rates or contract values — it populates as soon as either side has data.
            </p>
          </section>

        <!-- ── Timeline ──────────────────────────────── -->
        {:else if active === 'timeline'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Timeline</h2>
          <p class="text-sm text-slate-400 mb-5">A horizontal SVG strip showing the project's chronological event stream — sprints, milestones, charter dates, and public holidays — auto-scaled to the project's date range.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Accessing</h3>
            <p class="text-sm text-slate-300">Open a project and select Timeline from the project sidebar.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">What It Shows</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Sprint bands</span> — each sprint appears as a colored horizontal band spanning its start and end date.</li>
              <li><span class="font-medium text-slate-100">Point events</span> (milestones, charter dates, deadlines) — rendered as vertical ticks with labels above/below the strip, alternated to reduce overlap.</li>
              <li><span class="font-medium text-slate-100">Holiday markers</span> — public holidays are shown as markers on the timeline strip.</li>
              <li><span class="font-medium text-slate-100">Auto-scaling</span> — the x-axis scales from the earliest to the latest event in the project automatically.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Interacting</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Drag events</span> — drag a sprint range or milestone to reschedule it. Changes are saved to the project.</li>
              <li><span class="font-medium text-slate-100">Export</span> — export the timeline as an image for inclusion in presentations or reports.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Data Sources</h3>
            <p class="text-sm text-slate-300">The timeline aggregates events from the project's Sprints (if agile), Charter dates (start/end), and any milestone events in charts. The country setting in Project Settings determines which public holiday calendar is used.</p>
          </section>

        <!-- ── Stakeholder Manager ──────────────────────────────── -->
        {:else if active === 'stakeholders'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Stakeholder Manager</h2>
          <p class="text-sm text-slate-400 mb-5">The project-level stakeholder address book. Stores contact details, role, category, financial rates, availability, and notes per stakeholder. Budget rollup reads hourly rates and contract values from this register, while resource leveling reads stakeholder availability.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Accessing</h3>
            <p class="text-sm text-slate-300">Open a project and select Stakeholders from the project sidebar. This view complements the Stakeholder Analysis Matrix chart (power/interest grid) — the Manager is the detailed registry; the chart is the strategic visual.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Stakeholder Fields</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Name</dt><dd class="text-slate-400">Full name of the individual or organization.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Role</dt><dd class="text-slate-400">Project role or job title.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Organisation</dt><dd class="text-slate-400">Company or department.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Email / Phone</dt><dd class="text-slate-400">Contact details.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Category</dt><dd class="text-slate-400">Team, Vendor, Sponsor, or External.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Hourly Rate</dt><dd class="text-slate-400">Used in budget cost rollup calculations. GoPMgr stores money internally as integer minor units and rounds once at the money boundary.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Contract Value</dt><dd class="text-slate-400">For Vendor entries; summed in budget rollup using exact-cent minor-unit totals.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Availability</dt><dd class="text-slate-400">Resource capacity in units (1.0 = full time). Named Resource Capacity calendars in Project Settings can add weekly capacity and day overrides.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Notes</dt><dd class="text-slate-400">Engagement strategy, concerns, communication preferences.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Resource Capacity</h3>
            <p class="text-sm text-slate-300">
              Open Project Settings to add named resource calendars. Each calendar can
              set a default capacity, weekly capacity and day overrides, optional skill
              tags, and notes. CPM resource leveling and over-allocation warnings use
              stakeholder availability plus these calendars.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Filtering</h3>
            <p class="text-sm text-slate-300">Use the category filter (All / Team / Vendor / Sponsor / External) to narrow the list. Useful for reviewing engagement strategies for a specific group.</p>
          </section>

        <!-- ── Project Settings ──────────────────────────────── -->
        {:else if active === 'project-settings'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Project Settings</h2>
          <p class="text-sm text-slate-400 mb-5">
            Everything that belongs to one project (File &rarr; Project Settings while a project
            is open). Not to be confused with
            <button onclick={() => nav('app-settings')} class="text-cyan-400 underline hover:text-cyan-300">App Settings</button>,
            which holds your personal, cross-project preferences.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Classification &amp; basics</h3>
            <p class="text-sm text-slate-300">
              Name, description, owner, industry, sub-category, methodology, country code,
              lifecycle status, phase, start/end dates, and the budget cap. The classification
              fields drive terminology, Launchpad rules, and the country's holiday calendar on the
              <button onclick={() => nav('timeline')} class="text-cyan-400 underline hover:text-cyan-300">Timeline</button>.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Resource capacity calendars</h3>
            <p class="text-sm text-slate-300">
              Named calendars with weekly capacity and per-day overrides. CPM tasks reference them
              via resource assignments (units, optional calendar label, max-unit caps, skill tags);
              resource leveling uses the calendars to delay contended tasks, and CPM/Gantt show
              over-allocation badges against them once the project has a start date.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Export, signing &amp; fonts</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Default document signing method and certificate — see <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">Export &amp; Digital Signing</button>.</li>
              <li>Schedule report exports and MS Project interchange — see <button onclick={() => nav('import-export')} class="text-cyan-400 underline hover:text-cyan-300">Schedule Import &amp; Export</button>.</li>
              <li>Document font selection, including importing a .ttf for this project's PDFs.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Scenarios, compliance &amp; encryption</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>What-if scenarios are created and managed here — see <button onclick={() => nav('scenarios')} class="text-cyan-400 underline hover:text-cyan-300">Scenarios &amp; What-If</button>.</li>
              <li><span class="font-medium text-slate-100">Compliance mode</span> verifies the tamper-evident audit chain before the project opens and blocks it if the chain was altered. Export a JSON verification report — or repair evidence before any manual fix — from this panel.</li>
              <li>Eligible plaintext project databases can be migrated to encrypted storage — see <button onclick={() => nav('encryption')} class="text-cyan-400 underline hover:text-cyan-300">Database Encryption</button>.</li>
            </ul>
          </section>

        <!-- ── Scenarios & What-If ──────────────────────────────── -->
        {:else if active === 'scenarios'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Scenarios &amp; What-If</h2>
          <p class="text-sm text-slate-400 mb-5">
            Test schedule alternatives without touching the real plan. A scenario is an isolated
            partition: charts copied into it can be edited freely and thrown away — or promoted
            if the experiment wins.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>In <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>, create a scenario and select it as active.</li>
              <li>Copy a source chart into it — either with its <span class="font-medium text-slate-100">current data</span> or from a <span class="font-medium text-slate-100">saved schedule baseline</span>.</li>
              <li>Open the copy in the dedicated scenario editor and experiment: change durations, dependencies, whatever the question needs.</li>
              <li>Compare the edited scenario against its captured baseline data side by side.</li>
              <li>If the alternative is better, <span class="font-medium text-slate-100">promote it back to a named schedule baseline</span> from the editor; otherwise delete the scenario and nothing else changed.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Good to know</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Scenario copies never overwrite the source chart — promotion creates a baseline; it does not silently replace your working schedule.</li>
              <li>Scenario lifecycle actions are recorded in the tamper-evident audit chain, so compliance mode can account for them.</li>
              <li>For probabilistic (rather than structural) what-ifs, the CPM editor's Monte Carlo panel answers "how likely is this date" — see the <button onclick={() => nav('charts')} class="text-cyan-400 underline hover:text-cyan-300">Charts reference</button>.</li>
            </ul>
          </section>

        <!-- ── Schedule Import & Export ──────────────────────────────── -->
        {:else if active === 'import-export'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Schedule Import &amp; Export</h2>
          <p class="text-sm text-slate-400 mb-5">
            Move schedules between GoPMgr and other tools — Microsoft Project in, reports and
            interchange formats out.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Importing from Microsoft Project</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>On the Project Dashboard, choose the <span class="font-medium text-slate-100">Import MS Project XML</span> action.</li>
			  <li>Choose which dependencies, progress, and resource assignments to retain, then pick an MSPDI <span class="font-mono text-xs">.xml</span> file. GoPMgr stores a mapping receipt with the imported schedule chart.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              Binary or legacy formats (<span class="font-mono text-xs">.mpp</span>,
              <span class="font-mono text-xs">.pod</span>, <span class="font-mono text-xs">.mpx</span>)
              are not read directly — resave them as Microsoft Project XML in the source
              application first.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Exporting the schedule</h3>
            <p class="text-sm text-slate-300 mb-3">
              From <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>, export the current schedule report as:
            </p>
            <div class="overflow-x-auto">
              <table class="w-full text-sm border-collapse">
                <thead>
                  <tr class="border-b border-slate-700">
                    <th class="text-left py-1.5 pr-4 font-semibold text-slate-300">Format</th>
                    <th class="text-left py-1.5 font-semibold text-slate-300">Use</th>
                  </tr>
                </thead>
                <tbody class="text-slate-300">
                  {#each [
                    ['PDF / DOCX / ODT', 'Reports for reading and sign-off'],
                    ['MS Project XML (.xml)', 'Round-trip interchange with MS Project and compatible tools'],
                    ['CSV', 'Spreadsheet task lists'],
                    ['HTML', 'Browser viewing or publishing'],
                  ] as [fmt, use]}
                    <tr class="border-b border-slate-800">
                      <td class="py-1.5 pr-4 whitespace-nowrap font-medium text-slate-100">{fmt}</td>
                      <td class="py-1.5">{use}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
            <p class="text-sm text-slate-400 mt-3">
              Individual documents and charts export from the Dashboard with optional
              <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">digital signing</button>;
              the same schedule report is also available headless via the command line.
            </p>
          </section>

        <!-- ── Report Composer ──────────────────────────────── -->
        {:else if active === 'report-composer'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Report Composer</h2>
          <p class="text-sm text-slate-400 mb-5">Assemble multiple project documents into a single composite PDF — a "Project Plan pack," "Status pack," executive briefing, or any other multi-document report.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Accessing</h3>
            <p class="text-sm text-slate-300">Open a project and select Report Composer from the project sidebar or Documents panel.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Building a Report</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Set the report title and subtitle</span> — these appear on the cover page of the exported PDF.</li>
              <li><span class="font-medium text-slate-100">Pick documents.</span> All documents in the project are listed. Click a document to add it to the report.</li>
              <li><span class="font-medium text-slate-100">Reorder sections.</span> Drag documents up or down in the included list, or use the arrow buttons, to set the desired output order.</li>
              <li><span class="font-medium text-slate-100">Export.</span> Click Export PDF to generate the composite document. Each included document becomes a section in the output, and Status Reports with a linked CPM schedule include Earned Value when cost and progress data are available.</li>
              <li><span class="font-medium text-slate-100">Sign &amp; Export.</span> Optionally apply a PAdES digital signature to the entire composite PDF.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Tips</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Only documents belonging to the current project are available. Cross-project reports require exporting documents individually.</li>
              <li>The export uses the project's configured export theme (Modern, Classic, Archival) from Project Settings.</li>
              <li>For digital signing, configure a certificate in Project Settings before opening the Report Composer.</li>
            </ul>
          </section>

        <!-- ── Export & Digital Signing ──────────────────────────────── -->
        {:else if active === 'export-signing'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Export &amp; Digital Signing</h2>
          <p class="text-sm text-slate-400 mb-5">GoPMgr exports documents in multiple formats and supports PAdES-compliant digital signatures using a personal certificate (.p12/.pfx).</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Export Formats</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-16 shrink-0">PDF</dt><dd class="text-slate-400">Print-ready PDF. Supports optional PAdES digital signature.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-16 shrink-0">DOCX</dt><dd class="text-slate-400">Microsoft Word format. Compatible with Word 2013 and later.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-16 shrink-0">ODT</dt><dd class="text-slate-400">OpenDocument Text. Compatible with LibreOffice and Google Docs.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-16 shrink-0">CSV</dt><dd class="text-slate-400">Comma-separated values; available for tabular documents (register types).</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-16 shrink-0">HTML</dt><dd class="text-slate-400">Web-ready HTML output.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-16 shrink-0">MSPDI</dt><dd class="text-slate-400">Microsoft Project Data Interchange XML; for schedule documents.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Export Themes</h3>
            <p class="text-sm text-slate-300 mb-2">All exports apply a visual theme to headings, tables, and page layout:</p>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-24 shrink-0">Modern</dt><dd class="text-slate-400">Default. Clean contemporary styling.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-24 shrink-0">Classic</dt><dd class="text-slate-400">Traditional formal document styling.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-24 shrink-0">Archival</dt><dd class="text-slate-400">High-contrast black-and-white for long-term archival printing.</dd></div>
            </dl>
            <p class="text-xs text-slate-500 mt-2">Set the default export theme in Project Settings or App Settings. Project Settings overrides the app default for that project.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Exporting a Document</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li>Open the project and go to the Project Dashboard.</li>
              <li>In the Documents panel, find the document and click "Export."</li>
              <li>The file is written to your user exports directory and the path is shown in a toast notification.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Digital Signing (PAdES)</h3>
            <p class="text-sm text-slate-300 mb-2">PAdES (PDF Advanced Electronic Signatures) embeds a cryptographic signature into the PDF that verifies the document has not been modified after signing.</p>
            <p class="text-sm font-medium text-slate-200 mb-2">Configure a certificate:</p>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside mb-3">
              <li>Open Project Settings (File &rarr; Project Settings or the gear icon in the project sidebar).</li>
              <li>In "Export &amp; Signature Settings," select PAdES and browse to your .p12 or .pfx certificate file.</li>
              <li>To create PAdES-T exports, enable RFC 3161 timestamping and enter a credential-free HTTPS timestamp-authority endpoint. An optional policy OID and PEM trust root narrow the TSA policy and enable chain-trust verification.</li>
              <li>Save settings. The certificate path is stored; the password is never persisted.</li>
            </ol>
            <p class="text-sm font-medium text-slate-200 mb-2">Sign a document:</p>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li>From the Project Dashboard, click "Sign &amp; Export" on the document you want to sign.</li>
              <li>The Sign dialog shows the configured certificate path. You can choose a different certificate if needed.</li>
              <li>Enter the certificate password (used only for this operation — never stored).</li>
              <li>Click "Sign &amp; Export." The signed PAdES PDF is written to your exports directory.</li>
            </ol>
            <p class="text-xs text-slate-400 mt-3">
              Timestamping is fail-closed. If the configured authority is
              unavailable or returns an invalid token, GoPMgr writes no signed
              PDF and does not silently fall back to Baseline B. Without a TSA
              trust root, token integrity is validated while chain trust is
              reported as not evaluated.
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Audit Verification Reports</h3>
            <p class="text-sm text-slate-300">
              Project Settings can export a JSON audit verification report for
              the open project. The report records whether the tamper-evident
              audit chain is valid, how many events were checked, the terminal
              event hash, and first-invalid-event details if verification fails.
              Project, chart, document, schedule-baseline, scenario,
              scenario-chart copy, document approval, scenario-promotion
              approval, document signature, and signed combined-report actions
              are included in the chain.
              If a chain is damaged, export audit repair evidence before manual
              repair work; that artifact preserves the raw audit events and the
              verification result separately.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Export File Location</h3>
            <p class="text-sm text-slate-300">Exported files are saved under your GoPMgr data folder in <span class="font-mono text-xs bg-slate-900 px-1 rounded">&lt;username&gt;/exports/</span>. The full path is shown in a success toast after every export, and App Settings shows the parent data directory. Supported legacy PMForge data is copied into the current location on first launch. Use App Settings &rarr; Open Logs Folder to open diagnostic files when needed.</p>
          </section>

        <!-- ── Database Encryption ──────────────────────────────── -->
        {:else if active === 'encryption'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Database Encryption</h2>
          <p class="text-sm text-slate-400 mb-5">Each project stores its data in a SQLite database. GoPMgr can encrypt this database at rest using SQLCipher, which applies AES-256 encryption to the entire database file. This protects project data if the machine is lost or the filesystem is accessed directly.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Prerequisites</h3>
            <p class="text-sm text-slate-300 mb-2">Before enabling encryption, you must generate recovery codes for your account:</p>
            <ol class="space-y-1 text-sm text-slate-300 list-decimal list-inside">
              <li>Go to App Settings (top nav) and find the Recovery Codes section.</li>
              <li>Generate a new set of recovery codes. Store them securely (password manager, safe, printed).</li>
              <li>Recovery codes must be current — if you have old codes from before, reissue them. GoPMgr enforces this before allowing encryption to proceed.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Encrypting a Project</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li>Open the project you want to encrypt. The project must be opened from the project list (not just selected).</li>
              <li>Go to Project Settings (File &rarr; Project Settings, or the gear icon).</li>
              <li>Find the "Database Encryption" section. It shows the current state: Plaintext or Encrypted.</li>
              <li>Click "Encrypt Database." GoPMgr creates a backup of the plaintext database first and shows the backup path.</li>
              <li>After encryption completes, the state badge changes to "Encrypted."</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">What Is and Is Not Encrypted</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Encrypted:</span> the project database file (charts, documents, stakeholders, sprints, backlog items).</li>
              <li><span class="font-medium text-slate-100">Not encrypted:</span> the system database (user accounts, password hashes). Passwords are hashed with Argon2id. File attachments and exports stored outside the database are also not encrypted by this feature.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recovering Access</h3>
            <p class="text-sm text-slate-300">If you lose your passphrase, use a recovery code from the login screen to reset it. When you issue recovery codes, each code carries a wrapped copy of your Data Encryption Key (DEK). A passphrase reset via recovery code unwraps the DEK from the code and re-wraps it under the new passphrase — encrypted projects remain accessible. Legacy recovery codes issued before encryption was enabled do not carry a DEK wrap; that is why current recovery codes are required before enabling encryption.</p>
          </section>

        <!-- ── Backups & Data Safety ──────────────────────────────── -->
        {:else if active === 'backups'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Backups &amp; Data Safety</h2>
          <p class="text-sm text-slate-400 mb-5">
            GoPMgr is local-first: your data lives in ordinary files you can copy, so backup is
            simple — but it is <span class="font-medium text-slate-100">your</span> job. There is
            no cloud copy.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">What to back up</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Everything lives under your GoPMgr data folder: <span class="font-mono text-xs">system.db</span> (accounts) plus a private per-user folder with <span class="font-mono text-xs">projects</span>, <span class="font-mono text-xs">certs</span>, <span class="font-mono text-xs">exports</span>, and <span class="font-mono text-xs">logs</span>.</li>
              <li>Copying that folder while GoPMgr is closed is a complete backup. Encrypted projects stay encrypted in the copy — safe to store anywhere you trust with ciphertext.</li>
              <li>Keep your <span class="font-medium text-slate-100">recovery codes</span> with the backup: for encrypted projects, a restored file is only usable with your passphrase or a valid recovery code. Without both, it is unrecoverable by design.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Automatic safety nets</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>When you migrate a plaintext project to encrypted storage, GoPMgr <span class="font-medium text-slate-100">retains a pre-migration backup</span> and shows you its path — keep it until you have verified the encrypted project opens.</li>
              <li>Editors auto-save on an interval (App Settings) and show the last save time, so a crash costs at most the interval.</li>
              <li>Every export is written to your private <span class="font-mono text-xs">exports</span> folder with owner-only permissions.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Health checks &amp; repair</h3>
            <p class="text-sm text-slate-300">
              The GoPMgr binary doubles as a maintenance CLI for any project
              file (<span class="font-mono text-xs">.gopmgr</span>, or an
              older <span class="font-mono text-xs">.pmforge</span> project you haven't reopened yet):
              <span class="font-mono text-xs">--check</span> (integrity),
              <span class="font-mono text-xs">--repair</span> (self-healing),
              <span class="font-mono text-xs">--vacuum</span> (compaction), and
              <span class="font-mono text-xs">--export-audit</span> (audit log to CSV).
              For encrypted projects add <span class="font-mono text-xs">--username</span> and
              <span class="font-mono text-xs">--password-env</span> — the password comes from an
              environment variable, never the command line. See
              <button onclick={() => nav('install')} class="text-cyan-400 underline hover:text-cyan-300">Installing &amp; Running</button>
              for where the binary lives.
            </p>
          </section>

        <!-- ── Admin Panel ──────────────────────────────── -->
        {:else if active === 'admin-panel'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Admin Panel</h2>
          <p class="text-sm text-slate-400 mb-5">Administrator-only view for managing all GoPMgr user accounts on this machine. Accessible to accounts with the Admin role.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Accessing</h3>
            <p class="text-sm text-slate-300">Click "Admin" in the top navigation bar (visible only when signed in as an administrator). Also accessible via the Admin nav link added automatically for admin accounts.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">User List</h3>
            <p class="text-sm text-slate-300">Displays all accounts with: username, display name, role badge (Admin / Standard), and last login date. The signed-in user is marked "(you)" and cannot be edited from this list.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Creating an Account</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li>Click "Create user" to expand the creation form.</li>
              <li>Enter a username (3-32 characters, letters/digits/underscore/hyphen only).</li>
              <li>Enter a display name (optional; defaults to username).</li>
              <li>Set an initial password (minimum 8 characters). Share it securely — the user should change it after first login.</li>
              <li>Optionally check "Administrator account" to grant admin role immediately.</li>
              <li>Click "Create account."</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Role Management</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Click "Grant admin" / "Remove admin" next to a user to change their role. A confirmation step prevents accidental changes.</li>
              <li>The system enforces at least one administrator at all times — demoting the last admin is blocked.</li>
              <li>Administrators cannot change their own role from the Admin Panel.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Deleting Accounts</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Click "Delete" then "Confirm" to permanently remove an account from the system database.</li>
              <li>The user's data directory (projects, exports, certificates) is <span class="font-medium text-slate-100">not deleted</span> — project files remain on disk.</li>
              <li>Deleting the last admin account is blocked.</li>
              <li>Admins cannot delete their own account.</li>
            </ul>
          </section>

        <!-- ── App Settings ──────────────────────────────── -->
        {:else if active === 'app-settings'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">App Settings</h2>
          <p class="text-sm text-slate-400 mb-5">Per-user application preferences that apply across all projects. Distinct from Project Settings, which are per-project.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Accessing</h3>
            <p class="text-sm text-slate-300">Click "App Settings" in the top navigation bar, or choose File &rarr; Application Settings…</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Appearance</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Application Theme</dt><dd class="text-slate-400">Dark or Light. Preview applies immediately; save to persist.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Auto-Save</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Enable Auto-save</dt><dd class="text-slate-400">Toggle automatic saving of open editors. Editors also save manually with Cmd+S / Ctrl+S.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Auto-save Interval</dt><dd class="text-slate-400">15 seconds, 30 seconds, 1 minute (default), 2 minutes, or 5 minutes. Only writes when there are unsaved changes.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Defaults for New Projects</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Default Font</dt><dd class="text-slate-400">Font applied to newly created project documents. Per-project override available in Project Settings.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Export Theme</dt><dd class="text-slate-400">Modern (default), Classic, or Archival. Applied to document exports. Per-project override available.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Account Info</h3>
            <p class="text-sm text-slate-300">Shows your current version, signed-in username, and the resolved data directory location on disk.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Diagnostics</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Open Logs Folder</dt><dd class="text-slate-400">Opens the GoPMgr log directory in Finder/Explorer for troubleshooting.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Generate Bug Report</dt><dd class="text-slate-400">Creates a diagnostic report file in your data directory. Include this when reporting issues.</dd></div>
            </dl>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Become Administrator</h3>
            <p class="text-sm text-slate-300">If no administrator exists on the machine and your account is not an admin, a warning panel appears with a "Become administrator" button. This claims the admin role and cannot be undone via the UI (requires another admin to demote you). This option is only shown when the machine has no admin at all.</p>
          </section>

        {/if}
