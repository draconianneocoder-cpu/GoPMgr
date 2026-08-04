<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ProjectLaunchpad replaces the old "name + description" inline
  // form in ProjectPicker. Four-step wizard:
  //
  //   1. Industry tile selection      (Business / Admin / Engineering / Software / Construction / Custom)
  //   2. Sub-category (industry-aware list)
  //   3. Methodology (recommended set for the industry; user can override)
  //   4. Name + description + calendar policy + seed-artifact checkboxes
  //
  // On submit calls CreateProjectFromLaunchpad. The seed list shown
  // in step 4 comes from the backend's zen-go evaluation, so adding
  // a new industry/methodology row to the JDM auto-extends the GUI
  // suggestions.

  import { onMount, tick } from 'svelte';

  // Props — Launchpad can be opened from ProjectPicker; on close we
  // notify the parent so it can refresh its list.
  let {
    onCreated,
    onCancel,
  }: {
    onCreated: (project: ProjectMeta, projectPath?: string) => void;
    onCancel: () => void;
  } = $props();

  type Step = 1 | 2 | 3 | 4;
  const STEPS: { id: Step; label: string }[] = [
    { id: 1, label: 'Industry' },
    { id: 2, label: 'Focus' },
    { id: 3, label: 'Method' },
    { id: 4, label: 'Setup' },
  ];
  let step = $state<Step>(1);

  // Focus management: each step swaps out its content, which would otherwise
  // strand keyboard / screen-reader focus on <body>. On every step change we
  // move focus to the new step's heading so the change is announced and Tab
  // resumes from a sensible place. Skip the very first render so opening the
  // wizard does not yank focus unexpectedly.
  let headingEl = $state<HTMLElement>();
  let stepInitialised = false;
  $effect(() => {
    void step;
    if (!stepInitialised) {
      stepInitialised = true;
      return;
    }
    tick().then(() => headingEl?.focus());
  });

  // Selections
  let industry = $state('');
  let subCategory = $state('');
  let methodology = $state('');
  let name = $state('');
  let description = $state('');
  let countryCode = $state('US');
  let timeZone = $state('America/New_York');
  let calendarPolicies = $state<CalendarPolicy[]>([]);

  // Seed picker state
  let suggestedSeeds = $state<string[]>([]);
  let seedsChecked = $state<Record<string, boolean>>({});

  let busy = $state(false);
  let error = $state('');
  let calendarError = $state('');

  let canContinue = $derived(
    (step === 1 && industry !== '') ||
      (step === 2 && subCategory !== '') ||
      (step === 3 && methodology !== ''),
  );

  const INDUSTRIES = [
    { id: 'business',       label: 'Business',       blurb: 'Marketing, sales, finance, HR, operations.' },
    { id: 'administration', label: 'Administration', blurb: 'Legal, public sector, executive support, facilities.' },
    { id: 'engineering',    label: 'Engineering',    blurb: 'R&D, civil, mechanical, electrical, manufacturing.' },
    { id: 'software',       label: 'Software',       blurb: 'Web, mobile, AI/ML, DevOps, game dev.' },
    { id: 'construction',   label: 'Construction',   blurb: 'Residential, commercial, infrastructure, renovation.' },
    { id: 'custom',         label: 'Custom',         blurb: 'Blank slate — pick everything yourself.' },
  ];

  const SUB_CATEGORIES: Record<string, string[]> = {
    business:       ['Marketing', 'Sales', 'Finance', 'HR', 'Operations'],
    administration: ['Legal', 'Public Sector', 'Executive Support', 'Facility Management'],
    engineering:    ['R&D', 'Civil', 'Mechanical', 'Electrical', 'Manufacturing'],
    software:       ['Web Dev', 'Mobile App', 'AI/ML', 'DevOps', 'Game Dev'],
    construction:   ['Residential', 'Commercial', 'Infrastructure', 'Renovation'],
    custom:         ['General'],
  };

  // Methodology recommendations per industry (lowercase IDs match
  // the JDM's `methodology` column).
  const METHODOLOGIES: Record<string, { id: string; label: string; blurb: string }[]> = {
    business: [
      { id: 'lean',      label: 'Lean',      blurb: 'Eliminate waste; flow-based.' },
      { id: 'six_sigma', label: 'Six Sigma', blurb: 'Process improvement via DMAIC.' },
      { id: 'okrs',      label: 'OKRs',      blurb: 'Objectives & key results.' },
    ],
    administration: [
      { id: 'waterfall', label: 'Waterfall', blurb: 'Linear, sequential phases.' },
      { id: 'prince2',   label: 'PRINCE2',   blurb: 'Stage-gated governance.' },
      { id: 'pmbok',     label: 'PMBOK',     blurb: 'PMI process groups.' },
    ],
    engineering: [
      { id: 'cpm',       label: 'Critical Path', blurb: 'Network-based scheduling.' },
      { id: 'waterfall', label: 'Waterfall',     blurb: 'Sequential design / build / test.' },
      { id: 'six_sigma', label: 'Six Sigma',     blurb: 'Quality control loops.' },
    ],
    software: [
      { id: 'scrum',    label: 'Scrum',    blurb: 'Time-boxed sprints, backlog.' },
      { id: 'kanban',   label: 'Kanban',   blurb: 'Continuous flow, WIP limits.' },
      { id: 'scrumban', label: 'Scrumban', blurb: 'Hybrid: backlog + flow.' },
    ],
    construction: [
      { id: 'waterfall', label: 'Waterfall',         blurb: 'Phase-gated build.' },
      { id: 'lean',      label: 'Lean Construction', blurb: 'Pull planning; minimise waste.' },
      { id: 'cpm',       label: 'CPM',               blurb: 'Critical-path scheduling.' },
    ],
    custom: [
      { id: 'custom', label: 'Build it yourself', blurb: 'No starter artifacts.' },
    ],
  };

  onMount(async () => {
    try {
      calendarPolicies = (await window.go.main.App.ListCalendarPolicies()) ?? [];
      const current = calendarPolicies.find((policy) => policy.country_code === countryCode);
      if (current) {
        timeZone = current.time_zones[0] ?? 'UTC';
      } else if (calendarPolicies.length > 0) {
        countryCode = calendarPolicies[0].country_code;
        timeZone = calendarPolicies[0].time_zones[0] ?? 'UTC';
      } else {
        calendarError = 'No business-calendar policies are available.';
      }
    } catch (err: any) {
      calendarError = `Could not load business-calendar policies: ${err}`;
    }
  });

  function timeZonesFor(country: string): string[] {
    return calendarPolicies.find((policy) => policy.country_code === country)?.time_zones ?? [];
  }

  function updateCalendarPolicy() {
    const allowed = timeZonesFor(countryCode);
    // Preserve an explicit choice when it remains valid; otherwise use the
    // backend policy's first zone, which is also its documented default.
    if (!allowed.includes(timeZone)) timeZone = allowed[0] ?? '';
  }

  // When the user finishes step 3, ask the backend (zen-go) for the
  // recommended seed list and check them all by default.
  async function loadSeeds() {
    busy = true;
    error = '';
    try {
      const seeds = (await window.go.main.App.LaunchpadEvaluate(industry, methodology)) ?? [];
      suggestedSeeds = seeds;
      const next: Record<string, boolean> = {};
      for (const s of seeds) next[s] = true;
      seedsChecked = next;
    } catch (err: any) {
      error = `Could not load seed suggestions: ${err}`;
      suggestedSeeds = [];
    } finally {
      busy = false;
    }
  }

  async function create() {
    busy = true;
    error = '';
    try {
      const seeds = suggestedSeeds.filter((s) => seedsChecked[s]);
      const res = await window.go.main.App.CreateProjectFromLaunchpad(
        name,
        description,
        industry,
        subCategory,
        methodology,
        countryCode,
        timeZone,
        seeds,
      );
      onCreated(res.project, res.path);
    } catch (err: any) {
      error = `Create failed: ${err}`;
    } finally {
      busy = false;
    }
  }

  function next() {
    if (step === 3) {
      void loadSeeds();
    }
    if (step < 4) step = (step + 1) as Step;
  }
  function prev() {
    if (step > 1) step = (step - 1) as Step;
  }

  function selectIndustry(id: string) {
    if (industry === id) return;
    industry = id;
    subCategory = '';
    methodology = '';
  }

  function methodologyLabel(): string {
    return METHODOLOGIES[industry]?.find((item) => item.id === methodology)?.label ?? methodology;
  }

  function industryLabel(): string {
    return INDUSTRIES.find((item) => item.id === industry)?.label ?? industry;
  }

  // Pretty labels for seed strings the backend returns.
  const SEED_LABELS: Record<string, string> = {
    kanban:                   'Kanban board (default 4 columns)',
    backlog:                  '3 placeholder backlog items',
    sprint1:                  'Sprint 1 in planning state',
    wbs:                      'Work Breakdown Structure (empty root)',
    cpm:                      'CPM schedule (empty)',
    fishbone:                 'Fishbone (root-cause) diagram',
    control:                  'Control chart',
    pareto:                   'Pareto chart',
    cumulative_flow:          'Cumulative Flow diagram',
    swot:                     'SWOT matrix',
    charter:                  'Project Charter document',
    plan_word:                'Project Plan document',
    statement_of_work:        'Statement of Work',
    scope_statement:          'Scope Statement',
    risk_register:            'Risk Register',
    communication_plan:       'Communication Plan',
    status_report:            'Initial Status Report',
    stakeholder_analysis_doc: 'Stakeholder Analysis',
  };
  function seedLabel(s: string): string {
    return SEED_LABELS[s] ?? s;
  }

