<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import AppHeader from './AppHeader.svelte';

  type SectionId =
    | 'getting-started'
    | 'quick-start'
    | 'industry-matrix'
    | 'scrum'
    | 'kanban'
    | 'scrumban'
    | 'lean'
    | 'okrs'
    | 'waterfall'
    | 'prince2'
    | 'pmbok'
    | 'cpm'
    | 'six-sigma-method'
    | 'portfolio'
    | 'project-dashboard'
    | 'agile-boards'
    | 'budget'
    | 'timeline'
    | 'stakeholders'
    | 'project-settings'
    | 'scenarios'
    | 'import-export'
    | 'report-composer'
    | 'export-signing'
    | 'encryption'
    | 'backups'
    | 'admin-panel'
    | 'app-settings'
    | 'charts'
    | 'documents'
    | 'sigma-pack'
    | 'shortcuts'
    | 'glossary'
    | 'install';

  let active = $state<SectionId>('getting-started');

  const sidebar: { group: string; items: { id: SectionId; label: string }[] }[] = [
    {
      group: 'Overview',
      items: [
        { id: 'getting-started', label: 'Getting Started' },
        { id: 'industry-matrix', label: 'Industry & Methodology Matrix' },
      ],
    },
    {
      group: 'Tutorials',
      items: [{ id: 'quick-start', label: 'Quick Start: Your First Project' }],
    },
    {
      group: 'Methodologies',
      items: [
        { id: 'scrum', label: 'Scrum' },
        { id: 'kanban', label: 'Kanban' },
        { id: 'scrumban', label: 'Scrumban' },
        { id: 'lean', label: 'Lean' },
        { id: 'okrs', label: 'OKRs' },
        { id: 'waterfall', label: 'Waterfall' },
        { id: 'prince2', label: 'PRINCE2' },
        { id: 'pmbok', label: 'PMBOK' },
        { id: 'cpm', label: 'Critical Path (CPM)' },
        { id: 'six-sigma-method', label: 'Six Sigma' },
      ],
    },
    {
      group: 'Features',
      items: [
        { id: 'portfolio', label: 'Portfolio' },
        { id: 'project-dashboard', label: 'Project Dashboard' },
        { id: 'agile-boards', label: 'Kanban, Sprints & DORA' },
        { id: 'budget', label: 'Budget' },
        { id: 'timeline', label: 'Timeline' },
        { id: 'stakeholders', label: 'Stakeholder Manager' },
        { id: 'project-settings', label: 'Project Settings' },
        { id: 'scenarios', label: 'Scenarios & What-If' },
        { id: 'import-export', label: 'Schedule Import & Export' },
        { id: 'report-composer', label: 'Report Composer' },
        { id: 'export-signing', label: 'Export & Digital Signing' },
        { id: 'encryption', label: 'Database Encryption' },
        { id: 'backups', label: 'Backups & Data Safety' },
        { id: 'admin-panel', label: 'Admin Panel' },
        { id: 'app-settings', label: 'App Settings' },
      ],
    },
    {
      group: 'Reference',
      items: [
        { id: 'charts', label: 'Charts' },
        { id: 'documents', label: 'Documents' },
        { id: 'sigma-pack', label: 'DMAIC Pack' },
        { id: 'shortcuts', label: 'Keyboard Shortcuts & Accessibility' },
        { id: 'glossary', label: 'Glossary' },
        { id: 'install', label: 'Installing & Running' },
      ],
    },
  ];

  function nav(id: SectionId) {
    active = id;
  }

  // ── Search ────────────────────────────────────────────────────────
  // Only the active section's body is rendered, so the sidebar search
  // matches against this hand-curated keyword index plus the section
  // labels/groups. Keep entries lowercase; extend when adding sections.
  const KEYWORDS: Record<SectionId, string> = {
    'getting-started':
      'first launch create account admin administrator passphrase password login sign in users data directory navigation menu recovery codes new project launchpad',
    'quick-start':
      'tutorial beginner walkthrough first project step by step example schedule task dependency export pdf report onboarding start here how to begin',
    'industry-matrix':
      'seeded artifacts starter templates software construction engineering business administration custom combination launchpad',
    scrum: 'sprint backlog product owner scrum master velocity retrospective standup agile ceremonies story points',
    kanban: 'board columns wip limit work in progress flow pull continuous cards lanes cycle time',
    scrumban: 'hybrid sprint flow wip planning trigger bucket agile mix',
    lean: 'waste value stream pull kaizen continuous improvement muda flow efficiency',
    okrs: 'objectives key results goals alignment scoring quarterly cadence outcomes',
    waterfall: 'phases sequential requirements design implementation verification maintenance gantt milestones',
    prince2: 'stages governance business case project board tolerance exception work packages themes processes',
    pmbok: 'pmi process groups knowledge areas initiating planning executing monitoring closing pmp',
    cpm: 'critical path float slack forward pass backward pass network dependencies es ef ls lf duration schedule',
    'six-sigma-method': 'dmaic define measure analyze improve control defects variation belts quality spc',
    portfolio: 'dashboard all projects overview rollup analytics duckdb import csv xlsx status filter search cost',
    'project-dashboard':
      'open project charts documents budget committed contracts labour earned value evm spi cpi new chart export delete',
    'agile-boards':
      'kanban board columns cards drag drop wip limit work items backlog reorder priority story points sprints start complete active capacity goal dora deployment frequency lead time change failure rate mttr restore trend record deployment',
    budget:
      'budget panel committed remaining contracts labour estimate rollup cost money cents category breakdown over budget stakeholder rates hourly',
    'project-settings':
      'project settings name description owner industry methodology country status phase dates budget signing defaults certificate resource capacity calendars weekly overrides fonts import ttf compliance mode audit chain scenarios encryption migration schedule reports',
    scenarios:
      'scenario what if what-if copy chart baseline partition compare promote schedule baselines isolated experiment planning alternatives',
    'import-export':
      'import export ms project xml mspdi mpp interchange schedule report pdf docx odt csv html spreadsheet round trip',
    backups:
      'backup data safety copy project files pmforge folder pre-encryption backup retained restore integrity check repair vacuum maintenance cli recovery',
    timeline: 'calendar holidays country milestones dates schedule view months workdays',
    stakeholders: 'stakeholder manager power interest grid raci contacts influence engagement contract rates',
    'report-composer': 'combined report multiple documents charts assemble pdf export sections cover page',
    'export-signing':
      'pdf export sign pades digital signature certificate p12 pfx password gpg gnupg detached asc encrypt aes verify docx odt xlsx csv html mspdi xml formats',
    encryption:
      'sqlcipher database encryption at rest dek key passphrase lock secure recovery codes migrate plaintext',
    'admin-panel': 'user management create accounts admin recovery codes provision reset',
    'app-settings':
      'theme light dark auto save interval become administrator logs folder diagnostics preferences application settings',
    charts:
      'chart types catalog wbs network pert cpm gantt fishbone cause effect workflow activity raci swot stakeholder matrix line bar pareto pie burn up down cumulative flow control engines editors connect nodes dependencies baseline monte carlo histogram',
    documents:
      'charter scope statement risk register communication plan status report statement of work project plan word templates edit export',
    'sigma-pack': 'dmaic pack six sigma tollgate ctq sipoc capability control charts project view',
    shortcuts:
      'keyboard shortcuts hotkeys ctrl cmd save accessibility screen reader focus escape tab enter space navigate a11y announce reduced motion',
    glossary: 'terms definitions vocabulary jargon meaning dictionary',
    install: 'install run linux windows macos webkit dependencies build wails cli headless requirements',
  };

  let query = $state('');

  const filteredSidebar = $derived.by(() => {
    const q = query.trim().toLowerCase();
    if (!q) return sidebar;
    return sidebar
      .map((group) => ({
        group: group.group,
        items: group.items.filter(
          (item) =>
            item.label.toLowerCase().includes(q) ||
            group.group.toLowerCase().includes(q) ||
            KEYWORDS[item.id].includes(q),
        ),
      }))
      .filter((g) => g.items.length > 0);
  });

  const matchCount = $derived(filteredSidebar.reduce((n, g) => n + g.items.length, 0));
</script>