</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">
      New Project · Step {step} of 4
    </h1>
    <button onclick={onCancel} class="text-xs text-slate-400 hover:text-cyan-400">
      Cancel
    </button>
  </header>

  <ol
    aria-label="Project creation progress"
    class="grid grid-cols-4 border-b border-slate-800 bg-slate-950"
  >
    {#each STEPS as item}
      <li
        aria-current={item.id === step ? 'step' : undefined}
        class="border-t-2 px-3 py-2 text-center text-[11px] font-semibold uppercase tracking-wider
          {item.id <= step ? 'border-cyan-500 text-slate-200' : 'border-slate-800 text-slate-600'}"
      >
        <span class="mr-1 text-slate-500">{item.id}.</span>{item.label}
      </li>
    {/each}
  </ol>

  <main class="flex-1 p-8 max-w-5xl mx-auto w-full">
    {#if error}
      <p class="text-xs text-red-400 mb-3" role="alert">{error}</p>
    {/if}
    {#if calendarError}
      <p class="text-xs text-red-400 mb-3" role="alert">{calendarError}</p>
    {/if}

    {#if step === 1}
      <h2 bind:this={headingEl} tabindex="-1" class="text-lg font-bold mb-6 outline-none">What kind of project is this?</h2>
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {#each INDUSTRIES as ind (ind.id)}
          <button
            onclick={() => selectIndustry(ind.id)}
            aria-pressed={industry === ind.id}
            class="p-5 bg-slate-900 hover:bg-slate-800 border rounded-lg text-left
              {industry === ind.id ? 'border-cyan-500 ring-1 ring-cyan-500/30' : 'border-slate-800'}"
          >
            <div class="text-base font-bold text-slate-50">{ind.label}</div>
            <p class="text-xs text-slate-500 mt-1">{ind.blurb}</p>
          </button>
        {/each}
      </div>
    {:else if step === 2}
      <h2 bind:this={headingEl} tabindex="-1" class="text-lg font-bold mb-6 outline-none">
        Narrow it down (<span class="text-cyan-400">{industry}</span>)
      </h2>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-3">
        {#each SUB_CATEGORIES[industry] ?? [] as sub (sub)}
          <button
            onclick={() => { subCategory = sub; }}
            aria-pressed={subCategory === sub}
            class="p-4 bg-slate-900 hover:bg-slate-800 border rounded text-left text-sm
              {subCategory === sub ? 'border-cyan-500 ring-1 ring-cyan-500/30' : 'border-slate-800'}"
          >
            {sub}
          </button>
        {/each}
      </div>
    {:else if step === 3}
      <h2 bind:this={headingEl} tabindex="-1" class="text-lg font-bold mb-6 outline-none">
        Which methodology fits best?
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        {#each METHODOLOGIES[industry] ?? [] as m (m.id)}
          <button
            onclick={() => { methodology = m.id; }}
            aria-pressed={methodology === m.id}
            class="p-5 bg-slate-900 hover:bg-slate-800 border rounded-lg text-left
              {methodology === m.id ? 'border-cyan-500 ring-1 ring-cyan-500/30' : 'border-slate-800'}"
          >
            <div class="text-base font-bold text-slate-50">{m.label}</div>
            <p class="text-xs text-slate-500 mt-1">{m.blurb}</p>
          </button>
        {/each}
      </div>
    {:else}
      <h2 bind:this={headingEl} tabindex="-1" class="text-lg font-bold mb-6 outline-none">Project details &amp; starter artifacts</h2>

      <section class="mb-6 rounded-lg border border-slate-800 bg-slate-900/60 p-4" aria-labelledby="blueprint-heading">
        <h3 id="blueprint-heading" class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-3">
          Project blueprint
        </h3>
        <dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
          <div>
            <dt class="text-[11px] uppercase text-slate-500">Industry</dt>
            <dd class="mt-1 text-slate-100">{industryLabel()}</dd>
          </div>
          <div>
            <dt class="text-[11px] uppercase text-slate-500">Focus</dt>
            <dd class="mt-1 text-slate-100">{subCategory}</dd>
          </div>
          <div>
            <dt class="text-[11px] uppercase text-slate-500">Method</dt>
            <dd class="mt-1 text-slate-100">{methodologyLabel()}</dd>
          </div>
        </dl>
      </section>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Project name</span>
          <input
            bind:value={name}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            required
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Business calendar policy</span>
          <select
            bind:value={countryCode}
            onchange={updateCalendarPolicy}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
          >
            {#each calendarPolicies as policy (policy.country_code)}
              <option value={policy.country_code}>{policy.name}</option>
            {/each}
          </select>
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Schedule and chart time zone</span>
          <select
            bind:value={timeZone}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
          >
            {#each timeZonesFor(countryCode) as zone (zone)}
              <option value={zone}>{zone}</option>
            {/each}
          </select>
        </label>
      </div>
      <label class="block mb-6">
        <span class="text-xs text-slate-500 uppercase">Description</span>
        <textarea
          bind:value={description}
          rows="2"
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        ></textarea>
      </label>

      <div class="mb-6">
        <h3 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Starter artifacts (suggested for {industry} + {methodology})
        </h3>
        {#if busy && suggestedSeeds.length === 0}
          <p class="text-xs text-slate-500" role="status" aria-live="polite">Loading recommendations…</p>
        {:else if suggestedSeeds.length === 0}
          <p class="text-xs text-slate-500">
            No suggestions for this combination — you'll start with an empty project.
          </p>
        {:else}
          <ul class="space-y-1">
            {#each suggestedSeeds as s (s)}
              <li>
                <label class="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    bind:checked={seedsChecked[s]}
                    class="accent-cyan-500"
                  />
                  <span>{seedLabel(s)}</span>
                </label>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <div class="flex items-center justify-between">
        <button onclick={prev} class="text-xs text-slate-400 hover:text-cyan-400">
          ← Back
        </button>
        <button
          onclick={create}
          disabled={busy || !name || !countryCode || !timeZone}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
        >
          {busy ? 'Creating…' : 'Create project'}
        </button>
      </div>
    {/if}

    {#if step < 4}
      <div class="mt-8 flex items-center justify-between border-t border-slate-800 pt-5">
        {#if step > 1}
          <button onclick={prev} class="text-xs text-slate-400 hover:text-cyan-400">
            ← Back
          </button>
        {:else}
          <span></span>
        {/if}
        <button
          onclick={next}
          disabled={!canContinue || busy}
          class="rounded bg-cyan-600 px-4 py-2 text-xs font-bold uppercase text-white
            hover:bg-cyan-500 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {step === 3 && busy ? 'Loading…' : 'Continue'}
        </button>
      </div>
    {/if}
  </main>
</div>