<div class="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
  <AppHeader active="help" />

  <div class="flex flex-1 overflow-hidden">
    <!-- Sidebar -->
    <nav
      class="w-52 shrink-0 border-r border-slate-800 overflow-y-auto py-4 px-2"
      aria-label="Help sections"
    >
      <div class="px-2 mb-4">
        <input
          type="search"
          bind:value={query}
          placeholder="Search help…"
          aria-label="Search help sections"
          class="w-full bg-slate-900 border border-slate-800 rounded px-2 py-1.5 text-xs text-slate-200 placeholder:text-slate-600 focus:border-cyan-500 outline-none"
        />
        {#if query.trim()}
          <p class="mt-1.5 text-[10px] text-slate-500" role="status">
            {matchCount === 0
              ? 'No sections match.'
              : `${matchCount} section${matchCount === 1 ? '' : 's'} match.`}
          </p>
        {/if}
      </div>
      {#each filteredSidebar as group}
        <div class="mb-5">
          <p class="px-2 mb-1 text-[10px] font-bold uppercase tracking-widest text-slate-500">
            {group.group}
          </p>
          {#each group.items as item}
            <button
              onclick={() => nav(item.id)}
              class={`w-full text-left px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                active === item.id
                  ? 'bg-slate-800 text-cyan-400'
                  : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/50'
              }`}
            >
              {item.label}
            </button>
          {/each}
        </div>
      {/each}
    </nav>

    <!-- Content -->
    <main class="flex-1 overflow-y-auto">
      <div class="max-w-3xl mx-auto px-8 py-6">
        <h1 class="sr-only">Help</h1>

        <!-- ── Getting Started ─────────────────────────────────────── -->
        {#if active === 'getting-started'}
          <h2 class="text-xl font-bold text-slate-100 mb-4">Getting Started</h2>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">First Launch</h3>
            <p class="text-sm text-slate-300 mb-2">
              On first launch PMForge has no accounts. Enter a username, display name, and
              passphrase on the Create Account screen. The first account is prompted to become
              the administrator. At least one admin must exist before additional users can be added.
            </p>
            <p class="text-sm text-slate-300">
              If you skipped the admin claim, open App Settings and use "Become administrator"
              while no other admin exists on the machine.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Adding Users</h3>
            <p class="text-sm text-slate-300">
              Administrators create additional accounts from the
              <button onclick={() => nav('admin-panel')} class="text-cyan-400 underline hover:text-cyan-300">Admin Panel</button>.
              Each PMForge user gets their own isolated data directory. Multiple PMForge users
              can share a single OS account; project files are stored per-user and are not
              cross-accessible through the app.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Creating a Project — The Launchpad</h3>
            <p class="text-sm text-slate-300 mb-3">
              From the Portfolio dashboard, click "New Project" (or File &rarr; New Project). The Launchpad
              walks through four steps:
            </p>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Industry</span> — Software, Construction, Engineering, Business, Administration, or Custom.</li>
              <li><span class="font-medium text-slate-100">Focus area</span> — narrows the industry (for example Web Dev, Civil, or Marketing).</li>
              <li>
                <span class="font-medium text-slate-100">Methodology</span> — delivery approach.
                Each combination seeds different starter artifacts. See the
                <button onclick={() => nav('industry-matrix')} class="text-cyan-400 underline hover:text-cyan-300">Industry &amp; Methodology Matrix</button>.
              </li>
              <li><span class="font-medium text-slate-100">Details &amp; starter artifacts</span> — project name, optional description, country (drives holiday calendars), and checkboxes for the suggested starter artifacts. Click Create Project to finish.</li>
            </ol>
            <p class="text-sm text-slate-300 mt-3">
              New to PMForge? The
              <button onclick={() => nav('quick-start')} class="text-cyan-400 underline hover:text-cyan-300">Quick Start tutorial</button>
              walks the whole journey — account to exported report — in about ten minutes.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Navigation</h3>
            <p class="text-sm text-slate-300">
              The top bar provides: Portfolio (all projects), Projects (project picker), App Settings,
              Help (this guide), and Sign Out. Within an open project the sidebar gives access to Charts,
              Documents, and methodology-specific views (Kanban, Backlog, Sprints, DORA, Six Sigma, etc.).
              Use File &rarr; Close Project to return to the Portfolio without signing out.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Account Recovery</h3>
            <p class="text-sm text-slate-300">
              PMForge is a local-first application with no cloud backup. Generate recovery codes
              immediately after creating your account (App Settings &rarr; Recovery Codes section).
              Store them securely. Recovery codes let you reset your passphrase from the login screen.
              Recovery codes must be current before enabling database encryption.
            </p>
          </section>

        <!-- ── Quick Start tutorial ───────────────────────────────── -->
        {:else if active === 'quick-start'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Quick Start: Your First Project</h2>
          <p class="text-sm text-slate-400 mb-5">
            A ten-minute walkthrough from a fresh install to an exported PDF report. Every step
            names exactly what to click; no project-management experience is assumed.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">1 · Create your account</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>Launch PMForge. With no accounts yet, the Create Account screen appears.</li>
              <li>Enter a username, display name, and a passphrase you can remember — it also protects your encrypted projects.</li>
              <li>As the first user you are offered the administrator role; accept it.</li>
              <li>
                PMForge shows your <span class="font-medium text-slate-100">eight recovery codes once</span>.
                Store them somewhere safe (password manager, printed copy) before continuing —
                they are the only way back in if you forget your passphrase. There is no cloud reset.
              </li>
            </ol>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">2 · Create a project</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>From the Portfolio screen click <span class="font-medium text-slate-100">+ New Project</span> (or press Ctrl/⌘+N).</li>
              <li>Pick an industry tile — choose <span class="font-medium text-slate-100">Software</span> for this tutorial.</li>
              <li>Pick a focus area (any), then the <span class="font-medium text-slate-100">Scrum</span> methodology.</li>
              <li>Name the project, leave the suggested starter artifacts checked, and click <span class="font-medium text-slate-100">Create Project</span>.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              You land on the Project Dashboard: charts on top, documents below, budget at the side.
              The starter artifacts from the
              <button onclick={() => nav('industry-matrix')} class="text-cyan-400 underline hover:text-cyan-300">matrix</button>
              are already there.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">3 · Build a small schedule</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>On the Dashboard choose <span class="font-medium text-slate-100">New Chart</span> and pick <span class="font-medium text-slate-100">Critical Path (CPM)</span>.</li>
              <li>Click <span class="font-medium text-slate-100">+ Node</span> three times to add three activities.</li>
              <li>Click a node, then rename it and set a duration (days) in the side panel.</li>
              <li>To add a dependency: select the first node, click <span class="font-medium text-slate-100">Connect…</span>, then click the node that must follow it.</li>
              <li>Repeat until the three activities form a chain. The longest chain turns <span class="text-red-400 font-medium">red</span> — that is your critical path.</li>
              <li>Press <span class="font-medium text-slate-100">Ctrl/⌘+S</span> to save (or rely on auto-save if enabled in App Settings).</li>
            </ol>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">4 · Fill in your first document</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>Back on the Dashboard, open the seeded <span class="font-medium text-slate-100">Project Charter</span> under Documents.</li>
              <li>Fill in the purpose and objectives fields — a sentence each is enough for now.</li>
              <li>Save with Ctrl/⌘+S. The save time appears near the header, so you always know your work is on disk.</li>
            </ol>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">5 · Export a PDF report</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>On the Dashboard, use the export action next to the Project Charter.</li>
              <li>In the Signature Options dialog choose <span class="font-medium text-slate-100">No digital signature</span> for now and click Export.</li>
              <li>The PDF is written to your private exports folder — the toast tells you where.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              When you need tamper-evident output, come back to
              <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">Export &amp; Digital Signing</button>
              for PAdES certificates and GnuPG signatures.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Where to go next</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><button onclick={() => nav('scrum')} class="text-cyan-400 underline hover:text-cyan-300">Your methodology's guide</button> — boards, sprints, and the cadence PMForge sets up for you.</li>
              <li><button onclick={() => nav('charts')} class="text-cyan-400 underline hover:text-cyan-300">Charts reference</button> — all 21 chart types and when to reach for each.</li>
              <li><button onclick={() => nav('shortcuts')} class="text-cyan-400 underline hover:text-cyan-300">Keyboard Shortcuts &amp; Accessibility</button> — work faster, mouse optional.</li>
              <li><button onclick={() => nav('encryption')} class="text-cyan-400 underline hover:text-cyan-300">Database Encryption</button> — protect project files at rest.</li>
            </ul>
          </section>

        <!-- ── Industry & Methodology Matrix ──────────────────────── -->
        {:else if active === 'industry-matrix'}
          <h2 class="text-xl font-bold text-slate-100 mb-2">Industry &amp; Methodology Matrix</h2>
          <p class="text-sm text-slate-400 mb-5">
            The Launchpad seeds starter artifacts automatically for these combinations.
            All other industry/methodology pairings receive a Project Charter only. Additional
            artifacts can always be created manually after project creation.
          </p>

          <div class="overflow-x-auto mb-6">
            <table class="w-full text-sm border-collapse">
              <thead>
                <tr class="border-b border-slate-700">
                  <th class="text-left py-2 pr-4 font-semibold text-slate-300 whitespace-nowrap">Industry</th>
                  <th class="text-left py-2 pr-4 font-semibold text-slate-300 whitespace-nowrap">Methodology</th>
                  <th class="text-left py-2 font-semibold text-slate-300">Seeded Artifacts</th>
                </tr>
              </thead>
              <tbody class="text-slate-300">
                {#each [
                  { ind: 'Software', meth: 'scrum' as SectionId, mLabel: 'Scrum', arts: 'Kanban Board, Project Charter, Agile Backlog, Sprint 1' },
                  { ind: 'Software', meth: 'kanban' as SectionId, mLabel: 'Kanban', arts: 'Kanban Board, Project Charter, Agile Backlog' },
                  { ind: 'Software', meth: 'scrumban' as SectionId, mLabel: 'Scrumban', arts: 'Kanban Board, Project Charter, Agile Backlog' },
                  { ind: 'Construction', meth: 'waterfall' as SectionId, mLabel: 'Waterfall', arts: 'WBS, Statement of Work, Risk Register, CPM Chart' },
                  { ind: 'Construction', meth: 'lean' as SectionId, mLabel: 'Lean', arts: 'WBS, Cumulative Flow Diagram, Risk Register' },
                  { ind: 'Engineering', meth: 'cpm' as SectionId, mLabel: 'CPM', arts: 'CPM Chart, WBS, Risk Register, Project Charter' },
                  { ind: 'Engineering', meth: 'six-sigma-method' as SectionId, mLabel: 'Six Sigma', arts: 'Control Chart, Pareto Chart, Fishbone Diagram' },
                  { ind: 'Business', meth: 'lean' as SectionId, mLabel: 'Lean', arts: 'Pareto Chart, Cumulative Flow Diagram, SWOT Matrix' },
                  { ind: 'Business', meth: 'okrs' as SectionId, mLabel: 'OKRs', arts: 'Project Plan (Word), Stakeholder Analysis Document, Status Report' },
                  { ind: 'Administration', meth: 'waterfall' as SectionId, mLabel: 'Waterfall', arts: 'Project Charter, Scope Statement, Risk Register, Communication Plan' },
                  { ind: 'Administration', meth: 'prince2' as SectionId, mLabel: 'PRINCE2', arts: 'Project Charter, Project Plan (Word), Risk Register' },
                ] as row}
                  <tr class="border-b border-slate-800">
                    <td class="py-2 pr-4 whitespace-nowrap">{row.ind}</td>
                    <td class="py-2 pr-4">
                      <button onclick={() => nav(row.meth)} class="text-cyan-400 hover:underline">{row.mLabel}</button>
                    </td>
                    <td class="py-2">{row.arts}</td>
                  </tr>
                {/each}
                <tr>
                  <td class="py-2 pr-4 text-slate-500 italic">All others</td>
                  <td class="py-2 pr-4 text-slate-500 italic">Any</td>
                  <td class="py-2 text-slate-500 italic">Project Charter</td>
                </tr>
              </tbody>
            </table>
          </div>

        <!-- ── Scrum ───────────────────────────────────────────────── -->
        {:else if active === 'scrum'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Scrum</h2>
          <p class="text-sm text-slate-400 mb-5">Iterative agile framework using time-boxed sprints with defined roles, events, and artifacts.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Software development and product teams where requirements evolve frequently. Best when stakeholders can commit to a regular feedback cadence and the team has 3-9 members working on a single product.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Software &rarr; Scrum. Seeds: Kanban Board, Project Charter, Agile Backlog, Sprint 1. The Kanban Board is the daily tracking surface. The Backlog holds the ordered list of user stories. Sprint 1 is a pre-created sprint container ready for backlog items.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Populate the Backlog.</span> Add user stories with story-point estimates and priority.</li>
              <li><span class="font-medium text-slate-100">Sprint Planning.</span> Pull stories from Backlog into a sprint. Define the Sprint Goal.</li>
              <li><span class="font-medium text-slate-100">Daily execution.</span> Move stories across Kanban Board columns (To Do &rarr; In Progress &rarr; Done). Limit WIP.</li>
              <li><span class="font-medium text-slate-100">Sprint Review.</span> Mark stories done. Unfinished stories return to Backlog.</li>
              <li><span class="font-medium text-slate-100">Retrospective.</span> Record improvements. Create the next sprint.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Burn-Down Chart</span> — remaining story points per sprint day vs. ideal trajectory.</li>
              <li><span class="font-medium text-slate-100">Burn-Up Chart</span> — cumulative scope completed vs. total scope; useful when scope changes.</li>
              <li><span class="font-medium text-slate-100">Cumulative Flow Diagram</span> — WIP and flow health over time.</li>
              <li><span class="font-medium text-slate-100">DORA Metrics</span> — deployment frequency, lead time, change failure rate, mean time to restore.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Key Terminology</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Story / Task</dt><dd class="text-slate-400">User-facing work item with acceptance criteria.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Story Points</dt><dd class="text-slate-400">Relative effort estimate; team-calibrated, not hours.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Sprint</dt><dd class="text-slate-400">Time-boxed iteration (typically 2 weeks).</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Velocity</dt><dd class="text-slate-400">Story points completed per sprint; used for capacity planning.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">DoD</dt><dd class="text-slate-400">Definition of Done — shared criteria that make a story complete.</dd></div>
            </dl>
          </section>

        <!-- ── Kanban ──────────────────────────────────────────────── -->
        {:else if active === 'kanban'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Kanban</h2>
          <p class="text-sm text-slate-400 mb-5">Visual pull-system for continuous flow. Cards move through columns representing workflow states; WIP limits prevent overloading any stage.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Continuous delivery, support queues, maintenance, or any context with unpredictable demand and variable item sizes. No fixed iteration length.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Software &rarr; Kanban. Seeds: Kanban Board, Project Charter, Agile Backlog. Columns represent workflow states (e.g., Backlog, In Progress, Review, Done).</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Design board columns</span> to match your actual workflow states.</li>
              <li><span class="font-medium text-slate-100">Set WIP limits per column.</span> Forces team to finish before starting new work, exposing bottlenecks.</li>
              <li><span class="font-medium text-slate-100">Pull cards forward</span> only when downstream capacity exists.</li>
              <li><span class="font-medium text-slate-100">Monitor flow metrics</span> via Cumulative Flow Diagram.</li>
              <li><span class="font-medium text-slate-100">Replenishment meeting</span> (periodic) — review priorities and add new cards.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Cumulative Flow Diagram</span> — primary Kanban health indicator; widening bands signal WIP growth.</li>
              <li><span class="font-medium text-slate-100">Pareto Chart</span> — identify categories consuming the most cycle time.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Key Terminology</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Card / Task</dt><dd class="text-slate-400">Single unit of work; one request or feature.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">WIP Limit</dt><dd class="text-slate-400">Maximum cards in a column simultaneously.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Cycle Time</dt><dd class="text-slate-400">Time from card start to done.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Throughput</dt><dd class="text-slate-400">Cards completed per unit time (week/month).</dd></div>
            </dl>
          </section>

        <!-- ── Scrumban ────────────────────────────────────────────── -->
        {:else if active === 'scrumban'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Scrumban</h2>
          <p class="text-sm text-slate-400 mb-5">Hybrid of Scrum's prioritized backlog and Kanban's continuous pull flow. No fixed sprint boundaries.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Teams transitioning from Scrum to Kanban, or teams with a mix of planned feature work and unplanned support/maintenance. Provides backlog discipline without rigid sprint ceremonies.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Software &rarr; Scrumban. Seeds: Kanban Board, Project Charter, Agile Backlog. The Backlog provides priority ordering; the Kanban Board drives daily flow. No sprint containers — work is pulled continuously.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Maintain a prioritized Backlog.</span> Order by business value; estimates are optional.</li>
              <li><span class="font-medium text-slate-100">Pull from Backlog when capacity opens.</span> When a Kanban column falls below WIP limit, pull the top-priority item.</li>
              <li><span class="font-medium text-slate-100">Replenish periodically.</span> Review and reorder backlog on a cadence rather than at sprint boundaries.</li>
              <li><span class="font-medium text-slate-100">Optional timeboxed reviews</span> for stakeholder alignment without full sprint commitment.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Cumulative Flow Diagram</span> — primary flow health indicator.</li>
              <li><span class="font-medium text-slate-100">Burn-Down Chart</span> — optional; useful for time-bounded work even without formal sprints.</li>
            </ul>
          </section>

        <!-- ── Lean ────────────────────────────────────────────────── -->
        {:else if active === 'lean'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Lean</h2>
          <p class="text-sm text-slate-400 mb-5">Maximize customer value while eliminating waste. Originated in manufacturing (Toyota Production System); applied across construction, services, and business operations.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Process improvement projects focused on efficiency — Construction (reducing rework, material waste), Business operations (reducing approval delays, handoff waste), service delivery.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Construction &rarr; Lean seeds: WBS, Cumulative Flow Diagram, Risk Register. Business &rarr; Lean seeds: Pareto Chart, Cumulative Flow Diagram, SWOT Matrix. The Pareto Chart identifies the vital few waste sources (80/20 rule); the Cumulative Flow Diagram tracks process throughput.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Define value</span> from the customer's perspective. Non-value-adding steps are waste.</li>
              <li><span class="font-medium text-slate-100">Map the value stream.</span> Document every step. Use a Workflow Diagram.</li>
              <li><span class="font-medium text-slate-100">Identify waste</span> (overproduction, waiting, transport, over-processing, inventory, motion, defects). Rank with a Pareto Chart.</li>
              <li><span class="font-medium text-slate-100">Create flow.</span> Remove waste; redesign so value-adding steps flow without interruption.</li>
              <li><span class="font-medium text-slate-100">Establish pull.</span> Produce only what downstream demands.</li>
              <li><span class="font-medium text-slate-100">Pursue perfection (Kaizen).</span> Repeat the cycle continuously.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Pareto Chart</span> — rank waste categories to focus improvement effort.</li>
              <li><span class="font-medium text-slate-100">Line Chart</span> — track a KPI (cycle time, defect rate) over time.</li>
              <li><span class="font-medium text-slate-100">Workflow Diagram</span> — current-state and future-state process maps.</li>
            </ul>
          </section>

        <!-- ── OKRs ────────────────────────────────────────────────── -->
        {:else if active === 'okrs'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">OKRs</h2>
          <p class="text-sm text-slate-400 mb-5">Objectives and Key Results: a goal-setting framework that aligns teams to strategic outcomes through measurable, time-bound targets.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Strategic planning cycles (quarterly or annually), cross-functional alignment initiatives, or any Business project where the challenge is agreement on what success looks like.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Business &rarr; OKRs. Seeds: Project Plan (Word), Stakeholder Analysis Document, Status Report. The Stakeholder Analysis Document maps who influences which Objectives; the Status Report provides the check-in template for KR progress updates.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Structure</h3>
            <div class="bg-slate-900 rounded p-3 text-sm text-slate-300 mb-3">
              <p class="font-medium text-slate-100 mb-1">Objective — qualitative, inspiring direction.</p>
              <div class="pl-4 border-l border-slate-700 space-y-1">
                <p class="text-xs text-slate-400">Key Result 1 — measurable, time-bound. Graded 0.0–1.0 at period close.</p>
                <p class="text-xs text-slate-400">Key Result 2 — each KR has a numeric target. 3-5 KRs per Objective is typical.</p>
              </div>
            </div>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Draft Objectives</span> at organization or team level. Ambitious but achievable.</li>
              <li><span class="font-medium text-slate-100">Define Key Results</span> — 3-5 measurable outcomes per Objective, each with baseline and target.</li>
              <li><span class="font-medium text-slate-100">Assign ownership</span> for each KR. Document in Charter or Status Reports.</li>
              <li><span class="font-medium text-slate-100">Check-ins</span> (weekly or bi-weekly) — update KR progress in Status Reports.</li>
              <li><span class="font-medium text-slate-100">Grade at period close</span> — score each KR 0.0–1.0. A score of 0.7 is typical target; 1.0 may mean the bar was set too low.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts &amp; Documents</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Stakeholder Analysis Matrix</span> — seeded; map stakeholder power/interest per Objective.</li>
              <li><span class="font-medium text-slate-100">Bar Chart</span> — compare KR scores at period close.</li>
              <li><span class="font-medium text-slate-100">Line Chart</span> — track KR progress trend during the measurement period.</li>
              <li><span class="font-medium text-slate-100">Status Report</span> — periodic written check-in against each KR target.</li>
            </ul>
          </section>

        <!-- ── Waterfall ───────────────────────────────────────────── -->
        {:else if active === 'waterfall'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Waterfall</h2>
          <p class="text-sm text-slate-400 mb-5">Sequential, phase-gated delivery. Each phase completes fully before the next begins. Scope, schedule, and budget are defined upfront.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Well-defined, stable scope. Construction, infrastructure, compliance-driven projects, regulated environments, or any project where mid-stream rework is prohibitively expensive.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Construction &rarr; Waterfall seeds: WBS, Statement of Work, Risk Register, CPM Chart. Administration &rarr; Waterfall seeds: Project Charter, Scope Statement, Risk Register, Communication Plan. Extend with Budget, Team Charter, and RACI Matrix as planning matures.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Phases &amp; Key Artifacts</h3>
            <div class="space-y-2 text-sm">
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Initiation</span><span class="text-slate-300">Charter, Business Case, Proposal, Stakeholder Analysis</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Planning</span><span class="text-slate-300">Project Plan, WBS, Gantt/CPM Schedule, Risk Register, Budget, Communication Plan, Scope Statement</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Execution</span><span class="text-slate-300">Project Brief, Project Overview, Status Reports, Issue Log, Change Requests</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Monitoring</span><span class="text-slate-300">Status Reports, Variance Analysis, Issue Log updates</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Closing</span><span class="text-slate-300">Project Closure, lessons learned</span></div>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Gantt Chart</span> — primary schedule view; dependencies and critical path.</li>
              <li><span class="font-medium text-slate-100">WBS</span> — hierarchical scope decomposition.</li>
              <li><span class="font-medium text-slate-100">RACI Matrix</span> — responsibility across all work packages.</li>
              <li><span class="font-medium text-slate-100">Network Diagram</span> — visualize task dependencies before building the Gantt.</li>
            </ul>
          </section>

        <!-- ── PRINCE2 ──────────────────────────────────────────────── -->
        {:else if active === 'prince2'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">PRINCE2</h2>
          <p class="text-sm text-slate-400 mb-5">Projects In Controlled Environments. A process-based framework with seven principles, seven themes, and seven processes. Strong governance and stage-gate controls.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Government or public-sector projects, formal multi-organization initiatives, or wherever an auditable trail of decisions and approvals is required. Common in UK, European, and Commonwealth environments.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Administration &rarr; PRINCE2. Seeds: Project Charter (approximates PID), Project Plan (Word), Risk Register. Extend with Communication Plan and Team Charter.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Processes in PMForge</h3>
            <div class="space-y-2 text-sm text-slate-300">
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Starting Up (SU)</span><span>Project Brief — use Charter. Appoint Project Board and PM roles.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Initiating (IP)</span><span>PID = Charter + Project Plan + Risk Register + Communication Plan.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Directing (DP)</span><span>Project Board uses Status Reports and Change Requests for decision-point documents.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Controlling (CS)</span><span>Team Manager tracks work packages via Issue Log and Status Reports.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Managing Delivery (MP)</span><span>Work Package execution; Project Overview for acceptance snapshots.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Closing (CP)</span><span>Project Closure document; follow-on action recommendations in notes field.</span></div>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Gantt Chart</span> — stage and work-package schedule.</li>
              <li><span class="font-medium text-slate-100">WBS</span> — deliverable hierarchy by management stage.</li>
              <li><span class="font-medium text-slate-100">RACI Matrix</span> — responsibility across Project Board, PM, and Team Managers.</li>
            </ul>
          </section>

        <!-- ── PMBOK ───────────────────────────────────────────────── -->
        {:else if active === 'pmbok'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">PMBOK</h2>
          <p class="text-sm text-slate-400 mb-5">Project Management Body of Knowledge. PMI's comprehensive framework of processes, knowledge areas, and best practices. A knowledge standard — paired with a delivery approach (Waterfall, CPM, Agile, etc.) rather than used alone.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Five Process Groups</h3>
            <p class="text-sm text-slate-300 mb-3">PMBOK's five process groups correspond directly to PMForge's document Phase categories. Browsing documents by phase is browsing by PMBOK process group.</p>
            <div class="space-y-2 text-sm">
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Initiating</span><span class="text-slate-300">Charter, Business Case, Stakeholder Analysis. 5 PMForge documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Planning</span><span class="text-slate-300">Define scope, schedule, cost, quality, risk, comms, procurement. 14 PMForge documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Executing</span><span class="text-slate-300">Carry out the plan. Project Brief, Project Overview. 2 PMForge documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Monitoring</span><span class="text-slate-300">Track and regulate performance. Status Report, Issue Log, Change Request Form. 3 documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Closing</span><span class="text-slate-300">Formally close the project. Project Closure. 1 document.</span></div>
            </div>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Ten Knowledge Areas</h3>
            <div class="grid grid-cols-2 gap-x-6 gap-y-1 text-xs text-slate-300">
              <span>Integration &rarr; Charter, Project Plan</span>
              <span>Scope &rarr; Scope Statement, WBS</span>
              <span>Schedule &rarr; Project Schedule, Gantt, Network, CPM</span>
              <span>Cost &rarr; Project Budget</span>
              <span>Quality &rarr; Control Chart, Pareto, Fishbone</span>
              <span>Resource &rarr; RACI, Team Charter</span>
              <span>Communications &rarr; Communication Plan, Status Report</span>
              <span>Risk &rarr; Risk Register, SWOT Matrix</span>
              <span>Procurement &rarr; Statement of Work, Procurement Plan</span>
              <span>Stakeholder &rarr; Stakeholder Analysis Matrix &amp; Document</span>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Using PMBOK in PMForge</h3>
            <p class="text-sm text-slate-300">PMBOK does not have a dedicated Launchpad option. Use it as vocabulary and process reference while running a Waterfall, CPM, or PRINCE2 project. The Documents view's phase-based organization mirrors PMBOK's process group structure.</p>
          </section>

        <!-- ── CPM ────────────────────────────────────────────────── -->
        {:else if active === 'cpm'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Critical Path Method (CPM)</h2>
          <p class="text-sm text-slate-400 mb-5">Network-based scheduling that identifies the longest dependency chain through the project. Tasks on the critical path have zero float; any delay directly delays project completion.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Engineering and construction projects with tightly coupled task dependencies, complex parallel workstreams, or where identifying "must-finish-on-time" tasks is essential.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Engineering &rarr; CPM. Seeds: CPM Chart, WBS, Risk Register, Project Charter. The CPM Chart displays activity nodes with ES/EF/LS/LF and critical-path highlighting; the WBS structures the deliverable hierarchy; the Risk Register surfaces schedule threats early.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Key Concepts</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">ES / EF</dt><dd class="text-slate-400">Earliest Start / Earliest Finish — forward-pass through the network.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">LS / LF</dt><dd class="text-slate-400">Latest Start / Latest Finish — backward-pass through the network.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">Float</dt><dd class="text-slate-400">LS - ES. Time a task can slip without delaying the project end date.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">Critical Path</dt><dd class="text-slate-400">Longest path through the network; all tasks on it have zero float.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Open Network Diagram.</span> Add each activity as a node. Define predecessor relationships.</li>
              <li><span class="font-medium text-slate-100">Enter durations.</span> The chart displays ES/EF/LS/LF values and highlights the critical path.</li>
              <li><span class="font-medium text-slate-100">Add risk estimates.</span> For uncertain tasks, enter optimistic, likely, and pessimistic durations, then run Monte Carlo from the CPM editor aside to review P50/P80/P90 finish days, the finish-probability S-curve, and tornado risk drivers. Export PDF/A saves the same risk evidence as a shareable report.</li>
              <li><span class="font-medium text-slate-100">Open Gantt Chart</span> to view activities on a time axis with dependency arrows.</li>
              <li><span class="font-medium text-slate-100">Generate Resource Histogram</span> to compare resource demand bars with dashed capacity lines from stakeholder availability and Resource Capacity calendars.</li>
              <li><span class="font-medium text-slate-100">Update actuals</span> as work progresses. Track completion percentages.</li>
              <li><span class="font-medium text-slate-100">Link a Project Schedule document</span> as the official schedule reference.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PERT vs CPM</h3>
            <p class="text-sm text-slate-300">The PERT chart computes expected duration and variance from three-point estimates. CPM uses deterministic durations for the live schedule, with optional Monte Carlo estimates for probabilistic finish-date analysis. Both use activity-on-node notation.</p>
          </section>

        <!-- ── Six Sigma (Methodology) ─────────────────────────────── -->
        {:else if active === 'six-sigma-method'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Six Sigma</h2>
          <p class="text-sm text-slate-400 mb-5">Data-driven quality improvement targeting 3.4 defects per million opportunities (6&sigma; from the mean). Uses DMAIC for existing processes.</p>

          <p class="text-xs bg-slate-900 border border-slate-700 rounded px-3 py-2 text-slate-400 mb-5">
            Six Sigma exists in PMForge in two forms: as a Launchpad methodology (described here, seeds initial charts) and as a dedicated
            <button onclick={() => nav('sigma-pack')} class="text-cyan-400 underline hover:text-cyan-300">DMAIC Pack</button>
            with structured project and dashboard views for the full 5-phase workflow.
          </p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PMForge Launchpad Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Engineering &rarr; Six Sigma. Seeds: Control Chart, Pareto Chart, Fishbone Diagram. Control Chart establishes the baseline process signature (Measure phase); Pareto ranks defect categories (Analyze phase); Fishbone begins root-cause analysis (Analyze phase). Add a Project Charter manually to complete the Define phase, or use the full <button onclick={() => nav('sigma-pack')} class="text-cyan-400 underline hover:text-cyan-300">DMAIC Pack</button>.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">DMAIC at a Glance</h3>
            <div class="space-y-1 text-sm">
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Define</span><span class="text-slate-300">Problem statement, CTQ tree, SIPOC, project charter, team &amp; scope.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Measure</span><span class="text-slate-300">Data collection plan, baseline metrics, process capability, VoC.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Analyze</span><span class="text-slate-300">Root-cause analysis via Fishbone + 5 Whys, Pareto analysis.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Improve</span><span class="text-slate-300">Solution Matrix (impact/effort/risk scoring), pilot testing.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Control</span><span class="text-slate-300">Control Plan, Control Chart monitoring, handover to process owner.</span></div>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Belt Levels</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Green Belt</dt><dd class="text-slate-400">Part-time, local improvement projects.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Black Belt</dt><dd class="text-slate-400">Full-time, cross-functional project leadership.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Master Black Belt</dt><dd class="text-slate-400">Strategic deployment; coaches Black Belts.</dd></div>
            </dl>
          </section>

        <!-- ── Portfolio ───────────────────────────────────────────── -->
        {:else if active === 'portfolio'}
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
            <p class="text-sm text-slate-300">
              Production and installer builds include DuckDB-backed in-memory portfolio
              analytics for cross-project cost rollups and local CSV/TSV, Parquet, and
              JSON data import. Money totals are staged as integer minor units and
              converted to display values only after aggregation.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Creating Projects</h3>
            <p class="text-sm text-slate-300">Click "New Project" (top right) to launch the
              <button onclick={() => nav('getting-started')} class="text-cyan-400 underline hover:text-cyan-300">Project Launchpad</button>.
              Projects cannot be deleted from the Portfolio. Project files remain on disk; data directories are never removed by the application.
            </p>
          </section>

        <!-- ── Project Dashboard ───────────────────────────────────── -->
        {:else if active === 'project-dashboard'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Project Dashboard</h2>
          <p class="text-sm text-slate-400 mb-5">The central hub for a project — lists all charts and documents, provides direct access to methodology-specific views, and surfaces export and signing actions per artifact.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Charts Panel</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Lists all charts in the current project with kind badge and creation date.</li>
              <li>Click a chart name to open its editor.</li>
              <li>"New Chart" opens the chart kind picker (all 21 types available).</li>
              <li>Delete (two-click confirm) removes the chart permanently.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Documents Panel</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li>Lists all documents with kind badge, phase badge, and last-modified date.</li>
              <li>Click a document to open its structured editor.</li>
              <li>"New Document" shows all 25 document types organized by phase.</li>
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

        <!-- ── Timeline ────────────────────────────────────────────── -->
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

        <!-- ── Stakeholder Manager ─────────────────────────────────── -->
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
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Hourly Rate</dt><dd class="text-slate-400">Used in budget cost rollup calculations. PMForge stores money internally as integer minor units and rounds once at the money boundary.</dd></div>
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

        <!-- ── Report Composer ─────────────────────────────────────── -->
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

        {:else if active === 'import-export'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Schedule Import &amp; Export</h2>
          <p class="text-sm text-slate-400 mb-5">
            Move schedules between PMForge and other tools — Microsoft Project in, reports and
            interchange formats out.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Importing from Microsoft Project</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>On the Project Dashboard, choose the <span class="font-medium text-slate-100">Import MS Project XML</span> action.</li>
              <li>Pick an MSPDI <span class="font-mono text-xs">.xml</span> file; PMForge creates a schedule chart from its tasks and dependencies.</li>
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

        <!-- ── Export & Digital Signing ───────────────────────────── -->
        {:else if active === 'export-signing'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Export &amp; Digital Signing</h2>
          <p class="text-sm text-slate-400 mb-5">PMForge exports documents in multiple formats and supports PAdES-compliant digital signatures using a personal certificate (.p12/.pfx).</p>

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
              <li>In the "Digital Signatures (PDF/A)" section, enable signatures and browse to your .p12 or .pfx certificate file.</li>
              <li>Save settings. The certificate path is stored; the password is never persisted.</li>
            </ol>
            <p class="text-sm font-medium text-slate-200 mb-2">Sign a document:</p>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li>From the Project Dashboard, click "Sign &amp; Export" on the document you want to sign.</li>
              <li>The Sign dialog shows the configured certificate path. You can choose a different certificate if needed.</li>
              <li>Enter the certificate password (used only for this operation — never stored).</li>
              <li>Click "Sign &amp; Export." The signed PAdES PDF is written to your exports directory.</li>
            </ol>
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
            <p class="text-sm text-slate-300">Exported files are saved to <span class="font-mono text-xs bg-slate-900 px-1 rounded">~/Documents/PMForge/&lt;username&gt;/exports/</span>. The full path is shown in a success toast after every export. App Settings also shows your data directory location (the parent of exports/). Use App Settings &rarr; Open Logs Folder to open the log directory in Finder/Explorer if you need diagnostic files.</p>
          </section>

        <!-- ── Database Encryption ─────────────────────────────────── -->
        {:else if active === 'encryption'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Database Encryption</h2>
          <p class="text-sm text-slate-400 mb-5">Each project stores its data in a SQLite database. PMForge can encrypt this database at rest using SQLCipher, which applies AES-256 encryption to the entire database file. This protects project data if the machine is lost or the filesystem is accessed directly.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Prerequisites</h3>
            <p class="text-sm text-slate-300 mb-2">Before enabling encryption, you must generate recovery codes for your account:</p>
            <ol class="space-y-1 text-sm text-slate-300 list-decimal list-inside">
              <li>Go to App Settings (top nav) and find the Recovery Codes section.</li>
              <li>Generate a new set of recovery codes. Store them securely (password manager, safe, printed).</li>
              <li>Recovery codes must be current — if you have old codes from before, reissue them. PMForge enforces this before allowing encryption to proceed.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Encrypting a Project</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li>Open the project you want to encrypt. The project must be opened from the project list (not just selected).</li>
              <li>Go to Project Settings (File &rarr; Project Settings, or the gear icon).</li>
              <li>Find the "Database Encryption" section. It shows the current state: Plaintext or Encrypted.</li>
              <li>Click "Encrypt Database." PMForge creates a backup of the plaintext database first and shows the backup path.</li>
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

        <!-- ── Admin Panel ─────────────────────────────────────────── -->
        {:else if active === 'backups'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Backups &amp; Data Safety</h2>
          <p class="text-sm text-slate-400 mb-5">
            PMForge is local-first: your data lives in ordinary files you can copy, so backup is
            simple — but it is <span class="font-medium text-slate-100">your</span> job. There is
            no cloud copy.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">What to back up</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Everything lives under <span class="font-mono text-xs">~/Documents/PMForge/</span> by default: <span class="font-mono text-xs">system.db</span> (accounts) plus a private per-user folder with <span class="font-mono text-xs">projects</span>, <span class="font-mono text-xs">certs</span>, <span class="font-mono text-xs">exports</span>, and <span class="font-mono text-xs">logs</span>.</li>
              <li>Copying that folder while PMForge is closed is a complete backup. Encrypted projects stay encrypted in the copy — safe to store anywhere you trust with ciphertext.</li>
              <li>Keep your <span class="font-medium text-slate-100">recovery codes</span> with the backup: for encrypted projects, a restored file is only usable with your passphrase or a valid recovery code. Without both, it is unrecoverable by design.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Automatic safety nets</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>When you migrate a plaintext project to encrypted storage, PMForge <span class="font-medium text-slate-100">retains a pre-migration backup</span> and shows you its path — keep it until you have verified the encrypted project opens.</li>
              <li>Editors auto-save on an interval (App Settings) and show the last save time, so a crash costs at most the interval.</li>
              <li>Every export is written to your private <span class="font-mono text-xs">exports</span> folder with owner-only permissions.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Health checks &amp; repair</h3>
            <p class="text-sm text-slate-300">
              The PMForge binary doubles as a maintenance CLI for any
              <span class="font-mono text-xs">.pmforge</span> file:
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

        {:else if active === 'admin-panel'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Admin Panel</h2>
          <p class="text-sm text-slate-400 mb-5">Administrator-only view for managing all PMForge user accounts on this machine. Accessible to accounts with the Admin role.</p>

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

        <!-- ── App Settings ────────────────────────────────────────── -->
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
            <p class="text-sm text-slate-300">Shows your current version, signed-in username, and the data directory location on disk (e.g., ~/Documents/PMForge/username/).</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Diagnostics</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Open Logs Folder</dt><dd class="text-slate-400">Opens the PMForge log directory in Finder/Explorer for troubleshooting.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Generate Bug Report</dt><dd class="text-slate-400">Creates a diagnostic report file in your data directory. Include this when reporting issues.</dd></div>
            </dl>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Become Administrator</h3>
            <p class="text-sm text-slate-300">If no administrator exists on the machine and your account is not an admin, a warning panel appears with a "Become administrator" button. This claims the admin role and cannot be undone via the UI (requires another admin to demote you). This option is only shown when the machine has no admin at all.</p>
          </section>

        <!-- ── Charts Reference ────────────────────────────────────── -->
        {:else if active === 'charts'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Charts Reference</h2>
          <p class="text-sm text-slate-400 mb-5">PMForge includes 21 chart types organized across four rendering engines. Charts are created from the project's Charts panel or via New Chart on the Project Dashboard.</p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">DAG Engine — Schedule &amp; Hierarchy (7 charts)</h3>
            <p class="text-xs text-slate-500 mb-3">Activity-on-node directed graphs. Tasks are nodes; arrows are dependency relationships.</p>
            <div class="space-y-3">
              {#each [
                { name: 'Work Breakdown Structure (WBS)', desc: 'Hierarchical decomposition of project scope into deliverables and work packages. Root = project; leaf nodes = work packages.' },
                { name: 'Network Diagram', desc: 'Activity-on-node diagram showing precedence relationships. Displays ES/EF/LS/LF values and highlights the critical path.' },
                { name: 'PERT Chart', desc: 'Network diagram with three-point duration estimates per activity: Optimistic, Most Likely, Pessimistic. Use when durations are uncertain.' },
                { name: 'CPM Chart', desc: 'Activity nodes annotated with ES/EF/LS/LF and critical-path highlighting. Deterministic (single-point) durations.' },
                { name: 'Gantt Chart', desc: 'Schedule bars on a time axis with dependency arrows, critical-path highlighting, progress bars, and baseline overlay. Shares data with CPM.' },
                { name: 'Fishbone (Ishikawa) Diagram', desc: 'Root-cause analysis. Default cause categories (People, Process, Equipment, Materials, Environment, Measurement) branch from a central effect statement. Categories are editable.' },
                { name: 'Cause-and-Effect Diagram', desc: 'Generic cause/effect tree. More flexible than Fishbone; supports arbitrary nesting. Use when Fishbone categories do not fit the domain.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">Statistical Engine — Data Visualization (8 charts)</h3>
            <p class="text-xs text-slate-500 mb-3">Data series charts. Enter data series; the chart renders the visualization.</p>
            <div class="space-y-3">
              {#each [
                { name: 'Line Chart', desc: 'One or more series against a continuous x-axis. Use for trends over time (KPIs, cost curves, performance metrics).' },
                { name: 'Bar Chart', desc: 'Categorical comparison with vertical or horizontal bars. Compare values across discrete categories or time periods.' },
                { name: 'Pareto Chart', desc: 'Bar chart sorted descending with a cumulative-percentage line overlay. Identifies the vital few causes (80/20 rule).' },
                { name: 'Pie Chart', desc: 'Part-to-whole composition for 2-6 categories. Best when the whole is meaningful (budget breakdown, effort split).' },
                { name: 'Burn-Up Chart', desc: 'Cumulative scope completed vs. total scope over time. Distinguishes scope growth from delivery progress.' },
                { name: 'Burn-Down Chart', desc: 'Remaining work over time with an ideal trajectory reference line. Tracks sprint or release schedule performance.' },
                { name: 'Cumulative Flow Diagram', desc: 'Stacked area chart of work in each workflow state over time. Band widths reveal WIP; band slopes reveal throughput.' },
                { name: 'Control Chart', desc: 'Time series with Upper (UCL) and Lower (LCL) Control Limits. Points outside limits or non-random patterns indicate special-cause variation.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">Matrix Engine — Grid Diagrams (4 charts)</h3>
            <p class="text-xs text-slate-500 mb-3">Two-dimensional grids relating two sets of items or dimensions.</p>
            <div class="space-y-3">
              {#each [
                { name: 'RACI Matrix', desc: 'Responsibility assignment: Responsible (does the work), Accountable (owns the outcome), Consulted (input needed), Informed (kept updated) — per task/role cell. One A per task.' },
                { name: 'SWOT Matrix', desc: '2x2 grid: Strengths / Weaknesses (internal) vs. Opportunities / Threats (external). Favorable vs. unfavorable.' },
                { name: 'Stakeholder Analysis Matrix', desc: 'Stakeholders plotted on a Power vs. Interest grid. Position drives engagement strategy: high power/high interest = manage closely.' },
                { name: 'Matrix Diagram', desc: 'Generic m×n grid for relating any two dimensions — requirements traceability, quality function deployment, or any custom comparison.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">Flow Engine — Process Diagrams (2 charts)</h3>
            <div class="space-y-3">
              {#each [
                { name: 'Workflow Diagram', desc: 'Process flow with decisions, gates, and parallel paths. Use for current-state and future-state process mapping in Lean, BPM, or SOP documentation.' },
                { name: 'Activity Diagram', desc: 'UML-style activity flow with swimlanes for different actors or systems. Use when the process crosses organizational or system boundaries.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

        <!-- ── Documents Reference ─────────────────────────────────── -->
          <section class="mb-6 border-t border-slate-800 pt-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Editing schedule diagrams (WBS, Network, PERT, CPM, Workflow, Activity)</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">+ Node</span> adds an activity; click a node (or Tab to it and press Enter/Space) and edit its fields in the side panel.</li>
              <li>To add a dependency: select the source node, click <span class="font-medium text-slate-100">Connect…</span>, then click the destination node. <span class="font-medium text-slate-100">Clear edges</span> removes a node's links; <span class="font-medium text-slate-100">Delete node</span> removes it and its edges.</li>
              <li>In CPM (and Gantt links), the edge label sets the <span class="font-medium text-slate-100">dependency type and lag</span>: <span class="font-mono text-xs">FS</span>, <span class="font-mono text-xs">SS</span>, <span class="font-mono text-xs">FF</span>, or <span class="font-mono text-xs">SF</span> with optional <span class="font-mono text-xs">+n</span>/<span class="font-mono text-xs">-n</span> days — e.g. <span class="font-mono text-xs">SS+2</span> (start 2 days after the predecessor starts) or <span class="font-mono text-xs">FS-1</span> (overlap by a day). Blank means FS. This drives the computed schedule.</li>
              <li>Activity diagrams add <span class="font-medium text-slate-100">swimlanes</span> (one per role) and UML node shapes — initial, activity, decision, fork/join, final — chosen when adding the node.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Gantt editor specifics</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>The left grid edits tasks (name, duration in days, % complete, milestone ◆, delete); the right canvas draws the bars. <span class="font-medium text-slate-100">− / +</span> zoom the day scale.</li>
              <li>Bar colours carry meaning: <span class="text-red-400 font-medium">red</span> = critical path, teal strip = % complete, grey ghost = baseline, orange outline = over-allocated resource, amber ! = violated constraint. Hover any bar for its full story.</li>
              <li><span class="font-medium text-slate-100">Set baseline</span> snapshots today's schedule; ghost bars then show drift against it as the plan changes.</li>
              <li>Links are managed in the panel below the grid — pick from/to tasks and an optional type/lag label (same <span class="font-mono text-xs">FS/SS/FF/SF±n</span> syntax as CPM).</li>
              <li>Real calendar dates appear on bars once the project has a start date in <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Monte Carlo schedule risk (CPM)</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>In the CPM editor, give uncertain tasks optimistic / most-likely / pessimistic duration estimates (tasks without estimates keep their fixed duration).</li>
              <li>Choose a sampling distribution — triangular, beta-PERT, or normal — and run the simulation from the editor's side panel.</li>
              <li>Read the results: <span class="font-medium text-slate-100">P50 / P80 / P90</span> finish-day confidence points, a cumulative finish-probability S-curve, and a <span class="font-medium text-slate-100">tornado ranking</span> of which tasks drive the risk (critical-path frequency × P90−P50 spread).</li>
              <li>Use <span class="font-medium text-slate-100">Export PDF/A</span> to save a signed-off-ready risk report with the summary, S-curve, tornado drivers, and narrative.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              CPM can also generate a <span class="font-medium text-slate-100">Resource Histogram</span> —
              demand bars with dashed capacity lines from stakeholder availability and Project
              Settings resource calendars.
            </p>
          </section>

          <section class="mb-4">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Grid &amp; data-series editors</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">Stats charts</span> (Line, Bar, Pie, Pareto, Burn-Up/Down, Cumulative Flow, Control): enter series and values in the form under the live preview — it re-renders as you type. Control charts flag out-of-control points in red with the rule in the tooltip.</li>
              <li><span class="font-medium text-slate-100">RACI</span>: add roles and tasks, then set each cell to R/A/C/I from its dropdown. The validation tray warns when a task lacks exactly one Accountable.</li>
              <li><span class="font-medium text-slate-100">Matrix</span>: add rows and columns, type in any cell; row/column headers rename inline.</li>
              <li><span class="font-medium text-slate-100">SWOT</span>: four fixed quadrants; add/remove bullet items per quadrant.</li>
              <li><span class="font-medium text-slate-100">Fishbone / Cause-and-Effect</span>: manage categories and causes in the side panel (Fishbone can seed the classic six Ms); the diagram lays itself out.</li>
              <li>All editors save with <span class="font-mono text-xs">Ctrl/⌘+S</span> and participate in auto-save — see <button onclick={() => nav('shortcuts')} class="text-cyan-400 underline hover:text-cyan-300">Keyboard Shortcuts</button>.</li>
            </ul>
          </section>

        {:else if active === 'documents'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Documents Reference</h2>
          <p class="text-sm text-slate-400 mb-2">25 structured document types organized by PMBOK process group. Each has a structured editor with typed fields. Chart references can be embedded in documents. Export formats depend on document type.</p>
          <p class="text-xs text-slate-500 mb-5">Access documents from the project's Documents panel or Project Dashboard.</p>

          {#each [
            { phase: 'Initiation', count: 5, docs: [
              { name: 'Project Charter (Word)', desc: 'Formally authorizes the project. Captures purpose, objectives, scope, stakeholders, high-level schedule and budget, and sponsor sign-off. The foundational document referenced by all other planning artifacts.' },
              { name: 'Project Charter (Excel)', desc: 'Same content as the Word charter; default export format is XLSX.' },
              { name: 'Business Case', desc: 'Justifies the project investment: costs, benefits, risks, NPV/ROI analysis, and strategic alignment. Primary input to the go/no-go decision.' },
              { name: 'Project Proposal', desc: 'Persuasive overview to win stakeholder buy-in before formal authorization. Less rigorous than the Charter.' },
              { name: 'Stakeholder Analysis Document', desc: 'Narrative companion to the Stakeholder Analysis Matrix chart. Documents individual engagement strategies. Links to a Stakeholder Analysis chart.' },
            ]},
            { phase: 'Planning', count: 14, docs: [
              { name: 'Project Plan (Word)', desc: 'Most comprehensive PM document. Consolidates scope, schedule, budget, quality, risk, communications, procurement, and team plans. Links to CPM schedule, WBS, and RACI charts.' },
              { name: 'Project Plan (Excel)', desc: 'Same content as Word plan; exports to XLSX.' },
              { name: 'Project Schedule', desc: 'Authoritative timeline: every task with durations, predecessors, and the critical path. Requires a linked CPM or Gantt chart.' },
              { name: 'Work Breakdown Structure', desc: 'Narrative around the WBS chart: deliverable definitions, work-package owners, and acceptance criteria. Requires a linked WBS chart.' },
              { name: 'RACI Chart Document', desc: 'Prose companion to the RACI Matrix chart: role definitions and effective dates. Requires a linked RACI chart.' },
              { name: 'Risk Register', desc: 'Catalogue of potential risks with probability, impact, risk score, owner, mitigation strategy, and contingency plan.' },
              { name: 'Scope Statement', desc: 'Defines exactly what the project will and will not deliver. Includes deliverables, acceptance criteria, constraints, assumptions, and exclusions.' },
              { name: 'Project Budget', desc: 'Cost estimate broken down by category: labor, materials, equipment, subcontractors, contingency, and management reserve.' },
              { name: 'Communication Plan', desc: 'Who needs what information, in what format, on what cadence, and via which channel.' },
              { name: 'Project Execution Plan', desc: 'Operational plan: detailed task breakdown, resource assignments, and execution timeline.' },
              { name: 'Statement of Work', desc: 'Formal scope, deliverables, timeline, and responsibility definition issued to vendors or contractors.' },
              { name: 'Procurement Plan', desc: 'What will be procured externally, via which method, from which vendors, and on what schedule.' },
              { name: 'Requirements Document', desc: 'Functional and non-functional specifications the project must satisfy.' },
              { name: 'Team Charter', desc: 'Roles, responsibilities, deliverables, working agreements, decision rights, and escalation paths for the project team.' },
            ]},
            { phase: 'Execution', count: 2, docs: [
              { name: 'Project Brief', desc: 'Short, audience-oriented summary of the plan for non-PM stakeholders.' },
              { name: 'Project Overview', desc: '1-page snapshot: timeline, milestones, budget status, and key roles. Use at reviews and steering committee meetings.' },
            ]},
            { phase: 'Monitoring', count: 3, docs: [
              { name: 'Status Report', desc: 'Periodic check-in: work completed, upcoming work, schedule and budget variance, open risks, blockers, and optional linked CPM schedule for Earned Value in combined reports.' },
              { name: 'Issue Log', desc: 'Tracks problems that have occurred (vs. risks, which are potential). Each issue has owner, priority, and resolution plan.' },
              { name: 'Change Request Form', desc: 'Formally proposes a change to a project baseline. Includes impact analysis and approval fields.' },
            ]},
            { phase: 'Closing', count: 1, docs: [
              { name: 'Project Closure', desc: 'Formal end-of-project record: final deliverable acceptance, contract closure status, lessons learned, and sponsor sign-off.' },
            ]},
          ] as group}
            <section class="mb-6">
              <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">
                {group.phase} <span class="text-slate-500 font-normal">({group.count})</span>
              </h3>
              <div class="space-y-3">
                {#each group.docs as doc}
                  <div>
                    <p class="text-sm font-medium text-slate-100">{doc.name}</p>
                    <p class="text-xs text-slate-400 mt-0.5">{doc.desc}</p>
                  </div>
                {/each}
              </div>
            </section>
          {/each}

        <!-- ── DMAIC Pack ──────────────────────────────────────────── -->
        {:else if active === 'sigma-pack'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Six Sigma DMAIC Pack</h2>
          <p class="text-sm text-slate-400 mb-3">Dedicated structured environment for DMAIC projects, separate from the standard Launchpad chart seeds. Provides two views: Sigma Workspace (project dashboard) and Sigma Project (active project editor with phase-by-phase structure).</p>
          <p class="text-xs bg-slate-900 border border-slate-700 rounded px-3 py-2 text-slate-400 mb-5">
            For how the Launchpad seeds initial charts when selecting Six Sigma as a methodology, see
            <button onclick={() => nav('six-sigma-method')} class="text-cyan-400 underline hover:text-cyan-300">Six Sigma (Methodology)</button>.
          </p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Belt Levels</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Green Belt</dt><dd class="text-slate-400">Part-time, localized process improvements.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Black Belt</dt><dd class="text-slate-400">Full-time, cross-functional improvement projects.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Master Black Belt</dt><dd class="text-slate-400">Strategic deployment; coaches Black Belts.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Define Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Project Charter</span> — problem statement, goal statement, scope, team roles, timeline, and business case.</li>
              <li><span class="font-medium text-slate-100">Voice of Customer / CTQ</span> — captures customer needs and maps each to a measurable Critical to Quality characteristic with lower/upper specification limits. Entries derive the CTQ tree used in the project charter.</li>
              <li><span class="font-medium text-slate-100">SIPOC</span> — high-level process map: Suppliers &rarr; Inputs &rarr; Process &rarr; Outputs &rarr; Customers. Defines process boundaries (start trigger, end trigger).</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Measure Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Voice of Customer (VoC)</span> — customer needs with CTQ, specifications (lower/upper limits), measurement method, data collection plan, and priority.</li>
              <li><span class="font-medium text-slate-100">Control Chart</span> — baseline process signature before any improvement changes.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Analyze Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Fishbone Diagram</span> — structured root-cause analysis across cause categories.</li>
              <li><span class="font-medium text-slate-100">5 Whys</span> — iterative question technique stored per cause branch in the Fishbone structure. Drills to the root cause.</li>
              <li><span class="font-medium text-slate-100">Pareto Chart</span> — ranks defect causes by frequency or cost to identify the vital few (80/20).</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Improve Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Solution Matrix</span> — evaluates candidate solutions on Impact (1-10), Effort (1-10), Risk (1-10), and Cost. Statuses: Proposed, Pilot, Implemented.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Control Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Control Plan</span> — per process step: metric, specification limits, measurement method, frequency, owner, and response plan if out of control.</li>
              <li><span class="font-medium text-slate-100">Control Chart (ongoing)</span> — monitor the improved process with post-improvement control limits.</li>
            </ul>
          </section>

        <!-- ── Glossary ────────────────────────────────────────────── -->
        {:else if active === 'shortcuts'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Keyboard Shortcuts &amp; Accessibility</h2>
          <p class="text-sm text-slate-400 mb-5">
            PMForge is fully operable from the keyboard. Shortcuts use Ctrl on Windows/Linux and ⌘ on macOS.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Application (File menu)</h3>
            <div class="overflow-x-auto">
              <table class="w-full text-sm border-collapse">
                <thead>
                  <tr class="border-b border-slate-700">
                    <th class="text-left py-1.5 pr-4 font-semibold text-slate-300 w-40">Shortcut</th>
                    <th class="text-left py-1.5 font-semibold text-slate-300">Action</th>
                  </tr>
                </thead>
                <tbody class="text-slate-300">
                  {#each [
                    ['Ctrl/⌘ + N', 'New project (opens the Launchpad)'],
                    ['Ctrl/⌘ + O', 'Open project (project picker)'],
                    ['Ctrl/⌘ + D', 'Portfolio dashboard'],
                    ['Ctrl/⌘ + ,', 'Application settings'],
                    ['Ctrl/⌘ + W', 'Close the current project'],
                    ['Ctrl/⌘ + Q', 'Quit PMForge'],
                    ['F11', 'Maximize / restore the window (Window menu)'],
                  ] as [keys, action]}
                    <tr class="border-b border-slate-800">
                      <td class="py-1.5 pr-4 font-mono text-xs text-cyan-300 whitespace-nowrap">{keys}</td>
                      <td class="py-1.5">{action}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Editors &amp; diagrams</h3>
            <div class="overflow-x-auto">
              <table class="w-full text-sm border-collapse">
                <thead>
                  <tr class="border-b border-slate-700">
                    <th class="text-left py-1.5 pr-4 font-semibold text-slate-300 w-40">Shortcut</th>
                    <th class="text-left py-1.5 font-semibold text-slate-300">Action</th>
                  </tr>
                </thead>
                <tbody class="text-slate-300">
                  {#each [
                    ['Ctrl/⌘ + S', 'Save the open chart or document editor. Auto-save (App Settings) also saves on an interval.'],
                    ['Tab / Shift+Tab', 'Move between nodes in a diagram (WBS, Network, PERT, CPM, Workflow, Activity, Stakeholder, Cause-and-Effect).'],
                    ['Enter or Space', 'Select the focused diagram node; edit its fields in the side panel.'],
                    ['Enter', 'In the signature dialog password field: confirm the export.'],
                  ] as [keys, action]}
                    <tr class="border-b border-slate-800">
                      <td class="py-1.5 pr-4 font-mono text-xs text-cyan-300 whitespace-nowrap">{keys}</td>
                      <td class="py-1.5">{action}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Dialogs &amp; wizards</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-mono text-xs text-cyan-300">Esc</span> closes any dialog from anywhere inside it (for example the export Signature Options), cancelling the action.</li>
              <li><span class="font-mono text-xs text-cyan-300">Tab</span> cycles only through the dialog's own controls while it is open, and focus returns to the control that opened it when it closes.</li>
              <li>The New Project wizard moves focus to each step's heading as you advance, so keyboard and screen-reader users always know where they are.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Assistive technology</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Every navigation announces the destination view ("Gantt chart", "Portfolio") to screen readers.</li>
              <li>Save confirmations, loading states, and errors are announced as they happen via live regions.</li>
              <li>Rendered charts expose text descriptions (kind, title, series); Gantt bars have hover tooltips with day ranges, % complete, and critical-path status.</li>
              <li>Animations are disabled automatically when your OS requests reduced motion; a light theme is available in <button onclick={() => nav('app-settings')} class="text-cyan-400 underline hover:text-cyan-300">App Settings</button>.</li>
            </ul>
          </section>

        {:else if active === 'glossary'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Glossary</h2>
          <p class="text-sm text-slate-400 mb-5">Definitions for project management terms, methodology-specific vocabulary, and PMForge-specific concepts.</p>

          <div class="space-y-3 text-sm">
            {#each [
              { term: 'Accountability', def: 'Ultimate ownership of an outcome. In RACI, the "A" — one person owns the result even if others do the work.' },
              { term: 'Agile', def: 'Umbrella term for iterative delivery methodologies valuing customer collaboration and responding to change. Scrum, Kanban, and Scrumban are Agile frameworks.' },
              { term: 'Backlog', def: 'Ordered list of work items waiting to be pulled into active work. In Scrum: the Product Backlog. In Scrumban: maintained continuously.' },
              { term: 'Baseline', def: 'The approved, time-phased plan for scope, schedule, or cost against which performance is measured. Changes require formal change control.' },
              { term: 'Belt Level', def: 'Six Sigma practitioner certification: Green Belt (part-time, local projects), Black Belt (full-time, cross-functional), Master Black Belt (strategic deployment).' },
              { term: 'Burn-Down Chart', def: 'Remaining work (y) over time (x) with an ideal trajectory. Used in Scrum to track sprint progress.' },
              { term: 'Burn-Up Chart', def: 'Cumulative scope completed vs. total scope. Distinguishes scope growth from delivery progress.' },
              { term: 'Business Case', def: 'Document justifying the project investment: costs, benefits, risks, NPV/ROI. Primary go/no-go input.' },
              { term: 'Card / Task (Kanban)', def: 'A single unit of work on the Kanban Board.' },
              { term: 'Change Request', def: 'Formal proposal to modify a project baseline (scope, schedule, cost, quality). Requires review and approval.' },
              { term: 'Control Chart', def: 'Time series with UCL and LCL. Points outside limits or non-random patterns indicate special-cause variation.' },
              { term: 'Control Limits', def: 'Statistical boundaries on a Control Chart (±3σ from the process mean). Not the same as specification limits.' },
              { term: 'CPM', def: 'Critical Path Method. Network scheduling identifying the longest dependency chain. Critical path tasks have zero float.' },
              { term: 'Critical Path', def: 'Sequence of tasks from start to end with zero total float. Any delay to a critical path task delays the project end date.' },
              { term: 'CTQ', def: 'Critical to Quality. Measurable characteristic directly affecting customer perception of quality. Derived from VoC in Six Sigma.' },
              { term: 'Cumulative Flow Diagram', def: 'Stacked area chart of work items in each workflow state over time. Primary Kanban health indicator.' },
              { term: 'Cycle Time', def: 'Time from when a work item starts to when it is completed. Key delivery predictability metric in Kanban.' },
              { term: 'DMAIC', def: 'Six Sigma improvement cycle: Define → Measure → Analyze → Improve → Control. For improving existing processes.' },
              { term: 'DORA Metrics', def: 'Four DevOps metrics: Deployment Frequency, Lead Time for Changes, Change Failure Rate, Mean Time to Restore. Available in PMForge\'s DORA dashboard.' },
              { term: 'Epic', def: 'A large user story spanning multiple sprints, broken down into smaller stories before entering a sprint.' },
              { term: 'ES / EF', def: 'Earliest Start / Earliest Finish. Calculated during the forward pass through a CPM network.' },
              { term: 'Estimate (methodology-specific)', def: 'Scrum: Story Points. Kanban: Time Estimate. CPM: Duration. Waterfall: Duration. Lean: Effort. PRINCE2: Work Package Estimate. Six Sigma: Resource Plan.' },
              { term: 'Fishbone Diagram', def: 'Also Ishikawa Diagram. Root-cause analysis: effect at the "head"; default cause categories (People, Process, Equipment, Materials, Environment, Measurement) are the "bones." Categories are editable.' },
              { term: 'Float (Slack)', def: 'Amount of time a task can be delayed without delaying the project end date (total float) or a successor\'s start (free float).' },
              { term: 'Gantt Chart', def: 'Schedule bars on a time axis with dependency arrows, critical-path highlighting, and progress bars. Shares data with CPM in PMForge.' },
              { term: 'Issue Log', def: 'Tracks problems that have already occurred, with owner, priority, and resolution plan. Distinct from Risk Register (potential future events).' },
              { term: 'Iteration', def: 'Time-boxed work cycle. Scrum: Sprint. PRINCE2: Management Stage. Lean: Flow Cycle. Scrumban: continuous (no fixed iteration).' },
              { term: 'Kaizen', def: 'Lean concept of continuous, incremental improvement made frequently by everyone involved in the process.' },
              { term: 'Kanban Board', def: 'Visual work management: columns = workflow states; cards = work items. WIP limits cap items per column.' },
              { term: 'Key Result (KR)', def: 'OKRs: measurable, time-bound outcome tracking progress toward an Objective. Graded 0.0–1.0 at period close.' },
              { term: 'Launchpad', def: 'PMForge\'s project creation wizard. Guides through industry, methodology, and project name selection; seeds starter artifacts.' },
              { term: 'Lead Time', def: 'Total time from request to delivery. Includes queue time + cycle time.' },
              { term: 'LS / LF', def: 'Latest Start / Latest Finish. Calculated during the backward pass through a CPM network.' },
              { term: 'Milestone', def: 'Significant project event with no duration. Scrum: Definition of Done. Kanban: Throughput Target. CPM: Schedule Milestone.' },
              { term: 'Objective', def: 'OKRs: qualitative, inspiring goal statement answering "where do we want to go?" Supported by measurable Key Results.' },
              { term: 'OKRs', def: 'Objectives and Key Results. Goal-setting framework aligning teams to strategic outcomes through measurable, time-bound Key Results.' },
              { term: 'PAdES', def: 'PDF Advanced Electronic Signatures. Standard for embedding cryptographic signatures into PDF files. PMForge generates PAdES-compliant signed exports.' },
              { term: 'Pareto Chart', def: 'Bar chart sorted descending with a cumulative-percentage line. Based on the 80/20 principle: ~80% of effects come from ~20% of causes.' },
              { term: 'PERT', def: 'Program Evaluation and Review Technique. Network scheduling with three-point duration estimates per activity.' },
              { term: 'Planning Meeting', def: 'Scrum: Sprint Planning. Kanban: Replenishment Meeting. Lean: Plan-Do-Check-Act review. PRINCE2: Stage Planning Meeting.' },
              { term: 'PMBOK', def: 'Project Management Body of Knowledge. PMI\'s comprehensive PM framework. A knowledge standard, not a delivery methodology.' },
              { term: 'PRINCE2', def: 'Projects In Controlled Environments. Process-based framework with seven principles, themes, and processes. Strong governance and stage-gate controls.' },
              { term: 'Process Group', def: 'PMBOK grouping of related PM processes: Initiating, Planning, Executing, Monitoring & Controlling, Closing. Corresponds to PMForge\'s document Phase categories.' },
              { term: 'Project Charter', def: 'Foundational document formally authorizing a project. Captures purpose, objectives, scope, sponsor, high-level schedule and budget.' },
              { term: 'Pull System', def: 'Work produced when downstream capacity exists, not pushed by a schedule. Core Lean and Kanban principle.' },
              { term: 'RACI Matrix', def: 'Responsibility assignment: Responsible (does work), Accountable (owns outcome), Consulted (input needed), Informed (kept updated). One A per task.' },
              { term: 'Recovery Codes', def: 'PMForge one-time codes that allow passphrase reset from the login screen without the current passphrase. Generate from App Settings. Required before enabling database encryption.' },
              { term: 'Retrospective', def: 'Process improvement ceremony. Scrum: Sprint Retrospective. Kanban: Retrospective. PRINCE2: End-Stage Assessment lessons.' },
              { term: 'Risk Register', def: 'Catalogue of potential future risks with probability, impact, owner, mitigation strategy, and contingency plan.' },
              { term: 'Scrum', def: 'Agile framework with time-boxed sprints, Product Backlog, and three roles. Four ceremonies: Sprint Planning, Daily Scrum, Sprint Review, Retrospective.' },
              { term: 'Scrumban', def: 'Hybrid of Scrum\'s prioritized backlog and Kanban\'s continuous pull flow. No fixed sprint cadence.' },
              { term: 'Sigma Level', def: 'Process quality metric. 6 sigma = 3.4 DPMO. Use Control Charts and Pareto Charts to track improvement.' },
              { term: 'SIPOC', def: 'High-level process map: Suppliers → Inputs → Process → Outputs → Customers. Defines process boundaries in DMAIC Define phase.' },
              { term: 'Solution Matrix', def: 'Six Sigma Improve tool. Evaluates solutions on Impact, Effort, Risk, and Cost. Statuses: Proposed, Pilot, Implemented.' },
              { term: 'Sprint', def: 'Scrum time-boxed iteration (typically 2 weeks). Contains a Sprint Goal, committed backlog items, Review, and Retrospective.' },
              { term: 'SQLCipher', def: 'Open-source encrypted SQLite extension used by PMForge for database encryption at rest. Applies AES-256 to the entire project database file.' },
              { term: 'Stakeholder', def: 'Individual or group with an interest in or influence over the project. Mapped by power/interest in the Stakeholder Analysis Matrix.' },
              { term: 'Story (Scrum)', def: 'User-facing work item: "As a [role], I want [feature] so that [benefit]." Equivalent to a Task in most other methodologies.' },
              { term: 'Story Points', def: 'Relative effort estimate in Scrum. Team-defined scale (typically Fibonacci). Not correlated to hours.' },
              { term: 'SWOT Matrix', def: '2x2 strategic analysis: Strengths, Weaknesses (internal) vs. Opportunities, Threats (external).' },
              { term: 'Throughput', def: 'Work items completed per unit time. Primary Kanban flow metric alongside cycle time.' },
              { term: 'Timeline', def: 'PMForge view showing the project\'s chronological event stream as an SVG strip: sprint bands, milestones, and holiday markers.' },
              { term: 'User Story', def: 'See Story (Scrum).' },
              { term: 'Velocity', def: 'Average story points completed per sprint. Used for capacity planning and release forecasting.' },
              { term: 'VoC', def: 'Voice of Customer. Customer needs captured via surveys, interviews, or complaints. Input to CTQ derivation in Six Sigma.' },
              { term: 'WBS', def: 'Work Breakdown Structure. Hierarchical decomposition of total project scope into deliverables and work packages.' },
              { term: 'WIP Limit', def: 'Work-In-Progress Limit. Maximum items in a Kanban column simultaneously. Prevents overloading and surfaces bottlenecks.' },
            ] as entry}
              <div class="border-b border-slate-800 pb-3">
                <dt class="font-medium text-slate-100">{entry.term}</dt>
                <dd class="text-slate-400 mt-0.5">{entry.def}</dd>
              </div>
            {/each}
          </div>

        <!-- ── Installing & Running ───────────────────────────────── -->
        {:else if active === 'install'}
          <h2 class="text-xl font-bold text-slate-100 mb-2">Installing &amp; Running PMForge</h2>
          <p class="text-sm text-slate-400 mb-5">
            PMForge ships as a native installer for each platform. Download the
            file for your operating system from the project's Releases page and
            follow the steps below. The same guide lives in
            <code class="text-cyan-300">docs/INSTALL.md</code>.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Which file to download</h3>
            <ul class="text-sm text-slate-300 space-y-1 list-disc pl-5">
              <li><strong>Windows</strong> — <code>PMForge-…-amd64-setup.exe</code> (installer)</li>
              <li><strong>macOS (Apple Silicon)</strong> — <code>PMForge-…-arm64.dmg</code></li>
              <li><strong>Debian / Ubuntu</strong> — <code>pmforge-…-amd64.deb</code></li>
              <li><strong>Fedora / RHEL / openSUSE</strong> — <code>pmforge-…-x86_64.rpm</code></li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Install steps</h3>
            <ul class="text-sm text-slate-300 space-y-2 list-disc pl-5">
              <li><strong>Windows:</strong> double-click the <code>.exe</code> and follow the installer. Current builds are unsigned, so SmartScreen may warn — choose <em>More info → Run anyway</em>.</li>
              <li><strong>macOS:</strong> open the <code>.dmg</code> and drag <strong>PMForge</strong> to Applications. Unsigned builds trigger Gatekeeper — right-click the app then <em>Open</em> (or System Settings → Privacy &amp; Security → <em>Open Anyway</em>).</li>
              <li><strong>.deb:</strong> <code>sudo apt install ./pmforge-*.deb</code></li>
              <li><strong>.rpm:</strong> <code>sudo dnf install ./pmforge-*.rpm</code></li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Run from source</h3>
            <p class="text-sm text-slate-300 mb-2">
              Requires Go, Node, and the Wails CLI (plus GTK/WebKit dev packages
              on Linux):
            </p>
            <ul class="text-sm text-slate-300 space-y-1 list-disc pl-5">
              <li>In <code>frontend/</code>, run <code>npm ci</code> — use <code>npm ci</code>, <strong>not</strong> <code>npm install</code>.</li>
              <li>On Ubuntu 24.04+ Linux hosts, install <code>libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config</code>. PMForge builds with the Wails <code>webkit2_41</code> tag; GTK4/WebKitGTK 6.0 support requires a future Wails migration.</li>
              <li><code>make build</code> — produce the desktop binary/app with DuckDB analytics and the current Linux WebKit tag.</li>
              <li><code>make dev</code> — hot-reload development mode.</li>
            </ul>
            <p class="text-xs text-slate-500 mt-2">
              Full prerequisites, the per-format packaging commands, and signing
              notes are in <code class="text-cyan-300">docs/INSTALL.md</code>.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Your data stays local</h3>
            <p class="text-sm text-slate-300">
              However you install PMForge, every project lives in an encrypted
              file on your own machine — no account, cloud, or network is
              required. See <em>Database Encryption</em> for details.
            </p>
          </section>

        {/if}

      </div>
    </main>
  </div>
</div>
